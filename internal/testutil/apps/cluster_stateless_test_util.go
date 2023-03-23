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

	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

// MockStatelessComponentDeploy mocks a deployment workload of the stateless component.
func MockStatelessComponentDeploy(testCtx testutil.TestContext, clusterName, componentName string) *appsv1.Deployment {
	deployName := clusterName + "-" + componentName
	return NewDeploymentFactory(testCtx.DefaultNamespace, deployName, clusterName, componentName).SetMinReadySeconds(int32(10)).SetReplicas(int32(2)).
		AddContainer(corev1.Container{Name: DefaultNginxContainerName, Image: NginxImage}).Create(&testCtx).GetObject()
}

// MockStatelessPod mocks the pods of the deployment workload.
func MockStatelessPod(testCtx testutil.TestContext, deploy *appsv1.Deployment, clusterName, componentName, podName string) *corev1.Pod {
	var newRs *appsv1.ReplicaSet
	if deploy != nil {
		newRs = &appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				UID:  "ss-456",
				Name: deploy.Name + "-5847cb795c",
			},
		}
	}
	return NewPodFactory(testCtx.DefaultNamespace, podName).
		SetOwnerReferences("apps/v1", constant.ReplicaSetKind, newRs).
		AddAppInstanceLabel(clusterName).
		AddAppComponentLabel(componentName).
		AddAppManangedByLabel().
		AddContainer(corev1.Container{Name: DefaultNginxContainerName, Image: NginxImage}).
		Create(&testCtx).GetObject()
}
