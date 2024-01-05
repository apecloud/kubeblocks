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
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// componentRBACTransformer puts the RBAC objects at the beginning of the DAG
type componentRBACTransformer struct{}

var _ graph.Transformer = &componentRBACTransformer{}

func (t *componentRBACTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}
	if common.IsCompactMode(transCtx.ComponentOrig.Annotations) {
		transCtx.V(1).Info("Component is in compact mode, no need to create rbac related objects",
			"component", client.ObjectKeyFromObject(transCtx.ComponentOrig))
		return nil
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)

	serviceAccount, needCRB, err := buildServiceAccount(transCtx)
	if err != nil {
		return err
	}
	if serviceAccount == nil {
		transCtx.Logger.V(1).Info("buildServiceAccounts returns serviceAccount nil")
		return nil
	}

	if isServiceAccountExist(transCtx, serviceAccount.Name) {
		return nil
	}

	if !viper.GetBool(constant.EnableRBACManager) {
		transCtx.Logger.V(1).Info("rbac manager is disabled")
		transCtx.EventRecorder.Event(transCtx.Cluster, corev1.EventTypeWarning,
			string(ictrlutil.ErrorTypeNotFound), fmt.Sprintf("ServiceAccount %s is not exist", serviceAccount.Name))
		return ictrlutil.NewRequeueError(time.Second, "RBAC manager is disabled, but service account is not exist")
	}

	var parent client.Object
	rb := factory.BuildRoleBinding(transCtx.Cluster, serviceAccount.Name)
	graphCli.Create(dag, rb)
	parent = rb
	if needCRB {
		crb := factory.BuildClusterRoleBinding(transCtx.Cluster, serviceAccount.Name)
		graphCli.Create(dag, crb)
		graphCli.DependOn(dag, parent, crb)
		parent = crb
	}

	createServiceAccount(serviceAccount, graphCli, dag, parent)
	rsmList := graphCli.FindAll(dag, &workloads.ReplicatedStateMachine{})
	for _, rsm := range rsmList {
		// serviceAccount must be created before workload
		graphCli.DependOn(dag, rsm, serviceAccount)
	}

	return nil
}

func isProbesEnabled(compDef *appsv1alpha1.ComponentDefinition) bool {
	// TODO(component): lorry
	return compDef.Spec.LifecycleActions != nil && compDef.Spec.LifecycleActions.RoleProbe != nil
}

func isDataProtectionEnabled(backupTpl *appsv1alpha1.BackupPolicyTemplate, cluster *appsv1alpha1.Cluster, comp *appsv1alpha1.Component) bool {
	if backupTpl != nil {
		for _, policy := range backupTpl.Spec.BackupPolicies {
			// TODO(component): the definition of component referenced by backup policy.
			if policy.ComponentDefRef == comp.Spec.CompDef {
				return true
			}
			// TODO: Compatibility handling, remove it if the clusterDefinition is removed.
			for _, v := range cluster.Spec.ComponentSpecs {
				if v.ComponentDefRef == policy.ComponentDefRef {
					return true
				}
			}
		}
	}
	return false
}

func isVolumeProtectionEnabled(compDef *appsv1alpha1.ComponentDefinition) bool {
	for _, vol := range compDef.Spec.Volumes {
		if vol.HighWatermark > 0 && vol.HighWatermark < 100 {
			return true
		}
	}
	return false
}

func isServiceAccountExist(transCtx *componentTransformContext, serviceAccountName string) bool {
	synthesizedComp := transCtx.SynthesizeComponent
	namespaceName := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      serviceAccountName,
	}
	sa := &corev1.ServiceAccount{}
	if err := transCtx.Client.Get(transCtx.Context, namespaceName, sa); err != nil {
		// KubeBlocks will create a rolebinding only if it has RBAC access priority and
		// the rolebinding is not already present.
		if errors.IsNotFound(err) {
			transCtx.Logger.V(1).Info("ServiceAccount not exists", "namespaceName", namespaceName)
			return false
		}
		transCtx.Logger.Error(err, "get ServiceAccount failed")
		return false
	}
	return true
}

func isClusterRoleBindingExist(transCtx *componentTransformContext, serviceAccountName string) bool {
	synthesizedComp := transCtx.SynthesizeComponent
	namespaceName := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      "kb-" + synthesizedComp.ClusterName,
	}
	crb := &rbacv1.ClusterRoleBinding{}
	if err := transCtx.Client.Get(transCtx.Context, namespaceName, crb); err != nil {
		// KubeBlocks will create a cluster role binding only if it has RBAC access priority and
		// the cluster role binding is not already present.
		if errors.IsNotFound(err) {
			transCtx.Logger.V(1).Info("ClusterRoleBinding not exists", "namespaceName", namespaceName)
			return false
		}
		transCtx.Logger.Error(err, fmt.Sprintf("get cluster role binding failed: %s", namespaceName))
		return false
	}

	if crb.RoleRef.Name != constant.RBACClusterRoleName {
		transCtx.Logger.V(1).Info("rbac manager: ClusterRole not match", "ClusterRole",
			constant.RBACClusterRoleName, "clusterrolebinding.RoleRef", crb.RoleRef.Name)
	}

	isServiceAccountMatch := false
	for _, sub := range crb.Subjects {
		if sub.Kind == rbacv1.ServiceAccountKind && sub.Name == serviceAccountName {
			isServiceAccountMatch = true
			break
		}
	}

	if !isServiceAccountMatch {
		transCtx.Logger.V(1).Info("rbac manager: ServiceAccount not match", "ServiceAccount",
			serviceAccountName, "clusterrolebinding.Subjects", crb.Subjects)
	}
	return true
}

func isRoleBindingExist(transCtx *componentTransformContext, serviceAccountName string) bool {
	synthesizedComp := transCtx.SynthesizeComponent
	namespaceName := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      "kb-" + synthesizedComp.ClusterName,
	}
	rb := &rbacv1.RoleBinding{}
	if err := transCtx.Client.Get(transCtx.Context, namespaceName, rb); err != nil {
		// KubeBlocks will create a role binding only if it has RBAC access priority and
		// the role binding is not already present.
		if errors.IsNotFound(err) {
			transCtx.Logger.V(1).Info("RoleBinding not exists", "namespaceName", namespaceName)
			return false
		}
		transCtx.Logger.Error(err, fmt.Sprintf("get role binding failed: %s", namespaceName))
		return false
	}

	if rb.RoleRef.Name != constant.RBACRoleName {
		transCtx.Logger.V(1).Info("rbac manager: ClusterRole not match", "ClusterRole",
			constant.RBACRoleName, "rolebinding.RoleRef", rb.RoleRef.Name)
	}

	isServiceAccountMatch := false
	for _, sub := range rb.Subjects {
		if sub.Kind == rbacv1.ServiceAccountKind && sub.Name == serviceAccountName {
			isServiceAccountMatch = true
			break
		}
	}

	if !isServiceAccountMatch {
		transCtx.Logger.V(1).Info("rbac manager: ServiceAccount not match", "ServiceAccount",
			serviceAccountName, "rolebinding.Subjects", rb.Subjects)
	}
	return true
}

func getDefaultBackupPolicyTemplate(transCtx *componentTransformContext, clusterDefName string) (*appsv1alpha1.BackupPolicyTemplate, error) {
	backupPolicyTPLs := &appsv1alpha1.BackupPolicyTemplateList{}
	if err := transCtx.Client.List(transCtx.Context, backupPolicyTPLs, client.MatchingLabels{constant.ClusterDefLabelKey: clusterDefName}); err != nil {
		return nil, err
	}
	if len(backupPolicyTPLs.Items) == 0 {
		return nil, nil
	}
	for _, item := range backupPolicyTPLs.Items {
		if item.Annotations[dptypes.DefaultBackupPolicyTemplateAnnotationKey] == trueVal {
			return &item, nil
		}
	}
	return &backupPolicyTPLs.Items[0], nil
}

func buildServiceAccount(transCtx *componentTransformContext) (*corev1.ServiceAccount, bool, error) {
	var (
		cluster = transCtx.Cluster
		comp    = transCtx.Component
		compDef = transCtx.CompDef
	)

	// TODO(component): dependency on cluster definition
	backupPolicyTPL, err := getDefaultBackupPolicyTemplate(transCtx, cluster.Spec.ClusterDefRef)
	if err != nil {
		return nil, false, err
	}

	serviceAccountName := comp.Spec.ServiceAccountName
	volumeProtectionEnable := isVolumeProtectionEnabled(compDef)
	dataProtectionEnable := isDataProtectionEnabled(backupPolicyTPL, cluster, comp)
	if serviceAccountName == "" {
		// If probe, volume protection, and data protection are disabled at the same tme, then do not create a service account.
		if !isProbesEnabled(compDef) && !volumeProtectionEnable && !dataProtectionEnable {
			return nil, false, nil
		}
		serviceAccountName = constant.GenerateDefaultServiceAccountName(comp.Name)
	}

	if isRoleBindingExist(transCtx, serviceAccountName) && isServiceAccountExist(transCtx, serviceAccountName) {
		// Volume protection requires the clusterRoleBinding permission, if volume protection is not enabled or the corresponding clusterRoleBinding already exists, then skip.
		if !volumeProtectionEnable || isClusterRoleBindingExist(transCtx, serviceAccountName) {
			return nil, false, nil
		}
	}

	// if volume protection is enabled, the service account needs to be bound to the clusterRoleBinding.
	return factory.BuildServiceAccount(cluster, serviceAccountName), volumeProtectionEnable, nil
}

func createServiceAccount(serviceAccount *corev1.ServiceAccount, graphCli model.GraphClient, dag *graph.DAG, parent client.Object) {
	// serviceAccount must be created before roleBinding and clusterRoleBinding
	graphCli.Create(dag, serviceAccount)
	graphCli.DependOn(dag, parent, serviceAccount)
}
