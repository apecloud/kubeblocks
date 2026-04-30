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

package multicluster

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestIntoContext(t *testing.T) {
	ctx := IntoContext(context.Background(), "ctx1")
	p, err := FromContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ctx1", p)
}

func TestFromContext_NoPlacement(t *testing.T) {
	_, err := FromContext(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "no placement was present", err.Error())
}

func TestAssign(t *testing.T) {
	t.Run("no context placement", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1"}}
		result := Assign(context.Background(), obj, func() int { return 0 })
		assert.Nil(t, result.GetAnnotations())
	})

	t.Run("with context placement, no annotations", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx1")
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1"}}
		result := Assign(ctx, obj, func() int { return 0 })
		assert.Equal(t, "ctx1", result.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
	})

	t.Run("with context placement, existing annotations", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx1")
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:        "cm1",
			Annotations: map[string]string{"existing": "value"},
		}}
		result := Assign(ctx, obj, func() int { return 0 })
		assert.Equal(t, "ctx1", result.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
		assert.Equal(t, "value", result.GetAnnotations()["existing"])
	})

	t.Run("already assigned", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx2")
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name: "cm1",
			Annotations: map[string]string{
				constant.KBAppMultiClusterPlacementKey: "ctx1",
			},
		}}
		result := Assign(ctx, obj, func() int { return 0 })
		// Should not override
		assert.Equal(t, "ctx1", result.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
	})

	t.Run("multiple contexts with ordinal", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx1,ctx2,ctx3")
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1"}}
		result := Assign(ctx, obj, func() int { return 1 })
		assert.Equal(t, "ctx2", result.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
	})

	t.Run("ordinal wraps around", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx1,ctx2")
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1"}}
		result := Assign(ctx, obj, func() int { return 4 })
		assert.Equal(t, "ctx1", result.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
	})
}

func TestSetPlacementKey(t *testing.T) {
	t.Run("empty context", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1"}}
		setPlacementKey(obj, "")
		assert.Nil(t, obj.GetAnnotations())
	})

	t.Run("already set", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name: "cm1",
			Annotations: map[string]string{
				constant.KBAppMultiClusterPlacementKey: "existing",
			},
		}}
		setPlacementKey(obj, "new")
		assert.Equal(t, "existing", obj.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
	})

	t.Run("no annotations", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1"}}
		setPlacementKey(obj, "ctx1")
		assert.Equal(t, "ctx1", obj.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
	})

	t.Run("existing annotations", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:        "cm1",
			Annotations: map[string]string{"other": "val"},
		}}
		setPlacementKey(obj, "ctx1")
		assert.Equal(t, "ctx1", obj.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
		assert.Equal(t, "val", obj.GetAnnotations()["other"])
	})
}

func TestFromContextNObject(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		result := fromContextNObject(context.Background(), &corev1.ConfigMap{})
		assert.Nil(t, result)
	})

	t.Run("only context", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx1")
		result := fromContextNObject(ctx, &corev1.ConfigMap{})
		assert.Equal(t, []string{"ctx1"}, result)
	})

	t.Run("only object", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constant.KBAppMultiClusterPlacementKey: "ctx2",
			},
		}}
		result := fromContextNObject(context.Background(), obj)
		assert.Equal(t, []string{"ctx2"}, result)
	})

	t.Run("both set, intersection", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx1,ctx2,ctx3")
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constant.KBAppMultiClusterPlacementKey: "ctx2,ctx3",
			},
		}}
		result := fromContextNObject(ctx, obj)
		assert.Contains(t, result, "ctx2")
		assert.Contains(t, result, "ctx3")
		assert.Len(t, result, 2)
	})
}

func TestFromContext(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		//nolint:staticcheck
		result := fromContext(nil)
		assert.Nil(t, result)
	})

	t.Run("no placement in context", func(t *testing.T) {
		result := fromContext(context.Background())
		assert.Nil(t, result)
	})

	t.Run("with placement", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "a,b")
		result := fromContext(ctx)
		assert.Equal(t, []string{"a", "b"}, result)
	})
}

func TestFromObject(t *testing.T) {
	t.Run("nil object", func(t *testing.T) {
		result := fromObject(nil)
		assert.Nil(t, result)
	})

	t.Run("no annotations", func(t *testing.T) {
		obj := &corev1.ConfigMap{}
		result := fromObject(obj)
		assert.Nil(t, result)
	})

	t.Run("no placement annotation", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"other": "val"},
		}}
		result := fromObject(obj)
		assert.Nil(t, result)
	})

	t.Run("with placement", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constant.KBAppMultiClusterPlacementKey: "ctx1,ctx2",
			},
		}}
		result := fromObject(obj)
		assert.Equal(t, []string{"ctx1", "ctx2"}, result)
	})
}

func TestEnabled4Object(t *testing.T) {
	t.Run("no placement", func(t *testing.T) {
		obj := &corev1.ConfigMap{}
		assert.False(t, Enabled4Object(obj))
	})

	t.Run("with placement", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constant.KBAppMultiClusterPlacementKey: "ctx1",
			},
		}}
		assert.True(t, Enabled4Object(obj))
	})
}
