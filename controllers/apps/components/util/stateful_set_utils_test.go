/*
Copyright ApeCloud, Inc.
Copyright 2016 The Kubernetes Authors.

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

package util

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	apps "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

func TestGetParentNameAndOrdinal(t *testing.T) {
	set := testk8s.NewFakeStatefulSet("foo", 3)
	pod := testk8s.NewFakeStatefulSetPod(set, 1)
	if parent, ordinal := intctrlutil.GetParentNameAndOrdinal(pod); parent != set.Name {
		t.Errorf("Extracted the wrong parent name expected %s found %s", set.Name, parent)
	} else if ordinal != 1 {
		t.Errorf("Extracted the wrong ordinal expected %d found %d", 1, ordinal)
	}
	pod.Name = "1-bar"
	if parent, ordinal := intctrlutil.GetParentNameAndOrdinal(pod); parent != "" {
		t.Error("Expected empty string for non-member Pod parent")
	} else if ordinal != -1 {
		t.Error("Expected -1 for non member Pod ordinal")
	}
}

func TestIsMemberOf(t *testing.T) {
	set := testk8s.NewFakeStatefulSet("foo", 3)
	set2 := testk8s.NewFakeStatefulSet("bar", 3)
	set2.Name = "foo2"
	pod := testk8s.NewFakeStatefulSetPod(set, 1)
	if !IsMemberOf(set, pod) {
		t.Error("isMemberOf returned false negative")
	}
	if IsMemberOf(set2, pod) {
		t.Error("isMemberOf returned false positive")
	}
}

func TestGetPodRevision(t *testing.T) {
	set := testk8s.NewFakeStatefulSet("foo", 3)
	pod := testk8s.NewFakeStatefulSetPod(set, 1)
	if GetPodRevision(pod) != "" {
		t.Errorf("revision should be empty")
	}

	pod.Labels = make(map[string]string, 0)
	pod.Labels[apps.StatefulSetRevisionLabel] = "bar"

	if GetPodRevision(pod) != "bar" {
		t.Errorf("revision not matched")
	}
}

func TestStatefulSetPodsAreReady(t *testing.T) {
	sts := testk8s.NewFakeStatefulSet("test", 3)
	testk8s.MockStatefulSetReady(sts)
	ready := StatefulSetPodsAreReady(sts, *sts.Spec.Replicas)
	if !ready {
		t.Errorf("StatefulSet pods should be ready")
	}
	covertSts := ConvertToStatefulSet(sts)
	if covertSts == nil {
		t.Errorf("Covert to statefulSet should be succeed")
	}
	covertSts = ConvertToStatefulSet(&apps.Deployment{})
	if covertSts != nil {
		t.Errorf("Covert to statefulSet should be failed")
	}
	covertSts = ConvertToStatefulSet(nil)
	if covertSts != nil {
		t.Errorf("Covert to statefulSet should be failed")
	}
}

func TestSStatefulSetOfComponentIsReady(t *testing.T) {
	sts := testk8s.NewFakeStatefulSet("test", 3)
	testk8s.MockStatefulSetReady(sts)
	ready := StatefulSetOfComponentIsReady(sts, true, nil)
	if !ready {
		t.Errorf("StatefulSet should be ready")
	}
	ready = StatefulSetOfComponentIsReady(sts, false, nil)
	if ready {
		t.Errorf("StatefulSet should not be ready")
	}
}

var _ = Describe("StatefulSet utils test", func() {
	var (
		clusterName = "test-replication-cluster"
		stsName     = "test-sts"
		role        = "Primary"
	)
	cleanAll := func() {
		By("Cleaning resources")
		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)
		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)
	AfterEach(cleanAll)

	When("Updating a StatefulSet with `OnDelete` UpdateStrategy", func() {
		It("will not update pods of the StatefulSet util the pods have been manually deleted", func() {
			By("Creating a StatefulSet")
			sts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, stsName, clusterName, testapps.DefaultRedisCompName).
				AddContainer(corev1.Container{Name: testapps.DefaultRedisContainerName, Image: testapps.DefaultRedisImageName}).
				AddLabels(intctrlutil.AppInstanceLabelKey, clusterName,
					intctrlutil.AppComponentLabelKey, testapps.DefaultRedisCompName,
					intctrlutil.AppManagedByLabelKey, testapps.KubeBlocks,
					intctrlutil.RoleLabelKey, role).
				SetReplicas(1).
				Create(&testCtx).GetObject()

			By("Creating pods by the StatefulSet")
			testapps.MockReplicationComponentPods(testCtx, sts, clusterName, testapps.DefaultRedisCompName, role)
			Expect(IsStsAndPodsRevisionConsistent(testCtx.Ctx, k8sClient, sts)).Should(BeTrue())

			By("Updating the StatefulSet's UpdateRevision")
			sts.Status.UpdateRevision = "new-mock-revision"
			testk8s.PatchStatefulSetStatus(&testCtx, sts.Name, sts.Status)
			podList, err := GetPodListByStatefulSet(ctx, k8sClient, sts)
			Expect(err).To(Succeed())
			Expect(len(podList)).To(Equal(1))

			By("Deleting the pods of StatefulSet")
			Expect(DeleteStsPods(testCtx.Ctx, k8sClient, sts)).Should(Succeed())
			podList, err = GetPodListByStatefulSet(ctx, k8sClient, sts)
			Expect(err).To(Succeed())
			Expect(len(podList)).To(Equal(0))

			By("Creating new pods by StatefulSet with new UpdateRevision")
			testapps.MockReplicationComponentPods(testCtx, sts, clusterName, testapps.DefaultRedisCompName, role)
			Expect(IsStsAndPodsRevisionConsistent(testCtx.Ctx, k8sClient, sts)).Should(BeTrue())
		})
	})
})
