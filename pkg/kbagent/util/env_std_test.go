/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- EnvM2L ---

func TestEnvM2L_Nil(t *testing.T) {
	result := EnvM2L(nil)
	assert.Empty(t, result)
}

func TestEnvM2L_Empty(t *testing.T) {
	result := EnvM2L(map[string]string{})
	assert.Empty(t, result)
}

func TestEnvM2L_SingleEntry(t *testing.T) {
	result := EnvM2L(map[string]string{"KEY": "VALUE"})
	assert.Equal(t, []string{"KEY=VALUE"}, result)
}

func TestEnvM2L_MultipleEntries(t *testing.T) {
	m := map[string]string{"A": "1", "B": "2", "C": "3"}
	result := EnvM2L(m)
	sort.Strings(result)
	assert.Equal(t, []string{"A=1", "B=2", "C=3"}, result)
}

func TestEnvM2L_EmptyValue(t *testing.T) {
	result := EnvM2L(map[string]string{"KEY": ""})
	assert.Equal(t, []string{"KEY="}, result)
}

// --- EnvL2M ---

func TestEnvL2M_Nil(t *testing.T) {
	result := EnvL2M(nil)
	assert.Empty(t, result)
}

func TestEnvL2M_Empty(t *testing.T) {
	result := EnvL2M([]string{})
	assert.Empty(t, result)
}

func TestEnvL2M_SingleEntry(t *testing.T) {
	result := EnvL2M([]string{"KEY=VALUE"})
	assert.Equal(t, map[string]string{"KEY": "VALUE"}, result)
}

func TestEnvL2M_MultipleEntries(t *testing.T) {
	result := EnvL2M([]string{"A=1", "B=2", "C=3"})
	assert.Equal(t, map[string]string{"A": "1", "B": "2", "C": "3"}, result)
}

func TestEnvL2M_EmptyValue(t *testing.T) {
	result := EnvL2M([]string{"KEY="})
	assert.Equal(t, map[string]string{"KEY": ""}, result)
}

func TestEnvL2M_NoEquals(t *testing.T) {
	result := EnvL2M([]string{"KEYONLY"})
	assert.Equal(t, map[string]string{"KEYONLY": ""}, result)
}

func TestEnvL2M_ValueWithEquals(t *testing.T) {
	result := EnvL2M([]string{"KEY=a=b=c"})
	assert.Equal(t, map[string]string{"KEY": "a=b=c"}, result)
}

// --- DefaultEnvVars ---

func TestDefaultEnvVars_Length(t *testing.T) {
	vars := DefaultEnvVars()
	assert.Len(t, vars, 4)
}

func TestDefaultEnvVars_Names(t *testing.T) {
	vars := DefaultEnvVars()
	names := make([]string, len(vars))
	for i, v := range vars {
		names[i] = v.Name
	}
	assert.Contains(t, names, kbEnvNamespace)
	assert.Contains(t, names, kbEnvPodName)
	assert.Contains(t, names, kbEnvPodUID)
	assert.Contains(t, names, kbEnvNodeName)
}

func TestDefaultEnvVars_FieldRefs(t *testing.T) {
	vars := DefaultEnvVars()
	for _, v := range vars {
		require.NotNil(t, v.ValueFrom)
		require.NotNil(t, v.ValueFrom.FieldRef)
		assert.Equal(t, "v1", v.ValueFrom.FieldRef.APIVersion)
		assert.NotEmpty(t, v.ValueFrom.FieldRef.FieldPath)
	}
}

func TestDefaultEnvVars_NamespaceFieldPath(t *testing.T) {
	vars := DefaultEnvVars()
	for _, v := range vars {
		if v.Name == kbEnvNamespace {
			assert.Equal(t, "metadata.namespace", v.ValueFrom.FieldRef.FieldPath)
			return
		}
	}
	t.Fatal("namespace env var not found")
}

func TestDefaultEnvVars_PodNameFieldPath(t *testing.T) {
	vars := DefaultEnvVars()
	for _, v := range vars {
		if v.Name == kbEnvPodName {
			assert.Equal(t, "metadata.name", v.ValueFrom.FieldRef.FieldPath)
			return
		}
	}
	t.Fatal("pod name env var not found")
}

func TestDefaultEnvVars_PodUIDFieldPath(t *testing.T) {
	vars := DefaultEnvVars()
	for _, v := range vars {
		if v.Name == kbEnvPodUID {
			assert.Equal(t, "metadata.uid", v.ValueFrom.FieldRef.FieldPath)
			return
		}
	}
	t.Fatal("pod uid env var not found")
}

func TestDefaultEnvVars_NodeNameFieldPath(t *testing.T) {
	vars := DefaultEnvVars()
	for _, v := range vars {
		if v.Name == kbEnvNodeName {
			assert.Equal(t, "spec.nodeName", v.ValueFrom.FieldRef.FieldPath)
			return
		}
	}
	t.Fatal("node name env var not found")
}

// --- PodName / podName ---

func TestPodName_FromEnv(t *testing.T) {
	os.Setenv(kbEnvPodName, "my-pod-0")
	defer os.Unsetenv(kbEnvPodName)
	assert.Equal(t, "my-pod-0", PodName())
}

func TestPodName_Empty(t *testing.T) {
	os.Unsetenv(kbEnvPodName)
	assert.Empty(t, PodName())
}

// --- namespace ---

func TestNamespace_FromEnv(t *testing.T) {
	os.Setenv(kbEnvNamespace, "test-ns")
	defer os.Unsetenv(kbEnvNamespace)
	assert.Equal(t, "test-ns", namespace())
}

func TestNamespace_Empty(t *testing.T) {
	os.Unsetenv(kbEnvNamespace)
	assert.Empty(t, namespace())
}

// --- podUID ---

func TestPodUID_FromEnv(t *testing.T) {
	os.Setenv(kbEnvPodUID, "abc-123-uid")
	defer os.Unsetenv(kbEnvPodUID)
	assert.Equal(t, "abc-123-uid", podUID())
}

func TestPodUID_Empty(t *testing.T) {
	os.Unsetenv(kbEnvPodUID)
	assert.Empty(t, podUID())
}

// --- nodeName ---

func TestNodeName_FromEnv(t *testing.T) {
	os.Setenv(kbEnvNodeName, "node-1")
	defer os.Unsetenv(kbEnvNodeName)
	assert.Equal(t, "node-1", nodeName())
}

func TestNodeName_Empty(t *testing.T) {
	os.Unsetenv(kbEnvNodeName)
	assert.Empty(t, nodeName())
}

// --- roundtrip: EnvM2L → EnvL2M ---

func TestEnvRoundtrip(t *testing.T) {
	original := map[string]string{"FOO": "bar", "BAZ": "qux"}
	list := EnvM2L(original)
	back := EnvL2M(list)
	assert.Equal(t, original, back)
}
