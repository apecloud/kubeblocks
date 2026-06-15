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

package rollout

import (
	"strings"
	"testing"

	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func userDefinedComponentCluster() *appsv1.Cluster {
	return &appsv1.Cluster{
		Spec: appsv1.ClusterSpec{
			ComponentSpecs: []appsv1.ClusterComponentSpec{
				{
					Name:         "mysql",
					ComponentDef: "mysql-8.0-1.0.5",
				},
			},
		},
	}
}

func userDefinedShardingCluster() *appsv1.Cluster {
	return &appsv1.Cluster{
		Spec: appsv1.ClusterSpec{
			Shardings: []appsv1.ClusterSharding{
				{
					Name:        "mysql",
					ShardingDef: "mysql-sharding",
					Shards:      1,
					Template: appsv1.ClusterComponentSpec{
						ComponentDef: "mysql-8.0-1.0.5",
					},
				},
			},
		},
	}
}

func TestRolloutSetupPrecheckRejectsServiceVersionOnlyForUserDefinedComponent(t *testing.T) {
	cluster := userDefinedComponentCluster()
	rollout := &appsv1alpha1.Rollout{
		Spec: appsv1alpha1.RolloutSpec{
			Components: []appsv1alpha1.RolloutComponent{
				{
					Name:           "mysql",
					ServiceVersion: ptr.To("8.0.37"),
					Strategy: appsv1alpha1.RolloutStrategy{
						Replace: &appsv1alpha1.RolloutStrategyReplace{},
					},
				},
			},
		},
	}

	err := (&rolloutSetupTransformer{}).precheck(cluster, rollout)
	if err == nil || !strings.Contains(err.Error(), "serviceVersion and compDef must be defined for component mysql") {
		t.Fatalf("expected serviceVersion-only validation error, got %v", err)
	}
}

func TestRolloutSetupPrecheckRejectsCompDefOnlyForUserDefinedComponent(t *testing.T) {
	cluster := userDefinedComponentCluster()
	rollout := &appsv1alpha1.Rollout{
		Spec: appsv1alpha1.RolloutSpec{
			Components: []appsv1alpha1.RolloutComponent{
				{
					Name:    "mysql",
					CompDef: ptr.To("mysql-8.0-1.0.5"),
					Strategy: appsv1alpha1.RolloutStrategy{
						Replace: &appsv1alpha1.RolloutStrategyReplace{},
					},
				},
			},
		},
	}

	err := (&rolloutSetupTransformer{}).precheck(cluster, rollout)
	if err == nil || !strings.Contains(err.Error(), "serviceVersion and compDef must be defined for component mysql") {
		t.Fatalf("expected compDef-only validation error, got %v", err)
	}
}

func TestRolloutSetupPrecheckAllowsServiceVersionWithCompDefForUserDefinedComponent(t *testing.T) {
	cluster := userDefinedComponentCluster()
	rollout := &appsv1alpha1.Rollout{
		Spec: appsv1alpha1.RolloutSpec{
			Components: []appsv1alpha1.RolloutComponent{
				{
					Name:           "mysql",
					ServiceVersion: ptr.To("8.0.37"),
					CompDef:        ptr.To("mysql-8.0-1.0.5"),
					Strategy: appsv1alpha1.RolloutStrategy{
						Replace: &appsv1alpha1.RolloutStrategyReplace{},
					},
				},
			},
		},
	}

	if err := (&rolloutSetupTransformer{}).precheck(cluster, rollout); err != nil {
		t.Fatalf("expected precheck to allow explicit serviceVersion and compDef, got %v", err)
	}
}

func TestRolloutSetupPrecheckAllowsServiceVersionOnlyForTopologyComponent(t *testing.T) {
	cluster := &appsv1.Cluster{
		Spec: appsv1.ClusterSpec{
			ClusterDef: "mysql",
			Topology:   "semisync",
		},
	}
	rollout := &appsv1alpha1.Rollout{
		Spec: appsv1alpha1.RolloutSpec{
			Components: []appsv1alpha1.RolloutComponent{
				{
					Name:           "mysql",
					ServiceVersion: ptr.To("8.0.37"),
					Strategy: appsv1alpha1.RolloutStrategy{
						Replace: &appsv1alpha1.RolloutStrategyReplace{},
					},
				},
			},
		},
	}

	if err := (&rolloutSetupTransformer{}).precheck(cluster, rollout); err != nil {
		t.Fatalf("expected topology-based cluster to allow serviceVersion-only rollout, got %v", err)
	}
}

func TestRolloutSetupPrecheckRejectsServiceVersionOnlyForUserDefinedSharding(t *testing.T) {
	cluster := userDefinedShardingCluster()
	rollout := &appsv1alpha1.Rollout{
		Spec: appsv1alpha1.RolloutSpec{
			Shardings: []appsv1alpha1.RolloutSharding{
				{
					Name:           "mysql",
					ServiceVersion: ptr.To("8.0.37"),
					Strategy: appsv1alpha1.RolloutStrategy{
						Replace: &appsv1alpha1.RolloutStrategyReplace{},
					},
				},
			},
		},
	}

	err := (&rolloutSetupTransformer{}).precheck(cluster, rollout)
	if err == nil || !strings.Contains(err.Error(), "serviceVersion and compDef must be defined for sharding mysql") {
		t.Fatalf("expected serviceVersion-only validation error, got %v", err)
	}
}

func TestRolloutSetupPrecheckRejectsCompDefOnlyForUserDefinedSharding(t *testing.T) {
	cluster := userDefinedShardingCluster()
	rollout := &appsv1alpha1.Rollout{
		Spec: appsv1alpha1.RolloutSpec{
			Shardings: []appsv1alpha1.RolloutSharding{
				{
					Name:    "mysql",
					CompDef: ptr.To("mysql-8.0-1.0.5"),
					Strategy: appsv1alpha1.RolloutStrategy{
						Replace: &appsv1alpha1.RolloutStrategyReplace{},
					},
				},
			},
		},
	}

	err := (&rolloutSetupTransformer{}).precheck(cluster, rollout)
	if err == nil || !strings.Contains(err.Error(), "serviceVersion and compDef must be defined for sharding mysql") {
		t.Fatalf("expected compDef-only validation error, got %v", err)
	}
}

func TestRolloutSetupPrecheckAllowsServiceVersionWithCompDefForUserDefinedSharding(t *testing.T) {
	cluster := userDefinedShardingCluster()
	rollout := &appsv1alpha1.Rollout{
		Spec: appsv1alpha1.RolloutSpec{
			Shardings: []appsv1alpha1.RolloutSharding{
				{
					Name:           "mysql",
					ServiceVersion: ptr.To("8.0.37"),
					CompDef:        ptr.To("mysql-8.0-1.0.5"),
					Strategy: appsv1alpha1.RolloutStrategy{
						Replace: &appsv1alpha1.RolloutStrategyReplace{},
					},
				},
			},
		},
	}

	if err := (&rolloutSetupTransformer{}).precheck(cluster, rollout); err != nil {
		t.Fatalf("expected precheck to allow explicit serviceVersion and compDef, got %v", err)
	}
}
