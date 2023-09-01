/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestToPointerWithFailed(t *testing.T) {
	type args[T any] struct {
		v T
	}
	type testCase[T any] struct {
		name string
		args args[T]
		want *T
	}
	tests := []testCase[client.Object]{
		{
			name: "failed test",
			args: args[client.Object]{&corev1.ConfigMap{}},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Panics(t, func() {
				ToPointer(tt.args.v)
			})
		})
	}
}

func TestToPointer(t *testing.T) {
	type args[T any] struct {
		v T
	}
	type testCase[T any] struct {
		name string
		args args[T]
		want *T
	}
	tests := []testCase[string]{
		{
			name: "normal test",
			args: args[string]{v: "test"},
			want: func() *string {
				v := "test"
				return &v
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, ToPointer(tt.args.v), "ToPointer(%v)", tt.args.v)
		})
	}
}
