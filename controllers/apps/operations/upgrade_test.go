/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
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
		Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))
		// do upgrade
		_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
		Expect(err).ShouldNot(HaveOccurred())
		mockComponentIsOperating(opsRes.Cluster, appsv1alpha1.UpdatingClusterCompPhase, defaultCompName)
	}

	mockClusterRunning := func(clusterObject *appsv1alpha1.Cluster) {
		Expect(testapps.ChangeObjStatus(&testCtx, clusterObject, func() {
			clusterObject.Status.Phase = appsv1alpha1.RunningClusterPhase
			clusterObject.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
				defaultCompName: {
					Phase: appsv1alpha1.RunningClusterCompPhase,
				},
			}
		})).Should(Succeed())
	}

	initOpsResWithComponentDef := func(createCompVersion bool) (*appsv1alpha1.ComponentDefinition, *appsv1alpha1.ComponentDefinition, *OpsResource) {
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

	createUpgradeOpsRequest := func(clusterObject *appsv1alpha1.Cluster, upgradeSpec appsv1alpha1.Upgrade) *appsv1alpha1.OpsRequest {
		ops := testapps.NewOpsRequestObj("upgrade-ops-"+randomStr, testCtx.DefaultNamespace,
			clusterObject.Name, appsv1alpha1.UpgradeType)
		ops.Spec.Upgrade = &upgradeSpec
		opsRequest := testapps.CreateOpsRequest(ctx, testCtx, ops)
		// set ops phase to Pending
		opsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase
		return opsRequest
	}

	expectOpsSucceed := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, compNames ...string) {
		// mock component to running
		mockComponentIsOperating(opsRes.Cluster, appsv1alpha1.RunningClusterCompPhase, compNames...)
		_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsSucceedPhase))
	}

	mockPodsAppliedImage := func(cluster *appsv1alpha1.Cluster, releaseVersion string) {
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
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, appsv1alpha1.Upgrade{
				Components: []appsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            appsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ComponentDefinitionName: &compDef2.Name,
					},
				},
			})

			By("mock upgrade OpsRequest phase is Running")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, ops *appsv1alpha1.OpsRequest) {
				g.Expect(ops.Status.LastConfiguration.Components[defaultCompName].ComponentDefinitionName).Should(Equal(compDef1.Name))
			})).Should(Succeed())

			By("expect upgrade successfully with the image that is provided in the specified componentDefinition")
			mockPodsAppliedImage(opsRes.Cluster, release2)
			expectOpsSucceed(reqCtx, opsRes, defaultCompName)
		})

		It("Test upgrade OpsRequest with ComponentDef and ComponentVersion", func() {
			By("init operations resources")
			_, compDef2, opsRes := initOpsResWithComponentDef(true)

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, appsv1alpha1.Upgrade{
				Components: []appsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            appsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ServiceVersion:          pointer.String(serviceVer2),
						ComponentDefinitionName: &compDef2.Name,
					},
				},
			})

			By("expect for this opsRequest is Running")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1alpha1.Cluster) {
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
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, appsv1alpha1.Upgrade{
				Components: []appsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            appsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ComponentDefinitionName: pointer.String(""),
						ServiceVersion:          pointer.String(serviceVer1),
					},
				},
			})

			By("expect for this opsRequest is Running and reuse the original ComponentDefinition")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1alpha1.Cluster) {
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
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, appsv1alpha1.Upgrade{
				Components: []appsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            appsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ComponentDefinitionName: pointer.String(compDef1.Name),
						ServiceVersion:          pointer.String(""),
					},
				},
			})

			By(" expect for this opsRequest is Running")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1alpha1.Cluster) {
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
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, appsv1alpha1.Upgrade{
				Components: []appsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            appsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ComponentDefinitionName: pointer.String(compDef2.Name),
					},
				},
			})

			By("expecting no changes to serviceVersion")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDef2.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(serviceVer0))
			})).Should(Succeed())
		})

		It("Test upgrade OpsRequest when componentDefinitionName is nil", func() {
			By("init operations resources")
			compDef1, _, opsRes := initOpsResWithComponentDef(true)

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, appsv1alpha1.Upgrade{
				Components: []appsv1alpha1.UpgradeComponent{
					{
						ComponentOps:   appsv1alpha1.ComponentOps{ComponentName: defaultCompName},
						ServiceVersion: pointer.String(""),
					},
				},
			})

			By("expecting no changes to cluster.spec.componentSpec[0].componentDef")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDef1.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(""))
			})).Should(Succeed())
		})
		// TODO: add case with ClusterDefinition and topology
	})
})
