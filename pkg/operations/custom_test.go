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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

var _ = Describe("CustomOps", func() {
	var (
		randomStr     = testCtx.GetRandomStr()
		compDefName   = "test-compdef-" + randomStr
		clusterName   = "test-cluster-" + randomStr
		opsResource   *OpsResource
		compObj       *appsv1.Component
		opsDef        *opsv1alpha1.OpsDefinition
		reqCtx        intctrlutil.RequestCtx
		cluster       *appsv1.Cluster
		requiredParam = "sql"
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), cluster definition
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.JobSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.ComponentSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.SecretSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.ConfigMapSignature, inNS, ml)

		// non-namespaced
		testapps.ClearResources(&testCtx, generics.OpsDefinitionSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	It("keeps ManagedJob validation narrow without relaxing ordinary sharding Custom Ops", func() {
		newManagedDefinition := func() *opsv1alpha1.OpsDefinition {
			return &opsv1alpha1.OpsDefinition{Spec: opsv1alpha1.OpsDefinitionSpec{
				Actions: []opsv1alpha1.OpsAction{{
					Name:          "managed",
					FailurePolicy: opsv1alpha1.FailurePolicyFail,
					Workload: &opsv1alpha1.OpsWorkloadAction{
						Type:         opsv1alpha1.ManagedJobWorkload,
						BackoffLimit: 0,
						PodSpec: corev1.PodSpec{Containers: []corev1.Container{{
							Name: "runner", Image: "busybox",
						}}},
					},
				}},
			}}
		}
		newSnapshottedCustom := func() *opsv1alpha1.CustomOps {
			return &opsv1alpha1.CustomOps{
				ExecutionSnapshot: &opsv1alpha1.CustomOpsExecutionSnapshot{
					OpsDefinitionUID:        "opsdef-uid",
					OpsDefinitionGeneration: 1,
					OpsDefinitionSpecHash:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					TargetSnapshotHash:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				},
				CustomOpsComponents: []opsv1alpha1.CustomOpsComponent{{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "redis-a"},
				}},
			}
		}

		Expect(ValidateManagedJobOpsDefinition(newSnapshottedCustom(), newManagedDefinition())).Should(Succeed())
		emptyManagedComponents := newSnapshottedCustom()
		emptyManagedComponents.CustomOpsComponents = nil
		err := ValidateManagedJobOpsDefinition(emptyManagedComponents, newManagedDefinition())
		Expect(err).Should(MatchError("ManagedJob requires exactly one Custom OpsRequest component"))
		Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())

		multipleManagedComponents := newSnapshottedCustom()
		multipleManagedComponents.CustomOpsComponents = append(multipleManagedComponents.CustomOpsComponents,
			opsv1alpha1.CustomOpsComponent{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "redis-b"}})
		err = ValidateManagedJobOpsDefinition(multipleManagedComponents, newManagedDefinition())
		Expect(err).Should(MatchError("ManagedJob requires exactly one Custom OpsRequest component"))
		Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())

		ordinaryMultiComponent := multipleManagedComponents.DeepCopy()
		ordinaryMultiComponent.ExecutionSnapshot = nil
		ordinaryDefinition := newManagedDefinition()
		ordinaryDefinition.Spec.Actions[0].Workload.Type = opsv1alpha1.JobWorkload
		Expect(ValidateManagedJobOpsDefinition(ordinaryMultiComponent, ordinaryDefinition)).Should(Succeed())

		withExtractor := newManagedDefinition()
		withExtractor.Spec.PodInfoExtractors = []opsv1alpha1.PodInfoExtractor{{
			Name: "source",
			PodSelector: opsv1alpha1.PodSelector{
				MultiPodSelectionPolicy: opsv1alpha1.Any,
			},
		}}
		withExtractor.Spec.Actions[0].Workload.PodInfoExtractorName = "source"
		Expect(ValidateManagedJobOpsDefinition(newSnapshottedCustom(), withExtractor)).Should(Succeed())

		cases := []struct {
			name   string
			mutate func(*opsv1alpha1.OpsDefinition, *opsv1alpha1.CustomOps)
		}{
			{name: "missing snapshot", mutate: func(_ *opsv1alpha1.OpsDefinition, custom *opsv1alpha1.CustomOps) {
				custom.ExecutionSnapshot = nil
			}},
			{name: "ordinary Job with snapshot", mutate: func(def *opsv1alpha1.OpsDefinition, _ *opsv1alpha1.CustomOps) {
				def.Spec.Actions[0].Workload.Type = opsv1alpha1.JobWorkload
			}},
			{name: "multiple actions", mutate: func(def *opsv1alpha1.OpsDefinition, _ *opsv1alpha1.CustomOps) {
				def.Spec.Actions = append(def.Spec.Actions, def.Spec.Actions[0])
			}},
			{name: "ignore failures", mutate: func(def *opsv1alpha1.OpsDefinition, _ *opsv1alpha1.CustomOps) {
				def.Spec.Actions[0].FailurePolicy = opsv1alpha1.FailurePolicyIgnore
			}},
			{name: "retry", mutate: func(def *opsv1alpha1.OpsDefinition, _ *opsv1alpha1.CustomOps) {
				def.Spec.Actions[0].Workload.BackoffLimit = 1
			}},
			{name: "missing extractor definition", mutate: func(def *opsv1alpha1.OpsDefinition, _ *opsv1alpha1.CustomOps) {
				def.Spec.Actions[0].Workload.PodInfoExtractorName = "pod"
			}},
			{name: "unreferenced extractor definition", mutate: func(def *opsv1alpha1.OpsDefinition, _ *opsv1alpha1.CustomOps) {
				def.Spec.PodInfoExtractors = []opsv1alpha1.PodInfoExtractor{{
					Name: "pod",
					PodSelector: opsv1alpha1.PodSelector{
						MultiPodSelectionPolicy: opsv1alpha1.Any,
					},
				}}
			}},
			{name: "all-pod extractor", mutate: func(def *opsv1alpha1.OpsDefinition, _ *opsv1alpha1.CustomOps) {
				def.Spec.PodInfoExtractors = []opsv1alpha1.PodInfoExtractor{{
					Name: "pod",
					PodSelector: opsv1alpha1.PodSelector{
						MultiPodSelectionPolicy: opsv1alpha1.All,
					},
				}}
				def.Spec.Actions[0].Workload.PodInfoExtractorName = "pod"
			}},
			{name: "multiple extractor definitions", mutate: func(def *opsv1alpha1.OpsDefinition, _ *opsv1alpha1.CustomOps) {
				def.Spec.PodInfoExtractors = []opsv1alpha1.PodInfoExtractor{
					{Name: "pod", PodSelector: opsv1alpha1.PodSelector{MultiPodSelectionPolicy: opsv1alpha1.Any}},
					{Name: "unused", PodSelector: opsv1alpha1.PodSelector{MultiPodSelectionPolicy: opsv1alpha1.Any}},
				}
				def.Spec.Actions[0].Workload.PodInfoExtractorName = "pod"
			}},
			{name: "role selector", mutate: func(def *opsv1alpha1.OpsDefinition, _ *opsv1alpha1.CustomOps) {
				def.Spec.PodInfoExtractors = []opsv1alpha1.PodInfoExtractor{{
					Name: "pod", PodSelector: opsv1alpha1.PodSelector{Role: "primary", MultiPodSelectionPolicy: opsv1alpha1.Any},
				}}
				def.Spec.Actions[0].Workload.PodInfoExtractorName = "pod"
			}},
			{name: "unsupported field path", mutate: func(def *opsv1alpha1.OpsDefinition, _ *opsv1alpha1.CustomOps) {
				def.Spec.PodInfoExtractors = []opsv1alpha1.PodInfoExtractor{{
					Name: "pod", PodSelector: opsv1alpha1.PodSelector{MultiPodSelectionPolicy: opsv1alpha1.Any},
					Env: []opsv1alpha1.OpsEnvVar{{Name: "NODE", ValueFrom: &opsv1alpha1.OpsVarSource{
						FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
					}}},
				}}
				def.Spec.Actions[0].Workload.PodInfoExtractorName = "pod"
			}},
			{name: "duplicate env names", mutate: func(def *opsv1alpha1.OpsDefinition, _ *opsv1alpha1.CustomOps) {
				env := opsv1alpha1.OpsEnvVar{Name: "DUP", ValueFrom: &opsv1alpha1.OpsVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
				}}
				def.Spec.PodInfoExtractors = []opsv1alpha1.PodInfoExtractor{{
					Name: "pod", PodSelector: opsv1alpha1.PodSelector{MultiPodSelectionPolicy: opsv1alpha1.Any},
					Env: []opsv1alpha1.OpsEnvVar{env, env},
				}}
				def.Spec.Actions[0].Workload.PodInfoExtractorName = "pod"
			}},
			{name: "writable volume mount", mutate: func(def *opsv1alpha1.OpsDefinition, _ *opsv1alpha1.CustomOps) {
				def.Spec.PodInfoExtractors = []opsv1alpha1.PodInfoExtractor{{
					Name: "pod", PodSelector: opsv1alpha1.PodSelector{MultiPodSelectionPolicy: opsv1alpha1.Any},
					VolumeMounts: []corev1.VolumeMount{{Name: "tls", MountPath: "/tls"}},
				}}
				def.Spec.Actions[0].Workload.PodInfoExtractorName = "pod"
			}},
			{name: "component info", mutate: func(def *opsv1alpha1.OpsDefinition, _ *opsv1alpha1.CustomOps) {
				def.Spec.ComponentInfos = []opsv1alpha1.ComponentInfo{{ComponentDefinitionName: "redis"}}
			}},
			{name: "precondition", mutate: func(def *opsv1alpha1.OpsDefinition, _ *opsv1alpha1.CustomOps) {
				def.Spec.PreConditions = []opsv1alpha1.PreCondition{{}}
			}},
		}
		for _, test := range cases {
			By(test.name)
			def := newManagedDefinition()
			custom := newSnapshottedCustom()
			test.mutate(def, custom)
			Expect(ValidateManagedJobOpsDefinition(custom, def)).Should(HaveOccurred())
		}

		ordinaryCluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{Shardings: []appsv1.ClusterSharding{{
			Name: "redis-shard",
			Template: appsv1.ClusterComponentSpec{
				Name: "redis", ComponentDef: "redis",
			},
		}}}}
		_, err = validateAndGetCompSpec(ordinaryCluster, &opsv1alpha1.OpsDefinition{}, "redis-shard", false)
		Expect(err).Should(HaveOccurred())
		Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
		managedSpec, err := validateAndGetCompSpec(ordinaryCluster, &opsv1alpha1.OpsDefinition{}, "redis-shard", true)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(managedSpec.Name).Should(Equal("redis"))
	})

	It("makes snapshotted Custom operation inputs immutable at admission", func() {
		newCustomOpsRequest := func(name string, snapshotted bool) *opsv1alpha1.OpsRequest {
			opsRequest := testops.NewOpsRequestObj(name, testCtx.DefaultNamespace, clusterName, opsv1alpha1.CustomType)
			opsRequest.Spec.CustomOps = &opsv1alpha1.CustomOps{
				OpsDefinitionName: "managed-shard-add",
				CustomOpsComponents: []opsv1alpha1.CustomOpsComponent{{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "redis-shard"},
					Parameters:   []opsv1alpha1.Parameter{{Name: "shardAddToken", Value: "token-a"}},
				}},
			}
			if snapshotted {
				opsRequest.Spec.CustomOps.ExecutionSnapshot = &opsv1alpha1.CustomOpsExecutionSnapshot{
					OpsDefinitionUID:        "opsdef-uid",
					OpsDefinitionGeneration: 1,
					OpsDefinitionSpecHash:   strings.Repeat("a", 64),
					TargetSnapshotHash:      strings.Repeat("b", 64),
				}
			}
			return opsRequest
		}

		snapshotted := newCustomOpsRequest("snapshotted-custom-"+randomStr, true)
		Expect(testCtx.CreateObj(testCtx.Ctx, snapshotted)).Should(Succeed())

		expectInvalidUpdate := func(mutate func(*opsv1alpha1.OpsRequest)) {
			var updateErr error
			Eventually(func() bool {
				current := &opsv1alpha1.OpsRequest{}
				Expect(k8sClient.Get(testCtx.Ctx, client.ObjectKeyFromObject(snapshotted), current)).Should(Succeed())
				mutate(current)
				updateErr = k8sClient.Update(testCtx.Ctx, current)
				return !apierrors.IsConflict(updateErr)
			}).Should(BeTrue())
			Expect(apierrors.IsInvalid(updateErr)).Should(BeTrue(), "update error: %v", updateErr)
		}
		expectInvalidUpdate(func(current *opsv1alpha1.OpsRequest) {
			current.Spec.CustomOps.CustomOpsComponents[0].Parameters[0].Value = "token-b"
		})
		expectInvalidUpdate(func(current *opsv1alpha1.OpsRequest) {
			current.Spec.TimeoutSeconds = ptr.To[int32](30)
		})
		expectInvalidUpdate(func(current *opsv1alpha1.OpsRequest) {
			current.Spec.CustomOps = nil
		})
		expectInvalidUpdate(func(current *opsv1alpha1.OpsRequest) {
			current.Spec.Cancel = true
		})

		ordinary := newCustomOpsRequest("ordinary-custom-"+randomStr, false)
		Expect(testCtx.CreateObj(testCtx.Ctx, ordinary)).Should(Succeed())
		current := &opsv1alpha1.OpsRequest{}
		Expect(k8sClient.Get(testCtx.Ctx, client.ObjectKeyFromObject(ordinary), current)).Should(Succeed())
		current.Spec.CustomOps.CustomOpsComponents[0].Parameters[0].Value = "token-b"
		Expect(k8sClient.Update(testCtx.Ctx, current)).Should(Succeed())
	})

	It("runs workflow status transitions without executing new actions", func() {
		opsRequest := testops.NewOpsRequestObj("workflow-ops-"+randomStr, testCtx.DefaultNamespace,
			clusterName, opsv1alpha1.CustomType)
		opsRequest.Spec.CustomOps = &opsv1alpha1.CustomOps{
			CustomOpsComponents: []opsv1alpha1.CustomOpsComponent{{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
			}},
		}
		clusterObj := &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: testCtx.DefaultNamespace},
			Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name:           defaultCompName,
				ComponentDef:   "cmpd",
				ServiceVersion: "8.0.30",
			}}},
		}
		opsDefinition := &opsv1alpha1.OpsDefinition{Spec: opsv1alpha1.OpsDefinitionSpec{
			Actions: []opsv1alpha1.OpsAction{
				{Name: "ignore-failed", FailurePolicy: opsv1alpha1.FailurePolicyIgnore},
				{Name: "done", FailurePolicy: opsv1alpha1.FailurePolicyFail},
			},
			ComponentInfos: []opsv1alpha1.ComponentInfo{{
				ComponentDefinitionName: "cmpd",
				ImageMappings: []opsv1alpha1.ImageMappings{{
					ServiceVersions: []string{"8.0.30"},
					Images:          map[string]string{"runner": "custom:8.0.30"},
				}},
			}},
		}}
		opsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{
			defaultCompName: {
				ProgressDetails: []opsv1alpha1.ProgressStatusDetail{
					{ActionName: "ignore-failed", Status: opsv1alpha1.FailedProgressStatus},
					{ActionName: "done", Status: opsv1alpha1.SucceedProgressStatus},
				},
			},
		}
		opsResource := &OpsResource{OpsRequest: opsRequest, Cluster: clusterObj, OpsDef: opsDefinition}
		workflow := NewWorkflowContext(intctrlutil.RequestCtx{Ctx: testCtx.Ctx}, k8sClient, opsResource)
		Expect(workflow.getImages(&clusterObj.Spec.ComponentSpecs[0])).Should(HaveKeyWithValue("runner", "custom:8.0.30"))
		Expect(workflow.getImages(&appsv1.ClusterComponentSpec{ComponentDef: "other"})).Should(BeNil())

		status, err := workflow.Run(&opsRequest.Spec.CustomOps.CustomOpsComponents[0])
		Expect(err).ShouldNot(HaveOccurred())
		Expect(status.IsCompleted).Should(BeTrue())
		Expect(status.ExistFailure).Should(BeFalse())
		Expect(status.CompletedCount).Should(Equal(2))

		opsDefinition.Spec.Actions[0].FailurePolicy = opsv1alpha1.FailurePolicyFail
		opsRequest.Status.Components[defaultCompName] = opsv1alpha1.OpsRequestComponentStatus{
			ProgressDetails: []opsv1alpha1.ProgressStatusDetail{
				{ActionName: "ignore-failed", Status: opsv1alpha1.FailedProgressStatus},
			},
		}
		status, err = workflow.Run(&opsRequest.Spec.CustomOps.CustomOpsComponents[0])
		Expect(err).ShouldNot(HaveOccurred())
		Expect(status.IsCompleted).Should(BeTrue())
		Expect(status.ExistFailure).Should(BeTrue())
		Expect(status.CompletedCount).Should(Equal(1))

		opsRequest.Status.Components[defaultCompName] = opsv1alpha1.OpsRequestComponentStatus{}
		status, err = workflow.Run(&opsRequest.Spec.CustomOps.CustomOpsComponents[0])
		Expect(status).Should(BeNil())
		Expect(err).Should(HaveOccurred())
		Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())

		Expect(workflow.getAction(opsv1alpha1.OpsAction{ResourceModifier: &opsv1alpha1.OpsResourceModifierAction{}},
			&opsRequest.Spec.CustomOps.CustomOpsComponents[0], &clusterObj.Spec.ComponentSpecs[0], opsv1alpha1.ProgressStatusDetail{})).Should(BeNil())
	})

	createCustomOps := func(comp string, params []opsv1alpha1.Parameter) *opsv1alpha1.OpsRequest {
		opsName := "custom-ops-" + testCtx.GetRandomStr()
		ops := testops.NewOpsRequestObj(opsName, testCtx.DefaultNamespace,
			cluster.Name, opsv1alpha1.CustomType)
		ops.Spec.CustomOps = &opsv1alpha1.CustomOps{
			OpsDefinitionName: opsDef.Name,
			CustomOpsComponents: []opsv1alpha1.CustomOpsComponent{
				{
					ComponentOps: opsv1alpha1.ComponentOps{
						ComponentName: comp,
					},
					Parameters: params,
				},
			},
		}
		Expect(testCtx.CreateObj(testCtx.Ctx, ops)).Should(Succeed())
		ops.Status.Phase = opsv1alpha1.OpsPendingPhase
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

			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				SetSchedulingPolicy(&appsv1.SchedulingPolicy{
					Tolerations: []corev1.Toleration{
						{Operator: corev1.TolerationOpExists, Key: "test"},
					},
				}).
				WithRandomName().AddComponent(defaultCompName, componentDefObj.Name).
				SetServiceVersion("8.0.30").SetReplicas(1).Create(&testCtx).GetObject()

			fullCompName := constant.GenerateClusterComponentName(cluster.Name, defaultCompName)
			compObj = testapps.NewComponentFactory(testCtx.DefaultNamespace, fullCompName, compDefName).
				AddAnnotations(constant.KBAppClusterUIDKey, string(cluster.UID)).
				AddLabels(constant.AppInstanceLabelKey, cluster.Name).
				SetReplicas(1).
				Create(&testCtx).
				GetObject()

			By("create OpsDefinition")
			opsDef = testapps.CreateCustomizedObj(&testCtx, "resources/mysql-opsdefinition-sql.yaml",
				&opsv1alpha1.OpsDefinition{}, testCtx.UseDefaultNamespace())

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
			params := []opsv1alpha1.Parameter{
				{Name: "test", Value: "test"},
			}
			By(fmt.Sprintf("validate json schema, '%s' parameter is required", requiredParam))
			ops := createCustomOps(defaultCompName, params)
			opsResource.OpsRequest = ops
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(ops.Status.Conditions).ShouldNot(BeEmpty())
			Expect(ops.Status.Conditions[0].Message).Should(ContainSubstring(fmt.Sprintf("%s in body is required", requiredParam)))
			Expect(ops.Status.Phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
		})

		testWithValueFrom := func(requiredParameter opsv1alpha1.Parameter) {
			By("create custom Ops")
			ops := createCustomOps(defaultCompName, []opsv1alpha1.Parameter{requiredParameter})

			By("mock component is Running and opsRequest to Running")
			Expect(testapps.ChangeObjStatus(&testCtx, compObj, func() {
				compObj.Status.Phase = appsv1.RunningComponentPhase
			})).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, ops, func() {
				ops.Status.Phase = opsv1alpha1.OpsRunningPhase
			})).Should(Succeed())

			By("validate pass for json schema")
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ops.Status.Phase).Should(Equal(opsv1alpha1.OpsRunningPhase))

			By("check env of the job")
			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList, client.MatchingLabels{constant.OpsRequestNameLabelKey: ops.Name},
				client.InNamespace(ops.Namespace))).Should(Succeed())
			Expect(len(jobList.Items)).Should(Equal(1))
			container := jobList.Items[0].Spec.Template.Spec.Containers[0]
			for _, v := range container.Env {
				if v.Name == requiredParam {
					if requiredParameter.ValueFrom.SecretKeyRef != nil {
						Expect(v.ValueFrom.SecretKeyRef.Key).Should(Equal(requiredParam))
						Expect(v.ValueFrom.SecretKeyRef.Name).Should(Equal(requiredParameter.ValueFrom.SecretKeyRef.Name))
					} else {
						Expect(v.ValueFrom.ConfigMapKeyRef.Key).Should(Equal(requiredParam))
						Expect(v.ValueFrom.ConfigMapKeyRef.Name).Should(Equal(requiredParameter.ValueFrom.ConfigMapKeyRef.Name))
					}
					break
				}
			}
		}

		It("validate json parameter schemas with secret", func() {
			By("create custom Ops")
			secretName := "param-secret"
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: testCtx.DefaultNamespace,
				},
				StringData: map[string]string{
					requiredParam: "select 1",
				},
			}
			testapps.CreateK8sResource(&testCtx, secret)
			testWithValueFrom(opsv1alpha1.Parameter{
				Name: requiredParam, ValueFrom: &opsv1alpha1.ParameterSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
						Key: requiredParam,
					},
				},
			})
		})

		It("validate json parameter schemas with configMap", func() {
			By("create custom Ops")
			cmName := "param-configmap"
			secret := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: testCtx.DefaultNamespace,
				},
				Data: map[string]string{
					requiredParam: "select 1",
				},
			}
			testapps.CreateK8sResource(&testCtx, secret)
			testWithValueFrom(opsv1alpha1.Parameter{
				Name: requiredParam, ValueFrom: &opsv1alpha1.ParameterSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: cmName,
						},
						Key: requiredParam,
					},
				},
			})
		})

		It("Test custom ops when validate failed ", func() {
			By("create custom Ops")
			params := []opsv1alpha1.Parameter{
				{Name: requiredParam, Value: "select 1"},
			}
			ops := createCustomOps(defaultCompName, params)

			By("validate pass for json schema")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsResource)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ops.Status.Phase).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("validate the expression of preChecks, expect the ops phase to fail if component phase is not Running")
			opsResource.OpsRequest = ops
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ops.Status.Components[defaultCompName].PreCheckResult.Pass).Should(BeFalse())
			Expect(ops.Status.Components[defaultCompName].PreCheckResult.Message).Should(ContainSubstring("Component is not in Running status"))
			Expect(ops.Status.Phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
		})

		testCustomOps := func() {
			By("create custom Ops")
			params := []opsv1alpha1.Parameter{
				{Name: requiredParam, Value: "select 1"},
			}
			ops := createCustomOps(defaultCompName, params)

			By("mock component is Running")
			Expect(testapps.ChangeObjStatus(&testCtx, compObj, func() {
				compObj.Status.Phase = appsv1.RunningComponentPhase
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
			Expect(job.Spec.Template.Spec.Tolerations).Should(HaveLen(1))
			Expect(job.Spec.Template.Spec.Tolerations[0].Key).Should(Equal("test"))
			Expect(job.Spec.Template.Spec.Containers[0].Image).Should(Equal("docker.io/apecloud/apecloud-mysql-server:8.0.30"))
			patchJobPhase(job, batchv1.JobComplete)
			By("reconcile once and make the action succeed")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsResource.OpsRequest.Status.Components[defaultCompName].ProgressDetails[0].Status).Should(Equal(opsv1alpha1.SucceedProgressStatus))

			By("reconcile again and make the opsRequest succeed")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsResource)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsResource.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
		}

		It("Test custom ops when workload job completed ", func() {
			testCustomOps()
		})

		It("Should failed when creating ops with  a sharding component ahd the opsDef misses podInfoExtractors", func() {
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, "", "").
				WithRandomName().AddSharding(defaultCompName, "", compDefName).Create(&testCtx).GetObject()

			params := []opsv1alpha1.Parameter{
				{Name: "sql", Value: "select 1"},
			}
			ops := createCustomOps(defaultCompName, params)
			opsResource.Cluster = cluster
			By("validate pass for json schema")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsResource)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ops.Status.Phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
		})

		It("Test custom ops with sharding cluster", func() {
			By("init environment for sharding cluster")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, "", "").
				SetSchedulingPolicy(&appsv1.SchedulingPolicy{
					Tolerations: []corev1.Toleration{
						{Operator: corev1.TolerationOpExists, Key: "test"},
					},
				}).
				WithRandomName().AddSharding(defaultCompName, "", compDefName).Create(&testCtx).GetObject()
			cluster.Spec.Shardings[0].Template.ServiceVersion = "8.0.30"

			opsResource.Cluster = cluster

			Expect(testapps.ChangeObj(&testCtx, opsDef, func(obj *opsv1alpha1.OpsDefinition) {
				podExtraInfoName := "running-pod"
				obj.Spec.PodInfoExtractors = []opsv1alpha1.PodInfoExtractor{
					{
						Name: podExtraInfoName,
						PodSelector: opsv1alpha1.PodSelector{
							MultiPodSelectionPolicy: opsv1alpha1.Any,
						},
					},
				}
				obj.Spec.Actions[0].Workload.PodInfoExtractorName = podExtraInfoName
			})).Should(Succeed())

			// create a sharding component
			shardingShotCompName := fmt.Sprintf("%s-%s", defaultCompName, rand.String(sharding.ShardIDLength))
			shardingCompName := fmt.Sprintf("%s-%s", cluster.Name, shardingShotCompName)
			compObj = testapps.NewComponentFactory(testCtx.DefaultNamespace, shardingCompName, compDefName).
				AddLabels(constant.AppInstanceLabelKey, cluster.Name).
				AddLabels(constant.KBAppClusterUIDKey, string(cluster.UID)).
				AddLabels(constant.KBAppShardingNameLabelKey, defaultCompName).
				AddLabels(constant.KBAppComponentLabelKey, shardingShotCompName).
				SetReplicas(1).
				Create(&testCtx).
				GetObject()

			// create a pod which belongs to the sharding component
			pod := testapps.MockInstanceSetPod(&testCtx, nil, cluster.Name, shardingShotCompName, shardingCompName+"-0", "")
			Expect(testapps.ChangeObj(&testCtx, pod, func(obj *corev1.Pod) {
				pod.Labels[constant.KBAppShardingNameLabelKey] = defaultCompName
			})).Should(Succeed())

			testCustomOps()
		})
	})
})
