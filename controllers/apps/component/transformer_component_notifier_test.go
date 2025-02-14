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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("notifier transformer test", func() {
	const (
		compDefName = "test-compdef"
		clusterName = "test-cluster"
		compName    = "comp"

		compDefNameA           = "test-compdef-a"
		compNameA1, compNameA2 = "compA1", "compA2"
	)

	var (
		reader   *appsutil.MockReader
		dag      *graph.DAG
		transCtx *componentTransformContext

		newDAG = func(graphCli model.GraphClient, comp *appsv1.Component) *graph.DAG {
			d := graph.NewDAG()
			graphCli.Root(d, comp, comp, model.ActionStatusPtr())
			return d
		}
	)

	BeforeEach(func() {
		compDef := &appsv1.ComponentDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: compDefName,
			},
			Spec: appsv1.ComponentDefinitionSpec{},
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
				Generation: 2,
			},
			Spec: appsv1.ComponentSpec{
				Replicas: 3,
			},
			Status: appsv1.ComponentStatus{
				ObservedGeneration: 1,
			},
		}

		compDefA := &appsv1.ComponentDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:       compDefNameA,
				Generation: 1,
			},
			Spec: appsv1.ComponentDefinitionSpec{
				Vars: []appsv1.EnvVar{
					{
						Name: "replicas",
						ValueFrom: &appsv1.VarSource{
							ComponentVarRef: &appsv1.ComponentVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  compDefName,
									Optional: ptr.To(false),
								},
								ComponentVars: appsv1.ComponentVars{
									Replicas: &appsv1.VarRequired,
								},
							},
						},
					},
				},
			},
			Status: appsv1.ComponentDefinitionStatus{
				ObservedGeneration: 1,
				Phase:              appsv1.AvailablePhase,
			},
		}
		compA1 := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      constant.GenerateClusterComponentName(clusterName, compNameA1),
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: compNameA1,
				},
			},
			Spec: appsv1.ComponentSpec{},
		}
		compA2 := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      constant.GenerateClusterComponentName(clusterName, compNameA2),
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: compNameA2,
				},
			},
			Spec: appsv1.ComponentSpec{},
		}

		reader = &appsutil.MockReader{
			Objects: []client.Object{compDefA, compA1, compA2},
		}

		graphCli := model.NewGraphClient(reader)
		dag = newDAG(graphCli, comp)

		transCtx = &componentTransformContext{
			Context:       ctx,
			Client:        graphCli,
			EventRecorder: nil,
			Logger:        logger,
			CompDef:       compDef,
			Component:     comp,
			ComponentOrig: comp.DeepCopy(),
			SynthesizeComponent: &component.SynthesizedComponent{
				Namespace:   testCtx.DefaultNamespace,
				ClusterName: clusterName,
				Comp2CompDefs: map[string]string{
					compName:   compDefName,
					compNameA1: compDefNameA,
					compNameA2: compDefNameA,
				},
				Name:        compName,
				CompDefName: compDefName,
				Replicas:    3,
			},
		}
	})

	checkDependentsNotification := func(notify bool, compNames ...string) {
		graphCli := transCtx.Client.(model.GraphClient)
		objs := graphCli.FindAll(dag, &appsv1.Component{})
		if notify {
			Expect(objs).Should(HaveLen(1 + len(compNames)))
			for _, obj := range objs {
				shortName, _ := component.ShortName(clusterName, obj.GetName())
				if shortName == compName {
					continue
				}
				Expect(compNames).Should(ContainElement(shortName))
				Expect(obj.GetAnnotations()).Should(HaveKey(constant.ReconcileAnnotationKey))
				Expect(obj.GetAnnotations()[constant.ReconcileAnnotationKey]).Should(
					ContainSubstring(fmt.Sprintf("%s@", transCtx.SynthesizeComponent.Name)))
			}
		} else {
			Expect(objs).Should(HaveLen(1))
		}
	}

	Context("notify", func() {
		It("w/o dependent", func() {
			// remove it
			compDef := reader.Objects[0].(*appsv1.ComponentDefinition)
			compDef.Spec.Vars = nil

			transformer := &componentNotifierTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			checkDependentsNotification(false)
		})

		It("w/ dependent", func() {
			transformer := &componentNotifierTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			checkDependentsNotification(true, compNameA1, compNameA2)
		})
	})
})
