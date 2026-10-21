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

package instancetemplate

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/klauspost/compress/zstd"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestGetInstanceTemplateExtResolvesUnallocatedTemplates(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		t.Run(map[bool]string{false: "inline", true: "compressed"}[compressed], func(t *testing.T) {
			claim := corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data"}}
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
				Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](0), VolumeClaimTemplates: []corev1.PersistentVolumeClaim{claim}}}
			claim.Annotations = map[string]string{constant.PVCNamePrefixAnnotationKey: "custom"}
			templates := []workloads.InstanceTemplate{{Name: "archived", Replicas: ptr.To[int32](0), VolumeClaimTemplates: []corev1.PersistentVolumeClaim{claim}}}
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			if compressed {
				data, err := json.Marshal(templates)
				if err != nil {
					t.Fatal(err)
				}
				encoder, err := zstd.NewWriter(nil)
				if err != nil {
					t.Fatal(err)
				}
				defer encoder.Close()
				its.Annotations = map[string]string{templateRefAnnotationKey: `{"demo":"templates"}`}
				if err := tree.Add(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "templates", Namespace: its.Namespace},
					BinaryData: map[string][]byte{templateRefDataKey: encoder.EncodeAll(data, nil)}}); err != nil {
					t.Fatal(err)
				}
			} else {
				its.Spec.Instances = templates
			}
			ext, err := BuildInstanceSetExt(its, tree)
			if err != nil {
				t.Fatal(err)
			}
			if ext.InstanceTemplates[""] != nil {
				t.Fatal("zero allocation unexpectedly includes a default pseudo template")
			}
			for _, tc := range []struct{ template, pvcName string }{{"", "data-demo-0"}, {"archived", "custom-0"}} {
				template := GetInstanceTemplateExt(ext, tc.template)
				if template == nil || template.Name != tc.template || len(template.VolumeClaimTemplates) != 1 {
					t.Fatalf("recorded template %q was not resolved: %#v", tc.template, template)
				}
				if got := intctrlutil.ComposePVCName(template.VolumeClaimTemplates[0], its.Name, "demo-0"); got != tc.pvcName {
					t.Fatalf("template %q resolved PVC %q, want %q", tc.template, got, tc.pvcName)
				}
			}
			if got := GetInstanceTemplateExt(ext, "unknown"); got != nil {
				t.Fatalf("unknown named template was replaced by the default: %#v", got)
			}
		})
	}
}

func TestBuildAssignmentsTransfersExplicitFlatOrdinals(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](2), FlatInstanceOrdinal: true, Template: corev1.PodTemplateSpec{},
			Instances: []workloads.InstanceTemplate{
				{Name: "a", Replicas: ptr.To[int32](1), Ordinals: workloads.Ordinals{Discrete: []int32{1}}},
				{Name: "b", Replicas: ptr.To[int32](1), Ordinals: workloads.Ordinals{Discrete: []int32{0}}},
			},
		},
		Status: workloads.InstanceSetStatus{AssignedOrdinals: map[string]workloads.Ordinals{
			"a": {Discrete: []int32{0}}, "b": {Discrete: []int32{1}},
		}},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	assignments, _, err := BuildAssignments(tree, its)
	if err != nil {
		t.Fatal(err)
	}
	expected := []Assignment{{InstanceName: "demo-0", TemplateName: "b"}, {InstanceName: "demo-1", TemplateName: "a"}}
	if !reflect.DeepEqual(assignments, expected) {
		t.Fatalf("explicit ordinal transfer is incomplete: %#v", assignments)
	}
}

func TestBuildAssignmentsUsesFlatOrdinalAssignment(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](2), FlatInstanceOrdinal: true,
			Selector:  &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
			Template:  corev1.PodTemplateSpec{},
			Instances: []workloads.InstanceTemplate{{Name: "fast", Replicas: ptr.To[int32](1)}},
		},
		Status: workloads.InstanceSetStatus{AssignedOrdinals: map[string]workloads.Ordinals{
			"": {Discrete: []int32{0}}, "fast": {Discrete: []int32{2}},
		}},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	assignments, _, err := BuildAssignments(tree, its)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, assignment := range assignments {
		got[assignment.InstanceName] = assignment.TemplateName
	}
	if len(got) != 2 || got["demo-0"] != "" || got["demo-2"] != "fast" {
		t.Fatalf("flat ordinal mapping was guessed incorrectly: %#v", got)
	}
}

func TestTemplateNameFromLabelsPrefersSystemLabel(t *testing.T) {
	templateName, ok := TemplateNameFromLabels(map[string]string{
		TemplateNameLabelKey:                   "old-or-user-value",
		constant.KBAppInstanceTemplateLabelKey: "authoritative",
	})
	if !ok || templateName != "authoritative" {
		t.Fatalf("unexpected template label: %q, %v", templateName, ok)
	}
}

func TestResolveHistoricalTemplateUsesExplicitFlatOrdinalRelation(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: workloads.InstanceSetSpec{
			FlatInstanceOrdinal: true,
			Instances:           []workloads.InstanceTemplate{{Name: "fast", Ordinals: workloads.Ordinals{Discrete: []int32{7}}}},
		},
	}
	templateName, ok, err := ResolveHistoricalTemplate(its, "demo-7", []string{"", "fast"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok || templateName != "fast" {
		t.Fatalf("explicit flat ordinal relation was not used: %q, %v", templateName, ok)
	}
}
