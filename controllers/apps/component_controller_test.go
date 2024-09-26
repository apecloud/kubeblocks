/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/sethvargo/go-password/password"
	"golang.org/x/exp/maps"
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
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	kbagent "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	podAnnotationKey4Test = "component-replicas-test"
)

var _ = Describe("Component Controller", func() {
	const (
		compDefName     = "test-compdef"
		compVerName     = "test-compver"
		clusterName     = "test-cluster" // this become cluster prefix name if used with testapps.NewClusterFactory().WithRandomName()
		leader          = "leader"
		follower        = "follower"
		defaultCompName = "default"
	)

	var (
		compDefObj  *kbappsv1.ComponentDefinition
		compVerObj  *kbappsv1.ComponentVersion
		clusterObj  *kbappsv1.Cluster
		clusterKey  types.NamespacedName
		compObj     *kbappsv1.Component
		compKey     types.NamespacedName
		allSettings map[string]interface{}
	)

	resetTestContext := func() {
		compDefObj = nil
		compVerObj = nil
		clusterObj = nil
		if allSettings != nil {
			Expect(viper.MergeConfigMap(allSettings)).ShouldNot(HaveOccurred())
			allSettings = nil
		}
	}

	// Cleanups
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), cluster definition
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
	createAllDefinitionObjects := func() {
		By("Create a componentDefinition obj")
		compDefObj = testapps.NewComponentDefinitionFactory(compDefName).
			WithRandomName().
			AddAnnotations(constant.SkipImmutableCheckAnnotationKey, "true").
			SetDefaultSpec().
			Create(&testCtx).
			GetObject()

		By("Create a componentVersion obj")
		compVerObj = testapps.NewComponentVersionFactory(compVerName).
			SetDefaultSpec(compDefName).
			Create(&testCtx).
			GetObject()

		By("Mock kb-agent client for the default transformer of system accounts provision")
		testapps.MockKBAgentClientDefault()
	}

	waitForCreatingResourceCompletely := func(clusterKey client.ObjectKey, compNames ...string) {
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		cluster := &kbappsv1.Cluster{}
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, cluster, true)).Should(Succeed())
		for _, compName := range compNames {
			compPhase := kbappsv1.CreatingClusterCompPhase
			for _, spec := range cluster.Spec.ComponentSpecs {
				if spec.Name == compName && spec.Replicas == 0 {
					compPhase = kbappsv1.StoppedClusterCompPhase
				}
			}
			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(compPhase))
		}
	}

	createClusterObjX := func(clusterDefName, compName, compDefName string,
		processor func(*testapps.MockClusterFactory), phase *kbappsv1.ClusterPhase) {
		factory := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName).
			WithRandomName().
			AddComponent(compName, compDefName).
			SetReplicas(1)
		if processor != nil {
			processor(factory)
		}
		clusterObj = factory.Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter expected phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		if phase == nil {
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(kbappsv1.CreatingClusterPhase))
		} else {
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(*phase))
		}

		By("Waiting for the component enter expected phase")
		compKey = types.NamespacedName{
			Namespace: clusterObj.Namespace,
			Name:      component.FullName(clusterObj.Name, compName),
		}
		compObj = &kbappsv1.Component{}
		Eventually(testapps.CheckObjExists(&testCtx, compKey, compObj, true)).Should(Succeed())
		if phase == nil {
			Eventually(testapps.GetComponentObservedGeneration(&testCtx, compKey)).Should(BeEquivalentTo(1))
			Eventually(testapps.GetComponentPhase(&testCtx, compKey)).Should(Equal(kbappsv1.CreatingClusterCompPhase))
		}
	}

	createClusterObj := func(compName, compDefName string, processor func(*testapps.MockClusterFactory)) {
		By("Creating a cluster with new component definition")
		createClusterObjX("", compName, compDefName, processor, nil)
	}

	createClusterObjWithPhase := func(compName, compDefName string, processor func(*testapps.MockClusterFactory), phase kbappsv1.ClusterPhase) {
		By("Creating a cluster with new component definition")
		createClusterObjX("", compName, compDefName, processor, &phase)
	}

	mockCompRunning := func(compName string) {
		itsList := testk8s.ListAndCheckInstanceSetWithComponent(&testCtx, client.ObjectKeyFromObject(clusterObj), compName)
		Expect(itsList.Items).Should(HaveLen(1))
		its := itsList.Items[0]
		pods := testapps.MockInstanceSetPods(&testCtx, &its, clusterObj, compName)
		Expect(testapps.ChangeObjStatus(&testCtx, &its, func() {
			testk8s.MockInstanceSetReady(&its, pods...)
		})).ShouldNot(HaveOccurred())
		Eventually(testapps.GetComponentPhase(&testCtx, types.NamespacedName{
			Namespace: clusterObj.Namespace,
			Name:      component.FullName(clusterObj.Name, compName),
		})).Should(Equal(kbappsv1.RunningClusterCompPhase))
	}

	// createCompObj := func(compName, compDefName, serviceVersion string, processor func(*testapps.MockComponentFactory)) {
	//	By("Creating a component")
	//	factory := testapps.NewComponentFactory(testCtx.DefaultNamespace, component.FullName(clusterObj.Name, compName), compDefName).
	//		AddAnnotations(constant.KBAppClusterUIDKey, string(clusterObj.UID)),
	//		AddLabels(constant.AppInstanceLabelKey, clusterObj.Name).
	//		SetServiceVersion(serviceVersion).
	//		SetReplicas(1)
	//	if processor != nil {
	//		processor(factory)
	//	}
	//	compObj = factory.Create(&testCtx).GetObject()
	//	compKey = client.ObjectKeyFromObject(compObj)
	//
	//	Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
	//		g.Expect(comp.Status.ObservedGeneration).To(BeEquivalentTo(comp.Generation))
	//		g.Expect(comp.Status.Phase).To(Equal(kbappsv1.CreatingClusterCompPhase))
	//	})).Should(Succeed())
	// }

	changeCompReplicas := func(clusterName types.NamespacedName, replicas int32, comp *kbappsv1.ClusterComponentSpec) {
		Expect(testapps.GetAndChangeObj(&testCtx, clusterName, func(cluster *kbappsv1.Cluster) {
			for i, clusterComp := range cluster.Spec.ComponentSpecs {
				if clusterComp.Name == comp.Name {
					cluster.Spec.ComponentSpecs[i].Replicas = replicas
				}
			}
		})()).ShouldNot(HaveOccurred())
	}

	changeComponentReplicas := func(clusterName types.NamespacedName, replicas int32) {
		Expect(testapps.GetAndChangeObj(&testCtx, clusterName, func(cluster *kbappsv1.Cluster) {
			Expect(cluster.Spec.ComponentSpecs).Should(HaveLen(1))
			cluster.Spec.ComponentSpecs[0].Replicas = replicas
		})()).ShouldNot(HaveOccurred())
	}

	testChangeReplicas := func(compName, compDefName string) {
		createClusterObj(compName, compDefName, nil)
		replicasSeq := []int32{5, 3, 1, 2, 4}
		expectedOG := int64(1)
		for _, replicas := range replicasSeq {
			By(fmt.Sprintf("Change replicas to %d", replicas))
			changeComponentReplicas(clusterKey, replicas)
			expectedOG++

			By("Checking cluster status and the number of replicas changed")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *kbappsv1.Cluster) {
				g.Expect(fetched.Status.ObservedGeneration).To(BeEquivalentTo(expectedOG))
				g.Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(BeElementOf(kbappsv1.CreatingClusterPhase, kbappsv1.UpdatingClusterPhase))
			})).Should(Succeed())

			itsKey := compKey
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(int(*its.Spec.Replicas)).To(BeEquivalentTo(replicas))
			})).Should(Succeed())
		}
	}

	testChangeReplicasToZero := func(compName, compDefName string) {
		var (
			init   = int32(3)
			target = int32(0)
		)

		createClusterObj(compName, compDefName, func(f *testapps.MockClusterFactory) {
			f.SetReplicas(init)
		})

		By(fmt.Sprintf("change replicas to %d", target))
		changeComponentReplicas(clusterKey, target)

		By("checking the number of replicas in component as expected")
		Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
			g.Expect(comp.Spec.Replicas).Should(Equal(target))
		})).Should(Succeed())

		By("checking the component status can't be reconciled well")
		Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
			g.Expect(comp.Generation > comp.Status.ObservedGeneration).Should(BeTrue())
		})).Should(Succeed())

		By("checking the number of replicas in ITS unchanged")
		itsKey := compKey
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			g.Expect(*its.Spec.Replicas).Should(Equal(init))
		})).Should(Succeed())
	}

	testChangeReplicasToZeroWithReplicasLimit := func(compName, compDefName string) {
		var (
			init   = int32(3)
			target = int32(0)
		)

		By("set min replicas limit to 0")
		compDefKey := client.ObjectKeyFromObject(compDefObj)
		Eventually(testapps.GetAndChangeObj(&testCtx, compDefKey, func(compDef *kbappsv1.ComponentDefinition) {
			compDef.Spec.ReplicasLimit = &kbappsv1.ReplicasLimit{
				MinReplicas: 0,
				MaxReplicas: 5,
			}
		})).Should(Succeed())

		createClusterObj(compName, compDefName, func(f *testapps.MockClusterFactory) {
			f.SetReplicas(init)
		})

		By(fmt.Sprintf("change replicas to %d", target))
		changeComponentReplicas(clusterKey, target)

		By("checking the number of replicas in component as expected")
		Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
			g.Expect(comp.Spec.Replicas).Should(Equal(target))
			g.Expect(comp.Generation).Should(Equal(comp.Status.ObservedGeneration))
		})).Should(Succeed())

		By("checking the number of replicas in ITS as expected")
		itsKey := compKey
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			g.Expect(*its.Spec.Replicas).Should(Equal(target))
		})).Should(Succeed())
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

	mockComponentPVCsAndBound := func(comp *kbappsv1.ClusterComponentSpec, replicas int, create bool, storageClassName string) {
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

	mockPodsForTest := func(cluster *kbappsv1.Cluster, number int) []corev1.Pod {
		componentName := cluster.Spec.ComponentSpecs[0].Name
		compDefName := cluster.Spec.ComponentSpecs[0].ComponentDef
		clusterName := cluster.Name
		itsName := cluster.Name + "-" + componentName
		pods := make([]corev1.Pod, 0)
		for i := 0; i < number; i++ {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      itsName + "-" + strconv.Itoa(i),
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						constant.AppManagedByLabelKey:         constant.AppName,
						constant.AppNameLabelKey:              compDefName,
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

	horizontalScaleComp := func(updatedReplicas int, comp *kbappsv1.ClusterComponentSpec, storageClassName string, bpt *string) {
		By("Mocking component PVCs to bound")
		mockComponentPVCsAndBound(comp, int(comp.Replicas), true, storageClassName)

		By("Checking its replicas right")
		itsList := testk8s.ListAndCheckInstanceSetWithComponent(&testCtx, clusterKey, comp.Name)
		Expect(int(*itsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(comp.Replicas))

		By("Creating mock pods in InstanceSet")
		pods := mockPodsForTest(clusterObj, int(comp.Replicas))
		for _, pod := range pods {
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

		checkUpdatedItsReplicas := func() {
			By("Checking updated its replicas")
			Eventually(func() int32 {
				itsList := testk8s.ListAndCheckInstanceSetWithComponent(&testCtx, clusterKey, comp.Name)
				return *itsList.Items[0].Spec.Replicas
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
			if bpt != nil {
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
					scheme, _ := kbappsv1.SchemeBuilder.Build()
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

			if bpt != nil {
				testdp.CheckRestoreAndSetCompleted(&testCtx, clusterKey, comp.Name, updatedReplicas-int(comp.Replicas))
			}

			if bpt != nil {
				By("Checking Backup and Restore cleanup")
				Eventually(testapps.List(&testCtx, generics.BackupSignature, ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(0))
				Eventually(testapps.List(&testCtx, generics.RestoreSignature, ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(0))
			}

			checkUpdatedItsReplicas()

			By("Checking updated its replicas' PVC and size")
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
					g.Expect(k8sClient.List(testCtx.Ctx, &pvcList, client.MatchingLabels{
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

			checkUpdatedItsReplicas()

			By("Checking pvcs deleting")
			Eventually(func(g Gomega) {
				pvcList := corev1.PersistentVolumeClaimList{}
				g.Expect(k8sClient.List(testCtx.Ctx, &pvcList, client.MatchingLabels{
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
				g.Expect(k8sClient.List(testCtx.Ctx, &podList, client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: comp.Name,
				})).Should(Succeed())
				for _, pod := range podList.Items {
					ss := strings.Split(pod.Name, "-")
					ordinal, _ := strconv.Atoi(ss[len(ss)-1])
					if ordinal >= updatedReplicas {
						continue
					}
					// The annotation was updated by the mocked member leave action.
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

	setHorizontalScalePolicy := func(bpt *string, compDefNames ...string) {
		bptName := "nil"
		if bpt != nil {
			bptName = *bpt
		}
		By(fmt.Sprintf("set HorizontalScalePolicy as %s", bptName))
		for _, compDefName := range compDefNames {
			Expect(testapps.GetAndChangeObj(&testCtx, types.NamespacedName{Name: compDefName},
				func(compDef *kbappsv1.ComponentDefinition) {
					if compDef.Annotations == nil {
						compDef.Annotations = map[string]string{}
					}
					if bpt == nil || len(*bpt) == 0 {
						delete(compDef.Annotations, constant.HorizontalScaleBackupPolicyTemplateKey)
					} else {
						compDef.Annotations[constant.HorizontalScaleBackupPolicyTemplateKey] = *bpt
					}
				})()).ShouldNot(HaveOccurred())
		}
	}

	horizontalScale := func(updatedReplicas int, storageClassName string, policyType *string, compDefNames ...string) {
		defer kbagent.UnsetMockClient()

		cluster := &kbappsv1.Cluster{}
		Expect(k8sClient.Get(testCtx.Ctx, clusterKey, cluster)).Should(Succeed())
		initialGeneration := int(cluster.Status.ObservedGeneration)

		setHorizontalScalePolicy(policyType, compDefNames...)

		By("Mocking all components' PVCs to bound")
		for _, comp := range cluster.Spec.ComponentSpecs {
			mockComponentPVCsAndBound(&comp, int(comp.Replicas), true, storageClassName)
		}

		bpt := func(comp kbappsv1.ClusterComponentSpec) *string {
			compDef := &kbappsv1.ComponentDefinition{}
			Expect(k8sClient.Get(testCtx.Ctx, types.NamespacedName{Name: comp.ComponentDef}, compDef)).Should(Succeed())
			if len(compDef.Annotations) > 0 {
				template, ok := compDef.Annotations[constant.HorizontalScaleBackupPolicyTemplateKey]
				if ok {
					return &template
				}
			}
			return nil
		}

		for i, comp := range cluster.Spec.ComponentSpecs {
			testapps.MockKBAgentClient4HScale(&testCtx, clusterKey, comp.Name, podAnnotationKey4Test, updatedReplicas)

			By(fmt.Sprintf("H-scale component %s with policy %v", comp.Name, bpt(comp)))
			horizontalScaleComp(updatedReplicas, &cluster.Spec.ComponentSpecs[i], storageClassName, bpt(comp))
		}

		By("Checking cluster status and the number of replicas changed")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).
			Should(BeEquivalentTo(initialGeneration + len(cluster.Spec.ComponentSpecs)))
	}

	testHorizontalScale := func(compName, compDefName string, initialReplicas, updatedReplicas int32, bpt *string) {
		By("Creating a single component cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		createClusterObj(compName, compDefName, func(f *testapps.MockClusterFactory) {
			f.SetReplicas(initialReplicas).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				AddVolumeClaimTemplate(testapps.LogVolumeName, pvcSpec)
		})

		// REVIEW: this test flow, wait for running phase?
		testk8s.MockEnableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)

		horizontalScale(int(updatedReplicas), testk8s.DefaultStorageClassName, bpt, compDefName)
	}

	testVolumeExpansion := func(compDef *kbappsv1.ComponentDefinition, compName string, storageClass *storagev1.StorageClass) {
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
		createClusterObj(compName, compDef.GetName(), func(f *testapps.MockClusterFactory) {
			f.SetReplicas(int32(replicas)).
				SetServiceVersion(compDef.Spec.ServiceVersion).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				AddVolumeClaimTemplate(testapps.LogVolumeName, pvcSpec)
		})

		By("Checking the replicas")
		itsList := testk8s.ListAndCheckInstanceSet(&testCtx, clusterKey)
		its := &itsList.Items[0]
		Expect(*its.Spec.Replicas).Should(BeEquivalentTo(replicas))

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
				patch := client.MergeFrom(pvc.DeepCopy())
				pvc.Status.Phase = corev1.ClaimBound // only bound pvc allows resize
				if pvc.Status.Capacity == nil {
					pvc.Status.Capacity = corev1.ResourceList{}
				}
				pvc.Status.Capacity[corev1.ResourceStorage] = volumeQuantity
				Expect(k8sClient.Status().Patch(testCtx.Ctx, pvc, patch)).Should(Succeed())
			}
		}

		By("mock pods of component are available")
		mockPods := testapps.MockInstanceSetPods(&testCtx, its, clusterObj, compName)
		Expect(testapps.ChangeObjStatus(&testCtx, its, func() {
			testk8s.MockInstanceSetReady(its, mockPods...)
		})).ShouldNot(HaveOccurred())

		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(kbappsv1.RunningClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(kbappsv1.RunningClusterPhase))

		By("Updating data PVC storage size")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *kbappsv1.Cluster) {
			comp := &cluster.Spec.ComponentSpecs[0]
			for i, vct := range comp.VolumeClaimTemplates {
				if vct.Name == testapps.DataVolumeName {
					comp.VolumeClaimTemplates[i].Spec.Resources.Requests[corev1.ResourceStorage] = newVolumeQuantity
				}
			}
		})()).ShouldNot(HaveOccurred())

		By("Checking the resize operation in progress for data volume")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(kbappsv1.UpdatingClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(kbappsv1.UpdatingClusterPhase))
		for i := 0; i < replicas; i++ {
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(testapps.DataVolumeName, compName, i),
			}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(testCtx.Ctx, pvcKey, pvc)).Should(Succeed())
				g.Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(newVolumeQuantity))
				g.Expect(pvc.Status.Capacity[corev1.ResourceStorage]).To(Equal(volumeQuantity))
			}).Should(Succeed())
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
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(its), func(its *workloads.InstanceSet) {
			testk8s.MockInstanceSetReady(its, mockPods...)
		})()).ShouldNot(HaveOccurred())
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(kbappsv1.RunningClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(kbappsv1.RunningClusterPhase))

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
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *kbappsv1.Cluster) {
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
		createClusterObj(compName, compDefName, nil)

		By("check component finalizers and labels")
		Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
			// g.Expect(comp.Finalizers).Should(ContainElements(constant.DBComponentFinalizerName))
			g.Expect(comp.Finalizers).Should(ContainElements(constant.DBClusterFinalizerName))
			g.Expect(comp.Labels).Should(HaveKeyWithValue(constant.AppManagedByLabelKey, constant.AppName))
			g.Expect(comp.Labels).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterObj.Name))
			g.Expect(comp.Labels).Should(HaveKeyWithValue(constant.KBAppComponentLabelKey, compName))
		})).Should(Succeed())
	}

	testCompService := func(compName, compDefName string) {
		createClusterObj(compName, compDefName, nil)

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
		createClusterObj(compName, compDefName, nil)

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
		Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
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
			g.Expect(cond.Message).ShouldNot(ContainSubstring("root"))
			g.Expect(cond.Message).Should(ContainSubstring("admin"))
		})).Should(Succeed())
	}

	testCompSystemAccountOverride := func(compName, compDefName string) {
		passwordConfig := &kbappsv1.PasswordConfig{
			Length: 29,
		}
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      "sysaccount-override",
			},
			StringData: map[string]string{
				constant.AccountPasswdForSecret: "sysaccount-override",
			},
		}
		secretRef := func() *kbappsv1.ProvisionSecretRef {
			Expect(testCtx.CreateObj(testCtx.Ctx, &secret)).Should(Succeed())
			return &kbappsv1.ProvisionSecretRef{
				Name:      secret.Name,
				Namespace: testCtx.DefaultNamespace,
			}
		}

		createClusterObj(compName, compDefName, func(f *testapps.MockClusterFactory) {
			f.AddSystemAccount("root", passwordConfig, nil).
				AddSystemAccount("admin", nil, secretRef()).
				AddSystemAccount("not-exist", nil, nil)
		})

		By("check root account")
		rootSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateAccountSecretName(clusterObj.Name, compName, "root"),
		}
		Eventually(testapps.CheckObj(&testCtx, rootSecretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("root")))
			g.Expect(secret.Data).Should(HaveKey(constant.AccountPasswdForSecret))
			g.Expect(secret.Data[constant.AccountPasswdForSecret]).Should(HaveLen(int(passwordConfig.Length)))
		})).Should(Succeed())

		By("check admin account")
		adminSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateAccountSecretName(clusterObj.Name, compName, "admin"),
		}
		Eventually(testapps.CheckObj(&testCtx, adminSecretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("admin")))
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountPasswdForSecret, secret.Data[constant.AccountPasswdForSecret]))
		})).Should(Succeed())
	}

	testCompVars := func(compName, compDefName string) {
		compDefKey := client.ObjectKeyFromObject(compDefObj)
		Eventually(testapps.GetAndChangeObj(&testCtx, compDefKey, func(compDef *kbappsv1.ComponentDefinition) {
			compDef.Spec.Vars = []kbappsv1.EnvVar{
				{
					Name: "SERVICE_HOST",
					ValueFrom: &kbappsv1.VarSource{
						ServiceVarRef: &kbappsv1.ServiceVarSelector{
							ClusterObjectReference: kbappsv1.ClusterObjectReference{
								Name: compDefObj.Spec.Services[0].Name,
							},
							ServiceVars: kbappsv1.ServiceVars{
								Host: &kbappsv1.VarRequired,
							},
						},
					},
				},
				{
					Name: "SERVICE_PORT",
					ValueFrom: &kbappsv1.VarSource{
						ServiceVarRef: &kbappsv1.ServiceVarSelector{
							ClusterObjectReference: kbappsv1.ClusterObjectReference{
								Name: compDefObj.Spec.Services[0].Name,
							},
							ServiceVars: kbappsv1.ServiceVars{
								Port: &kbappsv1.NamedVar{},
							},
						},
					},
				},
				{
					Name: "USERNAME",
					ValueFrom: &kbappsv1.VarSource{
						CredentialVarRef: &kbappsv1.CredentialVarSelector{
							ClusterObjectReference: kbappsv1.ClusterObjectReference{
								Name: compDefObj.Spec.SystemAccounts[0].Name,
							},
							CredentialVars: kbappsv1.CredentialVars{
								Username: &kbappsv1.VarRequired,
							},
						},
					},
				},
				{
					Name: "PASSWORD",
					ValueFrom: &kbappsv1.VarSource{
						CredentialVarRef: &kbappsv1.CredentialVarSelector{
							ClusterObjectReference: kbappsv1.ClusterObjectReference{
								Name: compDefObj.Spec.SystemAccounts[0].Name,
							},
							CredentialVars: kbappsv1.CredentialVars{
								Password: &kbappsv1.VarRequired,
							},
						},
					},
				},
			}
		})).Should(Succeed())
		createClusterObj(compName, compDefName, nil)

		By("check workload template env")
		targetEnvVars := []corev1.EnvVar{
			{
				Name:  "SERVICE_HOST",
				Value: constant.GenerateComponentServiceName(clusterObj.Name, compName, compDefObj.Spec.Services[0].ServiceName),
			},
			{
				Name:  "SERVICE_PORT",
				Value: strconv.Itoa(int(compDefObj.Spec.Services[0].Spec.Ports[0].Port)),
			},
			{
				Name: "USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: constant.GenerateAccountSecretName(clusterObj.Name, compName, compDefObj.Spec.SystemAccounts[0].Name),
						},
						Key: constant.AccountNameForSecret,
					},
				},
			},
			{
				Name: "PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: constant.GenerateAccountSecretName(clusterObj.Name, compName, compDefObj.Spec.SystemAccounts[0].Name),
						},
						Key: constant.AccountPasswdForSecret,
					},
				},
			},
		}
		itsKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      compObj.Name,
		}
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			envVars, _ := buildEnvVarsNData(targetEnvVars)
			targetEnvVarsMapping := map[string]corev1.EnvVar{}
			for i, v := range envVars {
				targetEnvVarsMapping[v.Name] = envVars[i]
			}
			for _, cc := range [][]corev1.Container{its.Spec.Template.Spec.InitContainers, its.Spec.Template.Spec.Containers} {
				for _, c := range cc {
					envValueMapping := map[string]corev1.EnvVar{}
					for i, env := range c.Env {
						if _, ok := targetEnvVarsMapping[env.Name]; ok {
							envValueMapping[env.Name] = c.Env[i]
						}
					}
					g.Expect(envValueMapping).Should(BeEquivalentTo(targetEnvVarsMapping))
					// check envData source
					g.Expect(c.EnvFrom).Should(ContainElement(envConfigMapSource(clusterObj.Name, compName)))
				}
			}
		})).Should(Succeed())
		envCMKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateClusterComponentEnvPattern(clusterObj.Name, compName),
		}
		Eventually(testapps.CheckObj(&testCtx, envCMKey, func(g Gomega, cm *corev1.ConfigMap) {
			_, envData := buildEnvVarsNData(targetEnvVars)
			for k, v := range envData {
				Expect(cm.Data).Should(HaveKeyWithValue(k, v))
			}
		})).Should(Succeed())
	}

	testCompReplicasLimit := func(compName, compDefName string) {
		replicasLimit := &kbappsv1.ReplicasLimit{
			MinReplicas: 4,
			MaxReplicas: 16,
		}
		By("create component w/o replicas limit set")
		createClusterObj(compName, compDefName, func(f *testapps.MockClusterFactory) {
			f.SetReplicas(replicasLimit.MaxReplicas * 2)
		})
		itsKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      compObj.Name,
		}
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			g.Expect(*its.Spec.Replicas).Should(BeEquivalentTo(replicasLimit.MaxReplicas * 2))
		})).Should(Succeed())

		By("set replicas limit")
		compDefKey := client.ObjectKeyFromObject(compDefObj)
		Eventually(testapps.GetAndChangeObj(&testCtx, compDefKey, func(compDef *kbappsv1.ComponentDefinition) {
			compDef.Spec.ReplicasLimit = replicasLimit
		})).Should(Succeed())

		By("create component w/ replicas limit set - out-of-limit")
		for _, replicas := range []int32{replicasLimit.MinReplicas / 2, replicasLimit.MaxReplicas * 2} {
			createClusterObjWithPhase(compName, compDefName, func(f *testapps.MockClusterFactory) {
				f.SetReplicas(replicas)
			}, "")
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
				g.Expect(comp.Spec.Replicas).Should(BeEquivalentTo(replicas))
				g.Expect(comp.Status.Conditions).Should(HaveLen(1))
				g.Expect(comp.Status.Conditions[0].Type).Should(BeEquivalentTo(kbappsv1.ConditionTypeProvisioningStarted))
				g.Expect(comp.Status.Conditions[0].Status).Should(BeEquivalentTo(metav1.ConditionFalse))
				g.Expect(comp.Status.Conditions[0].Message).Should(ContainSubstring(replicasOutOfLimitError(replicas, *replicasLimit).Error()))
			})).Should(Succeed())
			itsKey := types.NamespacedName{
				Namespace: compObj.Namespace,
				Name:      compObj.Name,
			}
			Consistently(testapps.CheckObjExists(&testCtx, itsKey, &workloads.InstanceSet{}, false)).Should(Succeed())
		}

		By("create component w/ replicas limit set - ok")
		for _, replicas := range []int32{replicasLimit.MinReplicas, (replicasLimit.MinReplicas + replicasLimit.MaxReplicas) / 2, replicasLimit.MaxReplicas} {
			createClusterObj(compName, compDefName, func(f *testapps.MockClusterFactory) {
				f.SetReplicas(replicas)
			})
			itsKey := types.NamespacedName{
				Namespace: compObj.Namespace,
				Name:      compObj.Name,
			}
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(*its.Spec.Replicas).Should(BeEquivalentTo(replicas))
			})).Should(Succeed())
		}
	}

	testCompRole := func(compName, compDefName string) {
		createClusterObj(compName, compDefName, nil)

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
		itsKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      compObj.Name,
		}
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			g.Expect(its.Spec.Roles).Should(HaveExactElements(targetRoles))
		})).Should(Succeed())
	}

	testCompTLSConfig := func(compName, compDefName string) {
		createClusterObj(compName, compDefName, func(f *testapps.MockClusterFactory) {
			issuer := &kbappsv1.Issuer{
				Name: kbappsv1.IssuerKubeBlocks,
			}
			f.SetTLS(true).SetIssuer(issuer)
		})

		By("check TLS secret")
		secretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      plan.GenerateTLSSecretName(clusterObj.Name, compName),
		}
		Eventually(testapps.CheckObj(&testCtx, secretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKey(constant.CAName))
			g.Expect(secret.Data).Should(HaveKey(constant.CertName))
			g.Expect(secret.Data).Should(HaveKey(constant.KeyName))
		})).Should(Succeed())

		By("check pod's volumes and mounts")
		targetVolume := corev1.Volume{
			Name: constant.VolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretKey.Name,
					Items: []corev1.KeyToPath{
						{Key: constant.CAName, Path: constant.CAName},
						{Key: constant.CertName, Path: constant.CertName},
						{Key: constant.KeyName, Path: constant.KeyName},
					},
					Optional:    func() *bool { o := false; return &o }(),
					DefaultMode: func() *int32 { m := int32(0600); return &m }(),
				},
			},
		}
		targetVolumeMount := corev1.VolumeMount{
			Name:      constant.VolumeName,
			MountPath: constant.MountPath,
			ReadOnly:  true,
		}
		itsKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      compObj.Name,
		}
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			podSpec := its.Spec.Template.Spec
			g.Expect(podSpec.Volumes).Should(ContainElements(targetVolume))
			for _, c := range podSpec.Containers {
				g.Expect(c.VolumeMounts).Should(ContainElements(targetVolumeMount))
			}
		})).Should(Succeed())
	}

	testCompConfiguration := func(compName, compDefName string) {
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
		Eventually(testapps.CheckObjExists(&testCtx, saKey, &corev1.ServiceAccount{}, expectExisted)).Should(Succeed())
		Eventually(testapps.CheckObjExists(&testCtx, rbKey, &rbacv1.RoleBinding{}, expectExisted)).Should(Succeed())
	}

	testCompRBAC := func(compName, compDefName, saName string) {
		By("update comp definition to enable lifecycle actions")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(compDefObj), func(compDef *kbappsv1.ComponentDefinition) {
			compDef.Spec.LifecycleActions.Readonly = testapps.NewLifecycleAction("readonly")
			compDef.Spec.LifecycleActions.Readwrite = testapps.NewLifecycleAction("readwrite")
		})()).Should(Succeed())

		By("creating a component with target service account name")
		if len(saName) == 0 {
			saName = "test-sa-" + randomStr()
		}
		createClusterObj(compName, compDefName, func(f *testapps.MockClusterFactory) {
			f.SetServiceAccountName(saName)
		})

		By("check the service account used in Pod")
		itsKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      compObj.Name,
		}
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			g.Expect(its.Spec.Template.Spec.ServiceAccountName).To(Equal(saName))
		})).Should(Succeed())

		By("check the RBAC resources created")
		checkRBACResourcesExistence(saName, true)
	}

	testRecreateCompWithRBACCreateByKubeBlocks := func(compName, compDefName string) {
		saName := "test-sa-" + randomStr()
		testCompRBAC(compName, compDefName, saName)

		By("delete the cluster(component)")
		testapps.DeleteObject(&testCtx, clusterKey, &kbappsv1.Cluster{})
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &kbappsv1.Cluster{}, false)).Should(Succeed())

		By("check the RBAC resources deleted")
		checkRBACResourcesExistence(saName, false)

		By("re-create cluster(component) with same name")
		testCompRBAC(compName, compDefName, saName)
	}

	tesCreateCompWithRBACCreateByUser := func(compName, compDefName string) {
		saName := "test-sa-exist" + randomStr()

		testCompRBAC(compName, compDefName, saName)

		saKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      saName,
		}
		By("mock the ServiceAccount and RoleBinding created by user by setting annotations to nil")
		Eventually(testapps.GetAndChangeObj(&testCtx, saKey, func(sa *corev1.ServiceAccount) {
			sa.Annotations = nil
		})).Should(Succeed())
		rbKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      saName,
		}
		Eventually(testapps.GetAndChangeObj(&testCtx, rbKey, func(rb *rbacv1.RoleBinding) {
			rb.Annotations = nil
		})).Should(Succeed())

		By("delete the cluster(component)")
		testapps.DeleteObject(&testCtx, clusterKey, &kbappsv1.Cluster{})
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &kbappsv1.Cluster{}, true)).Should(Succeed())

		By("check the RBAC resources deleted")
		checkRBACResourcesExistence(saName, true)
	}

	testThreeReplicas := func(compName, compDefName string) {
		const replicas = 3

		By("Mock a cluster obj")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			WithRandomName().
			AddComponent(compName, compDefName).
			SetReplicas(replicas).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		var its *workloads.InstanceSet
		Eventually(func(g Gomega) {
			itsList := testk8s.ListAndCheckInstanceSet(&testCtx, clusterKey)
			g.Expect(itsList.Items).ShouldNot(BeEmpty())
			its = &itsList.Items[0]
		}).Should(Succeed())

		By("Creating mock pods in InstanceSet, and set controller reference")
		pods := mockPodsForTest(clusterObj, replicas)
		for i, pod := range pods {
			Expect(controllerutil.SetControllerReference(its, &pod, scheme.Scheme)).Should(Succeed())
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
			pods, err := intctrlutil.GetPodListByInstanceSet(ctx, k8sClient, its)
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

		// trigger its to reconcile as the underlying its is not created
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(its), func(its *workloads.InstanceSet) {
			its.Annotations["time"] = time.Now().Format(time.RFC3339)
		})()).Should(Succeed())

		By("Updating ITS status")
		itsPatch := client.MergeFrom(its.DeepCopy())
		its.Status.UpdateRevision = "mock-version"
		pods, err := intctrlutil.GetPodListByInstanceSet(ctx, k8sClient, its)
		Expect(err).Should(BeNil())
		var podList []*corev1.Pod
		for i := range pods {
			podList = append(podList, &pods[i])
		}
		testk8s.MockInstanceSetReady(its, podList...)
		Expect(k8sClient.Status().Patch(ctx, its, itsPatch)).Should(Succeed())

		By("Checking pods' role are updated in cluster status")
		Eventually(func(g Gomega) {
			fetched := &kbappsv1.Cluster{}
			g.Expect(k8sClient.Get(ctx, clusterKey, fetched)).To(Succeed())
			compName := fetched.Spec.ComponentSpecs[0].Name
			g.Expect(fetched.Status.Components != nil).To(BeTrue())
			g.Expect(fetched.Status.Components).To(HaveKey(compName))
			_, ok := fetched.Status.Components[compName]
			g.Expect(ok).Should(BeTrue())
		}).Should(Succeed())

		By("Waiting the component be running")
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).
			Should(Equal(kbappsv1.RunningClusterCompPhase))
	}

	testRestoreClusterFromBackup := func(compName string, compDef *kbappsv1.ComponentDefinition) {
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
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			WithRandomName().
			AddComponent(compName, compDef.GetName()).
			SetServiceVersion(compDef.Spec.ServiceVersion).
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
		testdp.MockRestoreCompleted(&testCtx, ml)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		itsList := testk8s.ListAndCheckInstanceSet(&testCtx, clusterKey)
		its := &itsList.Items[0]
		By("mock pod are available and wait for component enter running phase")
		mockPods := testapps.MockInstanceSetPods(&testCtx, its, clusterObj, compName)
		Expect(testapps.ChangeObjStatus(&testCtx, its, func() {
			testk8s.MockInstanceSetReady(its, mockPods...)
		})).ShouldNot(HaveOccurred())
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(kbappsv1.RunningClusterCompPhase))

		By("the restore container has been removed from init containers")
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(its), func(g Gomega, tmpIts *workloads.InstanceSet) {
			g.Expect(tmpIts.Spec.Template.Spec.InitContainers).Should(BeEmpty())
		})).Should(Succeed())

		By("clean up annotations after cluster running")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmpCluster *kbappsv1.Cluster) {
			g.Expect(tmpCluster.Status.Phase).Should(Equal(kbappsv1.RunningClusterPhase))
			// mock postReady restore completed
			testdp.MockRestoreCompleted(&testCtx, ml)
			g.Expect(tmpCluster.Annotations[constant.RestoreFromBackupAnnotationKey]).Should(BeEmpty())
		})).Should(Succeed())
	}

	testUpdateKubeBlocksToolsImage := func(compName, compDefName string) {
		createClusterObj(compName, compDefName, nil)

		oldToolsImage := viper.GetString(constant.KBToolsImage)
		newToolsImage := fmt.Sprintf("%s-%s", oldToolsImage, rand.String(4))
		defer func() {
			viper.Set(constant.KBToolsImage, oldToolsImage)
		}()

		underlyingWorkload := func() *workloads.InstanceSet {
			itsList := testk8s.ListAndCheckInstanceSet(&testCtx, clusterKey)
			return &itsList.Items[0]
		}

		initWorkloadGeneration := underlyingWorkload().GetGeneration()
		Expect(initWorkloadGeneration).ShouldNot(Equal(0))

		checkWorkloadGenerationAndToolsImage := func(assertion func(any, ...any) AsyncAssertion,
			workloadGenerationExpected int64, oldImageCntExpected, newImageCntExpected int) {
			assertion(func(g Gomega) {
				its := underlyingWorkload()
				g.Expect(its.Generation).Should(Equal(workloadGenerationExpected))
				oldImageCnt := 0
				newImageCnt := 0
				for _, c := range its.Spec.Template.Spec.Containers {
					if c.Image == oldToolsImage {
						oldImageCnt += 1
					}
					if c.Image == newToolsImage {
						newImageCnt += 1
					}
				}
				g.Expect(oldImageCnt + newImageCnt).Should(Equal(oldImageCntExpected + newImageCntExpected))
				g.Expect(oldImageCnt).Should(Equal(oldImageCntExpected))
				g.Expect(newImageCnt).Should(Equal(newImageCntExpected))
			}).Should(Succeed())
		}

		By("check the workload generation as init")
		checkWorkloadGenerationAndToolsImage(Consistently, initWorkloadGeneration, 1, 0)

		By("update kubeblocks tools image")
		viper.Set(constant.KBToolsImage, newToolsImage)

		By("update component annotation to trigger component status reconcile")
		Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
			comp.Annotations = map[string]string{"time": time.Now().Format(time.RFC3339)}
		})()).Should(Succeed())
		checkWorkloadGenerationAndToolsImage(Consistently, initWorkloadGeneration, 1, 0)

		By("update spec to trigger component spec reconcile, but workload not changed")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *kbappsv1.Cluster) {
			cluster.Spec.ComponentSpecs[0].ServiceRefs = []kbappsv1.ServiceRef{
				{Name: randomStr()}, // set a non-existed reference.
			}
		})()).Should(Succeed())
		checkWorkloadGenerationAndToolsImage(Consistently, initWorkloadGeneration, 1, 0)

		By("update replicas to trigger component spec and workload reconcile")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *kbappsv1.Cluster) {
			cluster.Spec.ComponentSpecs[0].Replicas += 1
		})()).Should(Succeed())
		checkWorkloadGenerationAndToolsImage(Eventually, initWorkloadGeneration+1, 0, 1)
	}

	Context("provisioning", func() {
		BeforeEach(func() {
			createAllDefinitionObjects()
			testdp.CreateBackupPolicyTpl(&testCtx, compDefObj.GetName())
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("component finalizers and labels", func() {
			testCompFinalizerNLabel(defaultCompName, compDefName)
		})

		It("with component zero replicas", func() {
			zeroReplicas := func(f *testapps.MockClusterFactory) { f.SetReplicas(0) }
			phase := kbappsv1.ClusterPhase("")
			createClusterObjX("", defaultCompName, compDefName, zeroReplicas, &phase)

			By("checking the component status can't be reconciled well")
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
				g.Expect(comp.Generation > comp.Status.ObservedGeneration).Should(BeTrue())
			})).Should(Succeed())
		})

		It("with component services", func() {
			testCompService(defaultCompName, compDefName)
		})

		It("with component system accounts", func() {
			testCompSystemAccount(defaultCompName, compDefName)
		})

		It("with component system accounts - override", func() {
			testCompSystemAccountOverride(defaultCompName, compDefName)
		})

		It("with component vars", func() {
			testCompVars(defaultCompName, compDefName)
		})

		It("with component replicas limit", func() {
			testCompReplicasLimit(defaultCompName, compDefName)
		})

		It("with component roles", func() {
			testCompRole(defaultCompName, compDefName)
		})

		It("with component roles - should success with one leader pod and two follower pods", func() {
			testThreeReplicas(defaultCompName, compDefObj.Name)
		})

		It("with component TlS", func() {
			testCompTLSConfig(defaultCompName, compDefName)
		})

		It("with component configurations", func() {
			testCompConfiguration(defaultCompName, compDefName)
		})

		It("with component RBAC set", func() {
			testCompRBAC(defaultCompName, compDefName, "")
		})

		It("re-create component with custom RBAC which is not exist and auto created by KubeBlocks", func() {
			testRecreateCompWithRBACCreateByKubeBlocks(defaultCompName, compDefName)
		})

		It("create component with custom RBAC which is already exist created by User", func() {
			tesCreateCompWithRBACCreateByUser(defaultCompName, compDefName)
		})

		It("update kubeblocks-tools image", func() {
			testUpdateKubeBlocksToolsImage(defaultCompName, compDefName)
		})
	})

	Context("h-scaling", func() {
		BeforeEach(func() {
			createAllDefinitionObjects()
			testdp.CreateBackupPolicyTpl(&testCtx, compDefObj.GetName())
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("should create/delete pods to match the desired replica number", func() {
			testChangeReplicas(defaultCompName, compDefObj.Name)
		})

		It("scale-in to 0", func() {
			testChangeReplicasToZero(defaultCompName, compDefObj.Name)
		})

		It("scale-in to 0 w/ min replicas limit as 0", func() {
			testChangeReplicasToZeroWithReplicasLimit(defaultCompName, compDefObj.Name)
		})

		It("scale-out from 1 to 3 with backup(snapshot) policy normally", func() {
			testHorizontalScale(defaultCompName, compDefObj.Name, 1, 3, pointer.String(testdp.BackupPolicyTPLName))
		})

		It("scale-out without data clone policy", func() {
			testHorizontalScale(defaultCompName, compDefObj.Name, 1, 3, nil)
		})

		It("scale-in from 3 to 1", func() {
			testHorizontalScale(defaultCompName, compDefObj.Name, 3, 1, pointer.String(testdp.BackupPolicyTPLName))
		})

		It("scale-in to 0 and PVCs should not been deleted", func() {
			testHorizontalScale(defaultCompName, compDefObj.Name, 3, 0, pointer.String(testdp.BackupPolicyTPLName))
		})

		Context("with different backup methods", func() {
			createNWaitClusterObj := func(components map[string]string,
				processor func(compName string, factory *testapps.MockClusterFactory),
				withFixedName ...bool) {
				Expect(components).ShouldNot(BeEmpty())

				By("Creating a cluster")
				clusterBuilder := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "")

				compNames := make([]string, 0, len(components))
				for compName, compDefName := range components {
					clusterBuilder = clusterBuilder.AddComponent(compName, compDefName)
					if processor != nil {
						processor(compName, clusterBuilder)
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

			testMultiCompHScale := func(bpt *string) {
				compNameNDef := map[string]string{
					fmt.Sprintf("%s-0", defaultCompName): compDefObj.Name,
					fmt.Sprintf("%s-1", defaultCompName): compDefObj.Name,
					fmt.Sprintf("%s-2", defaultCompName): compDefObj.Name,
				}
				initialReplicas := int32(1)
				updatedReplicas := int32(2)

				By("Creating a multi components cluster with VolumeClaimTemplate")
				pvcSpec := testapps.NewPVCSpec("1Gi")

				createNWaitClusterObj(compNameNDef, func(compName string, factory *testapps.MockClusterFactory) {
					factory.AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).SetReplicas(initialReplicas)
				}, false)

				horizontalScale(int(updatedReplicas), testk8s.DefaultStorageClassName, bpt, compDefObj.Name)
			}

			It("h-scale with volume snapshot", func() {
				testk8s.MockEnableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)
				testMultiCompHScale(pointer.String(testdp.BackupPolicyTPLName))
			})

			It("h-scale with backup tool", func() {
				testk8s.MockDisableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)
				testMultiCompHScale(pointer.String(testdp.BackupPolicyTPLName))
			})
		})
	})

	Context("volume expansion", func() {
		var (
			mockStorageClass *storagev1.StorageClass
		)

		BeforeEach(func() {
			createAllDefinitionObjects()
			testdp.CreateBackupPolicyTpl(&testCtx, compDefObj.GetName())
			mockStorageClass = testk8s.CreateMockStorageClass(&testCtx, testk8s.DefaultStorageClassName)
		})

		It("should update PVC request storage size accordingly", func() {
			testVolumeExpansion(compDefObj, defaultCompName, mockStorageClass)
		})

		It("should be able to recover if volume expansion fails", func() {
			testVolumeExpansionFailedAndRecover(defaultCompName, compDefName)
		})

		It("scale-out with data clone policy", func() {
			testVolumeExpansion(compDefObj, defaultCompName, mockStorageClass)
			testk8s.MockEnableVolumeSnapshot(&testCtx, mockStorageClass.Name)
			horizontalScale(5, mockStorageClass.Name, pointer.String(testdp.BackupPolicyTPLName), compDefObj.Name)
		})

		It("scale-out without data clone policy", func() {
			testVolumeExpansion(compDefObj, defaultCompName, mockStorageClass)
			horizontalScale(5, mockStorageClass.Name, nil, compDefObj.Name)
		})
	})

	Context("restore", func() {
		BeforeEach(func() {
			createAllDefinitionObjects()
			testdp.CreateBackupPolicyTpl(&testCtx, compDefObj.GetName())
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("test restore cluster from backup", func() {
			testRestoreClusterFromBackup(defaultCompName, compDefObj)
		})
	})

	Context("start & stop", func() {
		BeforeEach(func() {
			cleanEnv()
			createAllDefinitionObjects()
		})

		startComp := func() {
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *kbappsv1.Cluster) {
				cluster.Spec.ComponentSpecs[0].Stop = nil
			})()).Should(Succeed())
		}

		stopComp := func() {
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *kbappsv1.Cluster) {
				cluster.Spec.ComponentSpecs[0].Stop = func() *bool { b := true; return &b }()
			})()).Should(Succeed())
		}

		checkCompRunningAs := func(phase kbappsv1.ClusterComponentPhase) {
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
				g.Expect(comp.Status.ObservedGeneration).To(BeEquivalentTo(comp.Generation))
				if comp.Spec.Stop != nil {
					g.Expect(*comp.Spec.Stop).Should(BeFalse())
				}
				g.Expect(comp.Status.Phase).Should(Equal(phase))
			})).Should(Succeed())

			itsKey := compKey
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(*its.Spec.Replicas).To(BeEquivalentTo(1))
			})).Should(Succeed())
		}

		checkCompCreating := func() {
			checkCompRunningAs(kbappsv1.CreatingClusterCompPhase)
		}

		checkCompRunning := func() {
			checkCompRunningAs(kbappsv1.UpdatingClusterCompPhase)
		}

		checkCompStopped := func() {
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
				g.Expect(comp.Status.ObservedGeneration).To(BeEquivalentTo(comp.Generation))
				g.Expect(comp.Spec.Stop).ShouldNot(BeNil())
				g.Expect(*comp.Spec.Stop).Should(BeTrue())
				g.Expect(comp.Status.Phase).Should(Equal(kbappsv1.StoppedClusterCompPhase))
			})).Should(Succeed())

			itsKey := compKey
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(*its.Spec.Replicas).To(BeEquivalentTo(0))
			})).Should(Succeed())
		}

		It("stop a component", func() {
			createClusterObj(defaultCompName, compDefName, nil)
			checkCompCreating()

			By("stop it")
			stopComp()
			checkCompStopped()

			By("stop it again")
			stopComp()
			checkCompStopped()
		})

		It("start a component", func() {
			createClusterObj(defaultCompName, compDefName, nil)
			checkCompCreating()

			By("start it")
			startComp()
			checkCompCreating()

			By("stop it")
			stopComp()
			checkCompStopped()

			By("start it")
			startComp()
			checkCompRunning()

			By("start it again")
			startComp()
			checkCompRunning()
		})

		It("h-scale a stopped component", func() {
			createClusterObjWithPhase(defaultCompName, compDefName, func(f *testapps.MockClusterFactory) {
				f.SetStop(func() *bool { b := true; return &b }())
			}, kbappsv1.StoppedClusterPhase)
			checkCompStopped()

			By("scale-out")
			changeCompReplicas(clusterKey, 3, &clusterObj.Spec.ComponentSpecs[0])

			By("check comp & its")
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
				g.Expect(comp.Spec.Replicas).Should(Equal(3))
				g.Expect(comp.Status.ObservedGeneration < comp.Generation).Should(BeTrue())
				g.Expect(comp.Status.Phase).Should(Equal(kbappsv1.StoppedClusterCompPhase))
			}))
			itsKey := compKey
			Consistently(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(*its.Spec.Replicas).To(BeEquivalentTo(0))
			}))

			By("start it")
			startComp()

			By("check comp & its")
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
				g.Expect(comp.Spec.Replicas).Should(Equal(3))
				g.Expect(comp.Status.ObservedGeneration).Should(Equal(comp.Generation))
				g.Expect(comp.Status.Phase).Should(Equal(kbappsv1.UpdatingClusterCompPhase))

			}))
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(*its.Spec.Replicas).To(BeEquivalentTo(3))
			}))
		})

		// TODO: stop a component in h-scaling
	})

	Context("reconcile with definition and version", func() {
		BeforeEach(func() {
			cleanEnv()
			createAllDefinitionObjects()
		})

		testImageUnchangedAfterNewReleasePublished := func(release kbappsv1.ComponentVersionRelease) {
			prevRelease := compVerObj.Spec.Releases[0]

			By("check new release")
			Expect(prevRelease.Images).Should(HaveLen(len(release.Images)))
			Expect(maps.Keys(prevRelease.Images)).Should(BeEquivalentTo(maps.Keys(release.Images)))
			Expect(maps.Values(prevRelease.Images)).ShouldNot(BeEquivalentTo(maps.Values(release.Images)))

			// createCompObj(defaultCompName, compDefName, compVerObj.Spec.Releases[0].ServiceVersion, nil)
			createClusterObj(defaultCompName, compDefName, func(f *testapps.MockClusterFactory) {
				f.SetServiceVersion(prevRelease.ServiceVersion)
			})

			By("check the labels and image in its")
			itsKey := compKey
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				// check comp-def and service-version labels
				g.Expect(its.Annotations).ShouldNot(BeEmpty())
				g.Expect(its.Annotations).Should(HaveKeyWithValue(constant.AppComponentLabelKey, compObj.Spec.CompDef))
				g.Expect(its.Annotations).Should(HaveKeyWithValue(constant.KBAppServiceVersionKey, compObj.Spec.ServiceVersion))
				// check the image
				c := its.Spec.Template.Spec.Containers[0]
				g.Expect(c.Image).To(BeEquivalentTo(prevRelease.Images[c.Name]))
			})).Should(Succeed())

			By("publish a new release")
			compVerKey := client.ObjectKeyFromObject(compVerObj)
			Expect(testapps.GetAndChangeObj(&testCtx, compVerKey, func(compVer *kbappsv1.ComponentVersion) {
				compVer.Spec.Releases = append(compVer.Spec.Releases, release)
				compVer.Spec.CompatibilityRules[0].Releases = append(compVer.Spec.CompatibilityRules[0].Releases, release.Name)
			})()).Should(Succeed())

			By("trigger component reconcile")
			now := time.Now().Format(time.RFC3339)
			Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
				comp.Annotations["now"] = now
			})()).Should(Succeed())

			By("check the labels and image in its not changed")
			Consistently(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Annotations).ShouldNot(BeEmpty())
				// check comp-def and service-version labels unchanged
				g.Expect(its.Annotations).Should(HaveKeyWithValue(constant.AppComponentLabelKey, compObj.Spec.CompDef))
				g.Expect(its.Annotations).Should(HaveKeyWithValue(constant.KBAppServiceVersionKey, compObj.Spec.ServiceVersion))
				// check the image unchanged
				c := its.Spec.Template.Spec.Containers[0]
				g.Expect(c.Image).To(BeEquivalentTo(prevRelease.Images[c.Name]))
			})).Should(Succeed())
		}

		It("publish new release with different service version", func() {
			release := kbappsv1.ComponentVersionRelease{
				Name:           "8.0.30-r2",
				ServiceVersion: "8.0.31", // different service version
				Images: map[string]string{
					testapps.DefaultMySQLContainerName: "mysql:8.0.31", // new image
				},
			}
			testImageUnchangedAfterNewReleasePublished(release)
		})

		It("publish new release with same service version", func() {
			release := kbappsv1.ComponentVersionRelease{
				Name:           "8.0.30-r2",
				ServiceVersion: "8.0.30", // same service version
				Images: map[string]string{
					testapps.DefaultMySQLContainerName: "mysql:8.0.31", // new image
				},
			}
			testImageUnchangedAfterNewReleasePublished(release)
		})
	})
})
