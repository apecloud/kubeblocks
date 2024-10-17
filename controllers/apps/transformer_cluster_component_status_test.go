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

package apps

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("cluster component status transformer", func() {
	const (
		compDefName = "test-compdef"
	)

	var (
		transCtx *clusterTransformContext
		dag      *graph.DAG
	)

	newDag := func(graphCli model.GraphClient) *graph.DAG {
		dag = graph.NewDAG()
		graphCli.Root(dag, transCtx.OrigCluster, transCtx.Cluster, model.ActionStatusPtr())
		return dag
	}

	normalizeTransformContext := func(transCtx *clusterTransformContext) {
		var (
			cluster = transCtx.Cluster
			err     error
		)
		transformer := clusterNormalizationTransformer{}
		transCtx.components, transCtx.shardings, err = transformer.resolveCompsNShardingsFromSpecified(transCtx, cluster)
		Expect(err).Should(BeNil())

		err = transformer.validateNBuildAllCompSpecs(transCtx, cluster)
		Expect(err).Should(BeNil())
	}

	BeforeEach(func() {
		cluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, "test-cluster", "").
			AddComponent("comp1", compDefName).
			AddComponent("comp2", compDefName).
			AddSharding("sharding1", "", compDefName).
			AddSharding("sharding2", "", compDefName).
			GetObject()

		transCtx = &clusterTransformContext{
			Context:       testCtx.Ctx,
			Client:        model.NewGraphClient(&mockReader{objs: []client.Object{}}),
			EventRecorder: clusterRecorder,
			Logger:        logger,
			Cluster:       cluster.DeepCopy(),
			OrigCluster:   cluster,
		}
		normalizeTransformContext(transCtx)

		dag = newDag(transCtx.Client.(model.GraphClient))
	})

	Context("component", func() {
		It("empty", func() {
			transCtx.components = nil

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Components).Should(BeNil())
		})

		It("comp not created", func() {
			transCtx.Cluster.Status.Components = nil

			// only have comp1 object in the cluster
			reader := &mockReader{
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-comp1",
							Labels: map[string]string{
								constant.AppManagedByLabelKey: constant.AppName,
								constant.AppInstanceLabelKey:  transCtx.Cluster.Name,
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Components).Should(HaveLen(2))
			Expect(transCtx.Cluster.Status.Components).Should(HaveKey("comp1"))
			Expect(transCtx.Cluster.Status.Components["comp1"].Phase).Should(Equal(appsv1.RunningClusterCompPhase))
			Expect(transCtx.Cluster.Status.Components).Should(HaveKey("comp2"))
			Expect(transCtx.Cluster.Status.Components["comp2"].Phase).Should(Equal(appsv1.ClusterComponentPhase("")))
		})

		It("comp spec deleted", func() {
			// have seen the comp1 and comp2 objects in the cluster
			transCtx.Cluster.Status.Components = map[string]appsv1.ClusterComponentStatus{
				"comp1": {
					Phase: appsv1.RunningClusterCompPhase,
				},
				"comp2": {
					Phase: appsv1.RunningClusterCompPhase,
				},
			}

			reader := &mockReader{
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-comp1",
							Labels: map[string]string{
								constant.AppManagedByLabelKey: constant.AppName,
								constant.AppInstanceLabelKey:  transCtx.Cluster.Name,
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-comp2",
							Labels: map[string]string{
								constant.AppManagedByLabelKey: constant.AppName,
								constant.AppInstanceLabelKey:  transCtx.Cluster.Name,
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			// delete comp2 from cluster spec
			transCtx.components = transCtx.components[:1]

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Components).Should(HaveLen(2))
			Expect(transCtx.Cluster.Status.Components).Should(HaveKey("comp1"))
			Expect(transCtx.Cluster.Status.Components["comp1"].Phase).Should(Equal(appsv1.RunningClusterCompPhase))
			Expect(transCtx.Cluster.Status.Components).Should(HaveKey("comp2"))
			Expect(transCtx.Cluster.Status.Components["comp2"].Phase).Should(Equal(appsv1.DeletingClusterCompPhase))
		})

		It("comp object deleted", func() {
			// have seen the comp1 and comp2 objects in the cluster
			transCtx.Cluster.Status.Components = map[string]appsv1.ClusterComponentStatus{
				"comp1": {
					Phase: appsv1.RunningClusterCompPhase,
				},
				"comp2": {
					Phase: appsv1.RunningClusterCompPhase,
				},
			}

			// comp2 object is deleted???
			reader := &mockReader{
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-comp1",
							Labels: map[string]string{
								constant.AppManagedByLabelKey: constant.AppName,
								constant.AppInstanceLabelKey:  transCtx.Cluster.Name,
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Components).Should(HaveLen(2))
			Expect(transCtx.Cluster.Status.Components).Should(HaveKey("comp1"))
			Expect(transCtx.Cluster.Status.Components["comp1"].Phase).Should(Equal(appsv1.RunningClusterCompPhase))
			Expect(transCtx.Cluster.Status.Components).Should(HaveKey("comp2"))
			Expect(transCtx.Cluster.Status.Components["comp2"].Phase).Should(Equal(appsv1.ClusterComponentPhase("")))
		})

		It("comp deleted", func() {
			// have seen the comp1 and comp2 objects in the cluster
			transCtx.Cluster.Status.Components = map[string]appsv1.ClusterComponentStatus{
				"comp1": {
					Phase: appsv1.RunningClusterCompPhase,
				},
				"comp2": {
					Phase: appsv1.RunningClusterCompPhase,
				},
			}

			// delete comp2 object
			reader := &mockReader{
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-comp1",
							Labels: map[string]string{
								constant.AppManagedByLabelKey: constant.AppName,
								constant.AppInstanceLabelKey:  transCtx.Cluster.Name,
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			// delete comp2 spec
			transCtx.components = transCtx.components[:1]

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Components).Should(HaveLen(1))
			Expect(transCtx.Cluster.Status.Components).Should(HaveKey("comp1"))
			Expect(transCtx.Cluster.Status.Components["comp1"].Phase).Should(Equal(appsv1.RunningClusterCompPhase))
			Expect(transCtx.Cluster.Status.Components).ShouldNot(HaveKey("comp2"))
		})

		It("ok", func() {
			reader := &mockReader{
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-comp1",
							Labels: map[string]string{
								constant.AppManagedByLabelKey: constant.AppName,
								constant.AppInstanceLabelKey:  transCtx.Cluster.Name,
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-comp2",
							Labels: map[string]string{
								constant.AppManagedByLabelKey: constant.AppName,
								constant.AppInstanceLabelKey:  transCtx.Cluster.Name,
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.CreatingClusterCompPhase,
						},
					},
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Components).Should(HaveLen(2))
			Expect(transCtx.Cluster.Status.Components).Should(HaveKey("comp1"))
			Expect(transCtx.Cluster.Status.Components["comp1"].Phase).Should(Equal(appsv1.RunningClusterCompPhase))
			Expect(transCtx.Cluster.Status.Components).Should(HaveKey("comp2"))
			Expect(transCtx.Cluster.Status.Components["comp2"].Phase).Should(Equal(appsv1.CreatingClusterCompPhase))
		})

		It("phase changed", func() {
			transCtx.Cluster.Status.Components = map[string]appsv1.ClusterComponentStatus{
				"comp1": {
					Phase: appsv1.RunningClusterCompPhase,
				},
				"comp2": {
					Phase: appsv1.RunningClusterCompPhase,
				},
			}

			reader := &mockReader{
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-comp1",
							Labels: map[string]string{
								constant.AppManagedByLabelKey: constant.AppName,
								constant.AppInstanceLabelKey:  transCtx.Cluster.Name,
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.UpdatingClusterCompPhase,
						},
					},
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-comp2",
							Labels: map[string]string{
								constant.AppManagedByLabelKey: constant.AppName,
								constant.AppInstanceLabelKey:  transCtx.Cluster.Name,
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.DeletingClusterCompPhase,
						},
					},
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Components).Should(HaveLen(2))
			Expect(transCtx.Cluster.Status.Components).Should(HaveKey("comp1"))
			Expect(transCtx.Cluster.Status.Components["comp1"].Phase).Should(Equal(appsv1.UpdatingClusterCompPhase))
			Expect(transCtx.Cluster.Status.Components).Should(HaveKey("comp2"))
			Expect(transCtx.Cluster.Status.Components["comp2"].Phase).Should(Equal(appsv1.DeletingClusterCompPhase))
		})
	})

	Context("sharding", func() {
		It("empty", func() {
			transCtx.shardings = nil

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Shardings).Should(BeNil())
		})

		It("sharding not created", func() {
			transCtx.Cluster.Status.Shardings = nil

			// only have sharding1 object in the cluster
			reader := &mockReader{
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-sharding1",
							Labels: map[string]string{
								constant.AppManagedByLabelKey:      constant.AppName,
								constant.AppInstanceLabelKey:       transCtx.Cluster.Name,
								constant.KBAppShardingNameLabelKey: "sharding1",
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveLen(2))
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveKey("sharding1"))
			Expect(transCtx.Cluster.Status.Shardings["sharding1"].Phase).Should(Equal(appsv1.RunningClusterCompPhase))
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveKey("sharding2"))
			Expect(transCtx.Cluster.Status.Shardings["sharding2"].Phase).Should(Equal(appsv1.ClusterComponentPhase("")))
		})

		It("sharding spec deleted", func() {
			// have seen the sharding1 and sharding2 objects in the cluster
			transCtx.Cluster.Status.Shardings = map[string]appsv1.ClusterComponentStatus{
				"sharding1": {
					Phase: appsv1.RunningClusterCompPhase,
				},
				"sharding2": {
					Phase: appsv1.RunningClusterCompPhase,
				},
			}

			reader := &mockReader{
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-sharding1",
							Labels: map[string]string{
								constant.AppManagedByLabelKey:      constant.AppName,
								constant.AppInstanceLabelKey:       transCtx.Cluster.Name,
								constant.KBAppShardingNameLabelKey: "sharding1",
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-sharding2",
							Labels: map[string]string{
								constant.AppManagedByLabelKey:      constant.AppName,
								constant.AppInstanceLabelKey:       transCtx.Cluster.Name,
								constant.KBAppShardingNameLabelKey: "sharding2",
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			// delete sharding2 from cluster spec
			transCtx.shardings = transCtx.shardings[:1]

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveLen(2))
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveKey("sharding1"))
			Expect(transCtx.Cluster.Status.Shardings["sharding1"].Phase).Should(Equal(appsv1.RunningClusterCompPhase))
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveKey("sharding2"))
			Expect(transCtx.Cluster.Status.Shardings["sharding2"].Phase).Should(Equal(appsv1.DeletingClusterCompPhase))
		})

		It("sharding object deleted", func() {
			// have seen the sharding1 and sharding2 objects in the cluster
			transCtx.Cluster.Status.Shardings = map[string]appsv1.ClusterComponentStatus{
				"sharding1": {
					Phase: appsv1.RunningClusterCompPhase,
				},
				"sharding2": {
					Phase: appsv1.RunningClusterCompPhase,
				},
			}

			// sharding2 object is deleted???
			reader := &mockReader{
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-sharding1",
							Labels: map[string]string{
								constant.AppManagedByLabelKey:      constant.AppName,
								constant.AppInstanceLabelKey:       transCtx.Cluster.Name,
								constant.KBAppShardingNameLabelKey: "sharding1",
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveLen(2))
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveKey("sharding1"))
			Expect(transCtx.Cluster.Status.Shardings["sharding1"].Phase).Should(Equal(appsv1.RunningClusterCompPhase))
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveKey("sharding2"))
			Expect(transCtx.Cluster.Status.Shardings["sharding2"].Phase).Should(Equal(appsv1.ClusterComponentPhase("")))
		})

		It("sharding deleted", func() {
			// have seen the sharding1 and sharding2 objects in the cluster
			transCtx.Cluster.Status.Shardings = map[string]appsv1.ClusterComponentStatus{
				"sharding1": {
					Phase: appsv1.RunningClusterCompPhase,
				},
				"sharding2": {
					Phase: appsv1.RunningClusterCompPhase,
				},
			}

			// delete sharding2 object
			reader := &mockReader{
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-sharding1",
							Labels: map[string]string{
								constant.AppManagedByLabelKey:      constant.AppName,
								constant.AppInstanceLabelKey:       transCtx.Cluster.Name,
								constant.KBAppShardingNameLabelKey: "sharding1",
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			// delete sharding2 spec
			transCtx.shardings = transCtx.shardings[:1]

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveLen(1))
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveKey("sharding1"))
			Expect(transCtx.Cluster.Status.Shardings["sharding1"].Phase).Should(Equal(appsv1.RunningClusterCompPhase))
			Expect(transCtx.Cluster.Status.Components).ShouldNot(HaveKey("sharding2"))
		})

		It("ok", func() {
			reader := &mockReader{
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-sharding1",
							Labels: map[string]string{
								constant.AppManagedByLabelKey:      constant.AppName,
								constant.AppInstanceLabelKey:       transCtx.Cluster.Name,
								constant.KBAppShardingNameLabelKey: "sharding1",
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-sharding2",
							Labels: map[string]string{
								constant.AppManagedByLabelKey:      constant.AppName,
								constant.AppInstanceLabelKey:       transCtx.Cluster.Name,
								constant.KBAppShardingNameLabelKey: "sharding2",
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.CreatingClusterCompPhase,
						},
					},
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveLen(2))
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveKey("sharding1"))
			Expect(transCtx.Cluster.Status.Shardings["sharding1"].Phase).Should(Equal(appsv1.RunningClusterCompPhase))
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveKey("sharding2"))
			Expect(transCtx.Cluster.Status.Shardings["sharding2"].Phase).Should(Equal(appsv1.CreatingClusterCompPhase))
		})

		It("compose phases", func() {
			reader := &mockReader{
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-sharding1-01",
							Labels: map[string]string{
								constant.AppManagedByLabelKey:      constant.AppName,
								constant.AppInstanceLabelKey:       transCtx.Cluster.Name,
								constant.KBAppShardingNameLabelKey: "sharding1",
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-sharding1-02",
							Labels: map[string]string{
								constant.AppManagedByLabelKey:      constant.AppName,
								constant.AppInstanceLabelKey:       transCtx.Cluster.Name,
								constant.KBAppShardingNameLabelKey: "sharding1",
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.CreatingClusterCompPhase,
						},
					},
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveLen(2))
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveKey("sharding1"))
			Expect(transCtx.Cluster.Status.Shardings["sharding1"].Phase).Should(Equal(appsv1.UpdatingClusterCompPhase))
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveKey("sharding2"))
			Expect(transCtx.Cluster.Status.Shardings["sharding2"].Phase).Should(Equal(appsv1.ClusterComponentPhase("")))
		})

		It("phase changed", func() {
			transCtx.Cluster.Status.Shardings = map[string]appsv1.ClusterComponentStatus{
				"sharding1": {
					Phase: appsv1.CreatingClusterCompPhase,
				},
			}

			reader := &mockReader{
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-sharding1-01",
							Labels: map[string]string{
								constant.AppManagedByLabelKey:      constant.AppName,
								constant.AppInstanceLabelKey:       transCtx.Cluster.Name,
								constant.KBAppShardingNameLabelKey: "sharding1",
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "test-cluster-sharding1-02",
							Labels: map[string]string{
								constant.AppManagedByLabelKey:      constant.AppName,
								constant.AppInstanceLabelKey:       transCtx.Cluster.Name,
								constant.KBAppShardingNameLabelKey: "sharding1",
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningClusterCompPhase,
						},
					},
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			transformer := &clusterComponentStatusTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveLen(2))
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveKey("sharding1"))
			Expect(transCtx.Cluster.Status.Shardings["sharding1"].Phase).Should(Equal(appsv1.RunningClusterCompPhase))
			Expect(transCtx.Cluster.Status.Shardings).Should(HaveKey("sharding2"))
			Expect(transCtx.Cluster.Status.Shardings["sharding2"].Phase).Should(Equal(appsv1.ClusterComponentPhase("")))
		})
	})
})
