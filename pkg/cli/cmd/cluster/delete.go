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

package cluster

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/cli/delete"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
)

var (
	deleteExample = templates.Examples(`
		# delete a cluster named mycluster
		kbcli cluster delete mycluster
		# delete a cluster by label selector
		kbcli cluster delete --selector clusterdefinition.kubeblocks.io/name=apecloud-mysql
`)

	rbacEnabled = false
)

func NewDeleteCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := delete.NewDeleteOptions(f, streams, types.ClusterGVR())
	o.PreDeleteHook = clusterPreDeleteHook
	o.PostDeleteHook = clusterPostDeleteHook

	cmd := &cobra.Command{
		Use:               "delete NAME",
		Short:             "Delete clusters.",
		Example:           deleteExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(deleteCluster(o, args))
		},
	}
	o.AddFlags(cmd)
	cmd.Flags().BoolVar(&rbacEnabled, "rbac-enabled", false, "Specify whether rbac resources will be deleted by kbcli")
	return cmd
}

func deleteCluster(o *delete.DeleteOptions, args []string) error {
	if len(args) == 0 && len(o.LabelSelector) == 0 {
		return fmt.Errorf("missing cluster name or a lable selector")
	}
	o.Names = args
	return o.Run()
}

func clusterPreDeleteHook(o *delete.DeleteOptions, object runtime.Object) error {
	if object == nil {
		return nil
	}

	cluster, err := getClusterFromObject(object)
	if err != nil {
		return err
	}
	if cluster.Spec.TerminationPolicy == appsv1alpha1.DoNotTerminate {
		return fmt.Errorf("cluster %s is protected by termination policy %s, skip deleting", cluster.Name, appsv1alpha1.DoNotTerminate)
	}
	return nil
}

func clusterPostDeleteHook(o *delete.DeleteOptions, object runtime.Object) error {
	if object == nil {
		return nil
	}

	c, err := getClusterFromObject(object)
	if err != nil {
		return err
	}

	client, err := o.Factory.KubernetesClientSet()
	if err != nil {
		return err
	}

	if err = deleteDependencies(client, c.Namespace, c.Name); err != nil {
		return err
	}
	return nil
}

func deleteDependencies(client kubernetes.Interface, ns string, name string) error {
	if !rbacEnabled {
		return nil
	}

	klog.V(1).Infof("delete dependencies for cluster %s", name)
	var (
		saName                 = saNamePrefix + name
		roleName               = roleNamePrefix + name
		roleBindingName        = roleBindingNamePrefix + name
		clusterRoleName        = clusterRolePrefix + name
		clusterRoleBindingName = clusterRoleBindingPrefix + name
		allErr                 []error
	)

	// now, delete the dependencies, for postgresql, we delete sa, role and rolebinding
	ctx := context.TODO()
	gracePeriod := int64(0)
	deleteOptions := metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}
	checkErr := func(err error) bool {
		if err != nil && !apierrors.IsNotFound(err) {
			return true
		}
		return false
	}

	// delete cluster role binding
	klog.V(1).Infof("delete cluster role binding %s", clusterRoleBindingName)
	if err := client.RbacV1().ClusterRoleBindings().Delete(ctx, clusterRoleBindingName, deleteOptions); checkErr(err) {
		allErr = append(allErr, err)
	}

	// delete cluster role
	klog.V(1).Infof("delete cluster role %s", clusterRoleName)
	if err := client.RbacV1().ClusterRoles().Delete(ctx, clusterRoleName, deleteOptions); checkErr(err) {
		allErr = append(allErr, err)
	}

	// delete role binding
	klog.V(1).Infof("delete role binding %s", roleBindingName)
	if err := client.RbacV1().RoleBindings(ns).Delete(ctx, roleBindingName, deleteOptions); checkErr(err) {
		allErr = append(allErr, err)
	}

	// delete role
	klog.V(1).Infof("delete role %s", roleName)
	if err := client.RbacV1().Roles(ns).Delete(ctx, roleName, deleteOptions); checkErr(err) {
		allErr = append(allErr, err)
	}

	// delete service account
	klog.V(1).Infof("delete service account %s", saName)
	if err := client.CoreV1().ServiceAccounts(ns).Delete(ctx, saName, deleteOptions); checkErr(err) {
		allErr = append(allErr, err)
	}

	return errors.NewAggregate(allErr)
}

func getClusterFromObject(object runtime.Object) (*appsv1alpha1.Cluster, error) {
	if object.GetObjectKind().GroupVersionKind().Kind != appsv1alpha1.ClusterKind {
		return nil, fmt.Errorf("object %s is not of kind %s", object.GetObjectKind().GroupVersionKind().Kind, appsv1alpha1.ClusterKind)
	}
	u := object.(*unstructured.Unstructured)
	cluster := &appsv1alpha1.Cluster{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}
