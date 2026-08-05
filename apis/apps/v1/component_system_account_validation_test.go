/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package v1

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	celvalidator "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

func TestComponentSystemAccountSecretRefRevisionValidation(t *testing.T) {
	crdPath := filepath.Join("..", "..", "..", "config", "crd", "bases", "apps.kubeblocks.io_components.yaml")
	data, err := os.ReadFile(crdPath)
	if err != nil {
		t.Fatalf("read Component CRD: %v", err)
	}

	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatalf("decode Component CRD: %v", err)
	}
	var validation *apiextensionsv1.CustomResourceValidation
	for _, version := range crd.Spec.Versions {
		if version.Name == "v1" {
			validation = version.Schema
			break
		}
	}
	if validation == nil {
		t.Fatalf("Component CRD v1 schema is missing: %#v", crd.Spec.Versions)
	}

	root := validation.OpenAPIV3Schema
	spec := root.Properties["spec"]
	systemAccounts := spec.Properties["systemAccounts"]
	if systemAccounts.Items == nil || systemAccounts.Items.Schema == nil {
		t.Fatal("Component systemAccounts item schema is missing")
	}

	var internalSchema apiextensions.JSONSchemaProps
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(systemAccounts.Items.Schema, &internalSchema, nil); err != nil {
		t.Fatalf("convert system account schema: %v", err)
	}
	structuralSchema, err := schema.NewStructural(&internalSchema)
	if err != nil {
		t.Fatalf("build structural system account schema: %v", err)
	}
	validator := celvalidator.NewValidator(structuralSchema, false, celconfig.PerCallLimit)
	if validator == nil {
		t.Fatal("expected a CEL validator for ComponentSystemAccount")
	}

	tests := []struct {
		name       string
		account    map[string]interface{}
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "non-empty revision without secretRef",
			account: map[string]interface{}{
				"name":              "admin",
				"secretRefRevision": "revision-1",
			},
			wantErr:    true,
			wantErrMsg: "secretRef must be specified when secretRefRevision is non-empty",
		},
		{
			name: "absent revision without secretRef",
			account: map[string]interface{}{
				"name": "admin",
			},
		},
		{
			name: "explicitly empty revision without secretRef",
			account: map[string]interface{}{
				"name":              "admin",
				"secretRefRevision": "",
			},
		},
		{
			name: "non-empty revision with secretRef",
			account: map[string]interface{}{
				"name":              "admin",
				"secretRef":         map[string]interface{}{"name": "source-secret"},
				"secretRefRevision": "revision-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs, _ := validator.Validate(
				context.Background(),
				field.NewPath("systemAccount"),
				structuralSchema,
				tt.account,
				nil,
				celconfig.RuntimeCELCostBudget,
			)
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatal("expected validation to fail")
				}
				if !strings.Contains(errs.ToAggregate().Error(), tt.wantErrMsg) {
					t.Fatalf("expected validation error to contain %q, got %v", tt.wantErrMsg, errs)
				}
				return
			}
			if len(errs) != 0 {
				t.Fatalf("expected validation to succeed, got %v", errs)
			}
		})
	}
}
