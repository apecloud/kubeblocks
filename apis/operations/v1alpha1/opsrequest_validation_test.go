/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"strings"
	"testing"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

func TestValidateVolumeExpansion(t *testing.T) {
	volume := func(size string) []appsv1.PersistentVolumeClaimTemplate {
		return []appsv1.PersistentVolumeClaimTemplate{{Name: "data", Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}}}}}
	}
	for _, tc := range []struct {
		name      string
		change    func(*appsv1.ClusterComponentSpec, *VolumeExpansion)
		wantError string
	}{
		{name: "inherited storage"},
		{name: "same size retry", change: func(c *appsv1.ClusterComponentSpec, _ *VolumeExpansion) { c.VolumeClaimTemplates = volume("5Gi") }},
		{name: "reject a smaller declared target", change: func(c *appsv1.ClusterComponentSpec, _ *VolumeExpansion) { c.VolumeClaimTemplates = volume("6Gi") }, wantError: "less than declared size"},
		{name: "missing component", change: func(_ *appsv1.ClusterComponentSpec, r *VolumeExpansion) { r.ComponentName = "missing" }, wantError: "not found"},
		{name: "missing volume", change: func(_ *appsv1.ClusterComponentSpec, r *VolumeExpansion) { r.VolumeClaimTemplates[0].Name = "missing" }, wantError: "not found"},
		{name: "zero replicas with valid volume", change: func(c *appsv1.ClusterComponentSpec, _ *VolumeExpansion) { c.Replicas = 0 }},
		{name: "zero replicas with missing volume", change: func(c *appsv1.ClusterComponentSpec, r *VolumeExpansion) {
			c.Replicas = 0
			r.VolumeClaimTemplates[0].Name = "missing"
		}, wantError: "not found"},
		{name: "zero replicas with smaller target", change: func(c *appsv1.ClusterComponentSpec, _ *VolumeExpansion) {
			c.Replicas = 0
			c.VolumeClaimTemplates = volume("6Gi")
		}, wantError: "less than declared size"},
		{name: "empty volumes", change: func(_ *appsv1.ClusterComponentSpec, r *VolumeExpansion) { r.VolumeClaimTemplates = nil }},
		{name: "matching override", change: func(c *appsv1.ClusterComponentSpec, _ *VolumeExpansion) {
			c.Instances[0].VolumeClaimTemplates = volume("5Gi")
		}},
		{name: "smaller override", change: func(c *appsv1.ClusterComponentSpec, _ *VolumeExpansion) {
			c.Instances[0].VolumeClaimTemplates = volume("4Gi")
		}},
		{name: "larger override", change: func(c *appsv1.ClusterComponentSpec, _ *VolumeExpansion) {
			c.Instances[0].VolumeClaimTemplates = volume("6Gi")
		}},
		{name: "disabled override", change: func(c *appsv1.ClusterComponentSpec, _ *VolumeExpansion) {
			c.Instances[0].VolumeClaimTemplates = volume("4Gi")
			c.Instances[0].Replicas = ptr.To(int32(0))
		}},
		{name: "canary override", change: func(c *appsv1.ClusterComponentSpec, _ *VolumeExpansion) {
			c.Instances[0].VolumeClaimTemplates = volume("4Gi")
			c.Instances[0].Canary = ptr.To(true)
		}},
		{name: "stopped inherits", change: func(c *appsv1.ClusterComponentSpec, _ *VolumeExpansion) { c.Stop = ptr.To(true) }},
		{name: "stopped independent override", change: func(c *appsv1.ClusterComponentSpec, _ *VolumeExpansion) {
			c.Stop = ptr.To(true)
			c.Instances[0].VolumeClaimTemplates = volume("4Gi")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			comp := appsv1.ClusterComponentSpec{Name: "db", Replicas: 2, VolumeClaimTemplates: volume("3Gi"), Instances: []appsv1.InstanceTemplate{{Name: "custom"}}}
			expansion := VolumeExpansion{ComponentOps: ComponentOps{ComponentName: "db"}, VolumeClaimTemplates: []OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("5Gi")}}}
			if tc.change != nil {
				tc.change(&comp, &expansion)
			}
			cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{comp}}}
			ops := &OpsRequest{Spec: OpsRequestSpec{Type: VolumeExpansionType, SpecificOpsRequest: SpecificOpsRequest{VolumeExpansionList: []VolumeExpansion{expansion}}}}
			// A nil client proves this validation needs only the public Cluster declaration.
			err := ops.ValidateOps(context.Background(), nil, cluster)
			if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
		})
	}
	for _, tc := range []struct {
		name      string
		change    func(*appsv1.ClusterSharding)
		wantError bool
	}{
		{name: "inherited sharding"},
		{name: "smaller sharding target", change: func(s *appsv1.ClusterSharding) { s.Template.VolumeClaimTemplates = volume("6Gi") }, wantError: true},
		{name: "independent shard override", change: func(s *appsv1.ClusterSharding) { s.ShardTemplates[0].VolumeClaimTemplates = volume("4Gi") }},
		{name: "matching shard override", change: func(s *appsv1.ClusterSharding) { s.ShardTemplates[0].VolumeClaimTemplates = volume("5Gi") }},
		{name: "missing shard volume", change: func(s *appsv1.ClusterSharding) {
			s.ShardTemplates[0].VolumeClaimTemplates = volume("5Gi")
			s.ShardTemplates[0].VolumeClaimTemplates[0].Name = "other"
		}},
		{name: "inactive shard override", change: func(s *appsv1.ClusterSharding) {
			s.ShardTemplates[0].Shards = ptr.To(int32(0))
			s.ShardTemplates[0].VolumeClaimTemplates = volume("4Gi")
		}},
		{name: "independent instance override", change: func(s *appsv1.ClusterSharding) {
			s.Template.Instances = []appsv1.InstanceTemplate{{Name: "custom", VolumeClaimTemplates: volume("4Gi")}}
		}},
		{name: "replaced instances", change: func(s *appsv1.ClusterSharding) {
			s.Shards = 1
			s.Template.Instances = []appsv1.InstanceTemplate{{Name: "unused", VolumeClaimTemplates: volume("4Gi")}}
			s.ShardTemplates[0].Instances = []appsv1.InstanceTemplate{{Name: "custom", VolumeClaimTemplates: volume("5Gi")}}
		}},
		{name: "instance override replaces unused shard storage", change: func(s *appsv1.ClusterSharding) {
			s.ShardTemplates[0].Replicas = ptr.To(int32(1))
			s.ShardTemplates[0].VolumeClaimTemplates = volume("4Gi")
			s.ShardTemplates[0].Instances = []appsv1.InstanceTemplate{{Name: "custom", VolumeClaimTemplates: volume("5Gi")}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sharding := appsv1.ClusterSharding{Name: "shard", Shards: 2, Template: appsv1.ClusterComponentSpec{Replicas: 2, VolumeClaimTemplates: volume("3Gi")}, ShardTemplates: []appsv1.ShardTemplate{{Name: "custom", Shards: ptr.To(int32(1))}}}
			if tc.change != nil {
				tc.change(&sharding)
			}
			cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{Shardings: []appsv1.ClusterSharding{sharding}}}
			ops := &OpsRequest{Spec: OpsRequestSpec{Type: VolumeExpansionType, SpecificOpsRequest: SpecificOpsRequest{VolumeExpansionList: []VolumeExpansion{{ComponentOps: ComponentOps{ComponentName: "shard"}, VolumeClaimTemplates: []OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("5Gi")}}}}}}}
			err := ops.ValidateOps(context.Background(), nil, cluster)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, wantError = %t", err, tc.wantError)
			}
		})
	}
}
