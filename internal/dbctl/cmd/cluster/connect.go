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
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/engine"
	"github.com/apecloud/kubeblocks/internal/dbctl/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/exec"
)

type ConnectOptions struct {
	Use           string
	Short         string
	ClusterName   string
	InstanceName  string
	ContainerName string
}

// NewConnectCmd return the cmd of connecting a cluster
func NewConnectCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	c := &ConnectOptions{
		Use:   "connect",
		Short: "connect to a database cluster",
	}
	connectInput := &exec.ExecInput{
		Use:      c.Use,
		Short:    c.Short,
		Validate: c.validate,
		Complete: c.complete,
		AddFlags: c.addFlags,
	}
	o := exec.NewExecOptions(f, streams, connectInput)
	return o.Build()
}

// complete create exec parameters for connecting db cluster, especially logic for connect cmd
func (o *ConnectOptions) complete(f cmdutil.Factory, args []string) (*exec.ExecParams, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("you must specify the name of resource to connect")
	}
	o.ClusterName = args[0]
	namespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, err
	}
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	clientSet, err := f.KubernetesClientSet()
	if err != nil {
		return nil, err
	}
	podClient := clientSet.CoreV1()
	// set exec parameter
	pod, err := findTargetPod(podClient, o.ClusterName, o.InstanceName, namespace)
	if err != nil {
		return nil, err
	}
	command, err := getConnectCMD(pod)
	if err != nil {
		return nil, err
	}
	return &exec.ExecParams{
		Pod:           pod,
		Command:       command,
		ContainerName: o.ContainerName,
		Config:        config,
	}, nil
}

func (o *ConnectOptions) validate() error {
	if len(o.ClusterName) == 0 {
		return fmt.Errorf("cluster name must be specified")
	}
	return nil
}

func (o *ConnectOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.InstanceName, "instance", "i", "", "Instance name.")
	cmd.Flags().StringVarP(&o.ContainerName, "container", "c", "", "Container name.")
}

func findTargetPod(podClient coreclient.PodsGetter, clusterName string, podName string, namespace string) (*corev1.Pod, error) {
	var err error
	var pod *corev1.Pod
	if len(podName) != 0 {
		pod, err = podClient.Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	} else {
		pod, err = findPrimaryPod(podClient, clusterName, namespace)
	}
	return pod, err
}

func findPrimaryPod(podClient coreclient.PodsGetter, name string, namespace string) (*corev1.Pod, error) {
	pods, err := podClient.Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: util.InstanceLabel(name)})
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		// TODO: check role is primary
		return &pod, nil
	}
	return nil, fmt.Errorf("failed to find the pod to exec command")
}

func getConnectCMD(pod *corev1.Pod) ([]string, error) {
	typeName := getClusterType(pod)
	if typeName == "" {
		return nil, fmt.Errorf("typeName is nil, unsupported engine")
	}
	e, err := engine.New(typeName)
	if err != nil {
		return nil, err
	}
	return e.GetExecCommand("connect").Command, nil
}
func getClusterType(pod *corev1.Pod) string {
	if name, ok := pod.Labels["app.kubernetes.io/name"]; ok {
		return strings.Split(name, "-")[0]
	}
	return ""
}
