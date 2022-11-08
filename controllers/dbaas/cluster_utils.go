/*
Copyright ApeCloud Inc.

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

package dbaas

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// getObjectList get object list with cluster instance label
func getObjectList(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster, objectList client.ObjectList) error {
	matchLabels := client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey: cluster.Name,
	}
	inNamespace := client.InNamespace(cluster.Namespace)
	return cli.List(ctx, objectList, matchLabels, inNamespace)
}

// getComponentTypeMapWithCluster get component type map. key is component name, value is component type
func getComponentTypeMapWithCluster(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster) (map[string]dbaasv1alpha1.ComponentType, error) {
	var (
		clusterDef       = &dbaasv1alpha1.ClusterDefinition{}
		err              error
		componentTypeMap = map[string]dbaasv1alpha1.ComponentType{}
	)
	if err = cli.Get(ctx, client.ObjectKey{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
		return componentTypeMap, err
	}
	for _, v := range cluster.Spec.Components {
		for _, c := range clusterDef.Spec.Components {
			if c.TypeName != v.Type {
				continue
			}
			componentTypeMap[v.Name] = c.ComponentType
			break
		}
	}
	return componentTypeMap, nil
}

// checkConsensusStatefulSetRevision check whether the pods owned by StatefulSet belong to the statefulSet current version
func checkConsensusStatefulSetRevision(ctx context.Context, cli client.Client, sts *appsv1.StatefulSet) (bool, error) {
	var (
		statefulStatusRevisionIsEquals = true
		pods                           []corev1.Pod
		err                            error
	)
	if pods, err = component.GetPodListByStatefulSet(ctx, cli, sts); err != nil {
		return false, err
	}
	for _, pod := range pods {
		if component.GetPodRevision(&pod) == sts.Status.UpdateRevision {
			continue
		}
		statefulStatusRevisionIsEquals = false
		break
	}
	return statefulStatusRevisionIsEquals, nil
}
