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

package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/sethvargo/go-password/password"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	"github.com/apecloud/kubeblocks/pkg/controller/rsm"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	lorry "github.com/apecloud/kubeblocks/pkg/lorry/client"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	backupPolicyTPLName = "test-backup-policy-template-mysql"
	backupMethodName    = "test-backup-method"
	vsBackupMethodName  = "test-vs-backup-method"
	actionSetName       = "test-action-set"
	vsActionSetName     = "test-vs-action-set"
)

var (
	podAnnotationKey4Test = fmt.Sprintf("%s-test", constant.ComponentReplicasAnnotationKey)
)

var mockLorryClient = func(mock func(*lorry.MockClientMockRecorder)) {
	mockLorryCli := lorry.GetMockClient()
	if mockLorryCli == nil {
		ctrl := gomock.NewController(GinkgoT())
		mockLorryCli = lorry.NewMockClient(ctrl)
	}
	if mock != nil {
		mockCli := mockLorryCli.(*lorry.MockClient)
		mock(mockCli.EXPECT())
	}
	lorry.SetMockClient(mockLorryCli, nil)
}

var mockLorryClientDefault = func() {
	mockLorryClient(func(recorder *lorry.MockClientMockRecorder) {
		recorder.CreateUser(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		recorder.GrantUserRole(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	})
}

var mockLorryClient4HScale = func(clusterKey types.NamespacedName, compName string, replicas int) {
	mockLorryClient(func(recorder *lorry.MockClientMockRecorder) {
		recorder.JoinMember(gomock.Any()).Return(nil).AnyTimes()
		recorder.LeaveMember(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			var podList corev1.PodList
			labels := client.MatchingLabels{
				constant.AppInstanceLabelKey:    clusterKey.Name,
				constant.KBAppComponentLabelKey: compName,
			}
			if err := testCtx.Cli.List(ctx, &podList, labels, client.InNamespace(clusterKey.Namespace)); err != nil {
				return err
			}
			for _, pod := range podList.Items {
				if pod.Annotations == nil {
					panic(fmt.Sprintf("pod annotations is nil: %s", pod.Name))
				}
				if pod.Annotations[podAnnotationKey4Test] == fmt.Sprintf("%d", replicas) {
					continue
				}
				pod.Annotations[podAnnotationKey4Test] = fmt.Sprintf("%d", replicas)
				if err := testCtx.Cli.Update(ctx, &pod); err != nil {
					return err
				}
			}
			return nil
		}).AnyTimes()
	})
}

var _ = Describe("Component Controller", func() {
	const (
		clusterDefName     = "test-clusterdef"
		clusterVersionName = "test-clusterversion"
		compDefName        = "test-compdef"
		clusterName        = "test-cluster" // this become cluster prefix name if used with testapps.NewClusterFactory().WithRandomName()
		leader             = "leader"
		follower           = "follower"
		// REVIEW:
		// - setup componentName and componentDefName as map entry pair
		statelessCompName      = "stateless"
		statelessCompDefName   = "stateless"
		statefulCompName       = "stateful"
		statefulCompDefName    = "stateful"
		consensusCompName      = "consensus"
		consensusCompDefName   = "consensus"
		replicationCompName    = "replication"
		replicationCompDefName = "replication"
		defaultCompName        = "default"
		actionSetName          = "test-actionset"
	)

	var (
		clusterDefObj     *appsv1alpha1.ClusterDefinition
		clusterVersionObj *appsv1alpha1.ClusterVersion
		compDefObj        *appsv1alpha1.ComponentDefinition
		clusterObj        *appsv1alpha1.Cluster
		clusterKey        types.NamespacedName
		compObj           *appsv1alpha1.Component
		compKey           types.NamespacedName
		allSettings       map[string]interface{}
	)

	resetViperCfg := func() {
		if allSettings != nil {
			Expect(viper.MergeConfigMap(allSettings)).ShouldNot(HaveOccurred())
			allSettings = nil
		}
	}

	resetTestContext := func() {
		clusterDefObj = nil
		clusterVersionObj = nil
		clusterObj = nil
		resetViperCfg()
	}

	// Cleanups
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.VolumeSnapshotSignature, true, inNS)
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.BackupPolicyTemplateSignature, ml)
		testapps.ClearResources(&testCtx, generics.ActionSetSignature, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
		resetTestContext()
	}

	BeforeEach(func() {
		cleanEnv()
		allSettings = viper.AllSettings()
	})

	AfterEach(func() {
		cleanEnv()
	})

	randomStr := func() string {
		str, _ := password.Generate(6, 0, 0, true, false)
		return str
	}

	// test function helpers
	createAllWorkloadTypesClusterDef := func(noCreateAssociateCV ...bool) {
		By("Create a clusterDefinition obj")
		clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(testapps.StatefulMySQLComponent, statefulCompDefName).
			AddComponentDef(testapps.ConsensusMySQLComponent, consensusCompDefName).
			AddComponentDef(testapps.ReplicationRedisComponent, replicationCompDefName).
			AddComponentDef(testapps.StatelessNginxComponent, statelessCompDefName).
			Create(&testCtx).GetObject()

		if len(noCreateAssociateCV) > 0 && noCreateAssociateCV[0] {
			return
		}
		By("Create a clusterVersion obj")
		clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
			AddComponentVersion(statefulCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			AddComponentVersion(consensusCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			AddComponentVersion(replicationCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			AddComponentVersion(statelessCompDefName).AddContainerShort("nginx", testapps.NginxImage).
			Create(&testCtx).GetObject()

		By("Create a componentDefinition obj")
		compDefObj = testapps.NewComponentDefinitionFactory(compDefName).
			WithRandomName().
			SetDefaultSpec().
			Create(&testCtx).
			GetObject()

		By("Mock lorry client for the default transformer of system accounts provision")
		mockLorryClientDefault()
	}

	waitForCreatingResourceCompletely := func(clusterKey client.ObjectKey, compNames ...string) {
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		cluster := &appsv1alpha1.Cluster{}
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, cluster, true)).Should(Succeed())
		for _, compName := range compNames {
			compPhase := appsv1alpha1.CreatingClusterCompPhase
			for _, spec := range cluster.Spec.ComponentSpecs {
				if spec.Name == compName && spec.Replicas == 0 {
					compPhase = appsv1alpha1.StoppedClusterCompPhase
				}
			}
			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(compPhase))
		}
	}

	createClusterObjVx := func(compName, compDefName string, v2 bool, processor func(*testapps.MockClusterFactory)) {
		factory := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefObj.Name, clusterVersionObj.Name).
			WithRandomName()
		if !v2 {
			factory.AddComponent(compName, compDefName).SetReplicas(1)
		} else {
			factory.AddComponentV2(compName, compDefName).SetReplicas(1)
		}
		if processor != nil {
			processor(factory)
		}
		clusterObj = factory.Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter Creating phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		if clusterObj.Spec.ComponentSpecs[0].Replicas == 0 {
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StoppedClusterPhase))
		} else {
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))
		}

		By("Waiting for the component enter Creating phase")
		compKey = types.NamespacedName{
			Namespace: clusterObj.Namespace,
			Name:      component.FullName(clusterObj.Name, compName),
		}
		compObj = &appsv1alpha1.Component{}
		Eventually(testapps.CheckObjExists(&testCtx, compKey, compObj, true)).Should(Succeed())
		Eventually(testapps.GetComponentObservedGeneration(&testCtx, compKey)).Should(BeEquivalentTo(1))
		if compObj.Spec.Replicas == 0 {
			Eventually(testapps.GetComponentPhase(&testCtx, compKey)).Should(Equal(appsv1alpha1.StoppedClusterCompPhase))
		} else {
			Eventually(testapps.GetComponentPhase(&testCtx, compKey)).Should(Equal(appsv1alpha1.CreatingClusterCompPhase))
		}
	}

	createClusterObj := func(compName, compDefName string, processor func(*testapps.MockClusterFactory)) {
		By("Creating a cluster")
		createClusterObjVx(compName, compDefName, false, processor)
	}

	createClusterObjV2 := func(compName, compDefName string, processor func(*testapps.MockClusterFactory)) {
		By("Creating a cluster with new component definition")
		createClusterObjVx(compName, compDefName, true, processor)
	}

	mockCompRunning := func(compName string) {
		rsmList := testk8s.ListAndCheckRSMWithComponent(&testCtx, client.ObjectKeyFromObject(clusterObj), compName)
		Expect(rsmList.Items).Should(HaveLen(1))
		rsm := rsmList.Items[0]
		sts := testapps.NewStatefulSetFactory(rsm.Namespace, rsm.Name, clusterObj.Name, compName).
			SetReplicas(*rsm.Spec.Replicas).
			Create(&testCtx).
			GetObject()
		pods := testapps.MockConsensusComponentPods(&testCtx, sts, clusterObj.Name, compName)
		Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
			testk8s.MockStatefulSetReady(sts)
		})).ShouldNot(HaveOccurred())
		Expect(testapps.ChangeObjStatus(&testCtx, &rsm, func() {
			testk8s.MockRSMReady(&rsm, pods...)
		})).ShouldNot(HaveOccurred())
		Eventually(testapps.GetComponentPhase(&testCtx, types.NamespacedName{
			Namespace: clusterObj.Namespace,
			Name:      component.FullName(clusterObj.Name, compName),
		})).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
	}

	// createCompObjNoWait := func(compName, compDefName string, processor func(*testapps.MockComponentFactory)) {
	//	By("Creating a component")
	//	factory := testapps.NewComponentFactory(testCtx.DefaultNamespace, component.FullName(clusterName, compName), compDefName).
	//		SetReplicas(1)
	//	if processor != nil {
	//		processor(factory)
	//	}
	//	compObj = factory.Create(&testCtx).GetObject()
	//	compKey = client.ObjectKeyFromObject(compObj)
	// }

	// createCompObj := func(compName, compDefName string, processor func(*testapps.MockComponentFactory)) {
	//	createCompObjNoWait(compName, compDefName, processor)
	//	waitCompObjCreating(compName, compDefName)
	// }

	changeCompReplicas := func(clusterName types.NamespacedName, replicas int32, comp *appsv1alpha1.ClusterComponentSpec) {
		Expect(testapps.GetAndChangeObj(&testCtx, clusterName, func(cluster *appsv1alpha1.Cluster) {
			for i, clusterComp := range cluster.Spec.ComponentSpecs {
				if clusterComp.Name == comp.Name {
					cluster.Spec.ComponentSpecs[i].Replicas = replicas
				}
			}
		})()).ShouldNot(HaveOccurred())
	}

	changeComponentReplicas := func(clusterName types.NamespacedName, replicas int32) {
		Expect(testapps.GetAndChangeObj(&testCtx, clusterName, func(cluster *appsv1alpha1.Cluster) {
			Expect(cluster.Spec.ComponentSpecs).Should(HaveLen(1))
			cluster.Spec.ComponentSpecs[0].Replicas = replicas
		})()).ShouldNot(HaveOccurred())
	}

	checkSingleWorkload := func(compDefName string, expects func(g Gomega, sts *appsv1.StatefulSet, deploy *appsv1.Deployment)) {
		Eventually(func(g Gomega) {
			l := testk8s.ListAndCheckRSM(&testCtx, clusterKey)
			sts := rsm.ConvertRSMToSTS(&l.Items[0])
			expects(g, sts, nil)
		}).Should(Succeed())
	}

	testChangeReplicas := func(compName, compDefName string) {
		Expect(compDefName).Should(BeElementOf(statelessCompDefName, statefulCompDefName, replicationCompDefName, consensusCompDefName))
		createClusterObj(compName, compDefName, nil)
		replicasSeq := []int32{5, 3, 1, 0, 2, 4}
		expectedOG := int64(1)
		for _, replicas := range replicasSeq {
			By(fmt.Sprintf("Change replicas to %d", replicas))
			changeComponentReplicas(clusterKey, replicas)
			expectedOG++
			By("Checking cluster status and the number of replicas changed")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
				g.Expect(fetched.Status.ObservedGeneration).To(BeEquivalentTo(expectedOG))
				g.Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(BeElementOf(appsv1alpha1.CreatingClusterPhase, appsv1alpha1.UpdatingClusterPhase))
			})).Should(Succeed())

			checkSingleWorkload(compDefName, func(g Gomega, sts *appsv1.StatefulSet, deploy *appsv1.Deployment) {
				if sts != nil {
					g.Expect(int(*sts.Spec.Replicas)).To(BeEquivalentTo(replicas))
				} else {
					g.Expect(int(*deploy.Spec.Replicas)).To(BeEquivalentTo(replicas))
				}
			})
		}
	}

	getPVCName := func(vctName, compName string, i int) string {
		return fmt.Sprintf("%s-%s-%s-%d", vctName, clusterKey.Name, compName, i)
	}

	createPVC := func(clusterName, pvcName, compName, storageSize, storageClassName string) {
		if storageSize == "" {
			storageSize = "1Gi"
		}
		clusterBytes, _ := json.Marshal(clusterObj)
		testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName,
			compName, testapps.DataVolumeName).
			AddLabelsInMap(map[string]string{
				constant.AppInstanceLabelKey:    clusterName,
				constant.KBAppComponentLabelKey: compName,
				constant.AppManagedByLabelKey:   constant.AppName,
			}).AddAnnotations(constant.LastAppliedClusterAnnotationKey, string(clusterBytes)).
			SetStorage(storageSize).
			SetStorageClass(storageClassName).
			CheckedCreate(&testCtx)
	}

	mockComponentPVCsAndBound := func(comp *appsv1alpha1.ClusterComponentSpec, replicas int, create bool, storageClassName string) {
		for i := 0; i < replicas; i++ {
			for _, vct := range comp.VolumeClaimTemplates {
				pvcKey := types.NamespacedName{
					Namespace: clusterKey.Namespace,
					Name:      getPVCName(vct.Name, comp.Name, i),
				}
				if create {
					createPVC(clusterKey.Name, pvcKey.Name, comp.Name, vct.Spec.Resources.Requests.Storage().String(), storageClassName)
				}
				Eventually(testapps.CheckObjExists(&testCtx, pvcKey,
					&corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Phase = corev1.ClaimBound
					if pvc.Status.Capacity == nil {
						pvc.Status.Capacity = corev1.ResourceList{}
					}
					pvc.Status.Capacity[corev1.ResourceStorage] = pvc.Spec.Resources.Requests[corev1.ResourceStorage]
				})).Should(Succeed())
			}
		}
	}

	mockPodsForTest := func(cluster *appsv1alpha1.Cluster, number int) []corev1.Pod {
		clusterDefName := cluster.Spec.ClusterDefRef
		componentName := cluster.Spec.ComponentSpecs[0].Name
		clusterName := cluster.Name
		stsName := cluster.Name + "-" + componentName
		pods := make([]corev1.Pod, 0)
		for i := 0; i < number; i++ {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      stsName + "-" + strconv.Itoa(i),
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						constant.AppManagedByLabelKey:         constant.AppName,
						constant.AppNameLabelKey:              clusterDefName,
						constant.AppInstanceLabelKey:          clusterName,
						constant.KBAppComponentLabelKey:       componentName,
						appsv1.ControllerRevisionHashLabelKey: "mock-version",
					},
					Annotations: map[string]string{
						podAnnotationKey4Test: fmt.Sprintf("%d", number),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "mock-container",
						Image: "mock-container",
					}},
				},
			}
			pods = append(pods, *pod)
		}
		return pods
	}

	horizontalScaleComp := func(updatedReplicas int, comp *appsv1alpha1.ClusterComponentSpec,
		storageClassName string, policy *appsv1alpha1.HorizontalScalePolicy) {
		By("Mocking component PVCs to bound")
		mockComponentPVCsAndBound(comp, int(comp.Replicas), true, storageClassName)

		By("Checking rsm replicas right")
		rsmList := testk8s.ListAndCheckRSMWithComponent(&testCtx, clusterKey, comp.Name)
		Expect(int(*rsmList.Items[0].Spec.Replicas)).To(BeEquivalentTo(comp.Replicas))

		By("Creating mock pods in StatefulSet")
		pods := mockPodsForTest(clusterObj, int(comp.Replicas))
		for i, pod := range pods {
			if comp.ComponentDefRef == replicationCompDefName && i == 0 {
				By("mocking primary for replication to pass check")
				pods[0].ObjectMeta.Labels[constant.RoleLabelKey] = "primary"
			}
			Expect(testCtx.CheckedCreateObj(testCtx.Ctx, &pod)).Should(Succeed())
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(&pod), func(p *corev1.Pod) {
				// mock the status to pass the isReady(pod) check in consensus_set
				p.Status.Conditions = []corev1.PodCondition{{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}}
			})).Should(Succeed())
		}

		By(fmt.Sprintf("Changing replicas to %d", updatedReplicas))
		changeCompReplicas(clusterKey, int32(updatedReplicas), comp)

		checkUpdatedStsReplicas := func() {
			By("Checking updated sts replicas")
			Eventually(func() int32 {
				rsmList := testk8s.ListAndCheckRSMWithComponent(&testCtx, clusterKey, comp.Name)
				return *rsmList.Items[0].Spec.Replicas
			}).Should(BeEquivalentTo(updatedReplicas))
		}

		scaleOutCheck := func() {
			if comp.Replicas == 0 {
				return
			}

			ml := client.MatchingLabels{
				constant.AppInstanceLabelKey:    clusterKey.Name,
				constant.KBAppComponentLabelKey: comp.Name,
				constant.KBManagedByKey:         "cluster",
			}
			if policy != nil {
				By(fmt.Sprintf("Checking backup of component %s created", comp.Name))
				Eventually(testapps.List(&testCtx, generics.BackupSignature,
					ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(1))

				backupKey := types.NamespacedName{Name: fmt.Sprintf("%s-%s-scaling",
					clusterKey.Name, comp.Name),
					Namespace: testCtx.DefaultNamespace}
				By("Mocking backup status to completed")
				Expect(testapps.GetAndChangeObjStatus(&testCtx, backupKey, func(backup *dpv1alpha1.Backup) {
					backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
					backup.Status.PersistentVolumeClaimName = "backup-data"
					testdp.MockBackupStatusMethod(backup, testdp.BackupMethodName, testapps.DataVolumeName, testdp.ActionSetName)
				})()).Should(Succeed())

				if testk8s.IsMockVolumeSnapshotEnabled(&testCtx, storageClassName) {
					By("Mocking VolumeSnapshot and set it as ReadyToUse")
					pvcName := getPVCName(testapps.DataVolumeName, comp.Name, 0)
					volumeSnapshot := &snapshotv1.VolumeSnapshot{
						ObjectMeta: metav1.ObjectMeta{
							Name:      backupKey.Name,
							Namespace: backupKey.Namespace,
							Labels: map[string]string{
								dptypes.BackupNameLabelKey: backupKey.Name,
							}},
						Spec: snapshotv1.VolumeSnapshotSpec{
							Source: snapshotv1.VolumeSnapshotSource{
								PersistentVolumeClaimName: &pvcName,
							},
						},
					}
					scheme, _ := appsv1alpha1.SchemeBuilder.Build()
					Expect(controllerruntime.SetControllerReference(clusterObj, volumeSnapshot, scheme)).Should(Succeed())
					Expect(testCtx.CreateObj(testCtx.Ctx, volumeSnapshot)).Should(Succeed())
					readyToUse := true
					volumeSnapshotStatus := snapshotv1.VolumeSnapshotStatus{ReadyToUse: &readyToUse}
					volumeSnapshot.Status = &volumeSnapshotStatus
					Expect(k8sClient.Status().Update(testCtx.Ctx, volumeSnapshot)).Should(Succeed())
				}
			}

			By("Mock PVCs and set status to bound")
			mockComponentPVCsAndBound(comp, updatedReplicas, true, storageClassName)

			if policy != nil {
				checkRestoreAndSetCompleted(clusterKey, comp.Name, updatedReplicas-int(comp.Replicas))
			}

			if policy != nil {
				By("Checking Backup and Restore cleanup")
				Eventually(testapps.List(&testCtx, generics.BackupSignature, ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(0))
				Eventually(testapps.List(&testCtx, generics.RestoreSignature, ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(0))
			}

			checkUpdatedStsReplicas()

			By("Checking updated sts replicas' PVC and size")
			for _, vct := range comp.VolumeClaimTemplates {
				var volumeQuantity resource.Quantity
				for i := 0; i < updatedReplicas; i++ {
					pvcKey := types.NamespacedName{
						Namespace: clusterKey.Namespace,
						Name:      getPVCName(vct.Name, comp.Name, i),
					}
					Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
						if volumeQuantity.IsZero() {
							volumeQuantity = pvc.Spec.Resources.Requests[corev1.ResourceStorage]
						}
						Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(volumeQuantity))
						Expect(pvc.Status.Capacity[corev1.ResourceStorage]).To(Equal(volumeQuantity))
					})).Should(Succeed())
				}
			}
		}

		scaleInCheck := func() {
			if updatedReplicas == 0 {
				Consistently(func(g Gomega) {
					pvcList := corev1.PersistentVolumeClaimList{}
					g.Expect(testCtx.Cli.List(testCtx.Ctx, &pvcList, client.MatchingLabels{
						constant.AppInstanceLabelKey:    clusterKey.Name,
						constant.KBAppComponentLabelKey: comp.Name,
					})).Should(Succeed())
					for _, pvc := range pvcList.Items {
						ss := strings.Split(pvc.Name, "-")
						idx, _ := strconv.Atoi(ss[len(ss)-1])
						if idx >= updatedReplicas && idx < int(comp.Replicas) {
							g.Expect(pvc.DeletionTimestamp).Should(BeNil())
						}
					}
				}).Should(Succeed())
				return
			}

			checkUpdatedStsReplicas()

			By("Checking pvcs deleting")
			Eventually(func(g Gomega) {
				pvcList := corev1.PersistentVolumeClaimList{}
				g.Expect(testCtx.Cli.List(testCtx.Ctx, &pvcList, client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: comp.Name,
				})).Should(Succeed())
				for _, pvc := range pvcList.Items {
					ss := strings.Split(pvc.Name, "-")
					idx, _ := strconv.Atoi(ss[len(ss)-1])
					if idx >= updatedReplicas && idx < int(comp.Replicas) {
						g.Expect(pvc.DeletionTimestamp).ShouldNot(BeNil())
					}
				}
			}).Should(Succeed())

			By("Checking pod's annotation should be updated consistently")
			Eventually(func(g Gomega) {
				podList := corev1.PodList{}
				g.Expect(testCtx.Cli.List(testCtx.Ctx, &podList, client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: comp.Name,
				})).Should(Succeed())
				for _, pod := range podList.Items {
					ss := strings.Split(pod.Name, "-")
					ordinal, _ := strconv.Atoi(ss[len(ss)-1])
					if ordinal >= updatedReplicas {
						continue
					}
					g.Expect(pod.Annotations[podAnnotationKey4Test]).Should(Equal(fmt.Sprintf("%d", updatedReplicas)))
				}
			}).Should(Succeed())
		}

		if int(comp.Replicas) < updatedReplicas {
			scaleOutCheck()
		}
		if int(comp.Replicas) > updatedReplicas {
			scaleInCheck()
		}
	}

	setHorizontalScalePolicy := func(policyType appsv1alpha1.HScaleDataClonePolicyType, componentDefsWithHScalePolicy ...string) {
		By(fmt.Sprintf("Set HorizontalScalePolicy, policyType is %s", policyType))
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
			func(clusterDef *appsv1alpha1.ClusterDefinition) {
				// assign 1st component
				if len(componentDefsWithHScalePolicy) == 0 && len(clusterDef.Spec.ComponentDefs) > 0 {
					componentDefsWithHScalePolicy = []string{
						clusterDef.Spec.ComponentDefs[0].Name,
					}
				}
				for i, compDef := range clusterDef.Spec.ComponentDefs {
					if !slices.Contains(componentDefsWithHScalePolicy, compDef.Name) {
						continue
					}

					if len(policyType) == 0 {
						clusterDef.Spec.ComponentDefs[i].HorizontalScalePolicy = nil
						continue
					}

					By("Checking backup policy created from backup policy template")
					policyName := generateBackupPolicyName(clusterKey.Name, compDef.Name, "")
					clusterDef.Spec.ComponentDefs[i].HorizontalScalePolicy = &appsv1alpha1.HorizontalScalePolicy{
						Type:                     policyType,
						BackupPolicyTemplateName: backupPolicyTPLName,
					}

					Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKey{Name: policyName, Namespace: clusterKey.Namespace},
						&dpv1alpha1.BackupPolicy{}, true)).Should(Succeed())

					if policyType == appsv1alpha1.HScaleDataClonePolicyCloneVolume {
						By("creating actionSet if backup policy is backup")
						actionSet := &dpv1alpha1.ActionSet{
							ObjectMeta: metav1.ObjectMeta{
								Name:      actionSetName,
								Namespace: clusterKey.Namespace,
								Labels: map[string]string{
									constant.ClusterDefLabelKey: clusterDef.Name,
								},
							},
							Spec: dpv1alpha1.ActionSetSpec{
								Env: []corev1.EnvVar{
									{
										Name:  "test-name",
										Value: "test-value",
									},
								},
								BackupType: dpv1alpha1.BackupTypeFull,
								Backup: &dpv1alpha1.BackupActionSpec{
									BackupData: &dpv1alpha1.BackupDataActionSpec{
										JobActionSpec: dpv1alpha1.JobActionSpec{
											Image:   "xtrabackup",
											Command: []string{""},
										},
									},
								},
								Restore: &dpv1alpha1.RestoreActionSpec{
									PrepareData: &dpv1alpha1.JobActionSpec{
										Image: "xtrabackup",
										Command: []string{
											"sh",
											"-c",
											"/backup_scripts.sh",
										},
									},
								},
							},
						}
						testapps.CheckedCreateK8sResource(&testCtx, actionSet)
					}
				}
			})()).ShouldNot(HaveOccurred())
	}

	// @argument componentDefsWithHScalePolicy assign ClusterDefinition.spec.componentDefs[].horizontalScalePolicy for
	// the matching names. If not provided, will set 1st ClusterDefinition.spec.componentDefs[0].horizontalScalePolicy.
	horizontalScale := func(updatedReplicas int, storageClassName string,
		policyType appsv1alpha1.HScaleDataClonePolicyType, componentDefsWithHScalePolicy ...string) {
		defer lorry.UnsetMockClient()

		cluster := &appsv1alpha1.Cluster{}
		Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(Succeed())
		initialGeneration := int(cluster.Status.ObservedGeneration)

		setHorizontalScalePolicy(policyType, componentDefsWithHScalePolicy...)

		By("Mocking all components' PVCs to bound")
		for _, comp := range cluster.Spec.ComponentSpecs {
			mockComponentPVCsAndBound(&comp, int(comp.Replicas), true, storageClassName)
		}

		hscalePolicy := func(comp appsv1alpha1.ClusterComponentSpec) *appsv1alpha1.HorizontalScalePolicy {
			for _, componentDef := range clusterDefObj.Spec.ComponentDefs {
				if componentDef.Name == comp.ComponentDefRef {
					return componentDef.HorizontalScalePolicy
				}
			}
			return nil
		}

		By("Get the latest cluster def")
		Expect(k8sClient.Get(testCtx.Ctx, client.ObjectKeyFromObject(clusterDefObj), clusterDefObj)).Should(Succeed())
		for i, comp := range cluster.Spec.ComponentSpecs {
			mockLorryClient4HScale(clusterKey, comp.Name, updatedReplicas)

			By(fmt.Sprintf("H-scale component %s with policy %s", comp.Name, hscalePolicy(comp)))
			horizontalScaleComp(updatedReplicas, &cluster.Spec.ComponentSpecs[i], storageClassName, hscalePolicy(comp))
		}

		By("Checking cluster status and the number of replicas changed")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).
			Should(BeEquivalentTo(initialGeneration + len(cluster.Spec.ComponentSpecs)))
	}

	testHorizontalScale := func(compName, compDefName string, initialReplicas, updatedReplicas int32,
		dataClonePolicy appsv1alpha1.HScaleDataClonePolicyType) {
		By("Creating a single component cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		createClusterObj(compName, compDefName, func(f *testapps.MockClusterFactory) {
			f.SetReplicas(initialReplicas).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				AddVolumeClaimTemplate(testapps.LogVolumeName, pvcSpec)
		})

		// REVIEW: this test flow, wait for running phase?
		testk8s.MockEnableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)

		horizontalScale(int(updatedReplicas), testk8s.DefaultStorageClassName, dataClonePolicy, compDefName)
	}

	testVolumeExpansion := func(compName, compDefName string, storageClass *storagev1.StorageClass) {
		var (
			replicas          = 3
			volumeSize        = "1Gi"
			newVolumeSize     = "2Gi"
			volumeQuantity    = resource.MustParse(volumeSize)
			newVolumeQuantity = resource.MustParse(newVolumeSize)
		)

		By("Mock a StorageClass which allows resize")
		Expect(*storageClass.AllowVolumeExpansion).Should(BeTrue())

		By("Creating a cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec(volumeSize)
		pvcSpec.StorageClassName = &storageClass.Name

		By("Create cluster and waiting for the cluster initialized")
		createClusterObj(compName, compDefName, func(f *testapps.MockClusterFactory) {
			f.SetReplicas(int32(replicas)).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				AddVolumeClaimTemplate(testapps.LogVolumeName, pvcSpec)
		})

		By("Checking the replicas")
		rsmList := testk8s.ListAndCheckRSM(&testCtx, clusterKey)
		rsm := &rsmList.Items[0]
		sts := testapps.NewStatefulSetFactory(rsm.Namespace, rsm.Name, clusterObj.Name, compName).
			SetReplicas(*rsm.Spec.Replicas).
			Create(&testCtx).GetObject()
		Expect(*sts.Spec.Replicas).Should(BeEquivalentTo(replicas))

		By("Mock PVCs in Bound Status")
		for i := 0; i < replicas; i++ {
			for _, vctName := range []string{testapps.DataVolumeName, testapps.LogVolumeName} {
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getPVCName(vctName, compName, i),
						Namespace: clusterKey.Namespace,
						Labels: map[string]string{
							constant.AppManagedByLabelKey:   constant.AppName,
							constant.AppInstanceLabelKey:    clusterKey.Name,
							constant.KBAppComponentLabelKey: compName,
						}},
					Spec: pvcSpec.ToV1PersistentVolumeClaimSpec(),
				}
				Expect(testCtx.CreateObj(testCtx.Ctx, pvc)).Should(Succeed())
				pvc.Status.Phase = corev1.ClaimBound // only bound pvc allows resize
				if pvc.Status.Capacity == nil {
					pvc.Status.Capacity = corev1.ResourceList{}
				}
				pvc.Status.Capacity[corev1.ResourceStorage] = volumeQuantity
				Expect(k8sClient.Status().Update(testCtx.Ctx, pvc)).Should(Succeed())
			}
		}

		By("mock pods/sts of component are available")
		var mockPods []*corev1.Pod
		switch compDefName {
		case statelessCompDefName:
			// ignore
		case replicationCompDefName:
			mockPods = testapps.MockReplicationComponentPods(nil, testCtx, sts, clusterObj.Name, compDefName, nil)
		case statefulCompDefName, consensusCompDefName:
			mockPods = testapps.MockConsensusComponentPods(&testCtx, sts, clusterObj.Name, compName)
		}
		Expect(testapps.ChangeObjStatus(&testCtx, rsm, func() {
			testk8s.MockRSMReady(rsm, mockPods...)
		})).ShouldNot(HaveOccurred())
		Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
			testk8s.MockStatefulSetReady(sts)
		})).ShouldNot(HaveOccurred())

		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))

		By("Updating data PVC storage size")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			comp := &cluster.Spec.ComponentSpecs[0]
			for i, vct := range comp.VolumeClaimTemplates {
				if vct.Name == testapps.DataVolumeName {
					comp.VolumeClaimTemplates[i].Spec.Resources.Requests[corev1.ResourceStorage] = newVolumeQuantity
				}
			}
		})()).ShouldNot(HaveOccurred())

		By("Checking the resize operation in progress for data volume")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(appsv1alpha1.UpdatingClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.UpdatingClusterPhase))
		for i := 0; i < replicas; i++ {
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(testapps.DataVolumeName, compName, i),
			}
			Expect(k8sClient.Get(testCtx.Ctx, pvcKey, pvc)).Should(Succeed())
			Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(newVolumeQuantity))
			Expect(pvc.Status.Capacity[corev1.ResourceStorage]).To(Equal(volumeQuantity))
		}

		By("Mock resizing of data volumes finished")
		for i := 0; i < replicas; i++ {
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(testapps.DataVolumeName, compName, i),
			}
			Expect(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Status.Capacity[corev1.ResourceStorage] = newVolumeQuantity
			})()).ShouldNot(HaveOccurred())
		}

		By("Checking the resize operation finished")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))

		By("Checking data volumes are resized")
		for i := 0; i < replicas; i++ {
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(testapps.DataVolumeName, compName, i),
			}
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.Status.Capacity[corev1.ResourceStorage]).To(Equal(newVolumeQuantity))
			})).Should(Succeed())
		}

		By("Checking log volumes stay unchanged")
		for i := 0; i < replicas; i++ {
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(testapps.LogVolumeName, compName, i),
			}
			Expect(k8sClient.Get(testCtx.Ctx, pvcKey, pvc)).Should(Succeed())
			Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(volumeQuantity))
			Expect(pvc.Status.Capacity[corev1.ResourceStorage]).To(Equal(volumeQuantity))
		}
	}

	testVolumeExpansionFailedAndRecover := func(compName, compDefName string) {

		const storageClassName = "test-sc"
		const replicas = 3

		By("Mock a StorageClass which allows resize")
		sc := testapps.CreateStorageClass(&testCtx, storageClassName, true)

		By("Creating a cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		pvcSpec.StorageClassName = &sc.Name

		By("Create cluster and waiting for the cluster initialized")
		createClusterObj(compName, compDefName, func(f *testapps.MockClusterFactory) {
			f.SetReplicas(replicas).AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec)
		})

		By("Mock PVCs in Bound Status")
		for i := 0; i < replicas; i++ {
			tmpSpec := pvcSpec.ToV1PersistentVolumeClaimSpec()
			tmpSpec.VolumeName = getPVCName(testapps.DataVolumeName, compName, i)
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getPVCName(testapps.DataVolumeName, compName, i),
					Namespace: clusterKey.Namespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: clusterKey.Name,
					}},
				Spec: tmpSpec,
			}
			Expect(testCtx.CreateObj(testCtx.Ctx, pvc)).Should(Succeed())
			pvc.Status.Phase = corev1.ClaimBound // only bound pvc allows resize
			Expect(k8sClient.Status().Update(testCtx.Ctx, pvc)).Should(Succeed())
		}

		By("mocking PVs")
		for i := 0; i < replicas; i++ {
			pv := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getPVCName(testapps.DataVolumeName, compName, i), // use same name as pvc
					Namespace: clusterKey.Namespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: clusterKey.Name,
					}},
				Spec: corev1.PersistentVolumeSpec{
					Capacity: corev1.ResourceList{
						"storage": resource.MustParse("1Gi"),
					},
					AccessModes: []corev1.PersistentVolumeAccessMode{
						"ReadWriteOnce",
					},
					PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
					StorageClassName:              storageClassName,
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/opt/volume/nginx",
							Type: nil,
						},
					},
					ClaimRef: &corev1.ObjectReference{
						Name: getPVCName(testapps.DataVolumeName, compName, i),
					},
				},
			}
			Expect(testCtx.CreateObj(testCtx.Ctx, pv)).Should(Succeed())
		}

		changePVC := func(quantity resource.Quantity) {
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
				comp := &cluster.Spec.ComponentSpecs[0]
				comp.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = quantity
			})()).ShouldNot(HaveOccurred())
		}

		checkPVC := func(quantity resource.Quantity) {
			for i := 0; i < replicas; i++ {
				pvcKey := types.NamespacedName{
					Namespace: clusterKey.Namespace,
					Name:      getPVCName(testapps.DataVolumeName, compName, i),
				}
				Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
					g.Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(quantity))
				})).Should(Succeed())
			}
		}

		checkResizeOperationFinished := func(generation int64) {
			Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(generation))
			Eventually(testapps.GetComponentObservedGeneration(&testCtx, compKey)).Should(BeEquivalentTo(generation))
		}

		By("Updating the PVC storage size")
		newStorageValue := resource.MustParse("2Gi")
		changePVC(newStorageValue)

		By("Checking the resize operation finished")
		checkResizeOperationFinished(2)

		By("Checking PVCs are resized")
		checkPVC(newStorageValue)

		By("Updating the PVC storage size back")
		originStorageValue := resource.MustParse("1Gi")
		changePVC(originStorageValue)

		By("Checking the resize operation finished")
		checkResizeOperationFinished(3)

		By("Checking PVCs are resized")
		checkPVC(originStorageValue)
	}

	testCompFinalizerNLabel := func(compName, compDefName string) {
		createClusterObjV2(compName, compDefObj.Name, nil)

		By("check component finalizers and labels")
		Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *appsv1alpha1.Component) {
			// g.Expect(comp.Finalizers).Should(ContainElements(constant.DBComponentFinalizerName))
			g.Expect(comp.Finalizers).Should(ContainElements(constant.DBClusterFinalizerName))
			g.Expect(comp.Labels).Should(HaveKeyWithValue(constant.ComponentDefinitionLabelKey, comp.Spec.CompDef))
			g.Expect(comp.Labels).Should(HaveKeyWithValue(constant.AppManagedByLabelKey, constant.AppName))
			g.Expect(comp.Labels).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterObj.Name))
			g.Expect(comp.Labels).Should(HaveKeyWithValue(constant.KBAppComponentLabelKey, compName))
		})).Should(Succeed())
	}

	testCompService := func(compName, compDefName string) {
		createClusterObjV2(compName, compDefObj.Name, nil)

		targetPort := corev1.ServicePort{
			Protocol: corev1.ProtocolTCP,
			Port:     3306,
			TargetPort: intstr.IntOrString{
				Type:   intstr.String,
				StrVal: "mysql",
			},
		}

		By("check rw component services")
		rwSvcKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateComponentServiceName(clusterObj.Name, compName, "rw"),
		}
		Eventually(testapps.CheckObj(&testCtx, rwSvcKey, func(g Gomega, svc *corev1.Service) {
			g.Expect(svc.Spec.Ports).Should(ContainElements(targetPort))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.AppManagedByLabelKey, constant.AppName))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterObj.Name))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.KBAppComponentLabelKey, compName))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.RoleLabelKey, "leader"))

		})).Should(Succeed())

		By("check ro component services")
		roSvcKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateComponentServiceName(clusterObj.Name, compName, "ro"),
		}
		Eventually(testapps.CheckObj(&testCtx, roSvcKey, func(g Gomega, svc *corev1.Service) {
			g.Expect(svc.Spec.Ports).Should(ContainElements(targetPort))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.AppManagedByLabelKey, constant.AppName))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterObj.Name))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.KBAppComponentLabelKey, compName))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.RoleLabelKey, "follower"))
		})).Should(Succeed())
	}

	testCompSystemAccount := func(compName, compDefName string) {
		createClusterObjV2(compName, compDefObj.Name, nil)

		By("check root account")
		rootSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateAccountSecretName(clusterObj.Name, compName, "root"),
		}
		Eventually(testapps.CheckObj(&testCtx, rootSecretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("root")))
			g.Expect(secret.Data).Should(HaveKey(constant.AccountPasswdForSecret))
		})).Should(Succeed())

		By("check admin account")
		adminSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateAccountSecretName(clusterObj.Name, compName, "admin"),
		}
		Eventually(testapps.CheckObj(&testCtx, adminSecretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("admin")))
			g.Expect(secret.Data).Should(HaveKey(constant.AccountPasswdForSecret))
		})).Should(Succeed())

		By("mock component as Running")
		mockCompRunning(compName)

		By("wait accounts to be provisioned")
		Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *appsv1alpha1.Component) {
			g.Expect(len(comp.Status.Conditions) > 0).Should(BeTrue())
			var cond *metav1.Condition
			for i, c := range comp.Status.Conditions {
				if c.Type == accountProvisionConditionType {
					cond = &comp.Status.Conditions[i]
					break
				}
			}
			g.Expect(cond).ShouldNot(BeNil())
			g.Expect(cond.Status).Should(BeEquivalentTo(metav1.ConditionTrue))
			g.Expect(cond.Message).Should(ContainSubstring("root"))
			g.Expect(cond.Message).Should(ContainSubstring("admin"))
		})).Should(Succeed())
	}

	testCompConnCredential := func(compName, compDefName string) {
		createClusterObjV2(compName, compDefObj.Name, nil)

		By("check root conn credential")
		serviceName := constant.GenerateComponentServiceName(clusterObj.Name, compName, "rw")
		servicePort := "3306"
		endpoint := fmt.Sprintf("%s:%s", serviceName, servicePort)
		rootSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateComponentConnCredential(clusterObj.Name, compName, "root"),
		}
		Eventually(testapps.CheckObj(&testCtx, rootSecretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKeyWithValue("endpoint", []byte(endpoint)))
			g.Expect(secret.Data).Should(HaveKeyWithValue("host", []byte(serviceName)))
			g.Expect(secret.Data).Should(HaveKeyWithValue("port", []byte(servicePort)))
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("root")))
			g.Expect(secret.Data).Should(HaveKey(constant.AccountPasswdForSecret))
		})).Should(Succeed())

		By("check admin conn credential")
		adminSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateComponentConnCredential(clusterObj.Name, compName, "admin"),
		}
		Eventually(testapps.CheckObj(&testCtx, adminSecretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKeyWithValue("endpoint", []byte(endpoint)))
			g.Expect(secret.Data).Should(HaveKeyWithValue("host", []byte(serviceName)))
			g.Expect(secret.Data).Should(HaveKeyWithValue("port", []byte(servicePort)))
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("admin")))
			g.Expect(secret.Data).Should(HaveKey(constant.AccountPasswdForSecret))
		})).Should(Succeed())
	}

	testCompRole := func(compName, compDefName string) {
		createClusterObjV2(compName, compDefObj.Name, nil)

		By("check default component roles")
		targetRoles := []workloads.ReplicaRole{
			{
				Name:       "leader",
				AccessMode: workloads.ReadWriteMode,
				CanVote:    true,
				IsLeader:   true,
			},
			{
				Name:       "follower",
				AccessMode: workloads.ReadonlyMode,
				CanVote:    true,
				IsLeader:   false,
			},
			{
				Name:       "learner",
				AccessMode: workloads.NoneMode,
				CanVote:    false,
				IsLeader:   false,
			},
		}
		rsmKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      compObj.Name,
		}
		Eventually(testapps.CheckObj(&testCtx, rsmKey, func(g Gomega, rsm *workloads.ReplicatedStateMachine) {
			g.Expect(rsm.Spec.Roles).Should(HaveExactElements(targetRoles))
		})).Should(Succeed())
	}

	testCompTLSConfig := func(compName, compDefName string) {
		createClusterObjV2(compName, compDefObj.Name, func(f *testapps.MockClusterFactory) {
			issuer := &appsv1alpha1.Issuer{
				Name: appsv1alpha1.IssuerKubeBlocks,
			}
			f.SetTLS(true).SetIssuer(issuer)
		})

		By("check TLS secret")
		secretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      plan.GenerateTLSSecretName(clusterObj.Name, compName),
		}
		Eventually(testapps.CheckObj(&testCtx, secretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKey(factory.CAName))
			g.Expect(secret.Data).Should(HaveKey(factory.CertName))
			g.Expect(secret.Data).Should(HaveKey(factory.KeyName))
		})).Should(Succeed())

		By("check pod's volumes and mounts")
		targetVolume := corev1.Volume{
			Name: factory.VolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretKey.Name,
					Items: []corev1.KeyToPath{
						{Key: factory.CAName, Path: factory.CAName},
						{Key: factory.CertName, Path: factory.CertName},
						{Key: factory.KeyName, Path: factory.KeyName},
					},
					Optional: func() *bool { o := false; return &o }(),
				},
			},
		}
		targetVolumeMount := corev1.VolumeMount{
			Name:      factory.VolumeName,
			MountPath: factory.MountPath,
			ReadOnly:  true,
		}
		rsmKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      compObj.Name,
		}
		Eventually(testapps.CheckObj(&testCtx, rsmKey, func(g Gomega, rsm *workloads.ReplicatedStateMachine) {
			podSpec := rsm.Spec.Template.Spec
			g.Expect(podSpec.Volumes).Should(ContainElements(targetVolume))
			for _, c := range podSpec.Containers {
				g.Expect(c.VolumeMounts).Should(ContainElements(targetVolumeMount))
			}
		})).Should(Succeed())
	}

	testCompConfiguration := func(compName, compDefName string) {
	}

	testCompAffinityNToleration := func(compName, compDefName string) {
		const (
			topologyKey     = "testTopologyKey"
			labelKey        = "testNodeLabelKey"
			labelValue      = "testNodeLabelValue"
			tolerationKey   = "testTolerationKey"
			tolerationValue = "testTolerationValue"
		)

		By("Creating a component with affinity and toleration")
		affinity := appsv1alpha1.Affinity{
			PodAntiAffinity: appsv1alpha1.Required,
			TopologyKeys:    []string{topologyKey},
			NodeLabels: map[string]string{
				labelKey: labelValue,
			},
			Tenancy: appsv1alpha1.SharedNode,
		}
		toleration := corev1.Toleration{
			Key:      tolerationKey,
			Value:    tolerationValue,
			Operator: corev1.TolerationOpEqual,
			Effect:   corev1.TaintEffectNoSchedule,
		}
		createClusterObjV2(compName, compDefObj.Name, func(f *testapps.MockClusterFactory) {
			f.SetComponentAffinity(&affinity).AddComponentToleration(toleration)
		})

		By("Checking the Affinity, the TopologySpreadConstraints and Tolerations")
		rsmKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      compObj.Name,
		}
		Eventually(testapps.CheckObj(&testCtx, rsmKey, func(g Gomega, rsm *workloads.ReplicatedStateMachine) {
			podSpec := rsm.Spec.Template.Spec
			// node affinity
			g.Expect(podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).To(Equal(labelKey))
			// pod anti-affinity
			g.Expect(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution).Should(HaveLen(1))
			g.Expect(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey).To(Equal(topologyKey))
			// topology spread constraint
			g.Expect(podSpec.TopologySpreadConstraints).Should(HaveLen(1))
			// Required -> DoNotSchedule, Preferred -> ScheduleAnyway
			g.Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable).To(Equal(corev1.DoNotSchedule))
			g.Expect(podSpec.TopologySpreadConstraints[0].TopologyKey).To(Equal(topologyKey))
			// toleration
			g.Expect(podSpec.Tolerations).Should(HaveLen(2))
			g.Expect(podSpec.Tolerations[0]).Should(BeEquivalentTo(toleration))
		})).Should(Succeed())
	}

	checkRBACResourcesExistence := func(saName string, expectExisted bool) {
		saKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      saName,
		}
		rbKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      saName,
		}
		crbKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      saName,
		}
		Eventually(testapps.CheckObjExists(&testCtx, saKey, &corev1.ServiceAccount{}, expectExisted)).Should(Succeed())
		Eventually(testapps.CheckObjExists(&testCtx, rbKey, &rbacv1.RoleBinding{}, expectExisted)).Should(Succeed())
		Eventually(testapps.CheckObjExists(&testCtx, crbKey, &rbacv1.ClusterRoleBinding{}, expectExisted)).Should(Succeed())
	}

	testCompRBAC := func(compName, compDefName, saName string) {
		By("update comp definition to enable volume protection")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(compDefObj), func(compDef *appsv1alpha1.ComponentDefinition) {
			for i, vol := range compDef.Spec.Volumes {
				if vol.HighWatermark <= 0 || vol.HighWatermark >= 100 {
					compDef.Spec.Volumes[i].HighWatermark = 85
				}
			}
		})()).Should(Succeed())

		By("creating a component with target service account name")
		if len(saName) == 0 {
			saName = "test-sa-" + randomStr()
		}
		createClusterObjV2(compName, compDefObj.Name, func(f *testapps.MockClusterFactory) {
			f.SetServiceAccountName(saName)
		})

		By("check the service account used in Pod")
		rsmKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      compObj.Name,
		}
		Eventually(testapps.CheckObj(&testCtx, rsmKey, func(g Gomega, rsm *workloads.ReplicatedStateMachine) {
			g.Expect(rsm.Spec.Template.Spec.ServiceAccountName).To(Equal(saName))
		})).Should(Succeed())

		By("check the RBAC resources created")
		checkRBACResourcesExistence(saName, true)
	}

	testRecreateCompWithRBAC := func(compName, compDefName string) {
		saName := "test-sa-" + randomStr()
		testCompRBAC(compName, compDefName, saName)

		By("delete the cluster(component)")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, false)).Should(Succeed())

		By("check the RBAC resources deleted")
		checkRBACResourcesExistence(saName, false)

		By("re-create cluster(component) with same name")
		testCompRBAC(compName, compDefName, saName)
	}

	testReplicationWorkloadRunning := func(compName, compDefName string) {
		By("Mock a cluster obj with replication componentDefRef.")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(compName, compDefName).
			SetReplicas(testapps.DefaultReplicationReplicas).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compDefName)

		By("Checking statefulSet number")
		rsmList := testk8s.ListAndCheckRSMItemsCount(&testCtx, clusterKey, 1)
		rsm := &rsmList.Items[0]
		sts := testapps.NewStatefulSetFactory(rsm.Namespace, rsm.Name, clusterKey.Name, compName).
			SetReplicas(*rsm.Spec.Replicas).Create(&testCtx).GetObject()
		mockPods := testapps.MockReplicationComponentPods(nil, testCtx, sts, clusterObj.Name, compDefName, nil)
		Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
			testk8s.MockStatefulSetReady(sts)
		})).ShouldNot(HaveOccurred())
		Expect(testapps.ChangeObjStatus(&testCtx, rsm, func() {
			testk8s.MockRSMReady(rsm, mockPods...)
		})).ShouldNot(HaveOccurred())
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))
	}

	testThreeReplicas := func(compName, compDefName string) {
		const replicas = 3

		By("Mock a cluster obj")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(compName, compDefName).
			SetReplicas(replicas).AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		var rsm *workloads.ReplicatedStateMachine
		Eventually(func(g Gomega) {
			rsmList := testk8s.ListAndCheckRSM(&testCtx, clusterKey)
			g.Expect(rsmList.Items).ShouldNot(BeEmpty())
			rsm = &rsmList.Items[0]
		}).Should(Succeed())
		sts := testapps.NewStatefulSetFactory(rsm.Namespace, rsm.Name, clusterKey.Name, compName).
			AddAppComponentLabel(rsm.Labels[constant.KBAppComponentLabelKey]).
			AddAppInstanceLabel(rsm.Labels[constant.AppInstanceLabelKey]).
			SetReplicas(*rsm.Spec.Replicas).Create(&testCtx).GetObject()

		By("Creating mock pods in StatefulSet, and set controller reference")
		pods := mockPodsForTest(clusterObj, replicas)
		for i, pod := range pods {
			Expect(controllerutil.SetControllerReference(sts, &pod, scheme.Scheme)).Should(Succeed())
			Expect(testCtx.CreateObj(testCtx.Ctx, &pod)).Should(Succeed())
			patch := client.MergeFrom(pod.DeepCopy())
			// mock the status to pass the isReady(pod) check in consensus_set
			pod.Status.Conditions = []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}}
			Eventually(k8sClient.Status().Patch(ctx, &pod, patch)).Should(Succeed())
			role := "follower"
			if i == 0 {
				role = "leader"
			}
			patch = client.MergeFrom(pod.DeepCopy())
			pod.Labels[constant.RoleLabelKey] = role
			Eventually(k8sClient.Patch(ctx, &pod, patch)).Should(Succeed())
		}

		By("Checking pods' role are changed accordingly")
		Eventually(func(g Gomega) {
			pods, err := common.GetPodListByStatefulSet(ctx, k8sClient, sts)
			g.Expect(err).ShouldNot(HaveOccurred())
			// should have 3 pods
			g.Expect(pods).Should(HaveLen(3))
			// 1 leader
			// 2 followers
			leaderCount, followerCount := 0, 0
			for _, pod := range pods {
				switch pod.Labels[constant.RoleLabelKey] {
				case leader:
					leaderCount++
				case follower:
					followerCount++
				}
			}
			g.Expect(leaderCount).Should(Equal(1))
			g.Expect(followerCount).Should(Equal(2))
		}).Should(Succeed())

		// trigger rsm to reconcile as the underlying sts is not created
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(sts), func(rsm *workloads.ReplicatedStateMachine) {
			rsm.Annotations["time"] = time.Now().Format(time.RFC3339)
		})()).Should(Succeed())
		By("Checking pods' annotations")
		Eventually(func(g Gomega) {
			pods, err := common.GetPodListByStatefulSet(ctx, k8sClient, sts)
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(pods).Should(HaveLen(int(*sts.Spec.Replicas)))
			for _, pod := range pods {
				g.Expect(pod.Annotations).ShouldNot(BeNil())
				g.Expect(pod.Annotations[constant.ComponentReplicasAnnotationKey]).Should(Equal(strconv.Itoa(int(*sts.Spec.Replicas))))
			}
		}).Should(Succeed())
		rsmPatch := client.MergeFrom(rsm.DeepCopy())
		By("Updating RSM's status")
		rsm.Status.UpdateRevision = "mock-version"
		pods, err := common.GetPodListByStatefulSet(ctx, k8sClient, sts)
		Expect(err).Should(BeNil())
		var podList []*corev1.Pod
		for i := range pods {
			podList = append(podList, &pods[i])
		}
		testk8s.MockRSMReady(rsm, podList...)
		Expect(k8sClient.Status().Patch(ctx, rsm, rsmPatch)).Should(Succeed())

		stsPatch := client.MergeFrom(sts.DeepCopy())
		By("Updating StatefulSet's status")
		sts.Status.UpdateRevision = "mock-version"
		sts.Status.Replicas = int32(replicas)
		sts.Status.AvailableReplicas = int32(replicas)
		sts.Status.CurrentReplicas = int32(replicas)
		sts.Status.ReadyReplicas = int32(replicas)
		sts.Status.ObservedGeneration = sts.Generation
		Expect(k8sClient.Status().Patch(ctx, sts, stsPatch)).Should(Succeed())

		By("Checking consensus set pods' role are updated in cluster status")
		Eventually(func(g Gomega) {
			fetched := &appsv1alpha1.Cluster{}
			g.Expect(k8sClient.Get(ctx, clusterKey, fetched)).To(Succeed())
			compName := fetched.Spec.ComponentSpecs[0].Name
			g.Expect(fetched.Status.Components != nil).To(BeTrue())
			g.Expect(fetched.Status.Components).To(HaveKey(compName))
			_, ok := fetched.Status.Components[compName]
			g.Expect(ok).Should(BeTrue())
			// TODO(component): workload status
			// getStsPodsName := func(sts *appsv1.StatefulSet) []string {
			//	pods, err := common.GetPodListByStatefulSet(ctx, k8sClient, sts)
			//	Expect(err).To(Succeed())
			//
			//	names := make([]string, 0)
			//	for _, pod := range pods {
			//		names = append(names, pod.Name)
			//	}
			//	return names
			// }
			// consensusStatus := compStatus.ConsensusSetStatus
			// g.Expect(consensusStatus != nil).To(BeTrue())
			// g.Expect(consensusStatus.Leader.Pod).To(BeElementOf(getStsPodsName(sts)))
			// g.Expect(consensusStatus.Followers).Should(HaveLen(2))
			// g.Expect(consensusStatus.Followers[0].Pod).To(BeElementOf(getStsPodsName(sts)))
			// g.Expect(consensusStatus.Followers[1].Pod).To(BeElementOf(getStsPodsName(sts)))
		}).Should(Succeed())

		By("Waiting the component be running")
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).
			Should(Equal(appsv1alpha1.RunningClusterCompPhase))
	}

	testRestoreClusterFromBackup := func(compName, compDefName string) {
		By("mock backuptool object")
		backupPolicyName := "test-backup-policy"
		backupName := "test-backup"
		_ = testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml", &dpv1alpha1.ActionSet{}, testapps.RandomizedObjName())

		By("creating backup")
		backup := testdp.NewBackupFactory(testCtx.DefaultNamespace, backupName).
			SetBackupPolicyName(backupPolicyName).
			SetBackupMethod(testdp.BackupMethodName).
			Create(&testCtx).GetObject()

		By("mocking backup status completed, we don't need backup reconcile here")
		Eventually(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(backup), func(backup *dpv1alpha1.Backup) {
			backup.Status.PersistentVolumeClaimName = "backup-pvc"
			backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
			testdp.MockBackupStatusMethod(backup, testdp.BackupMethodName, testapps.DataVolumeName, testdp.ActionSetName)
		})).Should(Succeed())

		By("creating cluster with backup")
		restoreFromBackup := fmt.Sprintf(`{"%s":{"name":"%s"}}`, compName, backupName)
		pvcSpec := testapps.NewPVCSpec("1Gi")
		replicas := 3
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefObj.Name, clusterVersionObj.Name).
			WithRandomName().
			AddComponent(compName, compDefName).
			SetReplicas(int32(replicas)).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			AddAnnotations(constant.RestoreFromBackupAnnotationKey, restoreFromBackup).
			Create(&testCtx).
			GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		// mock pvcs have restored
		mockComponentPVCsAndBound(clusterObj.Spec.GetComponentByName(compName), replicas, true, testk8s.DefaultStorageClassName)

		By("wait for restore created")
		ml := client.MatchingLabels{
			constant.AppInstanceLabelKey:    clusterKey.Name,
			constant.KBAppComponentLabelKey: compName,
		}
		Eventually(testapps.List(&testCtx, generics.RestoreSignature,
			ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(1))

		By("Mocking restore phase to Completed")
		// mock prepareData restore completed
		mockRestoreCompleted(ml)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		rsmList := testk8s.ListAndCheckRSM(&testCtx, clusterKey)
		rsm := rsmList.Items[0]
		sts := testapps.NewStatefulSetFactory(rsm.Namespace, rsm.Name, clusterKey.Name, compName).
			SetReplicas(*rsm.Spec.Replicas).
			Create(&testCtx).GetObject()
		By("mock pod/sts are available and wait for component enter running phase")
		mockPods := testapps.MockConsensusComponentPods(&testCtx, sts, clusterObj.Name, compName)
		Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
			testk8s.MockStatefulSetReady(sts)
		})).ShouldNot(HaveOccurred())
		Expect(testapps.ChangeObjStatus(&testCtx, &rsm, func() {
			testk8s.MockRSMReady(&rsm, mockPods...)
		})).ShouldNot(HaveOccurred())
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))

		By("the restore container has been removed from init containers")
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(&rsm), func(g Gomega, tmpRSM *workloads.ReplicatedStateMachine) {
			g.Expect(tmpRSM.Spec.Template.Spec.InitContainers).Should(BeEmpty())
		})).Should(Succeed())

		By("clean up annotations after cluster running")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
			g.Expect(tmpCluster.Status.Phase).Should(Equal(appsv1alpha1.RunningClusterPhase))
			// mock postReady restore completed
			mockRestoreCompleted(ml)
			g.Expect(tmpCluster.Annotations[constant.RestoreFromBackupAnnotationKey]).Should(BeEmpty())
		})).Should(Succeed())
	}

	testBackupError := func(compName, compDefName string) {
		initialReplicas := int32(1)
		updatedReplicas := int32(3)
		testk8s.MockEnableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)

		By("Set HorizontalScalePolicy")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
			func(clusterDef *appsv1alpha1.ClusterDefinition) {
				for i, def := range clusterDef.Spec.ComponentDefs {
					if def.Name != compDefName {
						continue
					}
					clusterDef.Spec.ComponentDefs[i].HorizontalScalePolicy =
						&appsv1alpha1.HorizontalScalePolicy{Type: appsv1alpha1.HScaleDataClonePolicyCloneVolume,
							BackupPolicyTemplateName: backupPolicyTPLName}
				}
			})()).ShouldNot(HaveOccurred())

		By("Creating a cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		createClusterObj(compName, compDefName, func(f *testapps.MockClusterFactory) {
			f.SetReplicas(initialReplicas).AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec)
		})

		By("Create and Mock PVCs status to bound")
		for _, comp := range clusterObj.Spec.ComponentSpecs {
			mockComponentPVCsAndBound(&comp, int(comp.Replicas), true, testk8s.DefaultStorageClassName)
		}

		By(fmt.Sprintf("Changing replicas to %d", updatedReplicas))
		changeCompReplicas(clusterKey, updatedReplicas, &clusterObj.Spec.ComponentSpecs[0])
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))

		By("Waiting for the backup object been created")
		ml := client.MatchingLabels{
			constant.AppInstanceLabelKey:    clusterKey.Name,
			constant.KBAppComponentLabelKey: compName,
		}
		Eventually(testapps.List(&testCtx, generics.BackupSignature,
			ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(1))

		By("Mocking backup status to failed")
		backupList := dpv1alpha1.BackupList{}
		Expect(testCtx.Cli.List(testCtx.Ctx, &backupList, ml)).Should(Succeed())
		backupKey := types.NamespacedName{
			Namespace: backupList.Items[0].Namespace,
			Name:      backupList.Items[0].Name,
		}
		Expect(testapps.GetAndChangeObjStatus(&testCtx, backupKey, func(backup *dpv1alpha1.Backup) {
			backup.Status.Phase = dpv1alpha1.BackupPhaseFailed
		})()).Should(Succeed())

		By("Checking cluster status failed with backup error")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
			g.Expect(testk8s.IsMockVolumeSnapshotEnabled(&testCtx, testk8s.DefaultStorageClassName)).Should(BeTrue())
			g.Expect(cluster.Status.Conditions).ShouldNot(BeEmpty())
			var err error
			for _, cond := range cluster.Status.Conditions {
				if strings.Contains(cond.Message, "backup for horizontalScaling failed") {
					err = fmt.Errorf("has backup error")
					break
				}
			}
			g.Expect(err).Should(HaveOccurred())
		})).Should(Succeed())

		By("Expect for backup error event")
		Eventually(func(g Gomega) {
			eventList := corev1.EventList{}
			Expect(k8sClient.List(ctx, &eventList, client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
			hasBackupErrorEvent := false
			for _, v := range eventList.Items {
				if v.Reason == string(intctrlutil.ErrorTypeBackupFailed) {
					hasBackupErrorEvent = true
					break
				}
			}
			g.Expect(hasBackupErrorEvent).Should(BeTrue())
		}).Should(Succeed())
	}

	testUpdateKubeBlocksToolsImage := func(compName, compDefName string) {
		createClusterObj(compName, compDefName, nil)

		oldToolsImage := viper.GetString(constant.KBToolsImage)
		newToolsImage := fmt.Sprintf("%s-%s", oldToolsImage, rand.String(4))
		defer func() {
			viper.Set(constant.KBToolsImage, oldToolsImage)
		}()

		underlyingWorkload := func() *workloads.ReplicatedStateMachine {
			rsmList := testk8s.ListAndCheckRSM(&testCtx, clusterKey)
			return &rsmList.Items[0]
		}

		initWorkloadGeneration := underlyingWorkload().GetGeneration()
		Expect(initWorkloadGeneration).ShouldNot(Equal(0))

		checkWorkloadGenerationAndToolsImage := func(assertion func(any, ...any) AsyncAssertion,
			workloadGenerationExpected int64, oldImageCntExpected, newImageCntExpected int) {
			assertion(func(g Gomega) {
				sts := underlyingWorkload()
				g.Expect(sts.Generation).Should(Equal(workloadGenerationExpected))
				oldImageCnt := 0
				newImageCnt := 0
				for _, c := range sts.Spec.Template.Spec.Containers {
					if c.Image == oldToolsImage {
						oldImageCnt += 1
					}
					if c.Image == newToolsImage {
						newImageCnt += 1
					}
				}
				g.Expect(oldImageCnt).Should(Equal(oldImageCntExpected))
				g.Expect(newImageCnt).Should(Equal(newImageCntExpected))
			}).Should(Succeed())
		}

		By("check the workload generation as init")
		checkWorkloadGenerationAndToolsImage(Consistently, initWorkloadGeneration, 1, 0)

		By("update kubeblocks tools image")
		viper.Set(constant.KBToolsImage, newToolsImage)

		By("update component annotation to trigger component status reconcile")
		Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *appsv1alpha1.Component) {
			comp.Annotations = map[string]string{"time": time.Now().Format(time.RFC3339)}
		})()).Should(Succeed())
		checkWorkloadGenerationAndToolsImage(Consistently, initWorkloadGeneration, 1, 0)

		By("update spec to trigger component spec reconcile, but workload not changed")
		Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *appsv1alpha1.Component) {
			comp.Spec.ServiceRefs = []appsv1alpha1.ServiceRef{
				{Name: randomStr()}, // set a non-existed reference.
			}
		})()).Should(Succeed())
		checkWorkloadGenerationAndToolsImage(Consistently, initWorkloadGeneration, 1, 0)

		By("update replicas to trigger component spec and workload reconcile")
		Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *appsv1alpha1.Component) {
			comp.Spec.Replicas += 1
		})()).Should(Succeed())
		checkWorkloadGenerationAndToolsImage(Eventually, initWorkloadGeneration+1, 0, 1)
	}

	Context("component resources provisioning", func() {
		BeforeEach(func() {
			createAllWorkloadTypesClusterDef()
			createBackupPolicyTpl(clusterDefObj)
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("component finalizers and labels", func() {
			testCompFinalizerNLabel(defaultCompName, compDefName)
		})

		It("with component services", func() {
			testCompService(defaultCompName, compDefName)
		})

		It("with component system accounts", func() {
			testCompSystemAccount(defaultCompName, compDefName)
		})

		It("with component conn credentials", func() {
			testCompConnCredential(defaultCompName, compDefName)
		})

		It("with component roles", func() {
			testCompRole(defaultCompName, compDefName)
		})

		It("with component TlS", func() {
			testCompTLSConfig(defaultCompName, compDefName)
		})

		It("with component configurations", func() {
			testCompConfiguration(defaultCompName, compDefName)
		})

		It("with component affinity and toleration set", func() {
			testCompAffinityNToleration(defaultCompName, compDefName)
		})

		It("with component RBAC set", func() {
			testCompRBAC(defaultCompName, compDefName, "")
		})

		It("re-create component with RBAC set", func() {
			testRecreateCompWithRBAC(defaultCompName, compDefName)
		})
	})

	Context("when creating cluster with multiple kinds of components", func() {
		BeforeEach(func() {
			cleanEnv()
			createAllWorkloadTypesClusterDef()
			createBackupPolicyTpl(clusterDefObj)
		})

		createNWaitClusterObj := func(components map[string]string,
			addedComponentProcessor func(compName string, factory *testapps.MockClusterFactory),
			withFixedName ...bool) {
			Expect(components).ShouldNot(BeEmpty())

			By("Creating a cluster")
			clusterBuilder := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name)

			compNames := make([]string, 0, len(components))
			for compName, compDefName := range components {
				clusterBuilder = clusterBuilder.AddComponent(compName, compDefName)
				if addedComponentProcessor != nil {
					addedComponentProcessor(compName, clusterBuilder)
				}
				compNames = append(compNames, compName)
			}
			if len(withFixedName) == 0 || !withFixedName[0] {
				clusterBuilder.WithRandomName()
			}
			clusterObj = clusterBuilder.Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey, compNames...)
		}

		testMultiCompHScale := func(policyType appsv1alpha1.HScaleDataClonePolicyType) {
			compNameNDef := map[string]string{
				statefulCompName:    statefulCompDefName,
				consensusCompName:   consensusCompDefName,
				replicationCompName: replicationCompDefName,
			}
			initialReplicas := int32(1)
			updatedReplicas := int32(3)

			By("Creating a multi components cluster with VolumeClaimTemplate")
			pvcSpec := testapps.NewPVCSpec("1Gi")

			createNWaitClusterObj(compNameNDef, func(compName string, factory *testapps.MockClusterFactory) {
				factory.AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).SetReplicas(initialReplicas)
			}, false)

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey, statefulCompName, consensusCompName, replicationCompName)

			// statefulCompDefName not in componentDefsWithHScalePolicy, for nil backup policy test
			// REVIEW:
			//  1. this test flow, wait for running phase?
			horizontalScale(int(updatedReplicas), testk8s.DefaultStorageClassName, policyType, consensusCompDefName, replicationCompDefName)
		}

		It("h-scale with volume snapshot", func() {
			testk8s.MockEnableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)
			testMultiCompHScale(appsv1alpha1.HScaleDataClonePolicyCloneVolume)
		})

		It("h-scale with backup tool", func() {
			testk8s.MockDisableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)
			testMultiCompHScale(appsv1alpha1.HScaleDataClonePolicyCloneVolume)
		})
	})

	When("creating cluster with all workloadTypes (being Stateless|Stateful|Consensus|Replication) component", func() {
		compNameNDef := map[string]string{
			statelessCompName:   statelessCompDefName,
			statefulCompName:    statefulCompDefName,
			consensusCompName:   consensusCompDefName,
			replicationCompName: replicationCompDefName,
		}

		BeforeEach(func() {
			createAllWorkloadTypesClusterDef()
			createBackupPolicyTpl(clusterDefObj)
		})

		AfterEach(func() {
			cleanEnv()
		})

		for compName, compDefName := range compNameNDef {
			It(fmt.Sprintf("[comp: %s] should create/delete pods to match the desired replica number if updating cluster's replica number to a valid value", compName), func() {
				testChangeReplicas(compName, compDefName)
			})

			It(fmt.Sprintf("[comp: %s] update kubeblocks-tools image", compName), func() {
				testUpdateKubeBlocksToolsImage(compName, compDefName)
			})
		}
	})

	When("creating cluster with stateful workloadTypes (being Stateful|Consensus|Replication) component", func() {
		var (
			mockStorageClass *storagev1.StorageClass
		)

		compNameNDef := map[string]string{
			statefulCompName:    statefulCompDefName,
			consensusCompName:   consensusCompDefName,
			replicationCompName: replicationCompDefName,
		}

		BeforeEach(func() {
			createAllWorkloadTypesClusterDef()
			createBackupPolicyTpl(clusterDefObj)
			mockStorageClass = testk8s.CreateMockStorageClass(&testCtx, testk8s.DefaultStorageClassName)
		})

		for compName, compDefName := range compNameNDef {
			Context(fmt.Sprintf("[comp: %s] volume expansion", compName), func() {
				It("should update PVC request storage size accordingly", func() {
					testVolumeExpansion(compName, compDefName, mockStorageClass)
				})

				It("should be able to recover if volume expansion fails", func() {
					testVolumeExpansionFailedAndRecover(compName, compDefName)
				})
			})

			Context(fmt.Sprintf("[comp: %s] horizontal scale", compName), func() {
				It("scale-out from 1 to 3 with backup(snapshot) policy normally", func() {
					testHorizontalScale(compName, compDefName, 1, 3, appsv1alpha1.HScaleDataClonePolicyCloneVolume)
				})

				// TODO(component): events & conditions
				PIt("backup error at scale-out", func() {
					testBackupError(compName, compDefName)
				})

				It("scale-out without data clone policy", func() {
					testHorizontalScale(compName, compDefName, 1, 3, "")
				})

				It("scale-in from 3 to 1", func() {
					testHorizontalScale(compName, compDefName, 3, 1, appsv1alpha1.HScaleDataClonePolicyCloneVolume)
				})

				It("scale-in to 0 and PVCs should not been deleted", func() {
					testHorizontalScale(compName, compDefName, 3, 0, appsv1alpha1.HScaleDataClonePolicyCloneVolume)
				})

				It("scale-out from 0 and should work well", func() {
					testHorizontalScale(compName, compDefName, 0, 3, appsv1alpha1.HScaleDataClonePolicyCloneVolume)
				})
			})

			Context(fmt.Sprintf("[comp: %s] scale-out after volume expansion", compName), func() {
				It("scale-out with data clone policy", func() {
					testVolumeExpansion(compName, compDefName, mockStorageClass)
					testk8s.MockEnableVolumeSnapshot(&testCtx, mockStorageClass.Name)
					horizontalScale(5, mockStorageClass.Name, appsv1alpha1.HScaleDataClonePolicyCloneVolume, compDefName)
				})

				It("scale-out without data clone policy", func() {
					testVolumeExpansion(compName, compDefName, mockStorageClass)
					horizontalScale(5, mockStorageClass.Name, "", compDefName)
				})
			})
		}
	})

	When("creating cluster with workloadType=consensus component", func() {
		BeforeEach(func() {
			createAllWorkloadTypesClusterDef()
			createBackupPolicyTpl(clusterDefObj)
		})

		AfterEach(func() {
			cleanEnv()
		})

		// REVIEW/TODO: following test always failed at cluster.phase.observerGeneration=1 with cluster.phase.phase=creating
		It("Should success with primary pod and secondary pod", func() {
			testReplicationWorkloadRunning(replicationCompName, replicationCompDefName)
		})

		It("Should success with one leader pod and two follower pods", func() {
			testThreeReplicas(consensusCompName, consensusCompDefName)
		})

		It("test restore cluster from backup", func() {
			testRestoreClusterFromBackup(consensusCompName, consensusCompDefName)
		})
	})
})

func mockRestoreCompleted(ml client.MatchingLabels) {
	restoreList := dpv1alpha1.RestoreList{}
	Expect(testCtx.Cli.List(testCtx.Ctx, &restoreList, ml)).Should(Succeed())
	for _, rs := range restoreList.Items {
		err := testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(&rs), func(res *dpv1alpha1.Restore) {
			res.Status.Phase = dpv1alpha1.RestorePhaseCompleted
		})()
		Expect(client.IgnoreNotFound(err)).ShouldNot(HaveOccurred())
	}
}

func checkRestoreAndSetCompleted(clusterKey types.NamespacedName, compName string, scaleOutReplicas int) {
	By("Checking restore CR created")
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey:    clusterKey.Name,
		constant.KBAppComponentLabelKey: compName,
		constant.KBManagedByKey:         "cluster",
	}
	Eventually(testapps.List(&testCtx, generics.RestoreSignature,
		ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(scaleOutReplicas))

	By("Mocking restore phase to succeeded")
	mockRestoreCompleted(ml)
}
