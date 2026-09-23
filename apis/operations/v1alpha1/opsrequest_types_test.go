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
	"testing"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

var componentName = "mysql"

func mockExposeOps() *OpsRequest {
	ops := &OpsRequest{}
	ops.Spec.Type = ExposeType
	ops.Spec.ExposeList = []Expose{
		{
			ComponentName: componentName,
		},
	}
	return ops
}

func TestToExposeListToMap(t *testing.T) {
	ops := mockExposeOps()
	exposeMap := ops.Spec.ToExposeListToMap()
	if len(exposeMap) != len(ops.Spec.ExposeList) {
		t.Error(`Expected expose map length equals list length`)
	}
	if _, ok := exposeMap[componentName]; !ok {
		t.Error(`Expected component name map exists the key of "mysql"`)
	}
}

func TestValidateVolumeExpansionRejectsHeterogeneousShardingInstances(t *testing.T) {
	volume := func(size string) []appsv1.PersistentVolumeClaimTemplate {
		return []appsv1.PersistentVolumeClaimTemplate{{Name: "data", Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}}}}}
	}
	cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{Shardings: []appsv1.ClusterSharding{{
		Name: "shard", Shards: 2,
		Template:       appsv1.ClusterComponentSpec{Replicas: 1, VolumeClaimTemplates: volume("3Gi"), Instances: []appsv1.InstanceTemplate{{Name: "az-a"}}},
		ShardTemplates: []appsv1.ShardTemplate{{Name: "custom", Shards: ptr.To(int32(2)), VolumeClaimTemplates: volume("3Gi")}},
	}}}}
	ops := &OpsRequest{Spec: OpsRequestSpec{Type: VolumeExpansionType, SpecificOpsRequest: SpecificOpsRequest{VolumeExpansionList: []VolumeExpansion{{
		ComponentOps: ComponentOps{ComponentName: "shard"},
		Instances:    []InstanceVolumeClaimTemplate{{Name: "az-a", VolumeClaimTemplates: []OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("5Gi")}}}},
	}}}}}
	if err := ops.ValidateOps(context.Background(), nil, cluster); err == nil {
		t.Fatal("heterogeneous sharding instance expansion was accepted")
	}
}

func TestSetStatusAndMessage(t *testing.T) {
	p := ProgressStatusDetail{}
	message := "handle successfully"
	p.SetStatusAndMessage(SucceedProgressStatus, message)
	if p.Status != SucceedProgressStatus && p.Message != message {
		t.Error("set progressDetail status and message failed")
	}
}

func TestValidateUpgradeServiceVersion(t *testing.T) {
	tests := []struct {
		name    string
		upgrade *Upgrade
		wantErr bool
	}{
		{
			name:    "nil upgrade",
			upgrade: nil,
			wantErr: true,
		},
		{
			name:    "empty components",
			upgrade: &Upgrade{},
			wantErr: true,
		},
		{
			name: "valid service version with three parts",
			upgrade: &Upgrade{
				Components: []UpgradeComponent{
					{
						ComponentOps:   ComponentOps{ComponentName: "pg"},
						ServiceVersion: strPtr("17.5.0"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid service version without patch",
			upgrade: &Upgrade{
				Components: []UpgradeComponent{
					{
						ComponentOps:   ComponentOps{ComponentName: "pg"},
						ServiceVersion: strPtr("17.5"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "nil service version skips validation",
			upgrade: &Upgrade{
				Components: []UpgradeComponent{
					{
						ComponentOps:            ComponentOps{ComponentName: "pg"},
						ComponentDefinitionName: strPtr("new-compdef"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty service version string is valid",
			upgrade: &Upgrade{
				Components: []UpgradeComponent{
					{
						ComponentOps:   ComponentOps{ComponentName: "pg"},
						ServiceVersion: strPtr(""),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "service version with prefix v is valid",
			upgrade: &Upgrade{
				Components: []UpgradeComponent{
					{
						ComponentOps:   ComponentOps{ComponentName: "pg"},
						ServiceVersion: strPtr("v1.0.0"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid service version random text",
			upgrade: &Upgrade{
				Components: []UpgradeComponent{
					{
						ComponentOps:   ComponentOps{ComponentName: "pg"},
						ServiceVersion: strPtr("not-a-version"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid version with pre-release",
			upgrade: &Upgrade{
				Components: []UpgradeComponent{
					{
						ComponentOps:   ComponentOps{ComponentName: "pg"},
						ServiceVersion: strPtr("17.5.0-alpha.1"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid version with build metadata",
			upgrade: &Upgrade{
				Components: []UpgradeComponent{
					{
						ComponentOps:   ComponentOps{ComponentName: "pg"},
						ServiceVersion: strPtr("17.5.0+build123"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple components with mixed versions",
			upgrade: &Upgrade{
				Components: []UpgradeComponent{
					{
						ComponentOps:   ComponentOps{ComponentName: "pg"},
						ServiceVersion: strPtr("17.5.0"),
					},
					{
						ComponentOps:   ComponentOps{ComponentName: "redis"},
						ServiceVersion: strPtr("7.2"),
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &OpsRequest{}
			if tt.upgrade != nil {
				r.Spec.Upgrade = tt.upgrade
			}
			err := r.validateUpgrade(context.Background(), nil, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateUpgrade() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
