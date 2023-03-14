/*
Copyright ApeCloud, Inc.

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

func checkCompletedPhase(t *testing.T, phase appsv1alpha1.Phase) {
	isComplete := IsCompleted(phase)
	if !isComplete {
		t.Errorf("%s status is the completed status", phase)
	}
}

func TestIsCompleted(t *testing.T) {
	checkCompletedPhase(t, appsv1alpha1.FailedPhase)
	checkCompletedPhase(t, appsv1alpha1.RunningPhase)
	checkCompletedPhase(t, appsv1alpha1.AbnormalPhase)
}

func TestIsFailedOrAbnormal(t *testing.T) {
	if !IsFailedOrAbnormal(appsv1alpha1.AbnormalPhase) {
		t.Error("isAbnormal should be true")
	}
}

func TestIsProbeTimeout(t *testing.T) {
	podsReadyTime := &metav1.Time{Time: time.Now().Add(-10 * time.Minute)}
	compDef := &appsv1alpha1.ClusterComponentDefinition{
		Probes: &appsv1alpha1.ClusterDefinitionProbes{
			RoleChangedProbe:               &appsv1alpha1.ClusterDefinitionProbe{},
			RoleProbeTimeoutAfterPodsReady: appsv1alpha1.DefaultRoleProbeTimeoutAfterPodsReady,
		},
	}
	if !IsProbeTimeout(compDef, podsReadyTime) {
		t.Error("probe timed out should be true")
	}
}

func TestGetComponentPhase(t *testing.T) {
	var (
		isFailed   = true
		isAbnormal = true
	)
	status := GetComponentPhase(isFailed, isAbnormal)
	if status != appsv1alpha1.FailedPhase {
		t.Error("function GetComponentPhase should return Failed")
	}
	isFailed = false
	status = GetComponentPhase(isFailed, isAbnormal)
	if status != appsv1alpha1.AbnormalPhase {
		t.Error("function GetComponentPhase should return Abnormal")
	}
	isAbnormal = false
	status = GetComponentPhase(isFailed, isAbnormal)
	if status != "" {
		t.Error(`function GetComponentPhase should return ""`)
	}
}

func TestGetPhaseWithNoAvailableReplicas(t *testing.T) {
	status := GetPhaseWithNoAvailableReplicas(int32(0))
	if status != "" {
		t.Error(`function GetComponentPhase should return ""`)
	}
	status = GetPhaseWithNoAvailableReplicas(int32(2))
	if status != appsv1alpha1.FailedPhase {
		t.Error(`function GetComponentPhase should return "Failed"`)
	}
}

func TestAvailableReplicasAreConsistent(t *testing.T) {
	isConsistent := AvailableReplicasAreConsistent(int32(1), int32(1), int32(1))
	if !isConsistent {
		t.Error(`function GetComponentPhase should return "true"`)
	}
	isConsistent = AvailableReplicasAreConsistent(int32(1), int32(2), int32(1))
	if isConsistent {
		t.Error(`function GetComponentPhase should return "false"`)
	}
}

func TestGetCompPhaseByConditions(t *testing.T) {
	existLatestRevisionFailedPod := true
	primaryReplicaIsReady := true
	phase := GetCompPhaseByConditions(existLatestRevisionFailedPod, primaryReplicaIsReady, int32(1), int32(1), int32(1))
	if phase != "" {
		t.Error(`function GetComponentPhase should return ""`)
	}
	phase = GetCompPhaseByConditions(existLatestRevisionFailedPod, primaryReplicaIsReady, int32(2), int32(1), int32(1))
	if phase != appsv1alpha1.AbnormalPhase {
		t.Error(`function GetComponentPhase should return "Abnormal"`)
	}
	primaryReplicaIsReady = false
	phase = GetCompPhaseByConditions(existLatestRevisionFailedPod, primaryReplicaIsReady, int32(2), int32(1), int32(1))
	if phase != appsv1alpha1.FailedPhase {
		t.Error(`function GetComponentPhase should return "Failed"`)
	}
	existLatestRevisionFailedPod = false
	phase = GetCompPhaseByConditions(existLatestRevisionFailedPod, primaryReplicaIsReady, int32(2), int32(1), int32(1))
	if phase != "" {
		t.Error(`function GetComponentPhase should return ""`)
	}
}

var _ = Describe("Consensus Component", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		clusterDefName     = "mysql-clusterdef-" + randomStr
		clusterVersionName = "mysql-clusterversion-" + randomStr
		clusterName        = "mysql-" + randomStr
	)

	const (
		consensusCompDefRef = "consensus"
		consensusCompName   = "consensus"
		statelessCompName   = "stateless"
	)

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testapps.ClearResources(&testCtx, generics.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("Consensus Component test", func() {
		It("Consensus Component test", func() {
			By(" init cluster, statefulSet, pods")
			_, _, cluster := testapps.InitClusterWithHybridComps(testCtx, clusterDefName,
				clusterVersionName, clusterName, statelessCompName, "stateful", consensusCompName)
			sts := testapps.MockConsensusComponentStatefulSet(testCtx, clusterName, consensusCompName)
			testapps.MockStatelessComponentDeploy(testCtx, clusterName, statelessCompName)
			_ = testapps.MockConsensusComponentPods(testCtx, sts, clusterName, consensusCompName)

			By("test GetComponentDefByCluster function")
			componentDef, _ := GetComponentDefByCluster(ctx, k8sClient, *cluster, consensusCompDefRef)
			Expect(componentDef != nil).Should(BeTrue())

			By("test GetClusterByObject function")
			newCluster, _ := GetClusterByObject(ctx, k8sClient, sts)
			Expect(newCluster != nil).Should(BeTrue())

			By("test GetComponentPodList function")
			Eventually(func() bool {
				podList, _ := GetComponentPodList(ctx, k8sClient, *cluster, consensusCompName)
				return len(podList.Items) > 0
			}).Should(BeTrue())

			By("test GetObjectListByComponentName function")
			stsList := &appsv1.StatefulSetList{}
			_ = GetObjectListByComponentName(ctx, k8sClient, *cluster, stsList, consensusCompName)
			Expect(len(stsList.Items) > 0).Should(BeTrue())

			By("test GetComponentStatusMessageKey function")
			Expect(GetComponentStatusMessageKey("Pod", "mysql-01")).To(Equal("Pod/mysql-01"))

			By("test GetComponentStsMinReadySeconds")
			minReadySeconds, _ := GetComponentWorkloadMinReadySeconds(ctx, k8sClient, *cluster,
				appsv1alpha1.Stateless, statelessCompName)
			Expect(minReadySeconds).To(Equal(int32(10)))
			minReadySeconds, _ = GetComponentWorkloadMinReadySeconds(ctx, k8sClient, *cluster,
				appsv1alpha1.Consensus, statelessCompName)
			Expect(minReadySeconds).To(Equal(int32(0)))

			By("test GetCompRelatedObjectList function")
			stsList = &appsv1.StatefulSetList{}
			podList, _ := GetCompRelatedObjectList(ctx, k8sClient, *cluster, consensusCompName, stsList)
			Expect(len(stsList.Items) > 0 && len(podList.Items) > 0).Should(BeTrue())

			By("test GetComponentPhaseWhenPodsNotReady function")
			consensusComp := cluster.GetComponentByName(consensusCompName)
			checkExistFailedPodOfLatestRevision := func(pod *corev1.Pod, workload metav1.Object) bool {
				sts := workload.(*appsv1.StatefulSet)
				return !intctrlutil.PodIsReady(pod) && intctrlutil.PodIsControlledByLatestRevision(pod, sts)
			}
			// component phase should be Failed when available replicas is 0
			phase := GetComponentPhaseWhenPodsNotReady(podList, sts, consensusComp.Replicas,
				sts.Status.AvailableReplicas, checkExistFailedPodOfLatestRevision)
			Expect(phase).Should(Equal(appsv1alpha1.FailedPhase))

			// mock available replicas to component replicas
			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				testk8s.MockStatefulSetReady(sts)
			})).Should(Succeed())
			phase = GetComponentPhaseWhenPodsNotReady(podList, sts, consensusComp.Replicas,
				sts.Status.AvailableReplicas, checkExistFailedPodOfLatestRevision)
			Expect(len(phase) == 0).Should(BeTrue())

			// mock component is abnormal
			pod := &podList.Items[0]
			Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
				pod.Status.Conditions = nil
			})).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				sts.Status.AvailableReplicas = *sts.Spec.Replicas - 1
			})).Should(Succeed())
			phase = GetComponentPhaseWhenPodsNotReady(podList, sts, consensusComp.Replicas,
				sts.Status.AvailableReplicas, checkExistFailedPodOfLatestRevision)
			Expect(phase).Should(Equal(appsv1alpha1.AbnormalPhase))

		})
	})
})
