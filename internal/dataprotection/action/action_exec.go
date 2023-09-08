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

package action

import (
	"bytes"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

// ExecAction is an action that executes a command on a pod.
type ExecAction struct {
	// Name is the Name of the action.
	Name string

	// PodName is the Name of the pod to execute the command on.
	PodName string

	// Namespace is the Namespace of the pod to execute the command on.
	Namespace string

	// Command is the command to execute.
	Command []string

	// Container is the container to execute the command on.
	Container string

	// Timeout is the timeout for the command.
	Timeout metav1.Duration
}

func (e *ExecAction) GetName() string {
	return e.Name
}

func (e *ExecAction) Type() dpv1alpha1.ActionType {
	return dpv1alpha1.ActionTypeExec
}

func (e *ExecAction) Execute(ctx Context) (*dpv1alpha1.ActionStatus, error) {
	sb := newStatusBuilder(e)
	handleErr := func(err error) (*dpv1alpha1.ActionStatus, error) {
		return sb.withErr(err).build(), err
	}

	if err := e.validate(); err != nil {
		return handleErr(err)
	}

	kc, err := kubernetes.NewForConfig(ctx.RestClientConfig)
	if err != nil {
		return handleErr(err)
	}

	req := kc.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(e.Namespace).
		Name(e.PodName).
		SubResource("ExecAction")

	// if container not specified, exec will use the first container in the pod
	if e.Container != "" {
		req.Param("container", e.Container)
	}

	req.VersionedParams(&corev1.PodExecOptions{
		Container: e.Container,
		Command:   e.Command,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(ctx.RestClientConfig, "POST", req.URL())
	if err != nil {
		return handleErr(err)
	}

	var stdout, stderr bytes.Buffer
	streamOptions := remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	}

	errCh := make(chan error)
	go func() {
		err = executor.StreamWithContext(ctx.Ctx, streamOptions)
		errCh <- err
	}()

	var timeoutCh <-chan time.Time
	if e.Timeout.Duration > 0 {
		timer := time.NewTimer(e.Timeout.Duration)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	select {
	case err = <-errCh:
	case <-timeoutCh:
		return handleErr(errors.Errorf("timed out after %v", e.Timeout.Duration))
	}

	if err != nil {
		return handleErr(err)
	}
	return sb.phase(dpv1alpha1.ActionPhaseCompleted).completionTimestamp(nil).build(), nil
}

func (e *ExecAction) validate() error {
	if e.PodName == "" {
		return errors.New("pod Name is required")
	}
	if e.Namespace == "" {
		return errors.New("Namespace is required")
	}
	if len(e.Command) == 0 {
		return errors.New("command is required")
	}
	return nil
}

var _ Action = &ExecAction{}
