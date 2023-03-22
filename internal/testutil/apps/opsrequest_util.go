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
	"context"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

// CreateRestartOpsRequest creates a OpsRequest of restart type for testing.
func CreateRestartOpsRequest(testCtx testutil.TestContext, clusterName, opsRequestName string, componentNames []string) *appsv1alpha1.OpsRequest {
	opsRequest := NewOpsRequestObj(opsRequestName, testCtx.DefaultNamespace, clusterName, appsv1alpha1.RestartType)
	componentList := make([]appsv1alpha1.ComponentOps, len(componentNames))
	for i := range componentNames {
		componentList[i] = appsv1alpha1.ComponentOps{ComponentName: componentNames[i]}
	}
	opsRequest.Spec.RestartList = componentList
	return CreateK8sResource(testCtx, opsRequest).(*appsv1alpha1.OpsRequest)
}

// NewOpsRequestObj only generates the OpsRequest Object, instead of actually creating this resource.
func NewOpsRequestObj(opsRequestName, namespace, clusterName string, opsType appsv1alpha1.OpsType) *appsv1alpha1.OpsRequest {
	return &appsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opsRequestName,
			Namespace: namespace,
		},
		Spec: appsv1alpha1.OpsRequestSpec{
			ClusterRef: clusterName,
			Type:       opsType,
		},
	}
}

// CreateOpsRequest calls the api to create the OpsRequest resource.
func CreateOpsRequest(ctx context.Context, testCtx testutil.TestContext, opsRequest *appsv1alpha1.OpsRequest) *appsv1alpha1.OpsRequest {
	gomega.Expect(testCtx.CreateObj(ctx, opsRequest)).Should(gomega.Succeed())
	// wait until cluster created
	gomega.Eventually(CheckObjExists(&testCtx, client.ObjectKeyFromObject(opsRequest), opsRequest, true)).Should(gomega.Succeed())
	return opsRequest
}

// GetOpsRequestCompPhase gets the component phase of testing OpsRequest  for verification.
func GetOpsRequestCompPhase(ctx context.Context, testCtx testutil.TestContext, opsName, componentName string) func(g gomega.Gomega) appsv1alpha1.Phase {
	return func(g gomega.Gomega) appsv1alpha1.Phase {
		tmpOps := &appsv1alpha1.OpsRequest{}
		g.Expect(testCtx.Cli.Get(ctx, client.ObjectKey{Name: opsName,
			Namespace: testCtx.DefaultNamespace}, tmpOps)).Should(gomega.Succeed())
		if tmpOps.Status.Components == nil {
			return ""
		}
		return tmpOps.Status.Components[componentName].Phase
	}
}

// GetOpsRequestPhase gets the testing opsRequest phase for verification.
func GetOpsRequestPhase(testCtx *testutil.TestContext, opsKey types.NamespacedName) func(gomega.Gomega) appsv1alpha1.OpsPhase {
	return func(g gomega.Gomega) appsv1alpha1.OpsPhase {
		tmpOps := &appsv1alpha1.OpsRequest{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, opsKey, tmpOps)).To(gomega.Succeed())
		return tmpOps.Status.Phase
	}
}
