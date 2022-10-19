/*
Copyright 2022 The KubeBlocks Authors

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
	"context"
	"fmt"

	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var _ = Describe("OpsRequest webhook", func() {
	var (
		clusterDefinitionName    = "opsrequest-webhook-mysql-definition"
		appVersionName           = "opsrequest-webhook-mysql-appversion"
		appVersionNameForUpgrade = "opsrequest-webhook-mysql-upgrade-appversion"
		clusterName              = "opsrequest-webhook-mysql"
		opsRequestName           = "opsrequest-webhook-mysql-ops"
	)

	testUpgrade := func(cluster *Cluster, opsRequest *OpsRequest) {

		By("By testing when cluster not support upgrade")
		Expect(k8sClient.Create(ctx, opsRequest)).ShouldNot(Succeed())
		// set cluster support upgrade
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Operations.Upgradable = true
		Expect(k8sClient.Status().Patch(ctx, cluster, patch)).Should(Succeed())

		By("By testing when spec.clusterOps is null")
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
		// wait until OpsRequest created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: opsRequest.Name,
				Namespace: opsRequest.Namespace}, &OpsRequest{})
			return err == nil
		}, 10, 1).Should(BeTrue())
		By("By testing Immutable when status.phase in (Running,Succeed)")
		opsRequest.Status.Phase = RunningPhase
		Expect(k8sClient.Status().Update(ctx, opsRequest)).Should(Succeed())
		opsRequest.Spec.ClusterRef = "test"
		Expect(k8sClient.Update(ctx, opsRequest)).ShouldNot(Succeed())
	}

	testVerticalScaling := func(cluster *Cluster) {
		// set cluster support verticalScaling
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Operations.VerticalScalable = []string{"replicaSets"}
		Expect(k8sClient.Status().Patch(ctx, cluster, patch)).Should(Succeed())

		By("By testing verticalScaling opsRequest components is not consistent")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, VerticalScalingType)
		opsRequest.Spec.ComponentOpsList = []*ComponentOps{
			{ComponentNames: []string{"proxy1", "proxy"},
				VerticalScaling: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"cpu":    resource.MustParse("100m"),
						"memory": resource.MustParse("100Mi"),
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, opsRequest)).ShouldNot(Succeed())
		opsRequest.Spec.ComponentOpsList[0].ComponentNames = []string{"replicaSets"}
		Expect(k8sClient.Create(ctx, opsRequest)).Should(Succeed())
	}

	testVolumeExpansion := func(cluster *Cluster) {
		// set cluster support volumeExpansion
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Operations.VolumeExpandable = []*OperationComponent{
			{
				Name:                     "replicaSets",
				VolumeClaimTemplateNames: []string{"data"},
			},
		}
		Expect(k8sClient.Status().Patch(ctx, cluster, patch)).Should(Succeed())

		By("By testing volumeExpansion volumeClaimTemplate name is not consistent")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, VolumeExpansionType)
		opsRequest.Spec.ComponentOpsList = []*ComponentOps{
			{ComponentNames: []string{"replicaSets"},
				VolumeExpansion: []VolumeExpansion{
					{
						Name:    "data1",
						Storage: "2Gi",
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, opsRequest)).ShouldNot(Succeed())

		By("By testing volumeExpansion. if api is legal, it will create successfully")
		opsRequest.Spec.ComponentOpsList[0].VolumeExpansion[0].Name = "data"
		Expect(k8sClient.Create(ctx, opsRequest)).Should(Succeed())
	}

	testHorizontalScaling := func(cluster *Cluster) {
		// set cluster support horizontalScaling
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Operations.HorizontalScalable = []*OperationComponent{
			{
				Name: "replicaSets",
				Min:  1,
				Max:  3,
			},
		}
		Expect(k8sClient.Status().Patch(ctx, cluster, patch)).Should(Succeed())

		By("By testing horizontalScaling replica is not in [min,max]")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, HorizontalScalingType)
		opsRequest.Spec.ComponentOpsList = []*ComponentOps{
			{ComponentNames: []string{"replicaSets"},
				HorizontalScaling: &HorizontalScaling{
					Replicas: 4,
				},
			},
		}
		Expect(k8sClient.Create(ctx, opsRequest)).ShouldNot(Succeed())

		By("By testing horizontalScaling. if api is legal, it will create successfully")
		opsRequest.Spec.ComponentOpsList[0].HorizontalScaling.Replicas = 2
		Expect(k8sClient.Create(ctx, opsRequest)).Should(Succeed())
	}

	testRestart := func(cluster *Cluster) {
		// set cluster support restart
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Operations.Restartable = []string{"replicaSets"}
		Expect(k8sClient.Status().Patch(ctx, cluster, patch)).Should(Succeed())

		By("By testing restart when componentNames is not correct")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, RestartType)
		opsRequest.Spec.ComponentOpsList = []*ComponentOps{
			{ComponentNames: []string{"replicaSets1"}},
		}
		Expect(k8sClient.Create(ctx, opsRequest)).ShouldNot(Succeed())

		By("By testing restart. if api is legal, it will create successfully")
		opsRequest.Spec.ComponentOpsList[0].ComponentNames = []string{"replicaSets"}
		Expect(k8sClient.Create(ctx, opsRequest)).Should(Succeed())
	}

	Context("When appVersion create and update", func() {
		It("Should webhook validate passed", func() {
			By("By create a clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(k8sClient.Create(ctx, clusterDef)).Should(Succeed())
			// wait until ClusterDefinition created
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefinitionName}, clusterDef)
				return err == nil
			}, 10, 1).Should(BeTrue())

			By("By creating a appVersion")
			appVersion := createTestAppVersionObj(clusterDefinitionName, appVersionName)
			Expect(k8sClient.Create(ctx, appVersion)).Should(Succeed())
			// wait until AppVersion created
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: appVersionName}, appVersion)
				return err == nil
			}, 10, 1).Should(BeTrue())

			By("By testing spec.clusterDef is legal")
			opsRequest := createTestOpsRequest(clusterName, opsRequestName, UpgradeType)
			Expect(k8sClient.Create(ctx, opsRequest)).ShouldNot(Succeed())

			By("By create a new cluster ")
			cluster, _ := createTestCluster(clusterDefinitionName, appVersionName, clusterName)
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())

			// wait until Cluster created
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: cluster.Namespace}, &Cluster{})
				return err == nil
			}, 10, 1).Should(BeTrue())

			testUpgrade(cluster, opsRequest)

			testVerticalScaling(cluster)

			testVolumeExpansion(cluster)

			testHorizontalScaling(cluster)

			testRestart(cluster)

		})
	})
})

func createTestOpsRequest(clusterName, opsRequestName string, opsType OpsType) *OpsRequest {
	randomStr, _ := password.Generate(6, 0, 0, true, false)
	opsRequestYaml := fmt.Sprintf(`
apiVersion: dbaas.infracreate.com/v1alpha1
kind: OpsRequest
metadata:
  name: %s
  namespace: default
spec:
  clusterRef: %s
  type: %s
`, opsRequestName+randomStr, clusterName, opsType)
	opsRequest := &OpsRequest{}
	_ = yaml.Unmarshal([]byte(opsRequestYaml), opsRequest)
	return opsRequest
}
