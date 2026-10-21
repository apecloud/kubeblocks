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
	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"
)

func TestExporterEnvIsolation(t *testing.T) {
	secret := func(name, account string) corev1.EnvVar {
		return corev1.EnvVar{Name: name, ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: account}, Key: "password"}}}
	}
	vars := []corev1.EnvVar{secret("ROOT", "root-secret"), secret("MONITOR", "monitor-secret"), {Name: "MODE", Value: "cached"}}
	def := &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{exporterEnvVarsAnnotation: `["MONITOR","MODE"]`}}, Spec: appsv1.ComponentDefinitionSpec{Exporter: &appsv1.Exporter{ContainerName: "metrics"}}}
	exporter, selected, err := exporterEnvSelection(def, vars)
	if err != nil {
		t.Fatal(err)
	}
	comp := &component.SynthesizedComponent{PodSpec: &corev1.PodSpec{InitContainers: []corev1.Container{{Name: "init"}}, Containers: []corev1.Container{{Name: "db"}, {Name: "metrics", Env: []corev1.EnvVar{{Name: "PASSWORD", Value: "$(MONITOR)"}}}}}}
	sources := []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "component-env"}}}}
	injectSelectedExporterEnv(comp, vars, sources, exporter, selected)
	for _, c := range []corev1.Container{comp.PodSpec.InitContainers[0], comp.PodSpec.Containers[0]} {
		if !reflect.DeepEqual(c.Env, vars) || !reflect.DeepEqual(c.EnvFrom, sources) {
			t.Fatalf("normal container %s changed", c.Name)
		}
	}
	c := comp.PodSpec.Containers[1]
	if len(c.EnvFrom) != 0 || len(c.Env) != 3 || c.Env[0].Name != "MONITOR" || c.Env[0].ValueFrom.SecretKeyRef.Name != "monitor-secret" || c.Env[1].Name != "MODE" || c.Env[2].Name != "PASSWORD" {
		t.Fatalf("unexpected exporter injection: names=%v", []string{c.Env[0].Name, c.Env[1].Name, c.Env[2].Name})
	}
	for _, v := range c.Env {
		if v.Name == "ROOT" {
			t.Fatal("root leaked into exporter")
		}
	}
	// Selecting an empty array means no component vars and no envFrom.
	def.Annotations[exporterEnvVarsAnnotation] = "[]"
	exporter, selected, err = exporterEnvSelection(def, vars)
	if err != nil || exporter != "metrics" || len(selected) != 0 {
		t.Fatal("empty allowlist must deny all")
	}
	// No exporter in the runtime (disableExporter) remains valid.
	comp.PodSpec.Containers = comp.PodSpec.Containers[:1]
	injectSelectedExporterEnv(comp, vars, sources, exporter, selected)
}

func TestExporterEnvValidationAndLegacy(t *testing.T) {
	def := &appsv1.ComponentDefinition{Spec: appsv1.ComponentDefinitionSpec{Exporter: &appsv1.Exporter{ContainerName: "metrics"}}}
	if name, _, err := exporterEnvSelection(def, nil); err != nil || name != "" {
		t.Fatal("legacy definitions changed")
	}
	comp := &component.SynthesizedComponent{PodSpec: &corev1.PodSpec{Containers: []corev1.Container{{Name: "metrics"}}}}
	vars := []corev1.EnvVar{{Name: "ROOT", Value: "fixture"}}
	injectSelectedExporterEnv(comp, vars, nil, "", nil)
	if !reflect.DeepEqual(comp.PodSpec.Containers[0].Env, vars) {
		t.Fatal("legacy injection changed")
	}
	for _, raw := range []string{`null`, `{}`, `["ROOT","ROOT"]`, `[""]`, `["MISSING"]`, `[1]`, `["ROOT"] junk`} {
		t.Run(raw, func(t *testing.T) {
			def.Annotations = map[string]string{exporterEnvVarsAnnotation: raw}
			if _, _, err := exporterEnvSelection(def, vars); err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
	def.Annotations = map[string]string{exporterEnvVarsAnnotation: `["ROOT"]`}
	if _, _, err := exporterEnvSelection(def, append(vars, vars...)); err == nil {
		t.Fatal("duplicate resolved var accepted")
	}
	def.Spec.Exporter = nil
	if _, _, err := exporterEnvSelection(def, vars); err == nil {
		t.Fatal("policy without exporter accepted")
	}
}
