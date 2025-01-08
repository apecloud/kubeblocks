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
	"context"
	"fmt"
	"maps"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	"github.com/apecloud/kubeblocks/pkg/generics"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func ListOwnedPods(ctx context.Context, cli client.Reader, namespace, clusterName, compName string,
	opts ...client.ListOption) ([]*corev1.Pod, error) {
	return listPods(ctx, cli, namespace, clusterName, compName, nil, opts...)
}

func listPods(ctx context.Context, cli client.Reader, namespace, clusterName, compName string,
	labels map[string]string, opts ...client.ListOption) ([]*corev1.Pod, error) {
	if labels == nil {
		labels = constant.GetCompLabels(clusterName, compName)
	} else {
		maps.Copy(labels, constant.GetCompLabels(clusterName, compName))
	}
	if opts == nil {
		opts = make([]client.ListOption, 0)
	}
	opts = append(opts, inDataContext())
	return listObjWithLabelsInNamespace(ctx, cli, generics.PodSignature, namespace, labels, opts...)
}

func listObjWithLabelsInNamespace[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](
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

func inDataContext() *multicluster.ClientOption {
	return multicluster.InDataContext()
}

func PodFQDN(namespace, compName, podName string) string {
	return fmt.Sprintf("%s.%s-headless.%s.svc.%s", podName, compName, namespace, clusterDomain())
}

func ServiceFQDN(namespace, serviceName string) string {
	return fmt.Sprintf("%s.%s.svc.%s", serviceName, namespace, clusterDomain())
}

func clusterDomain() string {
	return viper.GetString(constant.KubernetesClusterDomainEnv)
}
