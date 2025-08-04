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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("post-provision transformer test", func() {
	const (
		compDefName = "test-compdef"
		clusterName = "test-cluster"
		compName    = "comp"
	)

	var (
		reader   *appsutil.MockReader
		dag      *graph.DAG
		transCtx *componentTransformContext
		compDef  *appsv1.ComponentDefinition
		comp     *appsv1.Component
	)

	newDAG := func(graphCli model.GraphClient, comp *appsv1.Component) *graph.DAG {
		d := graph.NewDAG()
		graphCli.Root(d, comp, comp, model.ActionStatusPtr())
		return d
	}

	BeforeEach(func() {
		compDef = &appsv1.ComponentDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: compDefName,
			},
			Spec: appsv1.ComponentDefinitionSpec{
				LifecycleActions: &appsv1.ComponentLifecycleActions{
					PostProvision: testapps.NewLifecycleAction("post-provision"),
				},
			},
		}

		comp = &appsv1.Component{
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
			},
			Spec: appsv1.ComponentSpec{
				CompDef:  compDef.Name,
				Replicas: 1,
			},
			Status: appsv1.ComponentStatus{
				Phase: appsv1.CreatingComponentPhase,
			},
		}

		reader = &appsutil.MockReader{
			Objects: []client.Object{compDef, comp},
		}

		graphCli := model.NewGraphClient(reader)
		dag = newDAG(graphCli, comp)
		synthesizeComponent, err := component.BuildSynthesizedComponent(ctx, reader, compDef, comp)
		Expect(err).To(BeNil())

		transCtx = &componentTransformContext{
			Context:             ctx,
			Client:              graphCli,
			EventRecorder:       nil,
			Logger:              logger,
			Component:           comp,
			ComponentOrig:       comp.DeepCopy(),
			SynthesizeComponent: synthesizeComponent,
		}
	})

	Context("post-provision", func() {
		Context("with pods", func() {
			var (
				postProvisionCompleted bool
			)

			BeforeEach(func() {
				postProvisionCompleted = false
				testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
					recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
						if req.Action == "postProvision" {
							postProvisionCompleted = true
						}
						return kbagentproto.ActionResponse{}, nil
					}).AnyTimes()
				})

				reader.Objects = append(reader.Objects, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      fmt.Sprintf("%s-0", constant.GenerateWorkloadNamePattern(clusterName, compName)),
						Labels: map[string]string{
							constant.AppManagedByLabelKey:   constant.AppName,
							constant.AppInstanceLabelKey:    clusterName,
							constant.KBAppComponentLabelKey: compName,
						},
					},
				})
			})

			It("ok", func() {
				transformer := &componentPostProvisionTransformer{}
				err := transformer.Transform(transCtx, dag)
				Expect(err).ShouldNot(BeNil())
				Expect(err.Error()).Should(ContainSubstring("requeue to waiting for post-provision annotation to be set"))
				Expect(postProvisionCompleted).Should(BeTrue())
			})

			It("fails when precondition not met", func() {
				compDef.Spec.LifecycleActions.PostProvision.PreCondition = ptr.To(appsv1.ComponentReadyPreConditionType)
				// need to regenerate synthesizeComponent to make the change to cmpd effective
				synthesizeComponent, err := component.BuildSynthesizedComponent(ctx, reader, compDef, comp)
				Expect(err).To(BeNil())
				transCtx.SynthesizeComponent = synthesizeComponent

				transformer := &componentPostProvisionTransformer{}
				err = transformer.Transform(transCtx, dag)
				Expect(err).ShouldNot(BeNil())
				Expect(err.Error()).Should(ContainSubstring("wait for lifecycle action precondition"))
				Expect(intctrlutil.IsDelayedRequeueError(err)).To(BeTrue())
				Expect(postProvisionCompleted).Should(BeFalse())
			})
		})

		It("no pods error", func() {
			transformer := &componentPostProvisionTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("has no pods to running the post-provision action"))
		})
	})
})
