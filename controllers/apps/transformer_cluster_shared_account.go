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
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// clusterSharedAccountTransformer handles the shared system accounts between components in a cluster.
type clusterSharedAccountTransformer struct{}

var _ graph.Transformer = &clusterSharedAccountTransformer{}

func (t *clusterSharedAccountTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.Cluster) {
		return nil
	}

	if common.IsCompactMode(transCtx.Cluster.Annotations) {
		transCtx.V(1).Info("Cluster is in compact mode, no need to create account related objects", "cluster", client.ObjectKeyFromObject(transCtx.Cluster))
		return nil
	}

	// currently, we only support shared system account for sharding components
	graphCli, _ := transCtx.Client.(model.GraphClient)
	return t.reconcileShardingsSharedAccounts(transCtx, graphCli, dag)
}

func (t *clusterSharedAccountTransformer) reconcileShardingsSharedAccounts(transCtx *clusterTransformContext,
	graphCli model.GraphClient, dag *graph.DAG) error {
	if len(transCtx.Cluster.Spec.ShardingSpecs) == 0 {
		return nil
	}

	for _, shardingSpec := range transCtx.Cluster.Spec.ShardingSpecs {
		if len(shardingSpec.Template.SystemAccounts) == 0 {
			return nil
		}
		for i, account := range shardingSpec.Template.SystemAccounts {
			needCreate, err := t.needCreateSharedAccount(transCtx, &account, shardingSpec)
			if err != nil {
				return err
			}
			if !needCreate {
				continue
			}
			if err := t.createNConvertToSharedAccountSecret(transCtx, &account, shardingSpec, graphCli, dag); err != nil {
				return err
			}
			shardingSpec.Template.SystemAccounts[i] = account
		}
	}
	return nil
}

func (t *clusterSharedAccountTransformer) needCreateSharedAccount(transCtx *clusterTransformContext,
	account *appsv1alpha1.ComponentSystemAccount, shardingSpec appsv1alpha1.ShardingSpec) (bool, error) {
	// respect the secretRef if it is set
	if account.SecretRef != nil {
		return false, nil
	}

	// if seed is not set, we consider it does not need to share the same account secret
	// TODO: wo may support another way to judge if the need to create shared account secret in the future
	if account.PasswordConfig == nil || len(account.PasswordConfig.Seed) == 0 {
		return false, nil
	}

	secretName := constant.GenerateShardingSharedAccountSecretName(transCtx.Cluster.Name, shardingSpec.Name, account.Name)
	if secret, err := t.checkShardingSharedAccountSecretExist(transCtx, transCtx.Cluster, secretName); err != nil {
		return false, err
	} else if secret != nil {
		return false, nil
	}

	return true, nil
}

func (t *clusterSharedAccountTransformer) createNConvertToSharedAccountSecret(transCtx *clusterTransformContext,
	account *appsv1alpha1.ComponentSystemAccount, shardingSpec appsv1alpha1.ShardingSpec, graphCli model.GraphClient, dag *graph.DAG) error {
	// Create the shared account secret if it does not exist
	secretName := constant.GenerateShardingSharedAccountSecretName(transCtx.Cluster.Name, shardingSpec.Name, account.Name)
	secret, err := t.buildAccountSecret(transCtx.Cluster, *account, shardingSpec.Name, secretName)
	if err != nil {
		return err
	}
	graphCli.Create(dag, secret)

	// Update account SecretRef to the shared secret
	account.SecretRef = &appsv1alpha1.ProvisionSecretRef{
		Name:      secret.Name,
		Namespace: transCtx.Cluster.Namespace,
	}

	return nil
}

func (t *clusterSharedAccountTransformer) checkShardingSharedAccountSecretExist(transCtx *clusterTransformContext,
	cluster *appsv1alpha1.Cluster, secretName string) (*corev1.Secret, error) {
	secretKey := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      secretName,
	}
	secret := &corev1.Secret{}
	err := transCtx.GetClient().Get(transCtx.GetContext(), secretKey, secret)
	switch {
	case err == nil:
		return secret, nil
	case apierrors.IsNotFound(err):
		return nil, nil
	default:
		return nil, err
	}
}

func (t *clusterSharedAccountTransformer) buildAccountSecret(cluster *appsv1alpha1.Cluster,
	account appsv1alpha1.ComponentSystemAccount, shardingName, secretName string) (*corev1.Secret, error) {
	password := []byte(factory.GetRestoreSystemAccountPassword(cluster.Annotations, shardingName, account.Name))
	if len(password) == 0 {
		password = t.generatePassword(account)
	}
	return t.buildAccountSecretWithPassword(cluster, account, shardingName, secretName, password)
}

func (t *clusterSharedAccountTransformer) generatePassword(account appsv1alpha1.ComponentSystemAccount) []byte {
	config := account.PasswordConfig
	passwd, _ := common.GeneratePassword((int)(config.Length), (int)(config.NumDigits), (int)(config.NumSymbols), false, "")
	switch config.LetterCase {
	case appsv1alpha1.UpperCases:
		passwd = strings.ToUpper(passwd)
	case appsv1alpha1.LowerCases:
		passwd = strings.ToLower(passwd)
	}
	return []byte(passwd)
}

func (t *clusterSharedAccountTransformer) buildAccountSecretWithPassword(cluster *appsv1alpha1.Cluster,
	account appsv1alpha1.ComponentSystemAccount, shardingName, secretName string, password []byte) (*corev1.Secret, error) {
	labels := constant.GetShardingWellKnownLabels(cluster.Name, shardingName)
	secret := builder.NewSecretBuilder(cluster.Namespace, secretName).
		AddLabelsInMap(labels).
		AddLabels(constant.ClusterAccountLabelKey, account.Name).
		PutData(constant.AccountNameForSecret, []byte(account.Name)).
		PutData(constant.AccountPasswdForSecret, password).
		SetImmutable(true).
		GetObject()
	return secret, nil
}
