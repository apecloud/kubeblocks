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

package v1

import (
	"strings"
	"testing"
)

func TestClusterValidatesInstanceTemplateLabelValues(t *testing.T) {
	cluster := &Cluster{
		Spec: ClusterSpec{
			ComponentSpecs: []ClusterComponentSpec{{
				Name: "mysql",
				Instances: []InstanceTemplate{{
					Name:   "az-a",
					Labels: map[string]string{"test": "test-wec1, test-wec2"},
				}},
			}},
		},
	}

	err := cluster.validateInstanceTemplateLabels()
	if err == nil {
		t.Fatal("expected invalid label value to be rejected")
	}
	if !strings.Contains(err.Error(), "spec.componentSpecs[0].instances[0].labels[test]") {
		t.Fatalf("expected component instance label path in error, got %v", err)
	}
}

func TestClusterValidatesShardingInstanceTemplateLabelValues(t *testing.T) {
	cluster := &Cluster{
		Spec: ClusterSpec{
			Shardings: []ClusterSharding{{
				Name: "shard",
				Template: ClusterComponentSpec{
					Instances: []InstanceTemplate{{
						Name:   "az-a",
						Labels: map[string]string{"test": "valid"},
					}},
				},
				ShardTemplates: []ShardTemplate{{
					Name: "shard-a",
					Instances: []InstanceTemplate{{
						Name:   "az-b",
						Labels: map[string]string{"test": "test-wec1, test-wec2"},
					}},
				}},
			}},
		},
	}

	err := cluster.validateInstanceTemplateLabels()
	if err == nil {
		t.Fatal("expected invalid sharding label value to be rejected")
	}
	if !strings.Contains(err.Error(), "spec.shardings[0].shardTemplates[0].instances[0].labels[test]") {
		t.Fatalf("expected sharding instance label path in error, got %v", err)
	}
}

func TestComponentValidatesInstanceTemplateLabelKeys(t *testing.T) {
	component := &Component{
		Spec: ComponentSpec{
			Instances: []InstanceTemplate{{
				Name:   "az-a",
				Labels: map[string]string{"bad/key/again": "valid"},
			}},
		},
	}

	err := component.validateInstanceTemplateLabels()
	if err == nil {
		t.Fatal("expected invalid label key to be rejected")
	}
	if !strings.Contains(err.Error(), "spec.instances[0].labels[bad/key/again]") {
		t.Fatalf("expected component instance label path in error, got %v", err)
	}
}

func TestClusterAllowsValidInstanceTemplateLabels(t *testing.T) {
	cluster := &Cluster{
		Spec: ClusterSpec{
			ComponentSpecs: []ClusterComponentSpec{{
				Name: "mysql",
				Instances: []InstanceTemplate{{
					Name: "az-a",
					Labels: map[string]string{
						"example.com/role": "primary",
						"empty":            "",
					},
				}},
			}},
		},
	}

	if err := cluster.validateInstanceTemplateLabels(); err != nil {
		t.Fatalf("expected valid labels to pass, got %v", err)
	}
}
