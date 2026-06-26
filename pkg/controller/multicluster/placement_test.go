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

	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestIntoContext_FromContext(t *testing.T) {
	t.Run("roundtrip single placement", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx-1")
		p, err := FromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "ctx-1", p)
	})

	t.Run("roundtrip multiple placements", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx-1,ctx-2,ctx-3")
		p, err := FromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "ctx-1,ctx-2,ctx-3", p)
	})

	t.Run("no placement in context", func(t *testing.T) {
		_, err := FromContext(context.Background())
		assert.Error(t, err)
		assert.Equal(t, "no placement was present", err.Error())
	})
}

func TestPlacementNotFoundError_Error(t *testing.T) {
	err := placementNotFoundError{}
	assert.Equal(t, "no placement was present", err.Error())
}

func TestAssign(t *testing.T) {
	ordinal := func() int { return 0 }

	t.Run("already has placement annotation", func(t *testing.T) {
		cm := newConfigMap("default", "cm1", annotationWithPlacement("existing"))
		result := Assign(context.Background(), cm, ordinal)
		assert.Equal(t, "existing", result.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
	})

	t.Run("no placement in context", func(t *testing.T) {
		cm := newConfigMap("default", "cm2", nil)
		result := Assign(context.Background(), cm, ordinal)
		// should return obj unchanged without annotation
		assert.Nil(t, result.GetAnnotations())
	})

	t.Run("empty placement in context", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "")
		cm := newConfigMap("default", "cm3", nil)
		result := Assign(ctx, cm, ordinal)
		assert.Nil(t, result.GetAnnotations())
	})

	t.Run("assigns to first context with ordinal 0", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx-a,ctx-b")
		cm := newConfigMap("default", "cm4", nil)
		result := Assign(ctx, cm, ordinal)
		assert.Equal(t, "ctx-a", result.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
	})

	t.Run("assigns to second context with ordinal 1", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx-a,ctx-b")
		cm := newConfigMap("default", "cm5", nil)
		ordinal1 := func() int { return 1 }
		result := Assign(ctx, cm, ordinal1)
		assert.Equal(t, "ctx-b", result.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
	})

	t.Run("ordinal wraps around", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx-a,ctx-b")
		cm := newConfigMap("default", "cm6", nil)
		ordinal2 := func() int { return 2 }
		result := Assign(ctx, cm, ordinal2)
		// 2 % 2 = 0 -> ctx-a
		assert.Equal(t, "ctx-a", result.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
	})

	t.Run("preserves existing annotations when setting placement", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx-a")
		cm := newConfigMap("default", "cm7", map[string]string{"foo": "bar"})
		result := Assign(ctx, cm, ordinal)
		assert.Equal(t, "bar", result.GetAnnotations()["foo"])
		assert.Equal(t, "ctx-a", result.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
	})
}

func TestSetPlacementKey(t *testing.T) {
	t.Run("already has placement annotation - skip", func(t *testing.T) {
		cm := newConfigMap("default", "cm1", annotationWithPlacement("existing"))
		setPlacementKey(cm, "new-ctx")
		assert.Equal(t, "existing", cm.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
	})

	t.Run("empty context - skip", func(t *testing.T) {
		cm := newConfigMap("default", "cm2", nil)
		setPlacementKey(cm, "")
		assert.Nil(t, cm.GetAnnotations())
	})

	t.Run("nil annotations - create map", func(t *testing.T) {
		cm := newConfigMap("default", "cm3", nil)
		setPlacementKey(cm, "ctx-1")
		assert.Equal(t, "ctx-1", cm.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
	})

	t.Run("non-nil annotations - add key", func(t *testing.T) {
		cm := newConfigMap("default", "cm4", map[string]string{"key1": "val1"})
		setPlacementKey(cm, "ctx-2")
		assert.Equal(t, "val1", cm.GetAnnotations()["key1"])
		assert.Equal(t, "ctx-2", cm.GetAnnotations()[constant.KBAppMultiClusterPlacementKey])
	})
}

func TestFromContext_Internal(t *testing.T) {
	t.Run("nil context returns nil", func(t *testing.T) {
		assert.Nil(t, fromContext(nil))
	})

	t.Run("no placement in context returns nil", func(t *testing.T) {
		assert.Nil(t, fromContext(context.Background()))
	})

	t.Run("single placement", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx-1")
		result := fromContext(ctx)
		assert.Equal(t, []string{"ctx-1"}, result)
	})

	t.Run("multiple placements", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx-1,ctx-2,ctx-3")
		result := fromContext(ctx)
		assert.Equal(t, []string{"ctx-1", "ctx-2", "ctx-3"}, result)
	})
}

func TestFromObject(t *testing.T) {
	t.Run("nil object returns nil", func(t *testing.T) {
		assert.Nil(t, fromObject(nil))
	})

	t.Run("nil annotations returns nil", func(t *testing.T) {
		cm := newConfigMap("default", "cm1", nil)
		assert.Nil(t, fromObject(cm))
	})

	t.Run("no placement key returns nil", func(t *testing.T) {
		cm := newConfigMap("default", "cm2", map[string]string{"other": "val"})
		assert.Nil(t, fromObject(cm))
	})

	t.Run("single placement", func(t *testing.T) {
		cm := newConfigMap("default", "cm3", annotationWithPlacement("ctx-1"))
		assert.Equal(t, []string{"ctx-1"}, fromObject(cm))
	})

	t.Run("multiple placements", func(t *testing.T) {
		cm := newConfigMap("default", "cm4", annotationWithPlacement("ctx-1,ctx-2"))
		assert.Equal(t, []string{"ctx-1", "ctx-2"}, fromObject(cm))
	})
}

func TestFromContextNObject(t *testing.T) {
	t.Run("both nil returns nil", func(t *testing.T) {
		result := fromContextNObject(context.Background(), nil)
		assert.Nil(t, result)
	})

	t.Run("only context has placement", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx-1,ctx-2")
		result := fromContextNObject(ctx, nil)
		assert.Equal(t, []string{"ctx-1", "ctx-2"}, result)
	})

	t.Run("only object has placement", func(t *testing.T) {
		cm := newConfigMap("default", "cm1", annotationWithPlacement("ctx-a,ctx-b"))
		result := fromContextNObject(context.Background(), cm)
		assert.Equal(t, []string{"ctx-a", "ctx-b"}, result)
	})

	t.Run("both have placement - intersection", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx-1,ctx-2,ctx-3")
		cm := newConfigMap("default", "cm2", annotationWithPlacement("ctx-2,ctx-3,ctx-4"))
		result := fromContextNObject(ctx, cm)
		// intersection of {ctx-1,ctx-2,ctx-3} and {ctx-2,ctx-3,ctx-4} = {ctx-2,ctx-3}
		assert.ElementsMatch(t, []string{"ctx-2", "ctx-3"}, result)
	})

	t.Run("both have placement - no intersection", func(t *testing.T) {
		ctx := IntoContext(context.Background(), "ctx-1,ctx-2")
		cm := newConfigMap("default", "cm3", annotationWithPlacement("ctx-3,ctx-4"))
		result := fromContextNObject(ctx, cm)
		assert.Empty(t, result)
	})

	t.Run("context nil but object has placement", func(t *testing.T) {
		cm := newConfigMap("default", "cm4", annotationWithPlacement("ctx-x"))
		result := fromContextNObject(nil, cm)
		assert.Equal(t, []string{"ctx-x"}, result)
	})
}
