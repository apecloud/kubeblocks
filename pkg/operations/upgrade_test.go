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
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
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
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
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

		It("Test upgrade OpsRequest with invalid service version should fail validation", func() {
			By("init operations resources")
			_, _, opsRes := initOpsResWithComponentDef(true)

			By("create Upgrade Ops with invalid service version")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, opsv1alpha1.Upgrade{
				Components: []opsv1alpha1.UpgradeComponent{
					{
						ComponentOps:   opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ServiceVersion: pointer.String("17.5"),
					},
				},
			})

			By("expect Do() fails with validation error and OpsRequest enters Failed phase")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsFailedPhase))
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

func TestUpgradeInstanceTemplateVersions(t *testing.T) {
	for _, tc := range []struct {
		name, version, templateImage   string
		flat, shard, wrongStatus       bool
		unlabeled, alternateDefinition bool
		ownerRecovery                  bool
		want                           opsv1alpha1.OpsPhase
		progress                       string
	}{
		{name: "shard publication recovery", version: "1.0.0", templateImage: "db:v1", shard: true, ownerRecovery: true, want: opsv1alpha1.OpsSucceedPhase, progress: "2/2"},
		{name: "owner publication recovery", version: "1.0.0", templateImage: "db:v1", ownerRecovery: true, want: opsv1alpha1.OpsSucceedPhase, progress: "2/2"},
		{name: "fixed template", version: "1.0.0", templateImage: "db:owner", want: opsv1alpha1.OpsSucceedPhase, progress: "2/2"},
		{name: "wrong template image", version: "1.0.0", templateImage: "db:wrong", want: opsv1alpha1.OpsRunningPhase, progress: "1/2"},
		{name: "old running image", version: "1.0.0", templateImage: "db:v1", wrongStatus: true, want: opsv1alpha1.OpsRunningPhase, progress: "1/2"},
		{name: "empty differential inherits default", version: "0.0.0", templateImage: "db:v2", want: opsv1alpha1.OpsSucceedPhase, progress: "2/2"},
		{name: "flat fixed template", version: "1.0.0", templateImage: "db:v1", flat: true, want: opsv1alpha1.OpsSucceedPhase, progress: "2/2"},
		{name: "flat template without ownership label", version: "1.0.0", templateImage: "db:v1", flat: true, unlabeled: true, want: opsv1alpha1.OpsRunningPhase, progress: "1/2"},
		{name: "shard instance template", version: "1.0.0", templateImage: "db:v1", shard: true, want: opsv1alpha1.OpsSucceedPhase, progress: "2/2"},
		{name: "shard wrong image", version: "1.0.0", templateImage: "db:wrong", shard: true, want: opsv1alpha1.OpsRunningPhase, progress: "1/2"},
		{name: "shard definition determines readiness", version: "1.0.0", templateImage: "db:v1", shard: true, alternateDefinition: true, want: opsv1alpha1.OpsSucceedPhase, progress: "2/2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{
				corev1.AddToScheme, appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme,
			} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			def := &appsv1.ComponentDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: "db"},
				Spec: appsv1.ComponentDefinitionSpec{
					Runtime: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:base"}}},
				},
			}
			ver := &appsv1.ComponentVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "db", Labels: map[string]string{"db": "db"}},
				Spec: appsv1.ComponentVersionSpec{
					CompatibilityRules: []appsv1.ComponentVersionCompatibilityRule{
						{CompDefs: []string{"db"}, Releases: []string{"r0", "r1", "r2"}},
					},
					Releases: []appsv1.ComponentVersionRelease{
						{Name: "r0", ServiceVersion: "0.0.0", Images: map[string]string{"db": "db:base"}},
						{Name: "r1", ServiceVersion: "1.0.0", Images: map[string]string{"db": "db:v1"}},
						{Name: "r2", ServiceVersion: "2.0.0", Images: map[string]string{"db": "db:v2"}},
					},
				},
				Status: appsv1.ComponentVersionStatus{Phase: appsv1.AvailablePhase},
			}
			spec := appsv1.ClusterComponentSpec{
				Name: "db", ComponentDef: "db", ServiceVersion: "2.0.0", Replicas: 2,
				FlatInstanceOrdinal: tc.flat,
				Instances:           []appsv1.InstanceTemplate{{Name: "fixed", CompDef: "db", ServiceVersion: tc.version}},
			}
			cluster := &appsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "ns", Generation: 3},
				Spec:       appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{spec}},
				Status: appsv1.ClusterStatus{
					Components: map[string]appsv1.ClusterComponentStatus{"db": {Phase: appsv1.RunningComponentPhase}},
				},
			}
			target, componentName := "db", "db"
			var shardLabels map[string]string
			if tc.shard {
				target, componentName = "shards", "shards-abc"
				defaultSpec := spec
				defaultSpec.Name = "shards"
				defaultSpec.Replicas = 3
				defaultSpec.Instances = nil
				cluster.Spec.ComponentSpecs = nil
				cluster.Spec.Shardings = []appsv1.ClusterSharding{{
					Name: target, Shards: 1, Template: defaultSpec,
					ShardTemplates: []appsv1.ShardTemplate{{
						Name: "special", Shards: ptr.To(int32(1)), Replicas: ptr.To(int32(2)), Instances: spec.Instances,
					}},
				}}
				cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{
					target: {Phase: appsv1.RunningComponentPhase},
				}
				labels := constant.GetCompLabels(cluster.Name, componentName)
				labels[constant.KBAppShardingNameLabelKey] = target
				labels[constant.KBAppShardTemplateLabelKey] = "special"
				shardLabels = labels
			}
			ops := &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "upgrade", Namespace: "ns"},
				Spec: opsv1alpha1.OpsRequestSpec{
					Type: opsv1alpha1.UpgradeType,
					SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
						Upgrade: &opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{{
							ComponentOps: opsv1alpha1.ComponentOps{ComponentName: target},
						}}},
					},
				},
				Status: opsv1alpha1.OpsRequestStatus{
					Phase: opsv1alpha1.OpsRunningPhase, StartTimestamp: metav1.NewTime(time.Now()),
				},
			}
			pod := func(name, image string) *corev1.Pod {
				return &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: name, Namespace: "ns", Labels: constant.GetCompLabels("cluster", componentName),
					},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: image}}},
					Status: corev1.PodStatus{
						Phase:             corev1.PodRunning,
						Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
						ContainerStatuses: []corev1.ContainerStatus{{Name: "db", Image: image, Ready: true}},
					},
				}
			}
			base := pod("cluster-"+componentName+"-0", "db:v2")
			base.Labels[constant.KBAppInstanceTemplateLabelKey] = ""
			fixed := pod("cluster-"+componentName+"-fixed-0", tc.templateImage)
			if tc.flat {
				fixed.Name = "cluster-" + componentName + "-1"
				fixed.Labels[constant.KBAppInstanceTemplateLabelKey] = "fixed"
			}
			if tc.wrongStatus {
				fixed.Status.ContainerStatuses[0].Image = "db:old"
			}
			if tc.unlabeled {
				delete(fixed.Labels, constant.KBAppInstanceTemplateLabelKey)
			}
			owner := &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-" + componentName, Namespace: "ns", Generation: 5,
					Labels:      constant.GetCompLabels(cluster.Name, componentName),
					Annotations: map[string]string{constant.KubeBlocksGenerationKey: "3"},
				},
			}
			if shardLabels != nil {
				owner.Labels = shardLabels
			}
			owner.Spec = appsv1.ComponentSpec{
				CompDef: spec.ComponentDef, ServiceVersion: spec.ServiceVersion, Replicas: spec.Replicas,
				Instances: spec.Instances, FlatInstanceOrdinal: spec.FlatInstanceOrdinal,
			}
			images := map[string]string{"db": "db:v1"}
			if tc.version == "0.0.0" {
				images = nil
			}
			if tc.name == "fixed template" {
				images["db"] = "db:owner"
			}
			its := &workloads.InstanceSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: owner.Name, Namespace: "ns", Labels: owner.Labels,
					Annotations: map[string]string{
						constant.KubeBlocksGenerationKey: "5", constant.AppComponentLabelKey: owner.Spec.CompDef,
						constant.KBAppServiceVersionKey: owner.Spec.ServiceVersion,
					},
				},
				Spec: workloads.InstanceSetSpec{
					Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "db", Image: "db:v2"}},
					}},
					Instances: []workloads.InstanceTemplate{{Name: "fixed", Images: images}},
				},
			}
			objects := []client.Object{def, ver, cluster, ops, base, fixed, owner, its}
			if tc.alternateDefinition {
				alternate := def.DeepCopy()
				alternate.Name = "alternate"
				def.Spec.Roles = []appsv1.ReplicaRole{{Name: "leader"}}
				ver.Labels[alternate.Name] = alternate.Name
				ver.Spec.CompatibilityRules[0].CompDefs = append(ver.Spec.CompatibilityRules[0].CompDefs, alternate.Name)
				owner.Spec.CompDef = alternate.Name
				its.Annotations[constant.AppComponentLabelKey] = alternate.Name
				cluster.Spec.Shardings[0].ShardTemplates[0].CompDef = ptr.To(alternate.Name)
				cluster.Spec.Shardings[0].ShardTemplates[0].ServiceVersion = ptr.To("2.0.0")
				objects = append(objects, alternate)
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops).WithObjects(objects...).Build()
			if tc.ownerRecovery {
				reconcileWaiting := func() {
					t.Helper()
					phase, requeue, err := (upgradeOpsHandler{}).ReconcileAction(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(100)})
					if err != nil || phase != opsv1alpha1.OpsRunningPhase || requeue <= 0 {
						t.Fatalf("owner wait phase=%s requeue=%s err=%v", phase, requeue, err)
					}
				}
				if tc.shard {
					if err := cli.Delete(context.Background(), owner); err != nil {
						t.Fatal(err)
					}
					reconcileWaiting()
					owner.ResourceVersion = ""
					if err := cli.Create(context.Background(), owner); err != nil {
						t.Fatal(err)
					}
					cluster.Spec.Shardings[0].ShardTemplates[0].Replicas = ptr.To(int32(2))
					cluster.Spec.Shardings[0].ShardTemplates[0].Instances[0].ServiceVersion = "1.0.0"
					owner.Spec.Replicas = 0
					owner.Spec.Instances = nil
				}
				owner.Annotations[constant.KubeBlocksGenerationKey] = "2"
				if err := cli.Update(context.Background(), owner); err != nil {
					t.Fatal(err)
				}
				reconcileWaiting()
				reconcileWaiting()
				if tc.shard {
					owner.Spec.Replicas = 2
					owner.Spec.Instances = spec.Instances
				}
				owner.Annotations[constant.KubeBlocksGenerationKey] = "3"
				if err := cli.Update(context.Background(), owner); err != nil {
					t.Fatal(err)
				}
				its.Annotations[constant.KubeBlocksGenerationKey] = "4"
				if err := cli.Update(context.Background(), its); err != nil {
					t.Fatal(err)
				}
				reconcileWaiting()
				reconcileWaiting()
				if err := cli.Delete(context.Background(), its); err != nil {
					t.Fatal(err)
				}
				reconcileWaiting()
				reconcileWaiting()
				its.ResourceVersion = ""
				its.Annotations[constant.KubeBlocksGenerationKey] = "5"
				its.Annotations[constant.KBAppServiceVersionKey] = "1.0.0"
				if err := cli.Create(context.Background(), its); err != nil {
					t.Fatal(err)
				}
				reconcileWaiting()
				its.Annotations[constant.KBAppServiceVersionKey] = owner.Spec.ServiceVersion
				if err := cli.Update(context.Background(), its); err != nil {
					t.Fatal(err)
				}
				fixed.Spec.Containers[0].Image = "db:wrong"
				if err := cli.Update(context.Background(), fixed); err != nil {
					t.Fatal(err)
				}
				phase, _, err := (upgradeOpsHandler{}).ReconcileAction(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(100)})
				if err != nil || phase != opsv1alpha1.OpsRunningPhase || ops.Status.Progress != "1/2" {
					t.Fatalf("wrong image phase=%s progress=%s err=%v", phase, ops.Status.Progress, err)
				}
				fixed.Spec.Containers[0].Image = "db:v1"
				if err := cli.Update(context.Background(), fixed); err != nil {
					t.Fatal(err)
				}
			}
			phase, _, err := (upgradeOpsHandler{}).ReconcileAction(
				intctrlutil.RequestCtx{Ctx: context.Background()}, cli,
				&OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(100)},
			)
			if err != nil {
				t.Fatal(err)
			}
			if phase != tc.want || ops.Status.Progress != tc.progress {
				t.Fatalf("phase=%s progress=%s, want %s %s", phase, ops.Status.Progress, tc.want, tc.progress)
			}
		})
	}
}

func TestUpgradeZeroShards(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		appsv1.AddToScheme, opsv1alpha1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "ns", Generation: 3},
		Spec: appsv1.ClusterSpec{Shardings: []appsv1.ClusterSharding{{
			Name: "shards", Shards: 0,
			Template: appsv1.ClusterComponentSpec{ComponentDef: "db", ServiceVersion: "2.0.0", Replicas: 2},
		}}},
	}
	ops := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "upgrade", Namespace: "ns"},
		Spec: opsv1alpha1.OpsRequestSpec{
			Type: opsv1alpha1.UpgradeType,
			SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
				Upgrade: &opsv1alpha1.Upgrade{Components: []opsv1alpha1.UpgradeComponent{{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "shards"},
				}}},
			},
		},
		Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops).WithObjects(cluster, ops).Build()
	phase, requeue, err := (upgradeOpsHandler{}).ReconcileAction(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli,
		&OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(100)},
	)
	if err != nil || phase != opsv1alpha1.OpsSucceedPhase || requeue != 0 || ops.Status.Progress != "0/0" {
		t.Fatalf("phase=%s requeue=%s progress=%s err=%v, want Succeed with 0/0", phase, requeue, ops.Status.Progress, err)
	}
	if _, err := GetOpsManager().Reconcile(intctrlutil.RequestCtx{Ctx: context.Background()}, cli,
		&OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(100)}); err != nil {
		t.Fatal(err)
	}
	stored := &opsv1alpha1.OpsRequest{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(ops), stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.Phase != opsv1alpha1.OpsSucceedPhase || stored.Status.Progress != "0/0" {
		t.Fatalf("persisted phase=%s progress=%s, want Succeed with 0/0", stored.Status.Phase, stored.Status.Progress)
	}
	cluster.Spec.Shardings[0].Shards = 1
	pending := ops.DeepCopy()
	pending.Name = "pending"
	pending.ResourceVersion = ""
	pending.Status = opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase, StartTimestamp: metav1.Now()}
	if err := cli.Create(context.Background(), pending); err != nil {
		t.Fatal(err)
	}
	pendingRes := &OpsResource{Cluster: cluster, OpsRequest: pending, Recorder: record.NewFakeRecorder(100)}
	if requeue, err := GetOpsManager().Reconcile(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, pendingRes); err != nil || requeue <= 0 {
		t.Fatalf("missing shard requeue=%s err=%v, want a retry", requeue, err)
	}
	pending.Spec.TimeoutSeconds = ptr.To(int32(1))
	pending.Status.StartTimestamp = metav1.NewTime(time.Now().Add(-2 * time.Second))
	if _, err := GetOpsManager().Reconcile(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, pendingRes); err != nil {
		t.Fatal(err)
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pending), stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.Phase != opsv1alpha1.OpsAbortedPhase {
		t.Fatalf("missing shard timed out with phase=%s, want Aborted", stored.Status.Phase)
	}
}
