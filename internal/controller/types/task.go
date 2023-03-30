/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
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

func (r *ReconcileTask) GetResourceWithObjectKey(objKey client.ObjectKey) client.Object {
	if r.Resources == nil {
		return nil
	}
	for _, obj := range *r.Resources {
		if obj.GetName() == objKey.Name && obj.GetNamespace() == objKey.Namespace {
			return obj
		}
	}
	return nil
}
