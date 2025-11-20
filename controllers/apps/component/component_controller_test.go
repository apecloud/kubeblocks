/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package component

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
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
		clusterName     = "test-cluster"
		leader          = "leader"
		follower        = "follower"
		defaultCompName = "default"
	)

	var (
		compDefObj        *kbappsv1.ComponentDefinition
		compVerObj        *kbappsv1.ComponentVersion
		clusterKey        types.NamespacedName
		clusterUID        string
		clusterGeneration int64
		compObj           *kbappsv1.Component
		compKey           types.NamespacedName
		settings          map[string]interface{}
	)

	resetTestContext := func() {
		compDefObj = nil
		compVerObj = nil
		if settings != nil {
			Expect(viper.MergeConfigMap(settings)).ShouldNot(HaveOccurred())
			settings = nil
		}
	}

	// Cleanups
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete components (and all dependent sub-resources), and component definitions & versions
		testapps.ClearComponentResources(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ServiceAccountSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.RoleSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.RoleBindingSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ConfigMapSignature, true, inNS, ml)
		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.StorageClassSignature, true, ml)

		resetTestContext()
	}

	BeforeEach(func() {
		cleanEnv()
		settings = viper.AllSettings()
	})

	AfterEach(func() {
		cleanEnv()
	})

	randomStr := func() string {
		str, _ := password.Generate(6, 0, 0, true, false)
		return str
	}

	createDefinitionObjects := func() {
		By("create a componentDefinition obj")
		compDefObj = testapps.NewComponentDefinitionFactory(compDefName).
			AddAnnotations(constant.SkipImmutableCheckAnnotationKey, "true").
			SetDefaultSpec().
			Create(&testCtx).
			GetObject()

		By("create a componentVersion obj")
		compVerObj = testapps.NewComponentVersionFactory(compVerName).
			SetDefaultSpec(compDefName).
			Create(&testCtx).
			GetObject()

		By("mock kb-agent client for the default transformer of system accounts provision")
		testapps.MockKBAgentClientDefault()
	}

	createCompObjX := func(compName, compDefName string, processor func(*testapps.MockComponentFactory), phase *kbappsv1.ComponentPhase) {
		By("randomize a cluster name and UID")
		clusterKey = types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      testapps.GetRandomizedKey("", clusterName).Name,
		}
		clusterUID = string(uuid.NewUUID())
		clusterGeneration = 1

		By("creating a component")
		compObjName := constant.GenerateClusterComponentName(clusterKey.Name, compName)
		factory := testapps.NewComponentFactory(testCtx.DefaultNamespace, compObjName, compDefName).
			AddLabels().
			AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(clusterGeneration, 10)).
			AddAnnotations(constant.CRDAPIVersionAnnotationKey, kbappsv1.GroupVersion.String()).
			AddAnnotations(constant.KBAppClusterUIDKey, clusterUID).
			AddLabelsInMap(constant.GetCompLabelsWithDef(clusterKey.Name, compName, compDefName)).
			SetReplicas(1)
		if processor != nil {
			processor(factory)
		}
		compObj = factory.Create(&testCtx).GetObject()
		compKey = client.ObjectKeyFromObject(compObj)

		By("waiting for the component enter expected phase")
		Eventually(testapps.CheckObjExists(&testCtx, compKey, compObj, true)).Should(Succeed())
		if phase == nil || *phase != "" {
			Eventually(testapps.ComponentReconciled(&testCtx, compKey)).Should(BeTrue())
		} else {
			Consistently(testapps.ComponentReconciled(&testCtx, compKey)).Should(BeFalse())
		}
		if phase == nil {
			Eventually(testapps.GetComponentPhase(&testCtx, compKey)).Should(Equal(kbappsv1.CreatingComponentPhase))
		} else if *phase != "" {
			Eventually(testapps.GetComponentPhase(&testCtx, compKey)).Should(Equal(*phase))
		}
	}

	createCompObj := func(compName, compDefName string, processor func(*testapps.MockComponentFactory)) {
		createCompObjX(compName, compDefName, processor, nil)
	}

	createCompObjWithPhase := func(compName, compDefName string, processor func(*testapps.MockComponentFactory), phase kbappsv1.ComponentPhase) {
		createCompObjX(compName, compDefName, processor, &phase)
	}

	mockCompRunning := func(compName string, comp *kbappsv1.Component) {
		itsList := testk8s.ListAndCheckInstanceSetWithComponent(&testCtx, clusterKey, compName)
		Expect(itsList.Items).Should(HaveLen(1))
		its := itsList.Items[0]
		pods := testapps.MockInstanceSetPods2(&testCtx, &its, clusterKey.Name, compName, comp)
		Expect(testapps.ChangeObjStatus(&testCtx, &its, func() {
			testk8s.MockInstanceSetReady(&its, pods...)
		})).ShouldNot(HaveOccurred())
		Eventually(testapps.GetComponentPhase(&testCtx, types.NamespacedName{
			Namespace: clusterKey.Namespace,
			Name:      component.FullName(clusterKey.Name, compName),
		})).Should(Equal(kbappsv1.RunningComponentPhase))
	}

	stableCompObservedGeneration := func(compKey types.NamespacedName, waitFor *time.Duration) (int64, *kbappsv1.Component) {
		sleepTime := 300 * time.Millisecond
		if waitFor != nil {
			sleepTime = *waitFor
		}
		time.Sleep(sleepTime)
		comp := &kbappsv1.Component{}
		Expect(testCtx.Cli.Get(testCtx.Ctx, compKey, comp)).Should(Succeed())
		return comp.Status.ObservedGeneration, comp
	}

	changeCompReplicas := func(compKey types.NamespacedName, replicas int32) {
		Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
			comp.Spec.Replicas = replicas
		})()).ShouldNot(HaveOccurred())
	}

	testChangeReplicas := func(compName, compDefName string) {
		compDefKey := client.ObjectKeyFromObject(compDefObj)
		Eventually(testapps.GetAndChangeObj(&testCtx, compDefKey, func(compDef *kbappsv1.ComponentDefinition) {
			compDef.Spec.LifecycleActions.MemberLeave = nil
		})).Should(Succeed())

		createCompObj(compName, compDefName, nil)
		expectedOG := int64(1)
		for _, replicas := range []int32{5, 3, 1, 2, 4} {
			By(fmt.Sprintf("change replicas to %d", replicas))
			changeCompReplicas(compKey, replicas)
			expectedOG++

			By("checking component status and the number of replicas changed")
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
				g.Expect(comp.Status.ObservedGeneration).To(BeEquivalentTo(expectedOG))
				g.Eventually(testapps.GetComponentPhase(&testCtx, compKey)).Should(BeElementOf(kbappsv1.CreatingComponentPhase, kbappsv1.UpdatingComponentPhase))
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

		createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
			f.SetReplicas(init).
				SetPVCRetentionPolicy(&kbappsv1.PersistentVolumeClaimRetentionPolicy{
					WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
				})
		})

		By(fmt.Sprintf("change replicas to %d", target))
		changeCompReplicas(compKey, target)

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

	changeReplicasLimit := func(compDefName string, minReplicas, maxReplicas int32) {
		By(fmt.Sprintf("set replicas limit to [%d, %d]", minReplicas, maxReplicas))
		compDefKey := types.NamespacedName{Name: compDefName}
		Eventually(testapps.GetAndChangeObj(&testCtx, compDefKey, func(compDef *kbappsv1.ComponentDefinition) {
			compDef.Spec.ReplicasLimit = &kbappsv1.ReplicasLimit{
				MinReplicas: minReplicas,
				MaxReplicas: maxReplicas,
			}
		})).Should(Succeed())
	}

	testChangeReplicasToZeroWithReplicasLimit := func(compName, compDefName string) {
		var (
			init   = int32(3)
			target = int32(0)
		)

		changeReplicasLimit(compDefName, 0, 16384)

		createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
			f.SetReplicas(init).
				SetPVCRetentionPolicy(&kbappsv1.PersistentVolumeClaimRetentionPolicy{
					WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
				})
		})

		By(fmt.Sprintf("change replicas to %d", target))
		changeCompReplicas(compKey, target)

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
		testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName,
			compName, testapps.DataVolumeName).
			AddLabelsInMap(map[string]string{
				constant.AppInstanceLabelKey:    clusterName,
				constant.KBAppComponentLabelKey: compName,
				constant.AppManagedByLabelKey:   constant.AppName,
			}).
			SetStorage(storageSize).
			SetStorageClass(storageClassName).
			CheckedCreate(&testCtx)
	}

	mockComponentPVCsAndBound := func(comp *kbappsv1.Component, compName string, replicas int, create bool, storageClassName string) {
		for i := 0; i < replicas; i++ {
			for _, vct := range comp.Spec.VolumeClaimTemplates {
				pvcKey := types.NamespacedName{
					Namespace: clusterKey.Namespace,
					Name:      getPVCName(vct.Name, compName, i),
				}
				if create {
					createPVC(clusterKey.Name, pvcKey.Name, compName, vct.Spec.Resources.Requests.Storage().String(), storageClassName)
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

	mockPodsForTest := func(clusterName, compName, compDefName string, number int) []*corev1.Pod {
		itsName := clusterName + "-" + compName
		pods := make([]*corev1.Pod, 0)
		for i := 0; i < number; i++ {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      itsName + "-" + strconv.Itoa(i),
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						constant.AppManagedByLabelKey:         constant.AppName,
						constant.AppNameLabelKey:              compDefName,
						constant.AppInstanceLabelKey:          clusterName,
						constant.KBAppComponentLabelKey:       compName,
						appsv1.ControllerRevisionHashLabelKey: "mock-version",
					},
					Annotations: map[string]string{
						podAnnotationKey4Test: fmt.Sprintf("%d", number),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "mock-container",
							Image: "mock-image",
						},
						testapps.MockKBAgentContainer(),
					},
				},
			}
			pods = append(pods, pod)
		}
		return pods
	}

	horizontalScaleComp := func(updatedReplicas int, comp *kbappsv1.Component, compName, storageClassName string) {
		By("Mocking component PVCs to bound")
		mockComponentPVCsAndBound(comp, compName, int(comp.Spec.Replicas), true, storageClassName)

		By("Checking its replicas right")
		itsList := testk8s.ListAndCheckInstanceSetWithComponent(&testCtx, clusterKey, compName)
		Expect(int(*itsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(comp.Spec.Replicas))

		By("Creating mock pods in InstanceSet")
		pods := mockPodsForTest(clusterKey.Name, compName, comp.Spec.CompDef, int(comp.Spec.Replicas))
		for i := range pods {
			if i == 0 {
				pods[i].Labels[constant.RoleLabelKey] = leader
			} else {
				pods[i].Labels[constant.RoleLabelKey] = follower
			}
			pods[i].Status.Conditions = []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}}
			Expect(testCtx.CheckedCreateObj(testCtx.Ctx, pods[i])).Should(Succeed())
		}
		Expect(testapps.ChangeObjStatus(&testCtx, &itsList.Items[0], func() {
			testk8s.MockInstanceSetReady(&itsList.Items[0], pods...)
		})).ShouldNot(HaveOccurred())

		By("Waiting for the component enter Running phase")
		Eventually(testapps.GetComponentPhase(&testCtx, compKey)).Should(Equal(kbappsv1.RunningComponentPhase))

		By(fmt.Sprintf("Changing replicas to %d", updatedReplicas))
		changeCompReplicas(compKey, int32(updatedReplicas))

		checkUpdatedItsReplicas := func() {
			By("Checking updated its replicas")
			Eventually(func() int32 {
				itsList := testk8s.ListAndCheckInstanceSetWithComponent(&testCtx, clusterKey, compName)
				return *itsList.Items[0].Spec.Replicas
			}).Should(BeEquivalentTo(updatedReplicas))
		}

		scaleOutCheck := func() {
			if comp.Spec.Replicas == 0 {
				return
			}

			By("Mock PVCs and set status to bound")
			mockComponentPVCsAndBound(comp, compName, updatedReplicas, true, storageClassName)

			checkUpdatedItsReplicas()

			By("Checking updated its replicas' PVC and size")
			for _, vct := range comp.Spec.VolumeClaimTemplates {
				var volumeQuantity resource.Quantity
				for i := 0; i < updatedReplicas; i++ {
					pvcKey := types.NamespacedName{
						Namespace: clusterKey.Namespace,
						Name:      getPVCName(vct.Name, compName, i),
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
			checkUpdatedItsReplicas()

			By("Checking pod's annotation should be updated consistently")
			Eventually(func(g Gomega) {
				podList := corev1.PodList{}
				g.Expect(k8sClient.List(testCtx.Ctx, &podList, client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: compName,
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

		if int(comp.Spec.Replicas) < updatedReplicas {
			scaleOutCheck()
		}
		if int(comp.Spec.Replicas) > updatedReplicas {
			scaleInCheck()
		}
	}

	horizontalScale := func(updatedReplicas int, storageClassName, compName string, compDefNames ...string) {
		defer kbacli.UnsetMockClient()

		initialGeneration, comp := stableCompObservedGeneration(compKey, nil)

		By("mock all component PVCs to bound")
		mockComponentPVCsAndBound(comp, compName, int(comp.Spec.Replicas), true, storageClassName)

		By("mock kb-agent for h-scale")
		testapps.MockKBAgentClient4HScale(&testCtx, clusterKey, compName, podAnnotationKey4Test, updatedReplicas)

		By(fmt.Sprintf("h-scale component %s", compName))
		horizontalScaleComp(updatedReplicas, comp, compName, storageClassName)

		By("check component status and the number of replicas changed")
		Eventually(testapps.GetComponentObservedGeneration(&testCtx, compKey)).Should(BeEquivalentTo(int(initialGeneration) + 1))
	}

	testHorizontalScale := func(compName, compDefName string, initialReplicas, updatedReplicas int32) {
		By("creating a component with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
			f.SetReplicas(initialReplicas).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				AddVolumeClaimTemplate(testapps.LogVolumeName, pvcSpec)
			if updatedReplicas == 0 {
				f.SetPVCRetentionPolicy(&kbappsv1.PersistentVolumeClaimRetentionPolicy{
					WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
				})
			}
		})
		horizontalScale(int(updatedReplicas), testk8s.DefaultStorageClassName, compName, compDefName)
	}

	testHorizontalScaleWithDataActions := func(compName, compDefName string, initialReplicas, updatedReplicas int32) {
		By("update cmpd to enable data actions")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(compDefObj), func(cmpd *kbappsv1.ComponentDefinition) {
			cmpd.Spec.LifecycleActions.DataDump = testapps.NewLifecycleAction("data-dump")
			cmpd.Spec.LifecycleActions.DataLoad = testapps.NewLifecycleAction("data-load")
		})()).Should(Succeed())

		By("creating a component with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
			f.SetReplicas(initialReplicas).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec)
		})

		horizontalScale(int(updatedReplicas), testk8s.DefaultStorageClassName, compName, compDefName)
	}

	testVolumeExpansion := func(compName, compDefName string) {
		var (
			replicas          = 3
			volumeSize        = "1Gi"
			newVolumeSize     = "2Gi"
			volumeQuantity    = resource.MustParse(volumeSize)
			newVolumeQuantity = resource.MustParse(newVolumeSize)
		)

		By("create a component with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec(volumeSize)
		createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
			f.SetReplicas(int32(replicas)).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				AddVolumeClaimTemplate(testapps.LogVolumeName, pvcSpec)
		})

		By("check the PVC in workload")
		itsKey := compKey
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			g.Expect(len(its.Spec.VolumeClaimTemplates)).Should(BeEquivalentTo(2))
			for _, vct := range its.Spec.VolumeClaimTemplates {
				g.Expect(vct.Spec.Resources.Requests[corev1.ResourceStorage]).Should(Equal(volumeQuantity))
			}
		})).Should(Succeed())

		By("update the data PVC size")
		Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
			for i, vct := range comp.Spec.VolumeClaimTemplates {
				if vct.Name == testapps.DataVolumeName {
					comp.Spec.VolumeClaimTemplates[i].Spec.Resources.Requests[corev1.ResourceStorage] = newVolumeQuantity
				}
			}
		})()).ShouldNot(HaveOccurred())

		By("check the PVC size in workload")
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			g.Expect(len(its.Spec.VolumeClaimTemplates)).Should(BeEquivalentTo(2))
			for _, vct := range its.Spec.VolumeClaimTemplates {
				if vct.Name == testapps.DataVolumeName {
					g.Expect(vct.Spec.Resources.Requests[corev1.ResourceStorage]).Should(Equal(newVolumeQuantity))
				} else {
					g.Expect(vct.Spec.Resources.Requests[corev1.ResourceStorage]).Should(Equal(volumeQuantity))
				}
			}
		})).Should(Succeed())
	}

	testCompFinalizerNLabel := func(compName, compDefName string) {
		createCompObj(compName, compDefName, nil)

		By("check component finalizers and labels")
		Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
			g.Expect(comp.Finalizers).Should(ContainElements(constant.DBComponentFinalizerName))
			g.Expect(comp.Labels).Should(HaveKeyWithValue(constant.AppManagedByLabelKey, constant.AppName))
			g.Expect(comp.Labels).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterKey.Name))
			g.Expect(comp.Labels).Should(HaveKeyWithValue(constant.KBAppComponentLabelKey, compName))
		})).Should(Succeed())
	}

	testCompService := func(compName, compDefName string) {
		createCompObj(compName, compDefName, nil)

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
			Name:      constant.GenerateComponentServiceName(clusterKey.Name, compName, "rw"),
		}
		Eventually(testapps.CheckObj(&testCtx, rwSvcKey, func(g Gomega, svc *corev1.Service) {
			g.Expect(svc.Spec.Ports).Should(ContainElements(targetPort))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.AppManagedByLabelKey, constant.AppName))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterKey.Name))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.KBAppComponentLabelKey, compName))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.RoleLabelKey, "leader"))

		})).Should(Succeed())

		By("check ro component services")
		roSvcKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateComponentServiceName(clusterKey.Name, compName, "ro"),
		}
		Eventually(testapps.CheckObj(&testCtx, roSvcKey, func(g Gomega, svc *corev1.Service) {
			g.Expect(svc.Spec.Ports).Should(ContainElements(targetPort))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.AppManagedByLabelKey, constant.AppName))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterKey.Name))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.KBAppComponentLabelKey, compName))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.RoleLabelKey, "follower"))
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
		createCompObj(compName, compDefName, nil)

		By("check workload template env")
		targetEnvVars := []corev1.EnvVar{
			{
				Name:  "SERVICE_HOST",
				Value: constant.GenerateComponentServiceName(clusterKey.Name, compName, compDefObj.Spec.Services[0].ServiceName),
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
							Name: constant.GenerateAccountSecretName(clusterKey.Name, compName, compDefObj.Spec.SystemAccounts[0].Name),
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
							Name: constant.GenerateAccountSecretName(clusterKey.Name, compName, compDefObj.Spec.SystemAccounts[0].Name),
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
					g.Expect(c.EnvFrom).Should(ContainElement(envConfigMapSource(clusterKey.Name, compName)))
				}
			}
		})).Should(Succeed())
		envCMKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateClusterComponentEnvPattern(clusterKey.Name, compName),
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
		createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
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
			createCompObjWithPhase(compName, compDefName, func(f *testapps.MockComponentFactory) {
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
			createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
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

	testCompRoles := func(compName, compDefName string) {
		createCompObj(compName, compDefName, nil)

		By("check default component roles")
		targetRoles := []workloads.ReplicaRole{
			{
				Name:                 "leader",
				ParticipatesInQuorum: true,
				UpdatePriority:       5,
			},
			{
				Name:                 "follower",
				ParticipatesInQuorum: true,
				UpdatePriority:       4,
			},
			{
				Name:                 "learner",
				ParticipatesInQuorum: false,
				UpdatePriority:       2,
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
		tls := kbappsv1.TLS{
			VolumeName:  "tls",
			MountPath:   "/etc/pki/tls",
			DefaultMode: ptr.To(int32(0600)),
			CAFile:      ptr.To("ca.pem"),
			CertFile:    ptr.To("cert.pem"),
			KeyFile:     ptr.To("key.pem"),
		}

		By("update comp definition to set the TLS")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(compDefObj), func(compDef *kbappsv1.ComponentDefinition) {
			compDef.Spec.TLS = &tls
		})()).Should(Succeed())

		createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
			issuer := &kbappsv1.Issuer{
				Name: kbappsv1.IssuerKubeBlocks,
			}
			f.SetTLSConfig(true, issuer)
		})

		By("check TLS secret")
		secretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      tlsSecretName(clusterKey.Name, compName),
		}
		Eventually(testapps.CheckObj(&testCtx, secretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKey(*tls.CAFile))
			g.Expect(secret.Data).Should(HaveKey(*tls.CertFile))
			g.Expect(secret.Data).Should(HaveKey(*tls.KeyFile))
		})).Should(Succeed())

		By("check pod's volumes and mounts")
		targetVolume := corev1.Volume{
			Name: tls.VolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secretKey.Name,
					Optional:    ptr.To(false),
					DefaultMode: tls.DefaultMode,
				},
			},
		}
		targetVolumeMount := corev1.VolumeMount{
			Name:      tls.VolumeName,
			MountPath: tls.MountPath,
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

	checkRBACResourcesExistence := func(saName, rbName string, expectExisted bool) {
		saKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      saName,
		}
		rbKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      rbName,
		}
		Eventually(testapps.CheckObjExists(&testCtx, saKey, &corev1.ServiceAccount{}, expectExisted)).Should(Succeed())
		Eventually(testapps.CheckObjExists(&testCtx, rbKey, &rbacv1.RoleBinding{}, expectExisted)).Should(Succeed())
	}

	checkRBACResourceOrphaned := func(saName, rbName string, orphaned bool) {
		saKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      saName,
		}
		rbKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      rbName,
		}
		Eventually(testapps.CheckObj(&testCtx, saKey, func(g Gomega, sa *corev1.ServiceAccount) {
			if orphaned {
				g.Expect(metav1.GetControllerOf(sa)).Should(BeNil())
			} else {
				g.Expect(metav1.GetControllerOf(sa)).ShouldNot(BeNil())
			}
		})).Should(Succeed())
		Eventually(testapps.CheckObj(&testCtx, rbKey, func(g Gomega, rb *rbacv1.RoleBinding) {
			if orphaned {
				g.Expect(metav1.GetControllerOf(rb)).Should(BeNil())
			} else {
				g.Expect(metav1.GetControllerOf(rb)).ShouldNot(BeNil())
			}
		})).Should(Succeed())
	}

	testCompRBAC := func(compName, compDefName, saName string) {
		By("creating a component with target service account name")
		if len(saName) == 0 {
			createCompObj(compName, compDefName, nil)
			saName = constant.GenerateDefaultServiceAccountName(compDefName)
		} else {
			createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
				f.SetServiceAccountName(saName)
			})
		}

		By("check the service account used in Pod")
		itsKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      compObj.Name,
		}
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			g.Expect(its.Spec.Template.Spec.ServiceAccountName).To(Equal(saName))
		})).Should(Succeed())

		By("check the RBAC resources status")
		checkRBACResourcesExistence(saName, fmt.Sprintf("%v-pod", saName), true)
	}

	testCompWithRBAC := func(compName, compDefName string) {
		testCompRBAC(compName, compDefName, "")
		By("delete the component")
		testapps.DeleteObject(&testCtx, compKey, &kbappsv1.Component{})
		Eventually(testapps.CheckObjExists(&testCtx, compKey, &kbappsv1.Component{}, false)).Should(Succeed())

		By("check the RBAC resources orphaned")
		saName := constant.GenerateDefaultServiceAccountName(compDefName)
		checkRBACResourceOrphaned(saName, fmt.Sprintf("%v-pod", saName), true)
	}

	testRecreateCompWithRBACCreateByKubeBlocks := func(compName, compDefName string) {
		testCompRBAC(compName, compDefName, "")

		By("delete the component")
		testapps.DeleteObject(&testCtx, compKey, &kbappsv1.Component{})
		Eventually(testapps.CheckObjExists(&testCtx, compKey, &kbappsv1.Component{}, false)).Should(Succeed())

		By("check the RBAC resources deleted")
		saName := constant.GenerateDefaultServiceAccountName(compDefName)
		checkRBACResourceOrphaned(saName, fmt.Sprintf("%v-pod", saName), true)

		By("re-create component with same name")
		testCompRBAC(compName, compDefName, "")
		checkRBACResourceOrphaned(saName, fmt.Sprintf("%v-pod", saName), false)
	}

	testSharedRBACResourceDeletion := func(compNamePrefix, compDefName string) {
		By("create first component")
		createCompObj(compNamePrefix+"-comp1", compDefName, nil)
		comp1Key := compKey

		By("check rbac resources owner")
		saName := constant.GenerateDefaultServiceAccountName(compDefName)
		saKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      saName,
		}
		Eventually(testapps.CheckObj(&testCtx, saKey, func(g Gomega, sa *corev1.ServiceAccount) {
			refs := sa.GetOwnerReferences()
			g.Expect(refs).Should(HaveLen(1))
			owner := refs[0]
			g.Expect(owner.Name).Should(Equal(comp1Key.Name))
		})).Should(Succeed())

		checkRBACResourcesExistence(saName, fmt.Sprintf("%v-pod", saName), true)

		By("create second cluster")
		createCompObj(compNamePrefix+"-comp2", compDefName, nil)
		comp2Key := compKey
		By("check rbac resources owner not modified")
		Consistently(testapps.CheckObj(&testCtx, saKey, func(g Gomega, sa *corev1.ServiceAccount) {
			refs := sa.GetOwnerReferences()
			g.Expect(refs).Should(HaveLen(1))
			owner := refs[0]
			g.Expect(owner.Name).Should(Equal(comp1Key.Name))
		})).Should(Succeed())

		By("delete first component")
		testapps.DeleteObject(&testCtx, comp1Key, &kbappsv1.Component{})
		Eventually(testapps.CheckObjExists(&testCtx, comp1Key, &kbappsv1.Component{}, false)).Should(Succeed())
		checkRBACResourceOrphaned(saName, fmt.Sprintf("%v-pod", saName), true)

		By("trigger reconcile")
		Expect(testapps.GetAndChangeObj(&testCtx, comp2Key, func(comp *kbappsv1.Component) {
			if comp.Annotations == nil {
				comp.Annotations = map[string]string{}
			}
			comp.Annotations["reconcile"] = time.Now().String()
		})()).Should(Succeed())
		By("check rbac resources adopted")
		checkRBACResourceOrphaned(saName, fmt.Sprintf("%v-pod", saName), false)
		Eventually(testapps.CheckObj(&testCtx, saKey, func(g Gomega, sa *corev1.ServiceAccount) {
			refs := sa.GetOwnerReferences()
			g.Expect(refs).Should(HaveLen(1))
			owner := refs[0]
			g.Expect(owner.Name).Should(Equal(comp2Key.Name))
		})).Should(Succeed())
	}

	testCreateCompWithNonExistRBAC := func(compName, compDefName string) {
		saName := "test-sa-non-exist" + randomStr()

		// component controller won't complete reconciliation, so the phase will be empty
		createCompObjWithPhase(compName, compDefName, func(f *testapps.MockComponentFactory) {
			f.SetServiceAccountName(saName)
		}, "")
		Consistently(testapps.GetComponentPhase(&testCtx, compKey)).Should(Equal(kbappsv1.ComponentPhase("")))
	}

	testCreateCompWithRBACCreateByUser := func(compName, compDefName string) {
		saName := "test-sa-exist" + randomStr()

		By("user manually creates ServiceAccount and RoleBinding")
		sa := builder.NewServiceAccountBuilder(testCtx.DefaultNamespace, saName).GetObject()
		testapps.CheckedCreateK8sResource(&testCtx, sa)

		testCompRBAC(compName, compDefName, saName)

		By("delete the component")
		testapps.DeleteObject(&testCtx, compKey, &kbappsv1.Component{})
		Eventually(testapps.CheckObjExists(&testCtx, compKey, &kbappsv1.Component{}, true)).Should(Succeed())

		By("check the serviceaccount not deleted")
		Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(sa), &corev1.ServiceAccount{}, true)).Should(Succeed())
	}

	testCompSystemAccount := func(compName, compDefName string) {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      "sysaccount",
			},
			StringData: map[string]string{
				constant.AccountPasswdForSecret: "sysaccount",
			},
		}
		secretRef := func() *kbappsv1.ProvisionSecretRef {
			Expect(testCtx.CreateObj(testCtx.Ctx, &secret)).Should(Succeed())
			return &kbappsv1.ProvisionSecretRef{
				Name:      secret.Name,
				Namespace: testCtx.DefaultNamespace,
			}
		}

		createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
			f.AddSystemAccount("admin", nil, nil, secretRef())
		})

		By("check root account")
		var rootHashedPassword string
		rootSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateAccountSecretName(clusterKey.Name, compName, "root"),
		}
		Eventually(testapps.CheckObj(&testCtx, rootSecretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("root")))
			g.Expect(secret.Data).Should(HaveKey(constant.AccountPasswdForSecret))
			rootHashedPassword = secret.Annotations[systemAccountHashAnnotation]
			g.Expect(rootHashedPassword).Should(BeEmpty()) // kb generated password
		})).Should(Succeed())

		By("check admin account")
		var adminHashedPassword string
		adminSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateAccountSecretName(clusterKey.Name, compName, "admin"),
		}
		Eventually(testapps.CheckObj(&testCtx, adminSecretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("admin")))
			g.Expect(secret.Data).Should(HaveKey(constant.AccountPasswdForSecret))
			adminHashedPassword = secret.Annotations[systemAccountHashAnnotation]
			g.Expect(adminHashedPassword).ShouldNot(BeEmpty()) // user-provided
		})).Should(Succeed())

		By("mock component as Running")
		mockCompRunning(compName, compObj)

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
			g.Expect(cond.Message).Should(ContainSubstring(fmt.Sprintf("%s:%s", "root", rootHashedPassword)))
			g.Expect(cond.Message).Should(ContainSubstring(fmt.Sprintf("%s:%s", "admin", adminHashedPassword)))
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

		createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
			f.AddSystemAccount("root", nil, passwordConfig, nil).
				AddSystemAccount("admin", nil, nil, secretRef())
		})

		By("check root account")
		rootSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateAccountSecretName(clusterKey.Name, compName, "root"),
		}
		Eventually(testapps.CheckObj(&testCtx, rootSecretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("root")))
			g.Expect(secret.Data).Should(HaveKey(constant.AccountPasswdForSecret))
			g.Expect(secret.Data[constant.AccountPasswdForSecret]).Should(HaveLen(int(passwordConfig.Length)))
		})).Should(Succeed())

		By("check admin account")
		adminSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateAccountSecretName(clusterKey.Name, compName, "admin"),
		}
		Eventually(testapps.CheckObj(&testCtx, adminSecretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("admin")))
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountPasswdForSecret, secret.Data[constant.AccountPasswdForSecret]))
		})).Should(Succeed())
	}

	testCompSystemAccountDisable := func(compName, compDefName string) {
		passwordConfig := &kbappsv1.PasswordConfig{
			Length: 29,
		}

		createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
			f.AddSystemAccount("root", ptr.To(false), passwordConfig, nil).
				AddSystemAccount("admin", ptr.To(true), passwordConfig, nil)
		})

		By("check root account")
		rootSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateAccountSecretName(clusterKey.Name, compName, "root"),
		}
		rootSecret := &corev1.Secret{}
		Eventually(testapps.CheckObjExists(&testCtx, rootSecretKey, rootSecret, true)).Should(Succeed())

		By("check admin account")
		adminSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateAccountSecretName(clusterKey.Name, compName, "admin"),
		}
		adminSecret := &corev1.Secret{}
		Consistently(testapps.CheckObjExists(&testCtx, adminSecretKey, adminSecret, false)).Should(Succeed())
	}

	testCompSystemAccountDisableAfterProvision := func(compName, compDefName string) {
		passwordConfig := &kbappsv1.PasswordConfig{
			Length: 29,
		}

		createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
			f.AddSystemAccount("root", ptr.To(false), passwordConfig, nil).
				AddSystemAccount("admin", ptr.To(false), passwordConfig, nil)
		})

		By("check the root account")
		rootSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateAccountSecretName(clusterKey.Name, compName, "root"),
		}
		rootSecret := &corev1.Secret{}
		Eventually(testapps.CheckObjExists(&testCtx, rootSecretKey, rootSecret, true)).Should(Succeed())

		By("check the admin account")
		adminSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateAccountSecretName(clusterKey.Name, compName, "admin"),
		}
		adminSecret := &corev1.Secret{}
		Eventually(testapps.CheckObjExists(&testCtx, adminSecretKey, adminSecret, true)).Should(Succeed())

		By("disable the admin account")
		Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
			for j, account := range comp.Spec.SystemAccounts {
				if account.Name == "admin" {
					comp.Spec.SystemAccounts[j].Disabled = ptr.To(true)
				}
			}
		})()).Should(Succeed())

		By("check the admin account is disabled")
		Eventually(testapps.CheckObjExists(&testCtx, adminSecretKey, adminSecret, false)).Should(Succeed())
	}

	testCompSystemAccountUpdate := func(compName, compDefName string) {
		passwordConfig := &kbappsv1.PasswordConfig{
			Length: 29,
		}
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      "sysaccount-update",
			},
			StringData: map[string]string{
				"sysaccount-update": "sysaccount-update",
			},
		}
		secretRef := func() *kbappsv1.ProvisionSecretRef {
			Expect(testCtx.CreateObj(testCtx.Ctx, &secret)).Should(Succeed())
			return &kbappsv1.ProvisionSecretRef{
				Name:      secret.Name,
				Namespace: testCtx.DefaultNamespace,
				Password:  "sysaccount-update",
			}
		}

		createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
			f.AddSystemAccount("root", nil, passwordConfig, nil).
				AddSystemAccount("admin", nil, nil, secretRef())
		})

		By("check root account")
		var rootHashedPassword string
		rootSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateAccountSecretName(clusterKey.Name, compName, "root"),
		}
		Eventually(testapps.CheckObj(&testCtx, rootSecretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("root")))
			g.Expect(secret.Data).Should(HaveKey(constant.AccountPasswdForSecret))
			g.Expect(secret.Data[constant.AccountPasswdForSecret]).Should(HaveLen(int(passwordConfig.Length)))
			rootHashedPassword = secret.Annotations[systemAccountHashAnnotation]
			g.Expect(rootHashedPassword).Should(BeEmpty()) // kb generated password
		})).Should(Succeed())

		By("check admin account")
		var adminHashedPassword string
		adminSecretKey := types.NamespacedName{
			Namespace: compObj.Namespace,
			Name:      constant.GenerateAccountSecretName(clusterKey.Name, compName, "admin"),
		}
		Eventually(testapps.CheckObj(&testCtx, adminSecretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("admin")))
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountPasswdForSecret, []byte("sysaccount-update")))
			adminHashedPassword = secret.Annotations[systemAccountHashAnnotation]
			g.Expect(adminHashedPassword).ShouldNot(BeEmpty()) // user-provided
		})).Should(Succeed())

		By("mock component as Running")
		mockCompRunning(compName, compObj)

		By("update the password of admin account")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(&secret), func(obj *corev1.Secret) {
			if obj.StringData == nil {
				obj.StringData = map[string]string{}
			}
			obj.StringData["sysaccount-update"] = "sysaccount-update-new"
		})()).Should(Succeed())

		By("trigger the component to reconcile")
		Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
			if comp.Annotations == nil {
				comp.Annotations = map[string]string{}
			}
			comp.Annotations["reconcile"] = time.Now().String()
		})()).Should(Succeed())

		By("check the admin account updated")
		var updatedAdminHashedPassword string
		Eventually(testapps.CheckObj(&testCtx, adminSecretKey, func(g Gomega, secret *corev1.Secret) {
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("admin")))
			g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountPasswdForSecret, []byte("sysaccount-update-new")))
			updatedAdminHashedPassword = secret.Annotations[systemAccountHashAnnotation]
			g.Expect(updatedAdminHashedPassword).ShouldNot(BeEmpty()) // user-provided
			g.Expect(updatedAdminHashedPassword).ShouldNot(Equal(adminHashedPassword))
		})).Should(Succeed())

		By("wait accounts to be updated")
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
			g.Expect(cond.Message).Should(ContainSubstring(fmt.Sprintf("%s:%s", "root", rootHashedPassword)))
			g.Expect(cond.Message).Should(ContainSubstring(fmt.Sprintf("%s:%s", "admin", updatedAdminHashedPassword)))
		})).Should(Succeed())
	}

	testFileTemplateVolumes := func(compName, compDefName, fileTemplate string) {
		createCompObj(compName, compDefName, nil)

		By("mock a file template object that not defined in the cmpd")
		labels := constant.GetCompLabels(clusterKey.Name, compName)
		labels[kubeBlockFileTemplateLabelKey] = "true"
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      "test-log-conf-not-defined",
				Labels:    labels,
			},
			Data: map[string]string{},
		}
		Expect(testCtx.CreateObj(testCtx.Ctx, cm)).Should(Succeed())

		// trigger the component to reconcile
		By("update the config template variables")
		Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
			comp.Spec.Configs = []kbappsv1.ClusterComponentConfig{
				{
					Name: ptr.To(fileTemplate),
					Variables: map[string]string{
						"LOG_LEVEL": "debug",
					},
				},
			}
		})()).Should(Succeed())

		By("check the pod volumes")
		itsKey := compKey
		fileTemplateCMKey := types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      fileTemplateObjectName(&component.SynthesizedComponent{FullCompName: compKey.Name}, fileTemplate),
		}
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			expectVolume := corev1.Volume{
				Name: fileTemplate,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: fileTemplateCMKey.Name,
						},
						DefaultMode: ptr.To[int32](0444),
					},
				},
			}
			g.Expect(its.Spec.Template.Spec.Volumes).Should(ContainElement(expectVolume))
		})).Should(Succeed())

		By("check the file template objects")
		Eventually(testapps.CheckObj(&testCtx, fileTemplateCMKey, func(g Gomega, cm *corev1.ConfigMap) {
			g.Expect(cm.Data).Should(HaveKeyWithValue("level", "debug")) // updated
		})).Should(Succeed())
		// deleted
		Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{}, false)).Should(Succeed())
	}

	testReconfigureAction := func(compName, compDefName, fileTemplate string) {
		createCompObj(compName, compDefName, nil)

		By("check the file template object")
		fileTemplateCMKey := types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      fileTemplateObjectName(&component.SynthesizedComponent{FullCompName: compKey.Name}, fileTemplate),
		}
		Eventually(testapps.CheckObj(&testCtx, fileTemplateCMKey, func(g Gomega, cm *corev1.ConfigMap) {
			g.Expect(cm.Data).Should(HaveKeyWithValue("level", "info"))
		})).Should(Succeed())

		By("update the config template variables")
		Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
			comp.Spec.Configs = []kbappsv1.ClusterComponentConfig{
				{
					Name: ptr.To(fileTemplate),
					Variables: map[string]string{
						"LOG_LEVEL": "debug",
					},
				},
			}
		})()).Should(Succeed())

		By("check the file template object again")
		Eventually(testapps.CheckObj(&testCtx, fileTemplateCMKey, func(g Gomega, cm *corev1.ConfigMap) {
			g.Expect(cm.Data).Should(HaveKeyWithValue("level", "debug"))
		})).Should(Succeed())

		By("check the workload updated")
		itsKey := compKey
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			g.Expect(its.Spec.Configs).Should(HaveLen(1))
			g.Expect(its.Spec.Configs[0].Name).Should(Equal(fileTemplate))
			g.Expect(its.Spec.Configs[0].Generation).Should(Equal(its.Generation))
			g.Expect(its.Spec.Configs[0].Reconfigure).ShouldNot(BeNil())
			g.Expect(its.Spec.Configs[0].ReconfigureActionName).Should(BeEmpty())
			g.Expect(its.Spec.Configs[0].Parameters).Should(HaveKey("KB_CONFIG_FILES_UPDATED"))
			g.Expect(its.Spec.Configs[0].Parameters["KB_CONFIG_FILES_UPDATED"]).Should(ContainSubstring("level"))
		})).Should(Succeed())
	}

	testReconfigureActionUDF := func(compName, compDefName, fileTemplate string) {
		createCompObj(compName, compDefName, func(f *testapps.MockComponentFactory) {
			f.SetConfigs([]kbappsv1.ClusterComponentConfig{
				{
					Name: ptr.To(fileTemplate),
					Variables: map[string]string{
						"LOG_LEVEL": "debug",
					},
					Reconfigure: testapps.NewLifecycleAction("reconfigure"),
				},
			})
		})

		By("check the file template object")
		fileTemplateCMKey := types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      fileTemplateObjectName(&component.SynthesizedComponent{FullCompName: compKey.Name}, fileTemplate),
		}
		Eventually(testapps.CheckObj(&testCtx, fileTemplateCMKey, func(g Gomega, cm *corev1.ConfigMap) {
			g.Expect(cm.Data).Should(HaveKeyWithValue("level", "debug"))
		})).Should(Succeed())

		By("update the config template variables")
		Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
			comp.Spec.Configs[0].Variables = map[string]string{
				"LOG_LEVEL": "warn",
			}
		})()).Should(Succeed())

		By("check the file template object again")
		Eventually(testapps.CheckObj(&testCtx, fileTemplateCMKey, func(g Gomega, cm *corev1.ConfigMap) {
			g.Expect(cm.Data).Should(HaveKeyWithValue("level", "warn"))
		})).Should(Succeed())

		By("check the workload updated")
		itsKey := compKey
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			g.Expect(its.Spec.Configs).Should(HaveLen(1))
			g.Expect(its.Spec.Configs[0].Name).Should(Equal(fileTemplate))
			g.Expect(its.Spec.Configs[0].Generation).Should(Equal(its.Generation))
			g.Expect(its.Spec.Configs[0].Reconfigure).ShouldNot(BeNil())
			g.Expect(its.Spec.Configs[0].ReconfigureActionName).Should(Equal(fmt.Sprintf("reconfigure-%s", fileTemplate)))
			g.Expect(its.Spec.Configs[0].Parameters).Should(HaveKey("KB_CONFIG_FILES_UPDATED"))
			g.Expect(its.Spec.Configs[0].Parameters["KB_CONFIG_FILES_UPDATED"]).Should(ContainSubstring("level"))
		})).Should(Succeed())
	}

	testReconfigureVolumeChanged := func(compName, compDefName, fileTemplate string) {
		testReconfigureAction(compName, compDefName, fileTemplate)

		By("update the cmpd to add a new config template (volume)")
		compDefKey := client.ObjectKeyFromObject(compDefObj)
		Expect(testapps.GetAndChangeObj(&testCtx, compDefKey, func(cmpd *kbappsv1.ComponentDefinition) {
			cmpd.Spec.Configs = append(cmpd.Spec.Configs, kbappsv1.ComponentFileTemplate{
				Name:       "server-conf",
				Template:   "test-log-conf-template", // reuse log-conf template
				Namespace:  testCtx.DefaultNamespace,
				VolumeName: "server-conf",
			})
			for i := range cmpd.Spec.Runtime.Containers {
				cmpd.Spec.Runtime.Containers[i].VolumeMounts =
					append(cmpd.Spec.Runtime.Containers[i].VolumeMounts, corev1.VolumeMount{
						Name:      "server-conf",
						MountPath: "/var/run/app/conf/server",
					})
			}
		})()).ShouldNot(HaveOccurred())

		By("check new file template object")
		newFileTemplateCMKey := types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      fileTemplateObjectName(&component.SynthesizedComponent{FullCompName: compKey.Name}, "server-conf"),
		}
		Eventually(testapps.CheckObjExists(&testCtx, newFileTemplateCMKey, &corev1.ConfigMap{}, true)).Should(Succeed())

		By("check the workload updated")
		itsKey := compKey
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			g.Expect(its.Spec.Configs).Should(BeNil())
		})).Should(Succeed())
	}

	testReconfigureRestart := func(compName, compDefName, fileTemplate string) {
		// mock the cmpd to set restartOnFileChange
		compDefKey := types.NamespacedName{Name: compDefName}
		Expect(testapps.GetAndChangeObj(&testCtx, compDefKey, func(cmpd *kbappsv1.ComponentDefinition) {
			for i := range cmpd.Spec.Configs {
				cmpd.Spec.Configs[i].RestartOnFileChange = ptr.To(true)
			}
		})()).ShouldNot(HaveOccurred())

		createCompObj(compName, compDefName, nil)

		By("check the file template object")
		fileTemplateCMKey := types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      fileTemplateObjectName(&component.SynthesizedComponent{FullCompName: compKey.Name}, fileTemplate),
		}
		Eventually(testapps.CheckObj(&testCtx, fileTemplateCMKey, func(g Gomega, cm *corev1.ConfigMap) {
			g.Expect(cm.Data).Should(HaveKeyWithValue("level", "info"))
		})).Should(Succeed())

		By("update the config template variables")
		Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
			comp.Spec.Configs = []kbappsv1.ClusterComponentConfig{
				{
					Name: ptr.To(fileTemplate),
					Variables: map[string]string{
						"LOG_LEVEL": "debug",
					},
				},
			}
		})()).Should(Succeed())

		By("check the file template object again")
		Eventually(testapps.CheckObj(&testCtx, fileTemplateCMKey, func(g Gomega, cm *corev1.ConfigMap) {
			g.Expect(cm.Data).Should(HaveKeyWithValue("level", "debug"))
		})).Should(Succeed())

		By("check the workload updated")
		itsKey := compKey
		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
			g.Expect(its.Spec.Configs).Should(BeNil())
			g.Expect(its.Spec.Template.Annotations).ShouldNot(BeNil())
			g.Expect(its.Spec.Template.Annotations).Should(HaveKey(constant.RestartAnnotationKey))
		})).Should(Succeed())
	}

	Context("provisioning", func() {
		BeforeEach(func() {
			createDefinitionObjects()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("component finalizers and labels", func() {
			testCompFinalizerNLabel(defaultCompName, compDefObj.Name)
		})

		It("with component zero replicas", func() {
			createCompObjWithPhase(defaultCompName, compDefObj.Name, func(f *testapps.MockComponentFactory) {
				f.SetReplicas(0)
			}, "")

			By("checking the component status can't be reconciled well")
			Consistently(testapps.ComponentReconciled(&testCtx, compKey)).Should(BeFalse())
		})

		It("with component services", func() {
			testCompService(defaultCompName, compDefObj.Name)
		})

		It("with component vars", func() {
			testCompVars(defaultCompName, compDefObj.Name)
		})

		It("with component replicas limit", func() {
			testCompReplicasLimit(defaultCompName, compDefObj.Name)
		})

		It("with component roles", func() {
			testCompRoles(defaultCompName, compDefObj.Name)
		})

		It("with component TlS", func() {
			testCompTLSConfig(defaultCompName, compDefObj.Name)
		})

		It("creates component RBAC resources", func() {
			testCompWithRBAC(defaultCompName, compDefObj.Name)
		})

		It("re-creates component with custom RBAC which is not exist and auto created by KubeBlocks", func() {
			testRecreateCompWithRBACCreateByKubeBlocks(defaultCompName, compDefObj.Name)
		})

		It("adopts an orphaned rbac resource", func() {
			testSharedRBACResourceDeletion(defaultCompName, compDefObj.Name)
		})

		It("creates component with non-exist serviceaccount", func() {
			testCreateCompWithNonExistRBAC(defaultCompName, compDefObj.Name)
		})

		It("create component with custom RBAC which is already exist created by User", func() {
			testCreateCompWithRBACCreateByUser(defaultCompName, compDefObj.Name)
		})
	})

	Context("system account", func() {
		BeforeEach(func() {
			createDefinitionObjects()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("provisioning", func() {
			testCompSystemAccount(defaultCompName, compDefObj.Name)
		})

		It("override", func() {
			testCompSystemAccountOverride(defaultCompName, compDefObj.Name)
		})

		It("disable", func() {
			testCompSystemAccountDisable(defaultCompName, compDefObj.Name)
		})

		It("disable - after provision", func() {
			testCompSystemAccountDisableAfterProvision(defaultCompName, compDefObj.Name)
		})

		It("update", func() {
			testCompSystemAccountUpdate(defaultCompName, compDefObj.Name)
		})
	})

	Context("h-scaling", func() {
		BeforeEach(func() {
			createDefinitionObjects()
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

		It("scale-out from 1 to 3", func() {
			testHorizontalScale(defaultCompName, compDefObj.Name, 1, 3)
		})

		It("scale-in from 3 to 1", func() {
			testHorizontalScale(defaultCompName, compDefObj.Name, 3, 1)
		})

		It("scale-in to 0 and PVCs should not been deleted", func() {
			changeReplicasLimit(compDefObj.Name, 0, 16384)

			testHorizontalScale(defaultCompName, compDefObj.Name, 3, 0)
		})

		It("h-scale with data actions", func() {
			testHorizontalScaleWithDataActions(defaultCompName, compDefObj.Name, 1, 2)
		})
	})

	Context("volume expansion", func() {
		var (
			mockStorageClass *storagev1.StorageClass
		)

		BeforeEach(func() {
			createDefinitionObjects()
			mockStorageClass = testk8s.CreateMockStorageClass(&testCtx, testk8s.DefaultStorageClassName)
		})

		It("should update PVC request storage size accordingly", func() {
			testVolumeExpansion(defaultCompName, compDefObj.Name)
		})

		It("scale-out", func() {
			testVolumeExpansion(defaultCompName, compDefObj.Name)
			horizontalScale(5, mockStorageClass.Name, defaultCompName, compDefObj.Name)
		})
	})

	Context("start & stop", func() {
		BeforeEach(func() {
			cleanEnv()
			createDefinitionObjects()
		})

		startComp := func() {
			Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
				comp.Spec.Stop = nil
			})()).Should(Succeed())
		}

		stopComp := func() {
			Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
				comp.Spec.Stop = ptr.To(true)
			})()).Should(Succeed())
		}

		checkCompRunningWithPhase := func(phase kbappsv1.ComponentPhase) {
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
				g.Expect(its.Spec.PersistentVolumeClaimRetentionPolicy).ShouldNot(BeNil())
				g.Expect(its.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled).Should(Equal(kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType))
			})).Should(Succeed())
		}

		checkCompCreating := func() {
			checkCompRunningWithPhase(kbappsv1.CreatingComponentPhase)
		}

		checkCompRunning := func() {
			checkCompRunningWithPhase(kbappsv1.StartingComponentPhase)
		}

		checkCompStopped := func() {
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
				g.Expect(comp.Status.ObservedGeneration).To(BeEquivalentTo(comp.Generation))
				g.Expect(comp.Spec.Stop).ShouldNot(BeNil())
				g.Expect(*comp.Spec.Stop).Should(BeTrue())
				g.Expect(comp.Status.Phase).Should(Equal(kbappsv1.StoppedComponentPhase))
			})).Should(Succeed())

			itsKey := compKey
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(*its.Spec.Replicas).To(BeEquivalentTo(0))
				g.Expect(its.Spec.PersistentVolumeClaimRetentionPolicy).ShouldNot(BeNil())
				g.Expect(its.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled).Should(Equal(kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType))
			})).Should(Succeed())
		}

		It("stop a component", func() {
			createCompObj(defaultCompName, compDefObj.Name, nil)
			checkCompCreating()

			By("stop it")
			stopComp()
			checkCompStopped()

			By("stop it again")
			stopComp()
			checkCompStopped()
		})

		It("start a component", func() {
			createCompObj(defaultCompName, compDefObj.Name, nil)
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
			createCompObjWithPhase(defaultCompName, compDefObj.Name, func(f *testapps.MockComponentFactory) {
				f.SetStop(ptr.To(true))
			}, kbappsv1.StoppedComponentPhase)
			checkCompStopped()

			By("scale-out")
			Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
				comp.Spec.Replicas = 3
			})()).ShouldNot(HaveOccurred())

			By("check comp & its")
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
				g.Expect(comp.Spec.Replicas).Should(Equal(3))
				g.Expect(comp.Status.ObservedGeneration < comp.Generation).Should(BeTrue())
				g.Expect(comp.Status.Phase).Should(Equal(kbappsv1.StoppedComponentPhase))
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
				g.Expect(comp.Status.Phase).Should(Equal(kbappsv1.UpdatingComponentPhase))
			}))
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(*its.Spec.Replicas).To(BeEquivalentTo(3))
			}))
		})

		It("h-scale a stopped component - w/ data actions", func() {
			By("update the cmpd object to set data actions")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(compDefObj),
				func(cmpd *kbappsv1.ComponentDefinition) {
					if cmpd.Spec.LifecycleActions == nil {
						cmpd.Spec.LifecycleActions = &kbappsv1.ComponentLifecycleActions{}
					}
					cmpd.Spec.LifecycleActions.DataLoad = testapps.NewLifecycleAction("data-load")
					cmpd.Spec.LifecycleActions.DataDump = testapps.NewLifecycleAction("data-dump")
				})()).Should(Succeed())

			createCompObjWithPhase(defaultCompName, compDefObj.Name, func(f *testapps.MockComponentFactory) {
				f.SetStop(ptr.To(true))
			}, kbappsv1.StoppedComponentPhase)
			checkCompStopped()

			By("scale-out")
			Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
				comp.Spec.Replicas = 3
			})()).ShouldNot(HaveOccurred())

			By("check comp & its")
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *kbappsv1.Component) {
				g.Expect(comp.Spec.Replicas).Should(Equal(3))
				g.Expect(comp.Status.ObservedGeneration < comp.Generation).Should(BeTrue())
				g.Expect(comp.Status.Phase).Should(Equal(kbappsv1.StoppedComponentPhase))
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
				g.Expect(comp.Status.Phase).Should(Equal(kbappsv1.UpdatingComponentPhase))
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
			createDefinitionObjects()
		})

		testImageUnchangedAfterNewReleasePublished := func(release kbappsv1.ComponentVersionRelease) {
			prevRelease := compVerObj.Spec.Releases[0]

			By("check new release")
			Expect(prevRelease.Images).Should(HaveLen(len(release.Images)))
			Expect(maps.Keys(prevRelease.Images)).Should(BeEquivalentTo(maps.Keys(release.Images)))
			Expect(maps.Values(prevRelease.Images)).ShouldNot(BeEquivalentTo(maps.Values(release.Images)))

			createCompObj(defaultCompName, compDefObj.Name, func(f *testapps.MockComponentFactory) {
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

	Context("registry config", func() {
		registry := "foo.bar"
		setRegistryConfig := func() {
			viper.Set(constant.CfgRegistries, map[string]any{
				"defaultRegistry": registry,
			})
			Expect(intctrlutil.LoadRegistryConfig()).Should(Succeed())
		}

		BeforeEach(func() {
			createDefinitionObjects()
		})

		AfterEach(func() {
			viper.Set(constant.CfgRegistries, nil)
			Expect(intctrlutil.LoadRegistryConfig()).Should(Succeed())
		})

		It("replaces image registry", func() {
			setRegistryConfig()

			createCompObj(defaultCompName, compDefObj.Name, nil)

			itsKey := compKey
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				// check the image
				c := its.Spec.Template.Spec.Containers[0]
				g.Expect(c.Image).To(HavePrefix(registry))
			})).Should(Succeed())
		})

		It("handles running its and upgrade", func() {
			createCompObj(defaultCompName, compDefObj.Name, func(f *testapps.MockComponentFactory) {
				f.SetServiceVersion(compDefObj.Spec.ServiceVersion)
			})
			itsKey := compKey
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				// check the image
				c := its.Spec.Template.Spec.Containers[0]
				g.Expect(c.Image).To(Equal(compVerObj.Spec.Releases[0].Images[c.Name]))
			})).Should(Succeed())

			setRegistryConfig()

			By("trigger component reconcile")
			now := time.Now().Format(time.RFC3339)
			Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
				comp.Annotations["now"] = now
			})()).Should(Succeed())

			Consistently(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				// check the image
				c := its.Spec.Template.Spec.Containers[0]
				g.Expect(c.Image).NotTo(HavePrefix(registry))
			})).Should(Succeed())

			By("replaces registry when upgrading")
			release := kbappsv1.ComponentVersionRelease{
				Name:           "8.0.31",
				ServiceVersion: "8.0.31",
				Images: map[string]string{
					testapps.DefaultMySQLContainerName: "docker.io/apecloud/mysql:8.0.31",
				},
			}

			By("publish a new release")
			compVerKey := client.ObjectKeyFromObject(compVerObj)
			Expect(testapps.GetAndChangeObj(&testCtx, compVerKey, func(compVer *kbappsv1.ComponentVersion) {
				compVer.Spec.Releases = append(compVer.Spec.Releases, release)
				compVer.Spec.CompatibilityRules[0].Releases = append(compVer.Spec.CompatibilityRules[0].Releases, release.Name)
			})()).Should(Succeed())

			By("update service version in component")
			Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *kbappsv1.Component) {
				comp.Spec.ServiceVersion = "8.0.31"
			})()).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				// check the image
				c := its.Spec.Template.Spec.Containers[0]
				g.Expect(c.Image).To(HavePrefix(registry))
			})).Should(Succeed())
		})
	})

	Context("file (config/script) template", func() {
		var (
			fileTemplate = "log-conf"
		)

		BeforeEach(func() {
			createDefinitionObjects()

			// create the config file template object
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      "test-log-conf-template",
				},
				Data: map[string]string{
					"level": "{{- if (index $ \"LOG_LEVEL\") }}\n\t{{- .LOG_LEVEL }}\n{{- else }}\n\t{{- \"info\" }}\n{{- end }}",
				},
			}
			Expect(testCtx.CreateObj(testCtx.Ctx, cm)).Should(Succeed())

			// mock the cmpd to add the config file template and volume mount
			compDefKey := client.ObjectKeyFromObject(compDefObj)
			Expect(testapps.GetAndChangeObj(&testCtx, compDefKey, func(cmpd *kbappsv1.ComponentDefinition) {
				cmpd.Spec.Configs = []kbappsv1.ComponentFileTemplate{
					{
						Name:       fileTemplate,
						Template:   "test-log-conf-template",
						Namespace:  testCtx.DefaultNamespace,
						VolumeName: fileTemplate,
					},
				}
				for i := range cmpd.Spec.Runtime.Containers {
					if cmpd.Spec.Runtime.Containers[i].VolumeMounts == nil {
						cmpd.Spec.Runtime.Containers[i].VolumeMounts = make([]corev1.VolumeMount, 0)
					}
					cmpd.Spec.Runtime.Containers[i].VolumeMounts =
						append(cmpd.Spec.Runtime.Containers[i].VolumeMounts, corev1.VolumeMount{
							Name:      fileTemplate,
							MountPath: "/var/run/app/conf/log",
						})
				}
				cmpd.Spec.LifecycleActions.Reconfigure = testapps.NewLifecycleAction("reconfigure")
			})()).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("add/delete volumes", func() {
			testFileTemplateVolumes(defaultCompName, compDefObj.Name, fileTemplate)
		})

		It("reconfigure - action", func() {
			testReconfigureAction(defaultCompName, compDefObj.Name, fileTemplate)
		})

		It("reconfigure - action udf", func() {
			testReconfigureActionUDF(defaultCompName, compDefObj.Name, fileTemplate)
		})

		It("reconfigure - volume changed", func() {
			testReconfigureVolumeChanged(defaultCompName, compDefObj.Name, fileTemplate)
		})

		It("reconfigure - restart", func() {
			testReconfigureRestart(defaultCompName, compDefObj.Name, fileTemplate)
		})
	})
})
