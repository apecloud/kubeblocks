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

package instance

import (
	"hash/fnv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func TestControllerRevisionName(t *testing.T) {
	assert.Equal(t, "my-inst-abc123", controllerRevisionName("my-inst", "abc123"))

	// long prefix gets truncated
	longName := make([]byte, 250)
	for i := range longName {
		longName[i] = 'a'
	}
	result := controllerRevisionName(string(longName), "hash")
	assert.True(t, len(result) <= 253)
}

func TestGetPodRevision(t *testing.T) {
	t.Run("no labels", func(t *testing.T) {
		pod := &corev1.Pod{}
		assert.Equal(t, "", getPodRevision(pod))
	})

	t.Run("with revision label", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					appsv1.ControllerRevisionHashLabelKey: "rev-123",
				},
			},
		}
		assert.Equal(t, "rev-123", getPodRevision(pod))
	})
}

func TestHashControllerRevision(t *testing.T) {
	cr := &appsv1.ControllerRevision{
		Data: runtime.RawExtension{Raw: []byte(`{"spec":{"template":{"$patch":"replace"}}}`)},
	}
	collision := int32(0)
	hash := hashControllerRevision(cr, &collision)
	assert.NotEmpty(t, hash)

	// same data should produce same hash
	hash2 := hashControllerRevision(cr, &collision)
	assert.Equal(t, hash, hash2)

	// different collision count should produce different hash
	collision2 := int32(1)
	hash3 := hashControllerRevision(cr, &collision2)
	assert.NotEqual(t, hash, hash3)
}

func TestNewRevision(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-inst",
			Namespace: "default",
			UID:       types.UID("test-uid"),
			Annotations: map[string]string{
				"key": "value",
			},
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "main", Image: "nginx:1.19"},
					},
				},
			},
		},
	}

	cr, err := newRevision(inst)
	require.NoError(t, err)
	assert.NotNil(t, cr)
	assert.Contains(t, cr.Name, "test-inst")
	assert.Equal(t, "value", cr.Annotations["key"])
	assert.Equal(t, "test", cr.Labels["app"])
	assert.NotEmpty(t, cr.Labels[controllerRevisionHashLabel])
}

func TestBuildInstancePodRevision(t *testing.T) {
	template := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"app": "test"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main", Image: "nginx:1.19"},
			},
		},
	}
	parent := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-inst",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}

	rev, err := buildInstancePodRevision(template, parent)
	require.NoError(t, err)
	assert.NotEmpty(t, rev)

	// same template should produce same revision
	rev2, err := buildInstancePodRevision(template, parent)
	require.NoError(t, err)
	assert.Equal(t, rev, rev2)
}

func TestDeepHashObject(t *testing.T) {
	// just verify it doesn't panic
	h := fnv.New32()
	deepHashObject(h, map[string]string{"key": "value"})
	assert.NotZero(t, h.Sum32())
}
