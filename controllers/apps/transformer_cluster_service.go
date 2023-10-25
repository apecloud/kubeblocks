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

package apps

import (
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// ClusterServiceTransformer handles cluster services.
type ClusterServiceTransformer struct{}

var _ graph.Transformer = &ClusterServiceTransformer{}

func (t *ClusterServiceTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for _, svc := range cluster.Spec.Services {
		service, err := t.buildService(transCtx, cluster, &svc)
		if err != nil {
			return err
		}
		if err = t.createOrUpdate(ctx, dag, graphCli, service); err != nil {
			return err
		}
	}
	return nil
}

func (t *ClusterServiceTransformer) buildService(transCtx *clusterTransformContext,
	cluster *appsv1alpha1.Cluster, service *appsv1alpha1.ClusterService) (*corev1.Service, error) {
	var (
		namespace   = cluster.Namespace
		clusterName = cluster.Name
	)

	builder := builder.NewServiceBuilder(namespace, service.Service.Name).
		AddLabelsInMap(constant.GetClusterWellKnownLabels(clusterName)).
		SetSpec(&service.Service.Spec)

	if len(service.ComponentSelector) > 0 {
		compDef, err := t.checkComponent(transCtx, cluster, service)
		if err != nil {
			return nil, err
		}

		builder.AddSelector(constant.KBAppComponentLabelKey, service.ComponentSelector)

		// TODO(component): role selector
		if len(service.RoleSelector) > 0 {
			if err := t.checkComponentRoles(compDef, service); err != nil {
				return nil, err
			}
			builder.AddSelector(constant.RoleLabelKey, strings.Join(service.RoleSelector, ","))
		}
	}
	return builder.GetObject(), nil
}

func (t *ClusterServiceTransformer) checkComponent(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster,
	service *appsv1alpha1.ClusterService) (*appsv1alpha1.ComponentDefinition, error) {
	compName := service.ComponentSelector
	for _, comp := range transCtx.ComponentSpecs {
		if comp.Name == compName {
			compDef, ok := transCtx.ComponentDefs[comp.ComponentDef]
			if !ok {
				return nil, fmt.Errorf("the component definition of service selector is not defined, service: %s, component: %s", service.Name, compName)
			}
			return compDef, nil
		}
	}
	return nil, fmt.Errorf("the component of service selector is not exist, service: %s, component: %s", service.Name, compName)
}

func (t *ClusterServiceTransformer) checkComponentRoles(compDef *appsv1alpha1.ComponentDefinition, service *appsv1alpha1.ClusterService) error {
	definedRoles := make(map[string]bool)
	for _, role := range compDef.Spec.Roles {
		definedRoles[strings.ToLower(role.Name)] = true
	}
	for _, role := range service.RoleSelector {
		if !definedRoles[strings.ToLower(role)] {
			return fmt.Errorf("role selector for service is not defined, service: %s, role: %s", service.Name, role)
		}
	}
	return nil
}

func (t *ClusterServiceTransformer) createOrUpdate(ctx graph.TransformContext,
	dag *graph.DAG, graphCli model.GraphClient, service *corev1.Service) error {
	key := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}
	obj := &corev1.Service{}
	if err := ctx.GetClient().Get(ctx.GetContext(), key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			graphCli.Create(dag, service)
			return nil
		}
		return err
	}
	objCopy := obj.DeepCopy()
	objCopy.Spec = service.Spec
	if !reflect.DeepEqual(obj, objCopy) || !reflect.DeepEqual(obj.Annotations, objCopy.Annotations) {
		graphCli.Update(dag, obj, objCopy)
	}
	return nil
}
