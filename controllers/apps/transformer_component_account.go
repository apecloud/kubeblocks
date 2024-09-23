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
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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
		existSecret, err := t.checkAccountSecretExist(ctx, synthesizeComp, account)
		if err != nil {
			return err
		}
		secret, err := t.buildAccountSecret(transCtx, synthesizeComp, account)
		if err != nil {
			return err
		}

		if existSecret == nil {
			graphCli.Create(dag, secret, inUniversalContext4G())
			continue
		}

		// just update existed account secret metadata if needed
		existSecretCopy := existSecret.DeepCopy()
		ctrlutil.MergeMetadataMapInplace(secret.Labels, &existSecretCopy.Labels)
		ctrlutil.MergeMetadataMapInplace(secret.Annotations, &existSecretCopy.Annotations)
		if !reflect.DeepEqual(existSecret, existSecretCopy) {
			graphCli.Update(dag, existSecret, existSecretCopy, inUniversalContext4G())
		}
	}
	// TODO: (good-first-issue) if an account is deleted from the Spec, the secret and account should be deleted
	return nil
}

func (t *componentAccountTransformer) checkAccountSecretExist(ctx graph.TransformContext,
	synthesizeComp *component.SynthesizedComponent, account appsv1.SystemAccount) (*corev1.Secret, error) {
	secretKey := types.NamespacedName{
		Namespace: synthesizeComp.Namespace,
		Name:      constant.GenerateAccountSecretName(synthesizeComp.ClusterName, synthesizeComp.Name, account.Name),
	}
	secret := &corev1.Secret{}
	err := ctx.GetClient().Get(ctx.GetContext(), secretKey, secret)
	switch {
	case err == nil:
		return secret, nil
	case apierrors.IsNotFound(err):
		return nil, nil
	default:
		return nil, err
	}
}

func (t *componentAccountTransformer) buildAccountSecret(ctx *componentTransformContext,
	synthesizeComp *component.SynthesizedComponent, account appsv1.SystemAccount) (*corev1.Secret, error) {
	var password []byte
	switch {
	case account.SecretRef != nil:
		var err error
		if password, err = t.getPasswordFromSecret(ctx, account); err != nil {
			return nil, err
		}
	default:
		password = t.buildPassword(ctx, account)
	}
	return t.buildAccountSecretWithPassword(ctx, synthesizeComp, account, password)
}

func (t *componentAccountTransformer) getPasswordFromSecret(ctx graph.TransformContext, account appsv1.SystemAccount) ([]byte, error) {
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

func (t *componentAccountTransformer) buildPassword(ctx *componentTransformContext, account appsv1.SystemAccount) []byte {
	// get restore password if exists during recovery.
	password := factory.GetRestoreSystemAccountPassword(ctx.SynthesizeComponent, account)
	if account.InitAccount && password == "" {
		// initAccount can also restore from factory.GetRestoreSystemAccountPassword(ctx.SynthesizeComponent, account).
		// This is compatibility processing.
		password = factory.GetRestorePassword(ctx.SynthesizeComponent)
	}
	if password == "" {
		return t.generatePassword(account)
	}
	return []byte(password)
}

func (t *componentAccountTransformer) generatePassword(account appsv1.SystemAccount) []byte {
	config := account.PasswordGenerationPolicy
	passwd, _ := common.GeneratePassword((int)(config.Length), (int)(config.NumDigits), (int)(config.NumSymbols), false, config.Seed)
	switch config.LetterCase {
	case appsv1.UpperCases:
		passwd = strings.ToUpper(passwd)
	case appsv1.LowerCases:
		passwd = strings.ToLower(passwd)
	}
	return []byte(passwd)
}

func (t *componentAccountTransformer) buildAccountSecretWithPassword(ctx *componentTransformContext,
	synthesizeComp *component.SynthesizedComponent, account appsv1.SystemAccount, password []byte) (*corev1.Secret, error) {
	secretName := constant.GenerateAccountSecretName(synthesizeComp.ClusterName, synthesizeComp.Name, account.Name)
	labels := constant.GetComponentWellKnownLabels(synthesizeComp.ClusterName, synthesizeComp.Name)
	secret := builder.NewSecretBuilder(synthesizeComp.Namespace, secretName).
		AddLabelsInMap(labels).
		AddLabelsInMap(synthesizeComp.DynamicLabels).
		AddLabels(constant.ClusterAccountLabelKey, account.Name).
		AddAnnotationsInMap(synthesizeComp.DynamicAnnotations).
		PutData(constant.AccountNameForSecret, []byte(account.Name)).
		PutData(constant.AccountPasswdForSecret, password).
		SetImmutable(true).
		GetObject()
	if err := setCompOwnershipNFinalizer(ctx.Component, secret); err != nil {
		return nil, err
	}
	return secret, nil
}
