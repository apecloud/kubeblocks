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
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"k8s.io/utils/ptr"
)

func TestHorizontalScalingAllocationValidationLimits(t *testing.T) {
	for _, online := range []bool{false, true} {
		for _, template := range []string{"", "reader"} {
			name := "scale-in/" + template
			if online {
				name = "scale-out/" + template
			}
			t.Run(name, func(t *testing.T) {
				component := appsv1.ClusterComponentSpec{Name: "db", Replicas: 3,
					Instances: []appsv1.InstanceTemplate{{Name: "reader", Replicas: ptr.To(int32(2))}}}
				request := HorizontalScaling{ComponentOps: ComponentOps{ComponentName: "db"}}
				changer := ReplicaChanger{Instances: []InstanceReplicasTemplate{{Name: "reader", ReplicaChanges: 1}}}
				state := workloadsv1.InstanceDesiredStateActive
				if online {
					state = workloadsv1.InstanceDesiredStateOffline
					request.ScaleOut = &ScaleOut{ReplicaChanger: changer, OfflineInstancesToOnline: []string{"cluster-db-1"}}
				} else {
					request.ScaleIn = &ScaleIn{ReplicaChanger: changer, OnlineInstancesToOffline: []string{"cluster-db-1"}}
				}
				ops := &OpsRequest{}
				validate := func() error {
					return ops.validateHorizontalScalingSpec(request, component, "cluster", false, 4, 2)
				}
				// Before capture, the named instance may be one of the explicit
				// reader changes. The known lower bound is one replica, not two.
				if err := validate(); err != nil {
					t.Fatalf("validation before source capture: %v", err)
				}
				ops.Status.LastConfiguration.Components = map[string]LastComponentConfiguration{"db": {
					Replicas: ptr.To(component.Replicas), Instances: component.Instances,
					SourceInstanceAssignments: []InstanceTemplateAssignment{{WorkloadName: "cluster-db",
						PodName: "cluster-db-1", TemplateName: template, DesiredState: state}},
				}}
				// After capture, the default template is a separate change: the
				// resulting 1 or 5 replicas must violate the limits [2, 4]. The same
				// reader instance instead counts once, regardless of its name.
				err := validate()
				if (err != nil) != (template == "") {
					t.Fatalf("template %q: expected limit violation=%t, got %v", template, template == "", err)
				}
			})
		}
	}
}

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
