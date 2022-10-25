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
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	cmdexec "k8s.io/kubectl/pkg/cmd/exec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/podcmd"
	"k8s.io/kubectl/pkg/scheme"
)

// ExecInput is used to transfer custom Complete & Validate & AddFlags
type ExecInput struct {
	// Use cobra command use
	Use string

	// Short is the short description shown in the 'help' output.
	Short string

	// CompleteFunc optional, custom complete options
	Complete func(args []string) error

	// ValidateFunc optional, custom validate func
	Validate func() error

	// AddFlags func optional, custom build flags
	AddFlags func(*cobra.Command)
}

type ExecOptions struct {
	cmdexec.StreamOptions

	Input    *ExecInput
	Factory  cmdutil.Factory
	Executor cmdexec.RemoteExecutor
	Config   *restclient.Config

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
		Use:   o.Input.Use,
		Short: o.Input.Short,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.validate())
			cmdutil.CheckErr(o.run())
		},
	}
	if o.Input.AddFlags != nil {
		o.Input.AddFlags(cmd)
	}
	return cmd
}

// complete receive exec parameters
func (o *ExecOptions) complete(args []string) error {
	var err error
	o.Config, err = o.Factory.ToRESTConfig()
	if err != nil {
		return err
	}

	o.Namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	// custom complete function
	if o.Input.Complete != nil {
		if err = o.Input.Complete(args); err != nil {
			return err
		}
	}
	return nil
}

func (o *ExecOptions) validate() error {
	// custom validate function
	if o.Input.Validate != nil {
		if err := o.Input.Validate(); err != nil {
			return err
		}
	}

	if len(o.Pod.Name) == 0 {
		return fmt.Errorf("pod, type/name must be specified")
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

func (o *ExecOptions) run() error {
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
