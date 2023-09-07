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
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// KubeExec is an action that executes a command on a pod.
type KubeExec struct {
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

func (e *KubeExec) GetName() string {
	return e.Name
}

func (e *KubeExec) Type() ActionType {
	return ActionTypeExec
}

func (e *KubeExec) Execute(ctx Context) error {
	if err := e.validate(); err != nil {
		return err
	}

	req := ctx.RestClient.Post().
		Resource("pods").
		Namespace(e.Namespace).
		Name(e.PodName).
		SubResource("Exec")

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
		return err
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
		return errors.Errorf("timed out after %v", e.Timeout.Duration)
	}
	return err
}

func (e *KubeExec) validate() error {
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

var _ Action = &KubeExec{}
