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
	gomega.Expect(testCtx.CreateObj(context.Background(), ops)).Should(gomega.Succeed())
	// wait until opsRequest created
	gomega.Eventually(func() bool {
		err := testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: opsRequestName, Namespace: testCtx.DefaultNamespace}, ops)
		return err == nil
	}, timeout, interval).Should(gomega.BeTrue())
	return ops
}
