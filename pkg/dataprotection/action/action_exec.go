/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
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

	BackupPolicy *dpv1alpha1.BackupPolicy

	BackupMethod *dpv1alpha1.BackupMethod

	TargetPod *corev1.Pod
}

func (e *ExecAction) Execute(ctx ActionContext) (*dpv1alpha1.ActionStatus, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	podSpec, err := e.buildPodSpec(ctx)
	if err != nil {
		return nil, err
	}
	e.JobAction.PodSpec = podSpec
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

func (e *ExecAction) buildPodSpec(ctx ActionContext) (*corev1.PodSpec, error) {
	envVars, err := utils.BuildEnvVarByTargetVars(ctx.Ctx, ctx.Client, e.TargetPod, e.BackupPolicy.Spec.Target.Vars)
	if err != nil {
		return nil, err
	}
	envVars = utils.MergeEnv(envVars, e.BackupMethod.Env)
	container := &corev1.Container{
		Name:            e.Name,
		Image:           viper.GetString(constant.KBToolsImage),
		Env:             envVars,
		ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
		Command:         []string{"kubectl"},
		Args: append([]string{
			"-n",
			e.TargetPod.Namespace,
			"exec",
			e.TargetPod.Name,
			"-c",
			e.Container,
			"--",
		}, e.Command...),
	}
	intctrlutil.InjectZeroResourcesLimitsIfEmpty(container)
	return &corev1.PodSpec{
		RestartPolicy:      corev1.RestartPolicyNever,
		ServiceAccountName: e.ServiceAccountName,
		Containers:         []corev1.Container{*container},
		Volumes:            []corev1.Volume{},
		// tolerate all taints
		Tolerations: []corev1.Toleration{
			{
				Operator: corev1.TolerationOpExists,
			},
		},
		Affinity:     &corev1.Affinity{},
		NodeSelector: map[string]string{},
	}, nil
}

var _ Action = &ExecAction{}
