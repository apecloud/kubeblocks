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

package multicluster

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestObjectLocationKnownAndUnknown(t *testing.T) {
	tests := []struct {
		name      string
		ctx       context.Context
		placement *string
		known     bool
		location  string
	}{
		{name: "missing provenance", ctx: context.Background()},
		{name: "known local", ctx: IntoContext(context.Background(), ""), known: true},
		{name: "single worker context", ctx: IntoContext(context.Background(), "worker-a"), known: true, location: "worker-a"},
		{name: "ambiguous context", ctx: IntoContext(context.Background(), "worker-a,worker-b")},
		{name: "exact object overrides ambiguous context", ctx: IntoContext(context.Background(), "worker-a,worker-b"), placement: func() *string { s := "worker-b"; return &s }(), known: true, location: "worker-b"},
		{name: "ambiguous object cannot use single context", ctx: IntoContext(context.Background(), "worker-a"), placement: func() *string { s := "worker-a,worker-b"; return &s }()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod"}}
			if tt.placement != nil {
				pod.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: *tt.placement}
			}
			location, known := ObjectLocation(tt.ctx, pod)
			if known != tt.known || location != tt.location {
				t.Fatalf("location=%q known=%t", location, known)
			}
		})
	}
	scheme := runtime.NewScheme()
	local := fake.NewClientBuilder().WithScheme(scheme).Build()
	if location, known := ObjectLocation(ObservationContext(context.Background(), local), &corev1.Pod{}); !known || location != "" {
		t.Fatal("single-cluster reader lacked local provenance")
	}
	if _, known := ObjectLocation(ObservationContext(context.Background(), &mclient{}), &corev1.Pod{}); known {
		t.Fatal("multicluster missing provenance guessed local")
	}
}
