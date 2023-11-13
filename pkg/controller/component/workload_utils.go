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

package component

import (
	"context"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	client2 "github.com/apecloud/kubeblocks/pkg/controller/client"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

func ListObjWithLabelsInNamespace[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](
	ctx context.Context, cli client.Reader, _ func(T, PT, L, PL), namespace string, labels client.MatchingLabels) ([]PT, error) {
	var objList L
	if err := cli.List(ctx, PL(&objList), labels, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	objs := make([]PT, 0)
	items := reflect.ValueOf(&objList).Elem().FieldByName("Items").Interface().([]T)
	for i := range items {
		objs = append(objs, &items[i])
	}
	return objs, nil
}

func ListRSMOwnedByComponent(ctx context.Context, cli client.Client, namespace string, labels client.MatchingLabels) ([]*workloads.ReplicatedStateMachine, error) {
	return ListObjWithLabelsInNamespace(ctx, cli, generics.RSMSignature, namespace, labels)
}

// GetObjectListByComponentName gets k8s workload list with component
func GetObjectListByComponentName(ctx context.Context, cli client2.ReadonlyClient, cluster appsv1alpha1.Cluster,
	objectList client.ObjectList, componentName string) error {
	matchLabels := constant.GetComponentWellKnownLabels(cluster.Name, componentName)
	inNamespace := client.InNamespace(cluster.Namespace)
	return cli.List(ctx, objectList, client.MatchingLabels(matchLabels), inNamespace)
}

// GetComponentDeployMinReadySeconds gets the deployment minReadySeconds of the component.
func GetComponentDeployMinReadySeconds(ctx context.Context,
	cli client.Client,
	cluster appsv1alpha1.Cluster,
	componentName string) (minReadySeconds int32, err error) {
	deployList := &appsv1.DeploymentList{}
	if err = GetObjectListByComponentName(ctx, cli, cluster, deployList, componentName); err != nil {
		return
	}
	if len(deployList.Items) > 0 {
		minReadySeconds = deployList.Items[0].Spec.MinReadySeconds
		return
	}
	return minReadySeconds, err
}

// GetComponentStsMinReadySeconds gets the statefulSet minReadySeconds of the component.
func GetComponentStsMinReadySeconds(ctx context.Context,
	cli client.Client,
	cluster appsv1alpha1.Cluster,
	componentName string) (minReadySeconds int32, err error) {
	stsList := &appsv1.StatefulSetList{}
	if err = GetObjectListByComponentName(ctx, cli, cluster, stsList, componentName); err != nil {
		return
	}
	if len(stsList.Items) > 0 {
		minReadySeconds = stsList.Items[0].Spec.MinReadySeconds
		return
	}
	return minReadySeconds, err
}

// GetComponentWorkloadMinReadySeconds gets the workload minReadySeconds of the component.
func GetComponentWorkloadMinReadySeconds(ctx context.Context,
	cli client.Client,
	cluster appsv1alpha1.Cluster,
	workloadType appsv1alpha1.WorkloadType,
	componentName string) (minReadySeconds int32, err error) {
	switch workloadType {
	case appsv1alpha1.Stateless:
		return GetComponentDeployMinReadySeconds(ctx, cli, cluster, componentName)
	default:
		return GetComponentStsMinReadySeconds(ctx, cli, cluster, componentName)
	}
}
