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
	"strings"
	"testing"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

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

func TestValidateUpgradeInstanceTemplateServiceVersion(t *testing.T) {
	for _, tc := range []struct {
		name          string
		version       *string
		parentVersion *string
		wantError     string
	}{
		{name: "omitted service version skips validation"},
		{name: "empty service version skips validation", version: strPtr("")},
		{name: "release", version: strPtr("17.5.0")},
		{name: "prerelease", version: strPtr("17.5.0-alpha.1")},
		{name: "build metadata", version: strPtr("17.5.0+build123")},
		{name: "v prefix", version: strPtr("v17.5.0")},
		{name: "missing patch", version: strPtr("17.5"), wantError: `instance template "canary" serviceVersion "17.5"`},
		{name: "random text", version: strPtr("not-a-version"), wantError: `instance template "canary" serviceVersion "not-a-version"`},
		{name: "parent and template versions", version: strPtr("18.0.0"), parentVersion: strPtr("17.5.0")},
		{name: "valid parent invalid template", version: strPtr("18.0"), parentVersion: strPtr("17.5.0"), wantError: `instance template "canary"`},
		{name: "invalid parent valid template", version: strPtr("18.0.0"), parentVersion: strPtr("17.5"), wantError: `serviceVersion "17.5"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := &OpsRequest{Spec: OpsRequestSpec{Type: UpgradeType, SpecificOpsRequest: SpecificOpsRequest{Upgrade: &Upgrade{
				Components: []UpgradeComponent{{
					ComponentOps:   ComponentOps{ComponentName: "pg"},
					ServiceVersion: tc.parentVersion,
					Instances: []UpgradeInstanceTemplate{
						{Name: "stable", ServiceVersion: strPtr("17.5.0")},
						{Name: "canary", ComponentDefinitionName: strPtr("pg-new"), ServiceVersion: tc.version},
					},
				}},
			}}}}
			cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name: "pg", Instances: []appsv1.InstanceTemplate{{Name: "stable"}, {Name: "canary"}},
			}}}}
			err := request.ValidateOps(context.Background(), nil, cluster)
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
}

func TestValidateUpgradeInstanceTemplateTargets(t *testing.T) {
	for _, tc := range []struct {
		name      string
		component string
		instance  string
		overrides []appsv1.ShardTemplate
		wantError string
	}{
		{name: "component", component: "pg", instance: "canary"},
		{name: "sharding", component: "shards", instance: "canary"},
		{name: "inherited instances", component: "shards", instance: "canary", overrides: []appsv1.ShardTemplate{{Name: "zone-a", CompDef: strPtr("pg-new")}}},
		{name: "missing template", component: "pg", instance: "missing", wantError: "missing"},
		{name: "missing owner", component: "missing", instance: "canary", wantError: "not found"},
		{name: "independent instances", component: "shards", instance: "canary", overrides: []appsv1.ShardTemplate{{Name: "zone-a", Instances: []appsv1.InstanceTemplate{{Name: "canary"}}}}, wantError: `shard template "zone-a" overriding instances`},
		{name: "empty independent instances", component: "shards", instance: "canary", overrides: []appsv1.ShardTemplate{{Name: "zone-a", Instances: []appsv1.InstanceTemplate{}}}, wantError: `shard template "zone-a" overriding instances`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			component := appsv1.ClusterComponentSpec{Name: "pg", Instances: []appsv1.InstanceTemplate{{Name: "canary"}}}
			cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{
				ComponentSpecs: []appsv1.ClusterComponentSpec{component},
				Shardings:      []appsv1.ClusterSharding{{Name: "shards", Template: component, ShardTemplates: tc.overrides}},
			}}
			request := &OpsRequest{Spec: OpsRequestSpec{Type: UpgradeType, SpecificOpsRequest: SpecificOpsRequest{Upgrade: &Upgrade{
				Components: []UpgradeComponent{{ComponentOps: ComponentOps{ComponentName: tc.component},
					Instances: []UpgradeInstanceTemplate{{Name: tc.instance, ComponentDefinitionName: strPtr("pg-new")}},
				}},
			}}}}
			err := request.ValidateOps(context.Background(), nil, cluster)
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

func TestUpgradeInstanceTemplateVersionPointers(t *testing.T) {
	for _, tc := range []struct {
		name string
		json string
	}{
		{name: "component definition only", json: `{"name":"canary","componentDefinitionName":"pg-new"}`},
		{name: "service version only", json: `{"name":"canary","serviceVersion":"18.0.0"}`},
		{name: "clear component definition", json: `{"name":"canary","componentDefinitionName":""}`},
		{name: "clear service version", json: `{"name":"canary","serviceVersion":""}`},
		{name: "clear both", json: `{"name":"canary","componentDefinitionName":"","serviceVersion":""}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var instance UpgradeInstanceTemplate
			if err := json.Unmarshal([]byte(tc.json), &instance); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(instance)
			if err != nil {
				t.Fatal(err)
			}
			if string(encoded) != tc.json {
				t.Fatalf("encoded instance = %s, want %s", encoded, tc.json)
			}
			upgrade := &UpgradeComponent{Instances: []UpgradeInstanceTemplate{instance}}
			copied := upgrade.DeepCopy()
			copied.Instances[0].Name = "changed"
			if copied.Instances[0].ComponentDefinitionName != nil {
				*copied.Instances[0].ComponentDefinitionName = "changed"
			}
			if copied.Instances[0].ServiceVersion != nil {
				*copied.Instances[0].ServiceVersion = "changed"
			}
			encoded, err = json.Marshal(upgrade.Instances[0])
			if err != nil {
				t.Fatal(err)
			}
			if string(encoded) != tc.json {
				t.Fatalf("deep copy changed original instance to %s, want %s", encoded, tc.json)
			}
		})
	}
}
