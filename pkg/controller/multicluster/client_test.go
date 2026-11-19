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

package multicluster

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestCompleteListRejectsUnavailableProvider(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	setupScheme(scheme)
	control := fake.NewClientBuilder().WithScheme(scheme).Build()
	available := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "config", Namespace: "default", Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: "available,unavailable"}}}).Build()
	cli := NewClient(control, map[string]client.Client{"available": available, "unavailable": newUnavailableClient("unavailable")})
	list := &corev1.ConfigMapList{}
	if err := cli.List(context.Background(), list, InDataContextUnspecified()); err != nil {
		t.Fatalf("ordinary partial list behavior changed: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatal("missing available provider object")
	}
	if err := cli.List(context.Background(), list, InDataContextUnspecifiedWithCompleteList()); err == nil {
		t.Fatal("complete list silently accepted unavailable provider")
	}
}

func TestCompleteListRecordsExactProvider(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "config", Namespace: "default", Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: "a,b"}}}
	control := fake.NewClientBuilder().WithScheme(scheme).Build()
	a := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()
	b := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm.DeepCopy()).Build()
	cli := NewClient(control, map[string]client.Client{"a": a, "b": b})
	list := &corev1.ConfigMapList{}
	if err := cli.List(context.Background(), list, InDataContextUnspecifiedWithCompleteList()); err != nil {
		t.Fatal(err)
	}
	origins := map[string]bool{}
	for _, item := range list.Items {
		origins[item.Annotations[constant.KBAppMultiClusterPlacementKey]] = true
	}
	if len(list.Items) != 2 || !origins["a"] || !origins["b"] {
		t.Fatalf("wrong provider origins: %v", origins)
	}
	for _, provider := range []client.Client{a, b} {
		actual := &corev1.ConfigMap{}
		if err := provider.Get(context.Background(), client.ObjectKeyFromObject(cm), actual); err != nil {
			t.Fatal(err)
		}
		if actual.Annotations[constant.KBAppMultiClusterPlacementKey] != "a,b" {
			t.Fatal("list persisted origin metadata")
		}
	}
}
