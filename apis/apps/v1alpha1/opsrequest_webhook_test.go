/*
Copyright (C) 2022 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package v1alpha1

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kubectl/pkg/util/storage"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("OpsRequest webhook", func() {

	var (
		randomStr                    = testCtx.GetRandomStr()
		clusterDefinitionName        = "opswebhook-mysql-definition-" + randomStr
		clusterVersionName           = "opswebhook-mysql-clusterversion-" + randomStr
		clusterVersionNameForUpgrade = "opswebhook-mysql-upgrade-" + randomStr
		clusterName                  = "opswebhook-mysql-" + randomStr
		opsRequestName               = "opswebhook-mysql-ops-" + randomStr
		replicaSetComponentName      = "replicasets"
		proxyComponentName           = "proxy"
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
		err = k8sClient.DeleteAllOf(ctx, &storagev1.StorageClass{})
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

	addClusterRequestAnnotation := func(cluster *Cluster, opsName string, toClusterPhase ClusterPhase) {
		clusterPatch := client.MergeFrom(cluster.DeepCopy())
		cluster.Annotations = map[string]string{
			opsRequestAnnotationKey: fmt.Sprintf(`[{"name":"%s","clusterPhase":"%s"}]`, opsName, toClusterPhase),
		}
		Expect(k8sClient.Patch(ctx, cluster, clusterPatch)).Should(Succeed())
	}

	createStorageClass := func(ctx context.Context, storageClassName string, isDefault string, allowVolumeExpansion bool) *storagev1.StorageClass {
		storageClass := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: storageClassName,
				Annotations: map[string]string{
					storage.IsDefaultStorageClassAnnotation: isDefault,
				},
			},
			Provisioner:          "kubernetes.io/no-provisioner",
			AllowVolumeExpansion: &allowVolumeExpansion,
		}
		err := testCtx.CheckedCreateObj(ctx, storageClass)
		if err != nil {
			fmt.Printf("create storage class error: %s\n", err.Error())
		}
		Expect(err).Should(BeNil())
		return storageClass
	}

	notFoundComponentsString := func(notFoundComponents string) string {
		return fmt.Sprintf("components: [%s] not found", notFoundComponents)
	}

	testUpgrade := func(cluster *Cluster) {
		opsRequest := createTestOpsRequest(clusterName, opsRequestName+"-upgrade", UpgradeType)

		By("By testing when spec.upgrade is null")
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("spec.upgrade"))

		By("By creating a new clusterVersion for upgrade")
		newClusterVersion := createTestClusterVersionObj(clusterDefinitionName, clusterVersionNameForUpgrade)
		Expect(testCtx.CreateObj(ctx, newClusterVersion)).Should(Succeed())

		By("By testing when target cluster version not exist")
		opsRequest.Spec.Upgrade = &Upgrade{ClusterVersionRef: clusterVersionName + "-not-exist"}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("not found"))

		By("Test Cluster Phase")
		opsRequest.Name = opsRequestName + "-upgrade-cluster-phase"
		opsRequest.Spec.Upgrade = &Upgrade{ClusterVersionRef: clusterVersionName}
		OpsRequestBehaviourMapper[UpgradeType] = OpsRequestBehaviour{
			FromClusterPhases: []ClusterPhase{RunningClusterPhase},
			ToClusterPhase:    SpecReconcilingClusterPhase, // original VersionUpgradingPhase,
		}
		// TODO: do VersionUpgradingPhase condition value check

		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("Upgrade is forbidden"))
		// update cluster phase to Running
		clusterPatch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Phase = RunningClusterPhase
		Expect(k8sClient.Status().Patch(ctx, cluster, clusterPatch)).Should(Succeed())

		By("Test existing other operations in cluster")
		// update cluster existing operations
		addClusterRequestAnnotation(cluster, "testOpsName", SpecReconcilingClusterPhase)
		Eventually(func() string {
			err := testCtx.CreateObj(ctx, opsRequest)
			if err == nil {
				return ""
			}
			return err.Error()
		}).Should(ContainSubstring("existing OpsRequest: testOpsName"))
		// test opsRequest reentry
		addClusterRequestAnnotation(cluster, opsRequest.Name, SpecReconcilingClusterPhase)
		By("By creating a upgrade opsRequest, it should be succeed")
		Eventually(func() bool {
			opsRequest.Spec.Upgrade.ClusterVersionRef = newClusterVersion.Name
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}).Should(BeTrue())

		// wait until OpsRequest created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: opsRequest.Name,
				Namespace: opsRequest.Namespace}, opsRequest)
			return err == nil
		}).Should(BeTrue())

		newClusterName := clusterName + "1"
		newCluster, _ := createTestCluster(clusterDefinitionName, clusterVersionName, newClusterName)
		Expect(testCtx.CheckedCreateObj(ctx, newCluster)).Should(Succeed())

		By("By testing Immutable when status.phase in Succeed")
		// if running in real cluster, the opsRequest will reconcile all the time.
		// so we should add eventually block.
		Eventually(func() bool {
			patch := client.MergeFrom(opsRequest.DeepCopy())
			opsRequest.Status.Phase = OpsSucceedPhase
			Expect(k8sClient.Status().Patch(ctx, opsRequest, patch)).Should(Succeed())

			patch = client.MergeFrom(opsRequest.DeepCopy())
			opsRequest.Spec.ClusterRef = newClusterName
			return Expect(k8sClient.Patch(ctx, opsRequest, patch).Error()).To(ContainSubstring("is forbidden when status.Phase is Succeed"))
		}).Should(BeTrue())
	}

	testVerticalScaling := func(cluster *Cluster) {
		verticalScalingList := []VerticalScaling{
			{
				ComponentOps:         ComponentOps{ComponentName: "vs-not-exist"},
				ResourceRequirements: corev1.ResourceRequirements{},
			},
			{
				ComponentOps: ComponentOps{ComponentName: proxyComponentName},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"cpu":    resource.MustParse("100m"),
						"memory": resource.MustParse("100Mi"),
					},
				},
			},
			{
				ComponentOps: ComponentOps{ComponentName: replicaSetComponentName},
				ResourceRequirements: corev1.ResourceRequirements{
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

		By("By testing verticalScaling opsRequest components is not exist")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, VerticalScalingType)
		opsRequest.Spec.VerticalScalingList = []VerticalScaling{verticalScalingList[0]}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring(notFoundComponentsString("vs-not-exist")))

		By("By testing verticalScaling opsRequest components is not consistent")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, VerticalScalingType)
		// [0] is not exist, and [1] is valid.
		opsRequest.Spec.VerticalScalingList = []VerticalScaling{verticalScalingList[0], verticalScalingList[1]}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("not found"))

		By("By testing verticalScaling opsRequest components partly")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, VerticalScalingType)
		opsRequest.Spec.VerticalScalingList = []VerticalScaling{verticalScalingList[1]}
		Expect(testCtx.CreateObj(ctx, opsRequest) == nil).Should(BeTrue())

		By("By testing requests cpu less than limits cpu")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, VerticalScalingType)
		opsRequest.Spec.VerticalScalingList = []VerticalScaling{verticalScalingList[2]}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("must be less than or equal to cpu limit"))
		Eventually(func() bool {
			opsRequest.Spec.VerticalScalingList[0].Requests[corev1.ResourceCPU] = resource.MustParse("100m")
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}).Should(BeTrue())
	}

	testVolumeExpansion := func(cluster *Cluster) {
		volumeExpansionList := []VolumeExpansion{
			{
				ComponentOps: ComponentOps{ComponentName: "ve-not-exist"},
				VolumeClaimTemplates: []OpsRequestVolumeClaimTemplate{
					{
						Name:    "data",
						Storage: resource.MustParse("2Gi"),
					},
				},
			},
			{
				ComponentOps: ComponentOps{ComponentName: replicaSetComponentName},
				VolumeClaimTemplates: []OpsRequestVolumeClaimTemplate{
					{
						Name:    "log",
						Storage: resource.MustParse("2Gi"),
					},
					{
						Name:    "data",
						Storage: resource.MustParse("2Gi"),
					},
				},
			},
			{
				ComponentOps: ComponentOps{ComponentName: replicaSetComponentName},
				VolumeClaimTemplates: []OpsRequestVolumeClaimTemplate{
					{
						Name:    "data",
						Storage: resource.MustParse("2Gi"),
					},
				},
			},
			{
				ComponentOps: ComponentOps{ComponentName: replicaSetComponentName},
				VolumeClaimTemplates: []OpsRequestVolumeClaimTemplate{
					{
						Name:    "log",
						Storage: resource.MustParse("2Gi"),
					},
					{
						Name:    "data",
						Storage: resource.MustParse("2Gi"),
					},
				},
			},
		}

		By("By testing volumeExpansion - target component not exist")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, VolumeExpansionType)
		opsRequest.Spec.VolumeExpansionList = []VolumeExpansion{volumeExpansionList[0]}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring(notFoundComponentsString("ve-not-exist")))

		By("By testing volumeExpansion - target volume not exist")
		opsRequest.Spec.VolumeExpansionList = []VolumeExpansion{volumeExpansionList[1]}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("volumeClaimTemplates: [log] not found in component: replicasets"))

		By("By testing volumeExpansion - create a new storage class")
		storageClassName := "sc-test-volume-expansion"
		storageClass := createStorageClass(testCtx.Ctx, storageClassName, "false", true)
		Expect(storageClass != nil).Should(BeTrue())

		By("By testing volumeExpansion - has no pvc")
		for _, compSpec := range cluster.Spec.ComponentSpecs {
			for _, vct := range compSpec.VolumeClaimTemplates {
				Expect(vct.Spec.StorageClassName == nil).Should(BeTrue())
			}
		}
		opsRequest.Spec.VolumeExpansionList = []VolumeExpansion{volumeExpansionList[2]}
		notSupportMsg := "volumeClaimTemplate: [data] not support volume expansion in component: replicasets, you can view infos by command: kubectl get sc"
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring(notSupportMsg))
		// TODO
		By("testing volumeExpansion - pvc exists")
		// TODO
		By("By testing volumeExpansion - (TODO)use specified storage class")
		// Eventually(func() bool {
		// 	 opsRequest.Spec.VolumeExpansionList = []VolumeExpansion{volumeExpansionList[3]}
		// 	 Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(BeNil())
		// }).Should(BeTrue())
	}

	testHorizontalScaling := func(clusterDef *ClusterDefinition, cluster *Cluster) {
		hScalingList := []HorizontalScaling{
			{
				ComponentOps: ComponentOps{ComponentName: "hs-not-exist"},
				Replicas:     2,
			},
			{
				ComponentOps: ComponentOps{ComponentName: proxyComponentName},
				Replicas:     2,
			},
			{
				ComponentOps: ComponentOps{ComponentName: replicaSetComponentName},
				Replicas:     2,
			},
		}

		By("By testing horizontalScaling - delete component proxy from cluster definition which is exist in cluster")
		patch := client.MergeFrom(clusterDef.DeepCopy())
		// delete component proxy from cluster definition
		if clusterDef.Spec.ComponentDefs[0].Name == proxyComponentName {
			clusterDef.Spec.ComponentDefs = clusterDef.Spec.ComponentDefs[1:]
		} else {
			clusterDef.Spec.ComponentDefs = clusterDef.Spec.ComponentDefs[:1]
		}
		Expect(k8sClient.Patch(ctx, clusterDef, patch)).Should(Succeed())
		Eventually(func() bool {
			tmp := &ClusterDefinition{}
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDef.Name, Namespace: clusterDef.Namespace}, tmp)
			return len(tmp.Spec.ComponentDefs) == 1
		}).Should(BeTrue())

		By("By testing horizontalScaling - target component not exist")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, HorizontalScalingType)
		opsRequest.Spec.HorizontalScalingList = []HorizontalScaling{hScalingList[0]}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring(notFoundComponentsString("hs-not-exist")))

		By("By testing horizontalScaling - target component not exist partly")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, HorizontalScalingType)
		opsRequest.Spec.HorizontalScalingList = []HorizontalScaling{hScalingList[0], hScalingList[2]}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring(notFoundComponentsString("hs-not-exist")))

		By("By testing horizontalScaling. if api is legal, it will create successfully")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, HorizontalScalingType)
		Eventually(func() bool {
			opsRequest.Spec.HorizontalScalingList = []HorizontalScaling{hScalingList[2]}
			return testCtx.CheckedCreateObj(ctx, opsRequest) == nil
		}).Should(BeTrue())

		By("test min, max is zero")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, HorizontalScalingType)
		Eventually(func() bool {
			opsRequest.Spec.HorizontalScalingList = []HorizontalScaling{hScalingList[2]}
			opsRequest.Spec.HorizontalScalingList[0].Replicas = 5
			return testCtx.CheckedCreateObj(ctx, opsRequest) == nil
		}).Should(BeTrue())
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
		}).Should(BeTrue())

		patch := client.MergeFrom(opsRequest.DeepCopy())
		opsRequest.Labels["test"] = "test-ops"
		Expect(k8sClient.Patch(ctx, opsRequest, patch)).Should(Succeed())
	}

	testRestart := func(cluster *Cluster) *OpsRequest {
		By("By testing restart when componentNames is not correct")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, RestartType)
		opsRequest.Spec.RestartList = []ComponentOps{
			{ComponentName: "replicasets1"},
		}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring(notFoundComponentsString("replicasets1")))

		By("By testing restart. if api is legal, it will create successfully")
		Eventually(func() bool {
			opsRequest.Spec.RestartList[0].ComponentName = replicaSetComponentName
			err := testCtx.CheckedCreateObj(ctx, opsRequest)
			return err == nil
		}).Should(BeTrue())
		return opsRequest
	}

	Context("When clusterVersion create and update", func() {
		It("Should webhook validate passed", func() {
			By("By create a clusterDefinition")

			clusterDef := &ClusterDefinition{}
			// wait until ClusterDefinition and ClusterVersion created
			Eventually(func() bool {
				clusterDef, _ = createTestClusterDefinitionObj(clusterDefinitionName)
				Expect(testCtx.CheckedCreateObj(ctx, clusterDef)).Should(Succeed())
				By("By creating a clusterVersion")
				clusterVersion := createTestClusterVersionObj(clusterDefinitionName, clusterVersionName)
				err := testCtx.CheckedCreateObj(ctx, clusterVersion)
				return err == nil
			}).Should(BeTrue())

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
			}).Should(BeTrue())

			testUpgrade(cluster)

			testVerticalScaling(cluster)

			testVolumeExpansion(cluster)

			testHorizontalScaling(clusterDef, cluster)

			opsRequest = testRestart(cluster)

			testWhenClusterDeleted(cluster, opsRequest)
		})
	})
})

func createTestOpsRequest(clusterName, opsRequestName string, opsType OpsType) *OpsRequest {
	randomStr, _ := password.Generate(6, 0, 0, true, false)
	opsRequestYaml := fmt.Sprintf(`
apiVersion: apps.kubeblocks.io/v1alpha1
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
