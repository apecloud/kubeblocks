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
	"strings"
	"testing"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestNormalizeVolumeExpansionInstanceTargets(t *testing.T) {
	component := &appsv1.ClusterComponentSpec{
		Name: "db",
		VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
			Name: "data",
			Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")}}},
		}},
		Instances: []appsv1.InstanceTemplate{{Name: "az-a"}},
	}
	request := VolumeExpansion{
		VolumeClaimTemplates: []OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("5Gi")}},
		Instances:            []InstanceVolumeClaimTemplate{{Name: "az-a", VolumeClaimTemplates: []OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("3Gi")}}}},
	}
	plan, err := NormalizeVolumeExpansion(request, component)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Targets) != 2 {
		t.Fatalf("targets=%d, want 2", len(plan.Targets))
	}
	target := plan.Targets[VolumeExpansionTargetKey{InstanceTemplateName: "az-a", VCTName: "data"}]
	if got := target.RequestedStorage.String(); got != "3Gi" {
		t.Fatalf("instance target=%s, want 3Gi", got)
	}
}

func TestNormalizeVolumeExpansionRejectsUnknownAndShrink(t *testing.T) {
	component := &appsv1.ClusterComponentSpec{Name: "db", VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{Name: "data", Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")}}}}}, Instances: []appsv1.InstanceTemplate{{Name: "az-a"}}}
	for name, request := range map[string]VolumeExpansion{
		"unknown instance": {Instances: []InstanceVolumeClaimTemplate{{Name: "missing", VolumeClaimTemplates: []OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("3Gi")}}}}},
		"shrink":           {Instances: []InstanceVolumeClaimTemplate{{Name: "az-a", VolumeClaimTemplates: []OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("1Gi")}}}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NormalizeVolumeExpansion(request, component); err == nil {
				t.Fatal("expected validation error")
			} else if !strings.Contains(err.Error(), "not found") && name == "unknown instance" {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
