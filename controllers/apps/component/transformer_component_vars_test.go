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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("vars transformer test", func() {
	const (
		clusterName = "test-cluster"
		compName    = "comp"
	)

	var (
		reader   *appsutil.MockReader
		dag      *graph.DAG
		transCtx *componentTransformContext
		newDAG   = func(graphCli model.GraphClient, comp *appsv1.Component) *graph.DAG {
			d := graph.NewDAG()
			graphCli.Root(d, comp, comp, model.ActionStatusPtr())
			return d
		}
	)

	BeforeEach(func() {
		reader = &appsutil.MockReader{
			Objects: []client.Object{},
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
			},
			Spec: appsv1.ComponentSpec{},
		}

		graphCli := model.NewGraphClient(reader)
		dag = newDAG(graphCli, comp)

		transCtx = &componentTransformContext{
			Context:   ctx,
			Client:    graphCli,
			Component: comp,
			SynthesizeComponent: &component.SynthesizedComponent{
				Namespace:   testCtx.DefaultNamespace,
				ClusterName: clusterName,
				Name:        compName,
			},
		}
	})

	checkEnvCM := func(action *model.Action, data map[string]string) {
		graphCli := transCtx.Client.(model.GraphClient)
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      constant.GenerateClusterComponentEnvPattern(clusterName, compName),
			},
		}
		if action != nil {
			v := graphCli.FindMatchedVertex(dag, cm)
			Expect(v).ShouldNot(BeNil())
			Expect(v.(*model.ObjectVertex).Action).ShouldNot(BeNil())
			Expect(*v.(*model.ObjectVertex).Action).Should(Equal(*action))
		}
		if data != nil {
			objs := graphCli.FindAll(dag, cm)
			Expect(objs).Should(HaveLen(1))
			Expect(objs[0].(*corev1.ConfigMap).Data).Should(Equal(data))
		}
	}

	Context("createOrUpdateEnvConfigMap", func() {
		It("vars env", func() {
			err := createOrUpdateEnvConfigMap(transCtx, dag, map[string]string{
				"foo": "bar",
			})
			Expect(err).Should(BeNil())
			checkEnvCM(model.ActionCreatePtr(), map[string]string{"foo": "bar"})
		})

		It("vars env + task env", func() {
			err := createOrUpdateEnvConfigMap(transCtx, dag, map[string]string{
				"foo": "bar",
			})
			Expect(err).Should(BeNil())
			checkEnvCM(model.ActionCreatePtr(), map[string]string{"foo": "bar"})

			err = createOrUpdateEnvConfigMap(transCtx, dag, map[string]string{
				"task": "scale-out",
			})
			Expect(err).Should(BeNil())
			checkEnvCM(model.ActionCreatePtr(), map[string]string{"foo": "bar", "task": "scale-out"})
		})

		It("overwrite", func() {
			err := createOrUpdateEnvConfigMap(transCtx, dag, map[string]string{
				"foo":  "bar",
				"task": "nil",
			})
			Expect(err).Should(BeNil())
			checkEnvCM(model.ActionCreatePtr(), map[string]string{"foo": "bar", "task": "nil"})

			err = createOrUpdateEnvConfigMap(transCtx, dag, map[string]string{
				"task": "scale-out",
			})
			Expect(err).Should(BeNil())
			checkEnvCM(model.ActionCreatePtr(), map[string]string{"foo": "bar", "task": "scale-out"})
		})

		It("update env", func() {
			// mock the env CM object
			reader.Objects = append(reader.Objects, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      constant.GenerateClusterComponentEnvPattern(clusterName, compName),
				},
				Data: map[string]string{
					"foo": "bar",
				},
			})

			err := createOrUpdateEnvConfigMap(transCtx, dag, map[string]string{
				"foo":  "bingo",
				"task": "nil",
			})
			Expect(err).Should(BeNil())
			checkEnvCM(model.ActionUpdatePtr(), map[string]string{"foo": "bingo", "task": "nil"})
		})

		It("delete env", func() {
			// mock the env CM object
			reader.Objects = append(reader.Objects, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      constant.GenerateClusterComponentEnvPattern(clusterName, compName),
				},
				Data: map[string]string{
					"foo":     "bar",
					"deleted": "anyway",
				},
			})

			err := createOrUpdateEnvConfigMap(transCtx, dag, map[string]string{
				"foo":  "bar",
				"task": "nil",
			})
			Expect(err).Should(BeNil())
			checkEnvCM(model.ActionUpdatePtr(), map[string]string{"foo": "bar", "task": "nil"})
		})
	})
})
