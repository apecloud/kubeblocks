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

package testutil

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockDeploymentReady mocks deployment is ready
func MockDeploymentReady(deploy *appsv1.Deployment, rsAvailableReason, rsName string) {
	deploy.Status.AvailableReplicas = *deploy.Spec.Replicas
	deploy.Status.ReadyReplicas = *deploy.Spec.Replicas
	deploy.Status.Replicas = *deploy.Spec.Replicas
	deploy.Status.ObservedGeneration = deploy.Generation
	deploy.Status.UpdatedReplicas = *deploy.Spec.Replicas
	deploy.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:    appsv1.DeploymentProgressing,
			Reason:  rsAvailableReason,
			Status:  corev1.ConditionTrue,
			Message: fmt.Sprintf(`ReplicaSet "%s" has successfully progressed.`, rsName),
		},
	}
}

// MockPodAvailable mocks pod is available
func MockPodAvailable(pod *corev1.Pod, lastTransitionTime metav1.Time) {
	pod.Status.Conditions = []corev1.PodCondition{
		{
			Type:               corev1.PodReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: lastTransitionTime,
		},
	}
}
