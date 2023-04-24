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

package replication

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.StatefulSetSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	testHandleReplicationSet := func() {

		By("Creating a cluster with replication workloadType.")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompDefName).
			SetReplicas(testapps.DefaultReplicationReplicas).
			SetPrimaryIndex(testapps.DefaultReplicationPrimaryIndex).
			Create(&testCtx).GetObject()

		By("Creating a statefulSet of replication workloadType.")
		container := corev1.Container{
			Name:            "mock-redis-container",
			Image:           testapps.DefaultRedisImageName,
			ImagePullPolicy: corev1.PullIfNotPresent,
		}
		sts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace,
			clusterObj.Name+"-"+testapps.DefaultRedisCompName, clusterObj.Name, testapps.DefaultRedisCompName).
			AddFinalizers([]string{DBClusterFinalizerName}).
			AddContainer(container).
			AddAppInstanceLabel(clusterObj.Name).
			AddAppComponentLabel(testapps.DefaultRedisCompName).
			AddAppManangedByLabel().
			SetReplicas(2).
			Create(&testCtx).GetObject()

		By("Creating Pods of replication workloadType.")
		for i := int32(0); i < *sts.Spec.Replicas; i++ {
			_ = testapps.NewPodFactory(testCtx.DefaultNamespace, fmt.Sprintf("%s-%d", sts.Name, i)).
				AddContainer(container).
				AddLabelsInMap(sts.Labels).
				AddRoleLabel(DefaultRole(i)).
				Create(&testCtx).GetObject()
		}
	}

	testNeedUpdateReplicationSetStatus := func() {
		By("Creating a cluster with replication workloadType.")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompDefName).Create(&testCtx).GetObject()

		By("init replicationSet cluster status")
		patch := client.MergeFrom(clusterObj.DeepCopy())
		clusterObj.Status.Phase = appsv1alpha1.RunningClusterPhase
		clusterObj.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
			testapps.DefaultRedisCompName: {
				Phase: appsv1alpha1.RunningClusterCompPhase,
				ReplicationSetStatus: &appsv1alpha1.ReplicationSetStatus{
					Primary: appsv1alpha1.ReplicationMemberStatus{
						Pod: clusterObj.Name + testapps.DefaultRedisCompName + "-0",
					},
					Secondaries: []appsv1alpha1.ReplicationMemberStatus{
						{
							Pod: clusterObj.Name + testapps.DefaultRedisCompName + "-1",
						},
						{
							Pod: clusterObj.Name + testapps.DefaultRedisCompName + "-2",
						},
					},
				},
			},
		}
		Expect(k8sClient.Status().Patch(context.Background(), clusterObj, patch)).Should(Succeed())

		By("testing sync cluster status with add pod")

		var podList []*corev1.Pod
		sts := testk8s.NewFakeStatefulSet(clusterObj.Name+testapps.DefaultRedisCompName, 4)

		for i := int32(0); i < *sts.Spec.Replicas; i++ {
			pod := testapps.NewPodFactory(testCtx.DefaultNamespace, fmt.Sprintf("%s-%d", sts.Name, i)).
				AddContainer(corev1.Container{Name: testapps.DefaultRedisContainerName, Image: testapps.DefaultRedisImageName}).
				AddRoleLabel(DefaultRole(i)).
				Create(&testCtx).GetObject()
			podList = append(podList, pod)
		}
		err := syncReplicationSetStatus(clusterObj.Status.Components[testapps.DefaultRedisCompName].ReplicationSetStatus, podList)
		Expect(err).Should(Succeed())
		Expect(len(clusterObj.Status.Components[testapps.DefaultRedisCompName].ReplicationSetStatus.Secondaries)).Should(Equal(3))

		By("testing sync cluster status with remove pod")
		var podRemoveList []*corev1.Pod
		*sts.Spec.Replicas -= 1
		podRemoveList = append(podRemoveList, podList[len(podList)-1])
		Expect(removeTargetPodsInfoInStatus(clusterObj.Status.Components[testapps.DefaultRedisCompName].ReplicationSetStatus,
			podRemoveList, clusterObj.Spec.ComponentSpecs[0].Replicas)).Should(Succeed())
		Expect(clusterObj.Status.Components[testapps.DefaultRedisCompName].ReplicationSetStatus.Secondaries).Should(HaveLen(2))
	}

	testHandleReplicationSetRoleChangeEvent := func() {
		By("Creating a cluster with replication workloadType.")
		clusterSwitchPolicy := &appsv1alpha1.ClusterSwitchPolicy{
			Type: appsv1alpha1.Noop,
		}
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompDefName).
			SetReplicas(testapps.DefaultReplicationReplicas).
			SetPrimaryIndex(testapps.DefaultReplicationPrimaryIndex).
			SetSwitchPolicy(clusterSwitchPolicy).
			Create(&testCtx).GetObject()

		By("Creating a statefulSet of replication workloadType.")
		container := corev1.Container{
			Name:            "mock-redis-container",
			Image:           testapps.DefaultRedisImageName,
			ImagePullPolicy: corev1.PullIfNotPresent,
		}
		sts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace,
			clusterObj.Name+"-"+testapps.DefaultRedisCompName, clusterObj.Name, testapps.DefaultRedisCompName).
			AddContainer(container).
			AddAppInstanceLabel(clusterObj.Name).
			AddAppComponentLabel(testapps.DefaultRedisCompName).
			AddAppManangedByLabel().
			SetReplicas(2).
			Create(&testCtx).GetObject()

		By("Creating Pods of replication workloadType.")
		var (
			primaryPod    *corev1.Pod
			secondaryPods []*corev1.Pod
		)
		for i := int32(0); i < *sts.Spec.Replicas; i++ {
			pod := testapps.NewPodFactory(testCtx.DefaultNamespace, fmt.Sprintf("%s-%d", sts.Name, i)).
				AddContainer(container).
				AddLabelsInMap(sts.Labels).
				AddRoleLabel(DefaultRole(i)).
				Create(&testCtx).GetObject()
			if pod.Labels[constant.RoleLabelKey] == string(Primary) {
				primaryPod = pod
			} else {
				secondaryPods = append(secondaryPods, pod)
			}
		}
		Expect(primaryPod).ShouldNot(BeNil())
		Expect(secondaryPods).ShouldNot(BeEmpty())

		By("Test update replicationSet pod role label with event driver, secondary change to primary.")
		reqCtx := intctrlutil.RequestCtx{
			Ctx: testCtx.Ctx,
			Log: log.FromContext(ctx).WithValues("event", testCtx.DefaultNamespace),
		}
		Expect(HandleReplicationSetRoleChangeEvent(k8sClient, reqCtx, clusterObj, testapps.DefaultRedisCompName,
			secondaryPods[0], string(Primary))).ShouldNot(HaveOccurred())

		By("Test when secondary change to primary, the old primary label has been updated at the same time, so return nil directly.")
		Expect(HandleReplicationSetRoleChangeEvent(k8sClient, reqCtx, clusterObj, testapps.DefaultRedisCompName,
			primaryPod, string(Secondary))).ShouldNot(HaveOccurred())
	}

	// Scenarios

	Context("test replicationSet util", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj with replication workloadType.")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompDefName).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication workloadType.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(testapps.DefaultRedisCompDefName).AddContainerShort(testapps.DefaultRedisContainerName, testapps.DefaultRedisImageName).
				Create(&testCtx).GetObject()

		})

		It("Test handReplicationSet with different conditions", func() {
			testHandleReplicationSet()
		})

		It("Test need update replicationSet status when horizontal scaling adds pod or removes pod", func() {
			testNeedUpdateReplicationSetStatus()
		})

		It("Test update pod role label by roleChangedEvent when ha switch", func() {
			testHandleReplicationSetRoleChangeEvent()
		})
	})
})
