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

package types

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/generics"
)

type ReconcileTask struct {
	ClusterDefinition *appsv1alpha1.ClusterDefinition
	ClusterVersion    *appsv1alpha1.ClusterVersion
	Cluster           *appsv1alpha1.Cluster
	Component         *component.SynthesizedComponent
	Resources         *[]client.Object
}

func InitReconcileTask(clusterDef *appsv1alpha1.ClusterDefinition, clusterVer *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) *ReconcileTask {
	resources := make([]client.Object, 0)
	return &ReconcileTask{
		ClusterDefinition: clusterDef,
		ClusterVersion:    clusterVer,
		Cluster:           cluster,
		Component:         component,
		Resources:         &resources,
	}
}

func (r *ReconcileTask) GetBuilderParams() builder.BuilderParams {
	return builder.BuilderParams{
		ClusterDefinition: r.ClusterDefinition,
		ClusterVersion:    r.ClusterVersion,
		Cluster:           r.Cluster,
		Component:         r.Component,
	}
}

func (r *ReconcileTask) AppendResource(objs ...client.Object) {
	if r == nil {
		return
	}
	*r.Resources = append(*r.Resources, objs...)
}

func (r *ReconcileTask) GetLocalResourceWithObjectKey(objKey client.ObjectKey, gvk schema.GroupVersionKind) client.Object {
	if r.Resources == nil {
		return nil
	}
	for _, obj := range *r.Resources {
		if obj.GetName() == objKey.Name && obj.GetNamespace() == objKey.Namespace {
			if generics.ToGVK(obj) == gvk {
				return obj
			}
		}
	}
	return nil
}
