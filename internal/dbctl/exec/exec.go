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

package exec

import (
	"context"
	"fmt"

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
)

// ExecInput is used to transfer custom Complete & Validate & AddFlags
type ExecInput struct {
	// Use cobra command use
	Use string

	// Short is the short description shown in the 'help' output.
	Short string

	// Example use example for cluster logs
	Example string

	// CompleteFunc optional, custom Complete options
	Complete func(args []string) error

	// ValidateFunc optional, custom Validate func
	Validate func() error

	// AddFlags Func optional, custom build flags
	AddFlags func(*cobra.Command)

	// RunFunc optional, custom Run logic and return false or error means no need to exec, conversely return true will continue run exec
	Run func() (bool, error)
}

type ExecOptions struct {
	cmdexec.StreamOptions

	Input     *ExecInput
	Factory   cmdutil.Factory
	Executor  cmdexec.RemoteExecutor
	Config    *restclient.Config
	ClientSet *kubernetes.Clientset

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

func (o *ExecOptions) Build(input *ExecInput) *cobra.Command {
	o.Input = input
	cmd := &cobra.Command{
		Use:     o.Input.Use,
		Short:   o.Input.Short,
		Example: o.Input.Example,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	if o.Input.AddFlags != nil {
		o.Input.AddFlags(cmd)
	}
	return cmd
}

// Complete receive exec parameters
func (o *ExecOptions) Complete(args []string) error {
	var err error
	o.Config, err = o.Factory.ToRESTConfig()
	if err != nil {
		return err
	}

	o.Namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.ClientSet, err = o.Factory.KubernetesClientSet()
	if err != nil {
		return err
	}

	// custom Complete function
	if o.Input.Complete != nil {
		if err = o.Input.Complete(args); err != nil {
			return err
		}
	}
	return nil
}

func (o *ExecOptions) Validate() error {
	var err error

	// custom Validate function
	if o.Input.Validate != nil {
		if err = o.Input.Validate(); err != nil {
			return err
		}
	}

	// pod is not get, try to get it by pod name
	if o.Pod == nil {
		if len(o.PodName) > 0 {
			if o.Pod, err = o.ClientSet.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.PodName, metav1.GetOptions{}); err != nil {
				return err
			}
		}
	}

	if o.Pod == nil {
		return fmt.Errorf("failed to get the pod to execute")
	}
	if len(o.Command) == 0 && o.Input.Run == nil {
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
	containerName := o.ContainerName
	if len(containerName) == 0 {
		container, err := podcmd.FindOrDefaultContainerByName(o.Pod, containerName, o.Quiet, o.ErrOut)
		if err != nil {
			return err
		}
		containerName = container.Name
	}
	o.ContainerName = containerName

	return nil
}

func (o *ExecOptions) Run() error {
	// custom run logic and direct return
	if o.Input.Run != nil {
		if continueExec, err := o.Input.Run(); err != nil || !continueExec {
			return err
		}
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
