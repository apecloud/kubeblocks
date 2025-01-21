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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
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
				WithRandomName().AddComponent(defaultCompName, componentDefObj.Name).SetReplicas(1).Create(&testCtx).GetObject()

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
			shardingNamePrefix := constant.GenerateClusterComponentName(cluster.Name, defaultCompName)
			shardingCompName := common.SimpleNameGenerator.GenerateName(shardingNamePrefix)
			compObj = testapps.NewComponentFactory(testCtx.DefaultNamespace, shardingCompName, compDefName).
				AddLabels(constant.AppInstanceLabelKey, cluster.Name).
				AddLabels(constant.KBAppClusterUIDKey, string(cluster.UID)).
				AddLabels(constant.KBAppShardingNameLabelKey, defaultCompName).
				AddLabels(constant.KBAppComponentLabelKey, shardingCompName).
				SetReplicas(1).
				Create(&testCtx).
				GetObject()

			// create a pod which belongs to the sharding component
			pod := testapps.MockInstanceSetPod(&testCtx, nil, cluster.Name, defaultCompName, fmt.Sprintf(shardingCompName+"-0"), "")
			Expect(testapps.ChangeObj(&testCtx, pod, func(obj *corev1.Pod) {
				pod.Labels[constant.KBAppShardingNameLabelKey] = defaultCompName
			})).Should(Succeed())

			testCustomOps()
		})
	})
})
