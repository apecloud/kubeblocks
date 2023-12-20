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
	"strings"

	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// componentAccountTransformer handles component system accounts.
type componentAccountTransformer struct{}

var _ graph.Transformer = &componentAccountTransformer{}

func (t *componentAccountTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}
	if common.IsCompactMode(transCtx.ComponentOrig.Annotations) {
		transCtx.V(1).Info("Component is in compact mode, no need to create account related objects", "component", client.ObjectKeyFromObject(transCtx.ComponentOrig))
		return nil
	}

	synthesizeComp := transCtx.SynthesizeComponent
	graphCli, _ := transCtx.Client.(model.GraphClient)

	for _, account := range synthesizeComp.SystemAccounts {
		exist, err := t.checkAccountSecretExist(ctx, synthesizeComp, account)
		if err != nil {
			return err
		}
		if exist {
			continue
		}
		secret, err := t.buildAccountSecret(ctx, synthesizeComp, account)
		if err != nil {
			return err
		}
		graphCli.Create(dag, secret)
	}
	return nil
}

func (t *componentAccountTransformer) checkAccountSecretExist(ctx graph.TransformContext,
	synthesizeComp *component.SynthesizedComponent, account appsv1alpha1.SystemAccount) (bool, error) {
	secretKey := types.NamespacedName{
		Namespace: synthesizeComp.Namespace,
		Name:      constant.GenerateAccountSecretName(synthesizeComp.ClusterName, synthesizeComp.Name, account.Name),
	}
	err := ctx.GetClient().Get(ctx.GetContext(), secretKey, &corev1.Secret{})
	switch {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err):
		return false, nil
	default:
		return false, err
	}
}

func (t *componentAccountTransformer) buildAccountSecret(ctx graph.TransformContext,
	synthesizeComp *component.SynthesizedComponent, account appsv1alpha1.SystemAccount) (*corev1.Secret, error) {
	var password []byte
	if account.SecretRef != nil {
		var err error
		if password, err = t.getPasswordFromSecret(ctx, account); err != nil {
			return nil, err
		}
	} else {
		password = t.generatePassword(account)
	}
	return t.buildAccountSecretWithPassword(synthesizeComp, account, password), nil
}

func (t *componentAccountTransformer) getPasswordFromSecret(ctx graph.TransformContext, account appsv1alpha1.SystemAccount) ([]byte, error) {
	secretKey := types.NamespacedName{
		Namespace: account.SecretRef.Namespace,
		Name:      account.SecretRef.Name,
	}
	secret := &corev1.Secret{}
	if err := ctx.GetClient().Get(ctx.GetContext(), secretKey, secret); err != nil {
		return nil, err
	}
	if len(secret.Data) == 0 || len(secret.Data[constant.AccountPasswdForSecret]) == 0 {
		return nil, fmt.Errorf("referenced account secret has no required credential field")
	}
	return secret.Data[constant.AccountPasswdForSecret], nil
}

func (t *componentAccountTransformer) generatePassword(account appsv1alpha1.SystemAccount) []byte {
	config := account.PasswordGenerationPolicy
	passwd, _ := password.Generate((int)(config.Length), (int)(config.NumDigits), (int)(config.NumSymbols), false, false)
	switch config.LetterCase {
	case appsv1alpha1.UpperCases:
		passwd = strings.ToUpper(passwd)
	case appsv1alpha1.LowerCases:
		passwd = strings.ToLower(passwd)
	}
	return []byte(passwd)
}

func (t *componentAccountTransformer) buildAccountSecretWithPassword(synthesizeComp *component.SynthesizedComponent,
	account appsv1alpha1.SystemAccount, password []byte) *corev1.Secret {
	secretName := constant.GenerateAccountSecretName(synthesizeComp.ClusterName, synthesizeComp.Name, account.Name)
	labels := constant.GetComponentWellKnownLabels(synthesizeComp.ClusterName, synthesizeComp.Name)
	return builder.NewSecretBuilder(synthesizeComp.Namespace, secretName).
		AddLabelsInMap(labels).
		AddLabels(constant.ClusterAccountLabelKey, account.Name).
		PutData(constant.AccountNameForSecret, []byte(account.Name)).
		PutData(constant.AccountPasswdForSecret, password).
		SetImmutable(true).
		GetObject()
}
