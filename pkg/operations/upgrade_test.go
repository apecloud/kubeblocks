/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package operations

import (
	"context"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	k8sappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Upgrade OpsRequest", func() {

	var (
		randomStr   = testCtx.GetRandomStr()
		clusterName = "cluster-for-ops-" + randomStr
		serviceVer0 = testapps.ServiceVersion("v0")
		serviceVer1 = testapps.ServiceVersion("v1")
		serviceVer2 = testapps.ServiceVersion("v2")
		release0    = testapps.ReleaseID("r0")
		release1    = testapps.ReleaseID("r1")
		release2    = testapps.ReleaseID("r2")
		release3    = testapps.ReleaseID("r3")
		release4    = testapps.ReleaseID("r4")
	)
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), cluster definition
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	makeUpgradeOpsIsRunning := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource) {
		By("mock upgrade OpsRequest phase is Running")
		_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))
		// do upgrade
		_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
		Expect(err).ShouldNot(HaveOccurred())
		mockComponentIsOperating(opsRes.Cluster, appsv1.UpdatingComponentPhase, defaultCompName)
		Expect(testapps.ChangeObjStatus(&testCtx, opsRes.OpsRequest, func() {
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase
		})).Should(Succeed())
	}

	mockClusterRunning := func(clusterObject *appsv1.Cluster) {
		Expect(testapps.ChangeObjStatus(&testCtx, clusterObject, func() {
			clusterObject.Status.Phase = appsv1.RunningClusterPhase
			clusterObject.Status.Components = map[string]appsv1.ClusterComponentStatus{
				defaultCompName: {
					Phase: appsv1.RunningComponentPhase,
				},
			}
		})).Should(Succeed())
	}

	initOpsResWithComponentDef := func(createCompVersion bool) (*appsv1.ComponentDefinition, *appsv1.ComponentDefinition, *OpsResource) {
		compDef1 := testapps.NewComponentDefinitionFactory(testapps.CompDefName("cmpd-1")).
			SetServiceVersion(testapps.ServiceVersion(serviceVer0)).
			SetRuntime(&corev1.Container{
				Name: testapps.DefaultMySQLContainerName, Image: testapps.AppImage(testapps.AppName, release0),
			}).Create(&testCtx).GetObject()

		compDef2 := testapps.NewComponentDefinitionFactory(testapps.CompDefName("cmpd-2")).
			SetServiceVersion(testapps.ServiceVersion(serviceVer2)).SetRuntime(&corev1.Container{
			Name: testapps.DefaultMySQLContainerName, Image: testapps.AppImage(testapps.AppName, release2),
		}).Create(&testCtx).GetObject()
		if createCompVersion {
			compVersion := testapps.NewComponentVersionFactory(testapps.CompVersionName).
				SetSpec(appsv1.ComponentVersionSpec{
					CompatibilityRules: []appsv1.ComponentVersionCompatibilityRule{
						{
							// use prefix
							CompDefs: []string{compDef1.Name},
							Releases: []string{release0, release1, release4}, // sv: v0, v1
						},
						{
							// use prefix
							CompDefs: []string{compDef2.Name},
							Releases: []string{release2, release3}, // sv: v2
						},
					},
					Releases: []appsv1.ComponentVersionRelease{
						{
							Name:           release0,
							Changes:        "init release",
							ServiceVersion: serviceVer0,
							Images: map[string]string{
								testapps.DefaultMySQLContainerName: testapps.AppImage(testapps.AppName, release0),
							},
						},
						{
							Name:           release1,
							Changes:        "update app image",
							ServiceVersion: serviceVer1,
							Images: map[string]string{
								testapps.DefaultMySQLContainerName: testapps.AppImage(testapps.AppName, release1),
							},
						},
						{
							Name:           release2,
							Changes:        "update app image",
							ServiceVersion: serviceVer2,
							Images: map[string]string{
								testapps.DefaultMySQLContainerName: testapps.AppImage(testapps.AppName, serviceVer2),
							},
						},
						{
							Name:           release3,
							Changes:        "publish a new service version",
							ServiceVersion: serviceVer2,
							Images: map[string]string{
								testapps.DefaultMySQLContainerName: testapps.AppImage(testapps.AppName, release3),
							},
						},
						{
							Name:           release4,
							Changes:        "update all app images for previous service version",
							ServiceVersion: serviceVer1,
							Images: map[string]string{
								testapps.DefaultMySQLContainerName: testapps.AppImage(testapps.AppName, release4),
							},
						},
					},
				}).
				Create(&testCtx).
				GetObject()
			// label the componentDef info to the ComponentVersion
			Expect(testapps.ChangeObj(&testCtx, compVersion, func(version *appsv1.ComponentVersion) {
				if version.Labels == nil {
					version.Labels = map[string]string{}
				}
				version.Labels[compDef1.Name] = compDef1.Name
				version.Labels[compDef2.Name] = compDef2.Name
			})).Should(Succeed())

			// mock ComponentVersion to Available
			Expect(testapps.ChangeObjStatus(&testCtx, compVersion, func() {
				compVersion.Status.Phase = appsv1.AvailablePhase
				compVersion.Status.ObservedGeneration = compVersion.Generation
			})).Should(Succeed())
		}
		// create the cluster with no clusterDefinition
		clusterObject := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			AddComponent(defaultCompName, compDef1.Name).
			SetServiceVersion(testapps.ServiceVersion("v0")).
			SetReplicas(int32(3)).Create(&testCtx).GetObject()
		opsRes := &OpsResource{
			Cluster:  clusterObject,
			Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
		}
		// mock component phase to running
		mockClusterRunning(clusterObject)
		return compDef1, compDef2, opsRes
	}

	createUpgradeOpsRequest := func(clusterObject *appsv1.Cluster, upgradeSpec opsv1alpha1.Upgrade) *opsv1alpha1.OpsRequest {
		ops := testops.NewOpsRequestObj("upgrade-ops-"+randomStr, testCtx.DefaultNamespace,
			clusterObject.Name, opsv1alpha1.UpgradeType)
		ops.Spec.Upgrade = &upgradeSpec
		opsRequest := testops.CreateOpsRequest(ctx, testCtx, ops)
		// set ops phase to Pending
		opsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
		return opsRequest
	}

	expectOpsSucceed := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, compNames ...string) {
		// mock component to running
		mockComponentIsOperating(opsRes.Cluster, appsv1.RunningComponentPhase, compNames...)
		_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsSucceedPhase))
	}

	mockPodsAppliedImage := func(cluster *appsv1.Cluster, releaseVersion string) {
		pods := testapps.MockInstanceSetPods(&testCtx, nil, cluster, defaultCompName)
		image := testapps.AppImage(testapps.AppName, releaseVersion)
		for i := range pods {
			pod := pods[i]
			Expect(testapps.ChangeObj(&testCtx, pod, func(pod *corev1.Pod) {
				pod.Spec.Containers[0].Image = image
			})).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{
					{
						Name: testapps.DefaultMySQLContainerName,
						// the latest release version will be selected.
						Image: image,
					},
				}
			})).Should(Succeed())
		}
	}

	Context("Test OpsRequest", func() {
		DescribeTable("updates only selected instance template version fields", func(sharding, parent bool, compDef, version *string) {
			compDef1, compDef2, opsRes := initOpsResWithComponentDef(true)
			if compDef != nil && *compDef != "" {
				compDef = &compDef2.Name
			}
			targetName := defaultCompName
			if sharding {
				targetName = "sharding"
			}
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(cluster *appsv1.Cluster) {
				cluster.Spec.ComponentSpecs[0].Instances = []appsv1.InstanceTemplate{
					{Name: "canary", CompDef: compDef1.Name, ServiceVersion: "1.0.0", Replicas: pointer.Int32(1),
						Ordinals: appsv1.Ordinals{Discrete: []int32{7}}, Labels: map[string]string{"role": "canary"},
						Annotations:      map[string]string{"annotation": "preserved"},
						SchedulingPolicy: &appsv1.SchedulingPolicy{NodeSelector: map[string]string{"zone": "a"}},
						Resources:        &corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}},
						Env:              []corev1.EnvVar{{Name: "PRESERVED", Value: "yes"}},
						VolumeClaimTemplates: []appsv1.ClusterComponentVolumeClaimTemplate{{Name: "data", Spec: appsv1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
						}}}},
					{Name: "stable", Replicas: pointer.Int32(1)},
					{Name: "untouched", CompDef: compDef1.Name, ServiceVersion: "1.0.0", Replicas: pointer.Int32(1)},
				}
				if sharding {
					cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: targetName, Shards: 2, Template: *cluster.Spec.ComponentSpecs[0].DeepCopy()}}
				}
			})).Should(Succeed())
			before := opsRes.Cluster.DeepCopy()
			expected := getComponentSpecOrShardingTemplate(before, targetName).DeepCopy()
			upgrade := opsv1alpha1.UpgradeComponent{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: targetName},
				Instances: []opsv1alpha1.UpgradeInstanceTemplate{
					{Name: "canary", ComponentDefinitionName: compDef, ServiceVersion: version},
					{Name: "stable", ServiceVersion: pointer.String("2.0.0")},
				}}
			if compDef != nil {
				expected.Instances[0].CompDef = *compDef
			}
			if version != nil {
				expected.Instances[0].ServiceVersion = *version
			}
			expected.Instances[1].ServiceVersion = "2.0.0"
			if parent {
				upgrade.ComponentDefinitionName, upgrade.ServiceVersion = &compDef2.Name, pointer.String(serviceVer2)
				expected.ComponentDef, expected.ServiceVersion = compDef2.Name, serviceVer2
			}
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{upgrade}})
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			persisted := &appsv1.Cluster{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.Cluster), persisted)).Should(Succeed())
			Expect(getComponentSpecOrShardingTemplate(persisted, targetName)).Should(Equal(expected))
			if sharding {
				Expect(persisted.Spec.ComponentSpecs).Should(Equal(before.Spec.ComponentSpecs))
				Expect(persisted.Spec.Shardings[0].Shards).Should(Equal(before.Spec.Shardings[0].Shards))
			}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsCreatingPhase))
			Expect(opsRes.OpsRequest.Status.LastConfiguration.Components[targetName].Instances).
				Should(Equal(getComponentSpecOrShardingTemplate(before, targetName).Instances))
			By("repeating the registered action from persisted state")
			opsRes.Cluster = persisted
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(persisted), persisted)).Should(Succeed())
			Expect(getComponentSpecOrShardingTemplate(persisted, targetName)).Should(Equal(expected))
			Expect(opsRes.OpsRequest.Status.LastConfiguration.Components[targetName].Instances).
				Should(Equal(getComponentSpecOrShardingTemplate(before, targetName).Instances))
		},
			Entry("component definition only", false, false, pointer.String("new-template-def"), nil),
			Entry("component version only", false, false, nil, pointer.String("2.0.0")),
			Entry("component both fields with parent upgrade", false, true, pointer.String("new-template-def"), pointer.String("2.0.0")),
			Entry("component clears definition without parent exception", false, false, pointer.String(""), nil),
			Entry("component clears version", false, false, nil, pointer.String("")),
			Entry("component resets both fields", false, false, pointer.String(""), pointer.String("")),
			Entry("sharding definition only", true, false, pointer.String("new-template-def"), nil),
			Entry("sharding version only", true, false, nil, pointer.String("2.0.0")),
			Entry("sharding both fields with parent upgrade", true, true, pointer.String("new-template-def"), pointer.String("2.0.0")),
			Entry("sharding resets both fields", true, false, pointer.String(""), pointer.String("")),
		)

		DescribeTable("revalidates instance template targets before applying a queued upgrade", func(sharding bool) {
			_, compDef2, opsRes := initOpsResWithComponentDef(true)
			targetName := defaultCompName
			if sharding {
				targetName = "sharding"
			}
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(cluster *appsv1.Cluster) {
				cluster.Spec.ComponentSpecs[0].Instances = []appsv1.InstanceTemplate{{Name: "canary", Replicas: pointer.Int32(1)}}
				if sharding {
					cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: targetName, Shards: 1, Template: *cluster.Spec.ComponentSpecs[0].DeepCopy()}}
				}
			})).Should(Succeed())
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: targetName}, ComponentDefinitionName: &compDef2.Name,
				Instances: []opsv1alpha1.UpgradeInstanceTemplate{{Name: "canary", ServiceVersion: pointer.String("2.0.0")}},
			}}})
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsCreatingPhase))
			By("removing the target before the next pre-action turn")
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(cluster *appsv1.Cluster) {
				if sharding {
					cluster.Spec.Shardings[0].Template.Instances = nil
				} else {
					cluster.Spec.ComponentSpecs[0].Instances = nil
				}
			})).Should(Succeed())
			before := getComponentSpecOrShardingTemplate(opsRes.Cluster, targetName).DeepCopy()
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.Cluster), opsRes.Cluster)).Should(Succeed())
			Expect(getComponentSpecOrShardingTemplate(opsRes.Cluster, targetName)).Should(Equal(before))
		}, Entry("component", false), Entry("sharding", true))

		DescribeTable("completes an upgrade with an instance template service version", func(templateVersion, templateRelease string, sharding bool, targets []opsv1alpha1.UpgradeInstanceTemplate) {
			compDef1, compDef2, opsRes := initOpsResWithComponentDef(true)
			upgrade := opsv1alpha1.UpgradeComponent{
				ComponentDefinitionName: &compDef2.Name, ServiceVersion: pointer.String(serviceVer2), Instances: targets,
			}
			ownerCompDef, ownerVersion, baseRelease := compDef2.Name, serviceVer2, release3
			if len(targets) > 0 {
				upgrade.ComponentDefinitionName, upgrade.ServiceVersion = nil, nil
				ownerCompDef, ownerVersion, baseRelease = compDef1.Name, serviceVer0, release0
				for i := range upgrade.Instances {
					if definition := upgrade.Instances[i].ComponentDefinitionName; definition != nil && *definition != "" {
						upgrade.Instances[i].ComponentDefinitionName = &compDef2.Name
					}
				}
			}
			DeferCleanup(func() {
				selector := client.MatchingLabels{constant.AppInstanceLabelKey: clusterName}
				testapps.ClearResources(&testCtx, generics.InstanceSetSignature, client.InNamespace(testCtx.DefaultNamespace), selector)
				testapps.ClearResources(&testCtx, generics.PodSignature, client.InNamespace(testCtx.DefaultNamespace), selector)
				testapps.ClearResources(&testCtx, generics.ComponentSignature, client.InNamespace(testCtx.DefaultNamespace), selector)
			})
			targetName, componentName := defaultCompName, defaultCompName
			if sharding {
				targetName, componentName = "sharding", "sharding-shard-0"
			}
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(cluster *appsv1.Cluster) {
				cluster.Spec.ComponentSpecs[0].Instances = []appsv1.InstanceTemplate{{
					Name: "pinned", CompDef: compDef1.Name, ServiceVersion: templateVersion,
					Replicas: pointer.Int32(1),
				}}
				if sharding {
					cluster.Spec.Shardings = []appsv1.ClusterSharding{{
						Name: targetName, Shards: 1, Template: *cluster.Spec.ComponentSpecs[0].DeepCopy(),
					}}
				}
			})).Should(Succeed())
			upgrade.ComponentOps = opsv1alpha1.ComponentOps{ComponentName: targetName}
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{upgrade}})
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			mockComponentIsOperating(opsRes.Cluster, appsv1.RunningComponentPhase, defaultCompName)
			if sharding {
				testapps.NewComponentFactory(testCtx.DefaultNamespace, clusterName+"-"+componentName, ownerCompDef).
					AddLabels(constant.AppInstanceLabelKey, clusterName).
					AddLabels(constant.KBAppClusterUIDKey, string(opsRes.Cluster.UID)).
					AddLabels(constant.KBAppShardingNameLabelKey, targetName).
					AddLabels(constant.KBAppComponentLabelKey, componentName).
					SetReplicas(3).Create(&testCtx)
				Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
					opsRes.Cluster.Status.Shardings = map[string]appsv1.ClusterComponentStatus{
						targetName: {Phase: appsv1.RunningComponentPhase},
					}
				})).Should(Succeed())
			}
			comp := &appsv1.Component{}
			compKey := client.ObjectKey{Namespace: testCtx.DefaultNamespace, Name: clusterName + "-" + componentName}
			if err := k8sClient.Get(ctx, compKey, comp); err != nil {
				comp = testapps.NewComponentFactory(testCtx.DefaultNamespace, compKey.Name, ownerCompDef).
					AddLabels(constant.AppInstanceLabelKey, clusterName).
					AddLabels(constant.KBAppComponentLabelKey, componentName).SetReplicas(3).Create(&testCtx).Get()
			}
			ownerImage := testapps.AppImage(testapps.AppName, templateRelease) + "-owner"
			assertPending := func() {
				phase, requeue, err := (upgradeOpsHandler{}).ReconcileAction(reqCtx, k8sClient, opsRes)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
				Expect(requeue).Should(BeNumerically(">", 0))
			}
			if len(upgrade.Instances) == 0 {
				for _, name := range []string{
					clusterName + "-" + componentName + "-0",
					clusterName + "-" + componentName + "-1",
					clusterName + "-" + componentName + "-pinned-0",
				} {
					image := testapps.AppImage(testapps.AppName, baseRelease)
					if name == clusterName+"-"+componentName+"-pinned-0" {
						image = testapps.AppImage(testapps.AppName, release0)
					}
					pod := testapps.MockInstanceSetPod(&testCtx, nil, clusterName, componentName, name, "")
					Expect(testapps.ChangeObj(&testCtx, pod, func(pod *corev1.Pod) {
						pod.Spec.Containers[0].Image = image
					})).Should(Succeed())
					Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
						pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: testapps.DefaultMySQLContainerName, Image: image}}
					})).Should(Succeed())
				}
			}
			By("waiting for the Component to consume the Cluster generation")
			Expect(testapps.ChangeObj(&testCtx, comp, func(comp *appsv1.Component) {
				comp.Annotations = map[string]string{constant.KubeBlocksGenerationKey: "invalid"}
			})).Should(Succeed())
			assertPending()
			Expect(testapps.ChangeObj(&testCtx, comp, func(comp *appsv1.Component) {
				comp.Annotations[constant.KubeBlocksGenerationKey] = strconv.FormatInt(opsRes.Cluster.Generation, 10)
				comp.Spec.ServiceVersion = ownerVersion
			})).Should(Succeed())
			By("waiting for the owned workload to exist")
			assertPending()
			its := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace, comp.Name, clusterName, componentName).
				SetReplicas(3).AddContainer(corev1.Container{Name: testapps.DefaultMySQLContainerName,
				Image: testapps.AppImage(testapps.AppName, baseRelease)}).Create(&testCtx).Get()
			By("waiting for the workload to consume the Component generation")
			Expect(testapps.ChangeObj(&testCtx, its, func(its *workloads.InstanceSet) {
				its.Annotations = map[string]string{constant.KubeBlocksGenerationKey: strconv.FormatInt(comp.Generation-1, 10)}
				if len(upgrade.Instances) > 0 {
					its.Spec.PodManagementPolicy = k8sappsv1.ParallelPodManagement
				}
				its.Spec.Instances = []workloads.InstanceTemplate{{Name: "pinned", Replicas: pointer.Int32(1),
					Images: map[string]string{testapps.DefaultMySQLContainerName: testapps.AppImage(testapps.AppName, release0)}}}
			})).Should(Succeed())
			assertPending()
			By("waiting when stop publishes the generation without the upgrade version")
			Expect(testapps.ChangeObj(&testCtx, its, func(its *workloads.InstanceSet) {
				its.Annotations[constant.KubeBlocksGenerationKey] = strconv.FormatInt(comp.Generation, 10)
				its.Annotations[constant.AppComponentLabelKey] = comp.Spec.CompDef
				its.Annotations[constant.KBAppServiceVersionKey] = "not-the-owner-version"
			})).Should(Succeed())
			assertPending()
			Expect(testapps.ChangeObj(&testCtx, its, func(its *workloads.InstanceSet) {
				its.Annotations[constant.KubeBlocksGenerationKey] = strconv.FormatInt(comp.Generation, 10)
				its.Annotations[constant.KBAppServiceVersionKey] = comp.Spec.ServiceVersion
				its.Spec.Instances[0].Images[testapps.DefaultMySQLContainerName] = ownerImage
			})).Should(Succeed())
			if len(upgrade.Instances) > 0 {
				tree := kubebuilderx.NewObjectTree()
				tree.SetRoot(its)
				for _, reconciler := range []kubebuilderx.Reconciler{instanceset.NewRevisionUpdateReconciler(), instanceset.NewReplicasAlignmentReconciler()} {
					_, err := reconciler.Reconcile(tree)
					Expect(err).ShouldNot(HaveOccurred())
				}
				Expect(k8sClient.Status().Update(ctx, its)).Should(Succeed())
				pods := tree.List(&corev1.Pod{})
				Expect(pods).Should(HaveLen(3))
				for _, object := range pods {
					pod := object.(*corev1.Pod)

					if pod.Name == clusterName+"-"+componentName+"-pinned-0" {
						pod.Spec.Containers[0].Image = testapps.AppImage(testapps.AppName, release0)
					}
					Expect(k8sClient.Create(ctx, pod)).Should(Succeed())
					Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
						pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
						pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: pod.Spec.Containers[0].Name, Image: pod.Spec.Containers[0].Image}}
					})).Should(Succeed())
				}
			}

			By("waiting while the template Pod still has the old spec and status images")
			phase, _, err := (upgradeOpsHandler{}).ReconcileAction(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/3"))
			pinnedPod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: testCtx.DefaultNamespace,
				Name: clusterName + "-" + componentName + "-pinned-0"}, pinnedPod)).Should(Succeed())
			expectedImage := ownerImage
			Expect(testapps.ChangeObj(&testCtx, pinnedPod, func(pod *corev1.Pod) {
				pod.Spec.Containers[0].Image = expectedImage
			})).Should(Succeed())
			By("waiting for the observed container image even after the Pod spec is updated")
			phase, _, err = (upgradeOpsHandler{}).ReconcileAction(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/3"))
			Expect(testapps.ChangeObjStatus(&testCtx, pinnedPod, func() {
				pinnedPod.Status.ContainerStatuses[0].Image = expectedImage
			})).Should(Succeed())
			By("completing once all Pods have their own expected images")
			phase, _, err = (upgradeOpsHandler{}).ReconcileAction(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("3/3"))
		},
			Entry("keeps the explicitly pinned version", serviceVer1, release4, false, nil),
			Entry("uses published owner images even when version parsing produces no overrides", serviceVer0, release3, false, nil),
			Entry("uses the actual shard component name for template ownership", serviceVer1, release4, true, nil),
			Entry("waits for owner images after a definition-only template change", "", release4, false,
				[]opsv1alpha1.UpgradeInstanceTemplate{{Name: "pinned", ComponentDefinitionName: pointer.String("resolved-template-def")}}),
			Entry("waits for owner images after resetting both template fields", serviceVer1, release3, false,
				[]opsv1alpha1.UpgradeInstanceTemplate{{Name: "pinned", ComponentDefinitionName: pointer.String(""), ServiceVersion: pointer.String("")}}),
			Entry("waits for owner images after a shard template definition-only change", "", release4, true,
				[]opsv1alpha1.UpgradeInstanceTemplate{{Name: "pinned", ComponentDefinitionName: pointer.String("resolved-template-def")}}),
			Entry("waits for owner images after resetting both shard template fields", serviceVer1, release3, true,
				[]opsv1alpha1.UpgradeInstanceTemplate{{Name: "pinned", ComponentDefinitionName: pointer.String(""), ServiceVersion: pointer.String("")}}),
			Entry("waits for owner images after a version-only template change", "", release4, false,
				[]opsv1alpha1.UpgradeInstanceTemplate{{Name: "pinned", ServiceVersion: pointer.String("2.0.0")}}),
			Entry("waits for owner images after changing both shard template fields", serviceVer1, release4, true,
				[]opsv1alpha1.UpgradeInstanceTemplate{{Name: "pinned", ComponentDefinitionName: pointer.String("resolved-template-def"), ServiceVersion: pointer.String("2.0.0")}}),
		)

		It("Test upgrade OpsRequest with ComponentDef and no ComponentVersion", func() {
			By("init operations resources ")
			compDef1, compDef2, opsRes := initOpsResWithComponentDef(false)

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{
				Components: []opsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ComponentDefinitionName: &compDef2.Name,
					},
				},
			})

			By("mock upgrade OpsRequest phase is Running")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, ops *opsv1alpha1.OpsRequest) {
				g.Expect(ops.Status.LastConfiguration.Components[defaultCompName].ComponentDefinitionName).Should(Equal(compDef1.Name))
			})).Should(Succeed())

			By("the ops is expected to be Running when the component phase is in a terminal state but progress is not completed")
			mockComponentIsOperating(opsRes.Cluster, appsv1.RunningComponentPhase, defaultCompName)
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsRunningPhase))

			By("expect upgrade successfully with the image that is provided in the specified componentDefinition")
			mockPodsAppliedImage(opsRes.Cluster, release2)
			expectOpsSucceed(reqCtx, opsRes, defaultCompName)
		})

		It("Test upgrade OpsRequest with ComponentDef and ComponentVersion", func() {
			By("init operations resources")
			_, compDef2, opsRes := initOpsResWithComponentDef(true)

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{
				Components: []opsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ServiceVersion:          pointer.String(serviceVer2),
						ComponentDefinitionName: &compDef2.Name,
					},
				},
			})

			By("expect for this opsRequest is Running")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDef2.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(serviceVer2))
			})).Should(Succeed())

			By("expect upgrade successfully")
			mockPodsAppliedImage(opsRes.Cluster, release3)
			expectOpsSucceed(reqCtx, opsRes, defaultCompName)
		})

		It("Test upgrade OpsRequest without ComponentDefinitionName but the cluster is created without clusterDefinition", func() {
			By("init operations resources")
			compDef1, _, opsRes := initOpsResWithComponentDef(true)

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{
				Components: []opsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ComponentDefinitionName: pointer.String(""),
						ServiceVersion:          pointer.String(serviceVer1),
					},
				},
			})

			By("expect for this opsRequest is Running and reuse the original ComponentDefinition")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDef1.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(serviceVer1))
			})).Should(Succeed())

			By("expect upgrade successfully with the latest release of the specified serviceVersion")
			mockPodsAppliedImage(opsRes.Cluster, release4)
			expectOpsSucceed(reqCtx, opsRes, defaultCompName)
		})

		It("Test upgrade OpsRequest when specified serviceVersion is empty", func() {
			By("init operations resources")
			compDef1, _, opsRes := initOpsResWithComponentDef(true)

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{
				Components: []opsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ComponentDefinitionName: pointer.String(compDef1.Name),
						ServiceVersion:          pointer.String(""),
					},
				},
			})

			By(" expect for this opsRequest is Running")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDef1.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(BeEmpty())
			})).Should(Succeed())

			By("looking forward to using the latest serviceVersion and releaseVersion")
			mockPodsAppliedImage(opsRes.Cluster, release4)
			expectOpsSucceed(reqCtx, opsRes, defaultCompName)
		})

		It("Test upgrade OpsRequest when serviceVersion is nil", func() {
			By("init operations resources")
			_, compDef2, opsRes := initOpsResWithComponentDef(true)

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{
				Components: []opsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ComponentDefinitionName: pointer.String(compDef2.Name),
					},
				},
			})

			By("expecting no changes to serviceVersion")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDef2.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(serviceVer0))
			})).Should(Succeed())
		})

		It("Test upgrade OpsRequest when componentDefinitionName is nil", func() {
			By("init operations resources")
			compDef1, _, opsRes := initOpsResWithComponentDef(true)

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{
				Components: []opsv1alpha1.UpgradeComponent{
					{
						ComponentOps:   opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ServiceVersion: pointer.String(""),
					},
				},
			})

			By("expecting no changes to cluster.spec.componentSpec[0].componentDef")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDef1.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(""))
			})).Should(Succeed())
		})
		// TODO: add case with ClusterDefinition and topology
	})
})

func TestUpgradeTemplateInitProgress(t *testing.T) {
	for _, tc := range []struct {
		name                                                string
		clear, sharding, sidecar, legacy, rewrite, deferred bool
	}{
		{name: "init-only version"},
		{name: "clear both fields", clear: true},
		{name: "shard clear both fields", clear: true, sharding: true},
		{name: "restartable init", sidecar: true},
		{name: "admission registry rewrite", rewrite: true},
		{name: "legacy direct template", legacy: true},
		{name: "deferred tools init", deferred: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
				g.Expect(add(scheme)).To(Succeed())
			}
			def := &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "db"}, Spec: appsv1.ComponentDefinitionSpec{
				ServiceVersion: "1.0.0", Runtime: corev1.PodSpec{
					Containers:     []corev1.Container{{Name: "db", Image: "db:1"}},
					InitContainers: []corev1.Container{{Name: "setup", Image: "setup:1"}},
				},
			}}
			if tc.sidecar {
				def.Spec.Runtime.InitContainers[0].RestartPolicy = ptr.To(corev1.ContainerRestartPolicyAlways)
			}
			version := &appsv1.ComponentVersion{ObjectMeta: metav1.ObjectMeta{Name: "db-version", Generation: 1, Labels: map[string]string{"db": "db"}},
				Spec: appsv1.ComponentVersionSpec{
					CompatibilityRules: []appsv1.ComponentVersionCompatibilityRule{{CompDefs: []string{"db"}, Releases: []string{"v2"}}},
					Releases:           []appsv1.ComponentVersionRelease{{Name: "v2", ServiceVersion: "2.0.0", Images: map[string]string{"db": "db:1", "setup": "setup:2"}}},
				}, Status: appsv1.ComponentVersionStatus{ObservedGeneration: 1, Phase: appsv1.AvailablePhase},
			}
			componentName, targetName := "db", "db"
			if tc.sharding {
				componentName, targetName = "sharding-shard-0", "sharding"
			}
			cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "cluster", Generation: 2},
				Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: componentName, ComponentDef: "db", ServiceVersion: "1.0.0", Replicas: 1,
					Instances: []appsv1.InstanceTemplate{{Name: "canary", Replicas: pointer.Int32(1), CompDef: "db", ServiceVersion: "1.0.0"}},
				}}}, Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{componentName: {Phase: appsv1.RunningComponentPhase}}},
			}
			if tc.clear || tc.legacy {
				cluster.Spec.ComponentSpecs[0].Instances[0].ServiceVersion = "2.0.0"
			}
			if tc.sharding {
				cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: targetName, Shards: 1, Template: cluster.Spec.ComponentSpecs[0]}}
				cluster.Spec.ComponentSpecs = nil
				cluster.Status.Shardings = map[string]appsv1.ClusterComponentStatus{targetName: {Phase: appsv1.RunningComponentPhase}}
			}
			target := opsv1alpha1.UpgradeComponent{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: targetName},
				Instances: []opsv1alpha1.UpgradeInstanceTemplate{{Name: "canary", ServiceVersion: pointer.String("2.0.0")}}}
			if tc.clear {
				target.Instances[0].ComponentDefinitionName, target.Instances[0].ServiceVersion = pointer.String(""), pointer.String("")
			}
			if tc.legacy {
				target.Instances = nil
			}
			ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "upgrade", Namespace: "default"},
				Spec:   opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.UpgradeType, SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{Upgrade: &opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{target}}}},
				Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase, StartTimestamp: metav1.Now()},
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops).WithObjects(cluster, ops, def, version).Build()
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			handler := GetOpsManager().OpsMap[opsv1alpha1.UpgradeType].OpsHandler
			res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(20)}
			g.Expect(handler.Action(reqCtx, cli, res)).To(Succeed())
			compSpec := getComponentSpecOrShardingTemplate(cluster, targetName)
			if tc.clear {
				g.Expect(compSpec.Instances[0].CompDef).To(BeEmpty())
				g.Expect(compSpec.Instances[0].ServiceVersion).To(BeEmpty())
			}
			publishedSpec := compSpec.DeepCopy()
			publishedSpec.Name = componentName
			comp, err := component.BuildComponent(cluster, publishedSpec, nil, nil)
			g.Expect(err).NotTo(HaveOccurred())
			comp.Generation = 2
			if tc.sharding {
				comp.Labels[constant.KBAppShardingNameLabelKey] = targetName
				comp.Labels[constant.KBAppClusterUIDKey] = string(cluster.UID)
			}
			g.Expect(cli.Create(ctx, comp)).To(Succeed())
			images := map[string]map[string]string{}
			if !tc.clear {
				images["canary"], err = component.ResolveInstanceTemplateImages4ServiceVersion(ctx, cli, def, "2.0.0", nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(images["canary"]["setup"]).To(HaveSuffix("setup:2"))
			}
			synth := &component.SynthesizedComponent{Namespace: cluster.Namespace, ClusterName: cluster.Name, Name: componentName,
				CompDefName: "db", ServiceVersion: "1.0.0", Generation: "2", Replicas: 1,
				PodSpec: def.Spec.Runtime.DeepCopy(), Instances: compSpec.Instances, InstanceImages: images}
			if tc.deferred {
				oldTools := viper.GetString(constant.KBToolsImage)
				t.Cleanup(func() { viper.Set(constant.KBToolsImage, oldTools) })
				viper.Set(constant.KBToolsImage, "mirror.local/apecloud/kubeblocks-tools:1.1.0")
				synth.PodSpec.InitContainers = []corev1.Container{{Name: kbagent.InitContainerName,
					Image: "mirror.local/apecloud/kubeblocks-tools:1.1.0", Command: kbagent.InitCommand()}}
				synth.PodSpec.Containers = append(synth.PodSpec.Containers, corev1.Container{
					Name: "kbagent", Image: "custom-action:1.0", Command: []string{kbagent.SharedBinaryPath}})
			}
			its, err := factory.BuildInstanceSet(synth, def)
			g.Expect(err).NotTo(HaveOccurred())
			its.Generation = 2
			buildPod := func(owner *workloads.InstanceSet) *corev1.Pod {
				tree := kubebuilderx.NewObjectTree()
				tree.SetRoot(owner)
				for _, reconciler := range []kubebuilderx.Reconciler{instanceset.NewFixMetaReconciler(), instanceset.NewRevisionUpdateReconciler(), instanceset.NewAssistantObjectReconciler(), instanceset.NewReplicasAlignmentReconciler()} {
					_, err := reconciler.Reconcile(tree)
					g.Expect(err).NotTo(HaveOccurred())
				}
				pods := tree.List(&corev1.Pod{})
				g.Expect(pods).To(HaveLen(1))
				return pods[0].(*corev1.Pod).DeepCopy()
			}
			desiredPod := buildPod(its)
			pod := desiredPod.DeepCopy()
			oldInit := "setup:1"
			if tc.clear {
				oldInit = "setup:2"
			}
			if tc.deferred {
				oldInit = "docker.io/apecloud/kubeblocks-tools:1.0.0"
				pod.Spec.InitContainers[0].Command = kbagent.LegacyInitCommand()
				pod.Spec.Containers[0].Image = "db:0"
			}
			pod.Spec.InitContainers[0].Image = oldInit
			pod.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Hour))
			pod.Status = corev1.PodStatus{Phase: corev1.PodRunning,
				Conditions:            []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: pod.CreationTimestamp}},
				ContainerStatuses:     []corev1.ContainerStatus{{Name: "db", Image: "db:1", Ready: true}},
				InitContainerStatuses: []corev1.ContainerStatus{{Name: "setup", Image: oldInit, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}},
			}
			if tc.deferred {
				pod.Status.ContainerStatuses[0].Image = "db:0"
			}
			g.Expect(cli.Create(ctx, its)).To(Succeed())
			g.Expect(cli.Create(ctx, pod)).To(Succeed())
			reconcile := func() (opsv1alpha1.OpsPhase, time.Duration, error) {
				freshOps, freshCluster := &opsv1alpha1.OpsRequest{}, &appsv1.Cluster{}
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(ops), freshOps)).To(Succeed())
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(cluster), freshCluster)).To(Succeed())
				res = &OpsResource{Cluster: freshCluster, OpsRequest: freshOps, Recorder: record.NewFakeRecorder(20)}
				return GetOpsManager().OpsMap[opsv1alpha1.UpgradeType].OpsHandler.ReconcileAction(reqCtx, cli, res)
			}
			for i := 0; i < 2; i++ {
				phase, requeue, err := reconcile()
				g.Expect(err).NotTo(HaveOccurred())
				if tc.legacy {
					g.Expect(phase).To(Equal(opsv1alpha1.OpsSucceedPhase))
					return
				}
				g.Expect(phase).To(Equal(opsv1alpha1.OpsRunningPhase))
				g.Expect(res.OpsRequest.Status.Progress).To(Equal("0/1"))
				g.Expect(requeue).To(BeNumerically(">", 0))
			}
			if !tc.deferred {
				pod.Spec.InitContainers = desiredPod.Spec.InitContainers
			}
			pod.Spec.Containers[0].Image = desiredPod.Spec.Containers[0].Image
			pod.Status.ContainerStatuses[0].Image = desiredPod.Spec.Containers[0].Image
			if tc.rewrite {
				pod.Spec.InitContainers[0].Image = "admission.local/rewritten/setup:2"
				pod.Spec.Containers[0].Image = "admission.local/rewritten/db:1"
			}
			if tc.sidecar {
				pod.Status.InitContainerStatuses[0].State = corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
			}
			status := pod.Status.DeepCopy()
			g.Expect(cli.Update(ctx, pod)).To(Succeed())
			pod.Status = *status
			g.Expect(cli.Status().Update(ctx, pod)).To(Succeed())
			updated, err := instanceset.IsPodUpdated(its, pod)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(updated).To(BeTrue())
			phase, _, err := reconcile()
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(phase).To(Equal(opsv1alpha1.OpsSucceedPhase))
			g.Expect(res.OpsRequest.Status.Progress).To(Equal("1/1"))
		})
	}
}
