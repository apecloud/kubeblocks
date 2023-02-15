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

package replicationset

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("ReplicationSet Util", func() {

	var (
		randomStr           = testCtx.GetRandomStr()
		clusterNamePrefix   = "cluster-replication"
		clusterDefName      = "cluster-def-replication-" + randomStr
		clusterVersionName  = "cluster-version-replication-" + randomStr
		replicationCompName = "replication"
	)

	var (
		clusterDefObj     *dbaasv1alpha1.ClusterDefinition
		clusterVersionObj *dbaasv1alpha1.ClusterVersion
		clusterObj        *dbaasv1alpha1.Cluster
	)

	const replicas = 2
	const redisImage = "redis:7.0.5"
	const redisCompType = "replication"
	const redisCompName = "redis-rsts"

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testdbaas.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	testHandleReplicationSet := func() {

		By("Creating a cluster with replication componentType.")
		clusterObj = testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(redisCompName, redisCompType).
			SetReplicas(replicas).
			SetPrimaryIndex(testdbaas.DefaultReplicationPrimaryIndex).
			Create(&testCtx).GetObject()

		By("Creating a statefulSet of replication componentType.")
		container := corev1.Container{
			Name:            "mock-redis-container",
			Image:           redisImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
		}

		stsList := make([]*appsv1.StatefulSet, 0)
		primaryStsName := clusterObj.Name + "-" + redisCompName + "-0"
		primarySts := testdbaas.NewStatefulSetFactory(testCtx.DefaultNamespace, primaryStsName, clusterObj.Name, redisCompName).
			AddContainer(container).
			AddLabels(intctrlutil.AppInstanceLabelKey, clusterObj.Name,
				intctrlutil.AppComponentLabelKey, redisCompName,
				intctrlutil.AppManagedByLabelKey, testdbaas.KubeBlocks,
				intctrlutil.RoleLabelKey, string(Primary)).
			SetReplicas(1).
			Create(&testCtx).GetObject()
		stsList = append(stsList, primarySts)

		By("Test statefulSet is primary should be true.")
		Expect(CheckStsIsPrimary(primarySts)).Should(BeTrue())

		secondaryStsName := clusterObj.Name + "-" + redisCompName + "-1"
		secondarySts := testdbaas.NewStatefulSetFactory(testCtx.DefaultNamespace, secondaryStsName, clusterObj.Name, redisCompName).
			AddContainer(container).
			AddLabels(intctrlutil.AppInstanceLabelKey, clusterObj.Name,
				intctrlutil.AppComponentLabelKey, redisCompName,
				intctrlutil.AppManagedByLabelKey, testdbaas.KubeBlocks,
				intctrlutil.RoleLabelKey, string(Secondary)).
			SetReplicas(1).
			Create(&testCtx).GetObject()
		stsList = append(stsList, secondarySts)

		By("Test statefulSet is primary should be false.")
		Expect(CheckStsIsPrimary(secondarySts)).ShouldNot(BeTrue())

		By("Test handleReplicationSet return err when there is no pod in sts.")
		err := HandleReplicationSet(ctx, k8sClient, clusterObj, stsList)
		Expect(err).ShouldNot(Succeed())
		Expect(err.Error()).Should(ContainSubstring("pod number in statefulset"))

		By("Creating Pods of replication componentType.")
		primaryPodName := primarySts.Name + "-0"
		_ = testdbaas.NewPodFactory(testCtx.DefaultNamespace, primaryPodName).
			AddContainer(container).
			AddLabelsInMap(primarySts.Labels).
			Create(&testCtx).GetObject()

		secondaryPodName := secondarySts.Name + "-0"
		_ = testdbaas.NewPodFactory(testCtx.DefaultNamespace, secondaryPodName).
			AddContainer(container).
			AddLabelsInMap(secondarySts.Labels).
			Create(&testCtx).GetObject()

		By("Test handleReplicationSet success when stsList count equal cluster.replicas.")
		err = HandleReplicationSet(ctx, k8sClient, clusterObj, stsList)
		Expect(err).Should(Succeed())

		By("Test handleReplicationSet scale-in return err when remove Finalizer after delete the sts")
		*clusterObj.Spec.Components[0].Replicas = replicas - 1
		err = HandleReplicationSet(ctx, k8sClient, clusterObj, stsList)
		Expect(err).ShouldNot(Succeed())
		Expect(err.Error()).Should(ContainSubstring("not found"))
	}

	testNeedUpdateReplicationSetStatus := func() {
		By("Creating a cluster with replication componentType.")
		clusterObj = testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(redisCompName, redisCompType).Create(&testCtx).GetObject()

		By("init replicationSet cluster status")
		patch := client.MergeFrom(clusterObj.DeepCopy())
		clusterObj.Status.Phase = dbaasv1alpha1.RunningPhase
		clusterObj.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
			redisCompName: {
				Phase: dbaasv1alpha1.RunningPhase,
				ReplicationSetStatus: &dbaasv1alpha1.ReplicationSetStatus{
					Primary: dbaasv1alpha1.ReplicationMemberStatus{
						Pod: clusterObj.Name + redisCompName + "-0-0",
					},
					Secondaries: []dbaasv1alpha1.ReplicationMemberStatus{
						{
							Pod: clusterObj.Name + redisCompName + "-1-0",
						},
						{
							Pod: clusterObj.Name + redisCompName + "-2-0",
						},
					},
				},
			},
		}
		Expect(k8sClient.Status().Patch(context.Background(), clusterObj, patch)).Should(Succeed())

		By("testing sync cluster status with add pod")
		var podList []*corev1.Pod
		set := testk8s.NewFakeStatefulSet(clusterObj.Name+redisCompName+"-3", 3)
		pod := testk8s.NewFakeStatefulSetPod(set, 0)
		pod.Labels = make(map[string]string, 0)
		pod.Labels[intctrlutil.RoleLabelKey] = "secondary"
		podList = append(podList, pod)
		Expect(needUpdateReplicationSetStatus(clusterObj.Status.Components[redisCompName].ReplicationSetStatus, podList)).Should(BeTrue())

		By("testing sync cluster status with remove pod")
		var podRemoveList []corev1.Pod
		set = testk8s.NewFakeStatefulSet(clusterObj.Name+redisCompName+"-2", 3)
		pod = testk8s.NewFakeStatefulSetPod(set, 0)
		pod.Labels = make(map[string]string, 0)
		pod.Labels[intctrlutil.RoleLabelKey] = "secondary"
		podRemoveList = append(podRemoveList, *pod)
		Expect(needRemoveReplicationSetStatus(clusterObj.Status.Components[redisCompName].ReplicationSetStatus, podRemoveList)).Should(BeTrue())
	}

	testGeneratePVCFromVolumeClaimTemplates := func() {
		By("Creating a cluster with replication componentType.")
		clusterObj = testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(redisCompName, redisCompType).
			SetReplicas(replicas).
			SetPrimaryIndex(testdbaas.DefaultReplicationPrimaryIndex).
			Create(&testCtx).GetObject()

		mockStsName := "mock-stateful-set-0"
		mockSts := testdbaas.NewStatefulSetFactory(testCtx.DefaultNamespace, mockStsName, clusterObj.Name, redisCompName).
			AddLabels(intctrlutil.AppInstanceLabelKey, clusterObj.Name,
				intctrlutil.AppComponentLabelKey, redisCompName,
				intctrlutil.AppManagedByLabelKey, testdbaas.KubeBlocks,
				intctrlutil.RoleLabelKey, string(Primary)).
			SetReplicas(1).
			Create(&testCtx).GetObject()

		mockVCTList := []corev1.PersistentVolumeClaimTemplate{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mock-vct",
					Namespace: testCtx.DefaultNamespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName: "data",
				},
			},
		}
		pvcMap := GeneratePVCFromVolumeClaimTemplates(mockSts, mockVCTList)
		for _, pvc := range pvcMap {
			Expect(pvc.Name == "mock-vct-mock-stateful-set-0-0").Should(BeTrue())
		}
	}

	// Scenarios

	Context("test replicationSet util", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj with replication componentType.")
			clusterDefObj = testdbaas.NewClusterDefFactory(clusterDefName, testdbaas.RedisType).
				AddComponent(testdbaas.ReplicationRedisComponent, replicationCompName).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication componentType.")
			clusterVersionObj = testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(replicationCompName).AddContainerShort("redis", redisImage).
				Create(&testCtx).GetObject()

		})

		It("Test handReplicationSet with different conditions", func() {
			testHandleReplicationSet()
		})

		It("Test need update replicationSet status when horizontal scaling adds pod or removes pod", func() {
			testNeedUpdateReplicationSetStatus()
		})

		It("Test generatePVC from volume claim templates", func() {
			testGeneratePVCFromVolumeClaimTemplates()
		})
	})
})
