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

package apps

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/testutil"
)

// MockStatelessComponentDeploy mocks a deployment workload of the stateless component.
func MockStatelessComponentDeploy(testCtx *testutil.TestContext, clusterName, componentName string) *appsv1.Deployment {
	deployName := clusterName + "-" + componentName
	return NewDeploymentFactory(testCtx.DefaultNamespace, deployName, clusterName, componentName).SetMinReadySeconds(int32(10)).SetReplicas(int32(2)).
		AddContainer(corev1.Container{Name: DefaultNginxContainerName, Image: NginxImage}).Create(testCtx).GetObject()
}

// MockStatelessPod mocks the pods of the deployment workload.
func MockStatelessPod(testCtx *testutil.TestContext, deploy *appsv1.Deployment, clusterName, componentName, podName string) *corev1.Pod {
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
		AddAppManagedByLabel().
		AddContainer(corev1.Container{Name: DefaultNginxContainerName, Image: NginxImage}).
		Create(testCtx).GetObject()
}
