/*
Copyright 2022.

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

package v1alpha1

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var _ = Describe("OpsRequest webhook", func() {
	var (
		clusterDefinitionName    = "opsrequest-webhook-mysql-definition"
		appVersionName           = "opsrequest-webhook-mysql-appversion"
		appVersionNameForUpgrade = "opsrequest-webhook-mysql-upgrade-appversion"
		clusterName              = "opsrequest-webhook-mysql"
		opsRequestName           = "opsrequest-webhook-mysql-ops"
		opsDefinitionName        = "opsrequest-webhook-ospdefinition"
	)
	Context("When appVersion create and update", func() {
		It("Should webhook validate passed", func() {
			By("By  create a clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(k8sClient.Create(ctx, clusterDef)).Should(Succeed())

			By("By creating a appVersion")
			appVersion := createTestAppVersionObj(clusterDefinitionName, appVersionName)
			Expect(k8sClient.Create(ctx, appVersion)).Should(Succeed())

			By("By testing spec.clusterDef is legal")
			opsRequest := createTestOpsRequest(clusterName, opsRequestName, UpgradeType)
			Expect(k8sClient.Create(ctx, opsRequest)).ShouldNot(Succeed())

			By("By create a new cluster ")
			cluster, _ := createTestCluster(clusterDefinitionName, appVersionName, clusterName)
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())

			By("By testing spec.clusterOps.upgrade is legal when it is null")
			Expect(k8sClient.Create(ctx, opsRequest)).ShouldNot(Succeed())

			By("By testing spec.clusterOps.upgrade.appVersionRef when it equals Cluster.spec.appVersionRef")
			opsRequest.Spec.ClusterOps = &ClusterOps{Upgrade: &Upgrade{
				AppVersionRef: appVersionName,
			}}
			Expect(k8sClient.Create(ctx, opsRequest)).ShouldNot(Succeed())

			By("By creating a appVersion for upgrade")
			newAppVersion := createTestAppVersionObj(clusterDefinitionName, appVersionNameForUpgrade)
			Expect(k8sClient.Create(ctx, newAppVersion)).Should(Succeed())

			By("By creating a upgrade opsRequest, it should be succeed")
			opsRequest.Spec.ClusterOps.Upgrade.AppVersionRef = appVersionNameForUpgrade
			Expect(k8sClient.Create(ctx, opsRequest)).Should(Succeed())

			By("By testing Immutable when status.phase in (Running,Succeed)")
			opsRequest.Status.Phase = RunningPhase
			Expect(k8sClient.Status().Update(ctx, opsRequest)).Should(Succeed())
			opsRequest.Spec.ClusterRef = "test"
			Expect(k8sClient.Update(ctx, opsRequest)).ShouldNot(Succeed())

			By("By testing verticalScaling opsRequest components is not consistent")
			opsDefinition := createTestOpsDefinition(clusterDefinitionName, opsDefinitionName, VerticalScalingType)
			Expect(k8sClient.Create(ctx, opsDefinition)).Should(Succeed())
			opsRequest = createTestOpsRequest(clusterName, opsRequestName, VerticalScalingType)
			opsRequest.Spec.ComponentOps = &ComponentOps{
				ComponentNames: []string{"proxy1", "proxy"},
				VerticalScaling: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"cpu":    resource.MustParse("100m"),
						"memory": resource.MustParse("100Mi"),
					},
				},
			}
			Expect(k8sClient.Create(ctx, opsRequest)).ShouldNot(Succeed())

		})
	})
})

func createTestOpsRequest(clusterName, opsRequestName string, opsType OpsType) *OpsRequest {
	opsRequestYaml := fmt.Sprintf(`
apiVersion: dbaas.infracreate.com/v1alpha1
kind: OpsRequest
metadata:
  name: %s
  namespace: default
spec:
  clusterRef: %s
  type: %s
`, opsRequestName, clusterName, opsType)
	opsRequest := &OpsRequest{}
	_ = yaml.Unmarshal([]byte(opsRequestYaml), opsRequest)
	return opsRequest
}
