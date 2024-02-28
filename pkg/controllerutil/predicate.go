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

package controllerutil

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var (
	managedNamespaces *sets.Set[string]
)

func NewNamespacedControllerManagedBy(mgr manager.Manager) *builder.Builder {
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(predicate.NewPredicateFuncs(namespacePredicateFilter))
}

func namespacePredicateFilter(object client.Object) bool {
	if managedNamespaces == nil {
		set := &sets.Set[string]{}
		namespaces := viper.GetString(strings.ReplaceAll(constant.ManagedNamespacesFlag, "-", "_"))
		if len(namespaces) > 0 {
			set.Insert(strings.Split(namespaces, ",")...)
		}
		managedNamespaces = set
	}
	if len(*managedNamespaces) == 0 || len(object.GetNamespace()) == 0 {
		return true
	}
	return managedNamespaces.Has(object.GetNamespace())
}
