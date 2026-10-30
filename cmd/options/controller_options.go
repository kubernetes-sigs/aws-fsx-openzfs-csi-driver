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

package options

import (
	flag "github.com/spf13/pflag"

	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/driver"
)

// ControllerOptions contains options and configuration settings for the controller service.
type ControllerOptions struct {
	// ClusterName is the value of the cluster tag.
	ClusterName string
	// ClusterTagKey is the key of the cluster tag.
	ClusterTagKey string
	// ClusterTagIsolation is disable, backfill or enforce.
	ClusterTagIsolation string
	// ExtraTags are added to every AWS resource the controller creates.
	ExtraTags map[string]string
}

func (s *ControllerOptions) AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&s.ClusterName, "cluster-name", "", "Name of the cluster. With --cluster-tag-key, every AWS resource the controller creates gets the tag <cluster-tag-key>=<cluster-name>.")
	fs.StringVar(&s.ClusterTagKey, "cluster-tag-key", "", "Key of the cluster tag, whose value is --cluster-name. Set it together with --cluster-name. Default empty: no cluster tag.")
	fs.StringVar(&s.ClusterTagIsolation, "cluster-tag-isolation", string(driver.ClusterTagIsolationDisable), "How the controller uses the cluster tag. Set it to match the IAM policy attached to the controller role. "+
		"disable: default behavior, plus the cluster tag on new resources; adds the driver tag to the root volumes of existing file systems the controller created, at startup and every hour. "+
		"backfill: requires --cluster-name and --cluster-tag-key; tags existing resources the controller created for this cluster, at startup and every hour; does not filter discovery. "+
		"enforce: requires --cluster-name and --cluster-tag-key; ignores resources that lack this cluster's tag value; does not sweep.")
	fs.StringToStringVar(&s.ExtraTags, "extra-tags", nil, "Tags (k1=v1,k2=v2) added to every AWS resource the controller creates. Not used to find, filter or isolate resources, and not the cluster tag. StorageClass and VolumeSnapshotClass Tags override them. Neither can set the cluster tag key or the driver tag, and extra tags cannot set *OnDeletion keys.")
}

// ClusterTagOptions returns the validated cluster tag options.
func (s *ControllerOptions) ClusterTagOptions() (*driver.ClusterTagOptions, error) {
	o := &driver.ClusterTagOptions{
		ClusterName: s.ClusterName,
		TagKey:      s.ClusterTagKey,
		Isolation:   driver.ClusterTagIsolation(s.ClusterTagIsolation),
	}
	if len(s.ExtraTags) > 0 {
		o.ExtraTags = s.ExtraTags
	}
	if err := o.Validate(); err != nil {
		return nil, err
	}
	return o, nil
}
