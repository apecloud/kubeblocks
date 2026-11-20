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
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func TestResolveFileTemplateVars(t *testing.T) {
	required := ptr.To(appsv1.VarRequired)
	resourceVar := func(name string, vars appsv1.ResourceVars) appsv1.EnvVar {
		return appsv1.EnvVar{Name: name, ValueFrom: &appsv1.VarSource{ResourceVarRef: &appsv1.ResourceVarSelector{ResourceVars: vars}}}
	}
	vars := []appsv1.EnvVar{
		resourceVar("CPU", appsv1.ResourceVars{CPU: required}),
		resourceVar("CPU_LIMIT", appsv1.ResourceVars{CPULimit: required}),
		resourceVar("MEMORY", appsv1.ResourceVars{Memory: required}),
		resourceVar("MEMORY_LIMIT", appsv1.ResourceVars{MemoryLimit: required}),
		resourceVar("DATA", appsv1.ResourceVars{Storage: &appsv1.NamedVar{Name: "data", Option: required}}),
		resourceVar("LOGS", appsv1.ResourceVars{Storage: &appsv1.NamedVar{Name: "logs", Option: required}}),
		resourceVar("ARCHIVE", appsv1.ResourceVars{Storage: &appsv1.NamedVar{Name: "archive", Option: required}}),
		{Name: "VERSION", ValueFrom: &appsv1.VarSource{ComponentVarRef: &appsv1.ComponentVarSelector{ComponentVars: appsv1.ComponentVars{ServiceVersion: required}}}},
		{Name: "REPLICAS", ValueFrom: &appsv1.VarSource{ComponentVarRef: &appsv1.ComponentVarSelector{ComponentVars: appsv1.ComponentVars{Replicas: required}}}},
		{Name: "PODS", ValueFrom: &appsv1.VarSource{ComponentVarRef: &appsv1.ComponentVarSelector{ComponentVars: appsv1.ComponentVars{PodNames: required}}}},
		{Name: "DOUBLED", Expression: ptr.To("{{ mul .CPU 2 }}")},
		{Name: "REFERENCE", Value: "$(CPU)/$(VERSION)"},
		{Name: "LITERAL", Value: "unchanged"},
	}
	cli, synth, instance := fileTemplateVarsFixture(t)
	original, env, err := ResolveTemplateNEnvVars(context.Background(), cli, synth, vars)
	if err != nil {
		t.Fatal(err)
	}
	componentVars, err := ResolveFileTemplateVars(context.Background(), cli, synth, vars, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(componentVars, original) {
		t.Fatalf("default file vars differ: %v != %v", componentVars, original)
	}
	scoped, err := ResolveFileTemplateVars(context.Background(), cli, synth, vars, instance)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"CPU": "3", "CPU_LIMIT": "4", "MEMORY": "1073741824", "MEMORY_LIMIT": "4294967296",
		"DATA": "32212254720", "LOGS": "21474836480", "ARCHIVE": "42949672960",
		"VERSION": "2.0.0", "REPLICAS": original["REPLICAS"], "PODS": original["PODS"],
		"DOUBLED": "6", "REFERENCE": "3/2.0.0", "LITERAL": "unchanged",
	} {
		if scoped[name] != want {
			t.Errorf("%s = %q, want %q", name, scoped[name], want)
		}
	}
	after, afterEnv, err := ResolveTemplateNEnvVars(context.Background(), cli, synth, vars)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, after) || !reflect.DeepEqual(env, afterEnv) {
		t.Fatal("file resolution changed Component variables or environment")
	}
	instance.Resources = &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0"), corev1.ResourceMemory: resource.MustParse("1536Mi")},
		Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("6")},
	}
	instance.ServiceVersion = ""
	scoped, err = ResolveFileTemplateVars(context.Background(), cli, synth, vars, instance)
	if err != nil {
		t.Fatal(err)
	}
	if scoped["CPU"] != "0" || scoped["CPU_LIMIT"] != "6" || scoped["MEMORY"] != "1610612736" || scoped["MEMORY_LIMIT"] != original["MEMORY_LIMIT"] || scoped["VERSION"] != original["VERSION"] {
		t.Fatalf("explicit zero or missing override inheritance failed: %v", scoped)
	}
}

func TestResolveFileTemplateVarsSelectors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ref       appsv1.ClusterObjectReference
		want      map[string]string
		wantError string
	}{
		{name: "omitted definition", want: map[string]string{"CPU": "3", "VERSION": "2.0.0"}},
		{name: "explicit current definition", ref: appsv1.ClusterObjectReference{CompDef: "db-v1"}, want: map[string]string{"CPU": "3", "VERSION": "2.0.0"}},
		{name: "singleton multiple selector", ref: appsv1.ClusterObjectReference{CompDef: "db-v1", MultipleClusterObjectOption: &appsv1.MultipleClusterObjectOption{Strategy: appsv1.MultipleClusterObjectStrategyIndividual}}, want: map[string]string{"CPU": "3", "VERSION": "2.0.0"}},
		{name: "other component", ref: appsv1.ClusterObjectReference{CompDef: "other-v1"}, want: map[string]string{"CPU": "7", "VERSION": "1.0.0"}},
		{name: "individual components", ref: appsv1.ClusterObjectReference{CompDef: "db", MultipleClusterObjectOption: &appsv1.MultipleClusterObjectOption{Strategy: appsv1.MultipleClusterObjectStrategyIndividual}}, want: map[string]string{"CPU_DB": "2", "CPU_PEER": "5", "VERSION_DB": "1.0.0", "VERSION_PEER": "1.0.0"}},
		{name: "combined components", ref: appsv1.ClusterObjectReference{CompDef: "db", MultipleClusterObjectOption: &appsv1.MultipleClusterObjectOption{Strategy: appsv1.MultipleClusterObjectStrategyCombined, CombinedOption: &appsv1.MultipleClusterObjectCombinedOption{ValueFormat: appsv1.FlattenFormat, FlattenFormat: &appsv1.MultipleClusterObjectValueFormatFlatten{Delimiter: ",", KeyValueDelimiter: ":"}}}}, want: map[string]string{"CPU": "db:2,peer:5", "VERSION": "db:1.0.0,peer:1.0.0"}},
		{name: "optional absent object", ref: appsv1.ClusterObjectReference{CompDef: "absent-v1", Optional: ptr.To(true)}, want: map[string]string{}},
		{name: "required absent object", ref: appsv1.ClusterObjectReference{CompDef: "absent-v1"}, wantError: "has no Component object"},
		{name: "no matching component", ref: appsv1.ClusterObjectReference{CompDef: "unknown"}, wantError: "has no Component object"},
		{name: "ambiguous selector", ref: appsv1.ClusterObjectReference{CompDef: "shared"}, wantError: "more than one referent component"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cli, synth, instance := fileTemplateVarsFixture(t)
			synth.Comp2CompDefs = map[string]string{"db": "db-v1", "peer": "db-v2", "other": "other-v1", "absent": "absent-v1", "a": "shared-v1", "b": "shared-v2"}
			vars := []appsv1.EnvVar{
				{Name: "CPU", ValueFrom: &appsv1.VarSource{ResourceVarRef: &appsv1.ResourceVarSelector{ClusterObjectReference: tc.ref, ResourceVars: appsv1.ResourceVars{CPU: ptr.To(appsv1.VarRequired)}}}},
				{Name: "VERSION", ValueFrom: &appsv1.VarSource{ComponentVarRef: &appsv1.ComponentVarSelector{ClusterObjectReference: tc.ref, ComponentVars: appsv1.ComponentVars{ServiceVersion: ptr.To(appsv1.VarRequired)}}}},
			}
			got, err := ResolveFileTemplateVars(context.Background(), cli, synth, vars, instance)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for name, want := range tc.want {
				if got[name] != want {
					t.Errorf("%s = %q, want %q", name, got[name], want)
				}
			}
			if len(tc.want) == 0 && len(got) != 0 {
				t.Fatalf("optional missing object produced vars: %v", got)
			}
		})
	}
}

func fileTemplateVarsFixture(t *testing.T) (client.Client, *SynthesizedComponent, *appsv1.InstanceTemplate) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := workloadsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	claim := func(name, size string) appsv1.PersistentVolumeClaimTemplate {
		return appsv1.PersistentVolumeClaimTemplate{Name: name, Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}}}}
	}
	comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "cluster-db"}, Spec: appsv1.ComponentSpec{
		CompDef: "db-v1", ServiceVersion: "1.0.0", Replicas: 3,
		Resources:            corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("1Gi")}, Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4"), corev1.ResourceMemory: resource.MustParse("2Gi")}},
		VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{claim("data", "10Gi"), claim("logs", "20Gi")},
	}}
	instance := &appsv1.InstanceTemplate{Name: "canary", ServiceVersion: "2.0.0", Replicas: ptr.To(int32(1)),
		Resources:            &corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("3")}, Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("4Gi")}},
		VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{claim("data", "30Gi"), claim("archive", "40Gi")},
	}
	comp.Spec.Instances = []appsv1.InstanceTemplate{*instance.DeepCopy()}
	other := comp.DeepCopy()
	other.Name = "cluster-other"
	other.Spec.CompDef = "other-v1"
	other.Spec.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("7")
	peer := comp.DeepCopy()
	peer.Name = "cluster-peer"
	peer.Spec.CompDef = "db-v2"
	peer.Spec.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("5")
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(comp, other, peer).Build()
	synth := &SynthesizedComponent{Namespace: "test", ClusterName: "cluster", Name: "db", CompDefName: "db-v1", Comp2CompDefs: map[string]string{"db": "db-v1"}}
	return cli, synth, instance
}
