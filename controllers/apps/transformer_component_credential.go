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

	corev1 "k8s.io/api/core/v1"

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
		secret, err := t.buildComponentCredential(transCtx, dag, synthesizeComp, credential)
		if err != nil {
			return err
		}
		if err = createOrUpdateCredentialSecret(ctx, dag, graphCli, secret); err != nil {
			return err
		}
	}
	return nil
}

func (t *componentCredentialTransformer) buildComponentCredential(transCtx *componentTransformContext, dag *graph.DAG,
	synthesizeComp *component.SynthesizedComponent, credential appsv1alpha1.ConnectionCredential) (*corev1.Secret, error) {
	data := make(map[string][]byte)
	if len(credential.ServiceName) > 0 {
		if err := t.buildCredentialEndpoint(synthesizeComp, credential, &data); err != nil {
			return nil, err
		}
	}
	if len(credential.AccountName) > 0 {
		var systemAccount *appsv1alpha1.SystemAccount
		for i, account := range synthesizeComp.SystemAccounts {
			if account.Name == credential.AccountName {
				systemAccount = &synthesizeComp.SystemAccounts[i]
				break
			}
		}
		if systemAccount == nil {
			return nil, fmt.Errorf("connection credential references a system account not defined")
		}
		if err := buildCredentialAccountFromSecret(transCtx, dag, synthesizeComp.Namespace, synthesizeComp.ClusterName,
			synthesizeComp.Name, systemAccount.Name, &data); err != nil {
			return nil, err
		}
	}

	secret := factory.BuildConnCredential4Component(synthesizeComp, credential.Name, data)
	return secret, nil
}

func (t *componentCredentialTransformer) buildCredentialEndpoint(synthesizeComp *component.SynthesizedComponent,
	credential appsv1alpha1.ConnectionCredential, data *map[string][]byte) error {
	var service *appsv1alpha1.Service
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
	if len(service.Spec.Ports) == 0 {
		return fmt.Errorf("connection credential references a service which doesn't define any ports, credential: %s, service: %s",
			credential.Name, credential.ServiceName)
	}
	if len(credential.PortName) == 0 && len(service.Spec.Ports) > 1 {
		return fmt.Errorf("connection credential should specify which port to use for the referenced service, credential: %s, service: %s",
			credential.Name, credential.ServiceName)
	}

	serviceName := constant.GenerateComponentServiceName(synthesizeComp.ClusterName, synthesizeComp.Name, service.ServiceName)
	buildCredentialEndpointFromService(credential, serviceName, service.Spec.Ports, data)
	return nil
}
