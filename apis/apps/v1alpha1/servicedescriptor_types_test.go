/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
