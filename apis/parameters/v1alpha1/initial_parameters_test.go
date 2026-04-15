/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"testing"

	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func TestInitialParametersRoundTrip(t *testing.T) {
	value := InitialParameters{
		"mysql": {
			Assignments: map[string]*string{
				"max_connections": ptr.To("200"),
			},
			Templates: map[string]ConfigTemplateExtension{
				"mysql-config": {
					TemplateRef: "custom-template",
					Namespace:   "default",
					Policy:      ReplacePolicy,
				},
			},
		},
	}

	raw, err := EncodeInitialParameters(value)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	decoded, err := DecodeInitialParameters(raw)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	spec := decoded.Get("mysql")
	if spec == nil {
		t.Fatalf("expected mysql spec")
	}
	if spec.Assignments["max_connections"] == nil || *spec.Assignments["max_connections"] != "200" {
		t.Fatalf("unexpected parameter value: %#v", spec.Assignments["max_connections"])
	}
	tpl := spec.Templates["mysql-config"]
	if tpl.TemplateRef != "custom-template" || tpl.Namespace != "default" || tpl.Policy != ReplacePolicy {
		t.Fatalf("unexpected template extension: %#v", tpl)
	}
}

func TestParseInitialParameters(t *testing.T) {
	cluster := &appsv1.Cluster{}
	if err := SetInitialParameters(cluster, InitialParameters{
		"mysql": {
			Assignments: map[string]*string{
				"max_connections": ptr.To("200"),
			},
		},
	}); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	decoded, err := ParseInitialParameters(cluster)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	spec := decoded.Get("mysql")
	if spec == nil || spec.Assignments["max_connections"] == nil || *spec.Assignments["max_connections"] != "200" {
		t.Fatalf("unexpected decoded payload: %#v", spec)
	}
}

func TestSetInitialParameters(t *testing.T) {
	cluster := &appsv1.Cluster{}
	params := InitialParameters{
		"mysql": {
			Assignments: map[string]*string{
				"max_connections": ptr.To("200"),
			},
		},
	}
	if err := SetInitialParameters(cluster, params); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if cluster.Annotations == nil {
		t.Fatalf("expected init parameter annotation to be set")
	}
	decoded, err := ParseInitialParameters(cluster)
	if err != nil {
		t.Fatalf("parse after set failed: %v", err)
	}
	if decoded.Get("mysql") == nil {
		t.Fatalf("expected mysql init parameter after set")
	}
	if err := SetInitialParameters(cluster, InitialParameters{}); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if cluster.Annotations != nil {
		if len(cluster.Annotations) != 0 {
			t.Fatalf("expected init parameter annotation to be cleared")
		}
	}
}
