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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlcomputil "github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("ReplicationSet Util", func() {

	var (
		clusterName        = "test-cluster-repl"
		clusterDefName     = "test-cluster-def-repl"
		clusterVersionName = "test-cluster-version-repl"
	)

	var (
		clusterDefObj     *appsv1alpha1.ClusterDefinition
		clusterVersionObj *appsv1alpha1.ClusterVersion
		clusterObj        *appsv1alpha1.Cluster
	)

	cleanAll := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.StatefulSetSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	defaultRole := func(index int32) string {
		role := constant.Secondary
		if index == 0 {
			role = constant.Primary
		}
		return role
	}

	testNeedDeaWithSwitchover := func() {
		By("Creating a cluster with replication workloadType.")
		switchoverCandidate := &appsv1alpha1.SwitchoverCandidate{
			Index:    0,
			Operator: appsv1alpha1.CandidateOpEqual,
		}
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompSpecName, testapps.DefaultRedisCompDefName).
			SetReplicas(testapps.DefaultReplicationReplicas).
			SetSwitchoverCandidate(switchoverCandidate).
			Create(&testCtx).GetObject()

		By("Creating a statefulSet of replication workloadType.")
		container := corev1.Container{
			Name:            "mock-redis-container",
			Image:           testapps.DefaultRedisImageName,
			ImagePullPolicy: corev1.PullIfNotPresent,
		}
		sts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace,
			clusterObj.Name+"-"+testapps.DefaultRedisCompSpecName, clusterObj.Name, testapps.DefaultRedisCompSpecName).
			AddFinalizers([]string{constant.DBClusterFinalizerName}).
			AddContainer(container).
			AddAppInstanceLabel(clusterObj.Name).
			AddAppComponentLabel(testapps.DefaultRedisCompSpecName).
			AddAppManangedByLabel().
			SetReplicas(2).
			Create(&testCtx).GetObject()

		By("Creating Pods of replication workloadType.")
		for i := int32(0); i < *sts.Spec.Replicas; i++ {
			_ = testapps.NewPodFactory(testCtx.DefaultNamespace, fmt.Sprintf("%s-%d", sts.Name, i)).
				AddContainer(container).
				AddLabelsInMap(sts.Labels).
				AddRoleLabel(defaultRole(i)).
				Create(&testCtx).GetObject()
		}
		component := &intctrlcomputil.SynthesizedComponent{
			Name:                clusterObj.Spec.ComponentSpecs[0].Name,
			SwitchoverCandidate: clusterObj.Spec.ComponentSpecs[0].SwitchoverCandidate,
		}

		By("Test cluster.status.Switchover.Condition is nil, switchoverCandidate consistent with pod role label, should not need to deal with switchover.")
		needSwitchover, err := NeedDealWithSwitchover(testCtx.Ctx, k8sClient, clusterObj, component)
		Expect(err).Should(Succeed())
		Expect(needSwitchover).Should(BeFalse())

		By("Test cluster.status.Switchover.Condition is nil, switchoverCandidate is not consistent with pod role label, should need to deal with switchover.")
		component.SwitchoverCandidate.Index = 1
		component.SwitchoverCandidate.Operator = appsv1alpha1.CandidateOpEqual
		needSwitchover, err = NeedDealWithSwitchover(testCtx.Ctx, k8sClient, clusterObj, component)
		Expect(err).Should(Succeed())
		Expect(needSwitchover).Should(BeTrue())

		By("Test cluster.status.Switchover.Condition is nil, switchoverCandidate is not consistent with pod role label and operator is NotEqual, should need to deal with switchover.")
		component.SwitchoverCandidate.Index = 0
		component.SwitchoverCandidate.Operator = appsv1alpha1.CandidateOpNotEqual
		needSwitchover, err = NeedDealWithSwitchover(testCtx.Ctx, k8sClient, clusterObj, component)
		Expect(err).Should(Succeed())
		Expect(needSwitchover).Should(BeTrue())

		By("Test cluster.status.Switchover.Condition is not nil and Status is False, switchoverCandidate is not consistent with pod role label, should need to deal with switchover.")
		component.SwitchoverCandidate.Index = 1
		component.SwitchoverCandidate.Operator = appsv1alpha1.CandidateOpEqual
		newSwitchoverCondition := initSwitchoverCondition(*component.SwitchoverCandidate, component.Name, metav1.ConditionFalse, ReasonSwitchoverStart, clusterObj.Generation)
		meta.SetStatusCondition(&clusterObj.Status.Conditions, *newSwitchoverCondition)
		needSwitchover, err = NeedDealWithSwitchover(testCtx.Ctx, k8sClient, clusterObj, component)
		Expect(err).Should(Succeed())
		Expect(needSwitchover).Should(BeTrue())

		By("Test cluster.status.Switchover.Condition is not nil and Status is True, switchoverCandidate is not consistent with pod role label but consistent with switchoverCondition, should not need to deal with switchover.")
		component.SwitchoverCandidate.Index = 1
		component.SwitchoverCandidate.Operator = appsv1alpha1.CandidateOpEqual
		newSwitchoverCondition = initSwitchoverCondition(*component.SwitchoverCandidate, component.Name, metav1.ConditionTrue, ReasonSwitchoverSucceed, clusterObj.Generation)
		meta.SetStatusCondition(&clusterObj.Status.Conditions, *newSwitchoverCondition)
		needSwitchover, err = NeedDealWithSwitchover(testCtx.Ctx, k8sClient, clusterObj, component)
		Expect(err).Should(Succeed())
		Expect(needSwitchover).Should(BeFalse())

		By("Test cluster.status.Switchover.Condition is not nil and Status is True, switchoverCandidate is not consistent with pod role label and not consistent with switchoverCondition, should need to deal with switchover.")
		component.SwitchoverCandidate.Index = 1
		component.SwitchoverCandidate.Operator = appsv1alpha1.CandidateOpEqual
		newSwitchoverCondition = initSwitchoverCondition(*component.SwitchoverCandidate, component.Name, metav1.ConditionTrue, ReasonSwitchoverSucceed, clusterObj.Generation)
		meta.SetStatusCondition(&clusterObj.Status.Conditions, *newSwitchoverCondition)
		component.SwitchoverCandidate.Index = 2
		component.SwitchoverCandidate.Operator = appsv1alpha1.CandidateOpEqual
		needSwitchover, err = NeedDealWithSwitchover(testCtx.Ctx, k8sClient, clusterObj, component)
		Expect(err).Should(Succeed())
		Expect(needSwitchover).Should(BeTrue())
	}

	testDoSwitchover := func() {
		By("Creating a cluster with replication workloadType.")
		switchoverCandidate := &appsv1alpha1.SwitchoverCandidate{
			Index:    1,
			Operator: appsv1alpha1.CandidateOpEqual,
		}
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompSpecName, testapps.DefaultRedisCompDefName).
			SetReplicas(testapps.DefaultReplicationReplicas).
			SetSwitchoverCandidate(switchoverCandidate).
			Create(&testCtx).GetObject()

		By("Creating a statefulSet of replication workloadType.")
		container := corev1.Container{
			Name:            "mock-redis-container",
			Image:           testapps.DefaultRedisImageName,
			ImagePullPolicy: corev1.PullIfNotPresent,
		}
		sts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace,
			clusterObj.Name+"-"+testapps.DefaultRedisCompSpecName, clusterObj.Name, testapps.DefaultRedisCompSpecName).
			AddFinalizers([]string{constant.DBClusterFinalizerName}).
			AddContainer(container).
			AddAppInstanceLabel(clusterObj.Name).
			AddAppComponentLabel(testapps.DefaultRedisCompSpecName).
			AddAppManangedByLabel().
			SetReplicas(2).
			Create(&testCtx).GetObject()

		By("Creating Pods of replication workloadType.")
		for i := int32(0); i < *sts.Spec.Replicas; i++ {
			_ = testapps.NewPodFactory(testCtx.DefaultNamespace, fmt.Sprintf("%s-%d", sts.Name, i)).
				AddContainer(container).
				AddLabelsInMap(sts.Labels).
				AddRoleLabel(defaultRole(i)).
				Create(&testCtx).GetObject()
		}
		component := &intctrlcomputil.SynthesizedComponent{
			Name:                clusterObj.Spec.ComponentSpecs[0].Name,
			CompDefName:         clusterObj.Spec.ComponentSpecs[0].ComponentDefRef,
			SwitchoverCandidate: clusterObj.Spec.ComponentSpecs[0].SwitchoverCandidate,
			SwitchoverSpec:      clusterDefObj.Spec.ComponentDefs[0].SwitchoverSpec,
			WorkloadType:        clusterDefObj.Spec.ComponentDefs[0].WorkloadType,
		}

		By("Test DoSwitchover failed when switchoverCandidate has changed because controller reconciles many times, and switch job has not finished. .")
		err := DoSwitchover(testCtx.Ctx, k8sClient, clusterObj, component)
		Expect(err).ShouldNot(Succeed())
		Expect(err.Error()).Should(ContainSubstring("job check conditions status failed"))

		By("Test PostOpsSwitchover failed because primary pod role label is not consistent with switchoverCandidate.")
		err = PostOpsSwitchover(testCtx.Ctx, k8sClient, clusterObj, component)
		Expect(err).ShouldNot(Succeed())
		Expect(err.Error()).Should(ContainSubstring("pod role label consistency check failed after switchover"))

		By("Test PostOpsSwitchover succeed because mocks pod role label consistent with switchoverCandidate.")
		component.SwitchoverCandidate.Index = 0
		component.SwitchoverCandidate.Operator = appsv1alpha1.CandidateOpEqual
		err = PostOpsSwitchover(testCtx.Ctx, k8sClient, clusterObj, component)
		Expect(err).Should(Succeed())
	}

	// Scenarios

	Context("test replicationSet util", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj with replication workloadType.")
			commandExecutorEnvItem := &appsv1alpha1.CommandExecutorEnvItem{
				Image: testapps.DefaultRedisImageName,
			}
			commandExecutorItem := &appsv1alpha1.CommandExecutorItem{
				Command: []string{"echo", "hello"},
				Args:    []string{},
			}
			switchoverSpec := &appsv1alpha1.SwitchoverSpec{
				CommandExecutorEnvItem: *commandExecutorEnvItem,
				WithCandidate: &appsv1alpha1.SwitchoverAction{
					CommandExecutorItem: *commandExecutorItem,
				},
				WithoutCandidate: &appsv1alpha1.SwitchoverAction{
					CommandExecutorItem: *commandExecutorItem,
				},
			}
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompDefName).
				AddSwitchoverSpec(switchoverSpec).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication workloadType.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponentVersion(testapps.DefaultRedisCompDefName).AddContainerShort(testapps.DefaultRedisContainerName, testapps.DefaultRedisImageName).
				Create(&testCtx).GetObject()

		})

		It("Test NeedDeaWithSwitchover with different conditions", func() {
			testNeedDeaWithSwitchover()
		})

		It("Test DoSwitchover when switchoverCandidate triggers", func() {
			testDoSwitchover()
		})
	})
})
