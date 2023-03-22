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

package operations

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/storage"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("OpsRequest Controller Volume Expansion Handler", func() {

	var (
		// waitDuration          = time.Second * 3
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
		storageClassName      = "csi-hostpath-sc-" + randomStr
	)

	const (
		vctName           = "data"
		consensusCompName = "consensus"
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
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
			consensusCompName, "data").SetStorage("2Gi").SetStorageClass(storageClassName).Create(&testCtx)
	}

	mockDoOperationOnCluster := func(cluster *appsv1alpha1.Cluster, opsRequestName string, toClusterPhase appsv1alpha1.ClusterPhase) {
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *appsv1alpha1.Cluster) {
			if tmpCluster.Annotations == nil {
				tmpCluster.Annotations = map[string]string{}
			}
			tmpCluster.Annotations[constant.OpsRequestAnnotationKey] = fmt.Sprintf(`[{"clusterPhase": "%s", "name":"%s"}]`, toClusterPhase, opsRequestName)
		})()).ShouldNot(HaveOccurred())

		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, myCluster *appsv1alpha1.Cluster) {
			g.Expect(getOpsRequestNameFromAnnotation(myCluster, appsv1alpha1.SpecReconcilingClusterPhase)).ShouldNot(BeEmpty()) // appsv1alpha1.VolumeExpandingPhase
			// TODO: add status condition expect for appsv1alpha1.VolumeExpandingPhase
		})).Should(Succeed())
	}

	initResourcesForVolumeExpansion := func(clusterObject *appsv1alpha1.Cluster, opsRes *OpsResource, index int) (*appsv1alpha1.OpsRequest, string) {
		currRandomStr := testCtx.GetRandomStr()
		ops := testapps.NewOpsRequestObj("volumeexpansion-ops-"+currRandomStr, testCtx.DefaultNamespace,
			clusterObject.Name, appsv1alpha1.VolumeExpansionType)
		ops.Spec.VolumeExpansionList = []appsv1alpha1.VolumeExpansion{
			{
				ComponentOps: appsv1alpha1.ComponentOps{ComponentName: consensusCompName},
				VolumeClaimTemplates: []appsv1alpha1.OpsRequestVolumeClaimTemplate{
					{
						Name:    vctName,
						Storage: resource.MustParse("3Gi"),
					},
				},
			},
		}
		opsRes.OpsRequest = ops

		// create opsRequest
		ops = testapps.CreateOpsRequest(ctx, testCtx, ops)

		By("mock do operation on cluster")
		mockDoOperationOnCluster(clusterObject, ops.Name, appsv1alpha1.SpecReconcilingClusterPhase) // appsv1alpha1.VolumeExpandingPhase
		// TODO: add status condition expect for appsv1alpha1.VolumeExpandingPhase

		// create-pvc
		pvcName := fmt.Sprintf("%s-%s-%s-%d", vctName, clusterObject.Name, consensusCompName, index)
		createPVC(clusterObject.Name, storageClassName, vctName, pvcName)
		// waiting pvc controller mark annotation to OpsRequest
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(ops), func(g Gomega, tmpOps *appsv1alpha1.OpsRequest) {
			g.Expect(tmpOps.Annotations).ShouldNot(BeNil())
			g.Expect(tmpOps.Annotations[constant.ReconcileAnnotationKey]).ShouldNot(BeEmpty())
		})).Should(Succeed())
		return ops, pvcName
	}

	mockVolumeExpansionActionAndReconcile := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, newOps *appsv1alpha1.OpsRequest) {
		Expect(testapps.ChangeObjStatus(&testCtx, newOps, func() {
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			newOps.Status.Phase = appsv1alpha1.OpsRunningPhase
			newOps.Status.StartTimestamp = metav1.Time{Time: time.Now()}
		})).ShouldNot(HaveOccurred())

		// do volume-expand action
		_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
		opsRes.OpsRequest = newOps
		_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
		Expect(err == nil).Should(BeTrue())
		Eventually(testapps.GetOpsRequestCompPhase(ctx, testCtx, newOps.Name, consensusCompName)).Should(Equal(appsv1alpha1.SpecReconcilingClusterCompPhase)) // VolumeExpandingPhase
		// TODO: add status condition expect for VolumeExpandingPhase
	}

	testWarningEventOnPVC := func(reqCtx intctrlutil.RequestCtx, clusterObject *appsv1alpha1.Cluster, opsRes *OpsResource) {
		// init resources for volume expansion
		newOps, pvcName := initResourcesForVolumeExpansion(clusterObject, opsRes, 1)

		By("mock run volumeExpansion action and reconcileAction")
		mockVolumeExpansionActionAndReconcile(reqCtx, opsRes, newOps)

		By("test warning event and volumeExpansion failed")
		// test when the event does not reach the conditions
		event := &corev1.Event{
			Count:   1,
			Type:    corev1.EventTypeWarning,
			Reason:  VolumeResizeFailed,
			Message: "You've reached the maximum modification rate per volume limit. Wait at least 6 hours between modifications per EBS volume.",
		}
		stsInvolvedObject := corev1.ObjectReference{
			Name:      pvcName,
			Kind:      constant.PersistentVolumeClaimKind,
			Namespace: "default",
		}
		event.InvolvedObject = stsInvolvedObject
		pvcEventHandler := PersistentVolumeClaimEventHandler{}
		Expect(pvcEventHandler.Handle(k8sClient, reqCtx, eventRecorder, event)).Should(Succeed())
		Eventually(testapps.GetOpsRequestCompPhase(ctx, testCtx, newOps.Name, consensusCompName)).Should(Equal(appsv1alpha1.SpecReconcilingClusterCompPhase)) // VolumeExpandingPhase
		// TODO: add status condition expect for VolumeExpandingPhase

		// test when the event reach the conditions
		event.Count = 5
		event.FirstTimestamp = metav1.Time{Time: time.Now()}
		event.LastTimestamp = metav1.Time{Time: time.Now().Add(61 * time.Second)}
		Expect(pvcEventHandler.Handle(k8sClient, reqCtx, eventRecorder, event)).Should(Succeed())
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(newOps), func(g Gomega, tmpOps *appsv1alpha1.OpsRequest) {
			progressDetails := tmpOps.Status.Components[consensusCompName].ProgressDetails
			g.Expect(len(progressDetails) > 0).Should(BeTrue())
			progressDetail := FindStatusProgressDetail(progressDetails, getPVCProgressObjectKey(pvcName))
			g.Expect(progressDetail.Status == appsv1alpha1.FailedProgressStatus).Should(BeTrue())
		})).Should(Succeed())
	}

	testVolumeExpansion := func(reqCtx intctrlutil.RequestCtx, clusterObject *appsv1alpha1.Cluster, opsRes *OpsResource, randomStr string) {
		// mock cluster is Running to support volume expansion ops
		Expect(testapps.ChangeObjStatus(&testCtx, clusterObject, func() {
			clusterObject.Status.Phase = appsv1alpha1.RunningClusterPhase
		})).ShouldNot(HaveOccurred())

		// init resources for volume expansion
		newOps, pvcName := initResourcesForVolumeExpansion(clusterObject, opsRes, 0)

		By("mock run volumeExpansion action and reconcileAction")
		mockVolumeExpansionActionAndReconcile(reqCtx, opsRes, newOps)

		By("mock pvc is resizing")
		pvcKey := client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}
		Expect(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
			pvc.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
				Type:               corev1.PersistentVolumeClaimResizing,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
			},
			}
		})()).ShouldNot(HaveOccurred())

		Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
			conditions := tmpPVC.Status.Conditions
			g.Expect(len(conditions) > 0 && conditions[0].Type == corev1.PersistentVolumeClaimResizing).Should(BeTrue())
		})).Should(Succeed())

		// waiting OpsRequest.status.components["consensus"].vct["data"] is running
		_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(newOps), func(g Gomega, tmpOps *appsv1alpha1.OpsRequest) {
			progressDetails := tmpOps.Status.Components[consensusCompName].ProgressDetails
			progressDetail := FindStatusProgressDetail(progressDetails, getPVCProgressObjectKey(pvcName))
			g.Expect(progressDetail != nil && progressDetail.Status == appsv1alpha1.ProcessingProgressStatus).Should(BeTrue())
		})).Should(Succeed())

		By("mock pvc resizing succeed")
		// mock pvc volumeExpansion succeed
		Expect(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
			pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("3Gi")}
		})()).ShouldNot(HaveOccurred())

		Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
			g.Expect(tmpPVC.Status.Capacity[corev1.ResourceStorage] == resource.MustParse("3Gi")).Should(BeTrue())
		})).Should(Succeed())

		// waiting OpsRequest.status.phase is succeed
		_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
		Expect(err == nil).Should(BeTrue())
		Expect(opsRes.OpsRequest.Status.Phase == appsv1alpha1.OpsSucceedPhase).Should(BeTrue())

		testWarningEventOnPVC(reqCtx, clusterObject, opsRes)
	}

	testDeleteRunningVolumeExpansion := func(clusterObject *appsv1alpha1.Cluster, opsRes *OpsResource) {
		// init resources for volume expansion
		newOps, pvcName := initResourcesForVolumeExpansion(clusterObject, opsRes, 2)
		Expect(testapps.ChangeObjStatus(&testCtx, clusterObject, func() {
			clusterObject.Status.Phase = appsv1alpha1.SpecReconcilingClusterPhase // appsv1alpha1.VolumeExpandingPhase
			// TODO: add status condition for VolumeExpandingPhase
		})).ShouldNot(HaveOccurred())
		Expect(k8sClient.Delete(ctx, newOps)).Should(Succeed())
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, &appsv1alpha1.OpsRequest{})
		}).Should(Satisfy(apierrors.IsNotFound))

		By("test handle the invalid volumeExpansion OpsRequest")
		pvc := &corev1.PersistentVolumeClaim{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, pvc)).Should(Succeed())
		Expect(handleVolumeExpansionWithPVC(intctrlutil.RequestCtx{Ctx: ctx}, k8sClient, pvc)).Should(Succeed())

		Eventually(testapps.GetClusterPhase(&testCtx, client.ObjectKeyFromObject(clusterObject))).Should(Equal(appsv1alpha1.RunningClusterPhase))
	}

	Context("Test VolumeExpansion", func() {
		It("VolumeExpansion should work", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			_, _, clusterObject := testapps.InitConsensusMysql(testCtx, clusterDefinitionName,
				clusterVersionName, clusterName, "consensus", consensusCompName)
			// init storageClass
			sc := testapps.CreateStorageClass(testCtx, storageClassName, true)
			Expect(testapps.ChangeObj(&testCtx, sc, func() {
				sc.Annotations = map[string]string{storage.IsDefaultStorageClassAnnotation: "true"}
			})).ShouldNot(HaveOccurred())

			opsRes := &OpsResource{
				Cluster:  clusterObject,
				Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
			}

			By("Test OpsManager.MainEnter function with ClusterOps")
			Expect(testapps.ChangeObjStatus(&testCtx, clusterObject, func() {
				clusterObject.Status.Phase = appsv1alpha1.RunningClusterPhase
				clusterObject.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
					consensusCompName: {
						Phase: appsv1alpha1.RunningClusterCompPhase,
					},
				}
			})).ShouldNot(HaveOccurred())

			By("Test VolumeExpansion")
			testVolumeExpansion(reqCtx, clusterObject, opsRes, randomStr)

			By("Test delete the Running VolumeExpansion OpsRequest")
			testDeleteRunningVolumeExpansion(clusterObject, opsRes)
		})
	})
})
