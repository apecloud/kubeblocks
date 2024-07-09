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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

var _ = Describe("OpsUtil functions", func() {

	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
		rebuildInstanceCount  = 2
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
		testapps.ClearResources(&testCtx, generics.BackupSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.RestoreSignature, inNS, ml)
		// default GracePeriod is 30s
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test Rebuild-Instance opsRequest", func() {
		createRebuildInstanceOps := func(backupName string, instanceNames ...string) *appsv1alpha1.OpsRequest {
			opsName := "rebuild-instance-" + testCtx.GetRandomStr()
			ops := testapps.NewOpsRequestObj(opsName, testCtx.DefaultNamespace,
				clusterName, appsv1alpha1.RebuildInstanceType)
			var instances []appsv1alpha1.Instance
			for _, insName := range instanceNames {
				instances = append(instances, appsv1alpha1.Instance{
					Name: insName,
				})
			}
			ops.Spec.RebuildFrom = []appsv1alpha1.RebuildInstance{
				{
					ComponentOps: appsv1alpha1.ComponentOps{ComponentName: consensusComp},
					Instances:    instances,
					BackupName:   backupName,
				},
			}
			opsRequest := testapps.CreateOpsRequest(ctx, testCtx, ops)
			opsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase
			return opsRequest
		}

		prepareOpsRes := func(backupName string) *OpsResource {
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterName)
			podList := initInstanceSetPods(ctx, k8sClient, opsRes)

			// fake to create the source pvc.
			for i := range podList {
				pvcName := fmt.Sprintf("%s-%s", testapps.DataVolumeName, podList[i].Name)
				testapps.NewPersistentVolumeClaimFactory(podList[i].Namespace, pvcName, clusterName, consensusComp, testapps.DataVolumeName).
					SetStorage("20Gi").Create(&testCtx)
			}

			By("Test the functions in ops_util.go")
			opsRes.OpsRequest = createRebuildInstanceOps(backupName, podList[0].Name, podList[1].Name)
			return opsRes
		}

		fakePVCSByRestore := func(opsRequest *appsv1alpha1.OpsRequest) *corev1.PersistentVolumeClaimList {
			pvcList := &corev1.PersistentVolumeClaimList{}
			for i := 0; i < 2; i++ {
				pvcName := fmt.Sprintf("rebuild-%s-%s-%d", opsRequest.UID[:8], common.CutString(consensusComp+"-"+testapps.DataVolumeName, 30), i)
				pv := testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName, consensusComp, testapps.DataVolumeName).
					AddAnnotations(rebuildFromAnnotation, opsRequest.Name).
					SetStorage("20Gi").Create(&testCtx).GetObject()
				pvcList.Items = append(pvcList.Items, *pv)
			}
			return pvcList
		}

		fakeTmpPVCBoundPV := func(pvcList *corev1.PersistentVolumeClaimList) []*corev1.PersistentVolume {
			var pvs []*corev1.PersistentVolume
			for i := range pvcList.Items {
				pvc := &pvcList.Items[i]
				_, ok := pvc.Annotations[rebuildFromAnnotation]
				if !ok {
					continue
				}
				pvName := pvc.Name + "-pv"
				pv := testapps.NewPersistentVolumeFactory(pvc.Namespace, pvc.Name+"-pv", pvc.Name).
					SetStorage("20Gi").
					SetClaimRef(pvc).
					Create(&testCtx).GetObject()
				pvs = append(pvs, pv)
				Expect(testapps.ChangeObj(&testCtx, pvc, func(p *corev1.PersistentVolumeClaim) {
					p.Spec.VolumeName = pvName
				})).Should(Succeed())
			}

			return pvs
		}

		It("test rebuild instance when cluster/component are mismatched", func() {
			By("init operations resources ")
			opsRes := prepareOpsRes("")
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

			By("fake cluster phase to Abnormal and component phase to Running")
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				opsRes.Cluster.Status.Phase = appsv1alpha1.AbnormalClusterPhase
			})).Should(Succeed())
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsCreatingPhase

			By("expect for opsRequest phase is Failed if the phase of component is not matched")
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsFailedPhase))
			Expect(opsRes.OpsRequest.Status.Conditions[0].Message).Should(ContainSubstring(fmt.Sprintf(`the phase of component "%s" can not be %s`, consensusComp, appsv1alpha1.RunningClusterCompPhase)))

			By("fake component phase to Abnormal")
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsCreatingPhase
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				compStatus := opsRes.Cluster.Status.Components[consensusComp]
				compStatus.Phase = appsv1alpha1.AbnormalClusterCompPhase
				opsRes.Cluster.Status.Components[consensusComp] = compStatus
			})).Should(Succeed())

			By("expect for opsRequest phase is Failed due to the pod is Available")
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsFailedPhase))

			By("fake pod is unavailable")
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsCreatingPhase
			for _, ins := range opsRes.OpsRequest.Spec.RebuildFrom[0].Instances {
				Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKey{Name: ins.Name, Namespace: opsRes.OpsRequest.Namespace}, func(pod *corev1.Pod) {
					pod.Status.Conditions = nil
				})()).Should(Succeed())
			}
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsCreatingPhase))
		})

		sourcePVCsShouldRebindPVs := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, pvcList *corev1.PersistentVolumeClaimList) {
			// fake the pvs
			pvs := fakeTmpPVCBoundPV(pvcList)

			// rebind the source pvcs and pvs
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			for i := range pvs {
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pvs[i]), func(g Gomega, pv *corev1.PersistentVolume) {
					g.Expect(pv.Spec.ClaimRef).Should(BeNil())
					g.Expect(pv.Spec.PersistentVolumeReclaimPolicy).Should(Equal(corev1.PersistentVolumeReclaimRetain))
					g.Expect(pv.Annotations[rebuildFromAnnotation]).Should(Equal(opsRes.OpsRequest.Name))
				}))
			}

			Expect(k8sClient.List(ctx, pvcList, client.MatchingLabels{constant.KBAppComponentLabelKey: consensusComp}, client.InNamespace(opsRes.OpsRequest.Namespace))).Should(Succeed())
			reCreatePVCCount := 0
			for i := range pvcList.Items {
				pvc := &pvcList.Items[i]
				rebuildFrom, ok := pvc.Annotations[rebuildFromAnnotation]
				if !ok {
					continue
				}
				reCreatePVCCount += 1
				Expect(rebuildFrom).Should(Equal(opsRes.OpsRequest.Name))
				Expect(pvc.Spec.VolumeName).Should(ContainSubstring("-pv"))
			}
			Expect(reCreatePVCCount).Should(Equal(rebuildInstanceCount))
		}

		waitForInstanceToAvailable := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, ignoreRoleCheck bool) {
			By("waiting for the rebuild instance to ready")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, ops *appsv1alpha1.OpsRequest) {
				compStatus := ops.Status.Components[consensusComp]
				g.Expect(compStatus.ProgressDetails[0].Message).Should(Equal(waitingForInstanceReadyMessage))
				g.Expect(compStatus.ProgressDetails[1].Message).Should(Equal(waitingForInstanceReadyMessage))
			}))

			By("fake th rebuild pods to ready ")
			// recreate the instances and fake it to ready.
			pods := initInstanceSetPods(ctx, k8sClient, opsRes)
			if ignoreRoleCheck {
				for i := range pods {
					Expect(testapps.ChangeObj(&testCtx, pods[i], func(pod *corev1.Pod) {
						if pod.Labels != nil {
							delete(pod.Labels, constant.RoleLabelKey)
						}
					})).Should(Succeed())
				}
			}
		}

		It("test rebuild instance with no backup", func() {
			By("init operations resources ")
			opsRes := prepareOpsRes("")
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsRunningPhase
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

			By("expect for the tmp pods and pvcs are created")
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			matchingLabels := client.MatchingLabels{
				constant.OpsRequestNameLabelKey:      opsRes.OpsRequest.Name,
				constant.OpsRequestNamespaceLabelKey: opsRes.OpsRequest.Namespace,
			}
			podList := &corev1.PodList{}
			Expect(k8sClient.List(ctx, podList, matchingLabels, client.InNamespace(opsRes.OpsRequest.Namespace))).Should(Succeed())
			Expect(podList.Items).Should(HaveLen(rebuildInstanceCount))
			pvcList := &corev1.PersistentVolumeClaimList{}
			Expect(k8sClient.List(ctx, pvcList, client.MatchingLabels{constant.KBAppComponentLabelKey: consensusComp}, client.InNamespace(opsRes.OpsRequest.Namespace))).Should(Succeed())
			tmpPVCCount := 0
			for i := range pvcList.Items {
				if _, ok := pvcList.Items[i].Annotations[rebuildFromAnnotation]; ok {
					tmpPVCCount += 1
				}
			}
			Expect(tmpPVCCount).Should(Equal(rebuildInstanceCount))

			By("fake the rebuilding pod to be Completed and fake pvs are created.")
			for i := range podList.Items {
				pod := &podList.Items[i]
				Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
					pod.Status.Phase = corev1.PodSucceeded
				})).Should(Succeed())
			}

			By("expect to create the source pvcs and the pvs have rebind them.")
			sourcePVCsShouldRebindPVs(reqCtx, opsRes, pvcList)

			By("expect the opsRequest to succeed")
			waitForInstanceToAvailable(reqCtx, opsRes, false)
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, ops *appsv1alpha1.OpsRequest) {
				g.Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsSucceedPhase))
			}))

			By("expect to clean up the tmp pods")
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Eventually(testapps.List(&testCtx, generics.PodSignature, matchingLabels, client.InNamespace(opsRes.OpsRequest.Namespace))).Should(HaveLen(0))
		})

		testRebuildInstanceWithBackup := func(ignoreRoleCheck bool) {
			By("init operation resources and backup")
			actionSet := testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml",
				&dpv1alpha1.ActionSet{}, testapps.WithName(testdp.ActionSetName))
			backup := testdp.NewBackupFactory(testCtx.DefaultNamespace, testdp.BackupName).
				SetBackupPolicyName(testdp.BackupPolicyName).
				SetBackupMethod(testdp.BackupMethodName).
				AddLabels(dptypes.BackupTypeLabelKey, string(dpv1alpha1.BackupTypeFull)).
				Create(&testCtx).GetObject()
			// fake backup is completed
			Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
				backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
				backup.Status.BackupMethod = &dpv1alpha1.BackupMethod{
					Name:          backup.Spec.BackupMethod,
					ActionSetName: actionSet.Name,
					TargetVolumes: &dpv1alpha1.TargetVolumeInfo{
						VolumeMounts: []corev1.VolumeMount{
							{Name: testapps.DataVolumeName, MountPath: "/test"},
						},
					},
				}
			})).Should(Succeed())
			opsRes := prepareOpsRes(backup.Name)
			if ignoreRoleCheck {
				Expect(testapps.ChangeObj(&testCtx, opsRes.OpsRequest, func(request *appsv1alpha1.OpsRequest) {
					if request.Annotations == nil {
						request.Annotations = map[string]string{}
					}
					request.Annotations[ignoreRoleCheckAnnotationKey] = "true"
				})).Should(Succeed())
			}
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsRunningPhase
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

			By("expect for the prepareData Restore CR has been created.")
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			matchingLabels := client.MatchingLabels{
				constant.OpsRequestNameLabelKey:      opsRes.OpsRequest.Name,
				constant.OpsRequestNamespaceLabelKey: opsRes.OpsRequest.Namespace,
			}
			restoreList := &dpv1alpha1.RestoreList{}
			Expect(k8sClient.List(ctx, restoreList, matchingLabels, client.InNamespace(opsRes.OpsRequest.Namespace))).Should(Succeed())
			Expect(restoreList.Items).Should(HaveLen(rebuildInstanceCount))

			By("fake to create the pvcs which should be created by Restore Controller and change restore phase to Completed")
			// create the pvcs
			pvcList := fakePVCSByRestore(opsRes.OpsRequest)
			fakeRestoresToCompleted := func() {
				// fake restores to Completed
				for i := range restoreList.Items {
					restore := &restoreList.Items[i]
					Expect(testapps.ChangeObjStatus(&testCtx, restore, func() {
						restore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
					})).Should(Succeed())
				}
			}
			fakeRestoresToCompleted()

			By("expect to create the source pvcs and the pvs have rebind them.")
			sourcePVCsShouldRebindPVs(reqCtx, opsRes, pvcList)

			By("expect to create the postReady restore after the instances are available")
			waitForInstanceToAvailable(reqCtx, opsRes, ignoreRoleCheck)
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(k8sClient.List(ctx, restoreList, matchingLabels, client.InNamespace(opsRes.OpsRequest.Namespace))).Should(Succeed())
			// The number of restores should be twice the number of instances that need to be restored.
			Expect(restoreList.Items).Should(HaveLen(rebuildInstanceCount * 2))

			By("fake the postReady restores to Completed and expect the opsRequest to Succeed.")
			fakeRestoresToCompleted()
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, ops *appsv1alpha1.OpsRequest) {
				g.Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsSucceedPhase))
			}))
		}

		It("test rebuild instance with backup", func() {
			testRebuildInstanceWithBackup(false)
		})

		It("test rebuild instance with backup and ignore role check", func() {
			testRebuildInstanceWithBackup(true)
		})

	})
})
