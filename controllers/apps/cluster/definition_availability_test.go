/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

KubeBlocks is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

KubeBlocks is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with KubeBlocks.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func definitionTestClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func TestValidateDefinitionAvailability(t *testing.T) {
	tests := []struct {
		name               string
		generation         int64
		observedGeneration int64
		phase              appsv1.Phase
		want               string
	}{
		{name: "available", generation: 2, observedGeneration: 2, phase: appsv1.AvailablePhase},
		{name: "stale", generation: 2, observedGeneration: 1, phase: appsv1.AvailablePhase, want: "not up to date"},
		{name: "unavailable", generation: 2, observedGeneration: 2, phase: appsv1.UnavailablePhase, want: "unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDefinitionAvailability("Definition", "test", tt.generation, tt.observedGeneration, tt.phase)
			if tt.want == "" && err != nil {
				t.Fatalf("validateDefinitionAvailability() error = %v", err)
			}
			if tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("validateDefinitionAvailability() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestResolveShardingDefinitionRejectsUnavailableLatest(t *testing.T) {
	available := &appsv1.ShardingDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-v1", Generation: 1},
		Status: appsv1.ShardingDefinitionStatus{
			ObservedGeneration: 1,
			Phase:              appsv1.AvailablePhase,
		},
	}
	unavailable := available.DeepCopy()
	unavailable.Name = "redis-v2"
	unavailable.Status.Phase = appsv1.UnavailablePhase

	_, err := resolveShardingDefinition(context.Background(),
		definitionTestClient(t, available, unavailable), "^redis-v")
	if err == nil || !strings.Contains(err.Error(), "redis-v2") || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("resolveShardingDefinition() error = %v", err)
	}
}

func TestResolveComponentDefinitionRejectsUnavailable(t *testing.T) {
	compDef := &appsv1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-v1", Generation: 1},
		Spec:       appsv1.ComponentDefinitionSpec{ServiceVersion: "7.0.0"},
		Status: appsv1.ComponentDefinitionStatus{
			ObservedGeneration: 1,
			Phase:              appsv1.UnavailablePhase,
		},
	}

	_, _, err := resolveCompDefinitionNServiceVersion(context.Background(),
		definitionTestClient(t, compDef), compDef.Name, compDef.Spec.ServiceVersion)
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("resolveCompDefinitionNServiceVersion() error = %v", err)
	}
}
