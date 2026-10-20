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
	"encoding/json"
	"reflect"
	"strings"
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

func TestValidateUpgradeInstanceServiceVersion(t *testing.T) {
	tests := []struct {
		name    string
		version *string
		wantErr bool
	}{
		{name: "omitted version"},
		{name: "empty version", version: ptr.To("")},
		{name: "three parts", version: ptr.To("17.5.0")},
		{name: "v prefix", version: ptr.To("v17.5.0")},
		{name: "prerelease", version: ptr.To("17.5.0-alpha.1")},
		{name: "build metadata", version: ptr.To("17.5.0+build123")},
		{name: "missing patch", version: ptr.To("17.5"), wantErr: true},
		{name: "random text", version: ptr.To("not-a-version"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &OpsRequest{Spec: OpsRequestSpec{Type: UpgradeType, SpecificOpsRequest: SpecificOpsRequest{
				Upgrade: &Upgrade{Components: []UpgradeComponent{{
					ComponentOps: ComponentOps{ComponentName: "pg"},
					Instances: []UpgradeInstanceTemplate{{Name: "canary", ComponentDefinitionName: ptr.To("postgresql-17"),
						ServiceVersion: tt.version}},
				}}},
			}}}
			before := ops.DeepCopy()
			cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name: "pg", Instances: []appsv1.InstanceTemplate{{Name: "canary"}},
			}}}}
			err := ops.ValidateOps(context.Background(), nil, cluster)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateOps() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && (!strings.Contains(err.Error(), "canary") || !strings.Contains(err.Error(), *tt.version)) {
				t.Errorf("validation error does not identify the template and version: %v", err)
			}
			if !reflect.DeepEqual(ops, before) {
				t.Error("validation changed the request")
			}
		})
	}
}

func TestValidateUpgradeMixedTargets(t *testing.T) {
	tests := []struct {
		name       string
		components []UpgradeComponent
		wantErr    bool
	}{
		{
			name: "multiple templates with invalid later version",
			components: []UpgradeComponent{{ComponentOps: ComponentOps{ComponentName: "pg"}, Instances: []UpgradeInstanceTemplate{
				{Name: "stable", ServiceVersion: ptr.To("17.4.0")},
				{Name: "canary", ServiceVersion: ptr.To("17.5")},
			}}},
			wantErr: true,
		},
		{
			name: "parent and template upgrades across components",
			components: []UpgradeComponent{
				{ComponentOps: ComponentOps{ComponentName: "pg"}, ServiceVersion: ptr.To("17.5.0"), Instances: []UpgradeInstanceTemplate{
					{Name: "stable", ServiceVersion: ptr.To("")},
					{Name: "canary", ServiceVersion: ptr.To("17.6.0")},
				}},
				{ComponentOps: ComponentOps{ComponentName: "redis"}, Instances: []UpgradeInstanceTemplate{
					{Name: "canary", ServiceVersion: ptr.To("7.2.0")},
				}},
			},
		},
		{
			name: "invalid template on later component",
			components: []UpgradeComponent{
				{ComponentOps: ComponentOps{ComponentName: "pg"}, ServiceVersion: ptr.To("17.5.0")},
				{ComponentOps: ComponentOps{ComponentName: "redis"}, Instances: []UpgradeInstanceTemplate{
					{Name: "canary", ServiceVersion: ptr.To("7.2")},
				}},
			},
			wantErr: true,
		},
		{
			name: "invalid parent with valid template",
			components: []UpgradeComponent{{ComponentOps: ComponentOps{ComponentName: "pg"}, ServiceVersion: ptr.To("17.5"),
				Instances: []UpgradeInstanceTemplate{{Name: "canary", ServiceVersion: ptr.To("17.6.0")}}}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &OpsRequest{Spec: OpsRequestSpec{Type: UpgradeType, SpecificOpsRequest: SpecificOpsRequest{
				Upgrade: &Upgrade{Components: tt.components},
			}}}
			cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{
				{Name: "pg", Instances: []appsv1.InstanceTemplate{{Name: "stable"}, {Name: "canary"}}},
				{Name: "redis", Instances: []appsv1.InstanceTemplate{{Name: "canary"}}},
			}}}
			if err := ops.ValidateOps(context.Background(), nil, cluster); (err != nil) != tt.wantErr {
				t.Errorf("ValidateOps() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpgradeInstanceTemplateJSONPreservesOverrides(t *testing.T) {
	for _, target := range []UpgradeInstanceTemplate{
		{Name: "canary", ComponentDefinitionName: ptr.To("postgresql-17")},
		{Name: "canary", ServiceVersion: ptr.To("17.5.0")},
		{Name: "canary", ComponentDefinitionName: ptr.To(""), ServiceVersion: ptr.To("")},
	} {
		data, err := json.Marshal(target)
		if err != nil {
			t.Fatal(err)
		}
		var decoded UpgradeInstanceTemplate
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(decoded, target) {
			t.Errorf("JSON round trip changed override presence: got %#v, want %#v", decoded, target)
		}
	}
}

func TestValidateUpgradeInstanceTargets(t *testing.T) {
	for _, tc := range []struct {
		name, owner, template, wantError                 string
		shard, independent, emptyIndependent, parentOnly bool
	}{
		{name: "component", owner: "db", template: "canary"},
		{name: "missing component template", owner: "db", template: "missing", wantError: "missing"},
		{name: "missing owner", owner: "missing", template: "canary", wantError: "canary"},
		{name: "sharding", owner: "shards", template: "canary", shard: true},
		{name: "missing sharding template", owner: "shards", template: "missing", shard: true, wantError: "missing"},
		{name: "independent instances", owner: "shards", template: "canary", shard: true, independent: true, wantError: "special"},
		{name: "empty independent instances", owner: "shards", template: "canary", shard: true, emptyIndependent: true, wantError: "special"},
		{name: "old parent request", owner: "shards", shard: true, independent: true, parentOnly: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := appsv1.ClusterComponentSpec{Name: "db", Instances: []appsv1.InstanceTemplate{{Name: "canary"}}}
			cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{spec}}}
			if tc.shard {
				cluster.Spec.ComponentSpecs = nil
				cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "shards", Shards: 1, Template: spec,
					ShardTemplates: []appsv1.ShardTemplate{{Name: "special", Replicas: ptr.To(int32(2)), ServiceVersion: ptr.To("2.0.0")}},
				}}
				if tc.independent {
					cluster.Spec.Shardings[0].ShardTemplates[0].Instances = []appsv1.InstanceTemplate{{Name: "canary"}}
				}
				if tc.emptyIndependent {
					cluster.Spec.Shardings[0].ShardTemplates[0].Instances = []appsv1.InstanceTemplate{}
				}
			}
			target := UpgradeComponent{ComponentOps: ComponentOps{ComponentName: tc.owner}}
			if tc.parentOnly {
				target.ServiceVersion = ptr.To("1.0.0")
			} else {
				target.Instances = []UpgradeInstanceTemplate{{Name: tc.template, ComponentDefinitionName: ptr.To("db-v2")}}
			}
			ops := &OpsRequest{Spec: OpsRequestSpec{Type: UpgradeType, SpecificOpsRequest: SpecificOpsRequest{
				Upgrade: &Upgrade{Components: []UpgradeComponent{target}},
			}}}
			err := ops.ValidateOps(context.Background(), nil, cluster)
			if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
		})
	}
}
