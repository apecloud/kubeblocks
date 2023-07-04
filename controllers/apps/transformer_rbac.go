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
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
)

// RBACTransformer puts the rbac at the beginning of the DAG
type RBACTransformer struct{}

const PGTYPE = "postgresql"

var _ graph.Transformer = &RBACTransformer{}

func (c *RBACTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	if cluster.IsDeleting() {
		return nil
	}

	root, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}

	serviceAccount, err := builder.BuildServiceAccount(cluster)
	if err != nil {
		return err
	}
	saVertex := ictrltypes.LifecycleObjectCreate(dag, serviceAccount, root)

	role, err := builder.BuildRole(cluster)
	if err != nil {
		return err
	}

	completeRoleRules(transCtx, role)
	roleVertex := ictrltypes.LifecycleObjectCreate(dag, role, root)

	roleBinding, err := builder.BuildRoleBinding(cluster)
	if err != nil {
		return err
	}
	rbVertex := ictrltypes.LifecycleObjectCreate(dag, roleBinding, root)
	dag.Connect(rbVertex, roleVertex)
	dag.Connect(rbVertex, saVertex)

	statefulSetVertices := ictrltypes.FindAll[*appsv1.StatefulSet](dag)
	for _, statefulSetVertex := range statefulSetVertices {
		// rbac must be created before statefulset
		dag.Connect(statefulSetVertex, rbVertex)
	}

	deploymentVertices := ictrltypes.FindAll[*appsv1.Deployment](dag)
	for _, deploymentVertex := range deploymentVertices {
		// rbac must be created before deployment
		dag.Connect(deploymentVertex, rbVertex)
	}
	return nil
}

func completeRoleRules(transCtx *ClusterTransformContext, role *rbacv1.Role) {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{"dataprotection.kubeblocks.io"},
			Resources: []string{"backups/status"},
			Verbs:     []string{"get", "update", "patch"},
		},
		{
			APIGroups: []string{"dataprotection.kubeblocks.io"},
			Resources: []string{"backups"},
			Verbs:     []string{"create", "get", "list", "update", "patch"},
		},
	}

	// postgresql need more rules for patroni
	if isPostgresqlCluster(transCtx) {
		rules = append(rules, []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"create", "get", "list", "patch", "update", "watch", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints"},
				Verbs:     []string{"create", "get", "list", "patch", "update", "watch", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "patch", "update", "watch"},
			},
		}...)
	}
	role.Rules = append(role.Rules, rules...)
}

func isPostgresqlCluster(transCtx *ClusterTransformContext) bool {
	cd := transCtx.ClusterDef
	cluster := transCtx.Cluster

	if cd.Spec.Type != PGTYPE {
		return false
	}

	for _, compSpec := range cluster.Spec.ComponentSpecs {
		for _, def := range cd.Spec.ComponentDefs {
			if def.Name == compSpec.ComponentDefRef && def.CharacterType == PGTYPE {
				return true
			}
		}
	}

	return false
}
