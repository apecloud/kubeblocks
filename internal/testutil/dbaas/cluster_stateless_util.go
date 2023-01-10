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

package dbaas

import (
	"context"
	"fmt"

	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
	"github.com/apecloud/kubeblocks/test/testdata"
)

// CreateStatelessCluster creates a cluster with a component of Stateless type for testing.
func CreateStatelessCluster(ctx context.Context, testCtx testutil.TestContext, clusterDefName, clusterVersionName, clusterName string) *dbaasv1alpha1.Cluster {
	clusterBytes, err := testdata.GetTestDataFileContent("stateless/cluster.yaml")
	if err != nil {
		return nil
	}
	clusterYaml := fmt.Sprintf(string(clusterBytes), clusterVersionName, clusterDefName, clusterName, clusterVersionName, clusterDefName)
	cluster := &dbaasv1alpha1.Cluster{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(gomega.Succeed())
	return CreateK8sResource(ctx, testCtx, cluster).(*dbaasv1alpha1.Cluster)
}

// MockStatelessComponentDeploy mocks a deployment workload of the stateless component.
func MockStatelessComponentDeploy(ctx context.Context, testCtx testutil.TestContext, clusterName, componentName string) *appsv1.Deployment {
	deployBytes, err := testdata.GetTestDataFileContent("stateless/deployment.yaml")
	if err != nil {
		return nil
	}
	deployName := clusterName + "-" + componentName
	deploymentYaml := fmt.Sprintf(string(deployBytes), componentName, clusterName, deployName, componentName, clusterName, componentName, clusterName)
	deploy := &appsv1.Deployment{}
	gomega.Expect(yaml.Unmarshal([]byte(deploymentYaml), deploy)).Should(gomega.Succeed())
	return CreateK8sResource(ctx, testCtx, deploy).(*appsv1.Deployment)
}

// MockStatelessPod mocks the pods of the deployment workload.
func MockStatelessPod(ctx context.Context, testCtx testutil.TestContext, clusterName, componentName, podName string) *corev1.Pod {
	podBytes, err := testdata.GetTestDataFileContent("stateless/deployment_pod.yaml")
	if err != nil {
		return nil
	}
	podYaml := fmt.Sprintf(string(podBytes), podName, componentName, clusterName)
	pod := &corev1.Pod{}
	gomega.Expect(yaml.Unmarshal([]byte(podYaml), pod)).Should(gomega.Succeed())
	return CreateK8sResource(ctx, testCtx, pod).(*corev1.Pod)
}
