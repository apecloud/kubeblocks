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
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var deleteExample = templates.Examples(`
		# delete a cluster named mycluster
		kbcli cluster delete mycluster
`)

func NewDeleteCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
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
	return cmd
}

func deleteCluster(o *delete.DeleteOptions, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing cluster name")
	}
	o.Names = args
	return o.Run()
}

func clusterPreDeleteHook(object runtime.Object) error {
	cluster, err := getClusterFromObject(object)
	if err != nil {
		return err
	}
	if cluster.Spec.TerminationPolicy == appsv1alpha1.DoNotTerminate {
		return fmt.Errorf("cluster %s is protected by termination policy %s, skip deleting", cluster.Name, appsv1alpha1.DoNotTerminate)
	}
	return nil
}

func clusterPostDeleteHook(object runtime.Object) error {
	cluster, err := getClusterFromObject(object)
	if err != nil {
		return err
	}

	// HACK: for a postgresql cluster, we need to delete the sa, role and rolebinding

	return nil
}

func getClusterFromObject(object runtime.Object) (*appsv1alpha1.Cluster, error) {
	if object.GetObjectKind().GroupVersionKind().Kind != appsv1alpha1.ClusterKind {
		return nil, fmt.Errorf("object %s is not of kind %s", object.GetObjectKind().GroupVersionKind().Kind, appsv1alpha1.ClusterKind)
	}
	unstructured := object.(*unstructured.Unstructured)
	cluster := &appsv1alpha1.Cluster{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}
