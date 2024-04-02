/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

	"github.com/stretchr/testify/assert"
)

func TestServiceDescriptorStatus(t *testing.T) {
	type fields struct {
		Phase              Phase
		Message            string
		ObservedGeneration int64
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{{
		name: "available phase test",
		fields: fields{
			Phase: AvailablePhase,
		},
		want: true,
	}, {
		name: "unavailable phase test",
		fields: fields{
			Phase: UnavailablePhase,
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sd := ServiceDescriptorStatus{
				Phase:              tt.fields.Phase,
				Message:            tt.fields.Message,
				ObservedGeneration: tt.fields.ObservedGeneration,
			}
			actual := sd.GetTerminalPhases()[0] == tt.fields.Phase
			assert.Equalf(t, tt.want, actual, "GetTerminalPhases() = %v, want %v", sd.GetTerminalPhases(), tt.want)
		})
	}
}
