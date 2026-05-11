/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package cluster

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// clusterShardingAccountTransformer handles shared system accounts for sharding.
type clusterShardingAccountTransformer struct{}

var _ graph.Transformer = &clusterShardingAccountTransformer{}

func (t *clusterShardingAccountTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if transCtx.OrigCluster.IsDeleting() {
		return nil
	}

	if common.IsCompactMode(transCtx.Cluster.Annotations) {
		transCtx.V(1).Info("Cluster is in compact mode, no need to create account related objects", "cluster", client.ObjectKeyFromObject(transCtx.Cluster))
		return nil
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	return t.reconcileShardingAccounts(transCtx, graphCli, dag)
}

func (t *clusterShardingAccountTransformer) reconcileShardingAccounts(transCtx *clusterTransformContext,
	graphCli model.GraphClient, dag *graph.DAG) error {
	for _, sharding := range transCtx.shardings {
		shardDef, ok := transCtx.shardingDefs[sharding.ShardingDef]
		if ok {
			for _, account := range shardDef.Spec.SystemAccounts {
				if ptr.Deref(account.Shared, false) {
					if err := t.reconcileShardingAccount(transCtx, graphCli, dag, sharding, account.Name); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (t *clusterShardingAccountTransformer) reconcileShardingAccount(transCtx *clusterTransformContext,
	graphCli model.GraphClient, dag *graph.DAG, sharding *appsv1.ClusterSharding, accountName string) error {
	exist, err := t.checkSystemAccountSecret(transCtx, sharding, accountName)
	if err != nil {
		return err
	}
	if !exist {
		obj, err := t.newSystemAccountSecret(transCtx, sharding, accountName)
		if err != nil {
			return err
		}
		graphCli.Create(dag, obj)
	}

	// TODO: update

	t.rewriteSystemAccount(transCtx, sharding.Name, accountName)

	return nil
}

func (t *clusterShardingAccountTransformer) checkSystemAccountSecret(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding, accountName string) (bool, error) {
	var (
		cluster = transCtx.Cluster
	)
	secretKey := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      shardingAccountSecretName(cluster.Name, sharding.Name, accountName),
	}
	secret := &corev1.Secret{}
	err := transCtx.GetClient().Get(transCtx.GetContext(), secretKey, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	return !apierrors.IsNotFound(err), nil
}

func (t *clusterShardingAccountTransformer) newSystemAccountSecret(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding, accountName string) (*corev1.Secret, error) {
	account, err := t.definedSystemAccount(transCtx, sharding, accountName)
	if err != nil {
		return nil, err
	}
	password, provisioned, err := t.buildPassword(transCtx, account)
	if err != nil {
		return nil, err
	}
	secret, err := t.newAccountSecretWithPassword(transCtx, sharding, accountName, password)
	if err != nil {
		return nil, err
	}
	if provisioned {
		if secret.Annotations == nil {
			secret.Annotations = map[string]string{}
		}
		secret.Annotations[constant.SystemAccountProvisionedAnnotationKey] = "true"
	}
	return secret, nil
}

func (t *clusterShardingAccountTransformer) definedSystemAccount(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding, accountName string) (synthesizedShardingSystemAccount, error) {
	var compAccount *appsv1.ComponentSystemAccount
	for i := range sharding.Template.SystemAccounts {
		if sharding.Template.SystemAccounts[i].Name == accountName {
			compAccount = &sharding.Template.SystemAccounts[i]
			break
		}
	}

	compDef, ok := transCtx.componentDefs[sharding.Template.ComponentDef]
	if !ok || compDef == nil {
		return synthesizedShardingSystemAccount{}, fmt.Errorf("component definition %s not found for sharding %s", sharding.Template.ComponentDef, sharding.Name)
	}

	override := func(account *appsv1.SystemAccount) synthesizedShardingSystemAccount {
		synthesized := synthesizedShardingSystemAccount{
			SystemAccount: *account,
		}
		if compAccount != nil {
			if compAccount.PasswordConfig != nil {
				synthesized.PasswordGenerationPolicy = *compAccount.PasswordConfig
			}
			synthesized.SecretRef = compAccount.SecretRef
		}
		return synthesized
	}

	for i, account := range compDef.Spec.SystemAccounts {
		if account.Name == accountName {
			return override(compDef.Spec.SystemAccounts[i].DeepCopy()), nil
		}
	}
	return synthesizedShardingSystemAccount{}, fmt.Errorf("system account %s not found in component definition %s", accountName, compDef.Name)
}

type synthesizedShardingSystemAccount struct {
	appsv1.SystemAccount
	SecretRef *appsv1.ProvisionSecretRef
}

func (t *clusterShardingAccountTransformer) buildPassword(transCtx *clusterTransformContext, account synthesizedShardingSystemAccount) ([]byte, bool, error) {
	if account.SecretRef != nil {
		return t.getPasswordFromSecret(transCtx, account)
	}
	password, err := common.GeneratePasswordByConfig(account.PasswordGenerationPolicy)
	return []byte(password), false, err
}

func (t *clusterShardingAccountTransformer) getPasswordFromSecret(transCtx *clusterTransformContext, account synthesizedShardingSystemAccount) ([]byte, bool, error) {
	secretKey := types.NamespacedName{
		Namespace: account.SecretRef.Namespace,
		Name:      account.SecretRef.Name,
	}
	if len(secretKey.Namespace) == 0 {
		secretKey.Namespace = transCtx.Cluster.Namespace
	}
	secret := &corev1.Secret{}
	if err := transCtx.GetClient().Get(transCtx.GetContext(), secretKey, secret); err != nil {
		return nil, false, err
	}

	passwordKey := constant.AccountPasswdForSecret
	if len(account.SecretRef.Password) > 0 {
		passwordKey = account.SecretRef.Password
	}
	if _, ok := secret.Data[passwordKey]; !ok {
		return nil, false, fmt.Errorf("referenced account secret has no required credential field: %s", passwordKey)
	}
	return secret.Data[passwordKey], secret.Annotations[constant.SystemAccountProvisionedAnnotationKey] == "true", nil
}

func (t *clusterShardingAccountTransformer) newAccountSecretWithPassword(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding, accountName string, password []byte) (*corev1.Secret, error) {
	var (
		cluster = transCtx.Cluster
	)
	compDef := transCtx.componentDefs[sharding.Template.ComponentDef]
	shardingLabels := map[string]string{
		constant.KBAppShardingNameLabelKey: sharding.Name,
	}
	secret := builder.NewSecretBuilder(cluster.Namespace, shardingAccountSecretName(cluster.Name, sharding.Name, accountName)).
		AddLabelsInMap(constant.GetClusterLabels(cluster.Name, shardingLabels)).
		AddLabelsInMap(sharding.Template.Labels).
		AddLabelsInMap(compDef.Spec.Labels).
		AddAnnotationsInMap(sharding.Template.Annotations).
		AddAnnotationsInMap(compDef.Spec.Annotations).
		PutData(constant.AccountNameForSecret, []byte(accountName)).
		PutData(constant.AccountPasswdForSecret, password).
		SetImmutable(true).
		GetObject()
	return secret, nil
}

func (t *clusterShardingAccountTransformer) rewriteSystemAccount(transCtx *clusterTransformContext, shardingName, accountName string) {
	var (
		cluster = transCtx.Cluster
	)
	newAccount := appsv1.ComponentSystemAccount{
		Name:     accountName,
		Disabled: ptr.To(false), // default to false
		SecretRef: &appsv1.ProvisionSecretRef{
			Name:      shardingAccountSecretName(cluster.Name, shardingName, accountName),
			Namespace: cluster.Namespace,
		},
	}

	// update sharding
	for i, sharding := range transCtx.shardings {
		if sharding.Name == shardingName {
			exist := false
			for j, account := range sharding.Template.SystemAccounts {
				if account.Name == accountName {
					newAccount.Disabled = account.Disabled
					transCtx.shardings[i].Template.SystemAccounts[j] = newAccount
					exist = true
					break
				}
			}
			if !exist {
				transCtx.shardings[i].Template.SystemAccounts =
					append(transCtx.shardings[i].Template.SystemAccounts, newAccount)
			}
			break
		}
	}

	// update sharding components
	shardingComps := transCtx.shardingComps[shardingName]
	for i := range shardingComps {
		shardingComps[i].SystemAccounts = append(shardingComps[i].SystemAccounts, newAccount)
	}
	transCtx.shardingComps[shardingName] = shardingComps
}

func shardingAccountSecretName(cluster, sharding, account string) string {
	return fmt.Sprintf("%s-%s-%s", cluster, sharding, account)
}
