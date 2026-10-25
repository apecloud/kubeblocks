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
	"errors"
	"testing"
	"time"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeletingComponentReleasesOwnedRestoresWhileWorkloadExists(t *testing.T) {
	now := metav1.NewTime(time.Now())
	controller := true
	comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "cluster-oracle", Namespace: "ns", UID: "component-uid", DeletionTimestamp: &now}}
	labels := constant.GetCompLabels("cluster", "oracle")
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: comp.Name, Namespace: comp.Namespace, Labels: labels}}
	owned := &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{Name: "failed-postready", Namespace: comp.Namespace, Finalizers: []string{dptypes.DataProtectionFinalizerName}, OwnerReferences: []metav1.OwnerReference{{APIVersion: appsv1.GroupVersion.String(), Kind: "Component", Name: comp.Name, UID: comp.UID, Controller: &controller}}}, Status: dpv1alpha1.RestoreStatus{Phase: dpv1alpha1.RestorePhaseFailed}}
	other := owned.DeepCopy()
	other.Name = "same-label-other-owner"
	other.Labels = labels
	other.OwnerReferences[0].UID = "other-component"
	unowned := owned.DeepCopy()
	unowned.Name = "unowned"
	unowned.OwnerReferences = nil
	anotherNS := owned.DeepCopy()
	anotherNS.Namespace = "other"
	cli := fake.NewClientBuilder().WithScheme(model.GetScheme()).WithObjects(its, owned, other, unowned, anotherNS).Build()
	graphCli := model.NewGraphClient(cli)
	dag := graph.NewDAG()
	graphCli.Root(dag, comp, comp, nil)
	transCtx := &componentTransformContext{Context: context.Background(), Client: graphCli, Logger: logr.Discard(), Component: comp, ComponentOrig: comp.DeepCopy()}
	err := (&componentDeletionTransformer{}).deleteCompResources(transCtx, graphCli, dag, comp, labels, kindsForCompDelete())
	if !errors.Is(err, graph.ErrPrematureStop) {
		t.Fatalf("expected waiting on workload, got %v", err)
	}
	if !graphCli.IsAction(dag, its, model.ActionDeletePtr()) {
		t.Fatal("workload cleanup not requested")
	}
	if !graphCli.IsAction(dag, owned, model.ActionDeletePtr()) {
		t.Fatal("owned failed restore must be deleted without waiting for its PVC-consuming workload")
	}
	for _, obj := range []client.Object{other, unowned, anotherNS} {
		if graphCli.IsAction(dag, obj, model.ActionDeletePtr()) {
			t.Fatalf("unrelated restore selected for deletion: %s/%s", obj.GetNamespace(), obj.GetName())
		}
	}
	// Execute the scheduled delete through the real plan builder: DP must still
	// receive its finalizer callback rather than having the Restore disappear.
	vertex := graphCli.FindMatchedVertex(dag, owned).(*model.ObjectVertex)
	if err := (&componentPlanBuilder{cli: cli}).reconcileDeleteObject(context.Background(), vertex); err != nil {
		t.Fatal(err)
	}
	deleting := &dpv1alpha1.Restore{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(owned), deleting); err != nil {
		t.Fatal(err)
	}
	if deleting.DeletionTimestamp.IsZero() || len(deleting.Finalizers) != 1 || deleting.Finalizers[0] != dptypes.DataProtectionFinalizerName {
		t.Fatalf("ordinary delete must preserve DP finalizer: %#v", deleting.ObjectMeta)
	}
	// Live components retain their restore resources.
	comp.DeletionTimestamp = nil
	dag = graph.NewDAG()
	graphCli.Root(dag, comp, comp, nil)
	if err := (&componentDeletionTransformer{}).Transform(transCtx, dag); err != nil {
		t.Fatal(err)
	}
	if graphCli.IsAction(dag, owned, model.ActionDeletePtr()) {
		t.Fatal("live component restore must be retained")
	}
}
