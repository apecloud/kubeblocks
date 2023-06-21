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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	apps "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

var _ = Describe("object deletion transformer test.", func() {
	const (
		namespace = "foo"
		name      = "bar"
	)

	var (
		rsm         *workloads.ReplicatedStateMachine
		transCtx    *rsmTransformContext
		dag         *graph.DAG
		transformer ObjectDeletionTransformer
	)
	mockDAG := func() *graph.DAG {
		d := graph.NewDAG()
		model.PrepareStatus(d, transCtx.rsmOrig, transCtx.rsm)
		return d
	}

	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetUID("foo-bar-uid").
			SetReplicas(3).
			GetObject()

		ctx := context.Background()
		logger := logf.FromContext(ctx).WithValues("rsm-test", namespace)
		transCtx = &rsmTransformContext{
			Context:       ctx,
			Client:        k8sMock,
			EventRecorder: nil,
			Logger:        logger,
			rsmOrig:       rsm.DeepCopy(),
			rsm:           rsm,
		}

		dag = mockDAG()
		transformer = ObjectDeletionTransformer{}
	})

	Context("rsm deletion", func() {
		It("should work well", func() {
			ts := metav1.NewTime(time.Now())
			transCtx.rsmOrig.DeletionTimestamp = &ts
			transCtx.rsm.DeletionTimestamp = &ts
			sts := mockUnderlyingSts(*rsm, rsm.Generation)
			headLessSvc := buildHeadlessSvc(*rsm)
			envConfig := buildEnvConfigMap(*rsm)
			actionName := getActionName(rsm.Name, int(rsm.Generation), 1, jobTypeSwitchover)
			action := builder.NewJobBuilder(name, actionName).
				AddLabelsInMap(map[string]string{
					model.AppInstanceLabelKey: rsm.Name,
					model.KBManagedByKey:      kindReplicatedStateMachine,
					jobScenarioLabel:          jobScenarioMembership,
					jobTypeLabel:              jobTypeSwitchover,
					jobHandledLabel:           jobHandledFalse,
				}).
				SetSuspend(false).
				GetObject()
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
					list.Items = []corev1.ConfigMap{*envConfig}
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &batchv1.JobList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *batchv1.JobList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []batchv1.Job{*action}
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					return nil
				}).Times(3)

			Expect(transformer.Transform(transCtx, dag)).Should(Equal(graph.ErrPrematureStop))
			dagExpected := mockDAG()
			model.PrepareDelete(dagExpected, action)
			model.PrepareDelete(dagExpected, envConfig)
			model.PrepareDelete(dagExpected, headLessSvc)
			model.PrepareDelete(dagExpected, sts)
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())
		})
	})
})
