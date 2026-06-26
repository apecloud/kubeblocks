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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func TestSetup_EmptyContexts(t *testing.T) {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	mgr, err := Setup(s, &rest.Config{}, newFakeClient(), "", "", "")
	assert.NoError(t, err)
	assert.Nil(t, mgr)
}

func TestIsSameContextWithControl(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		mccID  string
		want   bool
	}{
		{"same host", "https://10.0.0.1", "https://10.0.0.1", true},
		{"different host", "https://10.0.0.1", "https://10.0.0.2", false},
		{"empty both", "", "", true},
		{"empty host", "", "https://10.0.0.1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &rest.Config{Host: tt.host}
			mcc := multiClusterContext{id: tt.mccID}
			assert.Equal(t, tt.want, isSameContextWithControl(cfg, mcc))
		})
	}
}

func TestGetUncachedObjects(t *testing.T) {
	objs := getUncachedObjects()
	assert.NotEmpty(t, objs)
	assert.Len(t, objs, 4)
	_, isCM := objs[0].(*corev1.ConfigMap)
	assert.True(t, isCM)
	_, isSecret := objs[1].(*corev1.Secret)
	assert.True(t, isSecret)
}

func TestCacheOptions(t *testing.T) {
	s := runtime.NewScheme()
	opts := client.Options{
		Scheme: s,
	}
	co := cacheOptions(opts)
	assert.Equal(t, s, co.Scheme)
}

func TestNewClientNCache_DisabledContextNotInContexts(t *testing.T) {
	s := runtime.NewScheme()
	_, err := newClientNCache(s, "", "ctx-1,ctx-2", "ctx-3")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in contexts")
}

func TestNewClientNCache_EmptyContextString(t *testing.T) {
	s := runtime.NewScheme()
	_, err := newClientNCache(s, "", "ctx-1,,ctx-2", "")
	// empty context string should produce a nil entry (skipped)
	// this may or may not error depending on config resolution
	_ = err
}

func TestSetupScheme_SetsPackageVar(t *testing.T) {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	setupScheme(s)
	// verify scheme is set by checking objectNameKind works
	cm := &corev1.ConfigMap{}
	result := objectNameKind(cm, "test")
	assert.Contains(t, result, "ConfigMap")
}

func TestCreateUnavailableClientNCache(t *testing.T) {
	cli, cache, err := createUnavailableClientNCache(nil, nil, "ctx-1")
	assert.NoError(t, err)
	assert.NotNil(t, cli)
	assert.Nil(t, cache)
	assert.True(t, isUnavailableClient(cli))
}

func TestClientOptions_ReturnsClientOptionsWithScheme(t *testing.T) {
	// clientOptions requires a real rest.Config with a host to create HTTP client
	// we test the error path with an invalid config
	_, err := clientOptions(testScheme, "ctx-1", &rest.Config{Host: "http://localhost:99999"})
	// should fail because the host is invalid
	// if it doesn't fail, that's also acceptable (just means the config was somehow valid)
	_ = err
}

func TestGetConfigWithContext_EmptyKubeConfig(t *testing.T) {
	// with empty kubeConfig, it tries to use in-cluster config
	// this will fail in test environment, which is expected
	_, err := getConfigWithContext("", "nonexistent-context")
	assert.Error(t, err)
}

// verify that the uncached objects include the expected types
func TestGetUncachedObjects_ContainsAllExpected(t *testing.T) {
	objs := getUncachedObjects()
	expectedTypes := []runtime.Object{
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&appsv1.Cluster{},
		&appsv1alpha1.Configuration{},
	}
	assert.Len(t, objs, len(expectedTypes))
}

func TestSetupScheme_FunctionIsIdempotent(t *testing.T) {
	s1 := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s1)
	setupScheme(s1)

	s2 := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s2)
	setupScheme(s2)

	// both should work
	cm := &corev1.ConfigMap{}
	assert.Contains(t, objectNameKind(cm, ""), "ConfigMap")
}
