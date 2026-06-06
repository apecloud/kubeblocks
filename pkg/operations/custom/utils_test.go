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

package custom

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func newPodWithEnv(name string, envs ...corev1.EnvVar) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main", Env: envs},
			},
		},
	}
}

func TestBuildEnvVars_EmptyVarsReturnsEmpty(t *testing.T) {
	got, err := buildEnvVars(intctrlutil.RequestCtx{}, nil, newPodWithEnv("p"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

// Regression: Skyworth R18.14 Oracle DR add-standby — vars[i].ValueFrom == nil
// must produce a fail-loud FatalError, NOT a nil pointer dereference panic.
func TestBuildEnvVars_NilValueFromIsFailLoud(t *testing.T) {
	vars := []opsv1alpha1.OpsEnvVar{
		{Name: "MISSING_VALUEFROM", ValueFrom: nil},
	}
	pod := newPodWithEnv("p")
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("buildEnvVars must not panic on nil ValueFrom, got panic: %v", r)
		}
	}()
	_, err := buildEnvVars(intctrlutil.RequestCtx{}, nil, pod, vars)
	if err == nil {
		t.Fatalf("expected error for nil ValueFrom, got nil")
	}
	if !strings.Contains(err.Error(), "MISSING_VALUEFROM") {
		t.Fatalf("error must name the offending var, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "nil valueFrom") {
		t.Fatalf("error must describe the nil valueFrom contract violation, got %q", err.Error())
	}
}

// Regression: when EnvVarRef path is taken and the target container does not
// contain the env, the helper must return a fail-loud error that names the
// opsEnvVar (and not panic on a stale envVarRef pointer in a different code path).
func TestBuildEnvVars_EnvNotFoundIsFailLoud(t *testing.T) {
	vars := []opsv1alpha1.OpsEnvVar{
		{
			Name: "WANTED",
			ValueFrom: &opsv1alpha1.OpsVarSource{
				EnvVarRef: &opsv1alpha1.EnvVarRef{
					TargetContainerName: "main",
					EnvName:             "DOES_NOT_EXIST",
				},
			},
		},
	}
	pod := newPodWithEnv("p", corev1.EnvVar{Name: "OTHER", Value: "v"})
	_, err := buildEnvVars(intctrlutil.RequestCtx{}, nil, pod, vars)
	if err == nil {
		t.Fatalf("expected error when target env is missing, got nil")
	}
	if !strings.Contains(err.Error(), "WANTED") {
		t.Fatalf("error must name the opsEnvVar, got %q", err.Error())
	}
}

// Regression: Optional missing var must be silently skipped, not become an error.
func TestBuildEnvVars_OptionalMissingIsSkipped(t *testing.T) {
	vars := []opsv1alpha1.OpsEnvVar{
		{
			Name:     "OPT",
			Optional: pointer.Bool(true),
			ValueFrom: &opsv1alpha1.OpsVarSource{
				EnvVarRef: &opsv1alpha1.EnvVarRef{
					TargetContainerName: "main",
					EnvName:             "DOES_NOT_EXIST",
				},
			},
		},
		{
			Name: "PRESENT",
			ValueFrom: &opsv1alpha1.OpsVarSource{
				EnvVarRef: &opsv1alpha1.EnvVarRef{
					TargetContainerName: "main",
					EnvName:             "FOO",
				},
			},
		},
	}
	pod := newPodWithEnv("p", corev1.EnvVar{Name: "FOO", Value: "bar"})
	got, err := buildEnvVars(intctrlutil.RequestCtx{}, nil, pod, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only PRESENT, got %d entries: %v", len(got), got)
	}
	if got[0].Name != "PRESENT" || got[0].Value != "bar" {
		t.Fatalf("unexpected env var %+v", got[0])
	}
}

// Regression: invalid FieldRef must surface the wrapped parse/exec error,
// not panic via a nil envVarRef on the historical NewFatalError site.
func TestBuildEnvVars_BadFieldRefIsFailLoud(t *testing.T) {
	vars := []opsv1alpha1.OpsEnvVar{
		{
			Name: "FIELD",
			ValueFrom: &opsv1alpha1.OpsVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata..", // malformed jsonpath
				},
			},
		},
	}
	pod := newPodWithEnv("p")
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("buildEnvVars must not panic on malformed FieldPath, got panic: %v", r)
		}
	}()
	_, err := buildEnvVars(intctrlutil.RequestCtx{}, nil, pod, vars)
	if err == nil {
		t.Fatalf("expected error for malformed FieldPath, got nil")
	}
}

// Regression: valid FieldRef must resolve without panic, even though the
// envVarRef branch is not taken.
func TestBuildEnvVars_GoodFieldRefResolves(t *testing.T) {
	vars := []opsv1alpha1.OpsEnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &opsv1alpha1.OpsVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: ".metadata.name",
				},
			},
		},
	}
	pod := newPodWithEnv("ora-s-pod-0")
	got, err := buildEnvVars(intctrlutil.RequestCtx{}, nil, pod, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "POD_NAME" || got[0].Value != "ora-s-pod-0" {
		t.Fatalf("unexpected result: %+v", got)
	}
}
