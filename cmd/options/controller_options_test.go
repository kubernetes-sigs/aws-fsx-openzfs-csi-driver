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
	"reflect"
	"strings"
	"testing"

	flag "github.com/spf13/pflag"

	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/driver"
)

func TestControllerOptions(t *testing.T) {
	testCases := []struct {
		name    string
		args    []string
		want    driver.ClusterTagOptions
		wantErr string
	}{
		{
			name: "defaults",
			args: nil,
			want: driver.ClusterTagOptions{Isolation: driver.ClusterTagIsolationDisable},
		},
		{
			name: "cluster tag in disable",
			args: []string{"--cluster-name=my-cluster", "--cluster-tag-key=example.com/cluster"},
			want: driver.ClusterTagOptions{ClusterName: "my-cluster", TagKey: "example.com/cluster", Isolation: driver.ClusterTagIsolationDisable},
		},
		{
			name: "enforce with extra tags",
			args: []string{"--cluster-name=my-cluster", "--cluster-tag-key=example.com/cluster", "--cluster-tag-isolation=enforce", "--extra-tags=team=a,env=prod"},
			want: driver.ClusterTagOptions{ClusterName: "my-cluster", TagKey: "example.com/cluster", Isolation: driver.ClusterTagIsolationEnforce, ExtraTags: map[string]string{"team": "a", "env": "prod"}},
		},
		{
			name: "cluster name without key is disable",
			args: []string{"--cluster-name=my-cluster"},
			want: driver.ClusterTagOptions{ClusterName: "my-cluster", Isolation: driver.ClusterTagIsolationDisable},
		},
		{
			name:    "enforce with whitespace cluster name fails",
			args:    []string{"--cluster-name= ", "--cluster-tag-key=example.com/cluster", "--cluster-tag-isolation=enforce"},
			wantErr: "--cluster-tag-isolation=enforce requires --cluster-name and --cluster-tag-key",
		},
		{
			name:    "backfill without cluster name fails",
			args:    []string{"--cluster-tag-isolation=backfill"},
			wantErr: "--cluster-tag-isolation=backfill requires --cluster-name and --cluster-tag-key",
		},
		{
			name:    "unknown state fails",
			args:    []string{"--cluster-tag-isolation=strict"},
			wantErr: `invalid --cluster-tag-isolation "strict": must be "disable", "backfill" or "enforce"`,
		},
		{
			name:    "false fails",
			args:    []string{"--cluster-tag-isolation=false"},
			wantErr: `invalid --cluster-tag-isolation "false": must be "disable", "backfill" or "enforce"`,
		},
		{
			name:    "off fails",
			args:    []string{"--cluster-tag-isolation=off"},
			wantErr: `invalid --cluster-tag-isolation "off": must be "disable", "backfill" or "enforce"`,
		},
		{
			name:    "extra tag with the cluster tag key fails",
			args:    []string{"--cluster-name=my-cluster", "--cluster-tag-key=example.com/cluster", "--extra-tags=example.com/cluster=other"},
			wantErr: "must not contain the cluster tag key",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			o := &ControllerOptions{}
			o.AddFlags(fs)
			if err := fs.Parse(tc.args); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			got, err := o.ClusterTagOptions()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("ClusterTagOptions() error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ClusterTagOptions() error: %v", err)
			}
			if !reflect.DeepEqual(*got, tc.want) {
				t.Fatalf("ClusterTagOptions() = %+v, want %+v", *got, tc.want)
			}
		})
	}
}
