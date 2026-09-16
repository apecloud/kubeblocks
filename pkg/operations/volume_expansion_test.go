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
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
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
				constant.KBAppComponentLabelKey, consensusCompName, constant.KBAppInstanceTemplateLabelKey,
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
		It("preserves payload updates and control fields in the existing API", func() {
			ops := testops.NewOpsRequestObj("volumeexpansion-update-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace, clusterName, opsv1alpha1.VolumeExpansionType)
			ops.Spec.VolumeExpansionList = []opsv1alpha1.VolumeExpansion{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: consensusCompName}, VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: vctName, Storage: resource.MustParse("5Gi")}}}}
			ops = testops.CreateOpsRequest(ctx, testCtx, ops)
			current := ops.DeepCopy()
			current.Spec.VolumeExpansionList[0].VolumeClaimTemplates[0].Storage = resource.MustParse("6Gi")
			Expect(k8sClient.Update(ctx, current)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), current)).To(Succeed())
			current.Spec.VolumeExpansionList = nil
			Expect(k8sClient.Update(ctx, current)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), current)).To(Succeed())
			current.Spec.VolumeExpansionList = ops.Spec.VolumeExpansionList
			current.Spec.Cancel = true
			Expect(k8sClient.Update(ctx, current)).To(Succeed())
		})

		It("preserves an empty volume selection as a no-op", func() {
			_, clusterObject := testapps.InitConsensusMysql(&testCtx, clusterName, compDefName, consensusCompName)
			ops := testops.NewOpsRequestObj("volumeexpansion-empty-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace, clusterName, opsv1alpha1.VolumeExpansionType)
			ops.Spec.VolumeExpansionList = []opsv1alpha1.VolumeExpansion{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: consensusCompName}, VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{}}}
			ops = testops.CreateOpsRequest(ctx, testCtx, ops)
			Expect(testapps.ChangeObjStatus(&testCtx, ops, func() { ops.Status.Phase = opsv1alpha1.OpsPendingPhase })).To(Succeed())
			opsRes := &OpsResource{Cluster: clusterObject, OpsRequest: ops, Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller")}
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).NotTo(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).To(Equal(opsv1alpha1.OpsCreatingPhase))
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).NotTo(HaveOccurred())
			Expect(testapps.ChangeObjStatus(&testCtx, ops, func() {
				ops.Status.Phase = opsv1alpha1.OpsRunningPhase
				ops.Status.StartTimestamp = metav1.Now()
			})).To(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).NotTo(HaveOccurred())
			persisted := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), persisted)).To(Succeed())
			Expect(persisted.Status.Phase).To(Equal(opsv1alpha1.OpsSucceedPhase))
			Expect(persisted.Status.Progress).To(Equal("0/0"))
		})

		DescribeTable("expands only instances inheriting component storage", func(overridden int32, stopped bool) {
			testapps.CreateStorageClass(&testCtx, storageClassName, true)
			_, clusterObject := testapps.InitConsensusMysql(&testCtx, clusterName, compDefName, consensusCompName)
			Expect(testapps.ChangeObj(&testCtx, clusterObject, func(c *appsv1.Cluster) {
				comp := &c.Spec.ComponentSpecs[0]
				comp.Stop = ptr.To(stopped)
				comp.Instances = []appsv1.InstanceTemplate{{Name: "custom", Replicas: ptr.To(overridden), VolumeClaimTemplates: comp.VolumeClaimTemplates}}
				if overridden < comp.Replicas {
					other := comp.VolumeClaimTemplates[0].DeepCopy()
					other.Name = "other"
					comp.Instances = append(comp.Instances, appsv1.InstanceTemplate{Name: "inherited", Replicas: ptr.To(int32(1)), VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{*other}})
				}
			})).To(Succeed())
			testapps.MockInstanceSetComponent(&testCtx, clusterName, consensusCompName)
			testapps.MockInstanceSetStatus(testCtx, clusterObject, consensusCompName)
			opsRes := &OpsResource{Cluster: clusterObject, Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller")}
			ops, pvcNames := initResourceForItsVolumeExpansion(clusterObject, opsRes, "5Gi", int(clusterObject.Spec.ComponentSpecs[0].Replicas))
			Expect(testapps.ChangeObjStatus(&testCtx, ops, func() { ops.Status.Phase = opsv1alpha1.OpsPendingPhase })).To(Succeed())
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).NotTo(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).To(Equal(opsv1alpha1.OpsCreatingPhase))
			before := clusterObject.Spec.ComponentSpecs[0].DeepCopy()
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).NotTo(HaveOccurred())
			persistedCluster := &appsv1.Cluster{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(clusterObject), persistedCluster)).To(Succeed())
			Expect(persistedCluster.Spec.ComponentSpecs[0].Instances).To(Equal(before.Instances))
			Expect(persistedCluster.Spec.ComponentSpecs[0].VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().String()).To(Equal("5Gi"))
			Expect(testapps.ChangeObjStatus(&testCtx, ops, func() {
				ops.Status.Phase = opsv1alpha1.OpsRunningPhase
				ops.Status.StartTimestamp = metav1.Now()
				ops.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{consensusCompName: {ProgressDetails: []opsv1alpha1.ProgressStatusDetail{{ObjectKey: "PVC/previously-included-override", Status: opsv1alpha1.FailedProgressStatus}}}}
			})).To(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).NotTo(HaveOccurred())
			expected := int(before.Replicas - overridden)
			Expect(opsRes.OpsRequest.Status.Progress).To(Equal(fmt.Sprintf("0/%d", expected)))
			if expected > 0 {
				Expect(opsRes.OpsRequest.Status.Phase).To(Equal(opsv1alpha1.OpsRunningPhase))
			}
			for _, name := range pvcNames {
				pvc := &corev1.PersistentVolumeClaim{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: testCtx.DefaultNamespace, Name: name}, pvc)).To(Succeed())
				if pvc.Labels[constant.KBAppInstanceTemplateLabelKey] == "custom" {
					Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("2Gi"))
					continue
				}
				Expect(testapps.ChangeObj(&testCtx, pvc, func(v *corev1.PersistentVolumeClaim) {
					v.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("5Gi")
				})).To(Succeed())
				Expect(testapps.ChangeObjStatus(&testCtx, pvc, func() {
					pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("5Gi")}
				})).To(Succeed())
			}
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).NotTo(HaveOccurred())
			persistedOps := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), persistedOps)).To(Succeed())
			Expect(persistedOps.Status.Phase).To(Equal(opsv1alpha1.OpsSucceedPhase))
			Expect(persistedOps.Status.Progress).To(Equal(fmt.Sprintf("%d/%d", expected, expected)))
			Expect(persistedOps.Status.Components[consensusCompName].ProgressDetails).To(HaveLen(expected))
		}, Entry("mixed default and template instances", int32(1), false),
			Entry("all instances have independent storage", int32(3), false),
			Entry("stopped instances retain the same storage scope", int32(1), true))

		It("rejects an explicit unsupported storage class before writing the Cluster", func() {
			testapps.CreateStorageClass(&testCtx, storageClassName, false)
			_, clusterObject := testapps.InitConsensusMysql(&testCtx, clusterName, compDefName, consensusCompName)
			Expect(testapps.ChangeObj(&testCtx, clusterObject, func(c *appsv1.Cluster) {
				c.Spec.ComponentSpecs[0].VolumeClaimTemplates[0].Spec.StorageClassName = ptr.To(storageClassName)
			})).To(Succeed())
			before := clusterObject.Spec.DeepCopy()
			for _, phase := range []opsv1alpha1.OpsPhase{opsv1alpha1.OpsPendingPhase, opsv1alpha1.OpsCreatingPhase} {
				ops := testops.NewOpsRequestObj("volumeexpansion-sc-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace, clusterName, opsv1alpha1.VolumeExpansionType)
				ops.Spec.VolumeExpansionList = []opsv1alpha1.VolumeExpansion{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: consensusCompName}, VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: vctName, Storage: resource.MustParse("5Gi")}}}}
				ops = testops.CreateOpsRequest(ctx, testCtx, ops)
				Expect(testapps.ChangeObjStatus(&testCtx, ops, func() { ops.Status.Phase = phase })).To(Succeed())
				opsRes := &OpsResource{Cluster: clusterObject, OpsRequest: ops, Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller")}
				_, err := GetOpsManager().Do(intctrlutil.RequestCtx{Ctx: ctx}, k8sClient, opsRes)
				Expect(err).NotTo(HaveOccurred())
				persistedOps := &opsv1alpha1.OpsRequest{}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), persistedOps)).To(Succeed())
				Expect(persistedOps.Status.Phase).To(Equal(opsv1alpha1.OpsFailedPhase))
				persistedCluster := &appsv1.Cluster{}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(clusterObject), persistedCluster)).To(Succeed())
				Expect(persistedCluster.Spec).To(Equal(*before))
			}
		})

		It("skips unspecified storage class checks and keeps the existing convergence timeout", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			_, clusterObject := testapps.InitConsensusMysql(&testCtx, clusterName, compDefName, consensusCompName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, consensusCompName)
			testapps.MockInstanceSetStatus(testCtx, clusterObject, consensusCompName)
			testapps.CreateStorageClass(&testCtx, storageClassName, false)
			opsRes := &OpsResource{Cluster: clusterObject, Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller")}
			ops, _ := initResourcesForVolumeExpansion(clusterObject, opsRes, "5Gi", int(clusterObject.Spec.ComponentSpecs[0].Replicas))
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).NotTo(HaveOccurred())
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).NotTo(HaveOccurred())
			Expect(testapps.ChangeObjStatus(&testCtx, ops, func() {
				ops.Status.Phase = opsv1alpha1.OpsRunningPhase
				ops.Status.StartTimestamp = metav1.Now()
			})).To(Succeed())
			opsRes.OpsRequest = ops
			persistedCluster := &appsv1.Cluster{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(clusterObject), persistedCluster)).To(Succeed())
			Expect(persistedCluster.Spec.ComponentSpecs[0].VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().String()).To(Equal("5Gi"))
			delay, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).NotTo(HaveOccurred())
			Expect(delay).To(BeNumerically(">", 0))
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.OpsRequest, func() {
				opsRes.OpsRequest.Status.StartTimestamp = metav1.NewTime(time.Now().Add(-VolumeExpansionTimeOut - time.Minute))
			})).To(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).NotTo(HaveOccurred())
			persistedOps := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), persistedOps)).To(Succeed())
			Expect(persistedOps.Status.Phase).To(Equal(opsv1alpha1.OpsFailedPhase))
		})

		It("VolumeExpansion should work", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			_, clusterObject := testapps.InitConsensusMysql(&testCtx, clusterName, compDefName, consensusCompName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, consensusCompName)
			testapps.MockInstanceSetStatus(testCtx, clusterObject, consensusCompName)
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
			testapps.MockInstanceSetComponent(&testCtx, clusterName, consensusCompName)
			testapps.MockInstanceSetStatus(testCtx, clusterObject, consensusCompName)
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

func TestVolumeExpansionPreservesShardStorageOverrides(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	volumes := []appsv1.PersistentVolumeClaimTemplate{{Name: "data", Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("3Gi")}}}}}
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"}, Spec: appsv1.ClusterSpec{Shardings: []appsv1.ClusterSharding{{
		Name: "db", Shards: 2, Template: appsv1.ClusterComponentSpec{Replicas: 1, VolumeClaimTemplates: volumes},
		ShardTemplates: []appsv1.ShardTemplate{{Name: "wide", Shards: ptr.To(int32(1)), Replicas: ptr.To(int32(2))}, {Name: "independent", Shards: ptr.To(int32(1)), Replicas: ptr.To(int32(3)), VolumeClaimTemplates: volumes}},
	}}}}
	cluster = cluster.DeepCopy()
	ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "expand", Namespace: "default"}, Spec: opsv1alpha1.OpsRequestSpec{
		ClusterName: "demo", Type: opsv1alpha1.VolumeExpansionType,
		SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{VolumeExpansionList: []opsv1alpha1.VolumeExpansion{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}, VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("5Gi")}}}}},
	}, Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsCreatingPhase}}
	objects := []client.Object{cluster, ops}
	for _, entry := range []struct {
		name, template string
		replicas       int32
	}{{"db-a", "wide", 2}, {"db-b", "independent", 3}} {
		objects = append(objects, &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "demo-" + entry.name, Namespace: "default", Labels: constant.GetCompLabels("demo", entry.name, map[string]string{
			constant.KBAppShardingNameLabelKey: "db", constant.KBAppShardTemplateLabelKey: entry.template,
		})}, Spec: appsv1.ComponentSpec{Replicas: entry.replicas, VolumeClaimTemplates: volumes}})
	}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db-a", Namespace: "default"}, Spec: workloads.InstanceSetSpec{Replicas: ptr.To(int32(2))}}
	for i := 0; i < 2; i++ {
		name := fmt.Sprintf("demo-db-a-%d", i)
		its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: name, TemplateName: ptr.To("")})
		objects = append(objects, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-" + name, Namespace: "default", Labels: map[string]string{
			constant.AppInstanceLabelKey: "demo", constant.KBAppComponentLabelKey: "db-a", constant.VolumeClaimTemplateNameLabelKey: "data",
		}}, Spec: *volumes[0].Spec.DeepCopy(), Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound, Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("3Gi")}}})
	}
	objects = append(objects, its)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops, its, &corev1.PersistentVolumeClaim{}).WithObjects(objects...).Build()
	opsRes := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(50)}
	reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
	if _, err := GetOpsManager().Do(reqCtx, cli, opsRes); err != nil {
		t.Fatal(err)
	}
	persistedCluster := &appsv1.Cluster{}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(cluster), persistedCluster); err != nil {
		t.Fatal(err)
	}
	shard := persistedCluster.Spec.Shardings[0]
	if shard.Template.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().String() != "5Gi" || shard.ShardTemplates[1].VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().String() != "3Gi" {
		t.Fatal("component target or independent shard storage was changed incorrectly")
	}
	ops.Status.Phase = opsv1alpha1.OpsRunningPhase
	ops.Status.StartTimestamp = metav1.Now()
	if err := cli.Status().Update(ctx, ops); err != nil {
		t.Fatal(err)
	}
	if _, err := GetOpsManager().Reconcile(reqCtx, cli, opsRes); err != nil {
		t.Fatal(err)
	}
	if ops.Status.Phase != opsv1alpha1.OpsRunningPhase || ops.Status.Progress != "0/2" {
		t.Fatalf("phase=%s progress=%s, want Running 0/2", ops.Status.Phase, ops.Status.Progress)
	}
	for i := 0; i < 2; i++ {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("data-demo-db-a-%d", i), Namespace: "default"}, pvc); err != nil {
			t.Fatal(err)
		}
		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("5Gi")
		if err := cli.Update(ctx, pvc); err != nil {
			t.Fatal(err)
		}
		pvc.Status.Capacity[corev1.ResourceStorage] = resource.MustParse("5Gi")
		if err := cli.Status().Update(ctx, pvc); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := GetOpsManager().Reconcile(reqCtx, cli, opsRes); err != nil {
		t.Fatal(err)
	}
	persistedOps := &opsv1alpha1.OpsRequest{}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(ops), persistedOps); err != nil {
		t.Fatal(err)
	}
	if persistedOps.Status.Phase != opsv1alpha1.OpsSucceedPhase || persistedOps.Status.Progress != "2/2" {
		t.Fatalf("phase=%s progress=%s, want Succeed 2/2", persistedOps.Status.Phase, persistedOps.Status.Progress)
	}
	if len(persistedOps.Status.Components["db"].ProgressDetails) != 2 {
		t.Fatal("progress includes unaffected shard instances")
	}
}
