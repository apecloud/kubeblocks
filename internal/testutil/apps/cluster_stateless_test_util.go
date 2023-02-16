/*
Copyright ApeCloud, Inc.

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

package apps

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

// CreateStatelessCluster creates a cluster with a component of Stateless type for testing.
func CreateStatelessCluster(testCtx testutil.TestContext, clusterDefName, clusterVersionName, clusterName string, replicas int) *appsv1alpha1.Cluster {
	return CreateCustomizedObj(&testCtx, "stateless/cluster.yaml", &appsv1alpha1.Cluster{},
		CustomizeObjYAML(clusterVersionName, clusterDefName, clusterName, clusterVersionName, clusterDefName, replicas))
}

// MockStatelessComponentDeploy mocks a deployment workload of the stateless component.
func MockStatelessComponentDeploy(testCtx testutil.TestContext, clusterName, componentName string) *appsv1.Deployment {
	deployName := clusterName + "-" + componentName
	return CreateCustomizedObj(&testCtx, "stateless/deployment.yaml", &appsv1.Deployment{},
		CustomizeObjYAML(componentName, clusterName, deployName, componentName, clusterName, componentName, clusterName))
}

// MockStatelessPod mocks the pods of the deployment workload.
func MockStatelessPod(testCtx testutil.TestContext, deploy *appsv1.Deployment, clusterName, componentName, podName string) *corev1.Pod {
	return CreateCustomizedObj(&testCtx, "stateless/deployment_pod.yaml", &corev1.Pod{},
		CustomizeObjYAML(podName, componentName, clusterName), func(pod *corev1.Pod) {
			if deploy != nil {
				t := true
				pod.SetOwnerReferences([]metav1.OwnerReference{
					{APIVersion: "apps/v1", Kind: "Deployment", Controller: &t, BlockOwnerDeletion: &t, Name: deploy.Name, UID: deploy.UID},
				})
			}
		})
}
