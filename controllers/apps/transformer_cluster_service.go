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

package apps

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// clusterServiceTransformer handles cluster services.
type clusterServiceTransformer struct{}

var _ graph.Transformer = &clusterServiceTransformer{}

func (t *clusterServiceTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}
	if common.IsCompactMode(transCtx.OrigCluster.Annotations) {
		transCtx.V(1).Info("Cluster is in compact mode, no need to create service objects",
			"cluster", client.ObjectKeyFromObject(transCtx.OrigCluster))
		return nil
	}

	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)

	services, err := listOwnedClusterServices(transCtx.Context, transCtx.Client, cluster, withShardingLabel)
	if err != nil {
		return err
	}

	protoServices, err := t.buildClusterServices(transCtx, cluster)
	if err != nil {
		return err
	}

	toCreateServices, toDeleteServices, toUpdateServices := mapDiff(services, protoServices)

	for svc := range toCreateServices {
		graphCli.Create(dag, protoServices[svc], inDataContext4G())
	}
	for svc := range toUpdateServices {
		updateService(dag, graphCli, services[svc], protoServices[svc])
	}
	for svc := range toDeleteServices {
		graphCli.Delete(dag, services[svc], inDataContext4G())
	}
	return nil
}

func (t *clusterServiceTransformer) buildClusterServices(transCtx *clusterTransformContext,
	cluster *appsv1.Cluster) (map[string]*corev1.Service, error) {
	services := make(map[string]*corev1.Service)
	for i := range cluster.Spec.Services {
		svc := &cluster.Spec.Services[i]
		if isShardingSelector(svc.ComponentSelector, cluster) {
			// service points to a sharding component which should be handled by sharding controller
			continue
		}
		service, err := t.buildClusterService(transCtx, cluster, svc)
		if err != nil {
			return nil, err
		}
		services[service.Name] = service
	}
	return services, nil
}

func (t *clusterServiceTransformer) buildClusterService(transCtx *clusterTransformContext,
	cluster *appsv1.Cluster, service *appsv1.ClusterService) (*corev1.Service, error) {
	var selectors map[string]string
	if len(service.ComponentSelector) > 0 {
		for _, spec := range cluster.Spec.ComponentSpecs {
			if spec.Name != service.ComponentSelector {
				continue
			}
			selectors = map[string]string{
				constant.KBAppComponentLabelKey: service.ComponentSelector,
			}
		}
	}
	return t.buildService(transCtx, cluster, service, selectors)
}

func (t *clusterServiceTransformer) buildService(transCtx *clusterTransformContext, cluster *appsv1.Cluster,
	service *appsv1.ClusterService, selectors map[string]string) (*corev1.Service, error) {
	serviceName := constant.GenerateClusterServiceName(cluster.Name, service.ServiceName)
	builder := builder.NewServiceBuilder(cluster.Namespace, serviceName).
		AddLabelsInMap(constant.GetClusterLabels(cluster.Name)).
		AddAnnotationsInMap(service.Annotations).
		SetSpec(&service.Spec).
		AddSelectorsInMap(t.builtinSelector(cluster)).
		AddSelectorsInMap(selectors).
		Optimize4ExternalTraffic()

	if len(service.RoleSelector) > 0 {
		compDef, err := t.checkComponentDef(transCtx, cluster, service)
		if err != nil {
			return nil, err
		}
		if err := checkComponentRoles(compDef, service); err != nil {
			return nil, err
		}
		builder.AddSelector(constant.RoleLabelKey, service.RoleSelector)
	}

	return builder.GetObject(), nil
}

func (t *clusterServiceTransformer) builtinSelector(cluster *appsv1.Cluster) map[string]string {
	selectors := map[string]string{
		constant.AppManagedByLabelKey: constant.AppName,
		constant.AppInstanceLabelKey:  cluster.Name,
	}
	return selectors
}

func (t *clusterServiceTransformer) checkComponentDef(transCtx *clusterTransformContext,
	cluster *appsv1.Cluster, service *appsv1.ClusterService) (*appsv1.ComponentDefinition, error) {
	selector := service.ComponentSelector
	for _, spec := range cluster.Spec.ComponentSpecs {
		if spec.Name != selector {
			continue
		}
		compDef, ok := transCtx.ComponentDefs[spec.ComponentDef]
		if !ok {
			return nil, fmt.Errorf("the component definition of service selector is not defined, service: %s, component: %s", service.Name, selector)
		}
		return compDef, nil
	}
	return nil, fmt.Errorf("the component of service selector is not exist, service: %s, component: %s", service.Name, selector)
}

// isComponentSelector checks if the given component selector exists in the component specs.
func isComponentSelector(selector string, componentSpecs []appsv1.ClusterComponentSpec) bool {
	for _, comp := range componentSpecs {
		if comp.Name == selector {
			return true
		}
	}
	return false
}

func updateService(dag *graph.DAG, graphCli model.GraphClient, running, proto *corev1.Service) {
	newSvc := running.DeepCopy()
	newSvc.Spec = proto.Spec
	ctrlutil.MergeMetadataMapInplace(proto.Labels, &newSvc.Labels)
	ctrlutil.MergeMetadataMapInplace(proto.Annotations, &newSvc.Annotations)
	resolveServiceDefaultFields(&running.Spec, &newSvc.Spec)

	if !reflect.DeepEqual(running, newSvc) {
		graphCli.Update(dag, running, newSvc, inDataContext4G())
	}
}

func checkComponentRoles(compDef *appsv1.ComponentDefinition, clusterService *appsv1.ClusterService) error {
	definedRoles := make(map[string]bool)
	for _, role := range compDef.Spec.Roles {
		definedRoles[strings.ToLower(role.Name)] = true
	}
	if !definedRoles[strings.ToLower(clusterService.RoleSelector)] {
		return fmt.Errorf("role selector for service is not defined, service: %s, role: %s", clusterService.Name, clusterService.RoleSelector)
	}
	return nil
}

func listOwnedClusterServices(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, filter func(obj client.Object) bool) (map[string]*corev1.Service, error) {
	svcList := &corev1.ServiceList{}
	labels := client.MatchingLabels(constant.GetClusterLabels(cluster.Name))
	if err := cli.List(ctx, svcList, labels, client.InNamespace(cluster.Namespace)); err != nil {
		return nil, err
	}

	services := make(map[string]*corev1.Service)
	for i, svc := range svcList.Items {
		if model.IsOwnerOf(cluster, &svc) && (filter == nil || !filter(&svc)) {
			services[svc.Name] = &svcList.Items[i]
		}
	}
	return services, nil
}
