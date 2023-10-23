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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// RBACTransformer puts the rbac at the beginning of the DAG
type RBACTransformer struct{}

var _ graph.Transformer = &RBACTransformer{}

func (c *RBACTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)

	componentSpecs, err := getComponentSpecs(transCtx)
	if err != nil {
		return err
	}

	serviceAccounts, serviceAccountsNeedCrb, err := buildServiceAccounts(transCtx, componentSpecs)
	if err != nil {
		return err
	}

	if !viper.GetBool(constant.EnableRBACManager) {
		transCtx.Logger.V(1).Info("rbac manager is disabled")
		saNotExist := false
		for saName := range serviceAccounts {
			if !isServiceAccountExist(transCtx, saName) {
				transCtx.EventRecorder.Event(transCtx.Cluster, corev1.EventTypeWarning,
					string(ictrlutil.ErrorTypeNotFound), saName+" ServiceAccount is not exist")
				saNotExist = true
			}
		}
		if saNotExist {
			return ictrlutil.NewRequeueError(time.Second, "RBAC manager is disabed, but service account is not exist")
		}
		return nil
	}

	var parent client.Object
	rb := buildRoleBinding(cluster, serviceAccounts)
	graphCli.Create(dag, rb)
	parent = rb
	if len(serviceAccountsNeedCrb) > 0 {
		crb := buildClusterRoleBinding(cluster, serviceAccountsNeedCrb)
		graphCli.Create(dag, crb)
		graphCli.DependOn(dag, parent, crb)
		parent = crb
	}

	sas := createServiceAccounts(serviceAccounts, graphCli, dag, parent)
	stsList := graphCli.FindAll(dag, &appsv1.StatefulSet{})
	for _, sts := range stsList {
		// serviceaccount must be created before statefulset
		graphCli.DependOn(dag, sts, sas...)
	}

	deployList := graphCli.FindAll(dag, &appsv1.Deployment{})
	for _, deploy := range deployList {
		// serviceaccount must be created before deployment
		graphCli.DependOn(dag, deploy, sas...)
	}

	return nil
}

func isProbesEnabled(clusterDef *appsv1alpha1.ClusterDefinition, compSpec *appsv1alpha1.ClusterComponentSpec) bool {
	for _, compDef := range clusterDef.Spec.ComponentDefs {
		if compDef.Name == compSpec.ComponentDefRef && compDef.Probes != nil {
			return true
		}
	}
	return false
}

func isDataProtectionEnabled(backupTpl *appsv1alpha1.BackupPolicyTemplate, compSpec *appsv1alpha1.ClusterComponentSpec) bool {
	if backupTpl == nil {
		return false
	}
	for _, policy := range backupTpl.Spec.BackupPolicies {
		if policy.ComponentDefRef == compSpec.ComponentDefRef {
			return true
		}
	}
	return false
}

func isVolumeProtectionEnabled(clusterDef *appsv1alpha1.ClusterDefinition, compSpec *appsv1alpha1.ClusterComponentSpec) bool {
	for _, compDef := range clusterDef.Spec.ComponentDefs {
		if compDef.Name == compSpec.ComponentDefRef && compDef.VolumeProtectionSpec != nil {
			return true
		}
	}
	return false
}

func isServiceAccountExist(transCtx *clusterTransformContext, serviceAccountName string) bool {
	cluster := transCtx.Cluster
	namespaceName := types.NamespacedName{
		Namespace: cluster.Namespace,
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

func isClusterRoleBindingExist(transCtx *clusterTransformContext, serviceAccountName string) bool {
	cluster := transCtx.Cluster
	namespaceName := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      "kb-" + cluster.Name,
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

func isRoleBindingExist(transCtx *clusterTransformContext, serviceAccountName string) bool {
	cluster := transCtx.Cluster
	namespaceName := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      "kb-" + cluster.Name,
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

	if rb.RoleRef.Name != constant.RBACClusterRoleName {
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

func getComponentSpecs(transCtx *clusterTransformContext) ([]appsv1alpha1.ClusterComponentSpec, error) {
	cluster := transCtx.Cluster
	clusterDef := transCtx.ClusterDef
	componentSpecs := make([]appsv1alpha1.ClusterComponentSpec, 0, 1)
	compSpecMap := cluster.Spec.GetDefNameMappingComponents()
	for _, compDef := range clusterDef.Spec.ComponentDefs {
		comps := compSpecMap[compDef.Name]
		if len(comps) == 0 {
			// if componentSpecs is empty, it may be generated from the cluster template and cluster.
			reqCtx := ictrlutil.RequestCtx{
				Ctx: transCtx.Context,
				Log: log.Log.WithName("rbac"),
			}
			synthesizedComponent, err := component.BuildComponent(reqCtx, nil, cluster, transCtx.ClusterDef, &compDef, nil, nil)
			if err != nil {
				return nil, err
			}
			if synthesizedComponent == nil {
				continue
			}
			comps = []appsv1alpha1.ClusterComponentSpec{{
				ServiceAccountName: synthesizedComponent.ServiceAccountName,
				ComponentDefRef:    compDef.Name,
			}}
		}
		componentSpecs = append(componentSpecs, comps...)
	}
	return componentSpecs, nil
}

func getDefaultBackupPolicyTemplate(transCtx *clusterTransformContext, clusterDefName string) (*appsv1alpha1.BackupPolicyTemplate, error) {
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

func buildServiceAccounts(transCtx *clusterTransformContext, componentSpecs []appsv1alpha1.ClusterComponentSpec) (map[string]*corev1.ServiceAccount, map[string]*corev1.ServiceAccount, error) {
	serviceAccounts := map[string]*corev1.ServiceAccount{}
	serviceAccountsNeedCrb := map[string]*corev1.ServiceAccount{}
	clusterDef := transCtx.ClusterDef
	cluster := transCtx.Cluster
	backupPolicyTPL, err := getDefaultBackupPolicyTemplate(transCtx, clusterDef.Name)
	if err != nil {
		return serviceAccounts, serviceAccountsNeedCrb, err
	}
	for _, compSpec := range componentSpecs {
		serviceAccountName := compSpec.ServiceAccountName
		if serviceAccountName == "" {
			if !isProbesEnabled(clusterDef, &compSpec) && !isVolumeProtectionEnabled(clusterDef, &compSpec) && !isDataProtectionEnabled(backupPolicyTPL, &compSpec) {
				continue
			}
			serviceAccountName = "kb-" + cluster.Name
		}

		if isRoleBindingExist(transCtx, serviceAccountName) && isServiceAccountExist(transCtx, serviceAccountName) {
			if !isVolumeProtectionEnabled(clusterDef, &compSpec) || isClusterRoleBindingExist(transCtx, serviceAccountName) {
				continue
			}
		}

		if _, ok := serviceAccounts[serviceAccountName]; ok {
			continue
		}
		serviceAccount := factory.BuildServiceAccount(cluster)
		serviceAccount.Name = serviceAccountName
		serviceAccounts[serviceAccountName] = serviceAccount

		if isVolumeProtectionEnabled(clusterDef, &compSpec) {
			serviceAccountsNeedCrb[serviceAccountName] = serviceAccount
		}
	}
	return serviceAccounts, serviceAccountsNeedCrb, nil
}

func buildRoleBinding(cluster *appsv1alpha1.Cluster, serviceAccounts map[string]*corev1.ServiceAccount) *rbacv1.RoleBinding {
	roleBinding := factory.BuildRoleBinding(cluster)
	roleBinding.Subjects = []rbacv1.Subject{}
	for saName := range serviceAccounts {
		subject := rbacv1.Subject{
			Name:      saName,
			Namespace: cluster.Namespace,
			Kind:      rbacv1.ServiceAccountKind,
		}
		roleBinding.Subjects = append(roleBinding.Subjects, subject)
	}
	return roleBinding
}

func buildClusterRoleBinding(cluster *appsv1alpha1.Cluster, serviceAccounts map[string]*corev1.ServiceAccount) *rbacv1.ClusterRoleBinding {
	clusterRoleBinding := factory.BuildClusterRoleBinding(cluster)
	clusterRoleBinding.Subjects = []rbacv1.Subject{}
	for saName := range serviceAccounts {
		subject := rbacv1.Subject{
			Name:      saName,
			Namespace: cluster.Namespace,
			Kind:      rbacv1.ServiceAccountKind,
		}
		clusterRoleBinding.Subjects = append(clusterRoleBinding.Subjects, subject)
	}
	return clusterRoleBinding
}

func createServiceAccounts(serviceAccounts map[string]*corev1.ServiceAccount, graphCli model.GraphClient, dag *graph.DAG, parent client.Object) []client.Object {
	var sas []client.Object
	for _, sa := range serviceAccounts {
		// serviceaccount must be created before rolebinding and clusterrolebinding
		graphCli.Create(dag, sa)
		graphCli.DependOn(dag, parent, sa)
		sas = append(sas, sa)
	}
	return sas
}
