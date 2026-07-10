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

package v1alpha1

import (
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestLifecycleActionExecutionIdentityGolden(t *testing.T) {
	spec := validLifecycleActionExecutionSpec()

	key, canonical, err := ComputeLifecycleActionInvocationKey(spec)
	if err != nil {
		t.Fatalf("compute invocation key: %v", err)
	}
	const wantCanonical = `{"identityVersion":"v1","sourceRef":{"apiGroup":"parameters.kubeblocks.io","kind":"ComponentParameter","namespace":"demo","name":"mysql-parameter","uid":"source-uid"},"workloadRef":{"apiGroup":"workloads.kubeblocks.io","kind":"InstanceSet","namespace":"demo","name":"mysql","uid":"workload-uid"},"actionName":"reconfigure","context":{"type":"Reconfigure","reconfigure":{"configName":"mysql-config","targetConfigHash":"sha256:target","componentParameterGeneration":7,"operationUID":"operation-uid"}}}`
	if string(canonical) != wantCanonical {
		t.Fatalf("canonical invocation bytes changed:\n got: %s\nwant: %s", canonical, wantCanonical)
	}
	// The fixed digest protects the serialized identity contract, not merely determinism within one build.
	const wantKey = "l4bke42knudpv3jxscbk56asg5p7pgr4tm2uv736fziagbtlhjbq"
	if key != wantKey {
		t.Fatalf("invocation key changed: got %q want %q", key, wantKey)
	}

	spec.InvocationKey = key
	name, nameCanonical, err := ComputeLifecycleActionExecutionName(spec)
	if err != nil {
		t.Fatalf("compute execution name: %v", err)
	}
	const wantNameCanonical = `{"nameVersion":"v1","invocationKey":"l4bke42knudpv3jxscbk56asg5p7pgr4tm2uv736fziagbtlhjbq","target":{"type":"Pod","pod":{"clusterContext":{"type":"Placement","placement":"dc-a"},"namespace":"demo","componentName":"mysql","instanceName":"mysql-0","podName":"mysql-0","podUID":"pod-uid"}},"attempt":1}`
	if string(nameCanonical) != wantNameCanonical {
		t.Fatalf("canonical name bytes changed:\n got: %s\nwant: %s", nameCanonical, wantNameCanonical)
	}
	const wantName = "lae-2iolurowemizkxcpvjzzgqgc6fligixfwionji2c43dptf2a4bva"
	if name != wantName {
		t.Fatalf("execution name changed: got %q want %q", name, wantName)
	}
}

func TestLifecycleActionInvocationIdentityBoundaries(t *testing.T) {
	base := validLifecycleActionExecutionSpec()
	base.InvocationKey = ""
	baseKey := mustInvocationKey(t, base)

	tests := []struct {
		name   string
		mutate func(*LifecycleActionExecutionSpec)
	}{
		{name: "source api group", mutate: func(s *LifecycleActionExecutionSpec) { s.SourceRef.APIGroup = "apps.kubeblocks.io" }},
		{name: "source kind", mutate: func(s *LifecycleActionExecutionSpec) { s.SourceRef.Kind = "Other" }},
		{name: "source namespace", mutate: func(s *LifecycleActionExecutionSpec) { s.SourceRef.Namespace = "other" }},
		{name: "source name", mutate: func(s *LifecycleActionExecutionSpec) { s.SourceRef.Name = "other" }},
		{name: "source uid", mutate: func(s *LifecycleActionExecutionSpec) { s.SourceRef.UID = "other" }},
		{name: "workload uid", mutate: func(s *LifecycleActionExecutionSpec) { s.WorkloadRef.UID = "other" }},
		{name: "action", mutate: func(s *LifecycleActionExecutionSpec) { s.ActionName = "other" }},
		{name: "config", mutate: func(s *LifecycleActionExecutionSpec) { s.Context.Reconfigure.ConfigName = "other" }},
		{name: "target hash", mutate: func(s *LifecycleActionExecutionSpec) { s.Context.Reconfigure.TargetConfigHash = "other" }},
		{name: "generation", mutate: func(s *LifecycleActionExecutionSpec) { s.Context.Reconfigure.ComponentParameterGeneration++ }},
		{name: "operation uid", mutate: func(s *LifecycleActionExecutionSpec) { s.Context.Reconfigure.OperationUID = "other" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutated := base
			mutated.SourceRef = base.SourceRef
			mutated.WorkloadRef = base.WorkloadRef
			mutated.Context = base.Context
			reconfigure := *base.Context.Reconfigure
			mutated.Context.Reconfigure = &reconfigure
			tt.mutate(&mutated)
			if got := mustInvocationKey(t, mutated); got == baseKey {
				t.Fatalf("identity mutation did not change invocation key")
			}
		})
	}

	for _, mutate := range []func(*LifecycleActionExecutionSpec){
		func(s *LifecycleActionExecutionSpec) { s.Target.Pod.PodUID = "replacement" },
		func(s *LifecycleActionExecutionSpec) { s.Target.Pod.ClusterContext.Placement = stringPtr("dc-b") },
		func(s *LifecycleActionExecutionSpec) { s.Attempt++ },
	} {
		mutated := validLifecycleActionExecutionSpec()
		baseName := mustExecutionName(t, withInvocationKey(t, validLifecycleActionExecutionSpec()))
		mutate(&mutated)
		if got := mustInvocationKey(t, mutated); got != baseKey {
			t.Fatalf("target or attempt changed logical invocation key: got %q want %q", got, baseKey)
		}
		mutated = withInvocationKey(t, mutated)
		if got := mustExecutionName(t, mutated); got == baseName {
			t.Fatalf("target or attempt mutation did not change execution name")
		}
	}
}

func TestValidateLifecycleActionExecutionIdentity(t *testing.T) {
	execution := validLifecycleActionExecution(t)
	if err := ValidateLifecycleActionExecutionIdentity(execution); err != nil {
		t.Fatalf("valid execution rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*LifecycleActionExecution)
	}{
		{name: "wrong invocation key", mutate: func(e *LifecycleActionExecution) { e.Spec.InvocationKey = strings.Repeat("a", 52) }},
		{name: "wrong name", mutate: func(e *LifecycleActionExecution) { e.Name = "lae-wrong" }},
		{name: "namespace mismatch", mutate: func(e *LifecycleActionExecution) { e.Spec.SourceRef.Namespace = "other" }},
		{name: "owner reference", mutate: func(e *LifecycleActionExecution) { e.OwnerReferences = []metav1.OwnerReference{{Name: "source"}} }},
		{name: "placement annotation", mutate: func(e *LifecycleActionExecution) {
			e.Annotations = map[string]string{MultiClusterPlacementAnnotationKey: "dc-a"}
		}},
		{name: "zero attempt", mutate: func(e *LifecycleActionExecution) { e.Spec.Attempt = 0 }},
		{name: "empty source uid", mutate: func(e *LifecycleActionExecution) { e.Spec.SourceRef.UID = "" }},
		{name: "local with placement", mutate: func(e *LifecycleActionExecution) {
			e.Spec.Target.Pod.ClusterContext.Type = LifecycleActionClusterContextLocal
		}},
		{name: "placement without name", mutate: func(e *LifecycleActionExecution) {
			e.Spec.Target.Pod.ClusterContext.Placement = nil
		}},
		{name: "wrong context discriminator", mutate: func(e *LifecycleActionExecution) {
			e.Spec.Context.Type = "Other"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := execution.DeepCopy()
			tt.mutate(candidate)
			if err := ValidateLifecycleActionExecutionIdentity(candidate); err == nil {
				t.Fatalf("invalid execution accepted")
			}
		})
	}
}

func TestIdentityHelpersRejectIncompleteEnvelopes(t *testing.T) {
	invocationTests := []struct {
		name   string
		mutate func(*LifecycleActionExecutionSpec)
	}{
		{name: "identity version", mutate: func(s *LifecycleActionExecutionSpec) { s.IdentityVersion = "v2" }},
		{name: "source api group", mutate: func(s *LifecycleActionExecutionSpec) { s.SourceRef.APIGroup = "" }},
		{name: "source kind", mutate: func(s *LifecycleActionExecutionSpec) { s.SourceRef.Kind = "" }},
		{name: "source namespace", mutate: func(s *LifecycleActionExecutionSpec) { s.SourceRef.Namespace = "" }},
		{name: "source name", mutate: func(s *LifecycleActionExecutionSpec) { s.SourceRef.Name = "" }},
		{name: "source uid", mutate: func(s *LifecycleActionExecutionSpec) { s.SourceRef.UID = "" }},
		{name: "workload api group", mutate: func(s *LifecycleActionExecutionSpec) { s.WorkloadRef.APIGroup = "" }},
		{name: "workload kind", mutate: func(s *LifecycleActionExecutionSpec) { s.WorkloadRef.Kind = "" }},
		{name: "workload namespace", mutate: func(s *LifecycleActionExecutionSpec) { s.WorkloadRef.Namespace = "" }},
		{name: "workload name", mutate: func(s *LifecycleActionExecutionSpec) { s.WorkloadRef.Name = "" }},
		{name: "workload uid", mutate: func(s *LifecycleActionExecutionSpec) { s.WorkloadRef.UID = "" }},
		{name: "action", mutate: func(s *LifecycleActionExecutionSpec) { s.ActionName = "" }},
		{name: "context type", mutate: func(s *LifecycleActionExecutionSpec) { s.Context.Type = "Other" }},
		{name: "context body", mutate: func(s *LifecycleActionExecutionSpec) { s.Context.Reconfigure = nil }},
		{name: "config name", mutate: func(s *LifecycleActionExecutionSpec) { s.Context.Reconfigure.ConfigName = "" }},
		{name: "target hash", mutate: func(s *LifecycleActionExecutionSpec) { s.Context.Reconfigure.TargetConfigHash = "" }},
		{name: "generation", mutate: func(s *LifecycleActionExecutionSpec) { s.Context.Reconfigure.ComponentParameterGeneration = 0 }},
	}
	for _, tt := range invocationTests {
		t.Run("invocation/"+tt.name, func(t *testing.T) {
			spec := validLifecycleActionExecutionSpec()
			tt.mutate(&spec)
			key, canonical, err := ComputeLifecycleActionInvocationKey(spec)
			if err == nil || key != "" || canonical != nil {
				t.Fatalf("invalid invocation produced consumable identity: key=%q canonical=%q err=%v", key, canonical, err)
			}
		})
	}

	nameTests := []struct {
		name   string
		mutate func(*LifecycleActionExecutionSpec)
	}{
		{name: "invocation key empty", mutate: func(s *LifecycleActionExecutionSpec) { s.InvocationKey = "" }},
		{name: "invocation key alphabet", mutate: func(s *LifecycleActionExecutionSpec) { s.InvocationKey = strings.Repeat("1", 52) }},
		{name: "invocation key length", mutate: func(s *LifecycleActionExecutionSpec) { s.InvocationKey = strings.Repeat("a", 51) }},
		{name: "attempt", mutate: func(s *LifecycleActionExecutionSpec) { s.Attempt = 0 }},
		{name: "target type", mutate: func(s *LifecycleActionExecutionSpec) { s.Target.Type = "Other" }},
		{name: "target body", mutate: func(s *LifecycleActionExecutionSpec) { s.Target.Pod = nil }},
		{name: "target namespace", mutate: func(s *LifecycleActionExecutionSpec) { s.Target.Pod.Namespace = "" }},
		{name: "component", mutate: func(s *LifecycleActionExecutionSpec) { s.Target.Pod.ComponentName = "" }},
		{name: "instance", mutate: func(s *LifecycleActionExecutionSpec) { s.Target.Pod.InstanceName = "" }},
		{name: "pod name", mutate: func(s *LifecycleActionExecutionSpec) { s.Target.Pod.PodName = "" }},
		{name: "pod uid", mutate: func(s *LifecycleActionExecutionSpec) { s.Target.Pod.PodUID = "" }},
		{name: "cluster context type", mutate: func(s *LifecycleActionExecutionSpec) { s.Target.Pod.ClusterContext.Type = "Other" }},
		{name: "local with placement", mutate: func(s *LifecycleActionExecutionSpec) {
			s.Target.Pod.ClusterContext.Type = LifecycleActionClusterContextLocal
		}},
		{name: "placement missing", mutate: func(s *LifecycleActionExecutionSpec) { s.Target.Pod.ClusterContext.Placement = nil }},
		{name: "placement empty", mutate: func(s *LifecycleActionExecutionSpec) { s.Target.Pod.ClusterContext.Placement = stringPtr("") }},
	}
	for _, tt := range nameTests {
		t.Run("name/"+tt.name, func(t *testing.T) {
			spec := withInvocationKey(t, validLifecycleActionExecutionSpec())
			tt.mutate(&spec)
			name, canonical, err := ComputeLifecycleActionExecutionName(spec)
			if err == nil || name != "" || canonical != nil {
				t.Fatalf("invalid target produced consumable name: name=%q canonical=%q err=%v", name, canonical, err)
			}
		})
	}
}

func TestLifecycleActionExecutionAPIDoesNotOwnRetentionOrRawMessages(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeOf(LifecycleActionExecutionSpec{}),
		reflect.TypeOf(LifecycleActionExecutionStatus{}),
		reflect.TypeOf(LifecycleActionFailureDetail{}),
	} {
		assertNoForbiddenJSONField(t, typ)
	}

	execution := validLifecycleActionExecution(t)
	if len(execution.OwnerReferences) != 0 {
		t.Fatalf("normal API fixture must be ownerless")
	}
}

func validLifecycleActionExecutionSpec() LifecycleActionExecutionSpec {
	return LifecycleActionExecutionSpec{
		IdentityVersion: LifecycleActionIdentityVersionV1,
		SourceRef: ObjectIdentityRef{
			APIGroup:  "parameters.kubeblocks.io",
			Kind:      "ComponentParameter",
			Namespace: "demo",
			Name:      "mysql-parameter",
			UID:       types.UID("source-uid"),
		},
		WorkloadRef: ObjectIdentityRef{
			APIGroup:  "workloads.kubeblocks.io",
			Kind:      "InstanceSet",
			Namespace: "demo",
			Name:      "mysql",
			UID:       types.UID("workload-uid"),
		},
		ActionName: "reconfigure",
		Target: LifecycleActionTarget{
			Type: LifecycleActionTargetTypePod,
			Pod: &PodLifecycleActionTarget{
				ClusterContext: LifecycleActionClusterContext{Type: LifecycleActionClusterContextPlacement, Placement: stringPtr("dc-a")},
				Namespace:      "demo",
				ComponentName:  "mysql",
				InstanceName:   "mysql-0",
				PodName:        "mysql-0",
				PodUID:         types.UID("pod-uid"),
			},
		},
		Attempt: 1,
		Context: LifecycleActionContext{
			Type: LifecycleActionContextTypeReconfigure,
			Reconfigure: &ReconfigureLifecycleActionContext{
				ConfigName:                   "mysql-config",
				TargetConfigHash:             "sha256:target",
				ComponentParameterGeneration: 7,
				OperationUID:                 types.UID("operation-uid"),
			},
		},
	}
}

func validLifecycleActionExecution(t *testing.T) *LifecycleActionExecution {
	t.Helper()
	spec := validLifecycleActionExecutionSpec()
	key, _, err := ComputeLifecycleActionInvocationKey(spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.InvocationKey = key
	name, _, err := ComputeLifecycleActionExecutionName(spec)
	if err != nil {
		t.Fatal(err)
	}
	return &LifecycleActionExecution{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: name},
		Spec:       spec,
	}
}

func mustInvocationKey(t *testing.T, spec LifecycleActionExecutionSpec) string {
	t.Helper()
	key, _, err := ComputeLifecycleActionInvocationKey(spec)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func withInvocationKey(t *testing.T, spec LifecycleActionExecutionSpec) LifecycleActionExecutionSpec {
	t.Helper()
	spec.InvocationKey = mustInvocationKey(t, spec)
	return spec
}

func mustExecutionName(t *testing.T, spec LifecycleActionExecutionSpec) string {
	t.Helper()
	name, _, err := ComputeLifecycleActionExecutionName(spec)
	if err != nil {
		t.Fatal(err)
	}
	return name
}

func stringPtr(value string) *string {
	return &value
}

func assertNoForbiddenJSONField(t *testing.T, typ reflect.Type) {
	t.Helper()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		switch strings.ToLower(jsonName) {
		case "message", "rawmessage", "stdout", "stderr", "ttl", "ttlsecondsafterfinished", "retention", "retentionpolicy":
			t.Fatalf("%s exposes forbidden field %q", typ.Name(), jsonName)
		}
	}
}
