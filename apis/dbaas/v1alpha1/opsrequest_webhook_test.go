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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/yaml"
)

var _ = Describe("OpsRequest webhook", func() {

	var (
		randomStr                    = testCtx.GetRandomStr()
		clusterDefinitionName        = "opswebhook-mysql-definition-" + randomStr
		clusterVersionName           = "opswebhook-mysql-clusterversion-" + randomStr
		clusterVersionNameForUpgrade = "opswebhook-mysql-upgrade-" + randomStr
		clusterName                  = "opswebhook-mysql-" + randomStr
		opsRequestName               = "opswebhook-mysql-ops-" + randomStr
		timeout                      = time.Second * 10
		interval                     = time.Second
		replicaSetComponentName      = "replicasets"
	)
	cleanupObjects := func() {
		// Add any setup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &OpsRequest{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &ClusterVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
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

	testUpgrade := func(cluster *Cluster) {
		opsRequest := createTestOpsRequest(clusterName, opsRequestName+"-upgrade", UpgradeType)
		By("By creating a clusterVersion for upgrade")
		newClusterVersion := createTestClusterVersionObj(clusterDefinitionName, clusterVersionNameForUpgrade)
		Expect(testCtx.CreateObj(ctx, newClusterVersion)).Should(Succeed())

		By("By testing when cluster not support upgrade")
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("ClusterVersion must be greater than 1"))
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

		By("By testing when spec.upgrade is null")
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("spec.upgrade"))

		By("By testing spec.upgrade.clusterVersionRef when it equals Cluster.spec.clusterVersionRef")
		opsRequest.Spec.Upgrade = &Upgrade{ClusterVersionRef: clusterVersionName}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("can not equals Cluster.spec.clusterVersionRef"))

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
			opsRequest.Spec.Upgrade.ClusterVersionRef = newClusterVersion.Name
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		// wait until OpsRequest created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: opsRequest.Name,
				Namespace: opsRequest.Namespace}, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		newClusterName := clusterName + "1"
		newCluster, _ := createTestCluster(clusterDefinitionName, clusterVersionName, newClusterName)
		Expect(testCtx.CheckedCreateObj(ctx, newCluster)).Should(Succeed())

		By("By testing Immutable when status.phase in Succeed")
		// if running in real cluster, the opsRequest will reconcile all the time.
		// so we should add eventually block.
		Eventually(func() bool {
			patch = client.MergeFrom(opsRequest.DeepCopy())
			opsRequest.Status.Phase = SucceedPhase
			Expect(k8sClient.Status().Patch(ctx, opsRequest, patch)).Should(Succeed())

			patch = client.MergeFrom(opsRequest.DeepCopy())
			opsRequest.Spec.ClusterRef = newClusterName
			return Expect(k8sClient.Patch(ctx, opsRequest, patch).Error()).To(ContainSubstring("update OpsRequest is forbidden when status.Phase is Succeed"))
		}, timeout, interval).Should(BeTrue())
	}

	testVerticalScaling := func(cluster *Cluster) {
		// set cluster support verticalScaling
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Operations.VerticalScalable = []string{replicaSetComponentName}
		Expect(k8sClient.Status().Patch(ctx, cluster, patch)).Should(Succeed())
		// wait until patch succeed
		Eventually(func() bool {
			tmpCluster := &Cluster{}
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, tmpCluster)
			return len(cluster.Status.Operations.VerticalScalable) > 0
		}, timeout, interval).Should(BeTrue())

		By("By testing verticalScaling opsRequest components is not consistent")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, VerticalScalingType)
		verticalScaling := VerticalScaling{}
		verticalScaling.ComponentName = "proxy"
		verticalScaling.ResourceRequirements = &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				"cpu":    resource.MustParse("100m"),
				"memory": resource.MustParse("100Mi"),
			},
		}
		opsRequest.Spec.VerticalScalingList = []VerticalScaling{verticalScaling}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("not supported the VerticalScaling operation"))
		Eventually(func() bool {
			opsRequest.Spec.VerticalScalingList[0].ComponentName = replicaSetComponentName
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		By("By testing requests cpu less than limits cpu")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, VerticalScalingType)
		opsRequest.Spec.VerticalScalingList = []VerticalScaling{
			{
				ComponentOps: ComponentOps{ComponentName: replicaSetComponentName},
				ResourceRequirements: &corev1.ResourceRequirements{
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
			opsRequest.Spec.VerticalScalingList[0].Requests[corev1.ResourceCPU] = resource.MustParse("100m")
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	testVolumeExpansion := func(cluster *Cluster) {
		By("test not support volume expansion")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, VolumeExpansionType)
		opsRequest.Spec.VolumeExpansionList = []VolumeExpansion{
			{
				ComponentOps: ComponentOps{ComponentName: replicaSetComponentName},
				VolumeClaimTemplates: []OpsRequestVolumeClaimTemplate{
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
				Name:                     replicaSetComponentName,
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
		opsRequest.Spec.VolumeExpansionList[0].VolumeClaimTemplates[0].Name = "data1"
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("not support volume expansion"))

		By("By testing volumeExpansion. if api is legal, it will create successfully")
		Eventually(func() bool {
			opsRequest.Spec.VolumeExpansionList[0].VolumeClaimTemplates[0].Name = "data"
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	testHorizontalScaling := func(cluster *Cluster) {
		// set cluster support horizontalScaling
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Operations.HorizontalScalable = []OperationComponent{
			{
				Name: replicaSetComponentName,
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
			opsRequest.Spec.HorizontalScalingList = []HorizontalScaling{
				{
					ComponentOps: ComponentOps{ComponentName: replicaSetComponentName},
					Replicas:     2,
				},
			}
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		By("By testing horizontalScaling replica is not in [min,max]")
		opsRequest.Spec.HorizontalScalingList[0].Replicas = 4
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
			opsRequest.Spec.HorizontalScalingList = []HorizontalScaling{
				{
					ComponentOps: ComponentOps{ComponentName: "proxy"},
					Replicas:     5,
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
		Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: cluster.Namespace}, &Cluster{})
			return err != nil
		}, timeout, interval).Should(BeTrue())

		patch := client.MergeFrom(opsRequest.DeepCopy())
		opsRequest.Labels["test"] = "test-ops"
		Expect(k8sClient.Patch(ctx, opsRequest, patch)).Should(Succeed())
	}

	testRestart := func(cluster *Cluster) *OpsRequest {
		// set cluster support restart
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Operations.Restartable = []string{replicaSetComponentName}
		Expect(k8sClient.Status().Patch(ctx, cluster, patch)).Should(Succeed())
		// wait until patch succeed
		Eventually(func() bool {
			tmpCluster := &Cluster{}
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, tmpCluster)
			return len(cluster.Status.Operations.Restartable) > 0
		}, timeout, interval).Should(BeTrue())

		By("By testing restart when componentNames is not correct")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, RestartType)
		opsRequest.Spec.RestartList = []ComponentOps{
			{ComponentName: "replicasets1"},
		}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("not found in Cluster.spec.components[*].name"))

		By("By testing restart. if api is legal, it will create successfully")
		Eventually(func() bool {
			opsRequest.Spec.RestartList[0].ComponentName = replicaSetComponentName
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		return opsRequest
	}

	Context("When clusterVersion create and update", func() {
		It("Should webhook validate passed", func() {
			By("By create a clusterDefinition")

			// wait until ClusterDefinition and ClusterVersion created
			Eventually(func() bool {
				clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
				Expect(testCtx.CheckedCreateObj(ctx, clusterDef)).Should(Succeed())
				By("By creating a clusterVersion")
				clusterVersion := createTestClusterVersionObj(clusterDefinitionName, clusterVersionName)
				err := testCtx.CheckedCreateObj(ctx, clusterVersion)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			opsRequest := createTestOpsRequest(clusterName, opsRequestName, UpgradeType)
			cluster := &Cluster{}
			// wait until Cluster created
			Eventually(func() bool {
				By("By testing spec.clusterDef is legal")
				Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).ShouldNot(Succeed())
				By("By create a new cluster ")
				cluster, _ = createTestCluster(clusterDefinitionName, clusterVersionName, clusterName)
				err := testCtx.CheckedCreateObj(ctx, cluster)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			testUpgrade(cluster)

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
