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

package operations

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	kboperations "github.com/apecloud/kubeblocks/pkg/operations"
)

func TestUpgradeInstanceTemplateActionPersistence(t *testing.T) {
	for _, sharding := range []bool{false, true} {
		for _, failure := range []string{"Cluster conflict", "Running status patch"} {
			t.Run(fmt.Sprintf("sharding=%t/%s", sharding, failure), func(t *testing.T) {
				ctx := context.Background()
				reqCtx := ctrlutil.RequestCtx{Ctx: ctx}
				cluster, _ := ownerTestObjects()
				cluster.Spec.ComponentSpecs[0].ServiceVersion = "1.0.0"
				cluster.Spec.ComponentSpecs[0].Instances = []appsv1.InstanceTemplate{{Name: "canary"}}
				if sharding {
					cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "db", Shards: 1, Template: cluster.Spec.ComponentSpecs[0]}}
					cluster.Spec.ComponentSpecs = nil
				}
				spec := func(c *appsv1.Cluster) *appsv1.ClusterComponentSpec {
					if sharding {
						return &c.Spec.Shardings[0].Template
					}
					return &c.Spec.ComponentSpecs[0]
				}
				newRequest := func(name, version string) *opsv1alpha1.OpsRequest {
					return &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: cluster.Namespace}, Spec: opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.UpgradeType, Force: true, SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{Upgrade: &opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}, Instances: []opsv1alpha1.UpgradeInstanceTemplate{{Name: "canary", ServiceVersion: ptr.To(version)}}}}}}}, Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsPendingPhase}}
				}
				first, ops := newRequest("first", "2.0.0"), newRequest("clear", "")
				ops.Status.Extras = []map[string]string{{"unrelated": "keep"}}
				r := ownerTestReconciler(t)
				base := fake.NewClientBuilder().WithScheme(r.Scheme).WithStatusSubresource(cluster, first, ops).WithObjects(cluster, first, ops).Build()
				r.Client = base
				reload := func(request *opsv1alpha1.OpsRequest) *kboperations.OpsResource {
					t.Helper()
					c, o := &appsv1.Cluster{}, &opsv1alpha1.OpsRequest{}
					if err := base.Get(ctx, client.ObjectKeyFromObject(cluster), c); err != nil {
						t.Fatal(err)
					}
					if err := base.Get(ctx, client.ObjectKeyFromObject(request), o); err != nil {
						t.Fatal(err)
					}
					return &kboperations.OpsResource{Cluster: c, OpsRequest: o, Recorder: r.Recorder}
				}
				action := func(request *opsv1alpha1.OpsRequest) {
					t.Helper()
					if _, err := r.doOpsRequestAction(reqCtx, reload(request)); err != nil {
						t.Fatal(err)
					}
				}
				action(first)
				action(first)
				action(ops)
				original := reload(ops).OpsRequest.Status.LastConfiguration.DeepCopy()
				if got := original.Components["db"].InstanceTemplates[0]; got.CompDef != "" || got.ServiceVersion != "2.0.0" {
					t.Fatalf("snapshot=%#v", got)
				}
				if failure == "Running status patch" {
					current := reload(ops).Cluster
					spec(current).Instances[0].CompDef = "database"
					if err := base.Update(ctx, current); err != nil {
						t.Fatal(err)
					}
				}
				failed := false
				r.Client = interceptor.NewClient(base, interceptor.Funcs{
					Update: func(ctx context.Context, cli client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
						if _, ok := obj.(*appsv1.Cluster); ok && failure == "Cluster conflict" && !failed {
							failed = true
							current := &appsv1.Cluster{}
							if err := cli.Get(ctx, client.ObjectKeyFromObject(obj), current); err != nil {
								return err
							}
							spec(current).Instances[0].CompDef = "database"
							if err := cli.Update(ctx, current); err != nil {
								return err
							}
							return cli.Update(ctx, obj, opts...)
						}
						return cli.Update(ctx, obj, opts...)
					},
					SubResourcePatch: func(ctx context.Context, cli client.Client, name string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
						if request, ok := obj.(*opsv1alpha1.OpsRequest); ok && name == "status" && request.Name == ops.Name && request.Status.Phase == opsv1alpha1.OpsRunningPhase && failure == "Running status patch" && !failed {
							failed = true
							return fmt.Errorf("injected Running status patch failure")
						}
						return cli.SubResource(name).Patch(ctx, obj, patch, opts...)
					},
				})
				_, err := r.doOpsRequestAction(reqCtx, reload(ops))
				if err == nil || !failed {
					t.Fatalf("failure was not exercised: %v", err)
				}
				if failure == "Cluster conflict" && !apierrors.IsConflict(err) {
					t.Fatalf("expected actual resource-version conflict, got %v", err)
				}
				after := reload(ops)
				if after.OpsRequest.Status.Phase != opsv1alpha1.OpsCreatingPhase || !reflect.DeepEqual(after.OpsRequest.Status.Extras, ops.Status.Extras) {
					t.Fatalf("unpublished Action state persisted: %#v", after.OpsRequest.Status)
				}
				if !reflect.DeepEqual(&after.OpsRequest.Status.LastConfiguration, original) {
					t.Fatal("failure changed original snapshot")
				}
				wantVersion := "2.0.0"
				if failure == "Running status patch" {
					wantVersion = ""
				}
				if got := spec(after.Cluster).Instances[0]; got.CompDef != "database" || got.ServiceVersion != wantVersion {
					t.Fatalf("Cluster after failure=%#v", got)
				}
				// Owner backfill may run before a restarted controller retries Creating.
				spec(after.Cluster).Instances[0].ServiceVersion = "2.0.0"
				if err := base.Update(ctx, after.Cluster); err != nil {
					t.Fatal(err)
				}
				r = &OpsRequestReconciler{Client: base, Scheme: r.Scheme, Recorder: r.Recorder}
				action(ops)
				after = reload(ops)
				if after.OpsRequest.Status.Phase != opsv1alpha1.OpsRunningPhase {
					t.Fatalf("retry did not publish Running checkpoint: %#v", after.OpsRequest.Status)
				}
				if !reflect.DeepEqual(&after.OpsRequest.Status.LastConfiguration, original) {
					t.Fatal("retry changed original snapshot")
				}
				if got := spec(after.Cluster).Instances[0]; got.CompDef != "database" || got.ServiceVersion != "" {
					t.Fatalf("retry did not preserve current omitted field: %#v", got)
				}
				spec(after.Cluster).Instances[0].ServiceVersion = "2.0.0"
				if err := base.Update(ctx, after.Cluster); err != nil {
					t.Fatal(err)
				}
				if _, err := kboperations.GetOpsManager().Reconcile(reqCtx, base, reload(ops)); err != nil {
					t.Fatal(err)
				}
				if got := reload(ops).OpsRequest.Status; got.Phase != opsv1alpha1.OpsRunningPhase || !got.CompletionTimestamp.IsZero() {
					t.Fatalf("normal backfill terminated operation: %#v", got)
				}
				if len(after.OpsRequest.Status.Extras) != 2 || after.OpsRequest.Status.Extras[0]["unrelated"] != "keep" {
					t.Fatalf("retry checkpoint lost unrelated Extras: %#v", after.OpsRequest.Status.Extras)
				}
			})
		}
	}
}
