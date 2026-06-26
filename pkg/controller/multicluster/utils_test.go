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

package multicluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestEnabled4Object(t *testing.T) {
	tests := []struct {
		name string
		obj  client.Object
		want bool
	}{
		{"nil annotations", newConfigMap("default", "cm1", nil), false},
		{"empty annotations", newConfigMap("default", "cm2", map[string]string{}), false},
		{"unrelated annotations", newConfigMap("default", "cm3", map[string]string{"foo": "bar"}), false},
		{"with placement annotation", newConfigMap("default", "cm4", annotationWithPlacement("ctx-1")), true},
		{"with multiple placement", newConfigMap("default", "cm5", annotationWithPlacement("ctx-1,ctx-2")), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Enabled4Object(tt.obj)
			assert.Equal(t, tt.want, result)
		})
	}
}
