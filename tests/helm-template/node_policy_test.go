/*
Copyright 2026 The Kubernetes Authors.

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

// Package helmtemplate tests Helm chart rendering without a live cluster.
package helmtemplate

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func chartPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "charts", "aws-fsx-openzfs-csi-driver")
}

// helmBin returns bin/helm from `make bin/helm`, or helm from PATH.
func helmBin() string {
	_, file, _, _ := runtime.Caller(0)
	bin := filepath.Join(filepath.Dir(file), "..", "..", "bin", "helm")
	if _, err := os.Stat(bin); err == nil {
		return bin
	}
	return "helm"
}

func render(t *testing.T, args ...string) []*unstructured.Unstructured {
	t.Helper()
	cmd := exec.Command(helmBin(), append([]string{"template", "fsx-openzfs", chartPath()}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("helm template: %v\n%s", err, stderr.String())
	}
	var objs []*unstructured.Unstructured
	decoder := yaml.NewYAMLOrJSONDecoder(&stdout, 4096)
	for {
		obj := &unstructured.Unstructured{}
		if err := decoder.Decode(&obj.Object); err != nil {
			if errors.Is(err, io.EOF) {
				return objs
			}
			t.Fatalf("decode: %v", err)
		}
		if len(obj.Object) > 0 {
			objs = append(objs, obj)
		}
	}
}

func find(objs []*unstructured.Unstructured, kind, name string) *unstructured.Unstructured {
	for _, o := range objs {
		if o.GetKind() == kind && o.GetName() == name {
			return o
		}
	}
	return nil
}

// TestNodeAdmissionPolicy checks when the chart renders the ValidatingAdmissionPolicy that limits the node service
// account to removing the agent-not-ready taint from its own Node, and which service account it matches.
func TestNodeAdmissionPolicy(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		want     bool
		wantUser string
	}{
		{name: "Kubernetes 1.30", args: []string{"--kube-version", "1.30.0", "--namespace", "kube-system"}, want: true,
			wantUser: "system:serviceaccount:kube-system:fsx-openzfs-csi-node-sa"},
		{name: "Kubernetes 1.29", args: []string{"--kube-version", "1.29.0"}, want: false},
		{name: "disabled", args: []string{"--kube-version", "1.30.0", "--set", "node.validatingAdmissionPolicy.enabled=false"}, want: false},
		{name: "existing service account", args: []string{"--kube-version", "1.30.0", "--namespace", "storage",
			"--set", "node.serviceAccount.create=false", "--set", "node.serviceAccount.name=my-node-sa"}, want: true,
			wantUser: "system:serviceaccount:storage:my-node-sa"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			objs := render(t, tc.args...)
			policy := find(objs, "ValidatingAdmissionPolicy", "fsx-openzfs-csi-node")
			binding := find(objs, "ValidatingAdmissionPolicyBinding", "fsx-openzfs-csi-node")
			if (policy != nil) != tc.want || (binding != nil) != tc.want {
				t.Fatalf("policy rendered: %v, binding rendered: %v, want %v", policy != nil, binding != nil, tc.want)
			}
			if !tc.want {
				return
			}
			if got, _, _ := unstructured.NestedString(binding.Object, "spec", "policyName"); got != "fsx-openzfs-csi-node" {
				t.Errorf("binding policyName = %q", got)
			}
			if got, _, _ := unstructured.NestedStringSlice(binding.Object, "spec", "validationActions"); len(got) != 1 || got[0] != "Deny" {
				t.Errorf("binding validationActions = %v", got)
			}
			if got, _, _ := unstructured.NestedString(policy.Object, "spec", "failurePolicy"); got != "Fail" {
				t.Errorf("failurePolicy = %q", got)
			}
			conds, _, _ := unstructured.NestedSlice(policy.Object, "spec", "matchConditions")
			want := `request.userInfo.username == "` + tc.wantUser + `"`
			if len(conds) != 1 || conds[0].(map[string]interface{})["expression"] != want {
				t.Errorf("matchConditions = %v, want one with expression %s", conds, want)
			}
		})
	}
}
