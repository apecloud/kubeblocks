/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package apps

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
)

func listMonitorServices[T intctrlutil.Object, PT intctrlutil.PObject[T], L intctrlutil.ObjList[T], PL intctrlutil.PObjList[T, L]](
	ctx context.Context, cli client.Reader,
	clusterName, componentName string,
	component *appsv1alpha1.Component,
	_ func(T, PT, L, PL)) ([]T, error) {
	var objList L
	var objects []T

	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
	}
	if err := cli.List(ctx, PL(&objList), client.InNamespace(corev1.NamespaceAll), ml); err != nil {
		return nil, err
	}

	items := toObjects[T, L, PL](&objList)
	for _, object := range items {
		if isOwnerRef(PT(&object), component) {
			objects = append(objects, object)
		}
	}
	return objects, nil
}

func toObjects[T intctrlutil.Object, L intctrlutil.ObjList[T], PL intctrlutil.PObjList[T, L]](compList PL) []T {
	fValue := reflect.ValueOf(compList).Elem().FieldByName("Items")
	if !fValue.CanInterface() {
		return nil
	}
	value := fValue.Interface()
	switch v := value.(type) {
	case []T:
		return v
	case []*T:
		var rets []T
		for _, item := range v {
			rets = append(rets, *item)
		}
		return rets
	}
	return nil
}

func isOwnerRef(target, owner client.Object) bool {
	for _, ownerRef := range target.GetOwnerReferences() {
		if ownerRef.Name == owner.GetName() && ownerRef.UID == owner.GetUID() {
			return true
		}
	}
	return false
}

func deleteObjects[T intctrlutil.Object, PT intctrlutil.PObject[T]](objects []T, graphCli model.GraphClient, dag *graph.DAG) {
	for _, object := range objects {
		graphCli.Delete(dag, PT(&object))
	}
}
