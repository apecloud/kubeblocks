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

package kubebuilderx

import (
	"context"
	"reflect"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// ReadObjectTree reads all objects owned by the root object which is type of 'T' with key in 'req'.
func ReadObjectTree[T client.Object](ctx context.Context, reader client.Reader, req ctrl.Request, labelKeys []string, kinds ...client.ObjectList) (*ObjectTree, error) {
	root := *new(T)
	if err := reader.Get(ctx, req.NamespacedName, root); err != nil {
		return nil, err
	}

	children := make(model.ObjectSnapshot)
	ml := getMatchLabels(root, labelKeys)
	inNS := client.InNamespace(req.Namespace)
	for _, list := range kinds {
		if err := reader.List(ctx, list, inNS, ml); err != nil {
			return nil, err
		}
		// reflect get list.Items
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		l := items.Len()
		for i := 0; i < l; i++ {
			// get the underlying object
			object := items.Index(i).Addr().Interface().(client.Object)
			name, err := model.GetGVKName(object)
			if err != nil {
				return nil, err
			}
			children[*name] = object
		}
	}

	tree := &ObjectTree{
		Root:     root,
		Children: children,
	}

	return tree, nil
}

func getMatchLabels(root client.Object, labelKeys []string) client.MatchingLabels {
	labels := make(map[string]string, len(labelKeys))
	for _, key := range labelKeys {
		labels[key] = root.GetLabels()[key]
	}
	return labels
}
