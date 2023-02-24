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

package lifecycle

import (
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// cacheDiffTransformer reads all Vertex.Obj in cache and compute the diff DAG.
type cacheDiffTransformer struct {
	cc compoundCluster
	cli client.Client
	ctx intctrlutil.RequestCtx
}

type clusterSnapshot map[gvkName]client.Object

func ownKinds() []client.ObjectList {
	return []client.ObjectList{
		&appsv1.StatefulSetList{},
		&appsv1.DeploymentList{},
		&corev1.ServiceList{},
		&corev1.SecretList{},
		&corev1.ConfigMapList{},
		&corev1.PersistentVolumeClaimList{},
		&policyv1.PodDisruptionBudgetList{},
	}
}

// read all objects owned by our cluster
// using a brute search algorithm as all info we have is owner ref
// TODO: design a more efficient algorithm, such as label selector
func (c *cacheDiffTransformer) readCacheSnapshot() (clusterSnapshot, error) {
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()

	// list what kinds of object cluster owns
	kinds := ownKinds()
	snapshot := make(clusterSnapshot)
	for _, list := range kinds {
		if err := c.cli.List(c.ctx.Ctx, list); err != nil {
			return nil, err
		}
		// reflect get list.Items
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		l := items.Len()
		for i := 0; i < l; i++ {
			// get the underlying object
			object := items.Index(i).Addr().Interface().(client.Object)
			// put to snapshot if owned by our cluster
			if intctrlutil.IsOwnerOf(c.cc.cluster, object, scheme) {
				name := getGVKName(object)
				snapshot[name] = object
			}
		}
	}

	// put the cluster itself
	name := getGVKName(c.cc.cluster)
	snapshot[name] = c.cc.cluster

	return snapshot, nil
}

func (c *cacheDiffTransformer) Transform(dag *graph.DAG) error {
	// get the old snapshot
	snapshot, err := c.readCacheSnapshot()
	if err != nil {
		return err
	}

	// we have target snapshot in dag
	// now do the heavy lift:
	// compute the diff between cache and target spec and generate the plan

	return nil
}