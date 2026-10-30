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
	"fmt"
	"sort"
	"strings"

	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/cloud"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

// ClusterTagIsolation is the --cluster-tag-isolation state.
type ClusterTagIsolation string

const (
	// ClusterTagIsolationDisable keeps the default behavior, plus the cluster tag on new resources. It
	// adds the driver tag to the root volumes of existing file systems the driver created.
	ClusterTagIsolationDisable ClusterTagIsolation = "disable"
	// ClusterTagIsolationBackfill tags existing resources the driver created for this cluster. It does not filter.
	ClusterTagIsolationBackfill ClusterTagIsolation = "backfill"
	// ClusterTagIsolationEnforce ignores resources that lack this cluster's tag value. It does not sweep.
	ClusterTagIsolationEnforce ClusterTagIsolation = "enforce"
)

const (
	awsTagPrefix        = "aws:"
	maxTagKeyLength     = 128
	maxTagValueLength   = 256
	clusterNameFlag     = "--cluster-name"
	clusterTagKeyFlag   = "--cluster-tag-key"
	isolationFlag       = "--cluster-tag-isolation"
	extraTagsFlag       = "--extra-tags"
	clusterTagParamErr  = "Tags must not set the cluster tag %q; the driver sets it to the cluster name %q"
	reservedTagParamErr = "Tags must not set %q; the driver sets it"
)

// ClusterTagOptions configures the cluster tag, cluster tag isolation and extra tags of the controller.
type ClusterTagOptions struct {
	// ClusterName is the value of the cluster tag.
	ClusterName string
	// TagKey is the key of the cluster tag.
	TagKey string
	// Isolation is the cluster tag isolation state.
	Isolation ClusterTagIsolation
	// ExtraTags are added to every resource the controller creates.
	ExtraTags map[string]string
}

// ParseClusterTagIsolation parses a --cluster-tag-isolation value: exactly "", disable, backfill or
// enforce. An empty value means disable. Values are not trimmed or lowercased.
func ParseClusterTagIsolation(value string) (ClusterTagIsolation, error) {
	switch ClusterTagIsolation(value) {
	case "", ClusterTagIsolationDisable:
		return ClusterTagIsolationDisable, nil
	case ClusterTagIsolationBackfill, ClusterTagIsolationEnforce:
		return ClusterTagIsolation(value), nil
	}
	return "", fmt.Errorf(`invalid %s %q: must be "disable", "backfill" or "enforce"`, isolationFlag, value)
}

// logSettings logs the cluster tag isolation settings once at controller startup. Nil options
// mean disable.
func (o *ClusterTagOptions) logSettings() {
	var name, key string
	if o != nil {
		name, key = o.ClusterName, o.TagKey
	}
	kv := []interface{}{"clusterName", name, "tagKey", key, "clusterTagIsolation", o.isolation()}
	if o.isolation() != ClusterTagIsolationEnforce {
		kv = append(kv, "backfillInterval", BackfillInterval.String())
	}
	klog.InfoS("cluster tag isolation settings", kv...)
	if (name == "") != (key == "") {
		klog.InfoS("only one of " + clusterNameFlag + " and " + clusterTagKeyFlag + " is set; no cluster tag is written")
	}
}

// Validate checks the options. It is called at startup, so invalid settings stop the controller.
func (o *ClusterTagOptions) Validate() error {
	isolation, err := ParseClusterTagIsolation(string(o.Isolation))
	if err != nil {
		return err
	}
	o.Isolation = isolation

	// Whitespace-only values count as empty.
	if strings.TrimSpace(o.ClusterName) == "" {
		o.ClusterName = ""
	}
	if strings.TrimSpace(o.TagKey) == "" {
		o.TagKey = ""
	}
	if o.Isolation != ClusterTagIsolationDisable && (o.ClusterName == "" || o.TagKey == "") {
		return fmt.Errorf("%s=%s requires %s and %s", isolationFlag, o.Isolation, clusterNameFlag, clusterTagKeyFlag)
	}
	if o.TagKey != "" {
		if err := validateDriverTagKey(clusterTagKeyFlag, o.TagKey); err != nil {
			return err
		}
	}
	if len(o.ClusterName) > maxTagValueLength {
		return fmt.Errorf("%s must be at most %d characters", clusterNameFlag, maxTagValueLength)
	}
	for k, v := range o.ExtraTags {
		if err := validateDriverTagKey(extraTagsFlag, k); err != nil {
			return err
		}
		if o.TagKey != "" && k == o.TagKey {
			return fmt.Errorf("%s must not contain the cluster tag key %q", extraTagsFlag, k)
		}
		if len(v) > maxTagValueLength {
			return fmt.Errorf("%s value of %q must be at most %d characters", extraTagsFlag, k, maxTagValueLength)
		}
	}
	return nil
}

// validateDriverTagKey rejects keys that FSx reserves or that the driver already sets itself.
func validateDriverTagKey(flag, key string) error {
	switch {
	case key == "":
		return fmt.Errorf("%s: tag key must not be empty", flag)
	case len(key) > maxTagKeyLength:
		return fmt.Errorf("%s: tag key %q must be at most %d characters", flag, key, maxTagKeyLength)
	case strings.HasPrefix(strings.ToLower(key), awsTagPrefix):
		return fmt.Errorf("%s: tag key %q must not start with %q", flag, key, awsTagPrefix)
	case key == AwsFsxOpenZfsDriverTagKey:
		return fmt.Errorf("%s: tag key %q is set by the driver", flag, key)
	case strings.HasSuffix(key, deletionSuffix):
		return fmt.Errorf("%s: tag key %q must not end with %q, which the driver reserves for deletion parameters", flag, key, deletionSuffix)
	}
	return nil
}

// tagsNewResources reports whether new resources get the cluster tag.
func (o *ClusterTagOptions) tagsNewResources() bool {
	return o != nil && o.ClusterName != "" && o.TagKey != ""
}

// isolation returns the isolation state; nil options mean disable.
func (o *ClusterTagOptions) isolation() ClusterTagIsolation {
	if o == nil || o.Isolation == "" {
		return ClusterTagIsolationDisable
	}
	return o.Isolation
}

// errClusterTagConflict is returned when user-provided tags set a tag key the driver sets itself.
var errClusterTagConflict = errors.New("cluster tag conflict")

// validateProvidedTags rejects tags provided in a StorageClass, VolumeSnapshotClass or deletion parameter
// that set the driver tag or the cluster tag. The error wraps errClusterTagConflict.
func (o *ClusterTagOptions) validateProvidedTags(tags []map[string]string) error {
	for _, tag := range tags {
		switch {
		case tag["Key"] == AwsFsxOpenZfsDriverTagKey:
			return fmt.Errorf("%w: "+reservedTagParamErr, errClusterTagConflict, tag["Key"])
		case o.tagsNewResources() && tag["Key"] == o.TagKey:
			return fmt.Errorf("%w: "+clusterTagParamErr, errClusterTagConflict, o.TagKey, o.ClusterName)
		}
	}
	return nil
}

// mergeTags adds the extra tags and the cluster tag to tags provided in a StorageClass, VolumeSnapshotClass
// or deletion parameter, after validateProvidedTags. Provided tags override extra tags with the same key.
func (o *ClusterTagOptions) mergeTags(tags []map[string]string) []map[string]string {
	if o == nil {
		return tags
	}
	provided := make(map[string]bool, len(tags))
	for _, tag := range tags {
		provided[tag["Key"]] = true
	}
	keys := make([]string, 0, len(o.ExtraTags))
	for k := range o.ExtraTags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !provided[k] {
			tags = append(tags, map[string]string{"Key": k, "Value": o.ExtraTags[k]})
		}
	}
	if o.tagsNewResources() {
		tags = append(tags, map[string]string{"Key": o.TagKey, "Value": o.ClusterName})
	}
	return tags
}

// checkClusterTag runs before the controller acts on an existing file system, volume or snapshot.
// With --cluster-tag-isolation=enforce it refuses resources that lack this cluster's tag value.
// If the tags cannot be read, it logs the error and continues, so IAM decides.
func (d *controllerService) checkClusterTag(ctx context.Context, id string) error {
	o := d.clusterTagOptions()
	if o.isolation() != ClusterTagIsolationEnforce {
		return nil
	}
	r, err := d.cloud.DescribeResource(ctx, id)
	if err != nil {
		if !errors.Is(err, cloud.ErrNotFound) {
			klog.ErrorS(err, "failed to read cluster tag, continuing", "resource", id, "tagKey", o.TagKey)
		}
		return nil
	}
	if r.Tags[o.TagKey] != o.ClusterName {
		klog.InfoS("ignoring resource not tagged for this cluster", "resource", id, "tagKey", o.TagKey, "clusterName", o.ClusterName, "tagValue", r.Tags[o.TagKey])
		return status.Errorf(codes.FailedPrecondition, "resource %q does not have tag %s=%s; the controller ignores it because --cluster-tag-isolation=enforce", id, o.TagKey, o.ClusterName)
	}
	return nil
}

// clusterTagOptions returns the cluster tag options, or nil if none are configured.
func (d *controllerService) clusterTagOptions() *ClusterTagOptions {
	if d.driverOptions == nil {
		return nil
	}
	return d.driverOptions.clusterTag
}
