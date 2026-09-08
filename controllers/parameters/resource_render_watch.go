/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package parameters

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// Enqueue declared resource-var consumers, including currently unresolved refs.
// Matching the exact referents remains the resolver's responsibility. A cache
// scan is bounded to one Cluster; fingerprints suppress unrelated spec updates.
func (r *ComponentDrivenParameterReconciler) resourceInputDependents(ctx context.Context, obj client.Object) []reconcile.Request {
	var opts []client.ListOption
	var definition string
	switch source := obj.(type) {
	case *appsv1.Component:
		clusterName := source.Labels[constant.AppInstanceLabelKey]
		if clusterName == "" {
			return nil
		}
		opts = []client.ListOption{client.InNamespace(source.Namespace), client.MatchingLabels{constant.AppInstanceLabelKey: clusterName}}
	case *appsv1.Cluster:
		opts = []client.ListOption{client.InNamespace(source.Namespace), client.MatchingLabels{constant.AppInstanceLabelKey: source.Name}}
	case *appsv1.ComponentDefinition:
		definition = source.Name
	default:
		return nil
	}
	comps := &appsv1.ComponentList{}
	if err := r.List(ctx, comps, opts...); err != nil {
		log.FromContext(ctx).Error(err, "unable to list resource input consumers")
		return nil
	}
	definitions := make(map[string]*appsv1.ComponentDefinition)
	var requests []reconcile.Request
	for i := range comps.Items {
		comp := &comps.Items[i]
		if !comp.DeletionTimestamp.IsZero() {
			continue
		}
		if definition != "" {
			if comp.Spec.CompDef == definition {
				requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(comp)})
			}
			continue
		}
		cmpd, ok := definitions[comp.Spec.CompDef]
		if !ok {
			cmpd = &appsv1.ComponentDefinition{}
			if err := r.Get(ctx, client.ObjectKey{Name: comp.Spec.CompDef}, cmpd); err != nil {
				// Still enqueue on lookup failure; reconcile can report the error and retry.
				requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(comp)})
				continue
			}
			definitions[comp.Spec.CompDef] = cmpd
		}
		if len(resourceVars(cmpd)) > 0 {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(comp)})
		}
	}
	return requests
}
