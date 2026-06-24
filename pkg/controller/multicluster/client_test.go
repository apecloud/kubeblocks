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
)

func TestInDataContextOf(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	pod := func(name string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      name,
			},
		}
	}

	cli := NewClient(
		fake.NewClientBuilder().WithScheme(scheme).Build(),
		map[string]client.Client{
			"ctx-a": fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod("pod-a")).Build(),
			"ctx-b": fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod("pod-b")).Build(),
		},
	)

	var pods corev1.PodList
	if err := cli.List(context.Background(), &pods, client.InNamespace("default"), InDataContextOf("ctx-a")); err != nil {
		t.Fatal(err)
	}
	if len(pods.Items) != 1 || pods.Items[0].Name != "pod-a" {
		t.Fatalf("expected only ctx-a pod, got %#v", pods.Items)
	}

	pods = corev1.PodList{}
	if err := cli.List(context.Background(), &pods, client.InNamespace("default"), InDataContextOf("ctx-a, ctx-b")); err != nil {
		t.Fatal(err)
	}
	if len(pods.Items) != 2 {
		t.Fatalf("expected pods from both selected contexts, got %#v", pods.Items)
	}
}
