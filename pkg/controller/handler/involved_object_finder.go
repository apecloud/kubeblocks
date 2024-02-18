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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type involvedObjectFinder struct {
	baseFinder
}

var _ Finder = &involvedObjectFinder{}

func NewInvolvedObjectFinder(objectType runtime.Object) Finder {
	return &involvedObjectFinder{
		baseFinder{
			objectType: objectType,
		},
	}
}

func (finder *involvedObjectFinder) Find(ctx *FinderContext, object client.Object) *model.GVKNObjKey {
	event, ok := object.(*corev1.Event)
	if !ok {
		return nil
	}
	objectRef := event.InvolvedObject
	gv, err := schema.ParseGroupVersion(objectRef.APIVersion)
	if err != nil {
		return nil
	}
	gvk := schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    objectRef.Kind,
	}
	objKey := client.ObjectKey{
		Namespace: objectRef.Namespace,
		Name:      objectRef.Name,
	}
	objectGVK := finder.getGroupVersionKind(&ctx.Scheme)
	if objectGVK == nil {
		return nil
	}
	if objectGVK.Group != gvk.Group || objectGVK.Kind != gvk.Kind {
		return nil
	}
	return &model.GVKNObjKey{
		GroupVersionKind: gvk,
		ObjectKey:        objKey,
	}
}
