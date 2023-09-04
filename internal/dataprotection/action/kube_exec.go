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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KubeExec is an action that executes a command on a pod.
type KubeExec struct {
	// Name is the name of the action.
	Name string

	// PodName is the name of the pod to execute the command on.
	PodName string

	// Namespace is the namespace of the pod to execute the command on.
	Namespace string

	// Command is the command to execute.
	Command []string

	// Container is the container to execute the command on.
	Container string

	// ErrorMode is the error mode to use. If set to Fail, the action will fail
	// if the command fails.
	ErrorMode ErrorMode

	// Timeout is the timeout for the command.
	Timeout metav1.Duration
}

func (e *KubeExec) Execute() error {
	//TODO implement me
	panic("implement me")
}

var _ Action = &KubeExec{}
