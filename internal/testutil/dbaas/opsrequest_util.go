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
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

func CreateRestartOpsRequest(testCtx testutil.TestContext, clusterName, opsRequestName string, componentNames []string) *dbaasv1alpha1.OpsRequest {
	opsRequestYaml := fmt.Sprintf(`apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: %s
  labels:
    app.kubernetes.io/instance: %s
    app.kubernetes.io/managed-by: kubeblocks
  namespace: default
spec:
  clusterRef: %s
  componentOps:
  - componentNames: %v
  type: Restart`, opsRequestName, clusterName, clusterName, componentNames)
	ops := &dbaasv1alpha1.OpsRequest{}
	gomega.Expect(yaml.Unmarshal([]byte(opsRequestYaml), ops)).Should(gomega.Succeed())
	return CreateOpsRequest(testCtx, ops)
}

func GenerateOpsRequestObj(opsRequestName, clusterName string, opsType dbaasv1alpha1.OpsType) *dbaasv1alpha1.OpsRequest {
	opsYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: %s
  namespace: default
spec:
  clusterRef: %s
  type: %s`, opsRequestName, clusterName, opsType)
	opsRequest := &dbaasv1alpha1.OpsRequest{}
	_ = yaml.Unmarshal([]byte(opsYaml), opsRequest)
	return opsRequest
}

func CreateOpsRequest(testCtx testutil.TestContext, opsRequest *dbaasv1alpha1.OpsRequest) *dbaasv1alpha1.OpsRequest {
	gomega.Expect(testCtx.CreateObj(ctx, opsRequest)).Should(gomega.Succeed())
	// wait until cluster created
	newOps := &dbaasv1alpha1.OpsRequest{}
	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: opsRequest.Name,
			Namespace: testCtx.DefaultNamespace}, newOps)).Should(gomega.Succeed())
	}, timeout, interval).Should(gomega.Succeed())
	return newOps
}

// GetOpsRequestCompPhase get the component phase of OpsRequest.
func GetOpsRequestCompPhase(testCtx testutil.TestContext, opsName, componentName string) dbaasv1alpha1.Phase {
	tmpOps := &dbaasv1alpha1.OpsRequest{}
	err := testCtx.Cli.Get(ctx, client.ObjectKey{Name: opsName, Namespace: testCtx.DefaultNamespace}, tmpOps)
	if err != nil {
		return ""
	}
	statusComponents := tmpOps.Status.Components
	if statusComponents == nil {
		return ""
	}
	return statusComponents[componentName].Phase
}

// GetOpsRequestPhase get the opsRequest phase.
func GetOpsRequestPhase(testCtx testutil.TestContext, opsName string) func(g gomega.Gomega) dbaasv1alpha1.Phase {
	return func(g gomega.Gomega) dbaasv1alpha1.Phase {
		tmpOps := &dbaasv1alpha1.OpsRequest{}
		g.Expect(testCtx.Cli.Get(ctx, client.ObjectKey{Name: opsName, Namespace: testCtx.DefaultNamespace},
			tmpOps)).To(gomega.Succeed())
		return tmpOps.Status.Phase
	}
}
