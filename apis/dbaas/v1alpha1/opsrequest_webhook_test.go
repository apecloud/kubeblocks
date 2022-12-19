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
		ctx                      = context.Background()
	)
	cleanupObjects := func() {
		// Add any setup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &OpsRequest{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &AppVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	}
	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		cleanupObjects()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanupObjects()
	})

	testUpgrade := func(cluster *Cluster, opsRequest *OpsRequest) {
		By("By testing when cluster not support upgrade")
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("appversion must be greater than 1"))
		// set cluster support upgrade
		patch := client.MergeFrom(cluster.DeepCopy())
		if cluster.Status.Operations == nil {
			cluster.Status.Operations = &Operations{}
		}
		cluster.Status.Operations.Upgradable = true
		Expect(k8sClient.Status().Patch(ctx, cluster, patch)).Should(Succeed())
		// wait until patch succeed
		Eventually(func() bool {
			tmpCluster := &Cluster{}
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, tmpCluster)
			return tmpCluster.Status.Operations.Upgradable
		}, timeout, interval).Should(BeTrue())

		By("By testing when spec.clusterOps is null")
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("spec.clusterOps.upgrade"))

		By("By testing spec.clusterOps.upgrade.appVersionRef when it equals Cluster.spec.appVersionRef")
		opsRequest.Spec.ClusterOps = &ClusterOps{Upgrade: &Upgrade{
			AppVersionRef: appVersionName,
		}}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("can not equals Cluster.spec.appVersionRef"))

		By("By creating a appVersion for upgrade")
		newAppVersion := createTestAppVersionObj(clusterDefinitionName, appVersionNameForUpgrade)
		Expect(testCtx.CreateObj(ctx, newAppVersion)).Should(Succeed())

		By("Test Cluster Phase")
		OpsRequestBehaviourMapper[UpgradeType] = OpsRequestBehaviour{
			FromClusterPhases: []Phase{RunningPhase},
			ToClusterPhase:    UpdatingPhase,
		}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("Upgrade is forbidden"))
		// update cluster phase to Running
		clusterPatch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Phase = RunningPhase
		Expect(k8sClient.Status().Patch(ctx, cluster, clusterPatch)).Should(Succeed())

		By("Test existing other operations in cluster")
		// update cluster existing operations
		clusterPatch = client.MergeFrom(cluster.DeepCopy())
		cluster.Annotations = map[string]string{
			opsRequestAnnotationKey: `{"Updating":"testOpsName"}`,
		}
		Expect(k8sClient.Patch(ctx, cluster, clusterPatch)).Should(Succeed())
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("Existing OpsRequest: testOpsName"))
		// delete annotations cluster phase to Running
		clusterPatch = client.MergeFrom(cluster.DeepCopy())
		cluster.Annotations = nil
		Expect(k8sClient.Patch(ctx, cluster, clusterPatch)).Should(Succeed())

		By("By creating a upgrade opsRequest, it should be succeed")
		Eventually(func() bool {
			opsRequest.Spec.ClusterOps.Upgrade.AppVersionRef = newAppVersion.Name
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		// wait until OpsRequest created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: opsRequest.Name,
				Namespace: opsRequest.Namespace}, &OpsRequest{})
			return err == nil
		}, timeout, interval).Should(BeTrue())

		By("By testing Immutable when status.phase in Succeed")
		opsRequest.Status.Phase = SucceedPhase
		Expect(k8sClient.Status().Update(ctx, opsRequest)).Should(Succeed())
		opsRequest.Spec.ClusterRef = "test"
		Expect(k8sClient.Update(ctx, opsRequest).Error()).To(ContainSubstring("update OpsRequest is forbidden when status.Phase is Succeed"))
	}

	testVerticalScaling := func(cluster *Cluster) {
		// set cluster support verticalScaling
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Operations.VerticalScalable = []string{"replicasets"}
		Expect(k8sClient.Status().Patch(ctx, cluster, patch)).Should(Succeed())
		// wait until patch succeed
		Eventually(func() bool {
			tmpCluster := &Cluster{}
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, tmpCluster)
			return len(cluster.Status.Operations.VerticalScalable) > 0
		}, timeout, interval).Should(BeTrue())

		By("By testing verticalScaling opsRequest components is not consistent")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, VerticalScalingType)
		opsRequest.Spec.ComponentOpsList = []ComponentOps{
			{ComponentNames: []string{"proxy1", "proxy"},
				VerticalScaling: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"cpu":    resource.MustParse("100m"),
						"memory": resource.MustParse("100Mi"),
					},
				},
			},
		}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring(""))
		Eventually(func() bool {
			opsRequest.Spec.ComponentOpsList[0].ComponentNames = []string{"replicasets"}
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		By("By testing requests cpu less than limits cpu")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, VerticalScalingType)
		opsRequest.Spec.ComponentOpsList = []ComponentOps{
			{ComponentNames: []string{"replicasets"},
				VerticalScaling: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"cpu":    resource.MustParse("200m"),
						"memory": resource.MustParse("100Mi"),
					},
					Limits: corev1.ResourceList{
						"cpu":    resource.MustParse("100m"),
						"memory": resource.MustParse("100Mi"),
					},
				},
			},
		}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("must be less than or equal to cpu limit"))
		Eventually(func() bool {
			opsRequest.Spec.ComponentOpsList[0].VerticalScaling.Requests[corev1.ResourceCPU] = resource.MustParse("100m")
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	testVolumeExpansion := func(cluster *Cluster) {
		By("test not support volume expansion")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, VolumeExpansionType)
		opsRequest.Spec.ComponentOpsList = []ComponentOps{
			{ComponentNames: []string{"replicasets"},
				VolumeExpansion: []VolumeExpansion{
					{
						Name:    "data",
						Storage: resource.MustParse("2Gi"),
					},
				},
			},
		}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring(`Invalid value: "VolumeExpansion": not supported in Cluster`))
		// set cluster support volumeExpansion
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Operations.VolumeExpandable = []OperationComponent{
			{
				Name:                     "replicasets",
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
		opsRequest.Spec.ComponentOpsList[0].VolumeExpansion[0].Name = "data1"
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("not support volume expansion"))

		By("By testing volumeExpansion. if api is legal, it will create successfully")
		Eventually(func() bool {
			opsRequest.Spec.ComponentOpsList[0].VolumeExpansion[0].Name = "data"
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	testHorizontalScaling := func(cluster *Cluster) {
		// set cluster support horizontalScaling
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Operations.HorizontalScalable = []OperationComponent{
			{
				Name: "replicasets",
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
			opsRequest.Spec.ComponentOpsList = []ComponentOps{
				{ComponentNames: []string{"replicasets"},
					HorizontalScaling: &HorizontalScaling{
						Replicas: 2,
					},
				},
			}
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		By("By testing horizontalScaling replica is not in [min,max]")
		opsRequest.Spec.ComponentOpsList[0].HorizontalScaling.Replicas = 4
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("replicas must less than"))

		By("test min, max is zero")
		tmpCluster := &Cluster{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: cluster.Namespace}, tmpCluster)).Should(Succeed())
		patch = client.MergeFrom(tmpCluster.DeepCopy())
		tmpCluster.Status.Operations.HorizontalScalable = []OperationComponent{
			{
				Name: "proxy",
			},
		}
		Expect(k8sClient.Status().Patch(ctx, tmpCluster, patch)).Should(Succeed())
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, HorizontalScalingType)
		Eventually(func() bool {
			opsRequest.Spec.ComponentOpsList = []ComponentOps{
				{ComponentNames: []string{"proxy"},
					HorizontalScaling: &HorizontalScaling{
						Replicas: 5,
					},
				},
			}
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())

	}

	testWhenClusterDeleted := func(cluster *Cluster, opsRequest *OpsRequest) {
		By("delete cluster")
		newCluster := &Cluster{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: cluster.Namespace}, newCluster)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, newCluster)).Should(Succeed())

		By("test path labels")
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: cluster.Namespace}, &Cluster{})).ShouldNot(Succeed())
		patch := client.MergeFrom(opsRequest.DeepCopy())
		opsRequest.Labels["test"] = "test-ops"
		Expect(k8sClient.Patch(ctx, opsRequest, patch)).Should(Succeed())
	}

	testRestart := func(cluster *Cluster) *OpsRequest {
		// set cluster support restart
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Operations.Restartable = []string{"replicasets"}
		Expect(k8sClient.Status().Patch(ctx, cluster, patch)).Should(Succeed())
		// wait until patch succeed
		Eventually(func() bool {
			tmpCluster := &Cluster{}
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, tmpCluster)
			return len(cluster.Status.Operations.Restartable) > 0
		}, timeout, interval).Should(BeTrue())

		By("By testing restart when componentNames is not correct")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, RestartType)
		opsRequest.Spec.ComponentOpsList = []ComponentOps{
			{ComponentNames: []string{"replicasets1"}},
		}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("not found in Cluster.spec.components[*].name"))

		By("By testing restart. if api is legal, it will create successfully")
		Eventually(func() bool {
			opsRequest.Spec.ComponentOpsList[0].ComponentNames = []string{"replicasets"}
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		return opsRequest
	}

	Context("When appVersion create and update", func() {
		It("Should webhook validate passed", func() {
			By("By create a clusterDefinition")

			// wait until ClusterDefinition and AppVersion created
			Eventually(func() bool {
				clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
				Expect(testCtx.CheckedCreateObj(ctx, clusterDef)).Should(Succeed())
				By("By creating a appVersion")
				appVersion := createTestAppVersionObj(clusterDefinitionName, appVersionName)
				err := testCtx.CheckedCreateObj(ctx, appVersion)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			opsRequest := createTestOpsRequest(clusterName, opsRequestName, UpgradeType)
			cluster := &Cluster{}
			// wait until Cluster created
			Eventually(func() bool {
				By("By testing spec.clusterDef is legal")
				Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).ShouldNot(Succeed())
				By("By create a new cluster ")
				cluster, _ = createTestCluster(clusterDefinitionName, appVersionName, clusterName)
				err := testCtx.CheckedCreateObj(ctx, cluster)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			testUpgrade(cluster, opsRequest)

			testVerticalScaling(cluster)

			testVolumeExpansion(cluster)

			testHorizontalScaling(cluster)

			opsRequest = testRestart(cluster)

			testWhenClusterDeleted(cluster, opsRequest)

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
