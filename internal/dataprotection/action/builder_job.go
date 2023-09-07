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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

type JobActionBuilder struct {
	Name            string
	JobName         string
	Namespace       string
	NodeName        string
	BackupOffLimit  int32
	Env             []corev1.EnvVar
	EnvFrom         []corev1.EnvFromSource
	Job             *dpv1alpha1.JobAction
	Volumes         []corev1.Volume
	VolumeMounts    []corev1.VolumeMount
	RuntimeSettings *dpv1alpha1.RuntimeSettings
}

var _ Builder = &JobActionBuilder{}

func (j *JobActionBuilder) Build() Action {
	return &KubeJob{
		Name: j.Name,
		ObjectMeta: metav1.ObjectMeta{
			Name:      j.JobName,
			Namespace: j.Namespace,
		},
		PodSpec:      j.buildPodSpec(),
		BackOffLimit: j.BackupOffLimit,
	}
}

func (j *JobActionBuilder) buildPodSpec() *corev1.PodSpec {
	runAsUser := int64(0)
	container := corev1.Container{
		Name:            j.Name,
		Image:           j.Job.Image,
		Command:         j.Job.Command,
		Env:             j.Env,
		EnvFrom:         j.EnvFrom,
		VolumeMounts:    j.VolumeMounts,
		ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolptr.False(),
			RunAsUser:                &runAsUser,
		},
	}

	if j.RuntimeSettings != nil {
		container.Resources = j.RuntimeSettings.Resources
	}

	intctrlutil.InjectZeroResourcesLimitsIfEmpty(&container)

	return &corev1.PodSpec{
		Containers:    []corev1.Container{container},
		NodeName:      j.NodeName,
		Volumes:       j.Volumes,
		RestartPolicy: corev1.RestartPolicyNever,

		// tolerate all taints
		Tolerations: []corev1.Toleration{
			{
				Operator: corev1.TolerationOpExists,
			},
		},
	}
}
