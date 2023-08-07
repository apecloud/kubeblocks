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

	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	ictrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// RBACTransformer puts the rbac at the beginning of the DAG
type RBACTransformer struct{}

var _ graph.Transformer = &RBACTransformer{}

const (
	RBACRoleName        = "kubeblocks-cluster-pod-role"
	RBACClusterRoleName = "kubeblocks-volume-protection-pod-role"
	ServiceAccountKind  = "ServiceAccount"
)

func (c *RBACTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	clusterDef := transCtx.ClusterDef
	root, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}

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
			synthesizedComponent, err := component.BuildComponent(reqCtx, nil, cluster, transCtx.ClusterTemplate, transCtx.ClusterDef, &compDef, nil)
			if err != nil {
				return err
			}
			if synthesizedComponent == nil {
				continue
			}
			comps = []appsv1alpha1.ClusterComponentSpec{{ServiceAccountName: synthesizedComponent.ServiceAccountName}}
		}
		componentSpecs = append(componentSpecs, comps...)
	}

	for _, compSpec := range componentSpecs {
		serviceAccountName := compSpec.ServiceAccountName
		if serviceAccountName == "" {
			if !needToCreateRBAC(clusterDef) {
				return nil
			}
			serviceAccountName = "kb-" + cluster.Name
		}

		if !viper.GetBool(constant.EnableRBACManager) {
			transCtx.Logger.V(1).Info("rbac manager is disabled")
			if !isServiceAccountExist(transCtx, serviceAccountName, true) {
				return ictrlutil.NewRequeueError(time.Second,
					fmt.Sprintf("RBAC manager is disabed, but service account %s is not exsit", serviceAccountName))
			}
			return nil
		}

		if isClusterRoleBindingExist(transCtx, serviceAccountName) &&
			isServiceAccountExist(transCtx, serviceAccountName, false) {
			continue
		}

		clusterRoleBinding, err := builder.BuildClusterRoleBinding(cluster)
		if err != nil {
			return err
		}
		clusterRoleBinding.Subjects[0].Name = serviceAccountName
		crbVertex := ictrltypes.LifecycleObjectCreate(dag, clusterRoleBinding, root)

		roleBinding, err := builder.BuildRoleBinding(cluster)
		if err != nil {
			return err
		}
		roleBinding.Subjects[0].Name = serviceAccountName
		rbVertex := ictrltypes.LifecycleObjectCreate(dag, roleBinding, crbVertex)

		serviceAccount, err := builder.BuildServiceAccount(cluster)
		if err != nil {
			return err
		}
		serviceAccount.Name = serviceAccountName
		// serviceaccount must be created before rolebinding and clusterrolebinding
		saVertex := ictrltypes.LifecycleObjectCreate(dag, serviceAccount, rbVertex)

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
	return nil
}

func needToCreateRBAC(clusterDef *appsv1alpha1.ClusterDefinition) bool {
	for _, compDef := range clusterDef.Spec.ComponentDefs {
		if compDef.Probes != nil || compDef.VolumeProtectionSpec != nil {
			return true
		}
	}
	return false
}

func isServiceAccountExist(transCtx *ClusterTransformContext, serviceAccountName string, sendEvent bool) bool {
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
			if sendEvent {
				transCtx.EventRecorder.Event(transCtx.Cluster, corev1.EventTypeWarning,
					string(ictrlutil.ErrorTypeNotFound), serviceAccountName+" ServiceAccount is not exist")
			}
			return false
		}
		transCtx.Logger.Error(err, "get ServiceAccount failed")
		return false
	}
	return true
}

func isClusterRoleBindingExist(transCtx *ClusterTransformContext, serviceAccountName string) bool {
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

	if crb.RoleRef.Name != RBACClusterRoleName {
		transCtx.Logger.V(1).Info("rbac manager: ClusterRole not match", "ClusterRole",
			RBACClusterRoleName, "clusterrolebinding.RoleRef", crb.RoleRef.Name)
	}

	isServiceAccountMatch := false
	for _, sub := range crb.Subjects {
		if sub.Kind == ServiceAccountKind && sub.Name == serviceAccountName {
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
