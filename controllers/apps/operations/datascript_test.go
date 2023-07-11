package operations

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("DataScriptOps", func() {
	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr

		clusterObj  *appsv1alpha1.Cluster
		opsResource *OpsResource
		reqCtx      intctrlutil.RequestCtx
	)

	int32Ptr := func(i int32) *int32 {
		return &i
	}

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
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
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	createClusterDatascriptOps := func(comp string, ttlBeforeAbort int32) *appsv1alpha1.OpsRequest {
		opsName := "datascript-ops-" + testCtx.GetRandomStr()
		ops := testapps.NewOpsRequestObj(opsName, testCtx.DefaultNamespace,
			clusterObj.Name, appsv1alpha1.DataScriptType)
		ops.Spec.ScriptSpec = &appsv1alpha1.ScriptSpec{
			ComponentOps: appsv1alpha1.ComponentOps{ComponentName: comp},
			Script:       []string{"CREATE TABLE test (id INT);"},
		}
		ops.Spec.TTLSecondsBeforeAbort = int32Ptr(ttlBeforeAbort)
		Expect(testCtx.CreateObj(testCtx.Ctx, ops)).Should(Succeed())
		return ops
	}

	patchOpsPhase := func(opsKey client.ObjectKey, phase appsv1alpha1.OpsPhase) {
		ops := &appsv1alpha1.OpsRequest{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(testCtx.Ctx, opsKey, ops)).Should(Succeed())
			g.Expect(testapps.ChangeObjStatus(&testCtx, ops, func() {
				ops.Status.Phase = phase
			})).Should(Succeed())
		}).Should(Succeed())
	}

	patchClusterStatus := func(phase appsv1alpha1.ClusterPhase) {
		var compPhase appsv1alpha1.ClusterComponentPhase
		switch phase {
		case appsv1alpha1.RunningClusterPhase:
			compPhase = appsv1alpha1.RunningClusterCompPhase
		case appsv1alpha1.StoppedClusterPhase:
			compPhase = appsv1alpha1.StoppedClusterCompPhase
		case appsv1alpha1.FailedClusterPhase:
			compPhase = appsv1alpha1.FailedClusterCompPhase
		case appsv1alpha1.AbnormalClusterPhase:
			compPhase = appsv1alpha1.AbnormalClusterCompPhase
		case appsv1alpha1.CreatingClusterPhase:
			compPhase = appsv1alpha1.CreatingClusterCompPhase
		case appsv1alpha1.SpecReconcilingClusterPhase:
			compPhase = appsv1alpha1.SpecReconcilingClusterCompPhase
		}

		Expect(testapps.ChangeObjStatus(&testCtx, clusterObj, func() {
			clusterObj.Status.Phase = phase
			clusterObj.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
				consensusComp: {
					Phase: compPhase,
				},
				statelessComp: {
					Phase: compPhase,
				},
				statefulComp: {
					Phase: compPhase,
				},
			}
		})).Should(Succeed())
	}

	Context("with Cluster which has MySQL ConsensusSet", func() {
		BeforeEach(func() {
			By("mock cluster")
			_, _, clusterObj = testapps.InitClusterWithHybridComps(&testCtx, clusterDefinitionName,
				clusterVersionName, clusterName, statelessComp, statefulComp, consensusComp)

			By("init opsResource")
			opsResource = &OpsResource{
				Cluster:  clusterObj,
				Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
			}

			reqCtx = intctrlutil.RequestCtx{
				Ctx: testCtx.Ctx,
				Log: logf.FromContext(testCtx.Ctx).WithValues("datascript", testCtx.DefaultNamespace),
			}
		})

		It("create a datascript ops with ttlSecondsBeforeAbort-0, abort immediately", func() {
			By("patch cluster to creating")
			patchClusterStatus(appsv1alpha1.CreatingClusterPhase)

			By("create a datascript ops with ttlSecondsBeforeAbort=0")
			// create a datascript ops with ttlSecondsBeforeAbort=0
			ops := createClusterDatascriptOps(consensusComp, 0)
			opsKey := client.ObjectKeyFromObject(ops)
			patchOpsPhase(opsKey, appsv1alpha1.OpsCreatingPhase)
			Expect(k8sClient.Get(testCtx.Ctx, opsKey, ops)).Should(Succeed())
			opsResource.OpsRequest = ops

			reqCtx.Req = reconcile.Request{NamespacedName: opsKey}
			By("check the opsRequest phase, should fail")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsResource)
			Expect(err).Should(HaveOccurred())
			Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsFailedPhase))
		})

		It("create a datascript ops with ttlSecondsBeforeAbort=100, should requeue request", func() {
			By("patch cluster to creating")
			patchClusterStatus(appsv1alpha1.CreatingClusterPhase)

			By("create a datascript ops with ttlSecondsBeforeAbort=100")
			// create a datascript ops with ttlSecondsBeforeAbort=0
			ops := createClusterDatascriptOps(consensusComp, 100)
			opsKey := client.ObjectKeyFromObject(ops)
			patchOpsPhase(opsKey, appsv1alpha1.OpsPendingPhase)
			Expect(k8sClient.Get(testCtx.Ctx, opsKey, ops)).Should(Succeed())
			opsResource.OpsRequest = ops
			prevOpsStatus := ops.Status.Phase

			reqCtx.Req = reconcile.Request{NamespacedName: opsKey}
			By("check the opsRequest phase")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsResource)
			Expect(err).Should(Succeed())
			Expect(ops.Status.Phase).Should(Equal(prevOpsStatus))
		})

		It("create a datascript ops on running cluster", func() {
			By("patch cluster to running")
			patchClusterStatus(appsv1alpha1.RunningClusterPhase)

			By("create a datascript ops with ttlSecondsBeforeAbort=0")
			ops := createClusterDatascriptOps(consensusComp, 0)
			opsResource.OpsRequest = ops
			opsKey := client.ObjectKeyFromObject(ops)
			patchOpsPhase(opsKey, appsv1alpha1.OpsCreatingPhase)
			Expect(k8sClient.Get(testCtx.Ctx, opsKey, ops)).Should(Succeed())
			opsResource.OpsRequest = ops

			reqCtx.Req = reconcile.Request{NamespacedName: opsKey}
			By("check the opsRequest phase, should fail, cause pod is missing")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsResource)
			Expect(err).Should(HaveOccurred())
			Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsFailedPhase))
		})

		It("reconcile a datascript ops on running cluster", func() {
			By("patch cluster to running")
			patchClusterStatus(appsv1alpha1.RunningClusterPhase)

			By("create a datascript ops with ttlSecondsBeforeAbort=0")
			ops := createClusterDatascriptOps(consensusComp, 0)
			opsResource.OpsRequest = ops
			opsKey := client.ObjectKeyFromObject(ops)
			patchOpsPhase(opsKey, appsv1alpha1.OpsRunningPhase)
			Expect(k8sClient.Get(testCtx.Ctx, opsKey, ops)).Should(Succeed())
			opsResource.OpsRequest = ops

			reqCtx.Req = reconcile.Request{NamespacedName: opsKey}
			By("mock a job")
			comp := clusterObj.Spec.GetComponentByName(consensusComp)
			job, err := buildDataScriptJob(clusterObj, comp, ops, "mock-script")
			Expect(err).Should(Succeed())
			Expect(k8sClient.Create(testCtx.Ctx, job)).Should(Succeed())

			By("reconcile the opsRequest phase")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(err).Should(Succeed())
			Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsRunningPhase))
		})

		It("reconcile a datascript ops on running cluster, patch job to succeed", func() {
			By("patch cluster to running")
			patchClusterStatus(appsv1alpha1.RunningClusterPhase)

			By("create a datascript ops with ttlSecondsBeforeAbort=0")
			ops := createClusterDatascriptOps(consensusComp, 0)
			opsResource.OpsRequest = ops
			opsKey := client.ObjectKeyFromObject(ops)
			patchOpsPhase(opsKey, appsv1alpha1.OpsRunningPhase)
			Expect(k8sClient.Get(testCtx.Ctx, opsKey, ops)).Should(Succeed())
			opsResource.OpsRequest = ops

			reqCtx.Req = reconcile.Request{NamespacedName: opsKey}
			By("mock a job")
			comp := clusterObj.Spec.GetComponentByName(consensusComp)
			job, err := buildDataScriptJob(clusterObj, comp, ops, "mock-script")
			Expect(err).Should(Succeed())
			Expect(k8sClient.Create(testCtx.Ctx, job)).Should(Succeed())
			// patch job to running

			By("reconcile the opsRequest phase")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(err).Should(Succeed())
			Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsRunningPhase))

			By("patch job to succeed")
			Eventually(func(g Gomega) {
				g.Expect(testapps.ChangeObjStatus(&testCtx, job, func() {
					job.Status.Succeeded = 1
					job.Status.Conditions = append(job.Status.Conditions,
						batchv1.JobCondition{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						})
				}))
			}).Should(Succeed())

			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(err).Should(Succeed())
			Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsSucceedPhase))
		})

		It("reconcile a datascript ops on running cluster, patch job to failed", func() {
			By("patch cluster to running")
			patchClusterStatus(appsv1alpha1.RunningClusterPhase)

			By("create a datascript ops with ttlSecondsBeforeAbort=0")
			ops := createClusterDatascriptOps(consensusComp, 0)
			opsResource.OpsRequest = ops
			opsKey := client.ObjectKeyFromObject(ops)
			patchOpsPhase(opsKey, appsv1alpha1.OpsRunningPhase)
			Expect(k8sClient.Get(testCtx.Ctx, opsKey, ops)).Should(Succeed())
			opsResource.OpsRequest = ops

			reqCtx.Req = reconcile.Request{NamespacedName: opsKey}
			By("mock a job")
			comp := clusterObj.Spec.GetComponentByName(consensusComp)
			job, err := buildDataScriptJob(clusterObj, comp, ops, "mock-script")
			Expect(err).Should(Succeed())
			Expect(k8sClient.Create(testCtx.Ctx, job)).Should(Succeed())
			// patch job to running

			By("reconcile the opsRequest phase")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(err).Should(Succeed())
			Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsRunningPhase))

			By("patch job to succeed")
			Eventually(func(g Gomega) {
				g.Expect(testapps.ChangeObjStatus(&testCtx, job, func() {
					job.Status.Failed = 1
					job.Status.Conditions = append(job.Status.Conditions,
						batchv1.JobCondition{
							Type:   batchv1.JobFailed,
							Status: corev1.ConditionTrue,
						})
				}))
			}).Should(Succeed())

			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(err).Should(Succeed())
			Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsFailedPhase))
		})

		It("parse script from psec", func() {
			cmName := "test-configmap"
			secretName := "test-secret"

			opsName := "datascript-ops-" + testCtx.GetRandomStr()
			ops := testapps.NewOpsRequestObj(opsName, testCtx.DefaultNamespace,
				clusterObj.Name, appsv1alpha1.DataScriptType)
			ops.Spec.ScriptSpec = &appsv1alpha1.ScriptSpec{
				ComponentOps: appsv1alpha1.ComponentOps{ComponentName: consensusComp},
				Script:       []string{"CREATE TABLE test (id INT);"},
				ScriptFrom: &appsv1alpha1.ScriptFrom{
					ConfigMapRef: []corev1.ConfigMapKeySelector{
						{
							Key:                  "cm-key",
							LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
						},
					},
					SecretRef: []corev1.SecretKeySelector{
						{
							Key:                  "secret-key",
							LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						},
					},
				},
			}
			reqCtx.Req = reconcile.Request{NamespacedName: client.ObjectKeyFromObject(ops)}
			_, err := getScriptContent(reqCtx, k8sClient, ops.Spec.ScriptSpec)
			Expect(err).Should(HaveOccurred())

			// create configmap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: testCtx.DefaultNamespace,
				},
				Data: map[string]string{
					"cm-key": "CREATE TABLE t1 (id INT);",
				},
			}

			Expect(k8sClient.Create(testCtx.Ctx, configMap)).Should(Succeed())
			_, err = getScriptContent(reqCtx, k8sClient, ops.Spec.ScriptSpec)
			Expect(err).Should(HaveOccurred())

			// create configmap
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: testCtx.DefaultNamespace,
				},
				StringData: map[string]string{
					"secret-key": "CREATE TABLE t1 (id INT);",
				},
			}
			Expect(k8sClient.Create(testCtx.Ctx, secret)).Should(Succeed())
			_, err = getScriptContent(reqCtx, k8sClient, ops.Spec.ScriptSpec)
			Expect(err).Should(Succeed())
		})
	})
})
