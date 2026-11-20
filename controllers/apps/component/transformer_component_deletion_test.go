/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestComponentDeletionCollectsInstanceConfigMirrorsAfterWorkload(t *testing.T) {
	comp, synth, scheme := instanceConfigFixture(t)
	comp.DeletionTimestamp = &metav1.Time{Time: metav1.Now().Time}
	comp.Finalizers = []string{constant.DBComponentFinalizerName}
	comp.Status.Phase = appsv1.DeletingComponentPhase
	comp.Spec.TerminationPolicy = appsv1.Delete
	its, err := component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
	if err != nil {
		t.Fatal(err)
	}
	if err := controllerutil.SetControllerReference(comp, its, scheme); err != nil {
		t.Fatal(err)
	}
	mirror := managedInstanceConfig(comp, "special")
	authoritative := mirror.DeepCopy()
	authoritative.Annotations = map[string]string{instanceConfigMapPlacementsAnnotationKey: "removed-provider"}
	mirror.OwnerReferences = nil
	foreign := mirror.DeepCopy()
	foreign.Name = "foreign"
	foreign.Labels[constant.KBAppConfigMapOwnerLabelKey] = "previous-component"
	control := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appsv1.Component{}).WithObjects(comp, its, authoritative).Build()
	provider := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mirror, foreign).Build()
	cli := multicluster.NewClient(control, map[string]client.Client{"removed-provider": provider})
	for pass := 0; pass < 2; pass++ {
		if err := control.Get(context.Background(), client.ObjectKeyFromObject(comp), comp); err != nil {
			t.Fatal(err)
		}
		graphCli := model.NewGraphClient(cli)
		transCtx := &componentTransformContext{Context: context.Background(), Client: graphCli, Component: comp, ComponentOrig: comp.DeepCopy()}
		dag := graph.NewDAG()
		graphCli.Root(dag, comp, comp, model.ActionStatusPtr())
		builder := &componentPlanBuilder{cli: cli, transCtx: transCtx}
		err := (&componentDeletionTransformer{Client: cli, configProviders: []string{"removed-provider"}}).Transform(transCtx, dag)
		if pass == 0 && err != graph.ErrPrematureStop {
			t.Fatalf("workload deletion did not stop later cleanup: %v", err)
		}
		if pass == 1 && (!intctrlutil.IsRequeueError(err) || intctrlutil.IsDelayedRequeueError(err)) {
			t.Fatalf("mirror deletion did not retain Component finalizer: %v", err)
		}
		plan := &componentPlan{dag: dag, transCtx: transCtx, walkFunc: builder.defaultWalkFunc}
		if err := plan.Execute(); err != nil {
			t.Fatal(err)
		}
		err = provider.Get(context.Background(), client.ObjectKeyFromObject(mirror), &corev1.ConfigMap{})
		if pass == 0 && err != nil {
			t.Fatalf("mirror removed while workload existed: %v", err)
		}
		if pass == 1 && !apierrors.IsNotFound(err) {
			t.Fatalf("mirror remained after workload removal: %v", err)
		}
	}
	if err := provider.Get(context.Background(), client.ObjectKeyFromObject(foreign), &corev1.ConfigMap{}); err != nil {
		t.Fatalf("previous Component mirror was collected: %v", err)
	}
	if err := control.Get(context.Background(), client.ObjectKeyFromObject(comp), &appsv1.Component{}); err != nil {
		t.Fatalf("Component finalizer removed before cleanup confirmation: %v", err)
	}
	if err := control.Get(context.Background(), client.ObjectKeyFromObject(its), &workloads.InstanceSet{}); !apierrors.IsNotFound(err) {
		t.Fatalf("workload was not deleted: %v", err)
	}
}

func TestComponentDeletionSkipsUnrelatedProviders(t *testing.T) {
	comp, _, scheme := instanceConfigFixture(t)
	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	cli := interceptor.NewClient(base, interceptor.Funcs{List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
		if placement, _ := multicluster.FromContext(ctx); placement != "" {
			t.Fatal("single-cluster Component queried unrelated data providers")
		}
		return c.List(ctx, list, opts...)
	}})
	transCtx := &componentTransformContext{Context: context.Background(), Client: cli, Component: comp}
	if err := collectDeletingInstanceConfigMaps(transCtx, cli, constant.GetCompLabels("db", "mysql"), nil); err != nil {
		t.Fatalf("unrelated provider blocked Component cleanup: %v", err)
	}
}
