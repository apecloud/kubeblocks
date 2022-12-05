/*
Copyright ApeCloud Inc.

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
	"context"
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/engine"
	"github.com/apecloud/kubeblocks/internal/cli/exec"
)

type ConnectOptions struct {
	clusterName string
	database    string
	*exec.ExecOptions
}

// NewConnectCmd return the cmd of connecting a cluster
func NewConnectCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &ConnectOptions{ExecOptions: exec.NewExecOptions(f, streams)}
	input := &exec.ExecInput{
		Use:      "connect",
		Short:    "connect to a database cluster",
		Validate: o.validate,
		Complete: o.complete,
		AddFlags: o.addFlags,
	}
	return o.Build(input)
}

// complete create exec parameters for connecting cluster, especially logic for connect cmd
func (o *ConnectOptions) complete(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("you must specify the cluster to connect")
	}
	o.clusterName = args[0]

	dynamicClient, err := o.Factory.DynamicClient()
	if err != nil {
		return err
	}

	// get target pod name, if not specified, find default pod from cluster
	if len(o.PodName) == 0 {
		if o.PodName, err = cluster.GetDefaultPodName(dynamicClient, o.clusterName, o.Namespace); err != nil {
			return err
		}
	}

	// get the pod object
	pod, err := o.ClientSet.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// get the connect command and the target container
	engine, err := getEngineByPod(pod)
	if err != nil {
		return err
	}

	o.Command = engine.ConnectCommand(o.database)
	o.ContainerName = engine.EngineName()
	o.Pod = pod
	return nil
}

func (o *ConnectOptions) validate() error {
	if len(o.clusterName) == 0 {
		return fmt.Errorf("cluster name must be specified")
	}
	return nil
}

func (o *ConnectOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.PodName, "instance", "i", "", "The instance name to connect.")
	cmd.Flags().StringVarP(&o.database, "database", "D", "", "The database name to connect.")
}

func getEngineByPod(pod *corev1.Pod) (engine.Interface, error) {
	typeName, err := cluster.GetClusterTypeByPod(pod)
	if err != nil {
		return nil, err
	}

	engine, err := engine.New(typeName)
	if err != nil {
		return nil, err
	}

	return engine, nil
}
