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

package apps

import (
	"context"
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
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
		clusterTopology4Stop               = "test-topology-stop"
		clusterTopologyTemplate            = "test-topology-template"
		compDefName                        = "test-compdef"
		clusterName                        = "test-cluster"
		comp1aName                         = "comp-1a"
		comp1bName                         = "comp-1b"
		comp2aName                         = "comp-2a"
		comp2bName                         = "comp-2b"
		comp3aName                         = "comp-3a"
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
			AddClusterTopology(appsv1alpha1.ClusterTopology{
				Name: clusterTopology4Stop,
				Components: []appsv1alpha1.ClusterTopologyComponent{
					{
						Name:    comp1aName,
						CompDef: compDefName,
					},
					{
						Name:    comp2aName,
						CompDef: compDefName,
					},
					{
						Name:    comp3aName,
						CompDef: compDefName,
					},
				},
				Orders: &appsv1alpha1.ClusterTopologyOrders{
					Update: []string{comp1aName, comp2aName, comp3aName},
				},
			}).
			AddClusterTopology(appsv1alpha1.ClusterTopology{
				Name: clusterTopologyTemplate,
				Components: []appsv1alpha1.ClusterTopologyComponent{
					{
						Name:    comp1aName,
						CompDef: compDefName,
					},
					{
						Name:     comp2aName,
						CompDef:  compDefName,
						Template: pointer.Bool(true),
					},
					{
						Name:    comp2bName,
						CompDef: compDefName,
					},
					{
						Name:    comp3aName,
						CompDef: compDefName,
					},
				},
				Orders: &appsv1alpha1.ClusterTopologyOrders{
					Provision: []string{comp1aName, fmt.Sprintf("%s,%s", comp2aName, comp2bName), comp3aName},
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

	newTransformerNCtx := func(topology string, processors ...func(*testapps.MockClusterFactory)) (graph.Transformer, *clusterTransformContext, *graph.DAG) {
		f := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, "").
			WithRandomName().
			SetTopology(topology)
		if len(processors) > 0 {
			for _, processor := range processors {
				processor(f)
			}
		} else {
			f.SetReplicas(1)
		}
		cluster := f.GetObject()
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
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp2aName)))

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
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp2aName)))

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
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp2aName)))

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
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp2aName)))

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
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp2aName)))

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
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp2aName)))

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

		It("w/ orders update - stop", func() {
			transformer, transCtx, dag := newTransformerNCtx(clusterTopology4Stop)

			// mock to stop all components
			reader := &mockReader{
				objs: []client.Object{
					mockCompObj(transCtx, comp1aName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp2aName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp3aName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
				},
			}
			transCtx.Client = model.NewGraphClient(reader)
			for i := range transCtx.ComponentSpecs {
				transCtx.ComponentSpecs[i].Stop = &[]bool{true}[0]
			}
			transCtx.OrigCluster.Generation += 1 // mock cluster spec update

			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp2aName)))

			// should have the first component to update only
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &appsv1alpha1.Component{})
			Expect(len(objs)).Should(Equal(1))
			comp := objs[0].(*appsv1alpha1.Component)
			Expect(component.ShortName(transCtx.Cluster.Name, comp.Name)).Should(Equal(comp1aName))
			Expect(graphCli.IsAction(dag, comp, model.ActionUpdatePtr())).Should(BeTrue())
			Expect(comp.Spec.Stop).ShouldNot(BeNil())
			Expect(*comp.Spec.Stop).Should(BeTrue())
		})

		It("w/ orders update - stop the second component", func() {
			transformer, transCtx, dag := newTransformerNCtx(clusterTopology4Stop)

			// mock to stop all components and the first component has been stopped
			reader := &mockReader{
				objs: []client.Object{
					mockCompObj(transCtx, comp1aName, func(comp *appsv1alpha1.Component) {
						comp.Spec.Stop = &[]bool{true}[0]
						comp.Status.Phase = appsv1alpha1.StoppedClusterCompPhase
					}),
					mockCompObj(transCtx, comp2aName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
					mockCompObj(transCtx, comp3aName, func(comp *appsv1alpha1.Component) {
						comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
					}),
				},
			}
			transCtx.Client = model.NewGraphClient(reader)
			for i := range transCtx.ComponentSpecs {
				transCtx.ComponentSpecs[i].Stop = &[]bool{true}[0]
			}
			transCtx.OrigCluster.Generation += 1 // mock cluster spec update

			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp3aName)))

			// should have the second component to update only
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &appsv1alpha1.Component{})
			Expect(len(objs)).Should(Equal(1))
			comp := objs[0].(*appsv1alpha1.Component)
			Expect(component.ShortName(transCtx.Cluster.Name, comp.Name)).Should(Equal(comp2aName))
			Expect(graphCli.IsAction(dag, comp, model.ActionUpdatePtr())).Should(BeTrue())
			Expect(comp.Spec.Stop).ShouldNot(BeNil())
			Expect(*comp.Spec.Stop).Should(BeTrue())
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

		It("template component - has no components instantiated", func() {
			transformer, transCtx, dag := newTransformerNCtx(clusterTopologyTemplate)

			// check the components created, no components should be instantiated from the template automatically
			Expect(transCtx.ComponentSpecs).Should(HaveLen(3))
			Expect(transCtx.ComponentSpecs[0].Name).Should(Equal(comp1aName))
			Expect(transCtx.ComponentSpecs[1].Name).Should(Equal(comp2bName))
			Expect(transCtx.ComponentSpecs[2].Name).Should(Equal(comp3aName))

			// mock first component status as running
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
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp3aName)))

			// check other components
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &appsv1alpha1.Component{})
			Expect(len(objs)).Should(Equal(1))
			for _, obj := range objs {
				comp := obj.(*appsv1alpha1.Component)
				Expect(component.ShortName(transCtx.Cluster.Name, comp.Name)).Should(Equal(comp2bName))
				Expect(graphCli.IsAction(dag, comp, model.ActionCreatePtr())).Should(BeTrue())
			}
		})

		It("template component - has components instantiated", func() {
			transformer, transCtx, dag := newTransformerNCtx(clusterTopologyTemplate, func(f *testapps.MockClusterFactory) {
				f.AddComponent(fmt.Sprintf("%s-0", comp2aName), compDefName).
					AddComponent(fmt.Sprintf("%s-1", comp2aName), compDefName)
			})

			// check the components created
			Expect(transCtx.ComponentSpecs).Should(HaveLen(5))
			Expect(transCtx.ComponentSpecs[0].Name).Should(Equal(comp1aName))
			Expect(transCtx.ComponentSpecs[1].Name).Should(HavePrefix(comp2aName))
			Expect(transCtx.ComponentSpecs[2].Name).Should(HavePrefix(comp2aName))
			Expect(transCtx.ComponentSpecs[3].Name).Should(Equal(comp2bName))
			Expect(transCtx.ComponentSpecs[4].Name).Should(Equal(comp3aName))

			// mock first component status as running
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
			Expect(err.Error()).Should(And(ContainSubstring("retry later"), ContainSubstring(comp3aName)))

			// check other components
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &appsv1alpha1.Component{})
			Expect(len(objs)).Should(Equal(3))
			for _, obj := range objs {
				comp := obj.(*appsv1alpha1.Component)
				Expect(component.ShortName(transCtx.Cluster.Name, comp.Name)).Should(Or(HavePrefix(comp2aName), Equal(comp2bName)))
				Expect(graphCli.IsAction(dag, comp, model.ActionCreatePtr())).Should(BeTrue())
			}
		})
	})

	Context("testing component merge functionality", func() {
		var (
			oldCompObj *appsv1alpha1.Component
			newCompObj *appsv1alpha1.Component
		)

		BeforeEach(func() {
			// Initialize a base component
			oldCompObj = &appsv1alpha1.Component{
				Spec: appsv1alpha1.ComponentSpec{
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
				},
			}
			newCompObj = oldCompObj.DeepCopy()
		})

		It("should return nil when no changes are made", func() {
			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).To(BeNil())
		})

		It("should detect annotation changes", func() {
			newCompObj.Annotations = map[string]string{"key": "value"}
			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).NotTo(BeNil())
			Expect(result.Annotations).To(HaveKeyWithValue("key", "value"))
		})

		It("should detect label changes", func() {
			newCompObj.Labels = map[string]string{"app": "test"}
			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).NotTo(BeNil())
			Expect(result.Labels).To(HaveKeyWithValue("app", "test"))
		})

		It("should detect resource changes", func() {
			// Change CPU resource
			newCompObj.Spec.Resources.Limits[corev1.ResourceCPU] = resource.MustParse("2")
			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).NotTo(BeNil())
			Expect(result.Spec.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("2")))
		})

		It("should detect VolumeClaimTemplate changes", func() {
			// Add a volume claim template
			newCompObj.Spec.VolumeClaimTemplates = []appsv1alpha1.ClusterComponentVolumeClaimTemplate{
				{
					Name: "app-data",
					Spec: appsv1alpha1.PersistentVolumeClaimSpec{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			}
			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).NotTo(BeNil())
			Expect(result.Spec.VolumeClaimTemplates).To(HaveLen(1))
		})

		It("should normalize CPU resources", func() {
			// 1000m is equivalent to 1
			oldCompObj.Spec.Resources.Limits[corev1.ResourceCPU] = resource.MustParse("1")
			newCompObj.Spec.Resources.Limits[corev1.ResourceCPU] = resource.MustParse("1000m")
			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).To(BeNil()) // No change after normalization
		})

		It("should normalize memory resources", func() {
			// 1024Mi is equivalent to 1Gi
			oldCompObj.Spec.Resources.Limits[corev1.ResourceMemory] = resource.MustParse("1Gi")
			newCompObj.Spec.Resources.Limits[corev1.ResourceMemory] = resource.MustParse("1024Mi")
			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).To(BeNil()) // No change after normalization

			// 1536.5Mi is equivalent to 1611137024, and 1611137026 = 1611137024 + 2
			oldCompObj.Spec.Resources.Limits[corev1.ResourceMemory] = resource.MustParse("1611137024")
			newCompObj.Spec.Resources.Limits[corev1.ResourceMemory] = resource.MustParse("1536.5Mi")
			result = copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).To(BeNil())

			oldCompObj.Spec.Resources.Limits[corev1.ResourceMemory] = resource.MustParse("1611137026")
			newCompObj.Spec.Resources.Limits[corev1.ResourceMemory] = resource.MustParse("1536.5Mi")
			result = copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).NotTo(BeNil())

			oldCompObj.Spec.Resources.Limits[corev1.ResourceMemory] = resource.MustParse("1.5Gi")
			newCompObj.Spec.Resources.Limits[corev1.ResourceMemory] = resource.MustParse("1.512Gi")
			result = copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).NotTo(BeNil())
		})

		It("should handle nil resource limits", func() {
			oldCompObj.Spec.Resources.Limits = nil
			newCompObj.Spec.Resources.Limits = nil
			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).To(BeNil())
		})

		It("should handle nil resource requests", func() {
			oldCompObj.Spec.Resources.Requests = nil
			newCompObj.Spec.Resources.Requests = nil
			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).To(BeNil())
		})

		It("should detect changes when adding limits", func() {
			oldCompObj.Spec.Resources.Limits = nil
			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).NotTo(BeNil())
			Expect(result.Spec.Resources.Limits).NotTo(BeNil())
		})

		It("should detect changes in VolumeClaimTemplate storage requests", func() {
			vct := appsv1alpha1.ClusterComponentVolumeClaimTemplate{
				Name: "app-data",
				Spec: appsv1alpha1.PersistentVolumeClaimSpec{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
			}

			oldCompObj.Spec.VolumeClaimTemplates = []appsv1alpha1.ClusterComponentVolumeClaimTemplate{vct}
			newCompObj.Spec.VolumeClaimTemplates = []appsv1alpha1.ClusterComponentVolumeClaimTemplate{*vct.DeepCopy()}
			newCompObj.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] =
				resource.MustParse("2Gi")
			// Change storage request
			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).NotTo(BeNil())
			Expect(result.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]).
				To(Equal(resource.MustParse("2Gi")))
		})

		It("should normalize storage resources in VolumeClaimTemplates", func() {
			vct := appsv1alpha1.ClusterComponentVolumeClaimTemplate{
				Name: "app-data",
				Spec: appsv1alpha1.PersistentVolumeClaimSpec{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
			}
			oldCompObj.Spec.VolumeClaimTemplates = []appsv1alpha1.ClusterComponentVolumeClaimTemplate{vct}
			newCompObj.Spec.VolumeClaimTemplates = []appsv1alpha1.ClusterComponentVolumeClaimTemplate{*vct.DeepCopy()}

			// 1536Mi is equivalent to 1.5Gi
			oldCompObj.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] =
				resource.MustParse("1.5Gi")
			newCompObj.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] =
				resource.MustParse("1536Mi")
			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).To(BeNil()) // No change after normalization
		})

		It("should handle zero resource values", func() {
			oldCompObj.Spec.Resources.Limits[corev1.ResourceCPU] = resource.MustParse("0")
			newCompObj.Spec.Resources.Limits[corev1.ResourceCPU] = resource.MustParse("0m")

			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).To(BeNil()) // No change after normalization
		})

		It("should handle non-standard resource types", func() {
			customResource := "example.com/custom-resource"
			oldCompObj.Spec.Resources.Limits[corev1.ResourceName(customResource)] = resource.MustParse("5")
			newCompObj.Spec.Resources.Limits[corev1.ResourceName(customResource)] = resource.MustParse("10")
			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).NotTo(BeNil())
			Expect(result.Spec.Resources.Limits[corev1.ResourceName(customResource)]).
				To(Equal(resource.MustParse("10")))
		})

		It("should detect all changes when multiple fields change", func() {
			newCompObj.Labels = map[string]string{"app": "test"}
			newCompObj.Spec.Resources.Limits[corev1.ResourceCPU] = resource.MustParse("2")
			newCompObj.Spec.Replicas = 3

			result := copyAndMergeComponent(oldCompObj, newCompObj)
			Expect(result).NotTo(BeNil())
			Expect(result.Labels).To(HaveKeyWithValue("app", "test"))
			Expect(result.Spec.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("2")))
			Expect(result.Spec.Replicas).To(Equal(int32(3)))
		})
	})
})
