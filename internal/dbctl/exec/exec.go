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

package exec

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	cmdexec "k8s.io/kubectl/pkg/cmd/exec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/podcmd"

	"github.com/apecloud/kubeblocks/internal/dbctl/engine"
	"github.com/apecloud/kubeblocks/internal/dbctl/util"
)

// Options used to build a command that use exec to implement
type Options struct {
	cmdexec.StreamOptions
	Factory cmdutil.Factory

	Use   string
	Short string

	ClusterName      string
	EnforceNamespace bool

	clientset *kubernetes.Clientset
	Executor  cmdexec.RemoteExecutor
	Config    *restclient.Config
}

func NewExecOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, use string, short string) *Options {
	return &Options{
		Factory: f,
		StreamOptions: cmdexec.StreamOptions{
			IOStreams: streams,
			Stdin:     true,
			TTY:       true,
		},
		Use:      use,
		Short:    short,
		Executor: &cmdexec.DefaultRemoteExecutor{},
	}
}

func (o *Options) Build() *cobra.Command {
	cmd := &cobra.Command{
		Use:   o.Use,
		Short: o.Short,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(o.Factory, args))
			cmdutil.CheckErr(o.validate())
			cmdutil.CheckErr(o.run())
		},
	}

	cmd.Flags().StringVarP(&o.PodName, "instance", "i", "", "instance name")
	return cmd
}

func (o *Options) complete(f cmdutil.Factory, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("you must specify the name of resource to exec")
	}
	o.ClusterName = args[0]

	var err error
	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil
	}

	o.Config, err = f.ToRESTConfig()
	if err != nil {
		return err
	}

	o.clientset, err = f.KubernetesClientSet()
	if err != nil {
		return err
	}

	return nil
}

func (o *Options) validate() error {
	if len(o.ClusterName) == 0 {
		return fmt.Errorf("cluster name must be specified")
	}
	return nil
}

func (o *Options) run() error {
	var err error
	var pod *corev1.Pod
	var e engine.Interface

	if len(o.PodName) != 0 {
		pod, err = o.clientset.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.PodName, metav1.GetOptions{})
		if err != nil {
			return err
		}
	} else {
		// find pod in cluster to exec
		pod, err = findPrimaryPod(o.clientset, o.ClusterName, o.Namespace)
		if err != nil {
			return err
		}
	}

	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return fmt.Errorf("cannot exec into a container in a completed pod; current phase is %s", pod.Status.Phase)
	}

	typeName := getClusterType(pod)
	if typeName == "" {
		return fmt.Errorf("failed to get the cluster type")
	}

	e, err = engine.New(typeName)
	if err != nil {
		return err
	}

	info := e.GetExecCommand(o.Use)
	containerName := info.ContainerName
	if len(containerName) == 0 {
		container, err := podcmd.FindOrDefaultContainerByName(pod, containerName, true, o.ErrOut)
		if err != nil {
			return err
		}
		containerName = container.Name
	}

	// ensure we can recover the terminal while attached
	t := o.SetupTTY()

	var sizeQueue remotecommand.TerminalSizeQueue
	if t.Raw {
		// this call spawns a goroutine to monitor/update the terminal size
		sizeQueue = t.MonitorSize(t.GetSize())

		// unset p.Err if it was previously set because both stdout and stderr go over p.Out when tty is
		// true
		o.ErrOut = nil
	}

	fn := func() error {
		restClient, err := restclient.RESTClientFor(o.Config)
		if err != nil {
			return err
		}

		req := restClient.Post().
			Resource("pods").
			Name(pod.Name).
			Namespace(pod.Namespace).
			SubResource("exec")
		req.VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   info.Command,
			Stdin:     o.Stdin,
			Stdout:    o.Out != nil,
			Stderr:    o.ErrOut != nil,
			TTY:       t.Raw,
		}, scheme.ParameterCodec)

		return o.Executor.Execute("POST", req.URL(), o.Config, o.In, o.Out, o.ErrOut, t.Raw, sizeQueue)
	}

	if err := t.Safe(fn); err != nil {
		return err
	}

	return nil
}

func findPrimaryPod(clientset *kubernetes.Clientset, name string, namespace string) (*corev1.Pod, error) {
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: util.InstanceLabel(name)})
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		// TODO: check role is primary
		return &pod, nil
	}
	return nil, fmt.Errorf("failed to find the pod to exec command")
}

func getClusterType(pod *corev1.Pod) string {
	if name, ok := pod.Labels["app.kubernetes.io/name"]; ok {
		return strings.Split(name, "-")[0]
	}
	return ""
}
