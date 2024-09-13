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
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
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
	if len(transCtx.Cluster.Spec.ShardingSpecs) == 0 {
		return nil
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	for _, shardingSpec := range transCtx.Cluster.Spec.ShardingSpecs {
		if len(shardingSpec.Template.SystemAccounts) == 0 {
			continue
		}
		for i, account := range shardingSpec.Template.SystemAccounts {
			// respect the secretRef if it is set
			if account.SecretRef != nil {
				continue
			}

			// if seed is not set, we consider it does not need to share the same account secret
			if account.PasswordConfig != nil && len(account.PasswordConfig.Seed) == 0 {
				continue
			}

			// check if the shared account secret already exists
			secretName := constant.GenerateShardingSharedAccountSecretName(transCtx.Cluster.Name, shardingSpec.Name, account.Name)
			secret, err := t.checkShardingSharedAccountSecretExist(transCtx, transCtx.Cluster, secretName, graphCli, dag)
			if err != nil {
				return err
			}
			if secret != nil {
				continue
			}

			// create the shared account secret if not exist
			secret, err = t.buildAccountSecret(transCtx.Cluster, account, shardingSpec.Name, secretName)
			if err != nil {
				return err
			}
			graphCli.Create(dag, secret)

			// update account secretRef to the shared secret and reset the password seed
			shardingSpec.Template.SystemAccounts[i].SecretRef = &appsv1alpha1.ProvisionSecretRef{
				Name:      secret.Name,
				Namespace: transCtx.Cluster.Namespace,
			}
			shardingSpec.Template.SystemAccounts[i].PasswordConfig.Seed = ""
		}
	}
	return nil
}

func (t *clusterSharedAccountTransformer) checkShardingSharedAccountSecretExist(transCtx *clusterTransformContext,
	cluster *appsv1alpha1.Cluster, secretName string, graphCli model.GraphClient, dag *graph.DAG) (*corev1.Secret, error) {
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
		// check if secret exist in dag
		dagSecrets := graphCli.FindAll(dag, &corev1.Secret{})
		for _, v := range dagSecrets {
			ds := v.(*corev1.Secret)
			if ds.Name == secretName {
				return ds, nil
			}
		}
		return nil, nil
	default:
		return nil, err
	}
}

func (t *clusterSharedAccountTransformer) buildAccountSecret(cluster *appsv1alpha1.Cluster,
	account appsv1alpha1.ComponentSystemAccount, shardingName, secretName string) (*corev1.Secret, error) {
	password := t.generatePassword(account)
	return t.buildAccountSecretWithPassword(cluster, account, shardingName, secretName, password)
}

func (t *clusterSharedAccountTransformer) getPasswordFromSecret(ctx graph.TransformContext, account appsv1alpha1.SystemAccount) ([]byte, error) {
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
