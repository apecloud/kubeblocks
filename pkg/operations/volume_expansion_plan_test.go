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

package operations

import (
	"strings"
	"testing"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestNormalizeVolumeExpansionInstanceTargets(t *testing.T) {
	component := &appsv1.ClusterComponentSpec{Name: "db", VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
		Name: "data", Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")}}},
	}}, Instances: []appsv1.InstanceTemplate{{Name: "az-a"}}}
	request := opsv1alpha1.VolumeExpansion{
		VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("5Gi")}},
		Instances:            []opsv1alpha1.InstanceVolumeClaimTemplate{{Name: "az-a", VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("3Gi")}}}},
	}
	plan, err := normalizeVolumeExpansion(request, component)
	if err != nil {
		t.Fatal(err)
	}
	target := plan.Targets[volumeExpansionTargetKey{InstanceTemplateName: "az-a", VCTName: "data"}]
	if len(plan.Targets) != 2 || target.RequestedStorage.Cmp(resource.MustParse("3Gi")) != 0 {
		t.Fatalf("normalized targets = %#v", plan.Targets)
	}
}

func TestNormalizeVolumeExpansionRejectsUnknownAndShrink(t *testing.T) {
	component := &appsv1.ClusterComponentSpec{Name: "db", VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
		Name: "data", Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")}}},
	}}, Instances: []appsv1.InstanceTemplate{{Name: "az-a"}}}
	cases := map[string]opsv1alpha1.VolumeExpansion{
		"unknown instance": {Instances: []opsv1alpha1.InstanceVolumeClaimTemplate{{Name: "missing", VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("3Gi")}}}}},
		"shrink":           {Instances: []opsv1alpha1.InstanceVolumeClaimTemplate{{Name: "az-a", VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("1Gi")}}}}},
	}
	for name, request := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeVolumeExpansion(request, component); err == nil || name == "unknown instance" && !strings.Contains(err.Error(), "not found") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
