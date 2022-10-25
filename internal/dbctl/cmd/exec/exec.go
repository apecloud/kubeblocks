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
	Complete func(f cmdutil.Factory, args []string) (*ExecParams, error)

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
	Params   *ExecParams
}

// ExecParams the vital parameters for kubectl exec cmd
type ExecParams struct {
	Pod           *corev1.Pod
	ContainerName string
	Command       []string
	Config        *restclient.Config
}

func NewExecOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, input *ExecInput) *ExecOptions {
	return &ExecOptions{
		Input:   input,
		Factory: f,
		StreamOptions: cmdexec.StreamOptions{
			IOStreams: streams,
			Stdin:     true,
			TTY:       true,
		},
		Executor: &cmdexec.DefaultRemoteExecutor{},
	}
}

func (e *ExecOptions) Build() *cobra.Command {
	cmd := &cobra.Command{
		Use:   e.Input.Use,
		Short: e.Input.Short,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(e.complete(e.Factory, args))
			cmdutil.CheckErr(e.validate())
			cmdutil.CheckErr(e.run())
		},
	}
	if e.Input.AddFlags != nil {
		e.Input.AddFlags(cmd)
	}
	return cmd
}

// complete receive kubect exec parameters
func (e *ExecOptions) complete(f cmdutil.Factory, args []string) error {
	customParams, err := e.Input.Complete(f, args)
	if err != nil || customParams == nil {
		return err
	}
	e.Params = customParams
	return nil
}

func (e *ExecOptions) validate() error {
	if e.Input.Validate != nil {
		if err := e.Input.Validate(); err != nil {
			return err
		}
	}

	if len(e.Params.Pod.Name) == 0 {
		return fmt.Errorf("pod, type/name must be specified")
	}
	if len(e.Params.Command) == 0 {
		return fmt.Errorf("you must specify at least one command for the container")
	}
	if e.Out == nil || e.ErrOut == nil {
		return fmt.Errorf("both output and error output must be provided")
	}
	return nil
}

func (e *ExecOptions) run() error {
	pod := e.Params.Pod
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return fmt.Errorf("cannot exec into a container in a completed pod; current phase is %s", pod.Status.Phase)
	}
	containerName := e.Params.ContainerName
	if len(containerName) == 0 {
		container, err := podcmd.FindOrDefaultContainerByName(pod, containerName, e.Quiet, e.ErrOut)
		if err != nil {
			return err
		}
		containerName = container.Name
	}
	command := e.Params.Command
	// ensure we can recover the terminal while attached
	t := e.SetupTTY()

	var sizeQueue remotecommand.TerminalSizeQueue
	if t.Raw {
		// this call spawns a goroutine to monitor/update the terminal size
		sizeQueue = t.MonitorSize(t.GetSize())

		// unset p.Err if it was previously set because both stdout and stderr go over p.Out when tty is
		// true
		e.ErrOut = nil
	}

	fn := func() error {
		restClient, err := restclient.RESTClientFor(e.Params.Config)
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
			Command:   command,
			Stdin:     e.Stdin,
			Stdout:    e.Out != nil,
			Stderr:    e.ErrOut != nil,
			TTY:       t.Raw,
		}, scheme.ParameterCodec)

		return e.Executor.Execute("POST", req.URL(), e.Params.Config, e.In, e.Out, e.ErrOut, t.Raw, sizeQueue)
	}

	if err := t.Safe(fn); err != nil {
		return err
	}
	return nil
}
