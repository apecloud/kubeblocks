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
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
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

	if transCtx.Component.Spec.ServiceAccountName != "" {
		// user specifies a serviceaccount, nothing to do
		return nil
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	synthesizedComp := transCtx.SynthesizeComponent
	serviceAccountName := constant.GenerateDefaultServiceAccountName(synthesizedComp.ClusterName, synthesizedComp.Name)

	if !isServiceAccountExist(transCtx, serviceAccountName) &&
		!viper.GetBool(constant.EnableRBACManager) {
		transCtx.Logger.V(1).Info("rbac manager is disabled")
		transCtx.EventRecorder.Event(transCtx.Cluster, corev1.EventTypeWarning,
			string(ictrlutil.ErrorTypeNotFound), fmt.Sprintf("ServiceAccount %s does not exist", serviceAccountName))
		return ictrlutil.NewRequeueError(time.Second, "RBAC manager is disabled, but service account does not exist")
	}

	sa, err := createOrUpdateServiceAccount(transCtx, serviceAccountName, graphCli, dag)
	if err != nil {
		return err
	}

	role, err := createOrUpdateRole(transCtx, sa.Name, graphCli, dag)
	if err != nil {
		return err
	}

	rbs, err := createOrUpdateRoleBinding(transCtx, role, sa.Name, graphCli, dag)
	if err != nil {
		return err
	}

	// serviceAccount should be created before roleBinding and role
	for _, rb := range rbs {
		graphCli.DependOn(dag, rb, sa, role)
	}
	// serviceAccount should be created before workload
	itsList := graphCli.FindAll(dag, &workloads.InstanceSet{})
	for _, its := range itsList {
		graphCli.DependOn(dag, its, sa)
	}

	return nil
}

func isServiceAccountExist(transCtx *componentTransformContext, serviceAccountName string) bool {
	synthesizedComp := transCtx.SynthesizeComponent
	namespaceName := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      serviceAccountName,
	}
	sa := &corev1.ServiceAccount{}
	if err := transCtx.Client.Get(transCtx.Context, namespaceName, sa, inDataContext4C()); err != nil {
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

func createOrUpdate[T any, PT generics.PObject[T]](
	transCtx *componentTransformContext, obj PT, graphCli model.GraphClient, dag *graph.DAG, cmpFn func(oldObj, newObj PT) bool,
) (PT, error) {
	oldObj := PT(new(T))
	if err := transCtx.Client.Get(transCtx.Context, client.ObjectKeyFromObject(obj), oldObj); err != nil {
		if errors.IsNotFound(err) {
			graphCli.Create(dag, obj, inDataContext4G())
			return obj, nil
		}
		return nil, err
	}
	if !cmpFn(oldObj, obj) {
		graphCli.Update(dag, oldObj, obj, inDataContext4G())
	}
	return obj, nil
}

func createOrUpdateServiceAccount(transCtx *componentTransformContext, serviceAccountName string, graphCli model.GraphClient, dag *graph.DAG) (*corev1.ServiceAccount, error) {
	synthesizedComp := transCtx.SynthesizeComponent

	sa := factory.BuildServiceAccount(synthesizedComp, serviceAccountName)
	if err := setCompOwnershipNFinalizer(transCtx.Component, sa); err != nil {
		return nil, err
	}

	return createOrUpdate(transCtx, sa, graphCli, dag, func(old, new *corev1.ServiceAccount) bool {
		return reflect.DeepEqual(old.ImagePullSecrets, new.ImagePullSecrets) &&
			reflect.DeepEqual(old.Secrets, new.Secrets) &&
			*old.AutomountServiceAccountToken == *new.AutomountServiceAccountToken
	})
}

func createOrUpdateRole(
	transCtx *componentTransformContext, serviceAccountName string, graphCli model.GraphClient, dag *graph.DAG,
) (*rbacv1.Role, error) {
	role := factory.BuildComponentRole(transCtx.SynthesizeComponent, transCtx.CompDef, serviceAccountName)
	if role == nil {
		return nil, nil
	}
	if err := setCompOwnershipNFinalizer(transCtx.Component, role); err != nil {
		return nil, err
	}
	return createOrUpdate(transCtx, role, graphCli, dag, func(old, new *rbacv1.Role) bool {
		return reflect.DeepEqual(old.Rules, new.Rules)
	})
}

func createOrUpdateRoleBinding(
	transCtx *componentTransformContext, cmpdRole *rbacv1.Role, serviceAccountName string, graphCli model.GraphClient, dag *graph.DAG,
) ([]*rbacv1.RoleBinding, error) {
	cmpRoleBinding := func(old, new *rbacv1.RoleBinding) bool {
		return reflect.DeepEqual(old.Subjects, new.Subjects) && reflect.DeepEqual(old.RoleRef, new.RoleRef)
	}
	res := make([]*rbacv1.RoleBinding, 0)

	if cmpdRole != nil {
		cmpdRoleBinding := factory.BuildRoleBinding(transCtx.SynthesizeComponent, &rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     cmpdRole.Name,
		}, serviceAccountName)
		rb, err := createOrUpdate(transCtx, cmpdRoleBinding, graphCli, dag, cmpRoleBinding)
		if err != nil {
			return nil, err
		}
		res = append(res, rb)
	}

	clusterPodRoleBinding := factory.BuildRoleBinding(transCtx.SynthesizeComponent, &rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     constant.RBACRoleName,
	}, fmt.Sprintf("%v-pod", serviceAccountName))
	rb, err := createOrUpdate(transCtx, clusterPodRoleBinding, graphCli, dag, cmpRoleBinding)
	if err != nil {
		return nil, err
	}
	res = append(res, rb)
	return res, nil
}
