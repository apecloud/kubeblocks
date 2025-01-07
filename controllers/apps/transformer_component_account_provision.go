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
	"slices"
	"strings"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/component/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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

	comp := transCtx.Component
	compDef := transCtx.CompDef

	// provision accounts only when the component is running
	if comp.Status.Phase != appsv1.RunningComponentPhase {
		return nil
	}

	// has no lifecycle actions defined, skip the account provision
	lifecycleActions := compDef.Spec.LifecycleActions
	if lifecycleActions == nil || lifecycleActions.AccountProvision == nil {
		return nil
	}

	accounts, _ := synthesizeSystemAccounts(compDef.Spec.SystemAccounts, comp.Spec.SystemAccounts, true)

	secrets, err1 := listSystemAccountObjects(ctx, transCtx.SynthesizeComponent)
	if err1 != nil {
		return err1
	}
	protoNameSet := sets.New(maps.Keys(secrets)...)

	cond := t.provisionCond(transCtx)
	provisionedNameSet := t.getProvisionedAccounts(cond)

	createSet, deleteSet, updateSet := setDiff(provisionedNameSet, protoNameSet)
	if len(createSet) == 0 && len(deleteSet) == 0 && len(updateSet) == 0 {
		return nil
	}

	lfa, err2 := t.lifecycleAction(transCtx)
	if err2 != nil {
		return err2
	}

	var err3 error
	condCopy := cond.DeepCopy()
	for _, name := range sets.List(createSet) {
		if err := t.createAccount(transCtx, lfa, &cond, accounts[name], secrets[name]); err != nil {
			if err3 == nil {
				err3 = err
			}
		}
	}

	for _, name := range sets.List(deleteSet) {
		if err := t.deleteAccount(transCtx, lfa, &cond, accounts[name]); err != nil {
			if err3 == nil {
				err3 = err
			}
		}
	}

	for _, name := range sets.List(updateSet) {
		if err := t.updateAccount(transCtx, lfa, &cond, accounts[name], secrets[name]); err != nil {
			if err3 == nil {
				err3 = err
			}
		}
	}

	t.provisionCondDone(transCtx, condCopy, &cond, err3)

	return err3
}

func (t *componentAccountProvisionTransformer) lifecycleAction(transCtx *componentTransformContext) (lifecycle.Lifecycle, error) {
	synthesizedComp := transCtx.SynthesizeComponent
	pods, err := component.ListOwnedPods(transCtx.Context, transCtx.Client,
		synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name)
	if err != nil {
		return nil, err
	}
	lfa, err := lifecycle.New(transCtx.SynthesizeComponent, nil, pods...)
	if err != nil {
		return nil, err
	}
	return lfa, nil
}

func (t *componentAccountProvisionTransformer) createAccount(transCtx *componentTransformContext,
	lfa lifecycle.Lifecycle, cond *metav1.Condition, account synthesizedSystemAccount, secret *corev1.Secret) error {
	var (
		err error
	)

	// The secret of an initial account should be injected into the container through
	// specific account&password environment variables name supported by the engine.
	// When the engine starts up, it will automatically load and create this account.
	if !account.InitAccount {
		// TODO: restore account secret from backup.
		if transCtx.SynthesizeComponent.Annotations[constant.RestoreFromBackupAnnotationKey] == "" {
			// provision account when the component is not recovered from backup
			err = t.provision(transCtx, lfa, account.Statement.Create, secret)
		}
	}

	if err == nil {
		// TODO: how about the password restored from backup?
		t.updateProvisionedAccount(cond, account.Name, secret.Annotations[systemAccountHashAnnotation])
	}
	return err
}

func (t *componentAccountProvisionTransformer) deleteAccount(transCtx *componentTransformContext,
	lfa lifecycle.Lifecycle, cond *metav1.Condition, account synthesizedSystemAccount) error {
	if account.Statement == nil || len(account.Statement.Delete) == 0 {
		return fmt.Errorf("has no delete statement defined for system account: %s", account.Name)
	}

	err := lfa.AccountProvision(transCtx.Context, transCtx.Client, nil, account.Statement.Delete, account.Name, "")
	if lifecycle.IgnoreNotDefined(err) == nil {
		t.removeProvisionedAccount(cond, account.Name)
	}
	return lifecycle.IgnoreNotDefined(err)
}

func (t *componentAccountProvisionTransformer) updateAccount(transCtx *componentTransformContext,
	lfa lifecycle.Lifecycle, cond *metav1.Condition, account synthesizedSystemAccount, secret *corev1.Secret) error {
	hashedPassword := t.hashedPasswordFromCond(cond, account.Name)
	if hashedPassword == "" {
		return nil // passwords that generated by KB or restored from backup, do not support updating?
	}
	if verifySystemAccountPassword(secret, []byte(hashedPassword)) {
		return nil // the password is not changed
	}

	if account.Statement == nil || len(account.Statement.Update) == 0 {
		return fmt.Errorf("has no update statement defined for system account: %s", account.Name)
	}

	// TODO: how to notify other apps to update the new password?

	err := t.provision(transCtx, lfa, account.Statement.Update, secret)
	if err == nil {
		t.updateProvisionedAccount(cond, account.Name, secret.Annotations[systemAccountHashAnnotation])
	}
	return err
}

func (t *componentAccountProvisionTransformer) provision(transCtx *componentTransformContext,
	lfa lifecycle.Lifecycle, statement string, secret *corev1.Secret) error {
	username, password := secret.Data[constant.AccountNameForSecret], secret.Data[constant.AccountPasswdForSecret]
	if len(username) == 0 || len(password) == 0 {
		return nil
	}
	err := lfa.AccountProvision(transCtx.Context, transCtx.Client, nil, statement, string(username), string(password))
	return lifecycle.IgnoreNotDefined(err)
}

func (t *componentAccountProvisionTransformer) provisionCond(transCtx *componentTransformContext) metav1.Condition {
	for _, cond := range transCtx.Component.Status.Conditions {
		if cond.Type == accountProvisionConditionType {
			return cond
		}
	}
	return metav1.Condition{
		Type:               accountProvisionConditionType,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: transCtx.Component.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             accountProvisionConditionReasonInProgress,
		Message:            "",
	}
}

func (t *componentAccountProvisionTransformer) provisionCondDone(transCtx *componentTransformContext,
	condCopy, cond *metav1.Condition, err error) {
	if err == nil {
		cond.Status = metav1.ConditionTrue
		cond.Reason = accountProvisionConditionReasonDone
	} else {
		cond.Status = metav1.ConditionFalse
		// cond.Reason = err.Error() // TODO: error
	}

	if !reflect.DeepEqual(cond, condCopy) {
		cond.LastTransitionTime = metav1.Now()
	}
	cond.ObservedGeneration = transCtx.Component.Generation

	conditions := transCtx.Component.Status.Conditions
	if conditions == nil {
		conditions = make([]metav1.Condition, 0)
	}
	existed := false
	for i, c := range conditions {
		if c.Type == cond.Type {
			existed = true
			conditions[i] = *cond
		}
	}
	if !existed {
		conditions = append(conditions, *cond)
	}
	transCtx.Component.Status.Conditions = conditions
}

func (t *componentAccountProvisionTransformer) getProvisionedAccounts(cond metav1.Condition) sets.Set[string] {
	accounts := sets.New[string]()
	if len(cond.Message) > 0 {
		for _, e := range strings.Split(cond.Message, ",") {
			if len(e) > 0 {
				accounts.Insert(strings.Split(e, ":")[0])
			}
		}
	}
	return accounts
}

func (t *componentAccountProvisionTransformer) updateProvisionedAccount(cond *metav1.Condition, account, hashedPassword string) {
	accounts := make([]string, 0)
	if len(cond.Message) > 0 {
		accounts = strings.Split(cond.Message, ",")
	}
	idx := slices.IndexFunc(accounts, func(s string) bool {
		return strings.HasPrefix(s, fmt.Sprintf("%s:", account))
	})
	if idx >= 0 {
		accounts[idx] = fmt.Sprintf("%s:%s", account, hashedPassword)
	} else {
		accounts = append(accounts, fmt.Sprintf("%s:%s", account, hashedPassword))
	}
	cond.Message = strings.Join(accounts, ",")
}

func (t *componentAccountProvisionTransformer) removeProvisionedAccount(cond *metav1.Condition, account string) {
	accounts := make([]string, 0)
	if len(cond.Message) > 0 {
		accounts = strings.Split(cond.Message, ",")
	}
	accounts = slices.DeleteFunc(accounts, func(s string) bool {
		return strings.HasPrefix(s, fmt.Sprintf("%s:", account))
	})
	cond.Message = strings.Join(accounts, ",")
}

func (t *componentAccountProvisionTransformer) hashedPasswordFromCond(cond *metav1.Condition, account string) string {
	accounts := make([]string, 0)
	if len(cond.Message) > 0 {
		accounts = strings.Split(cond.Message, ",")
	}
	idx := slices.IndexFunc(accounts, func(s string) bool {
		return strings.HasPrefix(s, fmt.Sprintf("%s:", account))
	})
	if idx >= 0 {
		val := strings.Split(accounts[idx], ":")
		if len(val) == 2 {
			return val[1]
		}
	}
	return ""
}
