/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package operations

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
)

func TestRebuildRuntimeKeepsRetainedPVCOnlyInstance(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}

	const (
		namespace    = "default"
		clusterName  = "test-cluster"
		component    = "mysql"
		instanceName = "test-cluster-mysql-0"
	)
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Namespace: namespace,
		Name:      "data-" + instanceName,
		Labels:    constant.GetCompLabels(clusterName, component),
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc).Build()
	r := newRebuildInstanceRuntime(context.Background(), cli, &appsv1.Cluster{})

	instance, err := r.getInstance(namespace, clusterName, component, instanceName)
	if err != nil {
		t.Fatalf("get retained-PVC-only instance: %v", err)
	}
	if instance.name != instanceName || instance.componentName != component {
		t.Fatalf("unexpected instance identity: %#v", instance)
	}
	if instance.isAvailable(0, false) || instance.isFailedAndTimedOut() || instance.isDeleting() {
		t.Fatalf("retained-PVC-only instance must keep its non-Pod state: %#v", instance)
	}

	_, err = r.getInstance(namespace, clusterName, component, "missing")
	if !apierrors.IsNotFound(err) {
		t.Fatalf("missing instance should remain NotFound, got %v", err)
	}
}

func TestRebuildRuntimeKeepsMultiClusterRouting(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	const (
		namespace    = "default"
		clusterName  = "test-cluster"
		component    = "mysql"
		instanceName = "test-cluster-mysql-0"
		placement    = "data-context"
	)
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		constant.KBAppMultiClusterPlacementKey: placement,
	}}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Namespace: namespace,
		Name:      instanceName,
		Labels:    constant.GetCompLabels(clusterName, component),
	}}
	var getCalls, listCalls int
	assertDataRouting := func(ctx context.Context, opts ...any) {
		t.Helper()
		gotPlacement, err := multicluster.FromContext(ctx)
		if err != nil || gotPlacement != placement {
			t.Fatalf("unexpected data context placement %q: %v", gotPlacement, err)
		}
		for _, opt := range opts {
			if _, ok := opt.(*multicluster.ClientOption); ok {
				return
			}
		}
		t.Fatal("missing data-context client option")
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, cli client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			getCalls++
			dataOpts := make([]any, len(opts))
			for i := range opts {
				dataOpts[i] = opts[i]
			}
			assertDataRouting(ctx, dataOpts...)
			return cli.Get(ctx, key, obj, opts...)
		},
		List: func(ctx context.Context, cli client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			listCalls++
			dataOpts := make([]any, len(opts))
			for i := range opts {
				dataOpts[i] = opts[i]
			}
			assertDataRouting(ctx, dataOpts...)
			return cli.List(ctx, list, opts...)
		},
	}).Build()
	r := newRebuildInstanceRuntime(context.Background(), cli, cluster)
	if _, err := r.getInstance(namespace, clusterName, component, instanceName); err != nil {
		t.Fatalf("get instance through data context: %v", err)
	}
	if _, err := r.listInstances(namespace, clusterName, component); err != nil {
		t.Fatalf("list instances through data context: %v", err)
	}
	if getCalls == 0 || listCalls < 3 {
		t.Fatalf("expected routed Get and Pod/PVC List calls, got get=%d list=%d", getCalls, listCalls)
	}
}
