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

package cluster

import (
	"errors"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("clusterDeletionTransformer", func() {
	var (
		transCtx   *clusterTransformContext
		reader     client.Reader
		dag        *graph.DAG
		clusterDef *appsv1.ClusterDefinition
		cluster    *appsv1.Cluster
	)

	newDag := func(graphCli model.GraphClient) *graph.DAG {
		dag = graph.NewDAG()
		graphCli.Root(dag, transCtx.OrigCluster, transCtx.Cluster, model.ActionStatusPtr())
		return dag
	}

	BeforeEach(func() {
		clusterDef = testapps.NewClusterDefFactory("test-clusterdef").
			AddClusterTopology(appsv1.ClusterTopology{
				Name: "default",
				Components: []appsv1.ClusterTopologyComponent{
					{Name: "comp1", CompDef: "compdef1"},
					{Name: "comp2", CompDef: "compdef2"},
					{Name: "comp3", CompDef: "compdef3"},
				},
				Orders: &appsv1.ClusterTopologyOrders{
					Terminate: []string{"comp1", "comp2", "comp3"},
				},
			}).
			GetObject()
		clusterDef.Status.Phase = appsv1.AvailablePhase

		cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, "test-cluster", clusterDef.Name).
			SetTopology(clusterDef.Spec.Topologies[0].Name).
			AddComponent("comp1", "compdef1").
			AddComponent("comp2", "compdef2").
			AddComponent("comp3", "compdef3").
			GetObject()
		cluster.DeletionTimestamp = &metav1.Time{Time: time.Now()}

		reader = &appsutil.MockReader{
			Objects: []client.Object{
				clusterDef,
				&appsv1.Component{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      "test-cluster-comp1",
						Labels:    map[string]string{constant.AppInstanceLabelKey: cluster.Name},
					},
				},
				&appsv1.Component{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      "test-cluster-comp2",
						Labels:    map[string]string{constant.AppInstanceLabelKey: cluster.Name},
					},
				},
				&appsv1.Component{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      "test-cluster-comp3",
						Labels:    map[string]string{constant.AppInstanceLabelKey: cluster.Name},
					},
				},
			},
		}

		transCtx = &clusterTransformContext{
			Context:       testCtx.Ctx,
			Client:        model.NewGraphClient(reader),
			EventRecorder: clusterRecorder,
			Logger:        logger,
			Cluster:       cluster.DeepCopy(),
			OrigCluster:   cluster,
			clusterDef:    clusterDef,
		}
		dag = newDag(transCtx.Client.(model.GraphClient))
	})

	It("w/o terminate orders", func() {
		transCtx.Cluster.Spec.ClusterDef = ""
		transCtx.Cluster.Spec.Topology = ""

		transformer := &clusterDeletionTransformer{}
		err := transformer.Transform(transCtx, dag)
		Expect(err).ShouldNot(BeNil())
		Expect(errors.Is(err, graph.ErrPrematureStop)).Should(BeTrue())
		Expect(dag.Vertices()).Should(HaveLen(1 + 3))
	})

	It("w/ terminate orders", func() {
		transformer := &clusterDeletionTransformer{}
		err := transformer.Transform(transCtx, dag)
		Expect(err).ShouldNot(BeNil())
		Expect(err.Error()).Should(ContainSubstring("are not ready"))
		Expect(dag.Vertices()).Should(HaveLen(1 + 1))

		// delete component 1
		mockReader := reader.(*appsutil.MockReader)
		mockReader.Objects = slices.DeleteFunc(mockReader.Objects, func(obj client.Object) bool {
			return obj.GetName() == "test-cluster-comp1"
		})
		dag = newDag(transCtx.Client.(model.GraphClient))
		err = transformer.Transform(transCtx, dag)
		Expect(err).ShouldNot(BeNil())
		Expect(err.Error()).Should(ContainSubstring("are not ready"))
		Expect(dag.Vertices()).Should(HaveLen(1 + 1))

		// delete component 2
		mockReader.Objects = slices.DeleteFunc(mockReader.Objects, func(obj client.Object) bool {
			return obj.GetName() == "test-cluster-comp2"
		})
		dag = newDag(transCtx.Client.(model.GraphClient))
		err = transformer.Transform(transCtx, dag)
		Expect(err).ShouldNot(BeNil())
		Expect(errors.Is(err, graph.ErrPrematureStop)).Should(BeTrue())
		Expect(dag.Vertices()).Should(HaveLen(1 + 1))

		// delete component 3
		mockReader.Objects = slices.DeleteFunc(mockReader.Objects, func(obj client.Object) bool {
			return obj.GetName() == "test-cluster-comp3"
		})
		dag = newDag(transCtx.Client.(model.GraphClient))
		err = transformer.Transform(transCtx, dag)
		Expect(err).Should(Equal(graph.ErrPrematureStop))
		Expect(dag.Vertices()).Should(HaveLen(1))
	})
})
