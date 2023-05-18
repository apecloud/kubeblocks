/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

func TestIsFailedOrAbnormal(t *testing.T) {
	if !IsFailedOrAbnormal(appsv1alpha1.AbnormalClusterCompPhase) {
		t.Error("isAbnormal should be true")
	}
}

func TestIsProbeTimeout(t *testing.T) {
	podsReadyTime := &metav1.Time{Time: time.Now().Add(-10 * time.Minute)}
	compDef := &appsv1alpha1.ClusterComponentDefinition{
		Probes: &appsv1alpha1.ClusterDefinitionProbes{
			RoleProbe:                      &appsv1alpha1.ClusterDefinitionProbe{},
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
	if status != appsv1alpha1.FailedClusterCompPhase {
		t.Error("function GetComponentPhase should return Failed")
	}
	isFailed = false
	status = GetComponentPhase(isFailed, isAbnormal)
	if status != appsv1alpha1.AbnormalClusterCompPhase {
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
	if status != appsv1alpha1.FailedClusterCompPhase {
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
	if phase != appsv1alpha1.AbnormalClusterCompPhase {
		t.Error(`function GetComponentPhase should return "Abnormal"`)
	}
	primaryReplicaIsReady = false
	phase = GetCompPhaseByConditions(existLatestRevisionFailedPod, primaryReplicaIsReady, int32(2), int32(1), int32(1))
	if phase != appsv1alpha1.FailedClusterCompPhase {
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
		// testapps.ClearResources(&testCtx, generics.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("Consensus Component test", func() {
		It("Consensus Component test", func() {
			By(" init cluster, statefulSet, pods")
			_, _, cluster := testapps.InitClusterWithHybridComps(&testCtx, clusterDefName,
				clusterVersionName, clusterName, statelessCompName, "stateful", consensusCompName)
			sts := testapps.MockConsensusComponentStatefulSet(&testCtx, clusterName, consensusCompName)
			testapps.MockStatelessComponentDeploy(&testCtx, clusterName, statelessCompName)
			_ = testapps.MockConsensusComponentPods(&testCtx, sts, clusterName, consensusCompName)

			By("test GetComponentDefByCluster function")
			componentDef, _ := GetComponentDefByCluster(ctx, k8sClient, *cluster, consensusCompDefRef)
			Expect(componentDef != nil).Should(BeTrue())

			By("test GetClusterByObject function")
			newCluster, _ := GetClusterByObject(ctx, k8sClient, sts)
			Expect(newCluster != nil).Should(BeTrue())

			By("test consensusSet InitClusterComponentStatusIfNeed function")
			err := InitClusterComponentStatusIfNeed(cluster, consensusCompName, *componentDef)
			Expect(err).Should(Succeed())
			Expect(cluster.Status.Components[consensusCompName].ConsensusSetStatus).ShouldNot(BeNil())
			Expect(cluster.Status.Components[consensusCompName].ConsensusSetStatus.Leader.Pod).Should(Equal(ComponentStatusDefaultPodName))

			By("test ReplicationSet InitClusterComponentStatusIfNeed function")
			componentDef.WorkloadType = appsv1alpha1.Replication
			err = InitClusterComponentStatusIfNeed(cluster, consensusCompName, *componentDef)
			Expect(err).Should(Succeed())
			Expect(cluster.Status.Components[consensusCompName].ReplicationSetStatus).ShouldNot(BeNil())
			Expect(cluster.Status.Components[consensusCompName].ReplicationSetStatus.Primary.Pod).Should(Equal(ComponentStatusDefaultPodName))

			By("test GetObjectListByComponentName function")
			stsList := &appsv1.StatefulSetList{}
			_ = GetObjectListByComponentName(ctx, k8sClient, *cluster, stsList, consensusCompName)
			Expect(len(stsList.Items) > 0).Should(BeTrue())

			By("test GetObjectListByCustomLabels function")
			stsList = &appsv1.StatefulSetList{}
			matchLabel := GetComponentMatchLabels(cluster.Name, consensusCompName)
			_ = GetObjectListByCustomLabels(ctx, k8sClient, *cluster, stsList, client.MatchingLabels(matchLabel))
			Expect(len(stsList.Items) > 0).Should(BeTrue())

			By("test GetClusterComponentSpecByName function")
			clusterComp := GetClusterComponentSpecByName(*cluster, consensusCompName)
			Expect(clusterComp).ShouldNot(BeNil())

			By("test ComponentRuntimeReqArgsCheck function")
			err = ComponentRuntimeReqArgsCheck(k8sClient, cluster, clusterComp)
			Expect(err).Should(Succeed())
			By("test ComponentRuntimeReqArgsCheck function when cluster nil")
			err = ComponentRuntimeReqArgsCheck(k8sClient, nil, clusterComp)
			Expect(err).ShouldNot(Succeed())
			By("test ComponentRuntimeReqArgsCheck function when clusterComp nil")
			err = ComponentRuntimeReqArgsCheck(k8sClient, cluster, nil)
			Expect(err).ShouldNot(Succeed())

			By("test UpdateObjLabel function")
			stsObj := stsList.Items[0]
			err = UpdateObjLabel(ctx, k8sClient, stsObj, "test", "test")
			Expect(err).Should(Succeed())

			By("test PatchGVRCustomLabels of clusterDefinition")
			resource := &appsv1alpha1.GVKResource{
				Selector: GetComponentMatchLabels(cluster.Name, consensusCompName),
			}
			customLabelSpec := &appsv1alpha1.CustomLabelSpec{
				Key:       "custom-label-key",
				Value:     "$(KB_CLUSTER_NAME)-$(KB_COMP_NAME)",
				Resources: []appsv1alpha1.GVKResource{*resource},
			}
			By("test statefulSet resource PatchGVRCustomLabels")
			resource.GVK = "apps/v1/StatefulSet"
			err = PatchGVRCustomLabels(ctx, k8sClient, cluster, *resource, consensusCompName, customLabelSpec.Key, customLabelSpec.Value)
			Expect(err).Should(Succeed())
			By("test Pod resource PatchGVRCustomLabels")
			resource.GVK = "v1/Pod"
			err = PatchGVRCustomLabels(ctx, k8sClient, cluster, *resource, consensusCompName, customLabelSpec.Key, customLabelSpec.Value)
			Expect(err).Should(Succeed())
			By("test Deployment resource PatchGVRCustomLabels")
			resource.GVK = "apps/v1/Deployment"
			err = PatchGVRCustomLabels(ctx, k8sClient, cluster, *resource, consensusCompName, customLabelSpec.Key, customLabelSpec.Value)
			Expect(err).Should(Succeed())
			By("test Service resource PatchGVRCustomLabels")
			resource.GVK = "/v1/Service"
			err = PatchGVRCustomLabels(ctx, k8sClient, cluster, *resource, consensusCompName, customLabelSpec.Key, customLabelSpec.Value)
			Expect(err).Should(Succeed())
			By("test ConfigMap resource PatchGVRCustomLabels")
			resource.GVK = "/v1/ConfigMap"
			err = PatchGVRCustomLabels(ctx, k8sClient, cluster, *resource, consensusCompName, customLabelSpec.Key, customLabelSpec.Value)
			Expect(err).Should(Succeed())
			By("test CronJob resource PatchGVRCustomLabels")
			resource.GVK = "batch/v1/CronJob"
			err = PatchGVRCustomLabels(ctx, k8sClient, cluster, *resource, consensusCompName, customLabelSpec.Key, customLabelSpec.Value)
			Expect(err).Should(Succeed())
			By("test Invalid resource PatchGVRCustomLabels")
			resource.GVK = "Invalid"
			err = PatchGVRCustomLabels(ctx, k8sClient, cluster, *resource, consensusCompName, customLabelSpec.Key, customLabelSpec.Value)
			Expect(err.Error()).Should(ContainSubstring("invalid pattern"))
			By("test Invalid resource PatchGVRCustomLabels")
			resource.GVK = "apps/v1/Invalid"
			err = PatchGVRCustomLabels(ctx, k8sClient, cluster, *resource, consensusCompName, customLabelSpec.Key, customLabelSpec.Value)
			Expect(err.Error()).Should(ContainSubstring("kind is not supported for custom labels"))

			By("test GetCustomLabelWorkloadKind")
			workloadList := GetCustomLabelWorkloadKind()
			Expect(len(workloadList)).Should(Equal(5))

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

			By("test GetComponentInfoByPod function")
			componentName, componentDef, err := GetComponentInfoByPod(ctx, k8sClient, *cluster, &podList.Items[0])
			Expect(err).Should(Succeed())
			Expect(componentName).Should(Equal(consensusCompName))
			Expect(componentDef).ShouldNot(BeNil())
			By("test GetComponentInfoByPod function when Pod is nil")
			_, _, err = GetComponentInfoByPod(ctx, k8sClient, *cluster, nil)
			Expect(err).ShouldNot(Succeed())
			By("test GetComponentInfoByPod function when Pod component label is nil")
			podNoLabel := &podList.Items[0]
			delete(podNoLabel.Labels, constant.KBAppComponentLabelKey)
			_, _, err = GetComponentInfoByPod(ctx, k8sClient, *cluster, podNoLabel)
			Expect(err).ShouldNot(Succeed())

			By("test GetComponentPhaseWhenPodsNotReady function")
			consensusComp := cluster.Spec.GetComponentByName(consensusCompName)
			checkExistFailedPodOfLatestRevision := func(pod *corev1.Pod, workload metav1.Object) bool {
				sts := workload.(*appsv1.StatefulSet)
				return !intctrlutil.PodIsReady(pod) && intctrlutil.PodIsControlledByLatestRevision(pod, sts)
			}
			// component phase should be Failed when available replicas is 0
			phase := GetComponentPhaseWhenPodsNotReady(podList, sts, consensusComp.Replicas,
				sts.Status.AvailableReplicas, checkExistFailedPodOfLatestRevision)
			Expect(phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))

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
			Expect(phase).Should(Equal(appsv1alpha1.AbnormalClusterCompPhase))

		})
	})
})
