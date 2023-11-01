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

	"github.com/pkg/errors"
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

// componentCredentialTransformer handles component connection credentials.
type componentCredentialTransformer struct{}

var _ graph.Transformer = &componentCredentialTransformer{}

func (t *componentCredentialTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}

	synthesizeComp := transCtx.SynthesizeComponent
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for _, credential := range synthesizeComp.ConnectionCredentials {
		secret, err := t.buildConnCredential(transCtx, synthesizeComp, credential)
		if err != nil {
			return err
		}
		if err = t.createOrUpdate(ctx, dag, graphCli, secret); err != nil {
			return err
		}
	}
	return nil
}

func (t *componentCredentialTransformer) buildConnCredential(transCtx *componentTransformContext,
	synthesizeComp *component.SynthesizedComponent, credential appsv1alpha1.ConnectionCredential) (*corev1.Secret, error) {
	secret := factory.BuildConnCredential4Component(synthesizeComp, credential.Name)
	if len(credential.SecretName) != 0 {
		if err := t.buildFromExistedSecret(transCtx, credential, secret); err != nil {
			return nil, err
		}
	} else {
		if err := t.buildFromServiceAndAccount(transCtx, synthesizeComp, credential, secret); err != nil {
			return nil, err
		}
	}
	return secret, nil
}

func (t *componentCredentialTransformer) buildFromExistedSecret(transCtx *componentTransformContext,
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
	if err := transCtx.GetClient().Get(transCtx.GetContext(), secretKey, obj); err != nil {
		return err
	}
	secret.Immutable = obj.Immutable
	secret.Data = obj.Data
	secret.StringData = obj.StringData
	secret.Type = obj.Type
	return nil
}

func (t *componentCredentialTransformer) buildFromServiceAndAccount(transCtx *componentTransformContext,
	synthesizeComp *component.SynthesizedComponent, credential appsv1alpha1.ConnectionCredential, secret *corev1.Secret) error {
	data := make(map[string][]byte)
	if len(credential.ServiceName) > 0 {
		if err := t.buildEndpoint(synthesizeComp, credential, &data); err != nil {
			return err
		}
	}
	if len(credential.AccountName) > 0 {
		var systemAccount *appsv1alpha1.ComponentSystemAccount
		for i, account := range synthesizeComp.SystemAccounts {
			if account.Name == credential.AccountName {
				systemAccount = &synthesizeComp.SystemAccounts[i]
				break
			}
		}
		if systemAccount == nil {
			return errors.New("connection credential references a system account not defined")
		}
		if err := t.buildCredential(transCtx, synthesizeComp, systemAccount, &data); err != nil {
			return err
		}
	}
	secret.Data = data
	return nil
}

func (t *componentCredentialTransformer) buildEndpoint(synthesizeComp *component.SynthesizedComponent,
	credential appsv1alpha1.ConnectionCredential, data *map[string][]byte) error {
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
	credential appsv1alpha1.ConnectionCredential, service *appsv1alpha1.ComponentService, data *map[string][]byte) {
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
	(*data)["endpoint"] = []byte(fmt.Sprintf("%s:%d", serviceName, port))
	(*data)["host"] = []byte(serviceName)
	(*data)["port"] = []byte(fmt.Sprintf("%d", port))
}

func (t *componentCredentialTransformer) buildCredential(transCtx *componentTransformContext,
	synthesizedComp *component.SynthesizedComponent, account *appsv1alpha1.ComponentSystemAccount, data *map[string][]byte) error {
	secretKey := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      constant.GenerateAccountSecretName(synthesizedComp.ClusterName, synthesizedComp.Name, account.Name),
	}
	secret := &corev1.Secret{}
	if err := transCtx.GetClient().Get(transCtx.GetContext(), secretKey, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if secret, err = t.getAccountSecretFromLocalCache(transCtx, synthesizedComp, account); err != nil {
			return err
		}
	}
	maps.Copy(*data, secret.Data)
	return nil
}

func (t *componentCredentialTransformer) getAccountSecretFromLocalCache(transCtx *componentTransformContext,
	synthesizedComp *component.SynthesizedComponent, account *appsv1alpha1.ComponentSystemAccount) (*corev1.Secret, error) {
	graphCli, _ := transCtx.Client.(model.GraphClient)
	secretKey := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      constant.GenerateAccountSecretName(synthesizedComp.ClusterName, synthesizedComp.Name, account.Name),
	}
	secret := &corev1.Secret{}
	if err := graphCli.Get(transCtx.GetContext(), secretKey, secret); err != nil {
		return nil, err
	}
	return secret, nil
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
