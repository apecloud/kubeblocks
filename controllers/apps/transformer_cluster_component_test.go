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
	"context"
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

type mockReader struct {
	objs []client.Object
}

func (r *mockReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	for _, o := range r.objs {
		// ignore the GVK check
		if client.ObjectKeyFromObject(o) == key {
			reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(o).Elem())
			return nil
		}
	}
	return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
}

func (r *mockReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	items := reflect.ValueOf(list).Elem().FieldByName("Items")
	if !items.IsValid() {
		return fmt.Errorf("ObjectList has no Items field: %s", list.GetObjectKind().GroupVersionKind().String())
	}
	if len(r.objs) > 0 {
		objs := reflect.MakeSlice(items.Type(), 0, 0)
		for i := range r.objs {
			if reflect.TypeOf(r.objs[i]).Elem().AssignableTo(items.Type().Elem()) {
				objs = reflect.Append(objs, reflect.ValueOf(r.objs[i]).Elem())
			}
		}
		items.Set(objs)
	}
	return nil
}

var _ = Describe("cluster component transformer test", func() {
	const (
		clusterDefName                     = "test-clusterdef"
		clusterTopologyDefault             = "test-topology-default"
		clusterTopologyNoOrders            = "test-topology-no-orders"
		clusterTopologyProvisionNUpdateOOD = "test-topology-ood"
		compDefName                        = "test-compdef"
		clusterName                        = "test-cluster"
		comp1aName                         = "comp-1a"
		comp1bName                         = "comp-1b"
		comp2aName                         = "comp-2a"
		comp2bName                         = "comp-2b"
	)

	var (
		clusterDef *appsv1alpha1.ClusterDefinition
	)

	BeforeEach(func() {
		clusterDef = testapps.NewClusterDefFactory(clusterDefName).
			AddClusterTopology(appsv1alpha1.ClusterTopology{
				Name: clusterTopologyDefault,
				Components: []appsv1alpha1.ClusterTopologyComponent{
					{
						Name:    comp1aName,
						CompDef: compDefName,
					},
					{
						Name:    comp1bName,
						CompDef: compDefName,
					},
					{
						Name:    comp2aName,
						CompDef: compDefName,
					},
					{
						Name:    comp2bName,
						CompDef: compDefName,
					},
				},
				Orders: &appsv1alpha1.ClusterTopologyOrders{
					Provision: []string{
						fmt.Sprintf("%s,%s", comp1aName, comp1bName),
						fmt.Sprintf("%s,%s", comp2aName, comp2bName),
					},
					Terminate: []string{
						fmt.Sprintf("%s,%s", comp2aName, comp2bName),
						fmt.Sprintf("%s,%s", comp1aName, comp1bName),
					},
					Update: []string{
						fmt.Sprintf("%s,%s", comp1aName, comp1bName),
						fmt.Sprintf("%s,%s", comp2aName, comp2bName),
					},
				},
			}).
			AddClusterTopology(appsv1alpha1.ClusterTopology{
				Name: clusterTopologyNoOrders,
				Components: []appsv1alpha1.ClusterTopologyComponent{
					{
						Name:    comp1aName,
						CompDef: compDefName,
					},
					{
						Name:    comp1bName,
						CompDef: compDefName,
					},
					{
						Name:    comp2aName,
						CompDef: compDefName,
					},
					{
						Name:    comp2bName,
						CompDef: compDefName,
					},
				},
			}).
			AddClusterTopology(appsv1alpha1.ClusterTopology{
				Name: clusterTopologyProvisionNUpdateOOD,
				Components: []appsv1alpha1.ClusterTopologyComponent{
					{
						Name:    comp1aName,
						CompDef: compDefName,
					},
					{
						Name:    comp1bName,
						CompDef: compDefName,
					},
					{
						Name:    comp2aName,
						CompDef: compDefName,
					},
					{
						Name:    comp2bName,
						CompDef: compDefName,
					},
				},
				Orders: &appsv1alpha1.ClusterTopologyOrders{
					Provision: []string{
						fmt.Sprintf("%s,%s", comp1aName, comp1bName),
						fmt.Sprintf("%s,%s", comp2aName, comp2bName),
					},
					Update: []string{
						fmt.Sprintf("%s,%s", comp2aName, comp2bName),
						fmt.Sprintf("%s,%s", comp1aName, comp1bName),
					},
				},
			}).
			GetObject()
	})

	AfterEach(func() {})

	newDAG := func(graphCli model.GraphClient, cluster *appsv1alpha1.Cluster) *graph.DAG {
		d := graph.NewDAG()
		graphCli.Root(d, cluster, cluster, model.ActionStatusPtr())
		return d
	}

	buildCompSpecs := func(clusterDef *appsv1alpha1.ClusterDefinition, cluster *appsv1alpha1.Cluster) []*appsv1alpha1.ClusterComponentSpec {
		apiTransformer := ClusterAPINormalizationTransformer{}
		compSpecs, err := apiTransformer.buildCompSpecs4Topology(clusterDef, cluster)
		Expect(err).Should(BeNil())
		return compSpecs
	}

	mockCompObj := func(transCtx *clusterTransformContext, compName string, setters ...func(*appsv1alpha1.Component)) *appsv1alpha1.Component {
		var compSpec *appsv1alpha1.ClusterComponentSpec
		for i, spec := range transCtx.ComponentSpecs {
			if spec.Name == compName {
				compSpec = transCtx.ComponentSpecs[i]
				break
			}
		}
		Expect(compSpec).ShouldNot(BeNil())

		comp, err := component.BuildComponent(transCtx.Cluster, compSpec, nil, nil)
		Expect(err).Should(BeNil())

		for _, setter := range setters {
			if setter != nil {
				setter(comp)
			}
		}

		return comp
	}

	newTransformerNCtx := func(topology string) (graph.Transformer, *clusterTransformContext, *graph.DAG) {
		cluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName).
			WithRandomName().
			SetTopology(topology).
			SetReplicas(1).
			GetObject()
		graphCli := model.NewGraphClient(k8sClient)
		transCtx := &clusterTransformContext{
			Context:        ctx,
			Client:         graphCli,
			EventRecorder:  nil,
			Logger:         logger,
			Cluster:        cluster,
			OrigCluster:    cluster.DeepCopy(),
			ClusterDef:     clusterDef,
			ComponentSpecs: buildCompSpecs(clusterDef, cluster),
		}
		return &clusterComponentTransformer{}, transCtx, newDAG(graphCli, cluster)
	}

	Context("component orders", func() {
		It("w/o orders", func() {
			transformer, transCtx, dag := newTransformerNCtx(clusterTopologyNoOrders)
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check the components
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &appsv1alpha1.Component{})
			Expect(len(objs)).Should(Equal(4))
			for _, obj := range objs {
				comp := obj.(*appsv1alpha1.Component)
				Expect(graphCli.IsAction(dag, comp, model.ActionCreatePtr())).Should(BeTrue())
			}
		})

		It("w/ orders provision - has no predecessors", func() {
			transformer, transCtx, dag := newTransformerNCtx(clusterTopologyDefault)
			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp2aName), ContainSubstring(comp2bName)))

			// check the first two components
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &appsv1alpha1.Component{})
			Expect(len(objs)).Should(Equal(2))
			for _, obj := range objs {
				comp := obj.(*appsv1alpha1.Component)
				Expect(component.ShortName(transCtx.Cluster.Name, comp.Name)).Should(Or(Equal(comp1aName), Equal(comp1bName)))
				Expect(graphCli.IsAction(dag, comp, model.ActionCreatePtr())).Should(BeTrue())
			}
		})

		It("w/ orders provision - has a predecessor not ready", func() {
			transformer, transCtx, dag := newTransformerNCtx(clusterTopologyDefault)

			// mock first two components status as running and creating
			reader := &mockReader{
				objs: []client.Object{
					mockCompObj(transCtx, comp1aName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp1bName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.CreatingClusterCompPhase
					}),
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp2aName), ContainSubstring(comp2bName)))

			// should have no components to update
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &appsv1alpha1.Component{})
			Expect(len(objs)).Should(Equal(0))
		})

		It("w/ orders provision - has a predecessor in DAG", func() {
			transformer, transCtx, dag := newTransformerNCtx(clusterTopologyDefault)

			// mock one of first two components status as running
			reader := &mockReader{
				objs: []client.Object{
					mockCompObj(transCtx, comp1aName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp2aName), ContainSubstring(comp2bName)))

			// should have one component to create
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &appsv1alpha1.Component{})
			Expect(len(objs)).Should(Equal(1))
			comp := objs[0].(*appsv1alpha1.Component)
			Expect(component.ShortName(transCtx.Cluster.Name, comp.Name)).Should(Equal(comp1bName))
			Expect(graphCli.IsAction(dag, comp, model.ActionCreatePtr())).Should(BeTrue())
		})

		It("w/ orders provision - all predecessors ready", func() {
			transformer, transCtx, dag := newTransformerNCtx(clusterTopologyDefault)

			// mock first two components status as running
			reader := &mockReader{
				objs: []client.Object{
					mockCompObj(transCtx, comp1aName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp1bName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check the last two components
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &appsv1alpha1.Component{})
			Expect(len(objs)).Should(Equal(2))
			for _, obj := range objs {
				comp := obj.(*appsv1alpha1.Component)
				Expect(component.ShortName(transCtx.Cluster.Name, comp.Name)).Should(Or(Equal(comp2aName), Equal(comp2bName)))
				Expect(graphCli.IsAction(dag, comp, model.ActionCreatePtr())).Should(BeTrue())
			}
		})

		It("w/ orders update - has no predecessors", func() {
			transformer, transCtx, dag := newTransformerNCtx(clusterTopologyDefault)

			// mock first two components
			reader := &mockReader{
				objs: []client.Object{
					mockCompObj(transCtx, comp1aName, func(comp *appsv1alpha1.Component) {
						comp.Spec.Replicas = 2 // to update
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp1bName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp2aName), ContainSubstring(comp2bName)))

			// check the first component
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &appsv1alpha1.Component{})
			Expect(len(objs)).Should(Equal(1))
			comp := objs[0].(*appsv1alpha1.Component)
			Expect(component.ShortName(transCtx.Cluster.Name, comp.Name)).Should(Equal(comp1aName))
			Expect(graphCli.IsAction(dag, comp, model.ActionUpdatePtr())).Should(BeTrue())
		})

		It("w/ orders update - has a predecessor not ready", func() {
			transformer, transCtx, dag := newTransformerNCtx(clusterTopologyDefault)

			// mock components
			reader := &mockReader{
				objs: []client.Object{
					mockCompObj(transCtx, comp1aName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.CreatingClusterCompPhase // not ready
					}),
					mockCompObj(transCtx, comp1bName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp2aName, func(comp *appsv1alpha1.Component) {
						comp.Spec.Replicas = 2 // to update
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp2bName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
				},
			}
			transCtx.Client = model.NewGraphClient(reader)
			transCtx.OrigCluster.Generation += 1 // mock cluster spec update

			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			// TODO: should not contain comp2bName
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp2aName), ContainSubstring(comp2bName)))

			// should have no components to update
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &appsv1alpha1.Component{})
			Expect(len(objs)).Should(Equal(0))
		})

		It("w/ orders update - has a predecessor in DAG", func() {
			transformer, transCtx, dag := newTransformerNCtx(clusterTopologyDefault)

			// mock components
			reader := &mockReader{
				objs: []client.Object{
					mockCompObj(transCtx, comp1aName, func(comp *appsv1alpha1.Component) {
						comp.Spec.Replicas = 2 // to update
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp1bName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp2aName, func(comp *appsv1alpha1.Component) {
						comp.Spec.Replicas = 2 // to update
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp2bName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
				},
			}
			transCtx.Client = model.NewGraphClient(reader)
			transCtx.OrigCluster.Generation += 1 // mock cluster spec update

			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			// TODO: should not contain comp2bName
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp2aName), ContainSubstring(comp2bName)))

			// should have one component to update
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &appsv1alpha1.Component{})
			Expect(len(objs)).Should(Equal(1))
			comp := objs[0].(*appsv1alpha1.Component)
			Expect(component.ShortName(transCtx.Cluster.Name, comp.Name)).Should(Equal(comp1aName))
			Expect(graphCli.IsAction(dag, comp, model.ActionUpdatePtr())).Should(BeTrue())
		})

		It("w/ orders update - all predecessors ready", func() {
			transformer, transCtx, dag := newTransformerNCtx(clusterTopologyDefault)

			// mock components
			reader := &mockReader{
				objs: []client.Object{
					mockCompObj(transCtx, comp1aName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp1bName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp2aName, func(comp *appsv1alpha1.Component) {
						comp.Spec.Replicas = 2 // to update
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp2bName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
				},
			}
			transCtx.Client = model.NewGraphClient(reader)
			transCtx.OrigCluster.Generation += 1 // mock cluster spec update

			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &appsv1alpha1.Component{})
			Expect(len(objs)).Should(Equal(1))
			comp := objs[0].(*appsv1alpha1.Component)
			Expect(component.ShortName(transCtx.Cluster.Name, comp.Name)).Should(Equal(comp2aName))
			Expect(graphCli.IsAction(dag, comp, model.ActionUpdatePtr())).Should(BeTrue())
		})

		It("w/ orders provision & update - OOD", func() {
			transformer, transCtx, dag := newTransformerNCtx(clusterTopologyProvisionNUpdateOOD)

			// mock first two components status as running
			reader := &mockReader{
				objs: []client.Object{
					mockCompObj(transCtx, comp1aName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp1bName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
				},
			}
			transCtx.Client = model.NewGraphClient(reader)

			// comp2aName and comp2bName are not ready (exist) when updating comp1aName and comp1bName
			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(ictrlutil.IsDelayedRequeueError(err)).Should(BeTrue())

			// check the last two components under provisioning
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &appsv1alpha1.Component{})
			Expect(len(objs)).Should(Equal(2))
			for _, obj := range objs {
				comp := obj.(*appsv1alpha1.Component)
				Expect(component.ShortName(transCtx.Cluster.Name, comp.Name)).Should(Or(Equal(comp2aName), Equal(comp2bName)))
				Expect(graphCli.IsAction(dag, comp, model.ActionCreatePtr())).Should(BeTrue())
			}

			// mock last two components status as running
			reader.objs = append(reader.objs, []client.Object{
				mockCompObj(transCtx, comp2aName, func(comp *appsv1alpha1.Component) {
					comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
				}),
				mockCompObj(transCtx, comp2bName, func(comp *appsv1alpha1.Component) {
					comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
				}),
			}...)

			// try again
			err = transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())
		})
	})
})
