/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

func IntoContext(ctx context.Context, placement string) context.Context {
	return context.WithValue(ctx, placementKey{}, placement)
}

func FromContext(ctx context.Context) (string, error) {
	if v, ok := ctx.Value(placementKey{}).(string); ok {
		return v, nil
	}
	return "", placementNotFoundError{}
}

func Assign(ctx context.Context, obj client.Object, ordinal func() int) client.Object {
	// has been set
	if obj.GetAnnotations() != nil && obj.GetAnnotations()[constant.KBAppMultiClusterPlacementKey] != "" {
		return obj
	}

	placement, err := FromContext(ctx)
	if err != nil || len(placement) == 0 {
		return obj
	}
	contexts := strings.Split(placement, ",")
	context := contexts[ordinal()%len(contexts)]

	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(map[string]string{constant.KBAppMultiClusterPlacementKey: context})
	} else {
		obj.GetAnnotations()[constant.KBAppMultiClusterPlacementKey] = context
	}

	return obj
}

func setPlacementKey(obj client.Object, context string) {
	// has been set
	if obj.GetAnnotations() != nil && obj.GetAnnotations()[constant.KBAppMultiClusterPlacementKey] != "" {
		return
	}
	// the context is empty
	if len(context) == 0 {
		return
	}
	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(map[string]string{constant.KBAppMultiClusterPlacementKey: context})
	} else {
		obj.GetAnnotations()[constant.KBAppMultiClusterPlacementKey] = context
	}
}

type placementKey struct{}

type placementNotFoundError struct{}

func (placementNotFoundError) Error() string {
	return "no placement was present"
}

func fromContextNObject(ctx context.Context, obj client.Object) []string {
	p1, p2 := fromContext(ctx), fromObject(obj)
	switch {
	case p1 == nil:
		return p2
	case p2 == nil:
		return p1
	default:
		s1, s2 := sets.New(p1...), sets.New(p2...)
		// if !s1.IsSuperset(s2) {
		//	panic("runtime error")
		// }
		// return p2
		return sets.List(s1.Intersection(s2))
	}
}

func fromContext(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	p, err := FromContext(ctx)
	if err != nil {
		return nil
	}
	return strings.Split(p, ",")
}

func fromObject(obj client.Object) []string {
	if obj == nil || obj.GetAnnotations() == nil {
		return nil
	}
	p, ok := obj.GetAnnotations()[constant.KBAppMultiClusterPlacementKey]
	if !ok {
		return nil
	}
	return strings.Split(p, ",")
}
