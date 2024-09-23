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

package view

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func objectTypeToGVK(objectType *viewv1.ObjectType) (*schema.GroupVersionKind, error) {
	if objectType == nil {
		return nil, nil
	}
	gv, err := schema.ParseGroupVersion(objectType.APIVersion)
	if err != nil {
		return nil, err
	}
	gvk := gv.WithKind(objectType.Kind)
	return &gvk, nil
}

func objectRefToType(objectRef *corev1.ObjectReference) *viewv1.ObjectType {
	return &viewv1.ObjectType{
		APIVersion: objectRef.APIVersion,
		Kind:       objectRef.Kind,
	}
}

func objectReferenceToRef(reference *corev1.ObjectReference) (*model.GVKNObjKey, error) {
	if reference == nil {
		return nil, nil
	}
	gv, err := schema.ParseGroupVersion(reference.APIVersion)
	if err != nil {
		return nil, err
	}
	gvk := gv.WithKind(reference.Kind)
	return &model.GVKNObjKey{
		GroupVersionKind: gvk,
		ObjectKey: client.ObjectKey{
			Namespace: reference.Namespace,
			Name:      reference.Name,
		},
	}, nil
}

func getObjectsByGVK(ctx context.Context, cli client.Reader, scheme *runtime.Scheme, gvk *schema.GroupVersionKind, opts ...client.ListOption) ([]client.Object, error) {
	runtimeObjectList, err := scheme.New(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind + "List",
	})
	if err != nil {
		return nil, err
	}
	objectList, ok := runtimeObjectList.(client.ObjectList)
	if !ok {
		return nil, fmt.Errorf("list object is not a client.ObjectList for GVK %s", gvk)
	}
	if err = cli.List(ctx, objectList, opts...); err != nil {
		return nil, err
	}
	runtimeObjects, err := meta.ExtractList(objectList)
	if err != nil {
		return nil, err
	}
	var objects []client.Object
	for _, object := range runtimeObjects {
		o, ok := object.(client.Object)
		if !ok {
			return nil, fmt.Errorf("object is not a client.Object for GVK %s", gvk)
		}
		objects = append(objects, o)
	}

	return objects, nil
}
