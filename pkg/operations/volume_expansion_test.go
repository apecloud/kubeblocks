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

package operations

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/storage"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
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

	initResourcesForVolumeExpansion := func(clusterObject *appsv1.Cluster, opsRes *OpsResource, storage string, replicas int) (*opsv1alpha1.OpsRequest, []string) {
		pvcNames := opsRes.Cluster.GetVolumeClaimNames(consensusCompName)
		for _, pvcName := range pvcNames {
			createPVC(clusterObject.Name, storageClassName, vctName, pvcName)
			// mock pvc is Bound
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Status.Phase = corev1.ClaimBound
			})()).ShouldNot(HaveOccurred())

		}
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

	mockVolumeExpansionActionAndReconcile := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, newOps *opsv1alpha1.OpsRequest, pvcNames []string) {
		// first step, validate ops and update phase to Creating
		_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
		Expect(err).Should(BeNil())

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
		actualStorage string) {
		// mock cluster is Running to support volume expansion ops
		Expect(testapps.ChangeObjStatus(&testCtx, clusterObject, func() {
			clusterObject.Status.Phase = appsv1.RunningClusterPhase
		})).ShouldNot(HaveOccurred())

		// init resources for volume expansion
		comp := clusterObject.Spec.GetComponentByName(consensusCompName)
		newOps, pvcNames := initResourcesForVolumeExpansion(clusterObject, opsRes, requestStorage, int(comp.Replicas))

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
			testVolumeExpansion(reqCtx, clusterObject, opsRes, "3Gi", "3Gi")

			By("Test VolumeExpansion with inconsistent storageSize but it is valid")
			testVolumeExpansion(reqCtx, clusterObject, opsRes, "5G", "5Gi")

			By("Test delete the Running VolumeExpansion OpsRequest")
			testDeleteRunningVolumeExpansion(clusterObject, opsRes)
		})
	})
})
