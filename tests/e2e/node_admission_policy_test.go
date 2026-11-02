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

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authenticationv1 "k8s.io/api/authentication/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/version"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	agentNotReadyTaintKey = "fsx.openzfs.csi.aws.com/agent-not-ready"
	nodeNamespace         = "kube-system"
	nodeServiceAccount    = "fsx-openzfs-csi-node-sa"
	nodePodSelector       = "app.kubernetes.io/name=fsx-openzfs-csi-node"
	nodeNameExtraKey      = "authentication.kubernetes.io/node-name"
)

// The chart's ValidatingAdmissionPolicy limits the node service account to removing the agent-not-ready taint from
// its own Node. The spec uses a real token of that service account, bound to a running node pod, like the node pod's
// own token.
var _ = Describe("FSx for OpenZFS Node ValidatingAdmissionPolicy", func() {
	f := framework.NewDefaultFramework("fsx-openzfs-node-vap")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("limits the node service account to removing the agent-not-ready taint from its own Node", func(ctx context.Context) {
		cs := f.ClientSet
		serverVersion, err := cs.Discovery().ServerVersion()
		framework.ExpectNoError(err)
		if version.MustParseGeneric(serverVersion.GitVersion).LessThan(version.MajorMinor(1, 30)) {
			Skip("the policy is installed on Kubernetes 1.30 and later")
		}

		pod := runningNodePod(ctx, cs, "")
		ownNode := pod.Spec.NodeName

		expiration := int64(600)
		token, err := cs.CoreV1().ServiceAccounts(nodeNamespace).CreateToken(ctx, nodeServiceAccount, &authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				ExpirationSeconds: &expiration,
				BoundObjectRef:    &authenticationv1.BoundObjectReference{Kind: "Pod", APIVersion: "v1", Name: pod.Name, UID: pod.UID},
			},
		}, metav1.CreateOptions{})
		framework.ExpectNoError(err)
		nodeClient := clientWithToken(f.ClientConfig(), token.Status.Token)

		// The API server reads the node from the token, as the policy expects.
		review, err := nodeClient.AuthenticationV1().SelfSubjectReviews().Create(ctx, &authenticationv1.SelfSubjectReview{}, metav1.CreateOptions{})
		framework.ExpectNoError(err)
		Expect(review.Status.UserInfo.Username).To(Equal(fmt.Sprintf("system:serviceaccount:%s:%s", nodeNamespace, nodeServiceAccount)))
		Expect(review.Status.UserInfo.Extra[nodeNameExtraKey]).To(ConsistOf(authenticationv1.ExtraValue{ownNode}))

		// Another node. The policy denies every change below, so it is never changed.
		nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		framework.ExpectNoError(err)
		otherNode := ""
		for _, n := range nodes.Items {
			if n.Name != ownNode {
				otherNode = n.Name
				break
			}
		}
		Expect(otherNode).NotTo(BeEmpty(), "the cluster needs a second node")

		// Taint both nodes briefly, with NoSchedule so running pods stay. The node pods tolerate every taint.
		otherTaint := v1.Taint{Key: "fsx-openzfs-node-vap", Effect: v1.TaintEffectNoSchedule}
		for _, n := range []string{ownNode, otherNode} {
			addTaint(ctx, cs, n, v1.Taint{Key: agentNotReadyTaintKey, Effect: v1.TaintEffectNoSchedule})
			DeferCleanup(func(ctx context.Context) {
				for _, key := range []string{agentNotReadyTaintKey, otherTaint.Key} {
					if hasTaint(ctx, cs, n, key) {
						framework.ExpectNoError(removeTaint(ctx, cs, n, key))
					}
				}
			})
		}
		addTaint(ctx, cs, ownNode, otherTaint)

		By("denying the agent-not-ready taint removal on another node")
		err = removeTaint(ctx, nodeClient, otherNode, agentNotReadyTaintKey)
		Expect(err).To(MatchError(ContainSubstring("may only update its own Node object, not " + otherNode)))

		By("denying another taint on its own node")
		err = patchTaints(ctx, nodeClient, ownNode, func(taints []v1.Taint) []v1.Taint {
			return append(taints, v1.Taint{Key: "fsx-openzfs-node-vap-added", Effect: v1.TaintEffectNoSchedule})
		})
		Expect(err).To(MatchError(ContainSubstring("may only remove the " + agentNotReadyTaintKey + " taint")))

		By("denying the removal of another taint from its own node")
		err = removeTaint(ctx, nodeClient, ownNode, otherTaint.Key)
		Expect(err).To(MatchError(ContainSubstring("may only remove the " + agentNotReadyTaintKey + " taint")))

		for name, patch := range map[string]string{
			"a label":       `{"metadata":{"labels":{"fsx-openzfs-node-vap":"denied"}}}`,
			"an annotation": `{"metadata":{"annotations":{"fsx-openzfs-node-vap":"denied"}}}`,
			"unschedulable": `{"spec":{"unschedulable":true}}`,
		} {
			By("denying " + name + " on its own node")
			_, err = nodeClient.CoreV1().Nodes().Patch(ctx, ownNode, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
			Expect(err).To(MatchError(ContainSubstring("may only change the taints of its own Node object")))
		}

		By("denying a status update of its own node")
		_, err = nodeClient.CoreV1().Nodes().Patch(ctx, ownNode, types.MergePatchType,
			[]byte(`{"status":{"capacity":{"pods":"1"}}}`), metav1.PatchOptions{}, "status")
		Expect(apierrors.IsForbidden(err) || apierrors.IsInvalid(err)).To(BeTrue(), "status update: %v", err)

		By("denying the deletion of a node")
		err = nodeClient.CoreV1().Nodes().Delete(ctx, otherNode, metav1.DeleteOptions{DryRun: []string{metav1.DryRunAll}})
		Expect(apierrors.IsForbidden(err) || apierrors.IsInvalid(err)).To(BeTrue(), "delete: %v", err)

		By("allowing the agent-not-ready taint removal on its own node")
		framework.ExpectNoError(removeTaint(ctx, nodeClient, ownNode, agentNotReadyTaintKey))
		Expect(hasTaint(ctx, cs, ownNode, agentNotReadyTaintKey)).To(BeFalse())
		Expect(hasTaint(ctx, cs, ownNode, otherTaint.Key)).To(BeTrue())
		Expect(hasTaint(ctx, cs, otherNode, agentNotReadyTaintKey)).To(BeTrue())

		By("letting a restarted node pod remove the agent-not-ready taint from its node at startup")
		addTaint(ctx, cs, ownNode, v1.Taint{Key: agentNotReadyTaintKey, Effect: v1.TaintEffectNoSchedule})
		framework.ExpectNoError(cs.CoreV1().Pods(nodeNamespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}))
		Eventually(func() bool { return hasTaint(ctx, cs, ownNode, agentNotReadyTaintKey) }).
			WithTimeout(5 * time.Minute).WithPolling(5 * time.Second).Should(BeFalse())
		restarted := runningNodePod(ctx, cs, ownNode)
		Expect(restarted.UID).NotTo(Equal(pod.UID))
	})
})

// runningNodePod returns a running, ready node pod, on the given node if it is not empty.
func runningNodePod(ctx context.Context, cs clientset.Interface, node string) *v1.Pod {
	var pod *v1.Pod
	Eventually(func() *v1.Pod {
		pods, err := cs.CoreV1().Pods(nodeNamespace).List(ctx, metav1.ListOptions{LabelSelector: nodePodSelector})
		framework.ExpectNoError(err)
		pod = nil
		for i := range pods.Items {
			p := &pods.Items[i]
			if p.Status.Phase != v1.PodRunning || p.DeletionTimestamp != nil || p.Spec.NodeName == "" || (node != "" && p.Spec.NodeName != node) {
				continue
			}
			for _, c := range p.Status.Conditions {
				if c.Type == v1.PodReady && c.Status == v1.ConditionTrue {
					pod = p
					return pod
				}
			}
		}
		return nil
	}).WithTimeout(5*time.Minute).WithPolling(5*time.Second).ShouldNot(BeNil(), "no ready node pod (%s) on node %q", nodePodSelector, node)
	return pod
}

// clientWithToken returns a client for the same API server that authenticates only with the given bearer token.
func clientWithToken(config *rest.Config, token string) clientset.Interface {
	c := rest.AnonymousClientConfig(config)
	c.BearerToken = token
	client, err := clientset.NewForConfig(c)
	framework.ExpectNoError(err)
	return client
}

func hasTaint(ctx context.Context, cs clientset.Interface, node, key string) bool {
	n, err := cs.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
	framework.ExpectNoError(err)
	for _, t := range n.Spec.Taints {
		if t.Key == key {
			return true
		}
	}
	return false
}

func addTaint(ctx context.Context, cs clientset.Interface, node string, taint v1.Taint) {
	framework.ExpectNoError(patchTaints(ctx, cs, node, func(taints []v1.Taint) []v1.Taint {
		return append(taints, taint)
	}))
}

func removeTaint(ctx context.Context, cs clientset.Interface, node, key string) error {
	return patchTaints(ctx, cs, node, func(taints []v1.Taint) []v1.Taint {
		var keep []v1.Taint
		for _, t := range taints {
			if t.Key != key {
				keep = append(keep, t)
			}
		}
		return keep
	})
}

// patchTaints changes a node's taints with the same JSON patch the node pod sends in removeNotReadyTaint: test the
// current taints, then replace them. A node without taints gets an add instead, since the test needs the path. The
// patch carries the resourceVersion it read, so it does not overwrite concurrent taint changes, and retries on conflict.
func patchTaints(ctx context.Context, cs clientset.Interface, node string, change func([]v1.Taint) []v1.Taint) error {
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		var n *v1.Node
		n, err = cs.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
		if err != nil {
			return err
		}
		ops := []map[string]interface{}{
			{"op": "add", "path": "/metadata/resourceVersion", "value": n.ResourceVersion},
			{"op": "add", "path": "/spec/taints", "value": change(n.Spec.Taints)},
		}
		if len(n.Spec.Taints) > 0 {
			ops = []map[string]interface{}{
				{"op": "add", "path": "/metadata/resourceVersion", "value": n.ResourceVersion},
				{"op": "test", "path": "/spec/taints", "value": n.Spec.Taints},
				{"op": "replace", "path": "/spec/taints", "value": change(n.Spec.Taints)},
			}
		}
		patch, merr := json.Marshal(ops)
		if merr != nil {
			return merr
		}
		_, err = cs.CoreV1().Nodes().Patch(ctx, node, types.JSONPatchType, patch, metav1.PatchOptions{})
		// Retry when the node or its taints changed in between, for example by the node lifecycle controller.
		if err == nil || !(apierrors.IsConflict(err) || strings.Contains(err.Error(), "testing value")) {
			return err
		}
	}
	return err
}
