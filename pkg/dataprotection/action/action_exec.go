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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// ExecAction is an action that executes a command on a pod.
// This action will create a job to execute the command.
type ExecAction struct {
	JobAction

	// PodName is the Name of the pod to execute the command on.
	PodName string

	// Namespace is the Namespace of the pod to execute the command on.
	Namespace string

	// Command is the command to execute.
	Command []string

	// Container is the container to execute the command on.
	Container string

	// ServiceAccountName is the service account to use to build the job object.
	ServiceAccountName string

	// Timeout is the timeout for the command.
	Timeout metav1.Duration
}

func (e *ExecAction) Execute(ctx Context) (*dpv1alpha1.ActionStatus, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	e.JobAction.PodSpec = e.buildPodSpec()
	return e.JobAction.Execute(ctx)
}

func (e *ExecAction) validate() error {
	if e.PodName == "" {
		return errors.New("pod name is required")
	}
	if e.Namespace == "" {
		return errors.New("namespace is required")
	}
	if len(e.Command) == 0 {
		return errors.New("command is required")
	}
	return nil
}

func (e *ExecAction) buildPodSpec() *corev1.PodSpec {
	return &corev1.PodSpec{
		RestartPolicy:      corev1.RestartPolicyNever,
		ServiceAccountName: e.ServiceAccountName,
		Containers: []corev1.Container{
			{
				Name:            e.Name,
				Image:           viper.GetString(constant.KBToolsImage),
				ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
				Command:         []string{"kubectl"},
				Args: append([]string{
					"-n",
					e.Namespace,
					"exec",
					e.PodName,
					"-c",
					e.Container,
					"--",
				}, e.Command...),
			},
		},
		Volumes: []corev1.Volume{},
		// tolerate all taints
		Tolerations: []corev1.Toleration{
			{
				Operator: corev1.TolerationOpExists,
			},
		},
		Affinity:     &corev1.Affinity{},
		NodeSelector: map[string]string{},
	}
}

var _ Action = &ExecAction{}
