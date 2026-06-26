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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNewUnavailableClient(t *testing.T) {
	c := newUnavailableClient("ctx-1")
	assert.NotNil(t, c)
}

func TestIsUnavailableClient(t *testing.T) {
	tests := []struct {
		name string
		cli  client.Client
		want bool
	}{
		{"unavailable client", newUnavailableClient("ctx-1"), true},
		{"fake client", newFakeClient(), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isUnavailableClient(tt.cli))
		})
	}
}

func TestUnavailableClient_Scheme(t *testing.T) {
	c := newUnavailableClient("ctx-1")
	assert.Nil(t, c.Scheme())
}

func TestUnavailableClient_RESTMapper(t *testing.T) {
	c := newUnavailableClient("ctx-1")
	assert.Nil(t, c.RESTMapper())
}

func TestUnavailableClient_GroupVersionKindFor(t *testing.T) {
	c := newUnavailableClient("ctx-1")
	cm := &corev1.ConfigMap{}
	gvk, err := c.GroupVersionKindFor(cm)
	assert.Error(t, err)
	assert.Equal(t, schema.GroupVersionKind{}, gvk)
}

func TestUnavailableClient_IsObjectNamespaced(t *testing.T) {
	c := newUnavailableClient("ctx-1")
	cm := &corev1.ConfigMap{}
	namespaced, err := c.IsObjectNamespaced(cm)
	assert.Error(t, err)
	assert.False(t, namespaced)
}

func TestUnavailableClientReader_Get(t *testing.T) {
	c := newUnavailableClient("ctx-1")
	cm := &corev1.ConfigMap{}
	err := c.Get(context.Background(), types.NamespacedName{Name: "test"}, cm)
	assert.Error(t, err)
	assert.True(t, isUnavailableError(err))
}

func TestUnavailableClientReader_List(t *testing.T) {
	c := newUnavailableClient("ctx-1")
	list := &corev1.ConfigMapList{}
	err := c.List(context.Background(), list)
	assert.Error(t, err)
	assert.True(t, isUnavailableError(err))
}

func TestUnavailableClientWriter_AllMethodsReturnNil(t *testing.T) {
	c := newUnavailableClient("ctx-1")
	cm := &corev1.ConfigMap{}

	assert.NoError(t, c.Create(context.Background(), cm))
	assert.NoError(t, c.Delete(context.Background(), cm))
	assert.NoError(t, c.Update(context.Background(), cm))
	assert.NoError(t, c.Patch(context.Background(), cm, client.MergeFrom(cm.DeepCopy())))
	assert.NoError(t, c.DeleteAllOf(context.Background(), cm))
}

func TestUnavailableStatusClient_Status(t *testing.T) {
	c := newUnavailableClient("ctx-1")
	sw := c.Status()
	assert.NotNil(t, sw)
}

func TestUnavailableSubResourceClientConstructor_SubResource(t *testing.T) {
	c := newUnavailableClient("ctx-1")
	src := c.SubResource("status")
	assert.NotNil(t, src)
}

func TestUnavailableSubResourceReader_Get(t *testing.T) {
	c := newUnavailableClient("ctx-1")
	cm := &corev1.ConfigMap{}
	subObj := &corev1.ConfigMap{}
	err := c.SubResource("status").Get(context.Background(), cm, subObj)
	assert.Error(t, err)
	assert.True(t, isUnavailableError(err))
}

func TestUnavailableSubResourceWriter_AllMethodsReturnNil(t *testing.T) {
	c := newUnavailableClient("ctx-1")
	cm := &corev1.ConfigMap{}
	subObj := &corev1.ConfigMap{}

	sw := c.Status()
	assert.NoError(t, sw.Create(context.Background(), cm, subObj))
	assert.NoError(t, sw.Update(context.Background(), cm))
	assert.NoError(t, sw.Patch(context.Background(), cm, client.MergeFrom(cm.DeepCopy())))
}

func TestUnavailableClient_ImplementsClientInterface(t *testing.T) {
	var _ client.Client = &unavailableClient{}
	var _ client.Reader = &unavailableClientReader{}
	var _ client.Writer = &unavailableClientWriter{}
	var _ client.StatusClient = &unavailableStatusClient{}
	var _ client.SubResourceClientConstructor = &unavailableSubResourceClientConstructor{}
	var _ client.SubResourceClient = &unavailableSubResourceClient{}
	var _ client.SubResourceReader = &unavailableSubResourceReader{}
	var _ client.SubResourceWriter = &unavailableSubResourceWriter{}
}

func TestUnavailableClient_ErrorContext(t *testing.T) {
	c := newUnavailableClient("my-context")
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"}}
	err := c.Get(context.Background(), types.NamespacedName{Name: "my-cm"}, cm)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "my-context")
}

// reference to keep runtime import used
var _ runtime.Object = &corev1.ConfigMap{}
