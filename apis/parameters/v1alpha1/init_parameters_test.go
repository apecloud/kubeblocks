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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func TestInitParametersRoundTrip(t *testing.T) {
	value := InitParameters{
		"mysql": {
			Parameters: ParameterValueMap{
				"max_connections": strPtr("200"),
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

	raw, err := EncodeInitParameters(value)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	decoded, err := DecodeInitParameters(raw)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	spec := decoded.Get("mysql")
	if spec == nil {
		t.Fatalf("expected mysql spec")
	}
	if spec.Parameters["max_connections"] == nil || *spec.Parameters["max_connections"] != "200" {
		t.Fatalf("unexpected parameter value: %#v", spec.Parameters["max_connections"])
	}
	tpl := spec.Templates["mysql-config"]
	if tpl.TemplateRef != "custom-template" || tpl.Namespace != "default" || tpl.Policy != ReplacePolicy {
		t.Fatalf("unexpected template extension: %#v", tpl)
	}
}

func TestParseInitParameters(t *testing.T) {
	cluster := &appsv1.Cluster{}
	if err := SetInitParameters(cluster, InitParameters{
		"mysql": {
			Parameters: ParameterValueMap{
				"max_connections": strPtr("200"),
			},
		},
	}); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	decoded, err := ParseInitParameters(cluster)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	spec := decoded.Get("mysql")
	if spec == nil || spec.Parameters["max_connections"] == nil || *spec.Parameters["max_connections"] != "200" {
		t.Fatalf("unexpected decoded payload: %#v", spec)
	}
}

func TestSetInitParameters(t *testing.T) {
	cluster := &appsv1.Cluster{}
	params := InitParameters{
		"mysql": {
			Parameters: ParameterValueMap{
				"max_connections": strPtr("200"),
			},
		},
	}
	if err := SetInitParameters(cluster, params); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if cluster.Annotations == nil {
		t.Fatalf("expected init parameter annotation to be set")
	}
	decoded, err := ParseInitParameters(cluster)
	if err != nil {
		t.Fatalf("parse after set failed: %v", err)
	}
	if decoded.Get("mysql") == nil {
		t.Fatalf("expected mysql init parameter after set")
	}
	if err := SetInitParameters(cluster, InitParameters{}); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if cluster.Annotations != nil {
		if len(cluster.Annotations) != 0 {
			t.Fatalf("expected init parameter annotation to be cleared")
		}
	}
}

func strPtr(v string) *string {
	return &v
}
