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
	var (
		namespace   = synthesizeComp.Namespace
		clusterName = synthesizeComp.ClusterName
		compName    = synthesizeComp.Name
		replicas    = int(synthesizeComp.Replicas)
		data        = make(map[string][]byte)
	)

	if err := buildConnCredentialEndpoint(namespace, clusterName, compName, replicas,
		transCtx.CompDef, nil, synthesizeComp.ComponentServices, credential, &data); err != nil {
		return nil, err
	}

	if len(credential.Account.Account) > 0 {
		var systemAccount *appsv1alpha1.SystemAccount
		for i, account := range synthesizeComp.SystemAccounts {
			if account.Name == credential.Account.Account {
				systemAccount = &synthesizeComp.SystemAccounts[i]
				break
			}
		}
		if systemAccount == nil {
			return nil, fmt.Errorf("connection credential references a system account not defined")
		}
		if err := buildConnCredentialAccount(transCtx, dag, synthesizeComp.Namespace, synthesizeComp.ClusterName,
			synthesizeComp.Name, systemAccount.Name, &data); err != nil {
			return nil, err
		}
	}

	secret := factory.BuildConnCredential4Component(synthesizeComp, credential.Name, data)
	return secret, nil
}
