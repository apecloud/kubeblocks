/*
Copyright ApeCloud Inc.

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
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("OpsRequest Controller Volume Expansion Handler", func() {

	var (
		timeout  = time.Second * 10
		interval = time.Second * 1
		// waitDuration          = time.Second * 3
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
		storageClassName      = "csi-hostpath-sc-" + randomStr
		vctName               = "data"
		consensusCompName     = "consensus"
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.OpsRequestSignature, inNS, ml)
		// non-namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.StorageClassSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	assureDefaultStorageClassObj := func() *storagev1.StorageClass {
		By("By assure an default storageClass")
		scYAML := fmt.Sprintf(`
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: %s
  annotations:
    storageclass.kubernetes.io/is-default-class: "false"
provisioner: hostpath.csi.k8s.io
reclaimPolicy: Delete
volumeBindingMode: Immediate
allowVolumeExpansion: true
`, storageClassName)
		sc := &storagev1.StorageClass{}
		Expect(yaml.Unmarshal([]byte(scYAML), sc)).Should(Succeed())
		Expect(testCtx.CreateObj(ctx, sc)).Should(Succeed())
		return sc
	}

	createPVC := func(clusterName, scName, vctName, pvcName string) {
		pvcYaml := fmt.Sprintf(`apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  annotations:
    pv.kubernetes.io/bind-completed: "yes"
    pv.kubernetes.io/bound-by-controller: "yes"
    volume.beta.kubernetes.io/storage-provisioner: hostpath.csi.k8s.io
  labels:
    app.kubernetes.io/component-name: %s
    app.kubernetes.io/instance: %s
    app.kubernetes.io/managed-by: kubeblocks
    vct.kubeblocks.io/name: %s
  name: %s
  namespace: default
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 2Gi
  volumeMode: Filesystem
  storageClassName: %s
`, consensusCompName, clusterName, vctName, pvcName, scName)
		pvc := &corev1.PersistentVolumeClaim{}
		Expect(yaml.Unmarshal([]byte(pvcYaml), pvc)).Should(Succeed())
		err := testCtx.CreateObj(context.Background(), pvc)
		// maybe already created by controller in real cluster
		Expect(apierrors.IsAlreadyExists(err) || err == nil).Should(BeTrue())
		// wait until cluster created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, &corev1.PersistentVolumeClaim{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	mockDoOperationOnCluster := func(cluster *dbaasv1alpha1.Cluster, opsRequestName string, toClusterPhase dbaasv1alpha1.Phase) {
		tmpCluster := &dbaasv1alpha1.Cluster{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: cluster.Name, Namespace: testCtx.DefaultNamespace}, tmpCluster)).Should(Succeed())
		patch := client.MergeFrom(tmpCluster.DeepCopy())
		if tmpCluster.Annotations == nil {
			tmpCluster.Annotations = map[string]string{}
		}
		tmpCluster.Annotations[intctrlutil.OpsRequestAnnotationKey] = fmt.Sprintf(`[{"clusterPhase": "%s", "name":"%s"}]`, toClusterPhase, opsRequestName)
		Expect(k8sClient.Patch(ctx, tmpCluster, patch)).Should(Succeed())
		Eventually(func(g Gomega) bool {
			myCluster := &dbaasv1alpha1.Cluster{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: cluster.Name, Namespace: testCtx.DefaultNamespace}, myCluster)).Should(Succeed())
			return getOpsRequestNameFromAnnotation(myCluster, dbaasv1alpha1.VolumeExpandingPhase) != ""
		}, timeout, interval).Should(BeTrue())
	}

	initResourcesForVolumeExpansion := func(clusterObject *dbaasv1alpha1.Cluster, opsRes *OpsResource, index int) (*dbaasv1alpha1.OpsRequest, string) {
		currRandomStr := testCtx.GetRandomStr()
		ops := testdbaas.GenerateOpsRequestObj("volumeexpansion-ops-"+currRandomStr, clusterObject.Name, dbaasv1alpha1.VolumeExpansionType)
		ops.Spec.VolumeExpansionList = []dbaasv1alpha1.VolumeExpansion{
			{
				ComponentOps: dbaasv1alpha1.ComponentOps{ComponentName: consensusCompName},
				VolumeClaimTemplates: []dbaasv1alpha1.OpsRequestVolumeClaimTemplate{
					{
						Name:    vctName,
						Storage: resource.MustParse("3Gi"),
					},
				},
			},
		}
		opsRes.OpsRequest = ops
		// mock cluster to support volume expansion
		patch := client.MergeFrom(clusterObject.DeepCopy())
		clusterObject.Status.Operations = &dbaasv1alpha1.Operations{
			VolumeExpandable: []dbaasv1alpha1.OperationComponent{
				{
					VolumeClaimTemplateNames: []string{vctName},
					Name:                     consensusCompName,
				},
			},
		}
		Expect(k8sClient.Status().Patch(ctx, clusterObject, patch)).Should(Succeed())

		// create opsRequest
		ops = testdbaas.CreateOpsRequest(ctx, testCtx, ops)

		By("mock do operation on cluster")
		mockDoOperationOnCluster(clusterObject, ops.Name, dbaasv1alpha1.VolumeExpandingPhase)

		// create-pvc
		pvcName := fmt.Sprintf("%s-%s-%s-%d", vctName, clusterObject.Name, consensusCompName, index)
		createPVC(clusterObject.Name, storageClassName, vctName, pvcName)
		// waiting pvc controller mark annotation to OpsRequest
		Eventually(func(g Gomega) bool {
			tmpOps := &dbaasv1alpha1.OpsRequest{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: ops.Name, Namespace: testCtx.DefaultNamespace}, tmpOps)).Should(Succeed())
			return tmpOps.Annotations != nil && tmpOps.Annotations[intctrlutil.OpsRequestReconcileAnnotationKey] != ""
		}, timeout*2, interval).Should(BeTrue())
		return ops, pvcName
	}

	mockVolumeExpansionActionAndReconcile := func(opsRes *OpsResource, newOps *dbaasv1alpha1.OpsRequest) {
		patch := client.MergeFrom(newOps.DeepCopy())
		_ = GetOpsManager().Do(opsRes)
		newOps.Status.Phase = dbaasv1alpha1.RunningPhase
		newOps.Status.StartTimestamp = metav1.Time{Time: time.Now()}
		Expect(k8sClient.Status().Patch(ctx, newOps, patch)).Should(Succeed())

		opsRes.OpsRequest = newOps
		_, err := GetOpsManager().Reconcile(opsRes)
		Expect(err == nil).Should(BeTrue())
		Eventually(testdbaas.GetOpsRequestCompPhase(ctx, testCtx, newOps.Name, consensusCompName),
			timeout, interval).Should(Equal(dbaasv1alpha1.VolumeExpandingPhase))
	}

	testWarningEventOnPVC := func(clusterObject *dbaasv1alpha1.Cluster, opsRes *OpsResource) {
		// init resources for volume expansion
		newOps, pvcName := initResourcesForVolumeExpansion(clusterObject, opsRes, 1)

		By("mock run volumeExpansion action and reconcileAction")
		mockVolumeExpansionActionAndReconcile(opsRes, newOps)

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
			Kind:      intctrlutil.PersistentVolumeClaimKind,
			Namespace: "default",
		}
		event.InvolvedObject = stsInvolvedObject
		pvcEventHandler := PersistentVolumeClaimEventHandler{}
		reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
		Expect(pvcEventHandler.Handle(k8sClient, reqCtx, eventRecorder, event)).Should(Succeed())
		Eventually(testdbaas.GetOpsRequestCompPhase(ctx, testCtx, newOps.Name, consensusCompName), timeout, interval).Should(Equal(dbaasv1alpha1.VolumeExpandingPhase))

		// test when the event reach the conditions
		event.Count = 5
		event.FirstTimestamp = metav1.Time{Time: time.Now()}
		event.LastTimestamp = metav1.Time{Time: time.Now().Add(61 * time.Second)}
		Expect(pvcEventHandler.Handle(k8sClient, reqCtx, eventRecorder, event)).Should(Succeed())
		Eventually(func() bool {
			tmpOps := &dbaasv1alpha1.OpsRequest{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, tmpOps)
			progressDetails := tmpOps.Status.Components[consensusCompName].ProgressDetails
			if len(progressDetails) == 0 {
				return false
			}
			progressDetail := FindStatusProgressDetail(progressDetails, getPVCProgressObjectKey(pvcName))
			return progressDetail.Status == dbaasv1alpha1.FailedProgressStatus
		}, timeout, interval).Should(BeTrue())
	}

	testVolumeExpansion := func(clusterObject *dbaasv1alpha1.Cluster, opsRes *OpsResource, randomStr string) {
		// mock cluster is Running to support volume expansion ops
		patch := client.MergeFrom(clusterObject.DeepCopy())
		clusterObject.Status.Phase = dbaasv1alpha1.RunningPhase
		Expect(k8sClient.Status().Patch(ctx, clusterObject, patch)).Should(Succeed())

		// init resources for volume expansion
		newOps, pvcName := initResourcesForVolumeExpansion(clusterObject, opsRes, 0)

		By("mock run volumeExpansion action and reconcileAction")
		mockVolumeExpansionActionAndReconcile(opsRes, newOps)

		By("")

		By("mock pvc is resizing")
		pvc := &corev1.PersistentVolumeClaim{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, pvc)).Should(Succeed())
		patch = client.MergeFrom(pvc.DeepCopy())
		pvc.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
			Type:               corev1.PersistentVolumeClaimResizing,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		},
		}
		Expect(k8sClient.Status().Patch(ctx, pvc, patch)).Should(Succeed())
		Eventually(func() bool {
			tmpPVC := &corev1.PersistentVolumeClaim{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, tmpPVC)).Should(Succeed())
			conditions := tmpPVC.Status.Conditions
			return len(conditions) > 0 && conditions[0].Type == corev1.PersistentVolumeClaimResizing
		}, timeout, interval).Should(BeTrue())
		// waiting OpsRequest.status.components["consensus"].vct["data"] is running
		_, _ = GetOpsManager().Reconcile(opsRes)
		Eventually(func() bool {
			tmpOps := &dbaasv1alpha1.OpsRequest{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, tmpOps)
			progressDetails := tmpOps.Status.Components[consensusCompName].ProgressDetails
			progressDetail := FindStatusProgressDetail(progressDetails, getPVCProgressObjectKey(pvcName))
			return progressDetail != nil && progressDetail.Status == dbaasv1alpha1.ProcessingProgressStatus
		}, timeout, interval).Should(BeTrue())

		By("mock pvc resizing succeed")
		// mock pvc volumeExpansion succeed
		pvc = &corev1.PersistentVolumeClaim{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, pvc)).Should(Succeed())
		patch = client.MergeFrom(pvc.DeepCopy())
		pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("3Gi")}
		Expect(k8sClient.Status().Patch(ctx, pvc, patch)).Should(Succeed())
		Eventually(func() bool {
			tmpPVC := &corev1.PersistentVolumeClaim{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, tmpPVC)).Should(Succeed())
			return tmpPVC.Status.Capacity[corev1.ResourceStorage] == resource.MustParse("3Gi")
		}, timeout, interval).Should(BeTrue())
		// waiting OpsRequest.status.phase is succeed
		_, err := GetOpsManager().Reconcile(opsRes)
		Expect(err == nil).Should(BeTrue())
		Expect(opsRes.OpsRequest.Status.Phase == dbaasv1alpha1.SucceedPhase).Should(BeTrue())

		testWarningEventOnPVC(clusterObject, opsRes)
	}

	updateClusterPhase := func(cluster *dbaasv1alpha1.Cluster, phase dbaasv1alpha1.Phase) error {
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Phase = phase
		return k8sClient.Status().Patch(ctx, cluster, patch)
	}

	testDeleteRunningVolumeExpansion := func(clusterObject *dbaasv1alpha1.Cluster, opsRes *OpsResource) {
		// init resources for volume expansion
		newOps, pvcName := initResourcesForVolumeExpansion(clusterObject, opsRes, 2)
		Expect(updateClusterPhase(clusterObject, dbaasv1alpha1.VolumeExpandingPhase)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, newOps)).Should(Succeed())
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, &dbaasv1alpha1.OpsRequest{})
		}, timeout, interval).Should(Satisfy(apierrors.IsNotFound))

		By("test handle the invalid volumeExpansion OpsRequest")
		pvc := &corev1.PersistentVolumeClaim{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, pvc)).Should(Succeed())
		Expect(handleVolumeExpansionWithPVC(intctrlutil.RequestCtx{Ctx: ctx}, k8sClient, pvc)).Should(Succeed())

		Eventually(testdbaas.GetClusterPhase(ctx, testCtx, clusterObject.Name),
			timeout, interval).Should(Equal(dbaasv1alpha1.RunningPhase))
	}

	Context("Test VolumeExpansion", func() {
		It("VolumeExpansion should work", func() {
			_, _, clusterObject := testdbaas.InitConsensusMysql(ctx, testCtx, clusterDefinitionName,
				clusterVersionName, clusterName, consensusCompName)
			// init storageClass
			_ = assureDefaultStorageClassObj()

			opsRes := &OpsResource{
				Ctx:      context.Background(),
				Cluster:  clusterObject,
				Client:   k8sClient,
				Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
			}

			By("Test OpsManager.MainEnter function with ClusterOps")
			patch := client.MergeFrom(clusterObject.DeepCopy())
			clusterObject.Status.Phase = dbaasv1alpha1.RunningPhase
			clusterObject.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
				consensusCompName: {
					Phase: dbaasv1alpha1.RunningPhase,
				},
			}
			clusterObject.Status.Operations = &dbaasv1alpha1.Operations{
				VolumeExpandable: []dbaasv1alpha1.OperationComponent{
					{
						Name:                     consensusCompName,
						VolumeClaimTemplateNames: []string{vctName},
					},
				},
			}
			Expect(k8sClient.Status().Patch(context.Background(), clusterObject, patch)).Should(Succeed())

			By("Test VolumeExpansion")
			testVolumeExpansion(clusterObject, opsRes, randomStr)

			By("Test delete the Running VolumeExpansion OpsRequest")
			testDeleteRunningVolumeExpansion(clusterObject, opsRes)
		})
	})
})
