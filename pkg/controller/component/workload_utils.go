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

package component

import (
	"context"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

func ListObjWithLabelsInNamespace[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](
	ctx context.Context, cli client.Reader, _ func(T, PT, L, PL), namespace string, labels client.MatchingLabels, opts ...client.ListOption) ([]PT, error) {
	if opts == nil {
		opts = make([]client.ListOption, 0)
	}
	opts = append(opts, []client.ListOption{labels, client.InNamespace(namespace)}...)

	var objList L
	if err := cli.List(ctx, PL(&objList), opts...); err != nil {
		return nil, err
	}

	objs := make([]PT, 0)
	items := reflect.ValueOf(&objList).Elem().FieldByName("Items").Interface().([]T)
	for i := range items {
		objs = append(objs, &items[i])
	}
	return objs, nil
}

// GetObjectListByComponentName gets k8s workload list with component
func GetObjectListByComponentName(ctx context.Context, cli client.Reader, cluster appsv1alpha1.Cluster,
	objectList client.ObjectList, componentName string) error {
	matchLabels := constant.GetComponentWellKnownLabels(cluster.Name, componentName)
	inNamespace := client.InNamespace(cluster.Namespace)
	return cli.List(ctx, objectList, client.MatchingLabels(matchLabels), inNamespace)
}

// GetComponentRSMMinReadySeconds gets the statefulSet minReadySeconds of the component.
func GetComponentRSMMinReadySeconds(ctx context.Context,
	cli client.Client,
	cluster appsv1alpha1.Cluster,
	componentName string) (minReadySeconds int32, err error) {
	rsmList := &workloads.InstanceSetList{}
	if err = GetObjectListByComponentName(ctx, cli, cluster, rsmList, componentName); err != nil {
		return
	}
	if len(rsmList.Items) > 0 {
		minReadySeconds = rsmList.Items[0].Spec.MinReadySeconds
		return
	}
	return minReadySeconds, err
}
