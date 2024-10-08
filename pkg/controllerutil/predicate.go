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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	experimentalv1alpha1 "github.com/apecloud/kubeblocks/apis/experimental/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	storagev1alpha1 "github.com/apecloud/kubeblocks/apis/storage/v1alpha1"
	workloadsv1alpha1 "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var (
	managedNamespaces    *sets.Set[string]
	supportedAPIVersions = sets.New[string](
		appsv1alpha1.GroupVersion.String(),
		appsv1beta1.GroupVersion.String(),
		dpv1alpha1.GroupVersion.String(),
		experimentalv1alpha1.GroupVersion.String(),
		extensionsv1alpha1.GroupVersion.String(),
		storagev1alpha1.GroupVersion.String(),
		workloadsv1alpha1.GroupVersion.String(),
	)
)

func NewControllerManagedBy(mgr manager.Manager) *builder.Builder {
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(predicate.NewPredicateFuncs(namespacePredicateFilter)).
		WithEventFilter(predicate.NewPredicateFuncs(apiVersionPredicateFilter))
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

func apiVersionPredicateFilter(object client.Object) bool {
	annotations := object.GetAnnotations()
	if annotations == nil {
		return true
	}
	apiVersion, ok := annotations[constant.CRDAPIVersionAnnotationKey]
	if !ok {
		return true
	}
	return supportedAPIVersions.Has(apiVersion)
}
