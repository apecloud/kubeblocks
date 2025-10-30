/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package component

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("pre-terminate transformer test", func() {
	const (
		compDefName = "test-compdef"
		clusterName = "test-cluster"
		compName    = "comp"
	)

	var (
		reader   *appsutil.MockReader
		dag      *graph.DAG
		transCtx *componentTransformContext
	)

	newDAG := func(graphCli model.GraphClient, comp *appsv1.Component) *graph.DAG {
		d := graph.NewDAG()
		graphCli.Root(d, comp, comp, model.ActionStatusPtr())
		return d
	}

	provisioned := func(its *workloads.InstanceSet) {
		its.Status.InstanceStatus = []workloads.InstanceStatus{
			{
				PodName:     fmt.Sprintf("%s-0", its.Name),
				Provisioned: true,
			},
		}
	}

	BeforeEach(func() {
		compDef := &appsv1.ComponentDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: compDefName,
			},
			Spec: appsv1.ComponentDefinitionSpec{
				LifecycleActions: &appsv1.ComponentLifecycleActions{
					PreTerminate: testapps.NewLifecycleAction("pre-terminate"),
				},
			},
		}

		comp := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      constant.GenerateClusterComponentName(clusterName, compName),
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: compName,
				},
				Annotations: map[string]string{
					constant.KBAppClusterUIDKey: string(uuid.NewUUID()),
				},
				DeletionTimestamp: ptr.To(metav1.Now()),
			},
			Spec: appsv1.ComponentSpec{
				CompDef:  compDef.Name,
				Replicas: 1,
			},
			Status: appsv1.ComponentStatus{
				Phase: appsv1.DeletingComponentPhase,
			},
		}

		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      constant.GenerateWorkloadNamePattern(clusterName, compName),
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: compName,
				},
			},
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To(int32(1)),
			},
		}
		provisioned(its)

		reader = &appsutil.MockReader{
			Objects: []client.Object{compDef, its},
		}

		graphCli := model.NewGraphClient(reader)
		dag = newDAG(graphCli, comp)

		transCtx = &componentTransformContext{
			Context:       ctx,
			Client:        graphCli,
			EventRecorder: nil,
			Logger:        logger,
			Component:     comp,
			ComponentOrig: comp.DeepCopy(),
		}
	})

	Context("pre-terminate", func() {
		It("ok", func() {
			var (
				preTerminated bool
			)
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					if req.Action == "preTerminate" {
						preTerminated = true
					}
					return kbagentproto.ActionResponse{}, nil
				}).AnyTimes()
			})

			transformer := &componentPreTerminateTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("requeue to waiting for pre-terminate annotation to be set"))
			Expect(preTerminated).Should(BeTrue())
		})

		// It("no pods error", func() {
		//	transformer := &componentPreTerminateTransformer{}
		//	err := transformer.Transform(transCtx, dag)
		//	Expect(err).ShouldNot(BeNil())
		//	Expect(err.Error()).Should(ContainSubstring("has no pods to calling the pre-terminate action"))
		// })

		It("not-defined", func() {
			compDef := reader.Objects[0].(*appsv1.ComponentDefinition)
			compDef.Spec.LifecycleActions.PreTerminate = nil

			transformer := &componentPreTerminateTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
		})

		It("skip by the user", func() {
			transCtx.Component.Annotations[constant.SkipPreTerminateAnnotationKey] = "true"

			transformer := &componentPreTerminateTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
		})

		It("not provisioned", func() {
			its := reader.Objects[1].(*workloads.InstanceSet)
			its.Status.InstanceStatus[0].Provisioned = false

			transformer := &componentPreTerminateTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
		})

		It("not provisioned - has no its object", func() {
			reader.Objects = reader.Objects[:len(reader.Objects)-1]

			transformer := &componentPreTerminateTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
		})
	})
})
