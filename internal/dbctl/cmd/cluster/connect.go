/*
Copyright 2022 The KubeBlocks Authors

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
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/engine"
	"github.com/apecloud/kubeblocks/internal/dbctl/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/exec"
)

type ConnectOptions struct {
	ClusterName string
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
	o.ClusterName = args[0]

	clientSet, err := o.Factory.KubernetesClientSet()
	if err != nil {
		return err
	}

	// find the target pod to connect
	pod, err := findTargetPod(clientSet, o.ClusterName, o.PodName, o.Namespace)
	if err != nil {
		return err
	}

	// get the connect command and the target container
	command, containerName, err := getCommandAndContainer(pod)
	if err != nil {
		return err
	}

	// prefer user-specified container
	if len(o.ContainerName) > 0 {
		containerName = o.ContainerName
	}

	o.Command = command
	o.ContainerName = containerName
	o.Pod = pod
	return nil
}

func (o *ConnectOptions) validate() error {
	if len(o.ClusterName) == 0 {
		return fmt.Errorf("cluster name must be specified")
	}
	return nil
}

func (o *ConnectOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.PodName, "instance", "i", "", "The instance name to connect.")
	cmd.Flags().StringVarP(&o.ContainerName, "container", "c", "", "The container name to connect.")
}

func findTargetPod(clientset *kubernetes.Clientset, clusterName string, podName string, namespace string) (*corev1.Pod, error) {
	var err error
	var pod *corev1.Pod
	if len(podName) != 0 {
		pod, err = clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	} else {
		pod, err = util.GetPrimaryPod(clientset, clusterName, namespace)
	}
	return pod, err
}

func getCommandAndContainer(pod *corev1.Pod) ([]string, string, error) {
	var command []string
	var containerName string

	typeName, err := util.GetClusterTypeByPod(pod)
	if err != nil {
		return command, containerName, err
	}

	engine, err := engine.New(typeName)
	if err != nil {
		return command, containerName, err
	}

	info, err := engine.GetExecInfo("connect")
	if err != nil {
		return command, containerName, err
	}
	return info.Command, info.ContainerName, nil
}
