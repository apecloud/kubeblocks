/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
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
	if object.GetObjectKind().GroupVersionKind().Kind != appsv1alpha1.ClusterKind {
		klog.V(1).Infof("object %s is not of kind %s, skip PreDeleteHook.", object.GetObjectKind().GroupVersionKind().Kind, appsv1alpha1.ClusterKind)
		return nil
	}
	unstructed := object.(*unstructured.Unstructured)
	cluster := &appsv1alpha1.Cluster{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructed.Object, cluster); err != nil {
		return err
	}
	if cluster.Spec.TerminationPolicy == appsv1alpha1.DoNotTerminate {
		return fmt.Errorf("cluster %s is protected by termination policy %s, skip deleting", cluster.Name, appsv1alpha1.DoNotTerminate)
	}
	return nil
}
