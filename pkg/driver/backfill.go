/*
Copyright 2023 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/cloud"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// BackfillInterval is the time between two cluster tag backfill sweeps. New resources get the
// driver's tags at create time, so later sweeps only pick up resources an administrator tagged.
var BackfillInterval = time.Hour

// maxListedResources caps the resource lists in the sweep summary line.
const maxListedResources = 20

const volumeParamsParentVolumeId = "ParentVolumeId"

// provisionedByAnnotation is set by the external-provisioner on PersistentVolumes it provisioned.
const provisionedByAnnotation = "pv.kubernetes.io/provisioned-by"

var volumeSnapshotContentResource = schema.GroupVersionResource{
	Group:    "snapshot.storage.k8s.io",
	Version:  "v1",
	Resource: "volumesnapshotcontents",
}

// handle is the FSx resource ID of a PersistentVolume or VolumeSnapshotContent of this driver.
type handle struct {
	// ID is the volume or snapshot handle.
	ID string
	// Provisioned is true when the controller created the resource for this object: the PersistentVolume
	// was dynamically provisioned by this driver, or the VolumeSnapshotContent was dynamically created.
	Provisioned bool
	// Name is the PersistentVolume name, which the controller gives to the volumes it creates. Empty for
	// VolumeSnapshotContents.
	Name string
}

// clusterReferences are the FSx resource IDs that this cluster's Kubernetes objects refer to.
type clusterReferences struct {
	// Handles are the volume handles of this driver's PersistentVolumes and the snapshot handles of its
	// VolumeSnapshotContents. They are the resources the controller manages for this cluster.
	Handles []handle
	// ParentVolumes are the ParentVolumeId parameters of this driver's StorageClasses.
	ParentVolumes []string
}

// referenceLister lists the FSx resources that this cluster's Kubernetes objects refer to.
type referenceLister interface {
	List(ctx context.Context) (*clusterReferences, error)
}

// kubernetesReferenceLister reads PersistentVolumes, VolumeSnapshotContents and StorageClasses.
type kubernetesReferenceLister struct {
	client  kubernetes.Interface
	dynamic dynamic.Interface
}

func newKubernetesReferenceLister() (referenceLister, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &kubernetesReferenceLister{client: client, dynamic: dyn}, nil
}

func (l *kubernetesReferenceLister) List(ctx context.Context) (*clusterReferences, error) {
	refs := &clusterReferences{}

	pvs, err := l.client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, pv := range pvs.Items {
		if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == DriverName && pv.Spec.CSI.VolumeHandle != "" {
			refs.Handles = append(refs.Handles, handle{
				ID:          pv.Spec.CSI.VolumeHandle,
				Provisioned: pv.Annotations[provisionedByAnnotation] == DriverName,
				Name:        pv.Name,
			})
		}
	}

	contents, err := l.dynamic.Resource(volumeSnapshotContentResource).List(ctx, metav1.ListOptions{})
	switch {
	case err == nil:
		for _, c := range contents.Items {
			if h := snapshotHandle(&c); h.ID != "" {
				refs.Handles = append(refs.Handles, h)
			}
		}
	case apierrors.IsNotFound(err):
		// The snapshot CRDs are not installed.
	default:
		return nil, err
	}

	classes, err := l.client.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, sc := range classes.Items {
		if sc.Provisioner != DriverName {
			continue
		}
		if id := unquote(sc.Parameters[volumeParamsParentVolumeId]); id != "" {
			refs.ParentVolumes = append(refs.ParentVolumes, id)
		}
	}
	return refs, nil
}

// snapshotHandle returns the FSx snapshot of a VolumeSnapshotContent of this driver. A dynamically
// created VolumeSnapshotContent has spec.source.volumeHandle; a pre-provisioned one has
// spec.source.snapshotHandle.
func snapshotHandle(c *unstructured.Unstructured) handle {
	if driver, _, _ := unstructured.NestedString(c.Object, "spec", "driver"); driver != DriverName {
		return handle{}
	}
	source, _, _ := unstructured.NestedString(c.Object, "spec", "source", "volumeHandle")
	if id, _, _ := unstructured.NestedString(c.Object, "status", "snapshotHandle"); id != "" {
		return handle{ID: id, Provisioned: source != ""}
	}
	id, _, _ := unstructured.NestedString(c.Object, "spec", "source", "snapshotHandle")
	return handle{ID: id}
}

// unquote returns a JSON string parameter value without its quotes.
func unquote(value string) string {
	value = strings.TrimSpace(value)
	if u, err := strconv.Unquote(value); err == nil {
		return u
	}
	return value
}

// backfillCounts are the results of one sweep. The counters mean:
//   - tagged: tag calls that succeeded;
//   - failed: tag calls that failed (not also counted in remaining);
//   - needsAdministrator: distinct resources reported for an administrator;
//   - remaining: resources, or whole listings, that could not be checked because a list or
//     describe call failed. A failed Kubernetes listing counts 1.
type backfillCounts struct {
	tagged             int
	failed             int
	needsAdministrator int
	remaining          int

	failedResources             []string
	needsAdministratorResources []string
	firstError                  error
}

func (c *backfillCounts) setError(err error) {
	if c.firstError == nil {
		c.firstError = err
	}
}

func (c *backfillCounts) clean() bool {
	return c.failed == 0 && c.needsAdministrator == 0 && c.remaining == 0
}

// summary returns the key/value pairs of the "cluster tag backfill sweep finished" line.
func (c *backfillCounts) summary() []interface{} {
	kv := []interface{}{"tagged", c.tagged, "failed", c.failed, "needsAdministrator", c.needsAdministrator, "remaining", c.remaining}
	kv = appendResourceList(kv, "needsAdministrator", c.needsAdministratorResources)
	kv = appendResourceList(kv, "failed", c.failedResources)
	if (c.failed > 0 || c.remaining > 0) && c.firstError != nil {
		kv = append(kv, "firstError", c.firstError.Error())
	}
	return kv
}

// appendResourceList adds <name>Resources with at most maxListedResources ARNs, and <name>Total
// when the list was cut.
func appendResourceList(kv []interface{}, name string, resources []string) []interface{} {
	if len(resources) == 0 {
		return kv
	}
	if len(resources) > maxListedResources {
		return append(kv, name+"Resources", resources[:maxListedResources], name+"Total", len(resources))
	}
	return append(kv, name+"Resources", resources)
}

// backfiller adds the driver's tags to existing resources the controller created for this cluster.
// In backfill it adds the cluster tag. In disable it adds the driver tag, which only the root volumes
// of the file systems the controller created can lack.
type backfiller struct {
	cloud    cloud.Cloud
	lister   referenceLister
	options  *ClusterTagOptions
	complete bool
}

// runBackfill sweeps at startup and then every BackfillInterval until ctx is done.
func (d *controllerService) runBackfill(ctx context.Context) {
	lister, err := newKubernetesReferenceLister()
	if err != nil {
		klog.ErrorS(err, "failed to create Kubernetes client for cluster tag backfill")
		return
	}
	b := &backfiller{cloud: d.cloud, lister: lister, options: d.clusterTagOptions()}
	for {
		b.sweep(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(BackfillInterval):
		}
	}
}

// tag returns the key and value the sweep adds: the cluster tag in backfill, the driver tag otherwise.
func (b *backfiller) tag() (string, string) {
	if b.options.isolation() == ClusterTagIsolationBackfill {
		return b.options.TagKey, b.options.ClusterName
	}
	return AwsFsxOpenZfsDriverTagKey, "true"
}

// sweep visits every resource this cluster's PersistentVolumes, VolumeSnapshotContents and StorageClasses
// refer to, and the root volumes of the file systems among them. The controller owns a resource when it
// carries the driver tag, its PersistentVolume was provisioned by this driver (and, for a volume, the volume
// has the PersistentVolume's name), or its VolumeSnapshotContent was created dynamically. It also owns the
// root volume of a file system it owns. An owned resource without the tag gets it, in a request that
// contains only that tag; in backfill, the root volume must carry the driver tag too, because the backfill
// policy requires it. Every other resource without the tag, or that cannot be found, is reported for an
// administrator.
func (b *backfiller) sweep(ctx context.Context) backfillCounts {
	var counts backfillCounts
	key, value := b.tag()

	refs, err := b.lister.List(ctx)
	if err != nil {
		klog.V(4).InfoS("cluster tag backfill could not list Kubernetes objects", "err", err)
		counts.remaining++
		counts.setError(err)
		b.finish(&counts)
		return counts
	}

	visited := map[string]bool{}
	reported := map[string]bool{}
	needsAdministrator := func(id, resource, current string) {
		if reported[id] {
			return
		}
		reported[id] = true
		counts.needsAdministrator++
		counts.needsAdministratorResources = append(counts.needsAdministratorResources, resource)
		klog.V(4).InfoS("resource needs cluster tag from an administrator", "resource", resource, "tagKey", key, "currentValue", current)
	}
	describe := func(id string) *cloud.Resource {
		if id == "" || visited[id] {
			return nil
		}
		visited[id] = true
		r, err := b.cloud.DescribeResource(ctx, id)
		if err != nil {
			if errors.Is(err, cloud.ErrNotFound) {
				// A deleted resource, or one these credentials cannot see. Either way an administrator decides.
				klog.V(4).InfoS("cluster tag backfill: resource not found", "resource", id)
				needsAdministrator(id, id, "")
				return nil
			}
			klog.V(4).InfoS("cluster tag backfill could not read resource", "resource", id, "err", err)
			counts.remaining++
			counts.setError(err)
			return nil
		}
		return r
	}
	// visit tags r if the controller owns it, or reports it if it lacks the tag.
	visit := func(r *cloud.Resource, owned bool) {
		current, hasKey := r.Tags[key]
		switch {
		case hasKey && current == value:
		case !hasKey && owned:
			if err := b.cloud.TagResource(ctx, r.ARN, map[string]string{key: value}); err != nil {
				counts.failed++
				counts.failedResources = append(counts.failedResources, r.ARN)
				counts.setError(err)
				klog.V(4).InfoS("failed to add cluster tag to existing resource", "resource", r.ARN, "tagKey", key, "err", err)
				return
			}
			counts.tagged++
			r.Tags[key] = value
			klog.V(4).InfoS("added cluster tag to existing resource", "resource", r.ARN, "tagKey", key)
		default:
			needsAdministrator(r.ID, r.ARN, current)
		}
	}

	handles := append([]handle(nil), refs.Handles...)
	sort.Slice(handles, func(i, j int) bool { return handles[i].ID < handles[j].ID })
	var related []string
	for _, h := range handles {
		r := describe(h.ID)
		if r == nil {
			continue
		}
		owned := h.Provisioned && r.Tags[AwsFsxOpenZfsDriverTagKey] == "true" &&
			(!strings.HasPrefix(r.ID, cloud.VolumePrefix+"-") || r.Name == h.Name)
		visit(r, owned)
		// The root volume of a file system the driver created holds the snapshots and child volumes of that
		// file system. It carries the driver tag only if the StorageClass set CopyTagsToVolumes.
		if r.RootVolumeID != "" && r.Tags[AwsFsxOpenZfsDriverTagKey] == "true" {
			if root := describe(r.RootVolumeID); root != nil {
				visit(root, owned && (key == AwsFsxOpenZfsDriverTagKey || root.Tags[AwsFsxOpenZfsDriverTagKey] == "true"))
			}
		}
		related = append(related, r.ParentVolumeID)
	}
	related = append(related, refs.ParentVolumes...)
	sort.Strings(related)
	for _, id := range related {
		if r := describe(id); r != nil {
			visit(r, false)
		}
	}

	b.finish(&counts)
	return counts
}

// finish logs the one summary line of a sweep, and the completion line once per process, after
// the first sweep with nothing left to do. It is never logged again, even after a later sweep
// that is not clean.
func (b *backfiller) finish(counts *backfillCounts) {
	klog.InfoS("cluster tag backfill sweep finished", counts.summary()...)
	if counts.clean() && !b.complete {
		b.complete = true
		key, value := b.tag()
		klog.InfoS("cluster tag backfill complete", "tagKey", key, "tagValue", value)
	}
}
