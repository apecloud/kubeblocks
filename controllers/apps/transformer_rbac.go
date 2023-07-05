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
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
)

// RBACTransformer puts the rbac at the beginning of the DAG
type RBACTransformer struct{}

var _ graph.Transformer = &RBACTransformer{}

func (c *RBACTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	if !viper.GetBool("ENABLE_RBAC_MANAGER") {
		transCtx.Logger.Info("rbac manager is not enabled")
		return nil
	}

	root, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}

	for _, compSpec := range cluster.Spec.ComponentSpecs {
		serviceAccountName := compSpec.ServiceAccountName
		if isServiceAccountNotExist(transCtx, serviceAccountName) {
			serviceAccount, err := builder.BuildServiceAccount(cluster)
			if err != nil {
				return err
			}
			serviceAccount.Name = serviceAccountName
			saVertex := ictrltypes.LifecycleObjectCreate(dag, serviceAccount, nil)

			statefulSetVertices := ictrltypes.FindAll[*appsv1.StatefulSet](dag)
			for _, statefulSetVertex := range statefulSetVertices {
				// serviceaccount must be created before statefulset
				dag.Connect(statefulSetVertex, saVertex)
			}

			deploymentVertices := ictrltypes.FindAll[*appsv1.Deployment](dag)
			for _, deploymentVertex := range deploymentVertices {
				// serviceaccount must be created before deployment
				dag.Connect(deploymentVertex, saVertex)
			}
		}

		if isRoleBindingNotExist(transCtx) {
			roleBinding, err := builder.BuildRoleBinding(cluster)
			if err != nil {
				return err
			}
			roleBinding.Subjects[0].Name = serviceAccountName
			rbVertex := ictrltypes.LifecycleObjectCreate(dag, roleBinding, root)

			saVertices := ictrltypes.FindAll[*corev1.ServiceAccount](dag)
			for _, saVertex := range saVertices {
				// rolebinding must be created before serviceaccount
				dag.Connect(saVertex, rbVertex)
			}
		}

	}
	return nil
}

func isServiceAccountNotExist(transCtx *ClusterTransformContext, serviceAccountName string) bool {
	cluster := transCtx.Cluster
	namespaceName := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      serviceAccountName,
	}
	sa := &corev1.ServiceAccount{}
	if err := transCtx.Client.Get(transCtx.Context, namespaceName, sa); err != nil {
		// KubeBlocks will create a serviceaccount only if it has RBAC access priority and
		// the serviceaccount is not already present.
		if errors.IsNotFound(err) {
			return true
		}
		transCtx.Logger.V(0).Error(err, "get service account failed")
	}

	return false
}

func isRoleBindingNotExist(transCtx *ClusterTransformContext) bool {
	cluster := transCtx.Cluster
	namespaceName := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      "kb-" + cluster.Name,
	}
	rb := &rbacv1.RoleBinding{}
	if err := transCtx.Client.Get(transCtx.Context, namespaceName, rb); err != nil {
		// KubeBlocks will create a rolebinding only if it has RBAC access priority and
		// the rolebinding is not already present.
		if errors.IsNotFound(err) {
			return true
		}
		transCtx.Logger.V(0).Error(err, "get service account failed")
	}

	return false
}
