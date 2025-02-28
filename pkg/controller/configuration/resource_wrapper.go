/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type ResourceFetcher[T any] struct {
	obj *T
	*render.ResourceCtx

	ClusterObj      *appsv1.Cluster
	ComponentObj    *appsv1.Component
	ComponentDefObj *appsv1.ComponentDefinition
	ClusterComObj   *appsv1.ClusterComponentSpec

	ConfigMapObj          *corev1.ConfigMap
	ComponentParameterObj *parametersv1alpha1.ComponentParameter
}

func (r *ResourceFetcher[T]) Init(ctx *render.ResourceCtx, object *T) *T {
	r.obj = object
	r.ResourceCtx = ctx
	return r.obj
}

func (r *ResourceFetcher[T]) Wrap(fn func() error) (ret *T) {
	ret = r.obj
	if r.Err != nil {
		return
	}
	r.Err = fn()
	return
}

func (r *ResourceFetcher[T]) Cluster() *T {
	clusterKey := client.ObjectKey{
		Namespace: r.Namespace,
		Name:      r.ClusterName,
	}
	return r.Wrap(func() error {
		r.ClusterObj = &appsv1.Cluster{}
		return r.Client.Get(r.Context, clusterKey, r.ClusterObj)
	})
}

func (r *ResourceFetcher[T]) ComponentAndComponentDef() *T {
	componentKey := client.ObjectKey{
		Namespace: r.Namespace,
		Name:      constant.GenerateClusterComponentName(r.ClusterName, r.ComponentName),
	}
	return r.Wrap(func() error {
		r.ComponentObj = &appsv1.Component{}
		if err := r.Client.Get(r.Context, componentKey, r.ComponentObj, inDataContext()); err != nil {
			return err
		}
		if len(r.ComponentObj.Spec.CompDef) == 0 {
			return fmt.Errorf("componentDefinition not found in component: %s", r.ComponentObj.Name)
		}

		compDefKey := types.NamespacedName{
			Name: r.ComponentObj.Spec.CompDef,
		}
		r.ComponentDefObj = &appsv1.ComponentDefinition{}
		if err := r.Client.Get(r.Context, compDefKey, r.ComponentDefObj, inDataContext()); err != nil {
			return err
		}
		if r.ComponentDefObj.Status.Phase != appsv1.AvailablePhase {
			return fmt.Errorf("ComponentDefinition referenced is unavailable: %s", r.ComponentDefObj.Name)
		}
		return nil
	})
}

func (r *ResourceFetcher[T]) ComponentSpec() *T {
	return r.Wrap(func() (err error) {
		r.ClusterComObj, err = intctrlutil.GetComponentSpecByName(r.Context, r.Client, r.ClusterObj, r.ComponentName)
		if err != nil {
			return err
		}
		return
	})
}

func (r *ResourceFetcher[T]) ConfigMap(configSpec string) *T {
	cmKey := client.ObjectKey{
		Name:      cfgcore.GetComponentCfgName(r.ClusterName, r.ComponentName, configSpec),
		Namespace: r.Namespace,
	}

	return r.Wrap(func() error {
		r.ConfigMapObj = &corev1.ConfigMap{}
		return r.Client.Get(r.Context, cmKey, r.ConfigMapObj, inDataContextUnspecified())
	})
}

func (r *ResourceFetcher[T]) ComponentParameter() *T {
	configKey := client.ObjectKey{
		Name:      cfgcore.GenerateComponentConfigurationName(r.ClusterName, r.ComponentName),
		Namespace: r.Namespace,
	}
	return r.Wrap(func() error {
		componentParameters := &parametersv1alpha1.ComponentParameter{}
		if err := r.Client.Get(r.Context, configKey, componentParameters); err != nil {
			return err
		}
		r.ComponentParameterObj = componentParameters
		return nil
	})
}

func (r *ResourceFetcher[T]) Complete() error {
	return r.Err
}

type Fetcher struct {
	ResourceFetcher[Fetcher]
}

func NewResourceFetcher(resourceCtx *render.ResourceCtx) *Fetcher {
	f := &Fetcher{}
	return f.Init(resourceCtx, f)
}
