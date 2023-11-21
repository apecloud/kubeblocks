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
	)

	data := make(map[string][]byte)
	if err := t.buildClusterCredentialEndpoint(transCtx, credential, &data); err != nil {
		return nil, err
	}

	if len(credential.Account.Component) > 0 && len(credential.Account.AccountName) > 0 {
		if err := buildConnCredentialAccount(transCtx, dag, cluster.Namespace, cluster.Name,
			credential.Account.Component, credential.Account.AccountName, &data); err != nil {
			return nil, err
		}
	}

	secret := factory.BuildConnCredential4Cluster(cluster, credential.Name, data)
	return secret, nil
}

func (t *clusterCredentialTransformer) buildClusterCredentialEndpoint(transCtx *clusterTransformContext,
	credential appsv1alpha1.ConnectionCredential, data *map[string][]byte) error {
	var (
		cluster         = transCtx.Cluster
		namespace       = cluster.Namespace
		clusterName     = cluster.Name
		compName        string
		replicas        int
		clusterServices []appsv1alpha1.Service
		compServices    []appsv1alpha1.Service
		compDef         *appsv1alpha1.ComponentDefinition
	)
	if credential.Endpoint.ServiceEndpoint != nil {
		clusterServices = cluster.Spec.Services
	}
	podEndpoint := credential.Endpoint.PodEndpoint
	if podEndpoint != nil && len(podEndpoint.Component) > 0 {
		for _, compSpec := range transCtx.ComponentSpecs {
			// TODO: how about there are more than one component with the same definition?
			if compSpec.ComponentDef == podEndpoint.Component {
				compName = compSpec.Name
				replicas = int(compSpec.Replicas)
				compDef = transCtx.ComponentDefs[compSpec.ComponentDef]
				break
			}
		}
		if compDef != nil {
			compServices = compDef.Spec.Services
		}
	}
	return buildConnCredentialEndpoint(namespace, clusterName, compName, replicas, compDef, clusterServices, compServices, credential, data)
}

func buildConnCredentialEndpoint(namespace, clusterName, compName string, replicas int, compDef *appsv1alpha1.ComponentDefinition,
	clusterService, compService []appsv1alpha1.Service, credential appsv1alpha1.ConnectionCredential, data *map[string][]byte) error {
	var (
		endpoint = credential.Endpoint
		hosts    []string
		port     string
		err      error
	)
	if endpoint.ServiceEndpoint != nil && endpoint.PodEndpoint != nil {
		return fmt.Errorf("service and pod endpoint cannot be specified at the same time")
	}
	if endpoint.ServiceEndpoint != nil {
		if hosts, port, err = buildServiceEndpoint(clusterName, compName, clusterService, compService, credential); err != nil {
			return err
		}
	}
	if endpoint.PodEndpoint != nil {
		if hosts, port, err = buildPodEndpoint(namespace, clusterName, compName, replicas, compDef, credential); err != nil {
			return err
		}
	}
	buildCredentialEndpointFromService(credential, hosts, port, data)
	return nil
}

func buildServiceEndpoint(clusterName, compName string, clusterService, compService []appsv1alpha1.Service,
	credential appsv1alpha1.ConnectionCredential) ([]string, string, error) {
	var (
		endpoint = credential.Endpoint.ServiceEndpoint
		svcName  string
		svcPorts []corev1.ServicePort
	)
	svcName, svcPorts = lookupMatchedClusterService(clusterName, clusterService, endpoint)
	if len(svcName) == 0 {
		svcName, svcPorts = lookupMatchedCompService(clusterName, compName, compService, endpoint)
	}
	if len(svcName) == 0 {
		return nil, "", fmt.Errorf("connection credential references a service which is not definied: %s-%s", clusterName, credential.Name)
	}
	if len(svcPorts) == 0 {
		return nil, "", fmt.Errorf("connection credential references a service which doesn't define any ports: %s-%s", clusterName, credential.Name)
	}
	if len(endpoint.PortName) == 0 && len(svcPorts) > 1 {
		return nil, "", fmt.Errorf("connection credential should specify which port to use for the referenced service: %s-%s", clusterName, credential.Name)
	}
	port := ""
	if len(endpoint.PortName) == 0 {
		port = fmt.Sprintf("%d", svcPorts[0].Port)
	} else {
		for _, svcPort := range svcPorts {
			if svcPort.Name == endpoint.PortName {
				port = fmt.Sprintf("%d", svcPort.Port)
				break
			}
		}
	}
	return []string{svcName}, port, nil
}

func lookupMatchedClusterService(clusterName string, services []appsv1alpha1.Service,
	svcEndpoint *appsv1alpha1.ConnectionServiceEndpoint) (string, []corev1.ServicePort) {
	return lookupMatchedService(services, svcEndpoint, func(svcName string) string {
		return constant.GenerateClusterServiceName(clusterName, svcName)
	})
}

func lookupMatchedCompService(clusterName, compName string,
	services []appsv1alpha1.Service, svcEndpoint *appsv1alpha1.ConnectionServiceEndpoint) (string, []corev1.ServicePort) {
	return lookupMatchedService(services, svcEndpoint, func(svcName string) string {
		return constant.GenerateComponentServiceName(clusterName, compName, svcName)
	})
}

func lookupMatchedService(services []appsv1alpha1.Service, svcEndpoint *appsv1alpha1.ConnectionServiceEndpoint,
	svcNameBuilder func(svcName string) string) (string, []corev1.ServicePort) {
	for i, svc := range services {
		if svc.Name == svcEndpoint.ServiceName {
			return svcNameBuilder(svc.ServiceName), services[i].Spec.Ports
		}
	}
	return "", nil
}

func buildPodEndpoint(namespace, clusterName, compName string, replicas int, compDef *appsv1alpha1.ComponentDefinition,
	credential appsv1alpha1.ConnectionCredential) ([]string, string, error) {
	var (
		hosts = make([]string, 0)
		port  string
	)
	if len(compDef.Spec.Runtime.Containers) == 0 {
		return nil, "", fmt.Errorf("")
	}

	for ordinal := 0; ordinal < replicas; ordinal++ {
		hosts = append(hosts, constant.GeneratePodFQDN(namespace, clusterName, compName, ordinal))
	}

	svcEndpoint := credential.Endpoint.PodEndpoint
	container := compDef.Spec.Runtime.Containers[0]
	if len(svcEndpoint.Container) > 0 {
		for i, c := range compDef.Spec.Runtime.Containers {
			if c.Name == svcEndpoint.Container {
				container = compDef.Spec.Runtime.Containers[i]
				break
			}
		}
	}
	if len(container.Ports) == 0 {
		return nil, "", fmt.Errorf("")
	}
	if len(svcEndpoint.PortName) == 0 && len(container.Ports) > 1 {
		return nil, "", fmt.Errorf("")
	}
	containerPort := container.Ports[0]
	if len(svcEndpoint.PortName) > 0 {
		for i, p := range container.Ports {
			if p.Name == svcEndpoint.PortName {
				containerPort = container.Ports[i]
				break
			}
		}
	}
	if containerPort.HostPort > 0 {
		port = fmt.Sprintf("%d", containerPort.HostPort)
	} else {
		port = fmt.Sprintf("%d", containerPort.ContainerPort)
	}

	return hosts, port, nil
}

func buildCredentialEndpointFromService(credential appsv1alpha1.ConnectionCredential, hosts []string, port string, data *map[string][]byte) {
	// TODO(component): define the service and port pattern
	endpoints := make([]string, 0)
	if len(port) == 0 {
		endpoints = hosts
	} else {
		for i := range hosts {
			endpoints = append(endpoints, fmt.Sprintf("%s:%s", hosts[i], port))
		}
	}
	if len(endpoints) > 0 {
		(*data)["endpoint"] = []byte(strings.Join(endpoints, credential.Endpoint.Separator))
	}
	if len(hosts) > 0 {
		(*data)["host"] = []byte(hosts[0])
	}
	if len(port) > 0 {
		(*data)["port"] = []byte(port)
	}
}

func buildConnCredentialAccount(ctx graph.TransformContext, dag *graph.DAG,
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
