/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package operations

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubectl/pkg/util/storage"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

var _ = Describe("OpsRequest Controller Volume Expansion Handler", func() {

	var (
		randomStr        = testCtx.GetRandomStr()
		compDefName      = "test-compdef-" + randomStr
		clusterName      = "test-cluster-" + randomStr
		storageClassName = "csi-hostpath-sc-" + randomStr
	)

	const (
		vctName           = "data"
		consensusCompName = "consensus"
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), cluster definition
		testapps.ClearClusterResources(&testCtx)

		// delete component definition resources
		testapps.ClearComponentResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// delete pvc resources
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		// namespaced
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	createPVC := func(clusterName, scName, vctName, pvcName string) {
		// Note: in real k8s cluster, it maybe fails when pvc created by k8s controller.
		testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName,
			consensusCompName, testapps.DataVolumeName).AddLabels(constant.AppInstanceLabelKey, clusterName,
			constant.VolumeClaimTemplateNameLabelKey, testapps.DataVolumeName,
			constant.KBAppComponentLabelKey, consensusCompName).SetStorage("2Gi").SetStorageClass(storageClassName).CheckedCreate(&testCtx)
	}

	createItsPVC := func(clusterName, scName, vctName, pvcName, cmpName string) {
		var itsName string
		parts := strings.Split(pvcName, cmpName+"-")
		if len(parts) == 2 && strings.Contains(parts[1], "-") {
			p := strings.Split(parts[1], "-")
			itsName = p[0]
			testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName,
				consensusCompName, testapps.DataVolumeName).AddLabels(constant.AppInstanceLabelKey, clusterName,
				constant.VolumeClaimTemplateNameLabelKey, testapps.DataVolumeName,
				constant.KBAppComponentLabelKey, consensusCompName, constant.KBAppComponentInstanceTemplateLabelKey,
				itsName).SetStorage("2Gi").SetStorageClass(storageClassName).CheckedCreate(&testCtx)
		} else {
			testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName,
				consensusCompName, testapps.DataVolumeName).AddLabels(constant.AppInstanceLabelKey, clusterName,
				constant.VolumeClaimTemplateNameLabelKey, testapps.DataVolumeName,
				constant.KBAppComponentLabelKey, consensusCompName).SetStorage("2Gi").SetStorageClass(storageClassName).CheckedCreate(&testCtx)
		}
	}

	// getVolumeClaimNames gets all PVC names of component compName.
	//
	// cluster.Spec.GetComponentByName(compName).VolumeClaimTemplates[*].Name will be used if no claimNames provided
	//
	// nil return if:
	//   1. component compName not found or
	//   2. len(VolumeClaimTemplates)==0 or
	//   3. any claimNames not found
	getVolumeClaimNames := func(cluster *appsv1.Cluster, compName string, claimNames ...string) []string {
		if cluster == nil {
			return nil
		}
		comp := cluster.Spec.GetComponentByName(compName)
		if comp == nil {
			return nil
		}
		if len(comp.VolumeClaimTemplates) == 0 {
			return nil
		}
		if len(claimNames) == 0 {
			for _, template := range comp.VolumeClaimTemplates {
				claimNames = append(claimNames, template.Name)
			}
		}
		allExist := true
		for _, name := range claimNames {
			found := false
			for _, template := range comp.VolumeClaimTemplates {
				if template.Name == name {
					found = true
					break
				}
			}
			if !found {
				allExist = false
				break
			}
		}
		if !allExist {
			return nil
		}

		pvcNames := make([]string, 0)
		for _, claimName := range claimNames {
			for i := 0; i < int(comp.Replicas); i++ {
				pvcName := fmt.Sprintf("%s-%s-%s-%d", claimName, cluster.Name, compName, i)
				pvcNames = append(pvcNames, pvcName)
			}
		}
		return pvcNames
	}

	// getItsVolumeClaimNames gets all PVC names of component compName and instanceTemplateName.
	//
	// cluster.Spec.GetComponentByName(compName).VolumeClaimTemplates[*].Name will be used if no claimNames provided
	//
	// nil return if:
	//   1. component compName not found or
	//   2. len(VolumeClaimTemplates)==0 or
	//   3. any claimNames not found
	getItsVolumeClaimNames := func(cluster *appsv1.Cluster, compName string, claimNames ...string) []string {
		if cluster == nil {
			return nil
		}
		comp := cluster.Spec.GetComponentByName(compName)
		if comp == nil {
			return nil
		}
		if len(comp.VolumeClaimTemplates) == 0 {
			return nil
		}
		if len(claimNames) == 0 {
			for _, template := range comp.VolumeClaimTemplates {
				claimNames = append(claimNames, template.Name)
			}
		}
		allExist := true
		for _, name := range claimNames {
			found := false
			for _, template := range comp.VolumeClaimTemplates {
				if template.Name == name {
					found = true
					break
				}
			}
			if !found {
				allExist = false
				break
			}
		}
		if !allExist {
			return nil
		}

		pvcNames := make([]string, 0)
		for _, claimName := range claimNames {
			restReplicas := comp.Replicas
			for i := 0; i < int(comp.Replicas); i++ {
				// here we did not handle ordinals in instance template
				for _, its := range comp.Instances {
					for j := 0; j < int(*its.Replicas); j++ {
						pvcName := fmt.Sprintf("%s-%s-%s-%s-%d", claimName, cluster.Name, compName, its.Name, j)
						pvcNames = append(pvcNames, pvcName)
						restReplicas--
						i++
					}
				}
				for x := 0; x < int(restReplicas); x++ {
					pvcName := fmt.Sprintf("%s-%s-%s-%d", claimName, cluster.Name, compName, x)
					pvcNames = append(pvcNames, pvcName)
					i++
				}
			}
		}
		return pvcNames
	}

	initResourcesForVolumeExpansionBase := func(clusterObject *appsv1.Cluster, opsRes *OpsResource, storage string, replicas int, preparePvc func() []string) (*opsv1alpha1.OpsRequest, []string) {
		pvcNames := preparePvc()
		currRandomStr := testCtx.GetRandomStr()
		ops := testops.NewOpsRequestObj("volumeexpansion-ops-"+currRandomStr, testCtx.DefaultNamespace,
			clusterObject.Name, opsv1alpha1.VolumeExpansionType)
		ops.Spec.VolumeExpansionList = []opsv1alpha1.VolumeExpansion{
			{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: consensusCompName},
				VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{
					{
						Name:    vctName,
						Storage: resource.MustParse(storage),
					},
				},
			},
		}
		opsRes.OpsRequest = ops

		// create opsRequest
		ops = testops.CreateOpsRequest(ctx, testCtx, ops)
		return ops, pvcNames
	}

	initResourceForItsVolumeExpansion := func(clusterObject *appsv1.Cluster, opsRes *OpsResource, storage string, replicas int) (*opsv1alpha1.OpsRequest, []string) {
		return initResourcesForVolumeExpansionBase(clusterObject, opsRes, storage, replicas, func() []string {
			pvcNames := getItsVolumeClaimNames(opsRes.Cluster, consensusCompName)
			for _, pvcName := range pvcNames {
				createItsPVC(clusterObject.Name, storageClassName, vctName, pvcName, consensusCompName)
				// mock pvc is Bound
				Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Phase = corev1.ClaimBound
				})()).ShouldNot(HaveOccurred())

			}
			return pvcNames
		})
	}

	initResourcesForVolumeExpansion := func(clusterObject *appsv1.Cluster, opsRes *OpsResource, storage string, replicas int) (*opsv1alpha1.OpsRequest, []string) {
		return initResourcesForVolumeExpansionBase(clusterObject, opsRes, storage, replicas, func() []string {
			pvcNames := getVolumeClaimNames(opsRes.Cluster, consensusCompName)
			for _, pvcName := range pvcNames {
				createPVC(clusterObject.Name, storageClassName, vctName, pvcName)
				// mock pvc is Bound
				Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Phase = corev1.ClaimBound
				})()).ShouldNot(HaveOccurred())

			}
			return pvcNames
		})
	}

	mockVolumeExpansionActionAndReconcile := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, newOps *opsv1alpha1.OpsRequest, pvcNames []string) {
		// first step, validate ops and update phase to Creating
		_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
		Expect(err).Should(BeNil())
		// ops phase should not failed
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, func(g Gomega, tmpOps *opsv1alpha1.OpsRequest) {
			g.Expect(tmpOps.Status.Phase).To(Not(Equal(opsv1alpha1.OpsFailedPhase)))
		})).Should(Succeed())

		// next step, do volume-expand action
		_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
		Expect(err).Should(BeNil())

		By("mock pvc.spec.resources.request.storage has applied by cluster controller")
		for _, pvcName := range pvcNames {
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.Resources.Requests[corev1.ResourceStorage] = newOps.Spec.VolumeExpansionList[0].VolumeClaimTemplates[0].Storage
			})()).ShouldNot(HaveOccurred())
		}

		By("mock opsRequest is Running")
		Expect(testapps.ChangeObjStatus(&testCtx, newOps, func() {
			newOps.Status.Phase = opsv1alpha1.OpsRunningPhase
			newOps.Status.StartTimestamp = metav1.Time{Time: time.Now()}
		})).ShouldNot(HaveOccurred())

		// reconcile ops status
		opsRes.OpsRequest = newOps
		_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
		Expect(err).Should(BeNil())
	}

	testVolumeExpansion := func(reqCtx intctrlutil.RequestCtx,
		clusterObject *appsv1.Cluster,
		opsRes *OpsResource,
		requestStorage,
		actualStorage string,
		initResourceFunc func(clusterObject *appsv1.Cluster, opsRes *OpsResource, storage string, replicas int) (*opsv1alpha1.OpsRequest, []string)) {
		// mock cluster is Running to support volume expansion ops
		Expect(testapps.ChangeObjStatus(&testCtx, clusterObject, func() {
			clusterObject.Status.Phase = appsv1.RunningClusterPhase
		})).ShouldNot(HaveOccurred())

		// init resources for volume expansion
		comp := clusterObject.Spec.GetComponentByName(consensusCompName)
		newOps, pvcNames := initResourceFunc(clusterObject, opsRes, requestStorage, int(comp.Replicas))

		By("mock run volumeExpansion action and reconcileAction")
		mockVolumeExpansionActionAndReconcile(reqCtx, opsRes, newOps, pvcNames)

		By("mock pvc is resizing")
		for _, pvcName := range pvcNames {
			pvcKey := client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}
			Expect(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
					Type:               corev1.PersistentVolumeClaimResizing,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
				},
				}
				pvc.Status.Phase = corev1.ClaimBound
			})()).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
				conditions := tmpPVC.Status.Conditions
				g.Expect(len(conditions) > 0 && conditions[0].Type == corev1.PersistentVolumeClaimResizing).Should(BeTrue())
			})).Should(Succeed())

			// waiting OpsRequest.status.components["consensus"].vct["data"] is running
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(newOps), func(g Gomega, tmpOps *opsv1alpha1.OpsRequest) {
				progressDetails := tmpOps.Status.Components[consensusCompName].ProgressDetails
				progressDetail := findStatusProgressDetail(progressDetails, getPVCProgressObjectKey(pvcName))
				g.Expect(progressDetail != nil && progressDetail.Status == opsv1alpha1.ProcessingProgressStatus).Should(BeTrue())
				g.Expect(progressDetail.StartTime.IsZero()).Should(BeFalse())
			})).Should(Succeed())

			By("mock pvc resizing succeed")
			// mock pvc volumeExpansion succeed
			Expect(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(actualStorage)}
			})()).ShouldNot(HaveOccurred())
		}

		// waiting for OpsRequest.status.phase is succeed
		_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
		Expect(err).Should(BeNil())
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(newOps), func(g Gomega, tmpOps *opsv1alpha1.OpsRequest) {
			progressDetails := tmpOps.Status.Components[consensusCompName].ProgressDetails
			progressDetail := findStatusProgressDetail(progressDetails, getPVCProgressObjectKey(pvcNames[0]))
			g.Expect(progressDetail != nil && progressDetail.Status == opsv1alpha1.SucceedProgressStatus).Should(BeTrue())
			g.Expect(progressDetail.EndTime.IsZero()).Should(BeFalse())
		})).Should(Succeed())
		Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
	}

	testDeleteRunningVolumeExpansion := func(clusterObject *appsv1.Cluster, opsRes *OpsResource) {
		// init resources for volume expansion
		newOps, pvcNames := initResourcesForVolumeExpansion(clusterObject, opsRes, "5Gi", 1)
		Expect(k8sClient.Delete(ctx, newOps)).Should(Succeed())
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, &opsv1alpha1.OpsRequest{})
		}).Should(Satisfy(apierrors.IsNotFound))

		By("test handle the invalid volumeExpansion OpsRequest")
		pvc := &corev1.PersistentVolumeClaim{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: pvcNames[0], Namespace: testCtx.DefaultNamespace}, pvc)).Should(Succeed())

		Eventually(testapps.GetClusterPhase(&testCtx, client.ObjectKeyFromObject(clusterObject))).Should(Equal(appsv1.RunningClusterPhase))
	}

	Context("Test VolumeExpansion", func() {
		It("persists instance-only volume expansion requests", func() {
			ops := testops.NewOpsRequestObj("instance-volumeexpansion-"+testCtx.GetRandomStr(),
				testCtx.DefaultNamespace, clusterName, opsv1alpha1.VolumeExpansionType)
			ops.Spec.VolumeExpansionList = []opsv1alpha1.VolumeExpansion{{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: consensusCompName},
				Instances: []opsv1alpha1.InstanceVolumeClaimTemplate{{
					Name:                 "large",
					VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: vctName, Storage: resource.MustParse("10Gi")}},
				}},
			}}
			created := testops.CreateOpsRequest(ctx, testCtx, ops)
			stored := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(created), stored)).To(Succeed())
			Expect(stored.Spec.VolumeExpansionList).To(Equal(ops.Spec.VolumeExpansionList))
		})

		It("VolumeExpansion should work", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			_, clusterObject := testapps.InitConsensusMysql(&testCtx, clusterName, compDefName, consensusCompName)
			// init storageClass
			sc := testapps.CreateStorageClass(&testCtx, storageClassName, true)
			Expect(testapps.ChangeObj(&testCtx, sc, func(lsc *storagev1.StorageClass) {
				lsc.Annotations = map[string]string{storage.IsDefaultStorageClassAnnotation: "true"}
			})).ShouldNot(HaveOccurred())

			opsRes := &OpsResource{
				Cluster:  clusterObject,
				Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
			}

			By("Test OpsManager.MainEnter function with ClusterOps")
			Expect(testapps.ChangeObjStatus(&testCtx, clusterObject, func() {
				clusterObject.Status.Phase = appsv1.RunningClusterPhase
				clusterObject.Status.Components = map[string]appsv1.ClusterComponentStatus{
					consensusCompName: {
						Phase: appsv1.RunningComponentPhase,
					},
				}
			})).ShouldNot(HaveOccurred())

			By("Test VolumeExpansion with consistent storageSize")
			testVolumeExpansion(reqCtx, clusterObject, opsRes, "3Gi", "3Gi", initResourcesForVolumeExpansion)

			By("Test VolumeExpansion with inconsistent storageSize but it is valid")
			testVolumeExpansion(reqCtx, clusterObject, opsRes, "5G", "5Gi", initResourcesForVolumeExpansion)

			By("Test delete the Running VolumeExpansion OpsRequest")
			testDeleteRunningVolumeExpansion(clusterObject, opsRes)
		})

		It("VolumeExpandsion with InstanceTemplate", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			_, clusterObject := testapps.InitConsensusMysql(&testCtx, clusterName, compDefName, consensusCompName)
			// all pods use instance template
			Expect(testapps.ChangeObj(&testCtx, clusterObject, func(lc *appsv1.Cluster) {
				lc.Spec.ComponentSpecs[0].Instances = []appsv1.InstanceTemplate{
					{
						Name:     "foo",
						Replicas: ptr.To(int32(1)),
					},
					{
						Name:     "bar",
						Replicas: ptr.To(int32(2)),
					},
				}
			})).ShouldNot(HaveOccurred())
			// init storageClass
			sc := testapps.CreateStorageClass(&testCtx, storageClassName, true)
			Expect(testapps.ChangeObj(&testCtx, sc, func(lsc *storagev1.StorageClass) {
				lsc.Annotations = map[string]string{storage.IsDefaultStorageClassAnnotation: "true"}
			})).ShouldNot(HaveOccurred())
			opsRes := &OpsResource{
				Cluster:  clusterObject,
				Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
			}
			By("Test OpsManager.MainEnter function with ClusterOps")
			Expect(testapps.ChangeObjStatus(&testCtx, clusterObject, func() {
				clusterObject.Status.Phase = appsv1.RunningClusterPhase
				clusterObject.Status.Components = map[string]appsv1.ClusterComponentStatus{
					consensusCompName: {
						Phase: appsv1.RunningComponentPhase,
					},
				}
			})).ShouldNot(HaveOccurred())

			By("Test volume expansion with instance template")
			testVolumeExpansion(reqCtx, clusterObject, opsRes, "3Gi", "3Gi", initResourceForItsVolumeExpansion)
		})
	})
})

func TestVolumeExpansionInstances(t *testing.T) {
	for _, multipleVolumes := range []bool{false, true} {
		for _, sharding := range []bool{false, true} {
			for _, mode := range []string{"component", "instance", "both"} {
				t.Run(fmt.Sprintf("sharding=%v/multipleVolumes=%v/%s", sharding, multipleVolumes, mode), func(t *testing.T) {
					scheme := runtime.NewScheme()
					for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, opsv1alpha1.AddToScheme} {
						if err := add(scheme); err != nil {
							t.Fatal(err)
						}
					}
					vct := appsv1.ClusterComponentVolumeClaimTemplate{Name: "data"}
					vct.Spec.StorageClassName = ptr.To("expandable")
					vct.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
					vct.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}
					override := *vct.DeepCopy()
					override.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("5Gi")
					spec := appsv1.ClusterComponentSpec{
						Name: "db", Replicas: 4, OfflineInstances: []string{"test-db-inherit-2", "test-db-a-inherit-2", "test-db-b-inherit-2"}, VolumeClaimTemplates: []appsv1.ClusterComponentVolumeClaimTemplate{vct},
						Instances: []appsv1.InstanceTemplate{
							{Name: "inherit", Ordinals: appsv1.Ordinals{Discrete: []int32{2, 3}}}, // nil replicas defaults to one.
							{Name: "custom", VolumeClaimTemplates: []appsv1.ClusterComponentVolumeClaimTemplate{override}},
							{Name: "idle", Replicas: ptr.To(int32(0))},
						},
					}
					if multipleVolumes {
						logs := *vct.DeepCopy()
						logs.Name = "logs"
						untouched := *vct.DeepCopy()
						untouched.Name = "untouched"
						spec.VolumeClaimTemplates = append(spec.VolumeClaimTemplates, logs, untouched)
						extra := *vct.DeepCopy()
						extra.Name = "extra"
						spec.Instances[1].VolumeClaimTemplates = append(spec.Instances[1].VolumeClaimTemplates, extra)
					}
					cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}}
					components := []string{"db"}
					if sharding {
						cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "db", Template: spec}}
						components = []string{"db-a", "db-b"}
					} else {
						cluster.Spec.ComponentSpecs = []appsv1.ClusterComponentSpec{spec}
					}
					expansion := opsv1alpha1.VolumeExpansion{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}}
					if mode != "instance" {
						expansion.VolumeClaimTemplates = []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("2Gi")}}
					}
					if mode != "component" {
						expansion.Instances = []opsv1alpha1.InstanceVolumeClaimTemplate{
							{Name: "inherit", VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("3Gi")}}},
							{Name: "custom", VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("6Gi")}}},
							{Name: "idle", VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("4Gi")}}},
						}
					}
					if multipleVolumes {
						if mode != "instance" {
							expansion.VolumeClaimTemplates = append(expansion.VolumeClaimTemplates, opsv1alpha1.OpsRequestVolumeClaimTemplate{Name: "logs", Storage: resource.MustParse("3Gi")})
						}
						if mode != "component" {
							expansion.Instances[0].VolumeClaimTemplates = append(expansion.Instances[0].VolumeClaimTemplates, opsv1alpha1.OpsRequestVolumeClaimTemplate{Name: "logs", Storage: resource.MustParse("4Gi")})
							expansion.Instances[1].VolumeClaimTemplates = append(expansion.Instances[1].VolumeClaimTemplates, opsv1alpha1.OpsRequestVolumeClaimTemplate{Name: "extra", Storage: resource.MustParse("8Gi")})
						}
					}
					ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "expand", Namespace: "default"}, Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{VolumeExpansionList: []opsv1alpha1.VolumeExpansion{expansion}}}}
					ops.Status.StartTimestamp = metav1.Now()
					objects := []client.Object{cluster, ops}
					for _, comp := range components {
						if sharding {
							objects = append(objects, &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "test-" + comp, Namespace: "default", Labels: map[string]string{constant.AppInstanceLabelKey: "test", constant.KBAppShardingNameLabelKey: "db", constant.KBAppComponentLabelKey: comp}}})
						}
						for suffix, size := range map[string]string{"0": "2Gi", "1": "2Gi", "inherit-2": "1Gi", "inherit-3": "2Gi", "custom-0": "5Gi"} {
							if mode != "component" && suffix == "inherit-3" {
								size = "3Gi"
							}
							if mode != "component" && suffix == "custom-0" {
								size = "6Gi"
							}
							pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-test-" + comp + "-" + suffix, Namespace: "default", Labels: map[string]string{constant.AppInstanceLabelKey: "test", constant.KBAppComponentLabelKey: comp, constant.VolumeClaimTemplateNameLabelKey: "data"}}}
							pvc.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}
							pvc.Status.Capacity = pvc.Spec.Resources.Requests.DeepCopy()
							pvc.Status.Phase = corev1.ClaimBound
							objects = append(objects, pvc)
							if multipleVolumes {
								for _, name := range []string{"logs", "untouched", "extra"} {
									if name == "extra" && suffix != "custom-0" {
										continue
									}
									size := "1Gi"
									if name == "logs" && mode != "instance" {
										size = "3Gi"
									}
									if name == "logs" && suffix == "inherit-3" && mode != "component" {
										size = "4Gi"
									}
									if name == "extra" && mode != "component" {
										size = "8Gi"
									}
									other := pvc.DeepCopy()
									other.Name = name + "-test-" + comp + "-" + suffix
									other.Labels[constant.VolumeClaimTemplateNameLabelKey] = name
									other.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(size)
									other.Status.Capacity = other.Spec.Resources.Requests.DeepCopy()
									objects = append(objects, other)
								}
							}
						}
					}
					cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops).WithObjects(objects...).Build()
					res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(100)}
					req := intctrlutil.RequestCtx{Ctx: context.Background()}
					handler := volumeExpansionOpsHandler{}
					if err := handler.SaveLastConfiguration(req, cli, res); err != nil {
						t.Fatal(err)
					}
					if err := handler.Action(req, cli, res); err != nil {
						t.Fatal(err)
					}
					got := spec
					if sharding {
						got = cluster.Spec.Shardings[0].Template
					} else {
						got = cluster.Spec.ComponentSpecs[0]
					}
					want := "2Gi"
					if mode == "instance" {
						want = "1Gi"
					}
					if got.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Cmp(resource.MustParse(want)) != 0 {
						t.Fatal("incorrect component storage")
					}
					if mode != "component" {
						for i, want := range []string{"3Gi", "6Gi", "4Gi"} {
							volume := got.Instances[i].VolumeClaimTemplates[0]
							if volume.Spec.Resources.Requests.Storage().Cmp(resource.MustParse(want)) != 0 || *volume.Spec.StorageClassName != "expandable" || len(volume.Spec.AccessModes) != 1 {
								t.Fatalf("incorrect template volume: %+v", volume)
							}
						}
						last := ops.Status.LastConfiguration.Components["db"].Instances
						if len(last) != 3 || len(last[0].VolumeClaimTemplates) != 0 || last[1].VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Cmp(resource.MustParse("5Gi")) != 0 {
							t.Fatalf("incorrect previous templates: %+v", last)
						}
					} else if len(got.Instances[0].VolumeClaimTemplates) != 0 || got.Instances[1].VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Cmp(resource.MustParse("5Gi")) != 0 {
						t.Fatal("component expansion modified template overrides")
					}
					count := map[string]int{"component": 3, "instance": 2, "both": 4}[mode] * len(components)
					if multipleVolumes {
						count = map[string]int{"component": 7, "instance": 4, "both": 9}[mode] * len(components)
						assertVolume := func(volumes []appsv1.ClusterComponentVolumeClaimTemplate, name, want string) {
							t.Helper()
							for _, volume := range volumes {
								if volume.Name == name {
									if volume.Spec.Resources.Requests.Storage().Cmp(resource.MustParse(want)) != 0 {
										t.Fatalf("%s: expected %s, got %s", name, want, volume.Spec.Resources.Requests.Storage())
									}
									return
								}
							}
							t.Fatalf("missing volume %s", name)
						}
						logsSize, extraSize := "3Gi", "1Gi"
						if mode == "instance" {
							logsSize = "1Gi"
						}
						if mode != "component" {
							extraSize = "8Gi"
							assertVolume(got.Instances[0].VolumeClaimTemplates, "logs", "4Gi")
						}
						assertVolume(got.VolumeClaimTemplates, "logs", logsSize)
						assertVolume(got.VolumeClaimTemplates, "untouched", "1Gi")
						assertVolume(got.Instances[1].VolumeClaimTemplates, "extra", extraSize)
						for _, volume := range got.Instances[1].VolumeClaimTemplates {
							if volume.Name == "logs" {
								t.Fatal("partial override must keep inheriting logs")
							}
						}
					}
					// A PVC still resizing must keep the operation running.
					pending := &corev1.PersistentVolumeClaim{}
					key := client.ObjectKey{Namespace: "default", Name: "data-test-" + components[0] + "-inherit-3"}
					if multipleVolumes {
						key.Name = "logs-test-" + components[0] + "-inherit-3"
					}
					if err := cli.Get(req.Ctx, key, pending); err != nil {
						t.Fatal(err)
					}
					capacity := pending.Status.Capacity.DeepCopy()
					pending.Status.Capacity[corev1.ResourceStorage] = resource.MustParse("1Gi")
					if err := cli.Status().Update(req.Ctx, pending); err != nil {
						t.Fatal(err)
					}
					phase, _, err := handler.ReconcileAction(req, cli, res)
					if err != nil || phase != opsv1alpha1.OpsRunningPhase || ops.Status.Progress != fmt.Sprintf("%d/%d", count-1, count) {
						t.Fatalf("pending: phase=%s progress=%s err=%v", phase, ops.Status.Progress, err)
					}
					pending.Status.Capacity = capacity
					if err := cli.Status().Update(req.Ctx, pending); err != nil {
						t.Fatal(err)
					}
					phase, _, err = handler.ReconcileAction(req, cli, res)
					if err != nil || phase != opsv1alpha1.OpsSucceedPhase || ops.Status.Progress != fmt.Sprintf("%d/%d", count, count) {
						t.Fatalf("phase=%s progress=%s err=%v", phase, ops.Status.Progress, err)
					}
				})
			}
		}
	}
}
