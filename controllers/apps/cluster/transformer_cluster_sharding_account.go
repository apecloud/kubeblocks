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
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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
	running, err := t.getSystemAccountSecret(transCtx, sharding, accountName)
	if err != nil {
		return err
	}
	var revision string
	if running == nil {
		obj, err := t.newSystemAccountSecret(transCtx, sharding, accountName)
		if err != nil {
			return err
		}
		revision = obj.Annotations[constant.SecretRevisionAnnotationKey]
		graphCli.Create(dag, obj)
	} else {
		updated, recreate, err := t.updateSystemAccountSecret(transCtx, sharding, accountName, running)
		if err != nil {
			return err
		}
		if recreate {
			graphCli.Delete(dag, running)
			return intctrlutil.NewRequeueError(0,
				fmt.Sprintf("recreate immutable shared system account secret %s/%s", running.Namespace, running.Name))
		}
		revision = running.Annotations[constant.SecretRevisionAnnotationKey]
		if updated != nil {
			revision = updated.Annotations[constant.SecretRevisionAnnotationKey]
			graphCli.Update(dag, running, updated)
		}
	}

	t.rewriteSystemAccount(transCtx, sharding.Name, accountName, revision)

	return nil
}

func (t *clusterShardingAccountTransformer) getSystemAccountSecret(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding, accountName string) (*corev1.Secret, error) {
	var (
		cluster = transCtx.Cluster
	)
	secretKey := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      shardingAccountSecretName(cluster.Name, sharding.Name, accountName),
	}
	secret := &corev1.Secret{}
	err := transCtx.GetClient().Get(transCtx.GetContext(), secretKey, secret)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	managedLabels := constant.GetClusterLabels(cluster.Name, map[string]string{
		constant.KBAppShardingNameLabelKey: sharding.Name,
	})
	for key, value := range managedLabels {
		if secret.Labels[key] != value {
			return nil, fmt.Errorf("secret %s/%s already exists but is not managed by sharding %s",
				secret.Namespace, secret.Name, sharding.Name)
		}
	}
	if value, ok := secret.Labels[constant.SystemAccountLabelKey]; ok && value != accountName {
		return nil, fmt.Errorf("secret %s/%s is managed for system account %s, not %s",
			secret.Namespace, secret.Name, value, accountName)
	}
	return secret, nil
}

func (t *clusterShardingAccountTransformer) updateSystemAccountSecret(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding, accountName string, running *corev1.Secret) (*corev1.Secret, bool, error) {
	account, err := t.definedSystemAccount(transCtx, sharding, accountName)
	if err != nil {
		return nil, false, err
	}
	if account.SecretRef == nil {
		revision := running.Annotations[constant.SecretRevisionAnnotationKey]
		if revision != "" {
			return nil, false, nil
		}
		// Backfill a stable revision for managed Secrets created before the revision handshake.
		revision = string(uuid.NewUUID())
		updated := running.DeepCopy()
		setSecretRevision(updated, revision)
		return updated, false, nil
	}

	password, source, passwordKey, err := t.getPasswordSource(transCtx, account.SecretRef)
	if err != nil {
		return nil, false, err
	}
	if err := common.ValidateSystemAccountPassword(password); err != nil {
		return nil, false, err
	}
	runningPassword, ok := running.Data[constant.AccountPasswdForSecret]
	passwordChanged := !ok || !bytes.Equal(runningPassword, password)

	if passwordChanged && ptr.Deref(running.Immutable, false) {
		return nil, true, nil
	}

	revision := sourceSecretRevision(source, passwordKey)
	updated := running.DeepCopy()
	setSecretRevision(updated, revision)
	if passwordChanged {
		if updated.Data == nil {
			updated.Data = map[string][]byte{}
		}
		updated.Data[constant.AccountPasswdForSecret] = password
	}
	if passwordChanged || running.Annotations[constant.SecretRevisionAnnotationKey] != revision {
		return updated, false, nil
	}
	return nil, false, nil
}

func (t *clusterShardingAccountTransformer) newSystemAccountSecret(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding, accountName string) (*corev1.Secret, error) {
	account, err := t.definedSystemAccount(transCtx, sharding, accountName)
	if err != nil {
		return nil, err
	}
	password, source, passwordKey, err := t.buildPassword(transCtx, account)
	if err != nil {
		return nil, err
	}
	if err := common.ValidateSystemAccountPassword(password); err != nil {
		return nil, err
	}
	revision := string(uuid.NewUUID())
	if source != nil {
		revision = sourceSecretRevision(source, passwordKey)
	}
	return t.newAccountSecretWithPassword(transCtx, sharding, accountName, password, revision)
}

type synthesizedShardingSystemAccount struct {
	appsv1.SystemAccount
	SecretRef *appsv1.ProvisionSecretRef
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
		resolved := synthesizedShardingSystemAccount{SystemAccount: *account}
		if compAccount != nil {
			if compAccount.PasswordConfig != nil {
				resolved.PasswordConfig = compAccount.PasswordConfig.DeepCopy()
			}
			resolved.SecretRef = compAccount.SecretRef
		}
		return resolved
	}

	for i, account := range compDef.Spec.SystemAccounts {
		if account.Name == accountName {
			return override(compDef.Spec.SystemAccounts[i].DeepCopy()), nil
		}
	}
	return synthesizedShardingSystemAccount{}, fmt.Errorf("system account %s not found in component definition %s", accountName, compDef.Name)
}

func (t *clusterShardingAccountTransformer) buildPassword(transCtx *clusterTransformContext,
	account synthesizedShardingSystemAccount) ([]byte, *corev1.Secret, string, error) {
	if account.SecretRef != nil {
		return t.getPasswordSource(transCtx, account.SecretRef)
	}
	password, err := common.GenerateSystemAccountPassword(account.SystemAccount)
	return []byte(password), nil, "", err
}

func (t *clusterShardingAccountTransformer) getPasswordSource(transCtx *clusterTransformContext,
	secretRef *appsv1.ProvisionSecretRef) ([]byte, *corev1.Secret, string, error) {
	if secretRef.Namespace != "" && secretRef.Namespace != transCtx.Cluster.Namespace {
		return nil, nil, "", fmt.Errorf("cross-namespace secretRef is not supported for shared sharding system accounts")
	}
	secretKey := types.NamespacedName{Namespace: transCtx.Cluster.Namespace, Name: secretRef.Name}
	secret := &corev1.Secret{}
	if err := transCtx.GetClient().Get(transCtx.GetContext(), secretKey, secret); err != nil {
		return nil, nil, "", err
	}

	passwordKey := constant.AccountPasswdForSecret
	if secretRef.Password != "" {
		passwordKey = secretRef.Password
	}
	password, ok := secret.Data[passwordKey]
	if !ok {
		return nil, nil, "", fmt.Errorf("referenced account secret has no required credential field: %s", passwordKey)
	}
	return password, secret, passwordKey, nil
}

func (t *clusterShardingAccountTransformer) newAccountSecretWithPassword(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding, accountName string, password []byte, revision string) (*corev1.Secret, error) {
	var (
		cluster = transCtx.Cluster
	)
	compDef := transCtx.componentDefs[sharding.Template.ComponentDef]
	shardingLabels := map[string]string{
		constant.KBAppShardingNameLabelKey: sharding.Name,
	}
	secret := builder.NewSecretBuilder(cluster.Namespace, shardingAccountSecretName(cluster.Name, sharding.Name, accountName)).
		// Priority: static < dynamic < built-in
		AddLabelsInMap(compDef.Spec.Labels).
		AddLabelsInMap(sharding.Template.Labels).
		AddLabelsInMap(constant.GetClusterLabels(cluster.Name, shardingLabels)).
		AddLabels(constant.SystemAccountLabelKey, accountName).
		AddAnnotationsInMap(sharding.Template.Annotations).
		AddAnnotationsInMap(compDef.Spec.Annotations).
		AddAnnotations(constant.SecretRevisionAnnotationKey, revision).
		PutData(constant.AccountNameForSecret, []byte(accountName)).
		PutData(constant.AccountPasswdForSecret, password).
		GetObject()
	return secret, nil
}

func sourceSecretRevision(source *corev1.Secret, passwordKey string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(source.UID))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(source.ResourceVersion))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(passwordKey))
	return hex.EncodeToString(hash.Sum(nil))
}

func setSecretRevision(secret *corev1.Secret, revision string) {
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	secret.Annotations[constant.SecretRevisionAnnotationKey] = revision
}

func (t *clusterShardingAccountTransformer) rewriteSystemAccount(transCtx *clusterTransformContext, shardingName, accountName, revision string) {
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
		SecretRefRevision: revision,
	}

	// update sharding
	for i, sharding := range transCtx.shardings {
		if sharding.Name == shardingName {
			for _, account := range sharding.Template.SystemAccounts {
				if account.Name == accountName {
					newAccount.Disabled = account.Disabled
					break
				}
			}
			transCtx.shardings[i].Template.SystemAccounts =
				upsertSystemAccount(transCtx.shardings[i].Template.SystemAccounts, newAccount)
			break
		}
	}

	// update sharding components
	shardingComps := transCtx.shardingComps[shardingName]
	for i := range shardingComps {
		shardingComps[i].SystemAccounts = upsertSystemAccount(shardingComps[i].SystemAccounts, newAccount)
	}
	transCtx.shardingComps[shardingName] = shardingComps
}

func upsertSystemAccount(accounts []appsv1.ComponentSystemAccount,
	account appsv1.ComponentSystemAccount) []appsv1.ComponentSystemAccount {
	for i := range accounts {
		if accounts[i].Name == account.Name {
			accounts[i] = account
			return accounts
		}
	}
	return append(accounts, account)
}

func shardingAccountSecretName(cluster, sharding, account string) string {
	return fmt.Sprintf("%s-%s-%s", cluster, sharding, account)
}
