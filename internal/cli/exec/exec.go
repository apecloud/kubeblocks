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

package exec

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	cmdexec "k8s.io/kubectl/pkg/cmd/exec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/podcmd"
)

type ExecOptions struct {
	cmdexec.StreamOptions

	Factory  cmdutil.Factory
	Executor cmdexec.RemoteExecutor
	Config   *restclient.Config
	Client   *kubernetes.Clientset
	Dynamic  dynamic.Interface

	// Pod target pod to execute command
	Pod *corev1.Pod

	// Command is the command to execute
	Command []string
}

func NewExecOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *ExecOptions {
	return &ExecOptions{
		Factory: f,
		StreamOptions: cmdexec.StreamOptions{
			IOStreams: streams,
			Stdin:     true,
			TTY:       true,
		},
		Executor: &cmdexec.DefaultRemoteExecutor{},
	}
}

// Complete receive exec parameters
func (o *ExecOptions) Complete() error {
	var err error
	o.Config, err = o.Factory.ToRESTConfig()
	if err != nil {
		return err
	}

	o.Namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.Dynamic, err = o.Factory.DynamicClient()
	if err != nil {
		return err
	}

	o.Client, err = o.Factory.KubernetesClientSet()
	return err
}

func (o *ExecOptions) validate() error {
	var err error

	// pod is not get, try to get it by pod name
	if o.Pod == nil && len(o.PodName) > 0 {
		if o.Pod, err = o.Client.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.PodName, metav1.GetOptions{}); err != nil {
			return err
		}
	}

	if o.Pod == nil {
		return fmt.Errorf("failed to get the pod to execute")
	}
	if len(o.Command) == 0 {
		return fmt.Errorf("you must specify at least one command for the container")
	}
	if o.Out == nil || o.ErrOut == nil {
		return fmt.Errorf("both output and error output must be provided")
	}

	if o.Pod.Status.Phase == corev1.PodSucceeded ||
		o.Pod.Status.Phase == corev1.PodFailed {
		return fmt.Errorf("cannot exec into a container in a completed pod; current phase is %s", o.Pod.Status.Phase)
	}

	// check and get the container to execute command
	if len(o.ContainerName) == 0 {
		container, err := podcmd.FindOrDefaultContainerByName(o.Pod, "", o.Quiet, o.ErrOut)
		if err != nil {
			return err
		}
		o.ContainerName = container.Name
	}

	return nil
}

func (o *ExecOptions) Run() error {
	return o.RunWithRedirect(o.Out, o.ErrOut)
}

func (o *ExecOptions) RunWithRedirect(outWriter io.Writer, errWriter io.Writer) error {
	if err := o.validate(); err != nil {
		return err
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
			Name(o.Pod.Name).
			Namespace(o.Pod.Namespace).
			SubResource("exec")
		req.VersionedParams(&corev1.PodExecOptions{
			Container: o.ContainerName,
			Command:   o.Command,
			Stdin:     o.Stdin,
			Stdout:    outWriter != nil,
			Stderr:    errWriter != nil,
			TTY:       t.Raw,
		}, scheme.ParameterCodec)

		return o.Executor.Execute("POST", req.URL(), o.Config, o.In, outWriter, errWriter, t.Raw, sizeQueue)
	}

	if err := t.Safe(fn); err != nil {
		return err
	}
	return nil
}
