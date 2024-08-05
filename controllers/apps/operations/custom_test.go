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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("CustomOps", func() {
	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
		compDefName           = "apecloud-mysql"
		opsResource           *OpsResource
		compObj               *appsv1alpha1.Component
		opsDef                *appsv1alpha1.OpsDefinition
		reqCtx                intctrlutil.RequestCtx
		cluster               *appsv1alpha1.Cluster
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
		testapps.ClearResources(&testCtx, generics.JobSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.ComponentSignature, inNS, ml)

		// non-namespaced
		testapps.ClearResources(&testCtx, generics.OpsDefinitionSignature, ml)
		testapps.ClearResources(&testCtx, generics.ComponentDefinitionSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	createCustomOps := func(comp string, params []appsv1alpha1.Parameter) *appsv1alpha1.OpsRequest {
		opsName := "custom-ops-" + testCtx.GetRandomStr()
		ops := testapps.NewOpsRequestObj(opsName, testCtx.DefaultNamespace,
			cluster.Name, appsv1alpha1.CustomType)
		ops.Spec.CustomOps = &appsv1alpha1.CustomOps{
			OpsDefinitionName: opsDef.Name,
			CustomOpsComponents: []appsv1alpha1.CustomOpsComponent{
				{
					ComponentOps: appsv1alpha1.ComponentOps{
						ComponentName: comp,
					},
					Parameters: params,
				},
			},
		}
		Expect(testCtx.CreateObj(testCtx.Ctx, ops)).Should(Succeed())
		ops.Status.Phase = appsv1alpha1.OpsPendingPhase
		opsResource.OpsRequest = ops
		return ops
	}

	Context("with Cluster which has MySQL ConsensusSet", func() {
		BeforeEach(func() {
			By("create componentDefinition, cluster and component")
			componentDefObj := testapps.NewComponentDefinitionFactory(compDefName).
				SetDefaultSpec().
				Create(&testCtx).
				GetObject()

			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefinitionName).
				WithRandomName().AddComponentV2(consensusComp, componentDefObj.Name).SetReplicas(1).Create(&testCtx).GetObject()

			fullCompName := constant.GenerateClusterComponentName(cluster.Name, consensusComp)
			compObj = testapps.NewComponentFactory(testCtx.DefaultNamespace, fullCompName, compDefName).
				AddLabels(constant.AppInstanceLabelKey, cluster.Name).
				AddLabels(constant.KBAppClusterUIDLabelKey, string(cluster.UID)).
				SetReplicas(1).
				Create(&testCtx).
				GetObject()

			By("create OpsDefinition")
			opsDef = testapps.CreateCustomizedObj(&testCtx, "resources/mysql-opsdefinition-sql.yaml",
				&appsv1alpha1.OpsDefinition{}, testCtx.UseDefaultNamespace())

			By("init opsResource")
			opsResource = &OpsResource{
				Cluster:  cluster,
				Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
				OpsDef:   opsDef,
			}

			reqCtx = intctrlutil.RequestCtx{
				Ctx:      testCtx.Ctx,
				Recorder: opsResource.Recorder,
				Log:      logf.FromContext(testCtx.Ctx).WithValues("customOps", testCtx.DefaultNamespace),
			}
		})

		patchJobPhase := func(job *batchv1.Job, conditionType batchv1.JobConditionType) {
			Expect(testapps.ChangeObjStatus(&testCtx, job, func() {
				job.Status.Conditions = []batchv1.JobCondition{{
					Type: conditionType, Status: corev1.ConditionTrue,
				}}
			})).Should(Succeed())
		}

		It("validate json parameter schemas", func() {
			params := []appsv1alpha1.Parameter{
				{Name: "test", Value: "test"},
			}
			By("validate json schema, 'sql' parameter is required")
			ops := createCustomOps(consensusComp, params)
			opsResource.OpsRequest = ops
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(ops.Status.Conditions).ShouldNot(BeEmpty())
			Expect(ops.Status.Conditions[0].Message).Should(ContainSubstring("sql in body is required"))
			Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsFailedPhase))

		})

		It("Test custom ops when validate failed ", func() {
			By("create custom Ops")
			params := []appsv1alpha1.Parameter{
				{Name: "sql", Value: "select 1"},
			}
			ops := createCustomOps(consensusComp, params)

			By("validate pass for json schema")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsResource)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By("validate the expression of preChecks, expect the ops phase to fail if component phase is not Running")
			opsResource.OpsRequest = ops
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ops.Status.Components[consensusComp].PreCheckResult.Pass).Should(BeFalse())
			Expect(ops.Status.Components[consensusComp].PreCheckResult.Message).Should(ContainSubstring("Component is not in Running status"))
			Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsFailedPhase))
		})

		It("Test custom ops when workload job completed ", func() {
			By("create custom Ops")
			params := []appsv1alpha1.Parameter{
				{Name: "sql", Value: "select 1"},
			}
			ops := createCustomOps(consensusComp, params)

			By("mock component is Running")
			Expect(testapps.ChangeObjStatus(&testCtx, compObj, func() {
				compObj.Status.Phase = appsv1alpha1.RunningClusterCompPhase
			})).Should(Succeed())

			By("job should be created successfully")
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(err).ShouldNot(HaveOccurred())
			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList, client.MatchingLabels{constant.OpsRequestNameLabelKey: ops.Name},
				client.InNamespace(ops.Namespace))).Should(Succeed())
			Expect(len(jobList.Items)).Should(Equal(1))

			By("mock job is completed, expect for ops phase is Succeed")
			job := &jobList.Items[0]
			patchJobPhase(job, batchv1.JobComplete)
			By("reconcile once and make the action succeed")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsResource.OpsRequest.Status.Components[consensusComp].ProgressDetails[0].Status).Should(Equal(appsv1alpha1.SucceedProgressStatus))

			By("reconcile again and make the opsRequest succeed")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsResource.OpsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsSucceedPhase))
		})

	})
})
