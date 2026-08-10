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

package instancesetstatus

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestBuildActiveAllocationsUsesFlatOrdinalAssignment(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec: workloads.InstanceSetSpec{
			Replicas:            ptr.To[int32](2),
			FlatInstanceOrdinal: true,
			Selector:            &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
			Template:            corev1.PodTemplateSpec{},
			Instances: []workloads.InstanceTemplate{{
				Name:     "fast",
				Replicas: ptr.To[int32](1),
			}},
		},
		Status: workloads.InstanceSetStatus{AssignedOrdinals: map[string]workloads.Ordinals{
			"":     {Discrete: []int32{0}},
			"fast": {Discrete: []int32{2}},
		}},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	allocations, _, err := BuildActiveAllocations(tree, its)
	if err != nil {
		t.Fatal(err)
	}
	if len(allocations) != 2 {
		t.Fatalf("unexpected allocations: %#v", allocations)
	}
	got := map[string]string{}
	for _, allocation := range allocations {
		got[allocation.PodName] = allocation.TemplateName
	}
	if got["demo-0"] != "" || got["demo-2"] != "fast" {
		t.Fatalf("flat ordinal mapping was guessed incorrectly: %#v", got)
	}
}

func TestTemplateNameFromLabelsPrefersCurrentSystemLabel(t *testing.T) {
	templateName, ok, err := TemplateNameFromLabels(map[string]string{
		instancetemplate.TemplateNameLabelKey:  "legacy-or-user-value",
		constant.KBAppInstanceTemplateLabelKey: "authoritative",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok || templateName != "authoritative" {
		t.Fatalf("unexpected template label: %q, %v", templateName, ok)
	}
}

func TestHistoricalTemplateHintUsesExplicitFlatOrdinalRelation(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: workloads.InstanceSetSpec{
			FlatInstanceOrdinal: true,
			Instances: []workloads.InstanceTemplate{{
				Name:     "fast",
				Ordinals: workloads.Ordinals{Discrete: []int32{7}},
			}},
		},
	}
	templateName, ok, err := HistoricalTemplateHint(its, "demo-7", []string{"", "fast"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok || templateName != "fast" {
		t.Fatalf("explicit flat ordinal relation was not used: %q, %v", templateName, ok)
	}
}
