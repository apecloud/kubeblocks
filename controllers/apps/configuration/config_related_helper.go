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

package configuration

import (
	"context"
	"reflect"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

func retrieveRelatedComponentsByConfigmap[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](cli client.Client, ctx context.Context, configSpecName string, _ func(T, PT, L, PL), cfg client.ObjectKey, opts ...client.ListOption) ([]T, []string, error) {
	var objList L
	if err := cli.List(ctx, PL(&objList), opts...); err != nil {
		return nil, nil, err
	}

	objs := make([]T, 0)
	containers := cfgutil.NewSet()
	configSpecKey := core.GenerateTPLUniqLabelKeyWithConfig(configSpecName)
	items := toObjects[T, L, PL](&objList)
	for i := range items {
		obj := toResourceObject(&items[i])
		if objs == nil {
			return nil, nil, core.MakeError("failed to convert to resource object")
		}
		if !foundComponentConfigSpec(obj.GetAnnotations(), configSpecKey, cfg.Name) {
			continue
		}
		podTemplate := transformPodTemplate(obj)
		if podTemplate == nil {
			continue
		}
		volumeMounted := intctrlutil.GetVolumeMountName(podTemplate.Spec.Volumes, cfg.Name)
		if volumeMounted == nil {
			continue
		}
		// filter config manager sidecar container
		contains := intctrlutil.GetContainersByConfigmap(podTemplate.Spec.Containers,
			volumeMounted.Name, core.GenerateEnvFromName(cfg.Name),
			func(containerName string) bool {
				return constant.ConfigSidecarName == containerName
			})
		if len(contains) > 0 {
			objs = append(objs, items[i])
			containers.Add(contains...)
		}
	}
	return objs, containers.AsSlice(), nil
}

func transformPodTemplate(obj client.Object) *corev1.PodTemplateSpec {
	switch v := obj.(type) {
	default:
		return nil
	case *appv1.StatefulSet:
		return &v.Spec.Template
	case *appv1.Deployment:
		return &v.Spec.Template
	case *workloads.ReplicatedStateMachine:
		return &v.Spec.Template
	}
}

func toObjects[T generics.Object, L generics.ObjList[T], PL generics.PObjList[T, L]](compList PL) []T {
	return reflect.ValueOf(compList).Elem().FieldByName("Items").Interface().([]T)
}

func toResourceObject(obj any) client.Object {
	return obj.(client.Object)
}

func foundComponentConfigSpec(annotations map[string]string, key, value string) bool {
	return len(annotations) != 0 && annotations[key] == value
}
