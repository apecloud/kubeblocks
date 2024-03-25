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

package configuration

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type ResourceCtx struct {
	context.Context

	Err    error
	Client client.Client

	Namespace     string
	ClusterName   string
	ComponentName string
}

type ResourceFetcher[T any] struct {
	obj *T
	*ResourceCtx

	ClusterObj      *appsv1alpha1.Cluster
	ComponentObj    *appsv1alpha1.Component
	ComponentDefObj *appsv1alpha1.ComponentDefinition
	ClusterComObj   *appsv1alpha1.ClusterComponentSpec

	// Deprecated: this API will be removed from version 0.9.0
	ClusterDefObj *appsv1alpha1.ClusterDefinition
	// Deprecated: use ComponentDefinition instead
	ClusterVerObj *appsv1alpha1.ClusterVersion

	ConfigMapObj        *corev1.ConfigMap
	ConfigurationObj    *appsv1alpha1.Configuration
	ConfigConstraintObj *appsv1alpha1.ConfigConstraint
}

func (r *ResourceFetcher[T]) Init(ctx *ResourceCtx, object *T) *T {
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
		r.ClusterObj = &appsv1alpha1.Cluster{}
		return r.Client.Get(r.Context, clusterKey, r.ClusterObj)
	})
}

func (r *ResourceFetcher[T]) ComponentAndComponentDef() *T {
	componentKey := client.ObjectKey{
		Namespace: r.Namespace,
		Name:      constant.GenerateClusterComponentName(r.ClusterName, r.ComponentName),
	}
	return r.Wrap(func() error {
		r.ComponentObj = &appsv1alpha1.Component{}
		err := r.Client.Get(r.Context, componentKey, r.ComponentObj)
		if apierrors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}

		if len(r.ComponentObj.Spec.CompDef) == 0 {
			return nil
		}

		compDefKey := types.NamespacedName{
			Name: r.ComponentObj.Spec.CompDef,
		}
		r.ComponentDefObj = &appsv1alpha1.ComponentDefinition{}
		if err := r.Client.Get(r.Context, compDefKey, r.ComponentDefObj); err != nil {
			return err
		}
		if r.ComponentDefObj.Status.Phase != appsv1alpha1.AvailablePhase {
			return fmt.Errorf("ComponentDefinition referenced is unavailable: %s", r.ComponentDefObj.Name)
		}
		return nil
	})
}

// ClusterDef get clusterDefinition cr
// Deprecated: use ComponentDefinition instead
func (r *ResourceFetcher[T]) ClusterDef() *T {
	clusterDefKey := client.ObjectKey{
		Namespace: "",
		Name:      r.ClusterObj.Spec.ClusterDefRef,
	}
	return r.Wrap(func() error {
		r.ClusterDefObj = &appsv1alpha1.ClusterDefinition{}
		return r.Client.Get(r.Context, clusterDefKey, r.ClusterDefObj)
	})
}

// ClusterVer get clusterVersion cr
// Deprecated: this API will be removed from version 0.9.0
func (r *ResourceFetcher[T]) ClusterVer() *T {
	clusterVerKey := client.ObjectKey{
		Namespace: "",
		Name:      r.ClusterObj.Spec.ClusterVersionRef,
	}
	return r.Wrap(func() error {
		if clusterVerKey.Name == "" {
			return nil
		}
		r.ClusterVerObj = &appsv1alpha1.ClusterVersion{}
		return r.Client.Get(r.Context, clusterVerKey, r.ClusterVerObj)
	})
}

func (r *ResourceFetcher[T]) ComponentSpec() *T {
	return r.Wrap(func() (err error) {
		r.ClusterComObj, err = controllerutil.GetOriginalOrGeneratedComponentSpecByName(r.Context, r.Client, r.ClusterObj, r.ComponentName)
		if err != nil {
			return err
		}
		return
	})
}

func (r *ResourceFetcher[T]) Configuration() *T {
	configKey := client.ObjectKey{
		Name:      cfgcore.GenerateComponentConfigurationName(r.ClusterName, r.ComponentName),
		Namespace: r.Namespace,
	}
	return r.Wrap(func() (err error) {
		configuration := appsv1alpha1.Configuration{}
		err = r.Client.Get(r.Context, configKey, &configuration)
		if err != nil {
			return client.IgnoreNotFound(err)
		}
		r.ConfigurationObj = &configuration
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
		return r.Client.Get(r.Context, cmKey, r.ConfigMapObj)
	})
}

func (r *ResourceFetcher[T]) ConfigConstraints(ccName string) *T {
	return r.Wrap(func() error {
		if ccName != "" {
			r.ConfigConstraintObj = &appsv1alpha1.ConfigConstraint{}
			return r.Client.Get(r.Context, client.ObjectKey{Name: ccName}, r.ConfigConstraintObj)
		}
		return nil
	})
}

func (r *ResourceFetcher[T]) Complete() error {
	return r.Err
}

type Fetcher struct {
	ResourceFetcher[Fetcher]
}

func NewResourceFetcher(resourceCtx *ResourceCtx) *Fetcher {
	f := &Fetcher{}
	return f.Init(resourceCtx, f)
}
