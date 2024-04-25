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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func IsUnavailableError(err error) bool {
	_, ok := err.(*unavailableError)
	return ok
}

type unavailableError struct {
	context string
	call    string
	obj     string
}

func (e *unavailableError) Error() string {
	return fmt.Sprintf("cluster %s is unavailable, call: %s, object: %s", e.context, e.call, e.obj)
}

func genericUnavailableError(context string, obj runtime.Object) error {
	return &unavailableError{context, "Generic", objectNameKind(obj, "")}
}

func getUnavailableError(context string, obj client.Object) error {
	return &unavailableError{context, "Get", objectNameKind(obj, obj.GetName())}
}

func listUnavailableError(context string, obj runtime.Object) error {
	return &unavailableError{context, "List", objectNameKind(obj, "")}
}

func objectNameKind(obj runtime.Object, name string) string {
	gvk, _ := apiutil.GVKForObject(obj, scheme)
	return fmt.Sprintf("%s@%s", name, gvk.Kind)
}
