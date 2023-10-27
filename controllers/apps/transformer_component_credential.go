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

// componentCredentialTransformer handles referenced resources validation and load them into context
type componentCredentialTransformer struct{}

var _ graph.Transformer = &componentCredentialTransformer{}

func (t *componentCredentialTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	cctx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(cctx.ComponentOrig) {
		return nil
	}

	synthesizeComp := cctx.SynthesizeComponent
	graphCli, _ := cctx.Client.(model.GraphClient)
	for _, credential := range synthesizeComp.ConnectionCredentials {
		secret, err := t.buildConnCredential(ctx, synthesizeComp, credential)
		if err != nil {
			return err
		}
		if err = t.createOrUpdate(ctx, dag, graphCli, secret); err != nil {
			return err
		}
	}
	return nil
}

func (t *componentCredentialTransformer) buildConnCredential(ctx graph.TransformContext,
	synthesizeComp *component.SynthesizedComponent, credential appsv1alpha1.ConnectionCredential) (*corev1.Secret, error) {
	secret := factory.BuildConnCredential4Component(synthesizeComp, credential.Name)
	if len(credential.SecretName) != 0 {
		if err := t.buildFromExistedSecret(ctx, credential, secret); err != nil {
			return nil, err
		}
	} else {
		if err := t.buildFromServiceAndAccount(ctx, synthesizeComp, credential, secret); err != nil {
			return nil, err
		}
	}
	return secret, nil
}

func (t *componentCredentialTransformer) buildFromExistedSecret(ctx graph.TransformContext,
	credential appsv1alpha1.ConnectionCredential, secret *corev1.Secret) error {
	namespace := func() string {
		namespace := credential.SecretNamespace
		if len(namespace) == 0 {
			namespace = secret.Namespace
		}
		return namespace
	}
	secretKey := types.NamespacedName{
		Namespace: namespace(),
		Name:      credential.SecretName,
	}
	obj := &corev1.Secret{}
	if err := ctx.GetClient().Get(ctx.GetContext(), secretKey, obj); err != nil {
		return err
	}
	secret.Immutable = obj.Immutable
	secret.Data = obj.Data
	secret.StringData = obj.StringData
	secret.Type = obj.Type
	return nil
}

func (t *componentCredentialTransformer) buildFromServiceAndAccount(ctx graph.TransformContext,
	synthesizeComp *component.SynthesizedComponent, credential appsv1alpha1.ConnectionCredential, secret *corev1.Secret) error {
	data := make(map[string]string)
	if len(credential.ServiceName) > 0 {
		if err := t.buildEndpoint(synthesizeComp, credential, &data); err != nil {
			return err
		}
	}
	if len(credential.AccountName) > 0 {
		if err := t.buildCredential(ctx, synthesizeComp.Namespace, credential.AccountName, &data); err != nil {
			return err
		}
	}
	// TODO(component): define the format of conn-credential secret
	secret.StringData = data
	return nil
}

func (t *componentCredentialTransformer) buildEndpoint(synthesizeComp *component.SynthesizedComponent,
	credential appsv1alpha1.ConnectionCredential, data *map[string]string) error {
	var service *appsv1alpha1.ComponentService
	for i, svc := range synthesizeComp.ComponentServices {
		if svc.Name == credential.ServiceName {
			service = &synthesizeComp.ComponentServices[i]
			break
		}
	}
	if service == nil {
		return fmt.Errorf("connection credential references a service not definied, credential: %s, service: %s",
			credential.Name, credential.ServiceName)
	}
	if len(service.Ports) == 0 {
		return fmt.Errorf("connection credential references a service which doesn't define any ports, credential: %s, service: %s",
			credential.Name, credential.ServiceName)
	}
	if len(credential.PortName) == 0 && len(service.Ports) > 1 {
		return fmt.Errorf("connection credential should specify which port to use for the referenced service, credential: %s, service: %s",
			credential.Name, credential.ServiceName)
	}

	t.buildEndpointFromService(synthesizeComp, credential, service, data)
	return nil
}

func (t *componentCredentialTransformer) buildEndpointFromService(synthesizeComp *component.SynthesizedComponent,
	credential appsv1alpha1.ConnectionCredential, service *appsv1alpha1.ComponentService, data *map[string]string) {
	// TODO(component): service.ServiceName
	serviceName := constant.GenerateComponentServiceEndpoint(synthesizeComp.ClusterName, synthesizeComp.Name, string(service.ServiceName))

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

func (t *componentCredentialTransformer) buildCredential(ctx graph.TransformContext,
	namespace, accountName string, data *map[string]string) error {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      accountName, // TODO(component): secret name
	}
	secret := &corev1.Secret{}
	if err := ctx.GetClient().Get(ctx.GetContext(), key, secret); err != nil {
		return err
	}
	// TODO(component): which field should to use from accounts?
	maps.Copy(*data, secret.StringData)
	return nil
}

func (t *componentCredentialTransformer) createOrUpdate(ctx graph.TransformContext,
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
