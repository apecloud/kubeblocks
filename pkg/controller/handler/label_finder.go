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

package handler

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type labelFinder struct {
	baseFinder
	managedByKey   string
	managedByValue string
	parentNameKey  string
}

var _ Finder = &labelFinder{}

// NewLabelFinder return a finder which finds the owner of an object
func NewLabelFinder(ownerType runtime.Object, managedByKey, managedByValue, parentNameKey string) Finder {
	return &labelFinder{
		baseFinder:     baseFinder{objectType: ownerType},
		managedByKey:   managedByKey,
		managedByValue: managedByValue,
		parentNameKey:  parentNameKey,
	}
}

func (finder *labelFinder) Find(ctx *FinderContext, object client.Object) *model.GVKNObjKey {
	if len(object.GetLabels()) == 0 {
		return nil
	}
	if v, ok := object.GetLabels()[finder.managedByKey]; !ok {
		return nil
	} else if v != finder.managedByValue {
		return nil
	}
	name, ok := object.GetLabels()[finder.parentNameKey]
	if !ok {
		return nil
	}
	gvk := finder.getGroupVersionKind(&ctx.Scheme)
	if gvk == nil {
		return nil
	}
	objKey := types.NamespacedName{
		Namespace: object.GetNamespace(),
		Name:      name,
	}
	return &model.GVKNObjKey{
		GroupVersionKind: *gvk,
		ObjectKey:        objKey,
	}
}
