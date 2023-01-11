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

package testutil

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockDeploymentReady mock deployment is ready
func MockDeploymentReady(deploy *appsv1.Deployment, rsAvailableReason string) {
	deploy.Status.AvailableReplicas = *deploy.Spec.Replicas
	deploy.Status.ReadyReplicas = *deploy.Spec.Replicas
	deploy.Status.Replicas = *deploy.Spec.Replicas
	deploy.Status.ObservedGeneration = deploy.Generation
	deploy.Status.UpdatedReplicas = *deploy.Spec.Replicas
	deploy.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:   appsv1.DeploymentProgressing,
			Reason: rsAvailableReason,
			Status: corev1.ConditionTrue,
		},
	}
}

// MockPodAvailable mock pod is available
func MockPodAvailable(pod *corev1.Pod, lastTransitionTime metav1.Time) {
	pod.Status.Conditions = []corev1.PodCondition{
		{
			Type:               corev1.PodReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: lastTransitionTime,
		},
	}
}
