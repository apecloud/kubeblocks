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

package parameters

import (
	"testing"

	"k8s.io/utils/pointer"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

func TestValidateComponentParameterAssignments(t *testing.T) {
	paramsDef := testparameters.NewParametersDefinitionFactory("valkey-params").
		SetTemplateName("valkey-config").
		SetConfigFile("valkey.conf").
		SetFileFormatConfig(parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.RedisCfg}).
		Schema(`
parameter: {
  "maxmemory-samples"?: int & >=1 & <=10
  "maxmemory-policy"?: string & "noeviction" | "allkeys-lru" | "volatile-lru"
}`).
		GetObject()

	tests := []struct {
		name        string
		assignments parametersv1alpha1.ComponentParameters
		wantErr     bool
	}{
		{
			name: "valid integer and enum",
			assignments: parametersv1alpha1.ComponentParameters{
				"maxmemory-samples": pointer.String("5"),
				"maxmemory-policy":  pointer.String("volatile-lru"),
			},
		},
		{
			name: "reject integer below minimum",
			assignments: parametersv1alpha1.ComponentParameters{
				"maxmemory-samples": pointer.String("0"),
			},
			wantErr: true,
		},
		{
			name: "reject integer above maximum",
			assignments: parametersv1alpha1.ComponentParameters{
				"maxmemory-samples": pointer.String("11"),
			},
			wantErr: true,
		},
		{
			name: "reject integer parse error",
			assignments: parametersv1alpha1.ComponentParameters{
				"maxmemory-samples": pointer.String("not-an-int"),
			},
			wantErr: true,
		},
		{
			name: "reject enum miss",
			assignments: parametersv1alpha1.ComponentParameters{
				"maxmemory-policy": pointer.String("definitely-not-a-policy"),
			},
			wantErr: true,
		},
		{
			name: "reject unknown parameter when schema exists",
			assignments: parametersv1alpha1.ComponentParameters{
				"not-a-real-parameter": pointer.String("1"),
			},
			wantErr: true,
		},
		{
			name: "allow deletion for known parameter",
			assignments: parametersv1alpha1.ComponentParameters{
				"maxmemory-samples": nil,
			},
		},
		{
			name: "reject deletion for unknown parameter",
			assignments: parametersv1alpha1.ComponentParameters{
				"not-a-real-parameter": nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateComponentParameterAssignments(tt.assignments, []*parametersv1alpha1.ParametersDefinition{paramsDef})
			if tt.wantErr && err == nil {
				t.Fatalf("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no validation error, got %v", err)
			}
		})
	}
}
