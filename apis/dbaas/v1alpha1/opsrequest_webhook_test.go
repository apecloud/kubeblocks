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

package v1alpha1

import (
	"context"
	"fmt"
	"time"

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
		timeout                  = time.Second * 10
		interval                 = time.Second
	)

	testUpgrade := func(cluster *Cluster, opsRequest *OpsRequest) {

		By("By testing when cluster not support upgrade")
		Expect(k8sClient.Create(ctx, opsRequest)).ShouldNot(Succeed())
		// set cluster support upgrade
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Operations.Upgradable = true
		Expect(k8sClient.Status().Patch(ctx, cluster, patch)).Should(Succeed())
		// wait until patch succeed
		Eventually(func() bool {
			tmpCluster := &Cluster{}
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, tmpCluster)
			return tmpCluster.Status.Operations.Upgradable
		}, timeout, interval).Should(BeTrue())

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
		Eventually(func() bool {
			opsRequest.Spec.ClusterOps.Upgrade.AppVersionRef = newAppVersion.Name
			err := k8sClient.Create(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		// wait until OpsRequest created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: opsRequest.Name,
				Namespace: opsRequest.Namespace}, &OpsRequest{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
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
		// wait until patch succeed
		Eventually(func() bool {
			tmpCluster := &Cluster{}
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, tmpCluster)
			return len(cluster.Status.Operations.VerticalScalable) > 0
		}, timeout, interval).Should(BeTrue())

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
		Eventually(func() bool {
			opsRequest.Spec.ComponentOpsList[0].ComponentNames = []string{"replicaSets"}
			err := k8sClient.Create(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())
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
		// wait until patch succeed
		Eventually(func() bool {
			tmpCluster := &Cluster{}
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, tmpCluster)
			return len(cluster.Status.Operations.VolumeExpandable) > 0
		}, timeout, interval).Should(BeTrue())

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
		Eventually(func() bool {
			opsRequest.Spec.ComponentOpsList[0].VolumeExpansion[0].Name = "data"
			err := k8sClient.Create(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())
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
		// wait until patch succeed
		Eventually(func() bool {
			tmpCluster := &Cluster{}
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, tmpCluster)
			return len(cluster.Status.Operations.HorizontalScalable) > 0
		}, timeout, interval).Should(BeTrue())

		By("By testing horizontalScaling. if api is legal, it will create successfully")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, HorizontalScalingType)
		Eventually(func() bool {
			opsRequest.Spec.ComponentOpsList = []*ComponentOps{
				{ComponentNames: []string{"replicaSets"},
					HorizontalScaling: &HorizontalScaling{
						Replicas: 2,
					},
				},
			}
			err := k8sClient.Create(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		By("By testing horizontalScaling replica is not in [min,max]")
		opsRequest.Spec.ComponentOpsList[0].HorizontalScaling.Replicas = 4
		Expect(k8sClient.Create(ctx, opsRequest)).ShouldNot(Succeed())

	}

	testRestart := func(cluster *Cluster) {
		// set cluster support restart
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Operations.Restartable = []string{"replicaSets"}
		Expect(k8sClient.Status().Patch(ctx, cluster, patch)).Should(Succeed())
		// wait until patch succeed
		Eventually(func() bool {
			tmpCluster := &Cluster{}
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, tmpCluster)
			return len(cluster.Status.Operations.Restartable) > 0
		}, timeout, interval).Should(BeTrue())

		By("By testing restart when componentNames is not correct")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, RestartType)
		opsRequest.Spec.ComponentOpsList = []*ComponentOps{
			{ComponentNames: []string{"replicaSets1"}},
		}
		Expect(k8sClient.Create(ctx, opsRequest)).ShouldNot(Succeed())

		By("By testing restart. if api is legal, it will create successfully")
		Eventually(func() bool {
			opsRequest.Spec.ComponentOpsList[0].ComponentNames = []string{"replicaSets"}
			err := k8sClient.Create(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	Context("When appVersion create and update", func() {
		It("Should webhook validate passed", func() {
			By("By create a clusterDefinition")

			// wait until ClusterDefinition and AppVersion created
			Eventually(func() bool {
				clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
				Expect(k8sClient.Create(ctx, clusterDef)).Should(Succeed())
				By("By creating a appVersion")
				appVersion := createTestAppVersionObj(clusterDefinitionName, appVersionName)
				err := k8sClient.Create(ctx, appVersion)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			opsRequest := createTestOpsRequest(clusterName, opsRequestName, UpgradeType)
			cluster := &Cluster{}
			// wait until Cluster created
			Eventually(func() bool {
				By("By testing spec.clusterDef is legal")
				Expect(k8sClient.Create(ctx, opsRequest)).ShouldNot(Succeed())
				By("By create a new cluster ")
				cluster, _ = createTestCluster(clusterDefinitionName, appVersionName, clusterName)
				err := k8sClient.Create(ctx, cluster)
				return err == nil
			}, timeout, interval).Should(BeTrue())

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
apiVersion: dbaas.kubeblocks.io/v1alpha1
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
