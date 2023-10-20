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
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type delegatorFinder struct {
	baseFinder

	// nameLabels defines labels should be contained in event source object.
	// And these labels will be used to build the delegator object's name by concat in order by '-' separator.
	nameLabels []string
}

var _ Finder = &delegatorFinder{}

func NewDelegatorFinder(delegatorType runtime.Object, nameLabels []string) Finder {
	return &delegatorFinder{
		nameLabels: nameLabels,
		baseFinder: baseFinder{
			objectType: delegatorType,
		},
	}
}

func (finder *delegatorFinder) Find(ctx *FinderContext, object client.Object) *model.GVKNObjKey {
	// make sure all labels in NameLabels exist and get the corresponding values in order.
	labels := object.GetLabels()
	if labels == nil {
		return nil
	}
	var nameValues []string
	for _, label := range finder.nameLabels {
		if value, ok := labels[label]; !ok {
			return nil
		} else {
			nameValues = append(nameValues, value)
		}
	}
	// build the delegator object's name
	name := strings.Join(nameValues, "-")
	objKey := client.ObjectKey{
		Namespace: object.GetNamespace(),
		Name:      name,
	}
	delegatorGVK := finder.getGroupVersionKind(&ctx.Scheme)
	if delegatorGVK == nil {
		return nil
	}
	return &model.GVKNObjKey{
		GroupVersionKind: *delegatorGVK,
		ObjectKey:        objKey,
	}
}
