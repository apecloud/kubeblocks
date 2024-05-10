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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Upgrade OpsRequest", func() {

	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
	)
	const mysqlImageForUpdate = "docker.io/apecloud/apecloud-mysql-server:8.0.30"
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentDefinitionSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentVersionSignature, true, ml)

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
		mockComponentIsOperating(opsRes.Cluster, appsv1alpha1.UpdatingClusterCompPhase,
			consensusComp, statelessComp, statefulComp)
	}

	mockClusterRunning := func(clusterObject *appsv1alpha1.Cluster) {
		Expect(testapps.ChangeObjStatus(&testCtx, clusterObject, func() {
			clusterObject.Status.Phase = appsv1alpha1.RunningClusterPhase
			clusterObject.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
				consensusComp: {
					Phase: appsv1alpha1.RunningClusterCompPhase,
				},
			}
		})).Should(Succeed())
	}

	initOpsResWithComponentDef := func() ([]*appsv1alpha1.ComponentDefinition, *OpsResource) {
		compDefObjs := testapps.CreateCompDefinitionObjs(&testCtx, false)
		compVersion := testapps.CreateCompVersionObj(&testCtx, false)
		// label the componentDef info to the ComponentVersion
		Expect(testapps.ChangeObj(&testCtx, compVersion, func(version *appsv1alpha1.ComponentVersion) {
			if version.Labels == nil {
				version.Labels = map[string]string{}
			}
			for _, v := range compDefObjs {
				version.Labels[v.Name] = v.Name
			}
		})).Should(Succeed())

		// mock ComponentVersion to Available
		Expect(testapps.ChangeObjStatus(&testCtx, compVersion, func() {
			compVersion.Status.Phase = appsv1alpha1.AvailablePhase
			compVersion.Status.ObservedGeneration = compVersion.Generation
		})).Should(Succeed())
		// create the cluster with no clusterDefinition
		clusterObject := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "", "").
			AddComponentV2(consensusComp, compDefObjs[0].Name).
			SetServiceVersion(testapps.ServiceVersion("v0")).
			SetReplicas(int32(3)).Create(&testCtx).GetObject()
		opsRes := &OpsResource{
			Cluster:  clusterObject,
			Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
		}
		// mock component phase to running
		mockClusterRunning(clusterObject)
		return compDefObjs, opsRes
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

	checkPodImages := func(pods []*corev1.Pod, releaseId string) {
		for i := range pods {
			pod := pods[i]
			Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{
					{
						Name: testapps.AppName,
						// the latest release version will be selected.
						Image: testapps.AppImage(testapps.AppName, testapps.ReleaseID(releaseId)),
					},
					{
						Name: testapps.AppNameSamePrefix,
						// the latest release version will be selected.
						Image: testapps.AppImage(testapps.AppNameSamePrefix, testapps.ReleaseID(releaseId)),
					},
				}
			})).Should(Succeed())
		}
	}

	Context("Test OpsRequest", func() {
		It("Test upgrade OpsRequest with ClusterVersion", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, clusterObject := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)

			By("create Upgrade Ops")
			newClusterVersionName := "clusterversion-upgrade-" + randomStr
			_ = testapps.NewClusterVersionFactory(newClusterVersionName, clusterDefinitionName).
				AddComponentVersion(consensusComp).AddContainerShort(testapps.DefaultMySQLContainerName, mysqlImageForUpdate).
				Create(&testCtx).GetObject()
			opsRes.OpsRequest = createUpgradeOpsRequest(clusterObject, appsv1alpha1.Upgrade{ClusterVersionRef: &newClusterVersionName})

			By("mock upgrade OpsRequest phase is Running")
			makeUpgradeOpsIsRunning(reqCtx, opsRes)

			By("expect upgrade successfully")
			_ = testapps.MockInstanceSetPod(&testCtx, nil, clusterName, statelessComp, fmt.Sprintf(clusterName+"-"+statelessComp+"-0"), "", "")
			_ = testapps.MockInstanceSetPods(&testCtx, nil, clusterName, statefulComp)
			pods := testapps.MockInstanceSetPods(&testCtx, nil, clusterName, consensusComp)
			for i := range pods {
				pod := pods[i]
				Expect(testapps.ChangeObj(&testCtx, pod, func(pod *corev1.Pod) {
					pod.Spec.Containers[0].Image = mysqlImageForUpdate
				})).Should(Succeed())
				Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
					pod.Status.ContainerStatuses = []corev1.ContainerStatus{
						{
							Name:  testapps.DefaultMySQLContainerName,
							Image: mysqlImageForUpdate,
						},
					}
				})).Should(Succeed())
			}
			// mock component to running
			expectOpsSucceed(reqCtx, opsRes, consensusComp, statelessComp, statefulComp)
		})

		It("Test upgrade OpsRequest with ComponentDef", func() {
			By("init operations resources ")
			compDef1 := testapps.NewComponentDefinitionFactory(testapps.CompDefName(testapps.CompDefNames[0])).
				SetServiceVersion(testapps.ServiceVersion(testapps.ServiceVersions[0])).SetRuntime(&corev1.Container{
				Name: testapps.DefaultMySQLContainerName, Image: testapps.AppImage(testapps.AppName, testapps.ReleaseID("r0")),
			}).Create(&testCtx).GetObject()

			compDef2 := testapps.NewComponentDefinitionFactory(testapps.CompDefName(testapps.CompDefNames[1])).
				SetServiceVersion(testapps.ServiceVersion(testapps.ServiceVersions[1])).SetRuntime(&corev1.Container{
				Name: testapps.DefaultMySQLContainerName, Image: testapps.AppImage(testapps.AppName, testapps.ReleaseID("r1")),
			}).Create(&testCtx).GetObject()
			clusterObject := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "", "").
				AddComponentV2(consensusComp, compDef1.Name).
				SetReplicas(int32(3)).Create(&testCtx).GetObject()
			opsRes := &OpsResource{
				Cluster:  clusterObject,
				Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
			}
			// mock component phase to running
			mockClusterRunning(clusterObject)

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(clusterObject, appsv1alpha1.Upgrade{
				Components: []appsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            appsv1alpha1.ComponentOps{ComponentName: consensusComp},
						ComponentDefinitionName: &compDef2.Name,
					},
				},
			})

			By("mock upgrade OpsRequest phase is Running")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, ops *appsv1alpha1.OpsRequest) {
				g.Expect(ops.Status.LastConfiguration.Components[consensusComp].ComponentDefinitionName).Should(Equal(compDef1.Name))
			})).Should(Succeed())

			By("expect upgrade successfully")
			pods := testapps.MockInstanceSetPods(&testCtx, nil, clusterName, consensusComp)
			for i := range pods {
				pod := pods[i]
				Expect(testapps.ChangeObj(&testCtx, pod, func(pod *corev1.Pod) {
					pod.Spec.Containers[0].Image = testapps.AppImage(testapps.AppName, testapps.ReleaseID("r1"))
				})).Should(Succeed())
				Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
					pod.Status.ContainerStatuses = []corev1.ContainerStatus{
						{
							Name:  testapps.DefaultMySQLContainerName,
							Image: testapps.AppImage(testapps.AppName, testapps.ReleaseID("r1")),
						},
					}
				})).Should(Succeed())
			}
			expectOpsSucceed(reqCtx, opsRes, consensusComp)
		})

		It("Test upgrade OpsRequest with ComponentDef and ComponentVersion", func() {
			By("init operations resources")
			compDefObjs, opsRes := initOpsResWithComponentDef()

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, appsv1alpha1.Upgrade{
				Components: []appsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            appsv1alpha1.ComponentOps{ComponentName: consensusComp},
						ServiceVersion:          pointer.String(testapps.ServiceVersion("v1")),
						ComponentDefinitionName: &compDefObjs[0].Name,
					},
				},
			})

			By("expect for this opsRequest is Running")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDefObjs[0].Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			})).Should(Succeed())

			By("expect upgrade successfully")
			pods := testapps.MockInstanceSetPods(&testCtx, nil, clusterName, consensusComp)
			checkPodImages(pods, "r4")
			expectOpsSucceed(reqCtx, opsRes, consensusComp)
		})

		It("Test upgrade OpsRequest without ComponentDefinitionName but the cluster is created without clusterDefinition", func() {
			By("init operations resources")
			compDefs, opsRes := initOpsResWithComponentDef()

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, appsv1alpha1.Upgrade{
				Components: []appsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            appsv1alpha1.ComponentOps{ComponentName: consensusComp},
						ComponentDefinitionName: pointer.String(""),
						ServiceVersion:          pointer.String(testapps.ServiceVersion("v1")),
					},
				},
			})

			By("reuse the original ComponentDefinition and expect for this opsRequest is Running")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDefs[0].Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			})).Should(Succeed())

			By("expect upgrade successfully")
			pods := testapps.MockInstanceSetPods(&testCtx, nil, clusterName, consensusComp)
			checkPodImages(pods, "r4")
			expectOpsSucceed(reqCtx, opsRes, consensusComp)
		})

		It("Test upgrade OpsRequest when specified serviceVersion is empty", func() {
			By("init operations resources")
			compDefObjs, opsRes := initOpsResWithComponentDef()

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, appsv1alpha1.Upgrade{
				Components: []appsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            appsv1alpha1.ComponentOps{ComponentName: consensusComp},
						ComponentDefinitionName: pointer.String(compDefObjs[0].Name),
						ServiceVersion:          pointer.String(""),
					},
				},
			})

			By(" expect for this opsRequest is Running")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDefObjs[0].Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(BeEmpty())
			})).Should(Succeed())

			By("looking forward to using the latest serviceVersion")
			pods := testapps.MockInstanceSetPods(&testCtx, nil, clusterName, consensusComp)
			checkPodImages(pods, "r5")
			expectOpsSucceed(reqCtx, opsRes, consensusComp)
		})

		It("Test upgrade OpsRequest when serviceVersion is nil", func() {
			By("init operations resources")
			compDefObjs, opsRes := initOpsResWithComponentDef()

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, appsv1alpha1.Upgrade{
				Components: []appsv1alpha1.UpgradeComponent{
					{
						ComponentOps:            appsv1alpha1.ComponentOps{ComponentName: consensusComp},
						ComponentDefinitionName: pointer.String(compDefObjs[1].Name),
					},
				},
			})

			By("expecting no changes to serviceVersion")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDefObjs[1].Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(testapps.ServiceVersion("v0")))
			})).Should(Succeed())
		})

		It("Test upgrade OpsRequest when componentDefinitionName is nil", func() {
			By("init operations resources")
			compDefObjs, opsRes := initOpsResWithComponentDef()

			By("create Upgrade Ops")
			opsRes.OpsRequest = createUpgradeOpsRequest(opsRes.Cluster, appsv1alpha1.Upgrade{
				Components: []appsv1alpha1.UpgradeComponent{
					{
						ComponentOps:   appsv1alpha1.ComponentOps{ComponentName: consensusComp},
						ServiceVersion: pointer.String(""),
					},
				},
			})

			By("expecting no changes to componentDef")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			makeUpgradeOpsIsRunning(reqCtx, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDefObjs[0].Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(""))
			})).Should(Succeed())
		})
		// TODO: add case with ClusterDefinition and topology
	})
})
