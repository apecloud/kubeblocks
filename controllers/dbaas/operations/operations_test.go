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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var _ = Describe("OpsRequest Ops", func() {

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
		// replicaSetComponent   = "replicasets"
	)

	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &storagev1.StorageClass{},
			client.InNamespace(testCtx.DefaultNamespace),
			client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &appsv1.StatefulSet{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.OpsRequest{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		cleanupObjects()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanupObjects()
	})

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

	assureCfgTplObj := func(tplName, cmName, ns string) (*corev1.ConfigMap, *dbaasv1alpha1.ConfigurationTemplate) {
		By("Assuring an cm obj")

		cfgCM, err := testdata.GetResourceFromTestData[corev1.ConfigMap](
			"operations_config/configcm.yaml",
			testdata.WithNamespacedName(cmName, ns),
		)
		Expect(err).Should(Succeed())
		cfgTpl, err := testdata.GetResourceFromTestData[dbaasv1alpha1.ConfigurationTemplate](
			"operations_config/configtpl.yaml",
			testdata.WithNamespacedName(tplName, ns))
		Expect(err).Should(Succeed())

		Expect(testCtx.CheckedCreateObj(ctx, cfgCM)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, cfgTpl)).Should(Succeed())

		return cfgCM, cfgTpl
	}

	assureConfigInstanceObj := func(clusterName, componentName, ns string, cdComponent dbaasv1alpha1.ClusterDefinitionComponent) *corev1.ConfigMap {
		if cdComponent.ConfigSpec == nil {
			return nil
		}
		var cmObj *corev1.ConfigMap
		for _, tpl := range cdComponent.ConfigSpec.ConfigTemplateRefs {
			cmInsName := cfgcore.GetComponentCMName(clusterName, componentName, tpl)
			cfgCM, err := testdata.GetResourceFromTestData[corev1.ConfigMap]("operations_config/configcm.yaml",
				testdata.WithNamespacedName(cmInsName, ns),
				testdata.WithLabels(
					intctrlutil.AppNameLabelKey, clusterName,
					intctrlutil.AppInstanceLabelKey, clusterName,
					intctrlutil.AppComponentLabelKey, componentName,
					cfgcore.CMConfigurationTplNameLabelKey, tpl.ConfigTplRef,
					cfgcore.CMConfigurationConstraintsNameLabelKey, tpl.ConfigConstraintRef,
					cfgcore.CMConfigurationISVTplLabelKey, tpl.Name,
				),
			)
			Expect(err).Should(Succeed())
			Expect(testCtx.CheckedCreateObj(ctx, cfgCM)).Should(Succeed())
			cmObj = cfgCM
		}
		return cmObj
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
`, testdbaas.ConsensusComponentName, clusterName, vctName, pvcName, scName)
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

	generateOpsRequestObj := func(opsRequestName, clusterName string, opsType dbaasv1alpha1.OpsType) *dbaasv1alpha1.OpsRequest {
		opsYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: %s
  namespace: default
spec:
  clusterRef: %s
  type: %s`, opsRequestName, clusterName, opsType)
		opsRequest := &dbaasv1alpha1.OpsRequest{}
		_ = yaml.Unmarshal([]byte(opsYaml), opsRequest)
		return opsRequest
	}

	createOpsRequest := func(opsRequest *dbaasv1alpha1.OpsRequest) *dbaasv1alpha1.OpsRequest {
		Expect(testCtx.CreateObj(ctx, opsRequest)).Should(Succeed())
		// wait until cluster created
		newOps := &dbaasv1alpha1.OpsRequest{}
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: opsRequest.Name, Namespace: testCtx.DefaultNamespace}, newOps)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		return newOps
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
		Eventually(func() bool {
			myCluster := &dbaasv1alpha1.Cluster{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: cluster.Name, Namespace: testCtx.DefaultNamespace}, myCluster)
			return getOpsRequestNameFromAnnotation(myCluster, dbaasv1alpha1.VolumeExpandingPhase) != nil
		}, timeout, interval).Should(BeTrue())
	}

	initResourcesForVolumeExpansion := func(clusterObject *dbaasv1alpha1.Cluster, opsRes *OpsResource, index int) (*dbaasv1alpha1.OpsRequest, string) {
		currRandomStr := testCtx.GetRandomStr()
		ops := generateOpsRequestObj("volumeexpansion-ops-"+currRandomStr, clusterObject.Name, dbaasv1alpha1.VolumeExpansionType)
		ops.Spec.VolumeExpansionList = []dbaasv1alpha1.VolumeExpansion{
			{
				ComponentOps: dbaasv1alpha1.ComponentOps{ComponentName: testdbaas.ConsensusComponentName},
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
					Name:                     testdbaas.ConsensusComponentName,
				},
			},
		}
		Expect(k8sClient.Status().Patch(ctx, clusterObject, patch)).Should(Succeed())

		// create opsRequest
		newOps := createOpsRequest(ops)

		By("mock do operation on cluster")
		mockDoOperationOnCluster(clusterObject, ops.Name, dbaasv1alpha1.VolumeExpandingPhase)

		// create-pvc
		pvcName := fmt.Sprintf("%s-%s-%s-%d", vctName, clusterObject.Name, testdbaas.ConsensusComponentName, index)
		createPVC(clusterObject.Name, storageClassName, vctName, pvcName)
		// waiting pvc controller mark annotation to OpsRequest
		Eventually(func() bool {
			tmpOps := &dbaasv1alpha1.OpsRequest{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: ops.Name, Namespace: testCtx.DefaultNamespace}, tmpOps)
			if tmpOps.Annotations == nil {
				return false
			}
			_, ok := tmpOps.Annotations[intctrlutil.OpsRequestReconcileAnnotationKey]
			return ok
		}, timeout*2, interval).Should(BeTrue())
		return newOps, pvcName
	}

	mockVolumeExpansionActionAndReconcile := func(opsRes *OpsResource, newOps *dbaasv1alpha1.OpsRequest) {
		patch := client.MergeFrom(newOps.DeepCopy())
		_ = volumeExpansion{}.Action(opsRes)
		newOps.Status.Phase = dbaasv1alpha1.RunningPhase
		newOps.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
		Expect(k8sClient.Status().Patch(ctx, newOps, patch)).Should(Succeed())

		opsRes.OpsRequest = newOps
		_, err := GetOpsManager().Reconcile(opsRes)
		Expect(err == nil).Should(BeTrue())
		Eventually(func() bool {
			tmpOps := &dbaasv1alpha1.OpsRequest{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, tmpOps)
			statusComponents := tmpOps.Status.Components
			return statusComponents != nil && statusComponents[testdbaas.ConsensusComponentName].Phase == dbaasv1alpha1.VolumeExpandingPhase
		}, timeout, interval).Should(BeTrue())
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
		Eventually(func() bool {
			tmpOps := &dbaasv1alpha1.OpsRequest{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, tmpOps)
			statusComponents := tmpOps.Status.Components
			return statusComponents != nil && statusComponents[testdbaas.ConsensusComponentName].Phase == dbaasv1alpha1.VolumeExpandingPhase
		}, timeout, interval).Should(BeTrue())

		// test when the event reach the conditions
		event.Count = 5
		event.FirstTimestamp = metav1.Time{Time: time.Now()}
		event.LastTimestamp = metav1.Time{Time: time.Now().Add(61 * time.Second)}
		Expect(pvcEventHandler.Handle(k8sClient, reqCtx, eventRecorder, event)).Should(Succeed())
		Eventually(func() bool {
			tmpOps := &dbaasv1alpha1.OpsRequest{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, tmpOps)
			vcts := tmpOps.Status.Components[testdbaas.ConsensusComponentName].VolumeClaimTemplates
			if len(vcts) == 0 || len(vcts[vctName].PersistentVolumeClaimStatus) == 0 {
				return false
			}
			return vcts[vctName].PersistentVolumeClaimStatus[pvcName].Status == dbaasv1alpha1.FailedPhase
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
			vcts := tmpOps.Status.Components[testdbaas.ConsensusComponentName].VolumeClaimTemplates
			return len(vcts) > 0 && vcts[vctName].Status == dbaasv1alpha1.RunningPhase
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
		_, _ = GetOpsManager().Reconcile(opsRes)
		Eventually(func() bool {
			tmpOps := &dbaasv1alpha1.OpsRequest{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, tmpOps)
			return tmpOps.Status.Phase == dbaasv1alpha1.SucceedPhase
		}, timeout, interval).Should(BeTrue())

		testWarningEventOnPVC(clusterObject, opsRes)
	}

	testReconfigure := func(clusterObject *dbaasv1alpha1.Cluster,
		opsRes *OpsResource,
		clusterDefObj *dbaasv1alpha1.ClusterDefinition) {
		var (
			cfgObj *corev1.ConfigMap
		)

		{
			// mock cluster is Running to support reconfigure ops
			By("mock cluster status")
			patch := client.MergeFrom(clusterObject.DeepCopy())
			clusterObject.Status.Phase = dbaasv1alpha1.RunningPhase
			Expect(k8sClient.Status().Patch(ctx, clusterObject, patch)).Should(Succeed())
		}

		{
			By("mock config tpl")
			cmObj, tplObj := assureCfgTplObj("mysql-tpl-test", "mysql-cm-test", testCtx.DefaultNamespace)
			By("update clusterdefinition tpl")
			clusterDefObj.Spec.Components[0].ConfigSpec = &dbaasv1alpha1.ConfigurationSpec{
				ConfigTemplateRefs: []dbaasv1alpha1.ConfigTemplate{
					{
						Name:                "mysql-test",
						ConfigTplRef:        cmObj.Name,
						ConfigConstraintRef: tplObj.Name,
						VolumeName:          "mysql-config",
						Namespace:           testCtx.DefaultNamespace,
					},
				},
			}
			Expect(k8sClient.Update(ctx, clusterDefObj)).Should(Succeed())
			By("mock config cm object")
			cfgObj = assureConfigInstanceObj(clusterName, testdbaas.ConsensusComponentName, testCtx.DefaultNamespace, clusterDefObj.Spec.Components[0])
		}

		eventContext := cfgcore.ConfigEventContext{
			Cfg:       cfgObj,
			Component: &clusterDefObj.Spec.Components[0],
			Client:    k8sClient,
			ReqCtx: intctrlutil.RequestCtx{
				Ctx:      opsRes.Ctx,
				Log:      log.FromContext(opsRes.Ctx),
				Recorder: opsRes.Recorder,
			},
			Cluster: clusterObject,
		}

		By("mock reconfigure success")
		ops := generateOpsRequestObj("reconfigure-ops-"+randomStr, clusterObject.Name, dbaasv1alpha1.ReconfigureType)
		ops.Spec.Reconfigure = &dbaasv1alpha1.Reconfigure{
			Configurations: []dbaasv1alpha1.Configuration{{
				Name: "mysql-test",
				Keys: []dbaasv1alpha1.ParameterConfig{{
					Key: "my.cnf",
					Parameters: []dbaasv1alpha1.ParameterPair{
						{
							Key:   "binlog_stmt_cache_size",
							Value: func() *string { v := "4096"; return &v }(),
						},
						{
							Key:   "x",
							Value: func() *string { v := "abcd"; return &v }(),
						},
					},
				}},
			}},
			ComponentOps: dbaasv1alpha1.ComponentOps{
				ComponentName: testdbaas.ConsensusComponentName,
			},
		}
		opsRes.OpsRequest = ops
		Expect(testCtx.CheckedCreateObj(ctx, ops)).Should(Succeed())

		reAction := reconfigureAction{}
		Expect(reAction.reconfigure(opsRes)).Should(Succeed())
		Expect(reAction.Handle(eventContext, ops.Name, dbaasv1alpha1.ReconfiguringPhase, nil)).Should(Succeed())
		Expect(opsRes.Client.Get(opsRes.Ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
		_, _ = GetOpsManager().Reconcile(opsRes)
		Expect(opsRes.OpsRequest.Status.Phase).Should(BeEquivalentTo(dbaasv1alpha1.RunningPhase))
		Expect(reAction.Handle(eventContext, ops.Name, dbaasv1alpha1.SucceedPhase, nil)).Should(Succeed())
		Expect(opsRes.Client.Get(opsRes.Ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
		_, _ = GetOpsManager().Reconcile(opsRes)
		Expect(opsRes.OpsRequest.Status.Phase).Should(BeEquivalentTo(dbaasv1alpha1.SucceedPhase))

		// TODO add failed ut
		By("mock reconfigure failed")
	}

	Context("Test OpsRequest", func() {
		It("Should Test all OpsRequest", func() {
			clusterDef, _, clusterObject := testdbaas.InitConsensusMysql(testCtx, clusterDefinitionName, clusterVersionName, clusterName)
			// init storageClass
			_ = assureDefaultStorageClassObj()

			By("Test Upgrade Ops")
			ops := generateOpsRequestObj("upgrade-ops-"+randomStr, clusterObject.Name, dbaasv1alpha1.UpgradeType)
			ops.Spec.Upgrade = &dbaasv1alpha1.Upgrade{ClusterVersionRef: clusterVersionName}
			opsRes := &OpsResource{
				Ctx:        context.Background(),
				Cluster:    clusterObject,
				OpsRequest: ops,
				Client:     k8sClient,
				Recorder:   k8sManager.GetEventRecorderFor("opsrequest-controller"),
			}
			_ = UpgradeAction(opsRes)

			By("Test OpsManager.MainEnter function with ClusterOps")
			opsRes.Cluster.Status.Phase = dbaasv1alpha1.RunningPhase
			patch := client.MergeFrom(clusterObject.DeepCopy())
			clusterObject.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
				testdbaas.ConsensusComponentName: {
					Phase: dbaasv1alpha1.RunningPhase,
				},
			}
			Expect(k8sClient.Status().Patch(context.Background(), clusterObject, patch)).Should(Succeed())
			opsRes.OpsRequest.Status.Phase = dbaasv1alpha1.RunningPhase
			_, _ = GetOpsManager().Reconcile(opsRes)

			By("Test VolumeExpansion")
			testVolumeExpansion(clusterObject, opsRes, randomStr)

			By("Test VerticalScaling")
			ops = generateOpsRequestObj("verticalscaling-ops-"+randomStr, clusterObject.Name, dbaasv1alpha1.VerticalScalingType)
			ops.Spec.VerticalScalingList = []dbaasv1alpha1.VerticalScaling{
				{
					ComponentOps: dbaasv1alpha1.ComponentOps{ComponentName: testdbaas.ConsensusComponentName},
					ResourceRequirements: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("400m"),
							corev1.ResourceMemory: resource.MustParse("300Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("400m"),
							corev1.ResourceMemory: resource.MustParse("300Mi"),
						},
					},
				},
			}
			opsRes.OpsRequest = ops
			_ = VerticalScalingAction(opsRes)

			By("Test Restart")
			ops = generateOpsRequestObj("restart-ops-"+randomStr, clusterObject.Name, dbaasv1alpha1.RestartType)
			ops.Spec.RestartList = []dbaasv1alpha1.ComponentOps{
				{ComponentName: testdbaas.ConsensusComponentName},
			}
			ops.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
			opsRes.OpsRequest = ops
			testdbaas.MockConsensusComponentStatefulSet(testCtx, clusterObject.Name)
			_ = RestartAction(opsRes)

			By("Test HorizontalScaling")
			ops = generateOpsRequestObj("horizontalscaling-ops-"+randomStr, clusterObject.Name, dbaasv1alpha1.HorizontalScalingType)
			ops.Spec.HorizontalScalingList = []dbaasv1alpha1.HorizontalScaling{
				{
					ComponentOps: dbaasv1alpha1.ComponentOps{ComponentName: testdbaas.ConsensusComponentName},
					Replicas:     1,
				},
			}
			opsRes.OpsRequest = ops
			_ = HorizontalScalingAction(opsRes)

			By("Test Reconfigure")
			testReconfigure(clusterObject, opsRes, clusterDef)

			By("Test OpsManager.Do function with ComponentOps")
			_ = GetOpsManager().Do(opsRes)
			opsRes.Cluster.Status.Phase = dbaasv1alpha1.RunningPhase
			opsRes.OpsRequest.Status.Phase = dbaasv1alpha1.RunningPhase
			_ = GetOpsManager().Do(opsRes)
			_, _ = GetOpsManager().Reconcile(opsRes)
			// test getOpsRequestAnnotation function
			opsRes.Cluster.Annotations = map[string]string{
				intctrlutil.OpsRequestAnnotationKey: fmt.Sprintf(`{"Updating":"horizontalscaling-ops-%s"}`, randomStr),
			}
			_ = GetOpsManager().Do(opsRes)

			By("Test OpsManager.Reconcile when opsRequest is succeed")
			opsRes.OpsRequest.Status.Phase = dbaasv1alpha1.SucceedPhase
			opsRes.Cluster.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
				testdbaas.ConsensusComponentName: {
					Phase: dbaasv1alpha1.RunningPhase,
				},
			}
			_, _ = GetOpsManager().Reconcile(opsRes)

			By("Test the functions in ops_util.go")
			_ = patchOpsBehaviourNotFound(opsRes)
			_ = patchClusterPhaseMisMatch(opsRes)
			_ = patchClusterExistOtherOperation(opsRes, "horizontalscaling-ops-"+randomStr)
			_ = PatchClusterNotFound(opsRes)
			_ = patchClusterPhaseWhenExistsOtherOps(opsRes, []OpsRecorder{
				{
					Name:           "mysql-restart",
					ToClusterPhase: dbaasv1alpha1.PendingPhase,
				},
			})
		})
	})
})
