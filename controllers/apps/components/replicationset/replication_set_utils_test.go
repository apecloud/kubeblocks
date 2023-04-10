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
	"sigs.k8s.io/controller-runtime/pkg/log"

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
		testapps.ClearResources(&testCtx, generics.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	testHandleReplicationSet := func() {

		By("Creating a cluster with replication workloadType.")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompType).
			SetReplicas(testapps.DefaultReplicationReplicas).
			SetPrimaryIndex(testapps.DefaultReplicationPrimaryIndex).
			Create(&testCtx).GetObject()

		By("Creating a statefulSet of replication workloadType.")
		container := corev1.Container{
			Name:            "mock-redis-container",
			Image:           testapps.DefaultRedisImageName,
			ImagePullPolicy: corev1.PullIfNotPresent,
		}
		stsList := make([]*appsv1.StatefulSet, 0)
		secondaryName := clusterObj.Name + "-" + testapps.DefaultRedisCompName + "-1"
		for k, v := range map[string]string{
			string(Primary):   clusterObj.Name + "-" + testapps.DefaultRedisCompName + "-0",
			string(Secondary): secondaryName,
		} {
			sts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, v, clusterObj.Name, testapps.DefaultRedisCompName).
				AddFinalizers([]string{DBClusterFinalizerName}).
				AddContainer(container).
				AddAppInstanceLabel(clusterObj.Name).
				AddAppComponentLabel(testapps.DefaultRedisCompName).
				AddAppManangedByLabel().
				AddRoleLabel(k).
				SetReplicas(1).
				Create(&testCtx).GetObject()
			isStsPrimary, err := checkObjRoleLabelIsPrimary(sts)
			if k == string(Primary) {
				Expect(err).To(Succeed())
				Expect(isStsPrimary).Should(BeTrue())
			} else {
				Expect(err).To(Succeed())
				Expect(isStsPrimary).ShouldNot(BeTrue())
			}
			stsList = append(stsList, sts)
		}

		By("Creating Pods of replication workloadType.")
		for _, sts := range stsList {
			_ = testapps.NewPodFactory(testCtx.DefaultNamespace, sts.Name+"-0").
				AddContainer(container).
				AddLabelsInMap(sts.Labels).
				Create(&testCtx).GetObject()
		}

		By("Test ReplicationSet pod number of sts equals 1")
		_, err := getAndCheckReplicationPodByStatefulSet(ctx, k8sClient, stsList[0])
		Expect(err).Should(Succeed())

		By("Test handleReplicationSet success when stsList count equal cluster.replicas.")
		err = HandleReplicationSet(ctx, k8sClient, clusterObj, stsList)
		Expect(err).Should(Succeed())

		By("Test handleReplicationSet scale-in return err when remove Finalizer after delete the sts")
		clusterObj.Spec.ComponentSpecs[0].Replicas = testapps.DefaultReplicationReplicas - 1
		Expect(HandleReplicationSet(ctx, k8sClient, clusterObj, stsList)).Should(Succeed())
		Eventually(testapps.GetListLen(&testCtx, generics.StatefulSetSignature,
			client.InNamespace(testCtx.DefaultNamespace))).Should(Equal(1))

		By("Test handleReplicationSet scale replicas to 0")
		clusterObj.Spec.ComponentSpecs[0].Replicas = 0
		Expect(HandleReplicationSet(ctx, k8sClient, clusterObj, stsList[:1])).Should(Succeed())
		Eventually(testapps.GetListLen(&testCtx, generics.StatefulSetSignature, client.InNamespace(testCtx.DefaultNamespace))).Should(Equal(0))
		Expect(clusterObj.Status.Components[testapps.DefaultRedisCompName].Phase).Should(Equal(appsv1alpha1.StoppedClusterCompPhase))
	}

	testNeedUpdateReplicationSetStatus := func() {
		By("Creating a cluster with replication workloadType.")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompType).Create(&testCtx).GetObject()

		By("init replicationSet cluster status")
		patch := client.MergeFrom(clusterObj.DeepCopy())
		clusterObj.Status.Phase = appsv1alpha1.RunningClusterPhase
		clusterObj.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
			testapps.DefaultRedisCompName: {
				Phase: appsv1alpha1.RunningClusterCompPhase,
				ReplicationSetStatus: &appsv1alpha1.ReplicationSetStatus{
					Primary: appsv1alpha1.ReplicationMemberStatus{
						Pod: clusterObj.Name + testapps.DefaultRedisCompName + "-0-0",
					},
					Secondaries: []appsv1alpha1.ReplicationMemberStatus{
						{
							Pod: clusterObj.Name + testapps.DefaultRedisCompName + "-1-0",
						},
						{
							Pod: clusterObj.Name + testapps.DefaultRedisCompName + "-2-0",
						},
					},
				},
			},
		}
		Expect(k8sClient.Status().Patch(context.Background(), clusterObj, patch)).Should(Succeed())

		By("testing sync cluster status with add pod")
		var podList []*corev1.Pod
		sts := testk8s.NewFakeStatefulSet(clusterObj.Name+testapps.DefaultRedisCompName+"-3", 3)
		pod := testapps.NewPodFactory(testCtx.DefaultNamespace, sts.Name+"-0").
			AddContainer(corev1.Container{Name: testapps.DefaultRedisContainerName, Image: testapps.DefaultRedisImageName}).
			AddRoleLabel(string(Secondary)).
			Create(&testCtx).GetObject()
		podList = append(podList, pod)
		err := syncReplicationSetStatus(clusterObj.Status.Components[testapps.DefaultRedisCompName].ReplicationSetStatus, podList)
		Expect(err).Should(Succeed())
		Expect(len(clusterObj.Status.Components[testapps.DefaultRedisCompName].ReplicationSetStatus.Secondaries)).Should(Equal(3))

		By("testing sync cluster status with remove pod")
		var podRemoveList []corev1.Pod
		sts = testk8s.NewFakeStatefulSet(clusterObj.Name+testapps.DefaultRedisCompName+"-2", 3)
		pod = testapps.NewPodFactory(testCtx.DefaultNamespace, sts.Name+"-0").
			AddContainer(corev1.Container{Name: testapps.DefaultRedisContainerName, Image: testapps.DefaultRedisImageName}).
			AddRoleLabel(string(Secondary)).
			Create(&testCtx).GetObject()
		podRemoveList = append(podRemoveList, *pod)
		Expect(removeTargetPodsInfoInStatus(clusterObj.Status.Components[testapps.DefaultRedisCompName].ReplicationSetStatus,
			podRemoveList, clusterObj.Spec.ComponentSpecs[0].Replicas)).Should(Succeed())
		Expect(len(clusterObj.Status.Components[testapps.DefaultRedisCompName].ReplicationSetStatus.Secondaries)).Should(Equal(2))
	}

	testGeneratePVCFromVolumeClaimTemplates := func() {
		By("Creating a cluster with replication workloadType.")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompType).
			SetReplicas(testapps.DefaultReplicationReplicas).
			SetPrimaryIndex(testapps.DefaultReplicationPrimaryIndex).
			Create(&testCtx).GetObject()

		By("Creating a statefulSet of replication workloadType.")
		mockStsName := "mock-stateful-set-0"
		mockSts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, mockStsName, clusterObj.Name, testapps.DefaultRedisCompName).
			AddContainer(corev1.Container{Name: testapps.DefaultRedisContainerName, Image: testapps.DefaultRedisImageName}).
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
			Expect(pvc.Name).Should(BeEquivalentTo("mock-vct-mock-stateful-set-0-0"))
		}
	}

	testHandleReplicationSetRoleChangeEvent := func() {
		By("Creating a cluster with replication workloadType.")
		clusterSwitchPolicy := &appsv1alpha1.ClusterSwitchPolicy{
			Type: appsv1alpha1.Noop,
		}
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompType).
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
		stsList := make([]*appsv1.StatefulSet, 0)
		secondaryName := clusterObj.Name + "-" + testapps.DefaultRedisCompName + "-1"
		for k, v := range map[string]string{
			string(Primary):   clusterObj.Name + "-" + testapps.DefaultRedisCompName + "-0",
			string(Secondary): secondaryName,
		} {
			sts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, v, clusterObj.Name, testapps.DefaultRedisCompName).
				AddContainer(container).
				AddAppInstanceLabel(clusterObj.Name).
				AddAppComponentLabel(testapps.DefaultRedisCompName).
				AddAppManangedByLabel().
				AddRoleLabel(k).
				SetReplicas(1).
				Create(&testCtx).GetObject()
			isStsPrimary, err := checkObjRoleLabelIsPrimary(sts)
			if k == string(Primary) {
				Expect(err).To(Succeed())
				Expect(isStsPrimary).Should(BeTrue())
			} else {
				Expect(err).To(Succeed())
				Expect(isStsPrimary).ShouldNot(BeTrue())
			}
			stsList = append(stsList, sts)
		}

		By("Creating Pods of replication workloadType.")
		var (
			primaryPod   *corev1.Pod
			secondaryPod *corev1.Pod
		)
		for _, sts := range stsList {
			pod := testapps.NewPodFactory(testCtx.DefaultNamespace, sts.Name+"-0").
				AddContainer(container).
				AddLabelsInMap(sts.Labels).
				Create(&testCtx).GetObject()
			if sts.Labels[constant.RoleLabelKey] == string(Primary) {
				primaryPod = pod
			} else {
				secondaryPod = pod
			}
		}
		By("Test update replicationSet pod role label with event driver, secondary change to primary.")
		reqCtx := intctrlutil.RequestCtx{
			Ctx: testCtx.Ctx,
			Log: log.FromContext(ctx).WithValues("event", testCtx.DefaultNamespace),
		}
		err := HandleReplicationSetRoleChangeEvent(k8sClient, reqCtx, clusterObj, testapps.DefaultRedisCompName, secondaryPod, string(Primary))
		Expect(err).Should(Succeed())
		By("Test when secondary change to primary, the old primary label has been updated at the same time, so return nil directly.")
		err = HandleReplicationSetRoleChangeEvent(k8sClient, reqCtx, clusterObj, testapps.DefaultRedisCompName, primaryPod, string(Secondary))
		Expect(err).Should(BeNil())
	}

	// Scenarios

	Context("test replicationSet util", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj with replication workloadType.")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompType).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication workloadType.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(testapps.DefaultRedisCompType).AddContainerShort(testapps.DefaultRedisContainerName, testapps.DefaultRedisImageName).
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

		It("Test update pod role label by roleChangedEvent when ha switch", func() {
			testHandleReplicationSetRoleChangeEvent()
		})
	})
})
