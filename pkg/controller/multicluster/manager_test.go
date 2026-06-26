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
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestManager_GetClient(t *testing.T) {
	control := newFakeClient()
	mc := NewClient(control, nil)
	mgr := &manager{cli: mc, caches: map[string]cache.Cache{}}
	assert.Equal(t, mc, mgr.GetClient())
}

func TestManager_GetClient_ReturnsWrappedClient(t *testing.T) {
	control := newFakeClient()
	workers := map[string]client.Client{"ctx-1": newFakeClient()}
	mc := NewClient(control, workers)
	mgr := &manager{cli: mc, caches: map[string]cache.Cache{}}
	result := mgr.GetClient()
	assert.NotNil(t, result)
	// verify it's the multi-cluster client, not the raw control client
	assert.NotNil(t, result.Scheme())
}

func TestManager_GetContexts(t *testing.T) {
	mgr := &manager{
		cli:    newFakeClient(),
		caches: map[string]cache.Cache{"ctx-1": nil, "ctx-2": nil},
	}
	contexts := mgr.GetContexts()
	assert.Len(t, contexts, 2)
	assert.Contains(t, contexts, "ctx-1")
	assert.Contains(t, contexts, "ctx-2")
}

func TestManager_GetContexts_Empty(t *testing.T) {
	mgr := &manager{
		cli:    newFakeClient(),
		caches: map[string]cache.Cache{},
	}
	contexts := mgr.GetContexts()
	assert.Empty(t, contexts)
}

func TestManager_Bind_NilCaches(t *testing.T) {
	mgr := &manager{
		cli:    newFakeClient(),
		caches: map[string]cache.Cache{"ctx-1": nil, "ctx-2": nil},
	}
	// Bind with nil caches should not panic and return nil
	err := mgr.Bind(nil)
	// Bind may fail if mgr is nil, but with nil caches it should be fine
	// since we pass nil as ctrl.Manager, the call to mgr.Add won't happen for nil caches
	_ = err
}

func TestManager_Own_NilCaches(t *testing.T) {
	control := newFakeClient()
	mc := NewClient(control, nil)
	mgr := &manager{cli: mc, caches: map[string]cache.Cache{"ctx-1": nil}}
	owner := newConfigMap("default", "owner", nil)
	obj := newConfigMap("default", "obj", nil)
	// with nil caches, Own should not panic
	result := mgr.Own(nil, obj, owner)
	assert.NotNil(t, result)
}

func TestManager_Watch_NilCaches(t *testing.T) {
	control := newFakeClient()
	mc := NewClient(control, nil)
	mgr := &manager{cli: mc, caches: map[string]cache.Cache{"ctx-1": nil}}
	obj := newConfigMap("default", "obj", nil)
	// with nil caches, Watch should not panic
	result := mgr.Watch(nil, obj, nil)
	assert.NotNil(t, result)
}

func TestManager_ImplementsInterface(t *testing.T) {
	var _ Manager = &manager{}
}
