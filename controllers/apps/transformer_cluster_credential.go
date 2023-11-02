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

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// ClusterCredentialTransformer creates the connection credential secret
type ClusterCredentialTransformer struct{}

var _ graph.Transformer = &ClusterCredentialTransformer{}

func (t *ClusterCredentialTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	if t.isLegacyCluster(transCtx) {
		return t.transformClusterCredentialLegacy(transCtx, dag)
	}
	return t.transformClusterCredential(transCtx, dag)
}

func (t *ClusterCredentialTransformer) isLegacyCluster(transCtx *clusterTransformContext) bool {
	for _, comp := range transCtx.ComponentSpecs {
		compDef, ok := transCtx.ComponentDefs[comp.ComponentDef]
		if ok && (len(compDef.UID) > 0 || !compDef.CreationTimestamp.IsZero()) {
			return false
		}
	}
	return true
}

func (t *ClusterCredentialTransformer) transformClusterCredentialLegacy(transCtx *clusterTransformContext, dag *graph.DAG) error {
	graphCli, _ := transCtx.Client.(model.GraphClient)
	synthesizedComponent := t.buildSynthesizedComponentLegacy(transCtx)
	if synthesizedComponent != nil {
		secret := factory.BuildConnCredential(transCtx.ClusterDef, transCtx.Cluster, synthesizedComponent)

		if secret != nil {
			graphCli.Create(dag, secret)
		}
	}
	return nil
}

func (t *ClusterCredentialTransformer) buildSynthesizedComponentLegacy(transCtx *clusterTransformContext) *component.SynthesizedComponent {
	for _, compDef := range transCtx.ClusterDef.Spec.ComponentDefs {
		if compDef.Service == nil {
			continue
		}
		for _, compSpec := range transCtx.ComponentSpecs {
			if compDef.Name != compSpec.ComponentDefRef {
				continue
			}
			return &component.SynthesizedComponent{
				Name:     compSpec.Name,
				Services: []corev1.Service{{Spec: compDef.Service.ToSVCSpec()}},
			}
		}
	}
	return nil
}

func (t *ClusterCredentialTransformer) transformClusterCredential(transCtx *clusterTransformContext, dag *graph.DAG) error {
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for _, credential := range transCtx.Cluster.Spec.Credentials {
		secret, err := t.buildClusterCredential(transCtx, credential)
		if err != nil {
			return err
		}
		if err = t.createOrUpdate(transCtx, dag, graphCli, secret); err != nil {
			return err
		}
	}
	return nil
}

func (t *ClusterCredentialTransformer) buildClusterCredential(transCtx *clusterTransformContext, credential appsv1alpha1.ClusterCredential) (*corev1.Secret, error) {
	cluster := transCtx.Cluster
	secret := factory.BuildConnCredential4Cluster(cluster, credential.Name)

	var compDef *appsv1alpha1.ComponentDefinition
	// if len(credential.ComponentName) > 0 {
	//	// TODO(component): lookup comp def
	// }

	data := make(map[string]string)
	if len(credential.ServiceName) > 0 {
		if err := t.buildServiceEndpoint(cluster, compDef, credential, &data); err != nil {
			return nil, err
		}
	}
	if len(credential.ComponentName) > 0 && len(credential.AccountName) > 0 {
		if err := t.buildCredential(transCtx, cluster.Namespace, credential, &data); err != nil {
			return nil, err
		}
	}
	// TODO(component): define the format of conn-credential secret
	secret.StringData = data

	return secret, nil
}

func (t *ClusterCredentialTransformer) buildServiceEndpoint(cluster *appsv1alpha1.Cluster, compDef *appsv1alpha1.ComponentDefinition,
	credential appsv1alpha1.ClusterCredential, data *map[string]string) error {
	clusterSvc, compSvc, ports := t.lookupMatchedService(cluster, compDef, credential)
	if clusterSvc == nil && compSvc == nil {
		return fmt.Errorf("cluster credential references a service which is not definied: %s-%s", cluster.Name, credential.Name)
	}
	if len(ports) == 0 {
		return fmt.Errorf("cluster credential references a service which doesn't define any ports: %s-%s", cluster.Name, credential.Name)
	}
	if len(credential.PortName) == 0 && len(ports) > 1 {
		return fmt.Errorf("cluster credential should specify which port to use for the referenced service: %s-%s", cluster.Name, credential.Name)
	}

	if clusterSvc != nil {
		t.buildEndpointFromClusterService(credential, clusterSvc, data)
	} else {
		t.buildEndpointFromComponentService(cluster, credential, compSvc, data)
	}
	return nil
}

func (t *ClusterCredentialTransformer) lookupMatchedService(cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ComponentDefinition, credential appsv1alpha1.ClusterCredential) (*appsv1alpha1.ClusterService, *appsv1alpha1.ComponentService, []corev1.ServicePort) {
	for i, svc := range cluster.Spec.Services {
		if svc.Name == credential.ServiceName {
			return &cluster.Spec.Services[i], nil, cluster.Spec.Services[i].Service.Spec.Ports
		}
	}
	if len(credential.ComponentName) > 0 && compDef != nil {
		for i, svc := range compDef.Spec.Services {
			if svc.Name == credential.ServiceName {
				return nil, &compDef.Spec.Services[i], compDef.Spec.Services[i].Ports
			}
		}
	}
	return nil, nil, nil
}

func (t *ClusterCredentialTransformer) buildEndpointFromClusterService(credential appsv1alpha1.ClusterCredential,
	service *appsv1alpha1.ClusterService, data *map[string]string) {
	port := int32(0)
	if len(credential.PortName) == 0 {
		port = service.Service.Spec.Ports[0].Port
	} else {
		for _, servicePort := range service.Service.Spec.Ports {
			if servicePort.Name == credential.PortName {
				port = servicePort.Port
				break
			}
		}
	}
	// TODO(component): define the service and port pattern
	(*data)["service"] = service.Name
	(*data)["port"] = fmt.Sprintf("%d", port)
}

func (t *ClusterCredentialTransformer) buildEndpointFromComponentService(cluster *appsv1alpha1.Cluster,
	credential appsv1alpha1.ClusterCredential, service *appsv1alpha1.ComponentService, data *map[string]string) {
	// TODO(component): service.ServiceName
	serviceName := constant.GenerateComponentServiceEndpoint(cluster.Name, credential.ComponentName, string(service.ServiceName))

	port := int32(0)
	if len(credential.PortName) == 0 {
		port = service.Ports[0].Port
	} else {
		for _, servicePort := range service.Ports {
			if servicePort.Name == credential.PortName {
				port = servicePort.Port
				break
			}
		}
	}
	// TODO(component): define the service and port pattern
	(*data)["service"] = serviceName
	(*data)["port"] = fmt.Sprintf("%d", port)
}

func (t *ClusterCredentialTransformer) buildCredential(ctx graph.TransformContext, namespace string,
	credential appsv1alpha1.ClusterCredential, data *map[string]string) error {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      credential.AccountName, // TODO(component): secret name
	}
	secret := &corev1.Secret{}
	if err := ctx.GetClient().Get(ctx.GetContext(), key, secret); err != nil {
		return err
	}
	// TODO: which field should to use from accounts?
	maps.Copy(*data, secret.StringData)
	return nil
}

func (t *ClusterCredentialTransformer) createOrUpdate(ctx graph.TransformContext,
	dag *graph.DAG, graphCli model.GraphClient, secret *corev1.Secret) error {
	key := types.NamespacedName{
		Namespace: secret.Namespace,
		Name:      secret.Name,
	}
	obj := &corev1.Secret{}
	if err := ctx.GetClient().Get(ctx.GetContext(), key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			graphCli.Create(dag, secret)
			return nil
		}
		return err
	}
	objCopy := obj.DeepCopy()
	objCopy.Immutable = secret.Immutable
	objCopy.Data = secret.Data
	objCopy.StringData = secret.StringData
	objCopy.Type = secret.Type
	if !reflect.DeepEqual(obj, objCopy) {
		graphCli.Update(dag, obj, objCopy)
	}
	return nil
}
