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

package lifecycle

import (
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
)

func newRequeueError(after time.Duration, reason string) error {
	return &realRequeueError{
		reason:       reason,
		requeueAfter: after,
	}
}

func getGVKName(object client.Object, scheme *runtime.Scheme) (*gvkName, error) {
	gvk, err := apiutil.GVKForObject(object, scheme)
	if err != nil {
		return nil, err
	}
	return &gvkName{
		gvk:  gvk,
		ns:   object.GetNamespace(),
		name: object.GetName(),
	}, nil
}

func getAppInstanceML(cluster appsv1alpha1.Cluster) client.MatchingLabels {
	return client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.Name,
	}
}

// func getAppInstanceAndManagedByML(cluster appsv1alpha1.Cluster) client.MatchingLabels {
//	return client.MatchingLabels{
//		constant.AppInstanceLabelKey:  cluster.Name,
//		constant.AppManagedByLabelKey: constant.AppName,
//	}
// }

// read all objects owned by our cluster
func readCacheSnapshot(transCtx *ClusterTransformContext, cluster appsv1alpha1.Cluster, matchLabels client.MatchingLabels, kinds ...client.ObjectList) (clusterSnapshot, error) {
	// list what kinds of object cluster owns
	snapshot := make(clusterSnapshot)
	inNS := client.InNamespace(cluster.Namespace)
	for _, list := range kinds {
		if err := transCtx.Client.List(transCtx.Context, list, inNS, matchLabels); err != nil {
			return nil, err
		}
		// reflect get list.Items
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		l := items.Len()
		for i := 0; i < l; i++ {
			// get the underlying object
			object := items.Index(i).Addr().Interface().(client.Object)
			name, err := getGVKName(object, scheme)
			if err != nil {
				return nil, err
			}
			snapshot[*name] = object
		}
	}

	return snapshot, nil
}
