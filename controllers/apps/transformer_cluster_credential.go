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

// clusterCredentialTransformer creates the connection credential secret
type clusterCredentialTransformer struct{}

var _ graph.Transformer = &clusterCredentialTransformer{}

func (t *clusterCredentialTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	if t.isLegacyCluster(transCtx) {
		return t.transformClusterCredentialLegacy(transCtx, dag)
	}
	return t.transformClusterCredential(transCtx, dag)
}

func (t *clusterCredentialTransformer) isLegacyCluster(transCtx *clusterTransformContext) bool {
	for _, comp := range transCtx.ComponentSpecs {
		compDef, ok := transCtx.ComponentDefs[comp.ComponentDef]
		if ok && (len(compDef.UID) > 0 || !compDef.CreationTimestamp.IsZero()) {
			return false
		}
	}
	return true
}

func (t *clusterCredentialTransformer) transformClusterCredentialLegacy(transCtx *clusterTransformContext, dag *graph.DAG) error {
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

func (t *clusterCredentialTransformer) buildSynthesizedComponentLegacy(transCtx *clusterTransformContext) *component.SynthesizedComponent {
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

func (t *clusterCredentialTransformer) transformClusterCredential(transCtx *clusterTransformContext, dag *graph.DAG) error {
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for _, credential := range transCtx.Cluster.Spec.ConnectionCredentials {
		secret, err := t.buildClusterCredential(transCtx, dag, credential)
		if err != nil {
			return err
		}
		if err = createOrUpdateCredentialSecret(transCtx, dag, graphCli, secret); err != nil {
			return err
		}
	}
	return nil
}

func (t *clusterCredentialTransformer) buildClusterCredential(transCtx *clusterTransformContext, dag *graph.DAG,
	credential appsv1alpha1.ConnectionCredential) (*corev1.Secret, error) {
	var (
		cluster = transCtx.Cluster
		compDef *appsv1alpha1.ComponentDefinition
	)

	if len(credential.ComponentName) > 0 {
		for _, compSpec := range transCtx.ComponentSpecs {
			if compSpec.Name != credential.ComponentName {
				compDef = transCtx.ComponentDefs[compSpec.ComponentDef]
				break
			}
		}
	}

	data := make(map[string][]byte)
	if len(credential.ServiceName) > 0 {
		if err := t.buildCredentialEndpoint(cluster, compDef, credential, &data); err != nil {
			return nil, err
		}
	}
	if len(credential.ComponentName) > 0 && len(credential.AccountName) > 0 {
		if err := buildCredentialAccountFromSecret(transCtx, dag, cluster.Namespace, cluster.Name,
			credential.ComponentName, credential.AccountName, &data); err != nil {
			return nil, err
		}
	}

	secret := factory.BuildConnCredential4Cluster(cluster, credential.Name, data)
	return secret, nil
}

func (t *clusterCredentialTransformer) buildCredentialEndpoint(cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ComponentDefinition, credential appsv1alpha1.ConnectionCredential, data *map[string][]byte) error {
	serviceName, ports := t.lookupMatchedService(cluster, compDef, credential)
	if len(serviceName) == 0 {
		return fmt.Errorf("cluster credential references a service which is not definied: %s-%s", cluster.Name, credential.Name)
	}
	if len(ports) == 0 {
		return fmt.Errorf("cluster credential references a service which doesn't define any ports: %s-%s", cluster.Name, credential.Name)
	}
	if len(credential.PortName) == 0 && len(ports) > 1 {
		return fmt.Errorf("cluster credential should specify which port to use for the referenced service: %s-%s", cluster.Name, credential.Name)
	}

	buildCredentialEndpointFromService(credential, serviceName, ports, data)
	return nil
}

func (t *clusterCredentialTransformer) lookupMatchedService(cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ComponentDefinition, credential appsv1alpha1.ConnectionCredential) (string, []corev1.ServicePort) {
	for i, svc := range cluster.Spec.Services {
		if svc.Name == credential.ServiceName {
			serviceName := constant.GenerateClusterServiceName(cluster.Name, svc.ServiceName)
			return serviceName, cluster.Spec.Services[i].Spec.Ports
		}
	}
	if len(credential.ComponentName) > 0 && compDef != nil {
		for i, svc := range compDef.Spec.Services {
			if svc.Name == credential.ServiceName {
				serviceName := constant.GenerateComponentServiceName(cluster.Name, credential.ComponentName, svc.ServiceName)
				return serviceName, compDef.Spec.Services[i].Spec.Ports
			}
		}
	}
	return "", nil
}

func buildCredentialEndpointFromService(credential appsv1alpha1.ConnectionCredential,
	serviceName string, ports []corev1.ServicePort, data *map[string][]byte) {
	port := int32(0)
	if len(credential.PortName) == 0 {
		port = ports[0].Port
	} else {
		for _, servicePort := range ports {
			if servicePort.Name == credential.PortName {
				port = servicePort.Port
				break
			}
		}
	}
	// TODO(component): define the service and port pattern
	(*data)["endpoint"] = []byte(fmt.Sprintf("%s:%d", serviceName, port))
	(*data)["host"] = []byte(serviceName)
	(*data)["port"] = []byte(fmt.Sprintf("%d", port))
}

func buildCredentialAccountFromSecret(ctx graph.TransformContext, dag *graph.DAG,
	namespace, clusterName, compName, accountName string, data *map[string][]byte) error {
	secretKey := types.NamespacedName{
		Namespace: namespace,
		Name:      constant.GenerateAccountSecretName(clusterName, compName, accountName),
	}
	secret := &corev1.Secret{}
	if err := ctx.GetClient().Get(ctx.GetContext(), secretKey, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if secret, err = getAccountSecretFromLocalCache(ctx, dag, secretKey); err != nil {
			return err
		}
	}
	maps.Copy(*data, secret.Data)
	return nil
}

func getAccountSecretFromLocalCache(ctx graph.TransformContext, dag *graph.DAG, secretKey types.NamespacedName) (*corev1.Secret, error) {
	graphCli, _ := ctx.GetClient().(model.GraphClient)
	secrets := graphCli.FindAll(dag, &corev1.Secret{})
	for i, obj := range secrets {
		if obj.GetNamespace() == secretKey.Namespace && obj.GetName() == secretKey.Name {
			return secrets[i].(*corev1.Secret), nil
		}
	}
	return nil, fmt.Errorf("the account secret referenced is not found: %s", secretKey.String())
}

func createOrUpdateCredentialSecret(ctx graph.TransformContext, dag *graph.DAG, graphCli model.GraphClient, secret *corev1.Secret) error {
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
