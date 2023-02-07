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

package dbaas

import (
	"context"
	"fmt"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
	"github.com/apecloud/kubeblocks/test/testdata"
)

// CreateRestartOpsRequest creates a OpsRequest of restart type for testing.
func CreateRestartOpsRequest(ctx context.Context, testCtx testutil.TestContext, clusterName, opsRequestName string, componentNames []string) *dbaasv1alpha1.OpsRequest {
	opsBytes, err := testdata.GetTestDataFileContent("operations/restart.yaml")
	if err != nil {
		return nil
	}
	opsRequestYaml := fmt.Sprintf(string(opsBytes), opsRequestName, clusterName, clusterName, componentNames)
	ops := &dbaasv1alpha1.OpsRequest{}
	gomega.Expect(yaml.Unmarshal([]byte(opsRequestYaml), ops)).Should(gomega.Succeed())
	return CreateOpsRequest(ctx, testCtx, ops)
}

// NewOpsRequestObj only generates the OpsRequest Object, instead of actually creating this resource.
func NewOpsRequestObj(opsRequestName, namespace, clusterName string, opsType dbaasv1alpha1.OpsType) *dbaasv1alpha1.OpsRequest {
	return &dbaasv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opsRequestName,
			Namespace: namespace,
		},
		Spec: dbaasv1alpha1.OpsRequestSpec{
			ClusterRef: clusterName,
			Type:       opsType,
		},
	}
}

// CreateOpsRequest calls the api to create the OpsRequest resource.
func CreateOpsRequest(ctx context.Context, testCtx testutil.TestContext, opsRequest *dbaasv1alpha1.OpsRequest) *dbaasv1alpha1.OpsRequest {
	gomega.Expect(testCtx.CreateObj(ctx, opsRequest)).Should(gomega.Succeed())
	// wait until cluster created
	newOps := &dbaasv1alpha1.OpsRequest{}
	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: opsRequest.Name,
			Namespace: testCtx.DefaultNamespace}, newOps)).Should(gomega.Succeed())
	}, timeout, interval).Should(gomega.Succeed())
	return newOps
}

// GetOpsRequestCompPhase gets the component phase of testing OpsRequest  for verification.
func GetOpsRequestCompPhase(ctx context.Context, testCtx testutil.TestContext, opsName, componentName string) func(g gomega.Gomega) dbaasv1alpha1.Phase {
	return func(g gomega.Gomega) dbaasv1alpha1.Phase {
		tmpOps := &dbaasv1alpha1.OpsRequest{}
		g.Expect(testCtx.Cli.Get(ctx, client.ObjectKey{Name: opsName,
			Namespace: testCtx.DefaultNamespace}, tmpOps)).Should(gomega.Succeed())
		statusComponents := tmpOps.Status.Components
		if statusComponents == nil {
			return ""
		}
		return statusComponents[componentName].Phase
	}
}

// GetOpsRequestPhase gets the testing opsRequest phase for verification.
func GetOpsRequestPhase(testCtx *testutil.TestContext, opsKey types.NamespacedName) func(gomega.Gomega) dbaasv1alpha1.Phase {
	return func(g gomega.Gomega) dbaasv1alpha1.Phase {
		tmpOps := &dbaasv1alpha1.OpsRequest{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, opsKey, tmpOps)).To(gomega.Succeed())
		return tmpOps.Status.Phase
	}
}
