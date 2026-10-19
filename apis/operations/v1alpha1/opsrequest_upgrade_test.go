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

	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func TestValidateUpgradeInstanceServiceVersion(t *testing.T) {
	for _, tc := range []struct {
		name      string
		instances []UpgradeInstanceTemplate
		wantError bool
	}{
		{name: "existing component two-part version remains accepted"},
		{name: "omitted instance version", instances: []UpgradeInstanceTemplate{{Name: "canary", ComponentDefinitionName: ptr.To("postgresql")}}},
		{name: "empty instance version", instances: []UpgradeInstanceTemplate{{Name: "canary", ServiceVersion: ptr.To("")}}},
		{name: "three-part instance version", instances: []UpgradeInstanceTemplate{{Name: "canary", ServiceVersion: ptr.To("17.5.0")}}},
		{name: "prefixed instance version", instances: []UpgradeInstanceTemplate{{Name: "canary", ServiceVersion: ptr.To("v17.5.0")}}},
		{name: "prerelease instance version", instances: []UpgradeInstanceTemplate{{Name: "canary", ServiceVersion: ptr.To("17.5.0-rc.1")}}},
		{name: "build instance version", instances: []UpgradeInstanceTemplate{{Name: "canary", ServiceVersion: ptr.To("17.5.0+build.1")}}},
		{name: "two-part instance version rejected", instances: []UpgradeInstanceTemplate{{Name: "canary", ServiceVersion: ptr.To("17.5")}}, wantError: true},
		{name: "arbitrary instance version rejected", instances: []UpgradeInstanceTemplate{{Name: "canary", ServiceVersion: ptr.To("latest")}}, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ops := &OpsRequest{Spec: OpsRequestSpec{
				Type: UpgradeType,
				SpecificOpsRequest: SpecificOpsRequest{Upgrade: &Upgrade{Components: []UpgradeComponent{{
					ComponentOps:   ComponentOps{ComponentName: "db"},
					ServiceVersion: ptr.To("17.5"),
					Instances:      tc.instances,
				}}}},
			}}
			err := ops.ValidateOps(context.Background(), nil, &appsv1.Cluster{})
			if (err != nil) != tc.wantError {
				t.Fatalf("ValidateOps() error = %v, want error %v", err, tc.wantError)
			}
			if err != nil && !strings.Contains(err.Error(), `instance template "canary" serviceVersion`) {
				t.Fatalf("error does not identify the instance service version: %v", err)
			}
		})
	}
}

func TestUpgradeInstanceTemplateFieldPresence(t *testing.T) {
	template := UpgradeInstanceTemplate{Name: "canary", ServiceVersion: ptr.To("")}
	data, err := json.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "componentDefinitionName") || !strings.Contains(string(data), `"serviceVersion":""`) {
		t.Fatalf("field presence was not preserved: %s", data)
	}
	var decoded UpgradeInstanceTemplate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ComponentDefinitionName != nil || decoded.ServiceVersion == nil || *decoded.ServiceVersion != "" {
		t.Fatalf("decoded field presence was not preserved: %+v", decoded)
	}
	component := UpgradeComponent{Instances: []UpgradeInstanceTemplate{template}}
	copy := component.DeepCopy()
	*copy.Instances[0].ServiceVersion = "17.5.0"
	if *component.Instances[0].ServiceVersion != "" {
		t.Fatal("deep copy shares the instance version pointer")
	}
}
