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

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
	lorry "github.com/apecloud/kubeblocks/pkg/lorry/client"
	lorryModel "github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

const (
	accountProvisionConditionType             = "SystemAccountProvision"
	accountProvisionConditionReasonInProgress = "InProgress"
	accountProvisionConditionReasonDone       = "AllProvisioned"
)

// componentAccountProvisionTransformer provisions component system accounts.
type componentAccountProvisionTransformer struct{}

var _ graph.Transformer = &componentAccountProvisionTransformer{}

func (t *componentAccountProvisionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}
	if common.IsCompactMode(transCtx.ComponentOrig.Annotations) {
		transCtx.V(1).Info("Component is in compact mode, no need to create component account related objects",
			"component", client.ObjectKeyFromObject(transCtx.ComponentOrig))
		return nil
	}

	if len(transCtx.SynthesizeComponent.SystemAccounts) == 0 {
		return nil
	}
	if transCtx.Component.Status.Phase != appsv1alpha1.RunningClusterCompPhase {
		return nil
	}
	cond, provisioned := t.isProvisioned(transCtx)
	if provisioned {
		return nil
	}

	lifecycleActions := transCtx.CompDef.Spec.LifecycleActions
	if lifecycleActions == nil || lifecycleActions.AccountProvision == nil {
		return nil
	}
	// TODO: support custom handler for account
	// TODO: build lorry client if accountProvision is built-in
	lorryCli, err := t.buildLorryClient(transCtx)
	if err != nil {
		return err
	}
	if controllerutil.IsNil(lorryCli) {
		return nil
	}
	for _, account := range transCtx.SynthesizeComponent.SystemAccounts {
		if t.isAccountProvisioned(cond, account) {
			continue
		}
		if err = t.provisionAccount(transCtx, cond, lorryCli, account); err != nil {
			t.markProvisionAsFailed(transCtx, &cond, err)
			return err
		}
		t.markAccountProvisioned(&cond, account)
	}
	t.markProvisioned(transCtx, cond)

	return nil
}

func (t *componentAccountProvisionTransformer) isProvisioned(transCtx *componentTransformContext) (metav1.Condition, bool) {
	for _, cond := range transCtx.Component.Status.Conditions {
		if cond.Type == accountProvisionConditionType {
			if cond.Status == metav1.ConditionTrue {
				return cond, true
			}
			return cond, false
		}
	}
	return metav1.Condition{
		Type:               accountProvisionConditionType,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: transCtx.Component.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             accountProvisionConditionReasonInProgress,
		Message:            "",
	}, false
}

func (t *componentAccountProvisionTransformer) markProvisionAsFailed(transCtx *componentTransformContext, cond *metav1.Condition, err error) {
	cond.Status = metav1.ConditionFalse
	cond.ObservedGeneration = transCtx.Component.Generation
	cond.LastTransitionTime = metav1.Now()
	// cond.Reason = err.Error() // TODO: error
}

func (t *componentAccountProvisionTransformer) markProvisioned(transCtx *componentTransformContext, cond metav1.Condition) {
	cond.Status = metav1.ConditionTrue
	cond.ObservedGeneration = transCtx.Component.Generation
	cond.LastTransitionTime = metav1.Now()
	cond.Reason = accountProvisionConditionReasonDone

	conditions := transCtx.Component.Status.Conditions
	if conditions == nil {
		conditions = make([]metav1.Condition, 0)
	}
	existed := false
	for i, c := range conditions {
		if c.Type == cond.Type {
			existed = true
			conditions[i] = cond
		}
	}
	if !existed {
		conditions = append(conditions, cond)
	}
	transCtx.Component.Status.Conditions = conditions
}

func (t *componentAccountProvisionTransformer) isAccountProvisioned(cond metav1.Condition, account appsv1alpha1.SystemAccount) bool {
	if len(cond.Message) == 0 {
		return false
	}
	accounts := strings.Split(cond.Message, ",")
	return slices.Contains(accounts, account.Name)
}

func (t *componentAccountProvisionTransformer) markAccountProvisioned(cond *metav1.Condition, account appsv1alpha1.SystemAccount) {
	if len(cond.Message) == 0 {
		cond.Message = account.Name
		return
	}
	accounts := strings.Split(cond.Message, ",")
	if slices.Contains(accounts, account.Name) {
		return
	}
	accounts = append(accounts, account.Name)
	cond.Message = strings.Join(accounts, ",")
}

func (t *componentAccountProvisionTransformer) buildLorryClient(transCtx *componentTransformContext) (lorry.Client, error) {
	synthesizedComp := transCtx.SynthesizeComponent

	roleName := ""
	for _, role := range synthesizedComp.Roles {
		if role.Serviceable && role.Writable {
			roleName = role.Name
		}
	}
	if roleName == "" {
		return nil, nil
	}

	podList, err := component.GetComponentPodListWithRole(transCtx.Context, transCtx.Client, *transCtx.Cluster, synthesizedComp.Name, roleName)
	if err != nil {
		return nil, err
	}
	if podList == nil || len(podList.Items) == 0 {
		return nil, fmt.Errorf("unable to find appropriate pods to create accounts")
	}

	lorryCli, err := lorry.NewClient(podList.Items[0])
	if err != nil {
		return nil, err
	}
	return lorryCli, nil
}

func (t *componentAccountProvisionTransformer) provisionAccount(transCtx *componentTransformContext,
	cond metav1.Condition, lorryCli lorry.Client, account appsv1alpha1.SystemAccount) error {

	synthesizedComp := transCtx.SynthesizeComponent
	secret, err := t.getAccountSecret(transCtx, synthesizedComp, account)
	if err != nil {
		return err
	}

	username, password := secret.Data[constant.AccountNameForSecret], secret.Data[constant.AccountPasswdForSecret]
	if len(username) == 0 || len(password) == 0 {
		return nil
	}

	// TODO: re-define the role
	return lorryCli.CreateUser(transCtx.Context, string(username), string(password), string(lorryModel.SuperUserRole))
}

func (t *componentAccountProvisionTransformer) getAccountSecret(ctx graph.TransformContext,
	synthesizeComp *component.SynthesizedComponent, account appsv1alpha1.SystemAccount) (*corev1.Secret, error) {
	secretKey := types.NamespacedName{
		Namespace: synthesizeComp.Namespace,
		Name:      constant.GenerateAccountSecretName(synthesizeComp.ClusterName, synthesizeComp.Name, account.Name),
	}
	secret := &corev1.Secret{}
	if err := ctx.GetClient().Get(ctx.GetContext(), secretKey, secret); err != nil {
		return nil, err
	}
	return secret, nil
}
