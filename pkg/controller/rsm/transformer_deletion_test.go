/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package rsm

import (
	"context"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	apps "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

var _ = Describe("object deletion transformer test.", func() {
	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetUID(uid).
			AddMatchLabelsInMap(selectors).
			SetServiceName(headlessSvcName).
			SetReplicas(3).
			SetRoles(roles).
			SetMembershipReconfiguration(&reconfiguration).
			SetService(service).
			GetObject()
		controller := true
		rsm.OwnerReferences = []metav1.OwnerReference{
			{
				Kind:       reflect.TypeOf(v1alpha1.Cluster{}).Name(),
				Controller: &controller,
			},
		}
		transCtx = &rsmTransformContext{
			Context:       ctx,
			Client:        graphCli,
			EventRecorder: nil,
			Logger:        logger,
			rsmOrig:       rsm.DeepCopy(),
			rsm:           rsm,
		}

		dag = mockDAG()

		nodeAssignment := []workloads.NodeAssignment{
			{
				Name: name + "1",
			},
			{
				Name: name + "2",
			},
			{
				Name: name + "3",
			},
		}
		rsmForPods = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetUID(uid).
			AddMatchLabelsInMap(selectors).
			SetServiceName(headlessSvcName).
			SetRsmTransformPolicy(workloads.ToPod).
			SetReplicas(3).
			SetMembershipReconfiguration(&reconfiguration).
			SetNodeAssignment(nodeAssignment).
			SetService(service).
			GetObject()
		rsmForPods.OwnerReferences = []metav1.OwnerReference{
			{
				Kind:       reflect.TypeOf(v1alpha1.Cluster{}).Name(),
				Controller: &controller,
			},
		}
		transCtxForPods = &rsmTransformContext{
			Context:       ctx,
			Client:        graphCli,
			EventRecorder: nil,
			Logger:        logger,
			rsmOrig:       rsmForPods.DeepCopy(),
			rsm:           rsmForPods,
		}
		dagForPods = mockDAGForPods()

		transformer = &ObjectDeletionTransformer{}
	})

	Context("rsm deletion", func() {
		It("should work well", func() {
			ts := metav1.NewTime(time.Now())
			transCtx.rsmOrig.DeletionTimestamp = &ts
			transCtx.rsm.DeletionTimestamp = &ts
			sts := mockUnderlyingSts(*rsm, rsm.Generation)
			controller := true
			sts.OwnerReferences = []metav1.OwnerReference{
				{
					Kind:       reflect.TypeOf(workloads.ReplicatedStateMachine{}).Name(),
					Controller: &controller,
				},
			}
			headLessSvc := buildHeadlessSvc(*rsm)
			headLessSvc.SetOwnerReferences([]metav1.OwnerReference{
				{
					Kind:       reflect.TypeOf(workloads.ReplicatedStateMachine{}).Name(),
					Controller: &controller,
				},
			})
			envConfig := buildEnvConfigMap(*rsm)
			envConfig.SetOwnerReferences([]metav1.OwnerReference{
				{
					Kind:       reflect.TypeOf(workloads.ReplicatedStateMachine{}).Name(),
					Controller: &controller,
				},
			})
			envConfigShouldNotBeDeleted := buildEnvConfigMap(*rsm)
			envConfigShouldNotBeDeleted.SetOwnerReferences([]metav1.OwnerReference{
				{
					Kind:       reflect.TypeOf(v1alpha1.Cluster{}).Name(),
					Controller: &controller,
				},
			})
			envConfigShouldNotBeDeleted.Name = "env-cm-should-not-be-deleted"
			actionName := getActionName(rsm.Name, int(rsm.Generation), 1, jobTypeSwitchover)
			action := buildAction(rsm, actionName, jobTypeSwitchover, jobScenarioMembership, "", "")
			action.SetOwnerReferences([]metav1.OwnerReference{
				{
					Kind:       reflect.TypeOf(workloads.ReplicatedStateMachine{}).Name(),
					Controller: &controller,
				},
			})
			k8sMock.EXPECT().
				List(gomock.Any(), &apps.StatefulSetList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *apps.StatefulSetList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []apps.StatefulSet{*sts}
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.ServiceList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.ServiceList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []corev1.Service{*headLessSvc}
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.ConfigMapList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []corev1.ConfigMap{*envConfig, *envConfigShouldNotBeDeleted}
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &batchv1.JobList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *batchv1.JobList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []batchv1.Job{*action}
					return nil
				}).Times(1)

			Expect(transformer.Transform(transCtx, dag)).Should(Equal(graph.ErrPrematureStop))
			dagExpected := mockDAG()
			graphCli.Delete(dagExpected, transCtx.rsm)
			graphCli.Delete(dagExpected, action)
			graphCli.Delete(dagExpected, envConfig)
			graphCli.Delete(dagExpected, headLessSvc)
			graphCli.Delete(dagExpected, sts)
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())
		})
	})

	It("should work well if rsm manages pods", func() {
		ts := metav1.NewTime(time.Now())
		transCtxForPods.rsmOrig.DeletionTimestamp = &ts
		transCtxForPods.rsm.DeletionTimestamp = &ts
		pods := mockUnderlyingPods(*rsmForPods)
		controller := true
		for i := 0; i < len(pods); i++ {
			pods[i].OwnerReferences = []metav1.OwnerReference{
				{
					Kind:       reflect.TypeOf(workloads.ReplicatedStateMachine{}).Name(),
					Controller: &controller,
				},
			}
		}
		headLessSvc := buildHeadlessSvc(*rsmForPods)
		headLessSvc.SetOwnerReferences([]metav1.OwnerReference{
			{
				Kind:       reflect.TypeOf(workloads.ReplicatedStateMachine{}).Name(),
				Controller: &controller,
			},
		})
		envConfig := buildEnvConfigMap(*rsmForPods)
		envConfig.SetOwnerReferences([]metav1.OwnerReference{
			{
				Kind:       reflect.TypeOf(workloads.ReplicatedStateMachine{}).Name(),
				Controller: &controller,
			},
		})
		envConfigShouldNotBeDeleted := buildEnvConfigMap(*rsmForPods)
		envConfigShouldNotBeDeleted.SetOwnerReferences([]metav1.OwnerReference{
			{
				Kind:       reflect.TypeOf(v1alpha1.Cluster{}).Name(),
				Controller: &controller,
			},
		})
		envConfigShouldNotBeDeleted.Name = "env-cm-should-not-be-deleted"
		actionName := getActionName(rsmForPods.Name, int(rsmForPods.Generation), 1, jobTypeSwitchover)
		action := buildAction(rsmForPods, actionName, jobTypeSwitchover, jobScenarioMembership, "", "")
		action.SetOwnerReferences([]metav1.OwnerReference{
			{
				Kind:       reflect.TypeOf(workloads.ReplicatedStateMachine{}).Name(),
				Controller: &controller,
			},
		})
		k8sMock.EXPECT().
			List(gomock.Any(), &corev1.PodList{}, gomock.Any()).
			DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
				Expect(list).ShouldNot(BeNil())
				list.Items = pods
				return nil
			}).Times(1)
		k8sMock.EXPECT().
			List(gomock.Any(), &corev1.ServiceList{}, gomock.Any()).
			DoAndReturn(func(_ context.Context, list *corev1.ServiceList, _ ...client.ListOption) error {
				Expect(list).ShouldNot(BeNil())
				list.Items = []corev1.Service{*headLessSvc}
				return nil
			}).Times(1)
		k8sMock.EXPECT().
			List(gomock.Any(), &corev1.ConfigMapList{}, gomock.Any()).
			DoAndReturn(func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
				Expect(list).ShouldNot(BeNil())
				list.Items = []corev1.ConfigMap{*envConfig, *envConfigShouldNotBeDeleted}
				return nil
			}).Times(1)
		k8sMock.EXPECT().
			List(gomock.Any(), &batchv1.JobList{}, gomock.Any()).
			DoAndReturn(func(_ context.Context, list *batchv1.JobList, _ ...client.ListOption) error {
				Expect(list).ShouldNot(BeNil())
				list.Items = []batchv1.Job{*action}
				return nil
			}).Times(1)

		Expect(transformer.Transform(transCtxForPods, dagForPods)).Should(Equal(graph.ErrPrematureStop))
		dagExpected := mockDAGForPods()
		graphCli.Delete(dagExpected, transCtxForPods.rsm)
		graphCli.Delete(dagExpected, action)
		graphCli.Delete(dagExpected, envConfig)
		graphCli.Delete(dagExpected, headLessSvc)
		for i := 0; i < len(pods); i++ {
			graphCli.Delete(dagExpected, &pods[i])
		}
		Expect(dagForPods.Equals(dagExpected, less)).Should(BeTrue())
	})
})
