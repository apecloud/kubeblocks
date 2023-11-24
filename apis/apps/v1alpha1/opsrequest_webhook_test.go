/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/storage"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("OpsRequest webhook", func() {
	const (
		componentName      = "replicasets"
		proxyComponentName = "proxy"
	)
	var (
		randomStr                    = testCtx.GetRandomStr()
		clusterDefinitionName        = "opswebhook-mysql-definition-" + randomStr
		componentDefinitionName      = "opswk-compdef-" + randomStr
		clusterVersionName           = "opswebhook-mysql-clusterversion-" + randomStr
		clusterVersionNameForUpgrade = "opswebhook-mysql-upgrade-" + randomStr
		clusterName                  = "opswebhook-mysql-" + randomStr
		opsRequestName               = "opswebhook-mysql-ops-" + randomStr
	)

	int32Ptr := func(i int32) *int32 {
		return &i
	}

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
		Expect(err).Should(BeNil())
		return storageClass
	}

	createPVC := func(clusterName, compName, storageClassName, vctName string, index int) *corev1.PersistentVolumeClaim {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-%s-%d", vctName, clusterName, compName, index),
				Namespace: testCtx.DefaultNamespace,
				Labels: map[string]string{
					constant.AppInstanceLabelKey:             clusterName,
					constant.VolumeClaimTemplateNameLabelKey: vctName,
					constant.KBAppComponentLabelKey:          compName,
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": resource.MustParse("1Gi"),
					},
				},
				StorageClassName: &storageClassName,
			},
		}
		Expect(testCtx.CheckedCreateObj(ctx, pvc)).ShouldNot(HaveOccurred())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pvc), pvc)).ShouldNot(HaveOccurred())
		patch := client.MergeFrom(pvc.DeepCopy())
		pvc.Status.Capacity = corev1.ResourceList{
			"storage": resource.MustParse("1Gi"),
		}
		Expect(k8sClient.Status().Patch(ctx, pvc, patch)).ShouldNot(HaveOccurred())
		return pvc
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
			ToClusterPhase:    UpdatingClusterPhase, // original VersionUpgradingPhase,
		}
		// TODO: do VersionUpgradingPhase condition value check

		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("Upgrade is forbidden"))
		// update cluster phase to Running
		clusterPatch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Phase = RunningClusterPhase
		Expect(k8sClient.Status().Patch(ctx, cluster, clusterPatch)).Should(Succeed())

		By("By creating a upgrade opsRequest, it should be succeed")
		opsRequest.Spec.Upgrade.ClusterVersionRef = newClusterVersion.Name
		Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(Succeed())
		Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: opsRequest.Name,
			Namespace: opsRequest.Namespace}, opsRequest)).Should(Succeed())

		By("Expect an error for cancelling this opsRequest")
		opsRequest.Spec.Cancel = true
		Expect(k8sClient.Update(context.Background(), opsRequest).Error()).Should(ContainSubstring("forbidden to cancel the opsRequest which type not in ['VerticalScaling','HorizontalScaling']"))
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
				ComponentOps: ComponentOps{ComponentName: componentName},
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

		By("expect successful")
		opsRequest.Spec.VerticalScalingList[0].Requests[corev1.ResourceCPU] = resource.MustParse("100m")
		Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(Succeed())

		By("test spec immutable")
		newClusterName := clusterName + "1"
		newCluster, _ := createTestCluster(clusterDefinitionName, clusterVersionName, newClusterName)
		Expect(testCtx.CheckedCreateObj(ctx, newCluster)).Should(Succeed())

		testSpecImmutable := func(phase OpsPhase) {
			By(fmt.Sprintf("spec is immutable when status.phase in %s", phase))
			patch := client.MergeFrom(opsRequest.DeepCopy())
			opsRequest.Status.Phase = phase
			Expect(k8sClient.Status().Patch(ctx, opsRequest, patch)).Should(Succeed())

			patch = client.MergeFrom(opsRequest.DeepCopy())
			opsRequest.Spec.Cancel = true
			Expect(k8sClient.Patch(ctx, opsRequest, patch).Error()).To(ContainSubstring(fmt.Sprintf("is forbidden when status.Phase is %s", phase)))
		}
		phaseList := []OpsPhase{OpsSucceedPhase, OpsFailedPhase, OpsCancelledPhase}
		for _, phase := range phaseList {
			testSpecImmutable(phase)
		}

		By("test spec immutable except for cancel")
		testSpecImmutableExpectForCancel := func(phase OpsPhase) {
			patch := client.MergeFrom(opsRequest.DeepCopy())
			opsRequest.Status.Phase = phase
			Expect(k8sClient.Status().Patch(ctx, opsRequest, patch)).Should(Succeed())

			patch = client.MergeFrom(opsRequest.DeepCopy())
			By(fmt.Sprintf("cancel opsRequest when ops phase is %s", phase))
			opsRequest.Spec.Cancel = !opsRequest.Spec.Cancel
			Expect(k8sClient.Patch(ctx, opsRequest, patch)).ShouldNot(HaveOccurred())

			By(fmt.Sprintf("expect an error for updating spec.ClusterRef when ops phase is %s", phase))
			opsRequest.Spec.ClusterRef = newClusterName
			Expect(k8sClient.Patch(ctx, opsRequest, patch).Error()).To(ContainSubstring("forbidden to update spec.clusterRef"))
		}

		phaseList = []OpsPhase{OpsCreatingPhase, OpsRunningPhase, OpsCancellingPhase}
		for _, phase := range phaseList {
			testSpecImmutableExpectForCancel(phase)
		}
	}

	testVolumeExpansion := func(cluster *Cluster) {
		getSingleVolumeExpansionList := func(compName, vctName, storage string) []VolumeExpansion {
			return []VolumeExpansion{
				{
					ComponentOps: ComponentOps{ComponentName: compName},
					VolumeClaimTemplates: []OpsRequestVolumeClaimTemplate{
						{
							Name:    vctName,
							Storage: resource.MustParse(storage),
						},
					},
				},
			}
		}
		defaultVCTName := "data"
		logVCTName := "log"
		targetStorage := "2Gi"
		By("By testing volumeExpansion - target component not exist")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, VolumeExpansionType)
		opsRequest.Spec.VolumeExpansionList = getSingleVolumeExpansionList("ve-not-exist", defaultVCTName, targetStorage)
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring(notFoundComponentsString("ve-not-exist")))

		By("By testing volumeExpansion - target volume not exist")
		volumeExpansionList := []VolumeExpansion{{
			ComponentOps: ComponentOps{ComponentName: componentName},
			VolumeClaimTemplates: []OpsRequestVolumeClaimTemplate{
				{
					Name:    logVCTName,
					Storage: resource.MustParse(targetStorage),
				},
				{
					Name:    defaultVCTName,
					Storage: resource.MustParse(targetStorage),
				},
			},
		},
		}
		opsRequest.Spec.VolumeExpansionList = volumeExpansionList
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("volumeClaimTemplates: [log] not found in component: " + componentName))

		By("By testing volumeExpansion - storageClass do not support volume expansion")
		volumeExpansionList = getSingleVolumeExpansionList(componentName, defaultVCTName, targetStorage)
		opsRequest.Spec.VolumeExpansionList = volumeExpansionList
		notSupportMsg := fmt.Sprintf("volumeClaimTemplate: [data] not support volume expansion in component: %s, you can view infos by command: kubectl get sc", componentName)
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring(notSupportMsg))

		By("testing volumeExpansion - storageClass supports volume expansion")
		storageClassName := "standard"
		storageClass := createStorageClass(testCtx.Ctx, storageClassName, "true", true)
		Expect(storageClass).ShouldNot(BeNil())
		// mock to create pvc
		createPVC(clusterName, componentName, storageClassName, defaultVCTName, 0)

		By("create a pvc and storageClass does not support volume expansion")
		storageClassName1 := "standard1"
		storageClass1 := createStorageClass(testCtx.Ctx, storageClassName1, "false", false)
		Expect(storageClass1).ShouldNot(BeNil())
		createPVC(clusterName, componentName, storageClassName1, logVCTName, 0)

		By("testing volumeExpansion with smaller storage, expect an error occurs")
		opsRequest.Spec.VolumeExpansionList = getSingleVolumeExpansionList(componentName, defaultVCTName, "500Mi")
		Expect(testCtx.CreateObj(ctx, opsRequest)).Should(HaveOccurred())
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring(`requested storage size of volumeClaimTemplate "data" can not less than status.capacity.storage "1Gi"`))

		By("testing other volumeExpansion opsRequest exists")
		opsRequest.Spec.VolumeExpansionList = getSingleVolumeExpansionList(componentName, defaultVCTName, targetStorage)
		Expect(testCtx.CreateObj(ctx, opsRequest)).ShouldNot(HaveOccurred())
		// mock ops to running
		patch := client.MergeFrom(opsRequest.DeepCopy())
		opsRequest.Status.Phase = OpsRunningPhase
		Expect(k8sClient.Status().Patch(ctx, opsRequest, patch)).ShouldNot(HaveOccurred())
		// create another ops
		opsRequest1 := createTestOpsRequest(clusterName, opsRequestName+"1", VolumeExpansionType)
		opsRequest1.Spec.VolumeExpansionList = getSingleVolumeExpansionList(componentName, defaultVCTName, "3Gi")
		Expect(testCtx.CreateObj(ctx, opsRequest1).Error()).Should(ContainSubstring("existing other VolumeExpansion OpsRequest"))
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
				ComponentOps: ComponentOps{ComponentName: componentName},
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
		tmp := &ClusterDefinition{}
		_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDef.Name, Namespace: clusterDef.Namespace}, tmp)
		Expect(len(tmp.Spec.ComponentDefs)).Should(Equal(1))

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
		opsRequest.Spec.HorizontalScalingList = []HorizontalScaling{hScalingList[2]}
		Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(Succeed())

		By("test min, max is zero")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, HorizontalScalingType)
		opsRequest.Spec.HorizontalScalingList = []HorizontalScaling{hScalingList[2]}
		opsRequest.Spec.HorizontalScalingList[0].Replicas = 5
		Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(Succeed())
	}

	testSwitchover := func(clusterDef *ClusterDefinition, cluster *Cluster) {
		switchoverList := []Switchover{
			{
				ComponentOps: ComponentOps{ComponentName: "switchover-component-not-exist"},
				InstanceName: "*",
			},
			{
				ComponentOps: ComponentOps{ComponentName: componentName},
				InstanceName: "",
			},
			{
				ComponentOps: ComponentOps{ComponentName: componentName},
				InstanceName: "switchover-instance-name-not-exist",
			},
			{
				ComponentOps: ComponentOps{ComponentName: componentName},
				InstanceName: "*",
			},
			{
				ComponentOps: ComponentOps{ComponentName: componentName},
				InstanceName: fmt.Sprintf("%s-%s-0", cluster.Name, componentName),
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
		tmp := &ClusterDefinition{}
		_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDef.Name, Namespace: clusterDef.Namespace}, tmp)
		Expect(len(tmp.Spec.ComponentDefs)).Should(Equal(1))

		By("By testing switchover - target component not exist")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, SwitchoverType)
		opsRequest.Spec.SwitchoverList = []Switchover{switchoverList[0]}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring(notFoundComponentsString("switchover-component-not-exist")))

		By("By testing switchover - target switchover.Instance cannot be empty")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, SwitchoverType)
		opsRequest.Spec.SwitchoverList = []Switchover{switchoverList[1]}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("switchover.instanceName"))

		By("By testing switchover - clusterDefinition has no switchoverSpec and do not support switchover")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, SwitchoverType)
		opsRequest.Spec.SwitchoverList = []Switchover{switchoverList[3]}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("does not support switchover"))

		By("By testing switchover - target switchover.Instance cannot be empty")
		patch = client.MergeFrom(clusterDef.DeepCopy())
		commandExecutorEnvItem := CommandExecutorEnvItem{
			Image: "",
		}
		commandExecutorItem := CommandExecutorItem{
			Command: []string{"echo", "hello"},
			Args:    []string{},
		}
		switchoverSpec := &SwitchoverSpec{
			WithCandidate: &SwitchoverAction{
				CmdExecutorConfig: &CmdExecutorConfig{
					CommandExecutorEnvItem: commandExecutorEnvItem,
					CommandExecutorItem:    commandExecutorItem,
				},
			},
			WithoutCandidate: &SwitchoverAction{
				CmdExecutorConfig: &CmdExecutorConfig{
					CommandExecutorEnvItem: commandExecutorEnvItem,
					CommandExecutorItem:    commandExecutorItem,
				},
			},
		}
		clusterDef.Spec.ComponentDefs[0].SwitchoverSpec = switchoverSpec
		Expect(k8sClient.Patch(ctx, clusterDef, patch)).Should(Succeed())
		tmp = &ClusterDefinition{}
		_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDef.Name, Namespace: clusterDef.Namespace}, tmp)
		Expect(len(tmp.Spec.ComponentDefs)).Should(Equal(1))

		By("By testing switchover - switchover.InstanceName is * and should succeed ")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, SwitchoverType)
		opsRequest.Spec.SwitchoverList = []Switchover{switchoverList[3]}
		Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(Succeed())
	}

	testSwitchoverWithCompDef := func(clusterDef *ClusterDefinition, compDef *ComponentDefinition, cluster *Cluster) {
		switchoverList := []Switchover{
			{
				ComponentOps: ComponentOps{ComponentName: "switchover-component-not-exist"},
				InstanceName: "*",
			},
			{
				ComponentOps: ComponentOps{ComponentName: componentName},
				InstanceName: "",
			},
			{
				ComponentOps: ComponentOps{ComponentName: componentName},
				InstanceName: "switchover-instance-name-not-exist",
			},
			{
				ComponentOps: ComponentOps{ComponentName: componentName},
				InstanceName: "*",
			},
			{
				ComponentOps: ComponentOps{ComponentName: componentName},
				InstanceName: fmt.Sprintf("%s-%s-0", cluster.Name, componentName),
			},
		}

		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Spec.ComponentSpecs[0].ComponentDef = compDef.Name
		cluster.Spec.ComponentSpecs[1].ComponentDef = compDef.Name
		Expect(k8sClient.Patch(ctx, cluster, patch)).Should(Succeed())

		By("By testing switchover - target component not exist")
		opsRequest := createTestOpsRequest(clusterName, opsRequestName, SwitchoverType)
		opsRequest.Spec.SwitchoverList = []Switchover{switchoverList[0]}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring(notFoundComponentsString("switchover-component-not-exist")))

		By("By testing switchover - target switchover.Instance cannot be empty")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, SwitchoverType)
		opsRequest.Spec.SwitchoverList = []Switchover{switchoverList[1]}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("switchover.instanceName"))

		By("By testing switchover - componentDefinition has no ComponentSwitchoverSpec and do not support switchover")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, SwitchoverType)
		opsRequest.Spec.SwitchoverList = []Switchover{switchoverList[3]}
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("does not support switchover"))

		By("By testing switchover - target switchover.Instance cannot be empty")
		patch = client.MergeFrom(compDef.DeepCopy())
		commandExecutorEnvItem := &CommandExecutorEnvItem{
			Image: "test-image",
			Env:   []corev1.EnvVar{},
		}
		commandExecutorItem := &CommandExecutorItem{
			Command: []string{"echo", "hello"},
			Args:    []string{},
		}
		scriptSpecSelectors := []ScriptSpecSelector{
			{
				Name: "test-mock-cm",
			},
			{
				Name: "test-mock-cm-2",
			},
		}
		lifeCycleAction := &ComponentLifecycleActions{
			Switchover: &ComponentSwitchoverSpec{
				WithCandidate: &Action{
					Image: commandExecutorEnvItem.Image,
					Env:   commandExecutorEnvItem.Env,
					Exec: &ExecAction{
						Command: commandExecutorItem.Command,
						Args:    commandExecutorItem.Args,
					},
				},
				WithoutCandidate: &Action{
					Image: commandExecutorEnvItem.Image,
					Env:   commandExecutorEnvItem.Env,
					Exec: &ExecAction{
						Command: commandExecutorItem.Command,
						Args:    commandExecutorItem.Args,
					},
				},
				ScriptSpecSelectors: scriptSpecSelectors,
			},
		}

		compDef.Spec.LifecycleActions = lifeCycleAction
		Expect(k8sClient.Patch(ctx, compDef, patch)).Should(Succeed())
		tmp := &ComponentDefinition{}
		_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: compDef.Name}, tmp)
		Expect(tmp.Spec.LifecycleActions.Switchover).ShouldNot(BeNil())

		By("By testing switchover - switchover.InstanceName is * and should succeed ")
		opsRequest = createTestOpsRequest(clusterName, opsRequestName, SwitchoverType)
		opsRequest.Spec.SwitchoverList = []Switchover{switchoverList[3]}
		Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(Succeed())
	}

	testWhenClusterDeleted := func(cluster *Cluster, opsRequest *OpsRequest) {
		By("delete cluster")
		newCluster := &Cluster{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: cluster.Namespace}, newCluster)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, newCluster)).Should(Succeed())

		By("test path labels")
		Eventually(k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: cluster.Namespace}, &Cluster{})).Should(HaveOccurred())

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
		opsRequest.Spec.RestartList[0].ComponentName = componentName
		Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(Succeed())
		return opsRequest
	}

	testReconfiguring := func(cluster *Cluster, clusterDef *ClusterDefinition) {
		opsRequest := createTestOpsRequest(clusterName, opsRequestName+"-reconfiguring", ReconfiguringType)

		createReconfigureObj := func(compName string) *Reconfigure {
			return &Reconfigure{
				ComponentOps: ComponentOps{ComponentName: compName},
				Configurations: []ConfigurationItem{{Name: "for-test",
					Keys: []ParameterConfig{{
						Key: "test",
						Parameters: []ParameterPair{{
							Key:   "test",
							Value: func(t string) *string { return &t }("test")}},
					}},
				}}}
		}

		By("By testing when spec.reconfiguring is null")
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("spec.reconfigure"))

		By("By testing when target cluster definition not exist")
		opsRequest.Spec.Reconfigure = createReconfigureObj(componentName + "-not-exist")
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("not found"))
		opsRequest.Spec.Reconfigure = createReconfigureObj(componentName)
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("not found"))

		By("By creating a configmap")
		Expect(testCtx.CheckedCreateObj(ctx, createTestConfigmap(fmt.Sprintf("%s-%s-%s", opsRequest.Spec.ClusterRef, componentName, "for-test")))).Should(Succeed())
		opsRequest.Spec.Reconfigure = createReconfigureObj(componentName)
		Expect(testCtx.CreateObj(ctx, opsRequest).Error()).To(ContainSubstring("not found in configmap"))

		By("By test reconfiguring for file content")
		r := createReconfigureObj(componentName)
		r.Configurations[0].Keys[0].FileContent = "new context"
		opsRequest.Spec.Reconfigure = r
		Expect(testCtx.CreateObj(ctx, opsRequest)).To(Succeed())
	}

	Context("When clusterVersion create and update", func() {
		It("Should webhook validate passed", func() {
			// wait until ClusterDefinition and ClusterVersion created
			By("By create a clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(testCtx.CheckedCreateObj(ctx, clusterDef)).Should(Succeed())
			By("By creating a clusterVersion")
			clusterVersion := createTestClusterVersionObj(clusterDefinitionName, clusterVersionName)
			Expect(testCtx.CheckedCreateObj(ctx, clusterVersion)).Should(Succeed())
			By("By creating a componentDefinition")
			compDef := createTestComponentDefObj(componentDefinitionName)
			Expect(testCtx.CheckedCreateObj(ctx, compDef)).Should(Succeed())

			opsRequest := createTestOpsRequest(clusterName, opsRequestName, UpgradeType)

			// create Cluster
			By("By testing spec.clusterDef is legal")
			Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(HaveOccurred())
			By("By create a new cluster ")
			cluster, _ := createTestCluster(clusterDefinitionName, clusterVersionName, clusterName)
			Expect(testCtx.CheckedCreateObj(ctx, cluster)).Should(Succeed())

			testUpgrade(cluster)

			testVerticalScaling(cluster)

			testVolumeExpansion(cluster)

			testHorizontalScaling(clusterDef, cluster)

			testSwitchover(clusterDef, cluster)

			testReconfiguring(cluster, clusterDef)

			opsRequest = testRestart(cluster)

			testWhenClusterDeleted(cluster, opsRequest)
		})

		It("test switchover with component definition", func() {
			// wait until ClusterDefinition and ClusterVersion created
			By("By create a clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(testCtx.CheckedCreateObj(ctx, clusterDef)).Should(Succeed())
			By("By creating a clusterVersion")
			clusterVersion := createTestClusterVersionObj(clusterDefinitionName, clusterVersionName)
			Expect(testCtx.CheckedCreateObj(ctx, clusterVersion)).Should(Succeed())
			By("By creating a componentDefinition")
			compDef := createTestComponentDefObj(componentDefinitionName)
			Expect(testCtx.CheckedCreateObj(ctx, compDef)).Should(Succeed())

			opsRequest := createTestOpsRequest(clusterName, opsRequestName, UpgradeType)

			// create Cluster
			By("By testing spec.clusterDef is legal")
			Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(HaveOccurred())
			By("By create a new cluster ")
			cluster, _ := createTestCluster(clusterDefinitionName, clusterVersionName, clusterName)
			Expect(testCtx.CheckedCreateObj(ctx, cluster)).Should(Succeed())

			testSwitchoverWithCompDef(clusterDef, compDef, cluster)
		})

		It("check datascript opts", func() {
			OpsRequestBehaviourMapper[DataScriptType] = OpsRequestBehaviour{
				FromClusterPhases: []ClusterPhase{RunningClusterPhase},
			}

			By("By create a clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(testCtx.CheckedCreateObj(ctx, clusterDef)).Should(Succeed())
			By("By creating a clusterVersion")
			clusterVersion := createTestClusterVersionObj(clusterDefinitionName, clusterVersionName)
			Expect(testCtx.CheckedCreateObj(ctx, clusterVersion)).Should(Succeed())

			opsRequest := createTestOpsRequest(clusterName, opsRequestName, DataScriptType)
			opsRequest.Spec.ScriptSpec = &ScriptSpec{
				ComponentOps: ComponentOps{ComponentName: componentName},
			}

			// create Cluster
			By("By testing spec.clusterDef is legal")
			Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(HaveOccurred())
			By("By create a new cluster ")
			cluster, _ := createTestCluster(clusterDefinitionName, clusterVersionName, clusterName)
			Expect(testCtx.CheckedCreateObj(ctx, cluster)).Should(Succeed())

			By("By testing dataScript without script, should fail")
			Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(HaveOccurred())

			By("By testing dataScript, with script, no wait, should fail")
			opsRequest.Spec.ScriptSpec.Script = []string{"create database test;"}
			Expect(testCtx.CheckedCreateObj(ctx, opsRequest).Error()).To(ContainSubstring("DataScript is forbidden"))

			By("By testing dataScript, with illegal configmap, should fail")
			opsRequest.Spec.ScriptSpec.ScriptFrom = &ScriptFrom{
				ConfigMapRef: []corev1.ConfigMapKeySelector{
					{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-cm",
						},
						Key: "createdb",
					},
				},
			}

			Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(HaveOccurred())

			By("By testing dataScript, with illegal scriptFrom, should fail")
			opsRequest.Spec.ScriptSpec.ScriptFrom.SecretRef = nil
			Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(HaveOccurred())

			// patch cluster to running
			By("By patching cluster to running")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).Should(Succeed())
			clusterPatch := client.MergeFrom(cluster.DeepCopy())
			cluster.Status.Phase = RunningClusterPhase
			Expect(k8sClient.Status().Patch(ctx, cluster, clusterPatch)).Should(Succeed())

			Eventually(func() bool {
				_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)
				return cluster.Status.Phase == RunningClusterPhase
			}).Should(BeTrue())

			opsRequest.Spec.ScriptSpec.Script = []string{"create database test;"}
			opsRequest.Spec.ScriptSpec.ScriptFrom = nil
			opsRequest.Spec.TTLSecondsBeforeAbort = int32Ptr(0)
			Expect(testCtx.CheckedCreateObj(ctx, opsRequest)).Should(Succeed())
		})
	})
})

func createTestOpsRequest(clusterName, opsRequestName string, opsType OpsType) *OpsRequest {
	randomStr, _ := password.Generate(6, 0, 0, true, false)
	return &OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opsRequestName + randomStr,
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": clusterName,
				"ops.kubeblocks.io/ops-type": string(opsType),
			},
		},
		Spec: OpsRequestSpec{
			ClusterRef: clusterName,
			Type:       opsType,
		},
	}
}

func createTestConfigmap(cmName string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: "default",
		},
		Data: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}
}
