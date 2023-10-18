/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type ownerFinder struct {
	baseFinder
}

var _ Finder = &ownerFinder{}

// NewOwnerFinder return a finder which finds the owner of an object
func NewOwnerFinder(ownerType runtime.Object) Finder {
	return &ownerFinder{baseFinder{objectType: ownerType}}
}

func (finder *ownerFinder) Find(ctx *FinderContext, object client.Object) *model.GVKNObjKey {
	ownerRef := metav1.GetControllerOf(object)
	if ownerRef == nil {
		return nil
	}
	gv, err := schema.ParseGroupVersion(ownerRef.APIVersion)
	if err != nil {
		return nil
	}
	gvk := schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    ownerRef.Kind,
	}
	ownerGVK := finder.getGroupVersionKind(&ctx.Scheme)
	if ownerGVK == nil {
		return nil
	}
	if gvk.Group != ownerGVK.Group || gvk.Kind != ownerGVK.Kind {
		return nil
	}
	objKey := types.NamespacedName{
		Namespace: object.GetNamespace(),
		Name:      ownerRef.Name,
	}
	return &model.GVKNObjKey{
		GroupVersionKind: gvk,
		ObjectKey:        objKey,
	}
}
