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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestUnavailableError_Error(t *testing.T) {
	err := &unavailableError{
		context: "ctx-1",
		call:    "Get",
		obj:     "test-cm@ConfigMap",
	}
	msg := err.Error()
	assert.Contains(t, msg, "ctx-1")
	assert.Contains(t, msg, "Get")
	assert.Contains(t, msg, "test-cm@ConfigMap")
	assert.Contains(t, msg, "unavailable")
}

func TestIsUnavailableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"unavailable error", &unavailableError{context: "ctx", call: "Get", obj: "cm"}, true},
		{"generic error", errors.New("some error"), false},
		{"nil error", nil, false},
		{"wrapped unavailable error", fmt.Errorf("wrapped: %w", &unavailableError{context: "ctx"}), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isUnavailableError(tt.err))
		})
	}
}

func TestIgnoreUnavailableError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantNil bool
	}{
		{"unavailable error returns nil", &unavailableError{context: "ctx"}, true},
		{"generic error returns original", errors.New("some error"), false},
		{"nil returns nil", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ignoreUnavailableError(tt.err)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.err, result)
			}
		})
	}
}

func TestGenericUnavailableError(t *testing.T) {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "default"}}
	err := genericUnavailableError("ctx-1", cm)
	assert.NotNil(t, err)
	uErr, ok := err.(*unavailableError)
	assert.True(t, ok, "expected *unavailableError")
	assert.Equal(t, "ctx-1", uErr.context)
	assert.Equal(t, "Generic", uErr.call)
	assert.Contains(t, uErr.obj, "ConfigMap")
}

func TestGetUnavailableError(t *testing.T) {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"}}
	err := getUnavailableError("ctx-2", cm)
	assert.NotNil(t, err)
	uErr, ok := err.(*unavailableError)
	assert.True(t, ok)
	assert.Equal(t, "ctx-2", uErr.context)
	assert.Equal(t, "Get", uErr.call)
	assert.Contains(t, uErr.obj, "my-cm")
	assert.Contains(t, uErr.obj, "ConfigMap")
}

func TestListUnavailableError(t *testing.T) {
	cmList := &corev1.ConfigMapList{}
	err := listUnavailableError("ctx-3", cmList)
	assert.NotNil(t, err)
	uErr, ok := err.(*unavailableError)
	assert.True(t, ok)
	assert.Equal(t, "ctx-3", uErr.context)
	assert.Equal(t, "List", uErr.call)
	assert.Contains(t, uErr.obj, "ConfigMapList")
}

func TestObjectNameKind(t *testing.T) {
	tests := []struct {
		name     string
		obj      runtime.Object
		objName  string
		wantKind string
	}{
		{"ConfigMap with name", &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1"}}, "cm1", "ConfigMap"},
		{"ConfigMap with empty name", &corev1.ConfigMap{}, "", "ConfigMap"},
		{"ConfigMapList", &corev1.ConfigMapList{}, "", "ConfigMapList"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := objectNameKind(tt.obj, tt.objName)
			assert.Contains(t, result, tt.wantKind)
			if tt.objName != "" {
				assert.Contains(t, result, tt.objName)
			}
		})
	}
}
