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
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/dbaas/operations/util"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("OpsRequest Controller", func() {

	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
	)

	const (
		consensusComp       = "consensus"
		statelessComp       = "stateless"
		statefulComp        = "stateful"
		mysqlImageForUpdate = "docker.io/apecloud/apecloud-mysql-server:8.0.30"
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
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
		// default GracePeriod is 30s
		testdbaas.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
		// non-namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	initClusterForOps := func(opsRes *OpsResource) {
		Expect(opsutil.PatchClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
		opsRes.Cluster.Status.Phase = dbaasv1alpha1.RunningPhase
	}

	assureCfgTplObj := func(tplName, cmName, ns string) (*corev1.ConfigMap, *dbaasv1alpha1.ConfigConstraint) {
		By("Assuring an cm obj")

		cfgCM := testdbaas.NewCustomizedObj("operations_config/configcm.yaml",
			&corev1.ConfigMap{}, testdbaas.WithNamespacedName(cmName, ns))
		cfgTpl := testdbaas.NewCustomizedObj("operations_config/configtpl.yaml",
			&dbaasv1alpha1.ConfigConstraint{}, testdbaas.WithNamespacedName(tplName, ns))
		Expect(testCtx.CheckedCreateObj(ctx, cfgCM)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, cfgTpl)).Should(Succeed())

		return cfgCM, cfgTpl
	}

	assureConfigInstanceObj := func(clusterName, componentName, ns string, cdComponent *dbaasv1alpha1.ClusterDefinitionComponent) *corev1.ConfigMap {
		if cdComponent.ConfigSpec == nil {
			return nil
		}
		var cmObj *corev1.ConfigMap
		for _, tpl := range cdComponent.ConfigSpec.ConfigTemplateRefs {
			cmInsName := cfgcore.GetComponentCfgName(clusterName, componentName, tpl.VolumeName)
			cfgCM := testdbaas.NewCustomizedObj("operations_config/configcm.yaml",
				&corev1.ConfigMap{},
				testdbaas.WithNamespacedName(cmInsName, ns),
				testdbaas.WithLabels(
					intctrlutil.AppNameLabelKey, clusterName,
					intctrlutil.AppInstanceLabelKey, clusterName,
					intctrlutil.AppComponentLabelKey, componentName,
					cfgcore.CMConfigurationTplNameLabelKey, tpl.ConfigTplRef,
					cfgcore.CMConfigurationConstraintsNameLabelKey, tpl.ConfigConstraintRef,
					cfgcore.CMConfigurationISVTplLabelKey, tpl.Name,
					cfgcore.CMConfigurationTypeLabelKey, cfgcore.ConfigInstanceType,
				),
			)
			Expect(testCtx.CheckedCreateObj(ctx, cfgCM)).Should(Succeed())
			cmObj = cfgCM
		}
		return cmObj
	}

	testVerticalScaling := func(opsRes *OpsResource) {
		By("Test VerticalScaling")
		ops := testdbaas.NewOpsRequestObj("verticalscaling-ops-"+randomStr, testCtx.DefaultNamespace,
			clusterName, dbaasv1alpha1.VerticalScalingType)
		ops.Spec.VerticalScalingList = []dbaasv1alpha1.VerticalScaling{
			{
				ComponentOps: dbaasv1alpha1.ComponentOps{ComponentName: consensusComp},
				ResourceRequirements: corev1.ResourceRequirements{
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
		opsRes.OpsRequest = testdbaas.CreateOpsRequest(ctx, testCtx, ops)
		initClusterForOps(opsRes)
		By("test save last configuration and OpsRequest phase is Running")
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
		Eventually(testdbaas.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops))).Should(Equal(dbaasv1alpha1.RunningPhase))

		By("test vertical scale action function")
		vsHandler := verticalScalingHandler{}
		Expect(vsHandler.Action(opsRes)).Should(Succeed())
		_, _, err := vsHandler.ReconcileAction(opsRes)
		Expect(err == nil).Should(BeTrue())
	}

	getProgressDetailStatus := func(opsRes *OpsResource,
		componentName string,
		pod *corev1.Pod) dbaasv1alpha1.ProgressStatus {
		objectKey := GetProgressObjectKey(pod.Kind, pod.Name)
		progressDetails := opsRes.OpsRequest.Status.Components[componentName].ProgressDetails
		progressDetail := FindStatusProgressDetail(progressDetails, objectKey)
		var status dbaasv1alpha1.ProgressStatus
		if progressDetail != nil {
			status = progressDetail.Status
		}
		return status
	}

	testConsensusSetPodUpdating := func(opsRes *OpsResource, consensusPodList []corev1.Pod) {
		By("mock pod of statefulSet updating by deleting the pod")
		pod := &consensusPodList[0]
		testk8s.MockPodIsTerminating(ctx, testCtx, pod)
		_, _ = GetOpsManager().Reconcile(opsRes)
		Expect(getProgressDetailStatus(opsRes, consensusComp, pod)).Should(Equal(dbaasv1alpha1.ProcessingProgressStatus))

		By("mock one pod of StatefulSet to update successfully")
		testk8s.RemovePodFinalizer(ctx, testCtx, pod)
		testdbaas.MockConsensusComponentStsPod(testCtx, nil, clusterName, consensusComp,
			pod.Name, "leader", "ReadWrite")

		_, _ = GetOpsManager().Reconcile(opsRes)
		Expect(getProgressDetailStatus(opsRes, consensusComp, pod)).Should(Equal(dbaasv1alpha1.SucceedProgressStatus))
		Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/4"))
	}

	testStatelessPodUpdating := func(opsRes *OpsResource, pod *corev1.Pod) {
		By("create a new pod")
		newPodName := "busybox-" + testCtx.GetRandomStr()
		testdbaas.MockStatelessPod(testCtx, nil, clusterName, statelessComp, newPodName)
		newPod := &corev1.Pod{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: newPodName, Namespace: testCtx.DefaultNamespace}, newPod)).Should(Succeed())
		_, _ = GetOpsManager().Reconcile(opsRes)
		Expect(getProgressDetailStatus(opsRes, statelessComp, newPod)).Should(Equal(dbaasv1alpha1.ProcessingProgressStatus))
		Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/4"))

		By("mock new pod is ready")
		Expect(testdbaas.ChangeObjStatus(&testCtx, newPod, func() {
			lastTransTime := metav1.NewTime(time.Now().Add(-11 * time.Second))
			testk8s.MockPodAvailable(newPod, lastTransTime)
		})).Should(Succeed())

		_, _ = GetOpsManager().Reconcile(opsRes)
		Expect(getProgressDetailStatus(opsRes, statelessComp, newPod)).Should(Equal(dbaasv1alpha1.SucceedProgressStatus))
		Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/4"))
	}

	testRestart := func(opsRes *OpsResource, consensusPodList []corev1.Pod, statelessPod *corev1.Pod) {
		By("Test Restart")
		ops := testdbaas.NewOpsRequestObj("restart-ops-"+randomStr, testCtx.DefaultNamespace,
			clusterName, dbaasv1alpha1.RestartType)
		ops.Spec.RestartList = []dbaasv1alpha1.ComponentOps{
			{ComponentName: consensusComp},
			{ComponentName: statelessComp},
		}

		By("test restart OpsRequest is Running")
		initClusterForOps(opsRes)
		opsRes.OpsRequest = testdbaas.CreateOpsRequest(ctx, testCtx, ops)
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
		Eventually(testdbaas.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops))).Should(Equal(dbaasv1alpha1.RunningPhase))

		By("test restart action and reconcile function")
		testdbaas.MockConsensusComponentStatefulSet(testCtx, clusterName, consensusComp)
		testdbaas.MockStatelessComponentDeploy(testCtx, clusterName, statelessComp)
		rHandler := restartOpsHandler{}
		_ = rHandler.Action(opsRes)
		_, err := GetOpsManager().Reconcile(opsRes)
		Expect(err == nil).Should(BeTrue())

		if !testCtx.UsingExistingCluster() {
			By("mock testing the updates of consensus component")
			testConsensusSetPodUpdating(opsRes, consensusPodList)

			By("mock testing the updates of stateless component")
			Expect(opsRes.OpsRequest.Status.Components[statelessComp].Phase).Should(Equal(dbaasv1alpha1.RebootingPhase))
			testStatelessPodUpdating(opsRes, statelessPod)
		}
	}

	testUpgrade := func(opsRes *OpsResource, clusterObject *dbaasv1alpha1.Cluster) {
		By("Test Upgrade Ops")
		newClusterVersionName := "clusterversion-upgrade-" + randomStr
		_ = testdbaas.NewClusterVersionFactory(newClusterVersionName, clusterDefinitionName).
			AddComponent(statelessComp).AddContainerShort(testdbaas.DefaultNginxContainerName, "nginx:1.14.2").
			AddComponent(consensusComp).AddContainerShort(testdbaas.DefaultMySQLContainerName, mysqlImageForUpdate).
			AddComponent(statefulComp).AddContainerShort(testdbaas.DefaultMySQLContainerName, mysqlImageForUpdate).
			Create(&testCtx).GetObject()
		ops := testdbaas.NewOpsRequestObj("upgrade-ops-"+randomStr, testCtx.DefaultNamespace,
			clusterObject.Name, dbaasv1alpha1.UpgradeType)
		ops.Spec.Upgrade = &dbaasv1alpha1.Upgrade{ClusterVersionRef: newClusterVersionName}
		opsRes.OpsRequest = testdbaas.CreateOpsRequest(ctx, testCtx, ops)

		By("test upgrade OpsRequest phase is Running")
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
		Expect(opsRes.OpsRequest.Status.Phase == dbaasv1alpha1.RunningPhase).Should(BeTrue())

		By("Test OpsManager.MainEnter function ")
		_, _ = GetOpsManager().Reconcile(opsRes)
	}

	createHorizontalScaling := func(replicas int) *dbaasv1alpha1.OpsRequest {
		horizontalOpsName := "horizontalscaling-ops-" + testCtx.GetRandomStr()
		ops := testdbaas.NewOpsRequestObj(horizontalOpsName, testCtx.DefaultNamespace,
			clusterName, dbaasv1alpha1.HorizontalScalingType)
		ops.Spec.HorizontalScalingList = []dbaasv1alpha1.HorizontalScaling{
			{
				ComponentOps: dbaasv1alpha1.ComponentOps{ComponentName: consensusComp},
				Replicas:     int32(replicas),
			},
		}
		return testdbaas.CreateOpsRequest(ctx, testCtx, ops)
	}

	testHorizontalScaling := func(opsRes *OpsResource, podList []corev1.Pod) {
		By("Test HorizontalScaling with scale down replicas")
		opsRes.OpsRequest = createHorizontalScaling(1)
		initClusterForOps(opsRes)

		By("Test HorizontalScaling OpsRequest phase is running and do action")
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
		Expect(opsRes.OpsRequest.Status.Phase == dbaasv1alpha1.RunningPhase).Should(BeTrue())

		By("Test OpsManager.Reconcile function when horizontal scaling OpsRequest is Running")
		opsRes.Cluster.Status.Phase = dbaasv1alpha1.RunningPhase
		_, err := GetOpsManager().Reconcile(opsRes)
		Expect(err == nil).Should(BeTrue())

		if !testCtx.UsingExistingCluster() {
			By("mock the pod is terminating")
			pod := &podList[0]
			testk8s.MockPodIsTerminating(ctx, testCtx, pod)
			_, _ = GetOpsManager().Reconcile(opsRes)
			Expect(getProgressDetailStatus(opsRes, consensusComp, pod)).Should(Equal(dbaasv1alpha1.ProcessingProgressStatus))

			By("mock the pod is deleted and progressDetail status should be succeed")
			testk8s.RemovePodFinalizer(ctx, testCtx, pod)
			_, _ = GetOpsManager().Reconcile(opsRes)
			Expect(getProgressDetailStatus(opsRes, consensusComp, pod)).Should(Equal(dbaasv1alpha1.SucceedProgressStatus))
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/2"))
		}

		By("test GetOpsRequestAnnotation function")
		patch := client.MergeFrom(opsRes.Cluster.DeepCopy())
		opsAnnotationString := fmt.Sprintf(`[{"name":"%s","clusterPhase":"HorizontalScaling"},{"name":"test-not-exists-ops","clusterPhase":"VolumeExpanding"}]`,
			opsRes.OpsRequest.Name)
		opsRes.Cluster.Annotations = map[string]string{
			intctrlutil.OpsRequestAnnotationKey: opsAnnotationString,
		}
		Expect(k8sClient.Patch(ctx, opsRes.Cluster, patch)).Should(Succeed())
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())

		By("Test OpsManager.Reconcile when opsRequest is succeed")
		opsRes.OpsRequest.Status.Phase = dbaasv1alpha1.SucceedPhase
		opsRes.Cluster.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
			consensusComp: {
				Phase: dbaasv1alpha1.RunningPhase,
			},
		}
		_, err = GetOpsManager().Reconcile(opsRes)
		Expect(err == nil).Should(BeTrue())

		By("Test HorizontalScaling with scale up replica")
		initClusterForOps(opsRes)
		expectClusterComponentReplicas := int32(2)
		opsRes.Cluster.Spec.Components[1].Replicas = &expectClusterComponentReplicas
		opsRes.OpsRequest = createHorizontalScaling(3)
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())

		_, err = GetOpsManager().Reconcile(opsRes)
		Expect(err == nil).Should(BeTrue())
		if !testCtx.UsingExistingCluster() {
			By("mock scale up pods")
			podName := fmt.Sprintf("%s-%s-%d", clusterName, consensusComp, 0)
			testdbaas.MockConsensusComponentStsPod(testCtx, nil, clusterName, consensusComp,
				podName, "leader", "ReadWrite")
			pod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, pod)).Should(Succeed())
			_, _ = GetOpsManager().Reconcile(opsRes)
			Expect(getProgressDetailStatus(opsRes, consensusComp, pod)).Should(Equal(dbaasv1alpha1.SucceedProgressStatus))
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/1"))
		}
	}

	testReconfigure := func(clusterObject *dbaasv1alpha1.Cluster,
		opsRes *OpsResource,
		clusterDefObj *dbaasv1alpha1.ClusterDefinition) {
		var (
			cfgObj       *corev1.ConfigMap
			stsComponent *dbaasv1alpha1.ClusterDefinitionComponent
		)

		By("Test Reconfigure")
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
			patch := client.MergeFrom(clusterDefObj.DeepCopy())
			for i := range clusterDefObj.Spec.Components {
				component := &clusterDefObj.Spec.Components[i]
				if component.TypeName != consensusComp {
					continue
				}
				stsComponent = component
				component.ConfigSpec = &dbaasv1alpha1.ConfigurationSpec{
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
			}

			Expect(k8sClient.Patch(ctx, clusterDefObj, patch)).Should(Succeed())
			By("mock config cm object")
			cfgObj = assureConfigInstanceObj(clusterName, consensusComp, testCtx.DefaultNamespace, stsComponent)
		}

		By("mock event context")
		eventContext := cfgcore.ConfigEventContext{
			CfgCM:     cfgObj,
			Component: &clusterDefObj.Spec.Components[0],
			Client:    k8sClient,
			ReqCtx: intctrlutil.RequestCtx{
				Ctx:      opsRes.Ctx,
				Log:      log.FromContext(opsRes.Ctx),
				Recorder: opsRes.Recorder,
			},
			Cluster: clusterObject,
			TplName: "mysql-test",
			ConfigPatch: &cfgcore.ConfigPatchInfo{
				AddConfig:    map[string]interface{}{},
				UpdateConfig: map[string][]byte{},
				DeleteConfig: map[string]interface{}{},
			},
			PolicyStatus: cfgcore.PolicyExecStatus{
				PolicyName:    "simple",
				SucceedCount:  2,
				ExpectedCount: 3,
			},
		}

		By("mock reconfigure success")
		ops := testdbaas.NewOpsRequestObj("reconfigure-ops-"+randomStr, testCtx.DefaultNamespace,
			clusterName, dbaasv1alpha1.ReconfiguringType)
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
			ComponentOps: dbaasv1alpha1.ComponentOps{ComponentName: consensusComp},
		}
		opsRes.OpsRequest = ops
		Expect(testCtx.CheckedCreateObj(ctx, ops)).Should(Succeed())

		reAction := reconfigureAction{}
		Expect(reAction.Action(opsRes)).Should(Succeed())
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
			clusterDef, _, clusterObject := testdbaas.InitClusterWithHybridComps(testCtx, clusterDefinitionName,
				clusterVersionName, clusterName, statelessComp, "stateful", consensusComp)
			opsRes := &OpsResource{
				Ctx:      context.Background(),
				Cluster:  clusterObject,
				Client:   k8sClient,
				Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
			}
			By("mock cluster is Running and the status operations")
			Expect(testdbaas.ChangeObjStatus(&testCtx, clusterObject, func() {
				clusterObject.Status.Phase = dbaasv1alpha1.RunningPhase
				clusterObject.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
					consensusComp: {
						Phase: dbaasv1alpha1.RunningPhase,
					},
					statelessComp: {
						Phase: dbaasv1alpha1.RunningPhase,
					},
				}
				clusterObject.Status.Operations = &dbaasv1alpha1.Operations{
					Upgradable:       true,
					Restartable:      []string{consensusComp, statelessComp},
					VerticalScalable: []string{consensusComp, statelessComp},
					HorizontalScalable: []dbaasv1alpha1.OperationComponent{
						{
							Name: consensusComp,
						},
						{
							Name: statelessComp,
						},
					},
				}
			})).Should(Succeed())
			opsRes.Cluster = clusterObject

			var (
				consensusPodList []corev1.Pod
				statelessPod     = &corev1.Pod{}
			)
			if !testCtx.UsingExistingCluster() {
				// mock the pods of consensusSet component
				testdbaas.MockConsensusComponentPods(testCtx, nil, clusterName, consensusComp)
				podList, err := util.GetComponentPodList(opsRes.Ctx, opsRes.Client, opsRes.Cluster, consensusComp)
				Expect(err).Should(Succeed())
				consensusPodList = podList.Items

				// mock the pods od stateless component
				podName := "busybox-" + randomStr
				testdbaas.MockStatelessPod(testCtx, nil, clusterName, statelessComp, podName)
			}
			// the opsRequest will use startTime to check some condition.
			// if there is no sleep for 1 second, unstable error may occur.
			time.Sleep(time.Second)

			// test upgrade OpsRequest
			testUpgrade(opsRes, clusterObject)

			// test vertical scaling OpsRequest
			testVerticalScaling(opsRes)

			// test restart consensus component and stateless component
			testRestart(opsRes, consensusPodList, statelessPod)

			testReconfigure(clusterObject, opsRes, clusterDef)

			// test horizontalScaling and test the progressDetail
			testHorizontalScaling(opsRes, consensusPodList)

			By("Test the functions in ops_util.go")
			Expect(PatchValidateErrorCondition(opsRes, "validate error")).Should(Succeed())
			Expect(patchOpsHandlerNotSupported(opsRes)).Should(Succeed())
			Expect(isOpsRequestFailedPhase(dbaasv1alpha1.FailedPhase)).Should(BeTrue())
			Expect(PatchClusterNotFound(opsRes)).Should(Succeed())
			opsRecorder := []dbaasv1alpha1.OpsRecorder{
				{
					Name:           "mysql-restart",
					ToClusterPhase: dbaasv1alpha1.RebootingPhase,
				},
			}
			Expect(patchClusterPhaseWhenExistsOtherOps(opsRes, opsRecorder)).Should(Succeed())
			index, opsRecord := GetOpsRecorderFromSlice(opsRecorder, "mysql-restart")
			Expect(index == 0 && opsRecord.Name == "mysql-restart").Should(BeTrue())
		})
	})
})
