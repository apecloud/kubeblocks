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

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
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

const (
	systemAccountLabel          = "apps.kubeblocks.io/system-account"
	systemAccountHashAnnotation = "apps.kubeblocks.io/system-account-hash"
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

	synthesizedComp := transCtx.SynthesizeComponent
	graphCli, _ := transCtx.Client.(model.GraphClient)

	// exist account objects
	secrets, err := listSystemAccountObjects(ctx, synthesizedComp)
	if err != nil {
		return err
	}
	runningNameSet := sets.New(maps.Keys(secrets)...)

	// proto accounts
	accounts, err := synthesizeSystemAccounts(transCtx.CompDef.Spec.SystemAccounts,
		transCtx.Component.Spec.SystemAccounts, false)
	if err != nil {
		return err
	}
	protoNameSet := sets.New(maps.Keys(accounts)...)

	createSet, deleteSet, updateSet := setDiff(runningNameSet, protoNameSet)

	for _, name := range sets.List(createSet) {
		if err := t.createAccount(transCtx, dag, graphCli, accounts[name]); err != nil {
			return err
		}
	}

	for _, name := range sets.List(deleteSet) {
		t.deleteAccount(transCtx, dag, graphCli, secrets[name])
	}

	for _, name := range sets.List(updateSet) {
		if err := t.updateAccount(transCtx, dag, graphCli, accounts[name], secrets[name]); err != nil {
			return err
		}
	}

	return nil
}

func (t *componentAccountTransformer) createAccount(transCtx *componentTransformContext,
	dag *graph.DAG, graphCli model.GraphClient, account synthesizedSystemAccount) error {
	secret, err := t.buildAccountSecret(transCtx, transCtx.SynthesizeComponent, account)
	if err != nil {
		return err
	}
	graphCli.Create(dag, secret, inUniversalContext4G())
	return nil
}

func (t *componentAccountTransformer) deleteAccount(transCtx *componentTransformContext,
	dag *graph.DAG, graphCli model.GraphClient, secret *corev1.Secret) {
	graphCli.Delete(dag, secret, inUniversalContext4G())
}

func (t *componentAccountTransformer) updateAccount(transCtx *componentTransformContext,
	dag *graph.DAG, graphCli model.GraphClient, account synthesizedSystemAccount, running *corev1.Secret) error {
	secret, err := t.buildAccountSecret(transCtx, transCtx.SynthesizeComponent, account)
	if err != nil {
		return err
	}

	runningCopy := running.DeepCopy()
	if account.SecretRef != nil {
		// sync password from the external secret
		runningCopy.Data[constant.AccountPasswdForSecret] = secret.Data[constant.AccountPasswdForSecret]
	}
	ctrlutil.MergeMetadataMapInplace(secret.Labels, &runningCopy.Labels)
	ctrlutil.MergeMetadataMapInplace(secret.Annotations, &runningCopy.Annotations)
	if !reflect.DeepEqual(running, runningCopy) {
		graphCli.Update(dag, running, runningCopy, inUniversalContext4G())
	}
	return nil
}

func (t *componentAccountTransformer) buildAccountSecret(ctx *componentTransformContext,
	synthesizeComp *component.SynthesizedComponent, account synthesizedSystemAccount) (*corev1.Secret, error) {
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
	return t.buildAccountSecretWithPassword(ctx, synthesizeComp, account, password, account.SecretRef != nil)
}

func (t *componentAccountTransformer) getPasswordFromSecret(ctx graph.TransformContext, account synthesizedSystemAccount) ([]byte, error) {
	secretKey := types.NamespacedName{
		Namespace: account.SecretRef.Namespace,
		Name:      account.SecretRef.Name,
	}
	secret := &corev1.Secret{}
	if err := ctx.GetClient().Get(ctx.GetContext(), secretKey, secret); err != nil {
		return nil, err
	}

	passwordKey := constant.AccountPasswdForSecret
	if len(account.SecretRef.Password) > 0 {
		passwordKey = account.SecretRef.Password
	}
	if len(secret.Data) == 0 || len(secret.Data[passwordKey]) == 0 {
		return nil, fmt.Errorf("referenced account secret has no required credential field: %s", passwordKey)
	}
	return secret.Data[passwordKey], nil
}

func (t *componentAccountTransformer) buildPassword(ctx *componentTransformContext, account synthesizedSystemAccount) []byte {
	// get restore password if exists during recovery.
	password := factory.GetRestoreSystemAccountPassword(ctx.SynthesizeComponent.Annotations, ctx.SynthesizeComponent.Name, account.Name)
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

func (t *componentAccountTransformer) generatePassword(account synthesizedSystemAccount) []byte {
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
	synthesizeComp *component.SynthesizedComponent, account synthesizedSystemAccount, password []byte, signature bool) (*corev1.Secret, error) {
	secretName := constant.GenerateAccountSecretName(synthesizeComp.ClusterName, synthesizeComp.Name, account.Name)
	secret := builder.NewSecretBuilder(synthesizeComp.Namespace, secretName).
		// Priority: static < dynamic < built-in
		AddLabelsInMap(synthesizeComp.StaticLabels).
		AddLabelsInMap(synthesizeComp.DynamicLabels).
		AddLabelsInMap(constant.GetCompLabels(synthesizeComp.ClusterName, synthesizeComp.Name)).
		AddLabels(systemAccountLabel, account.Name).
		AddAnnotationsInMap(synthesizeComp.StaticAnnotations).
		AddAnnotationsInMap(synthesizeComp.DynamicAnnotations).
		PutData(constant.AccountNameForSecret, []byte(account.Name)).
		PutData(constant.AccountPasswdForSecret, password).
		SetImmutable(true).
		GetObject()
	if signature {
		if err := signatureSystemAccountPassword(secret); err != nil {
			return nil, err
		}
	}
	if err := setCompOwnershipNFinalizer(ctx.Component, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func listSystemAccountObjects(ctx graph.TransformContext,
	synthesizedComp *component.SynthesizedComponent) (map[string]*corev1.Secret, error) {
	opts := []client.ListOption{
		client.InNamespace(synthesizedComp.Namespace),
		client.MatchingLabels(constant.GetCompLabels(synthesizedComp.ClusterName, synthesizedComp.Name)),
	}
	secretList := &corev1.SecretList{}
	if err := ctx.GetClient().List(ctx.GetContext(), secretList, opts...); err != nil {
		return nil, err
	}

	m := make(map[string]*corev1.Secret)
	for i, secret := range secretList.Items {
		if accountName, ok := secret.Labels[systemAccountLabel]; ok {
			m[accountName] = &secretList.Items[i]
		}
	}
	return m, nil
}

func signatureSystemAccountPassword(secret *corev1.Secret) error {
	password := secret.Data[constant.AccountPasswdForSecret]
	hashedPassword, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	secret.Annotations[systemAccountHashAnnotation] = string(hashedPassword)
	return nil
}

func verifySystemAccountPassword(secret *corev1.Secret, hashedPassword []byte) bool {
	password := secret.Data[constant.AccountPasswdForSecret]
	err := bcrypt.CompareHashAndPassword(hashedPassword, password)
	return err == nil
}

type synthesizedSystemAccount struct {
	appsv1.SystemAccount
	Disabled  *bool
	SecretRef *appsv1.ProvisionSecretRef
}

func synthesizeSystemAccounts(compDefAccounts []appsv1.SystemAccount,
	compAccounts []appsv1.ComponentSystemAccount, keepDisabled bool) (map[string]synthesizedSystemAccount, error) {
	accounts := make(map[string]synthesizedSystemAccount)
	for _, account := range compDefAccounts {
		accounts[account.Name] = synthesizedSystemAccount{
			SystemAccount: account,
		}
	}

	merge := func(account synthesizedSystemAccount, compAccount appsv1.ComponentSystemAccount) synthesizedSystemAccount {
		if compAccount.PasswordConfig != nil {
			account.PasswordGenerationPolicy = *compAccount.PasswordConfig
		}
		account.Disabled = compAccount.Disabled
		account.SecretRef = compAccount.SecretRef
		return account
	}

	for i := range compAccounts {
		account, ok := accounts[compAccounts[i].Name]
		if !ok {
			return nil, fmt.Errorf("system account %s not defined in component definition", compAccounts[i].Name)
		}
		accounts[account.Name] = merge(account, compAccounts[i])
	}

	if !keepDisabled {
		for _, name := range maps.Keys(accounts) {
			account := accounts[name]
			if account.Disabled != nil && *account.Disabled {
				if account.InitAccount {
					return nil, fmt.Errorf("cannot disable init system account: %s", name)
				}
				delete(accounts, name)
			}
		}
	}
	return accounts, nil
}
