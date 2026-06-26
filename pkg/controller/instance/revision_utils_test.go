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

package instance

import (
	"encoding/json"
	"hash/fnv"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

func TestNewRevision(t *testing.T) {
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetUID(types.UID("uid-1234")).
		AddAnnotations("custom-annotation", "val").
		SetPodTemplate(corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "mysql"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
			},
		}).
		SetSelectorMatchLabels(map[string]string{"app": "mysql"}).
		GetObject()

	cr, err := newRevision(inst)
	if err != nil {
		t.Fatalf("newRevision() error = %v", err)
	}
	if cr == nil {
		t.Fatal("newRevision() returned nil")
	}
	if cr.Revision != 1 {
		t.Fatalf("revision = %d, want 1", cr.Revision)
	}
	if cr.Labels[controllerRevisionHashLabel] == "" {
		t.Fatal("expected hash label to be set")
	}
	if cr.Annotations["custom-annotation"] != "val" {
		t.Fatalf("expected annotation to be copied, got %#v", cr.Annotations)
	}
	if len(cr.OwnerReferences) != 1 {
		t.Fatalf("expected 1 owner reference, got %d", len(cr.OwnerReferences))
	}
	if cr.OwnerReferences[0].Name != "mysql-0" {
		t.Fatalf("owner ref name = %s, want mysql-0", cr.OwnerReferences[0].Name)
	}
}

func TestGetPatch(t *testing.T) {
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetUID(types.UID("uid-1234")).
		SetPodTemplate(corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
			},
		}).
		SetSelectorMatchLabels(map[string]string{"app": "mysql"}).
		GetObject()

	patch, err := getPatch(inst)
	if err != nil {
		t.Fatalf("getPatch() error = %v", err)
	}
	if len(patch) == 0 {
		t.Fatal("expected non-empty patch")
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(patch, &raw); err != nil {
		t.Fatalf("patch is not valid JSON: %v", err)
	}
	spec, ok := raw["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("expected spec in patch")
	}
	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		t.Fatal("expected template in spec")
	}
	if template["$patch"] != "replace" {
		t.Fatalf("expected $patch=replace in template, got %v", template["$patch"])
	}
}

func TestNewControllerRevision(t *testing.T) {
	parent := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysql-0",
			Namespace: "default",
			UID:       types.UID("uid-1234"),
			Labels:    map[string]string{"app": "mysql"},
		},
	}
	data := runtime.RawExtension{Raw: []byte(`{"test":"data"}`)}
	collision := int32(0)

	cr, err := newControllerRevision(parent, controllerKind, parent.Labels, data, 1, &collision)
	if err != nil {
		t.Fatalf("newControllerRevision() error = %v", err)
	}
	if cr == nil {
		t.Fatal("newControllerRevision() returned nil")
	}
	if cr.Revision != 1 {
		t.Fatalf("revision = %d, want 1", cr.Revision)
	}
	if cr.Labels["app"] != "mysql" {
		t.Fatalf("expected label app=mysql, got %#v", cr.Labels)
	}
	if cr.Labels[controllerRevisionHashLabel] == "" {
		t.Fatal("expected hash label to be set")
	}
	if !strings.HasPrefix(cr.Name, "mysql-0-") {
		t.Fatalf("expected name to start with 'mysql-0-', got %s", cr.Name)
	}
	if len(cr.OwnerReferences) != 1 {
		t.Fatalf("expected 1 owner reference, got %d", len(cr.OwnerReferences))
	}
}

func TestHashControllerRevision(t *testing.T) {
	// with raw data
	cr := &appsv1.ControllerRevision{
		Data: runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
	}
	hash1 := hashControllerRevision(cr, nil)
	if hash1 == "" {
		t.Fatal("expected non-empty hash")
	}

	// with probe
	probe := int32(1)
	hash2 := hashControllerRevision(cr, &probe)
	if hash2 == "" {
		t.Fatal("expected non-empty hash with probe")
	}
	if hash1 == hash2 {
		t.Fatal("hash should differ when probe is added")
	}

	// empty data
	emptyCR := &appsv1.ControllerRevision{}
	hash3 := hashControllerRevision(emptyCR, nil)
	if hash3 == "" {
		t.Fatal("expected non-empty hash for empty data")
	}
}

func TestDeepHashObject(t *testing.T) {
	hasher := fnv.New32()
	deepHashObject(hasher, map[string]string{"key": "value"})
	h1 := hasher.Sum32()
	if h1 == 0 {
		t.Fatal("expected non-zero hash")
	}
	// Same object should produce same hash
	hasher2 := fnv.New32()
	deepHashObject(hasher2, map[string]string{"key": "value"})
	h2 := hasher2.Sum32()
	if h1 != h2 {
		t.Fatalf("expected same hash for same object: %d vs %d", h1, h2)
	}
	// Different object should produce different hash
	hasher3 := fnv.New32()
	deepHashObject(hasher3, map[string]string{"key": "different"})
	h3 := hasher3.Sum32()
	if h1 == h3 {
		t.Fatal("expected different hash for different object")
	}
}

func TestControllerRevisionName(t *testing.T) {
	// normal case
	name := controllerRevisionName("mysql-0", "abc123")
	if name != "mysql-0-abc123" {
		t.Fatalf("name = %s, want mysql-0-abc123", name)
	}
	// long prefix truncation
	longPrefix := strings.Repeat("a", 300)
	name = controllerRevisionName(longPrefix, "hash")
	if len(name) > 253 {
		t.Fatalf("name length = %d, should be <= 253", len(name))
	}
	if !strings.HasSuffix(name, "-hash") {
		t.Fatalf("expected name to end with -hash, got %s", name)
	}
	if len(name) != 223+len("-hash") {
		t.Fatalf("expected truncated name length %d, got %d", 223+len("-hash"), len(name))
	}
}

func TestGetPodRevision(t *testing.T) {
	// nil labels
	pod := &corev1.Pod{}
	if rev := getPodRevision(pod); rev != "" {
		t.Fatalf("getPodRevision() with nil labels = %q, want empty", rev)
	}
	// no revision label
	pod.Labels = map[string]string{"app": "mysql"}
	if rev := getPodRevision(pod); rev != "" {
		t.Fatalf("getPodRevision() without revision label = %q, want empty", rev)
	}
	// with revision label
	pod.Labels[appsv1.ControllerRevisionHashLabelKey] = "rev-123"
	if rev := getPodRevision(pod); rev != "rev-123" {
		t.Fatalf("getPodRevision() = %q, want rev-123", rev)
	}
}

func TestBuildInstancePodRevision(t *testing.T) {
	parent := builder.NewInstanceBuilder("default", "mysql-0").
		SetUID(types.UID("uid-1234")).
		AddAnnotations("annotation-key", "val").
		SetPodTemplate(corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "mysql"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
			},
		}).
		SetSelectorMatchLabels(map[string]string{"app": "mysql"}).
		GetObject()

	template := &parent.Spec.Template

	rev, err := buildInstancePodRevision(template, parent)
	if err != nil {
		t.Fatalf("buildInstancePodRevision() error = %v", err)
	}
	if rev == "" {
		t.Fatal("expected non-empty revision")
	}

	// same template should produce same revision
	rev2, err := buildInstancePodRevision(template, parent)
	if err != nil {
		t.Fatalf("buildInstancePodRevision() second call error = %v", err)
	}
	if rev != rev2 {
		t.Fatalf("expected same revision for same template: %s vs %s", rev, rev2)
	}

	// different template should produce different revision
	template2 := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "mysql",
				Env:  []corev1.EnvVar{{Name: "EXTRA", Value: "v"}},
			}},
		},
	}
	rev3, err := buildInstancePodRevision(template2, parent)
	if err != nil {
		t.Fatalf("buildInstancePodRevision() with different template error = %v", err)
	}
	if rev == rev3 {
		t.Fatal("expected different revision for different template")
	}
}

func TestControllerKindVar(t *testing.T) {
	expected := appsv1.SchemeGroupVersion.WithKind("Instance")
	if controllerKind != expected {
		t.Fatalf("controllerKind = %v, want %v", controllerKind, expected)
	}
}

func TestNewControllerRevisionWithNilLabels(t *testing.T) {
	parent := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-0",
			Namespace: "default",
			UID:       types.UID("uid-test"),
		},
	}
	data := runtime.RawExtension{Raw: []byte(`{}`)}
	collision := int32(0)
	cr, err := newControllerRevision(parent, controllerKind, nil, data, 1, &collision)
	if err != nil {
		t.Fatalf("newControllerRevision() error = %v", err)
	}
	if cr == nil {
		t.Fatal("expected non-nil ControllerRevision")
	}
}

// Ensure we import schema for the test file to compile
var _ = schema.GroupVersion{}
