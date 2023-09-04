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

package controllerutil

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ResourceCtx struct {
	context.Context

	Err    error
	Client client.Client

	Namespace     string
	ClusterName   string
	ComponentName string
}

type ResourceFetcher struct {
	ResourceCtx

	ClusterObj       *appsv1alpha1.Cluster
	ClusterDefObj    *appsv1alpha1.ClusterDefinition
	ClusterVerObj    *appsv1alpha1.ClusterVersion
	ConfigurationObj *appsv1alpha1.Configuration

	ClusterComObj    *appsv1alpha1.ClusterComponentSpec
	ClusterDefComObj *appsv1alpha1.ClusterComponentDefinition
	ClusterVerComObj *appsv1alpha1.ClusterComponentVersion
}

func NewResourceFetcher(ctx ResourceCtx) *ResourceFetcher {
	return &ResourceFetcher{ResourceCtx: ctx}
}

func (r *ResourceFetcher) Wrap(fn func() error) (ret *ResourceFetcher) {
	ret = r
	if ret.Err != nil {
		return
	}
	ret.Err = fn()
	return
}

func (r *ResourceFetcher) Cluster() *ResourceFetcher {
	clusterKey := client.ObjectKey{
		Namespace: r.Namespace,
		Name:      r.ClusterName,
	}
	return r.Wrap(func() error {
		r.ClusterObj = &appsv1alpha1.Cluster{}
		return r.Client.Get(r.Context, clusterKey, r.ClusterObj)
	})
}

func (r *ResourceFetcher) ClusterDef() *ResourceFetcher {
	clusterDefKey := client.ObjectKey{
		Namespace: "",
		Name:      r.ClusterObj.Spec.ClusterDefRef,
	}
	return r.Wrap(func() error {
		r.ClusterDefObj = &appsv1alpha1.ClusterDefinition{}
		return r.Client.Get(r.Context, clusterDefKey, r.ClusterDefObj)
	})
}

func (r *ResourceFetcher) ClusterVer() *ResourceFetcher {
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
func (r *ResourceFetcher) ClusterDefComponent() *ResourceFetcher {
	foundFn := func() (err error) {
		if r.ClusterComObj == nil {
			return
		}
		r.ClusterDefComObj = r.ClusterDefObj.GetComponentDefByName(r.ClusterComObj.ComponentDefRef)
		return
	}
	return r.Wrap(foundFn)
}

func (r *ResourceFetcher) ClusterComponent() *ResourceFetcher {
	return r.Wrap(func() (err error) {
		r.ClusterComObj = r.ClusterObj.Spec.GetComponentByName(r.ComponentName)
		return
	})
}

func (r *ResourceFetcher) Complete() error {
	return r.Err
}
