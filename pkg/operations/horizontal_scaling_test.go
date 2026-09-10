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
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	k8sappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	opsutil "github.com/apecloud/kubeblocks/pkg/operations/util"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

var _ = Describe("HorizontalScaling OpsRequest", func() {

	var (
		randomStr       = testCtx.GetRandomStr()
		compDefName     = "test-compdef-" + randomStr
		shardingDefName = "test-shardingdef-" + randomStr
		clusterName     = "test-cluster-" + randomStr
		insTplName      = "foo"
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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
		// default GracePeriod is 30s
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.RestoreSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentSignature, true, inNS)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	initClusterAnnotationAndPhaseForOps := func(opsRes *OpsResource) {
		Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
		Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
			opsRes.Cluster.Status.Phase = appsv1.RunningClusterPhase
		})).ShouldNot(HaveOccurred())
	}

	Context("Test OpsRequest", func() {
		commonHScaleConsensusCompTest := func(reqCtx intctrlutil.RequestCtx,
			changeClusterSpec func(cluster *appsv1.Cluster),
			horizontalScaling opsv1alpha1.HorizontalScaling,
			ignoreHscaleStrictValidate,
			skipCompReplicasCheck bool) (*OpsResource, []*corev1.Pod) {
			By("init operations resources with CLusterDefinition/Hybrid components Cluster/consensus Pods")
			opsRes, compDef, _ := initOperationsResources(compDefName, clusterName)
			// mock component
			comp, err := component.BuildComponent(opsRes.Cluster, &opsRes.Cluster.Spec.ComponentSpecs[0], nil, nil)
			Expect(err).Should(BeNil())
			Expect(k8sClient.Create(testCtx.Ctx, comp)).Should(Succeed())
			Expect(testapps.ChangeObj(&testCtx, compDef, func(compDef *appsv1.ComponentDefinition) {
				compDef.Spec.ReplicasLimit = &appsv1.ReplicasLimit{
					MinReplicas: 0,
					MaxReplicas: 10,
				}
			})).Should(Succeed())
			its := testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			if changeClusterSpec != nil {
				Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(cluster *appsv1.Cluster) {
					changeClusterSpec(cluster)
				})).Should(Succeed())
			}
			pods := testapps.MockInstanceSetPods(&testCtx, its, opsRes.Cluster, defaultCompName)
			if !hscaleFromBackup(horizontalScaling) {
				_, err = publishHScaleAllocation(ctx, k8sClient, opsRes.Cluster, defaultCompName, &opsRes.Cluster.Spec.ComponentSpecs[0])
				Expect(err).ShouldNot(HaveOccurred())
			}
			By("create opsRequest for horizontal scaling of consensus component")
			initClusterAnnotationAndPhaseForOps(opsRes)
			horizontalScaling.ComponentName = defaultCompName
			opsRes.OpsRequest = createHorizontalScaling(clusterName, horizontalScaling, ignoreHscaleStrictValidate)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			mockComponentIsOperating(opsRes.Cluster, appsv1.UpdatingComponentPhase, defaultCompName)

			By("expect for opsRequest phase is Creating after doing action")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("check for the replicas of consensus component after doing action again when opsRequest phase is Creating")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			if !skipCompReplicasCheck {
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, tmpCluster *appsv1.Cluster) {
					lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[defaultCompName]
					expectedCompReplicas := *lastCompConfiguration.Replicas
					scaleIn := horizontalScaling.ScaleIn
					if scaleIn != nil {
						if scaleIn.ReplicaChanges != nil {
							expectedCompReplicas -= *scaleIn.ReplicaChanges
						} else {
							expectedCompReplicas -= int32(len(scaleIn.OnlineInstancesToOffline))
						}
					}
					scaleOut := horizontalScaling.ScaleOut
					if scaleOut != nil {
						switch {
						case scaleOut.ReplicaChanges != nil:
							expectedCompReplicas += *scaleOut.ReplicaChanges
						case len(scaleOut.NewInstances) > 0:
							for _, v := range scaleOut.NewInstances {
								expectedCompReplicas += *v.Replicas
							}
						default:
							expectedCompReplicas += int32(len(scaleOut.OfflineInstancesToOnline))
						}
					}
					g.Expect(tmpCluster.Spec.GetComponentByName(defaultCompName).Replicas).Should(BeEquivalentTo(expectedCompReplicas))
				})).Should(Succeed())
			}

			By("Test OpsManager.Reconcile function when horizontal scaling OpsRequest is Running")
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			return opsRes, pods
		}

		checkOpsRequestPhaseIsSucceed := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource) {
			By("expect for opsRequest phase is Succeed after pods has been scaled and component phase is Running")
			// mock consensus component is Running
			mockConsensusCompToRunning(opsRes)
			if !hscaleFromBackup(opsRes.OpsRequest.Spec.HorizontalScalingList[0]) {
				_, err := publishHScaleAllocation(ctx, k8sClient, opsRes.Cluster, defaultCompName, &opsRes.Cluster.Spec.ComponentSpecs[0])
				Expect(err).ShouldNot(HaveOccurred())
			}
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
		}

		checkCancelledSucceed := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource) {
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsCancelledPhase))
			opsProgressDetails := opsRes.OpsRequest.Status.Components[defaultCompName].ProgressDetails
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/2"))
			Expect(len(opsProgressDetails)).Should(Equal(2))
		}

		deletePods := func(pods ...*corev1.Pod) {
			for i := range pods {
				testk8s.MockPodIsTerminating(ctx, testCtx, pods[i])
				testk8s.RemovePodFinalizer(ctx, testCtx, pods[i])
			}
		}

		createPods := func(templateName string, ordinals ...int) []*corev1.Pod {
			var pods []*corev1.Pod
			prefix := ""
			if templateName != "" {
				prefix = "-" + templateName
			}
			for i := range ordinals {
				podName := fmt.Sprintf("%s-%s%s-%d", clusterName, defaultCompName, prefix, ordinals[i])
				pod := testapps.MockInstanceSetPod(&testCtx, nil, clusterName, defaultCompName, podName, "follower")
				pods = append(pods, pod)
			}
			return pods
		}

		testHScaleReplicas := func(
			changeClusterSpec func(cluster *appsv1.Cluster),
			horizontalScaling opsv1alpha1.HorizontalScaling,
			mockHScale func(podList []*corev1.Pod)) {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, podList := commonHScaleConsensusCompTest(reqCtx, changeClusterSpec, horizontalScaling, false, false)
			mockHScale(podList)
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
		}

		testCancelHScale := func(
			horizontalScaling opsv1alpha1.HorizontalScaling,
			isScaleDown bool) {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, podList := commonHScaleConsensusCompTest(reqCtx, nil, horizontalScaling, false, false)
			var pod *corev1.Pod
			if isScaleDown {
				By("delete the pod")
				pod = podList[2]
				deletePods(pod)
			} else {
				By("create the pod")
				pod = createPods("", 3)[0]
			}
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			By("cancel HScale opsRequest after one pod has been deleted")
			cancelOpsRequest(reqCtx, opsRes, time.Now().Add(-1*time.Second))
			if isScaleDown {
				By("re-create the pod for rollback")
				createPods("", 2)
			} else {
				By("delete the pod for rollback")
				deletePods(pod)
			}
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			By("expect for opsRequest phase is Succeed after pods has been scaled and component phase is Running")
			mockConsensusCompToRunning(opsRes)
			checkCancelledSucceed(reqCtx, opsRes)
			Expect(findStatusProgressDetail(opsRes.OpsRequest.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(constant.PodKind, pod.Name)).Status).Should(Equal(opsv1alpha1.SucceedProgressStatus))
		}

		It("test to scale out replicas with `scaleOut`", func() {
			By("scale out replicas from 3 to 5 with `scaleOut`")
			horizontalScaling := opsv1alpha1.HorizontalScaling{ScaleOut: &opsv1alpha1.ScaleOut{}}
			horizontalScaling.ScaleOut.ReplicaChanges = pointer.Int32(2)
			testHScaleReplicas(nil, horizontalScaling, func(podList []*corev1.Pod) {
				By("create the pods(ordinal:[3,4])")
				createPods("", 3, 4)
			})
		})

		It("test to scale out replicas from a full backup", func() {
			By("create Backup")
			backupName := "backup-for-ops-" + randomStr
			backup := testdp.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				SetBackupPolicyName(testdp.BackupPolicyName).
				SetBackupMethod(testdp.VSBackupMethodName).
				Create(&testCtx).GetObject()

			Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
				backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
				backup.Status.BackupMethod = &dpv1alpha1.BackupMethod{
					Name:            testdp.VSBackupMethodName,
					SnapshotVolumes: pointer.Bool(true),
					TargetVolumes: &dpv1alpha1.TargetVolumeInfo{
						Volumes: []string{"data"},
					},
				}
				backup.Status.Targets = []dpv1alpha1.BackupStatusTarget{
					{BackupTarget: dpv1alpha1.BackupTarget{
						Name:        "target-a",
						PodSelector: &dpv1alpha1.PodSelector{},
					}},
					{BackupTarget: dpv1alpha1.BackupTarget{
						Name:        "target-b",
						PodSelector: &dpv1alpha1.PodSelector{Strategy: dpv1alpha1.PodSelectionStrategyAll},
					}},
				}
			})).Should(Succeed())

			By("scale out replicas from a full backup")
			restoreEnv := []corev1.EnvVar{{Name: "RESTORE_ENV", Value: "true"}}
			horizontalScaling := opsv1alpha1.HorizontalScaling{ScaleOut: &opsv1alpha1.ScaleOut{
				FromBackup: &opsv1alpha1.FromBackup{
					Name:             backupName,
					SourceTargetName: "target-b",
					RestoreEnv:       restoreEnv,
				},
			}}

			horizontalScaling.ScaleOut.ReplicaChanges = pointer.Int32(2)
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx, Recorder: eventRecorder}
			opsRes, _ := commonHScaleConsensusCompTest(reqCtx, nil, horizontalScaling, false, true)
			restoreList := &dpv1alpha1.RestoreList{}
			Expect(k8sClient.List(ctx, restoreList, client.MatchingLabels{
				constant.OpsRequestNameLabelKey: opsRes.OpsRequest.Name,
			}, client.InNamespace(opsRes.OpsRequest.Namespace))).Should(Succeed())
			Expect(restoreList.Items).Should(HaveLen(2))
			for i := range restoreList.Items {
				Expect(restoreList.Items[i].Spec.Backup.SourceTargetName).Should(Equal("target-b"))
				Expect(restoreList.Items[i].Spec.PrepareDataConfig.RequiredPolicyForAllPodSelection).NotTo(BeNil())
				Expect(restoreList.Items[i].Spec.PrepareDataConfig.RequiredPolicyForAllPodSelection.DataRestorePolicy).
					Should(Equal(dpv1alpha1.OneToOneRestorePolicy))
			}

			By("mock restore phase to completed")
			comp, compDef, err := component.GetCompNCompDefByName(reqCtx.Ctx, k8sClient, opsRes.Cluster.Namespace, constant.GenerateClusterComponentName(opsRes.Cluster.Name, defaultCompName))
			Expect(err).Should(BeNil())
			synthesizedComponent, err := component.BuildSynthesizedComponent(reqCtx.Ctx, k8sClient, compDef, comp)
			Expect(err).Should(BeNil())
			changeRestorePhaseToComplete := func(index int32) {
				restoreMGR := plan.NewRestoreManager(reqCtx.Ctx, k8sClient, opsRes.Cluster, model.GetScheme(), map[string]string{
					constant.OpsRequestNameLabelKey: opsRes.OpsRequest.Name,
				}, 1, index)
				// check restore status
				restoreMeta := restoreMGR.GetRestoreObjectMeta(synthesizedComponent, dpv1alpha1.PrepareData, "")
				testapps.GetAndChangeObjStatus(&testCtx, types.NamespacedName{
					Namespace: restoreMeta.Namespace,
					Name:      restoreMeta.Name,
				}, func(restore *dpv1alpha1.Restore) {
					Expect(restore.Spec.Env).Should(Equal(restoreEnv))
					restore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
				})
			}
			changeRestorePhaseToComplete(3)
			changeRestorePhaseToComplete(4)

			By("expect component replicas to 5")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.GetComponentByName(defaultCompName).Replicas).Should(Equal(pointer.Int32(5)))
			}))

			By("mock pods to created and expect opsRequest phase to Succeed")
			createPods("", 3, 4)
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
		})

		It("test to scale in replicas with `scaleIn`", func() {
			By("scale in replicas from 3 to 1")
			horizontalScaling := opsv1alpha1.HorizontalScaling{ScaleIn: &opsv1alpha1.ScaleIn{}}
			horizontalScaling.ScaleIn.ReplicaChanges = pointer.Int32(2)
			testHScaleReplicas(nil, horizontalScaling, func(podList []*corev1.Pod) {
				By("delete the pods")
				deletePods(podList[1], podList[2])
			})
		})

		It("cancel the opsRequest which scaling in replicas with `replicas`", func() {
			By("scale in replicas of component from 3 to 1")
			testCancelHScale(opsv1alpha1.HorizontalScaling{ScaleIn: &opsv1alpha1.ScaleIn{
				ReplicaChanger: opsv1alpha1.ReplicaChanger{
					ReplicaChanges: pointer.Int32(2),
				},
			}}, true)
		})

		It("cancel the opsRequest which scaling out replicas with `replicas`", func() {
			By("scale out replicas of component from 3 to 5")
			testCancelHScale(opsv1alpha1.HorizontalScaling{ScaleOut: &opsv1alpha1.ScaleOut{
				ReplicaChanger: opsv1alpha1.ReplicaChanger{
					ReplicaChanges: pointer.Int32(2),
				},
			}}, false)
		})

		It("cancel the opsRequest which scaling out replicas with `scaleOut`", func() {
			By("scale out replicas of component from 3 to 5")
			testCancelHScale(opsv1alpha1.HorizontalScaling{ScaleOut: &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(2)}}}, false)
		})

		It("cancel the opsRequest which scaling in replicas with `scaleIn`", func() {
			By("scale in replicas of component from 3 to 1")
			testCancelHScale(opsv1alpha1.HorizontalScaling{ScaleIn: &opsv1alpha1.ScaleIn{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(2)}}}, true)
		})

		setClusterCompSpec := func(cluster *appsv1.Cluster, instances []appsv1.InstanceTemplate, offlineInstances []string) {
			for i, v := range cluster.Spec.ComponentSpecs {
				if v.Name == defaultCompName {
					cluster.Spec.ComponentSpecs[i].OfflineInstances = offlineInstances
					cluster.Spec.ComponentSpecs[i].Instances = instances
					break
				}
			}
		}

		testHScaleWithSpecifiedPod := func(changeClusterSpec func(cluster *appsv1.Cluster),
			horizontalScaling opsv1alpha1.HorizontalScaling,
			expectOfflineInstances []string,
			mockHScale func(podList []*corev1.Pod),
			ignoreHscalingStrictValidate bool) *OpsResource {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, podList := commonHScaleConsensusCompTest(reqCtx, changeClusterSpec, horizontalScaling, ignoreHscalingStrictValidate, false)
			By("verify cluster spec is correct")
			targetSpec := opsRes.Cluster.Spec.GetComponentByName(defaultCompName)
			Expect(targetSpec.OfflineInstances).Should(HaveLen(len(expectOfflineInstances)))
			expectedOfflineInsSet := sets.New(expectOfflineInstances...)
			for _, v := range targetSpec.OfflineInstances {
				_, ok := expectedOfflineInsSet[v]
				Expect(ok).Should(BeTrue())
			}

			By("mock specified pods deleted or created")
			mockHScale(podList)
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
			return opsRes
		}

		It("test offline the specified pod of the component", func() {
			toDeletePodName := fmt.Sprintf("%s-%s-1", clusterName, defaultCompName)
			offlineInstances := []string{toDeletePodName}
			opsRes := testHScaleWithSpecifiedPod(func(cluster *appsv1.Cluster) {
				setClusterCompSpec(cluster, []appsv1.InstanceTemplate{
					{Name: insTplName, Replicas: pointer.Int32(1)},
				}, nil)
			}, opsv1alpha1.HorizontalScaling{
				ScaleIn: &opsv1alpha1.ScaleIn{
					ReplicaChanger:           opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)},
					OnlineInstancesToOffline: offlineInstances,
				},
			}, offlineInstances, func(podList []*corev1.Pod) {
				By(fmt.Sprintf(`delete the specified pod "%s"`, toDeletePodName))
				deletePods(podList[2])
			}, false)
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/1"))
		})

		It("test offline the specified pod and scale out another replicas", func() {
			toDeletePodName := fmt.Sprintf("%s-%s-1", clusterName, defaultCompName)
			offlineInstances := []string{toDeletePodName}
			opsRes := testHScaleWithSpecifiedPod(func(cluster *appsv1.Cluster) {
				setClusterCompSpec(cluster, []appsv1.InstanceTemplate{
					{Name: insTplName, Replicas: pointer.Int32(1)},
				}, nil)
			}, opsv1alpha1.HorizontalScaling{
				ScaleIn: &opsv1alpha1.ScaleIn{
					ReplicaChanger:           opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)},
					OnlineInstancesToOffline: offlineInstances,
				},
				ScaleOut: &opsv1alpha1.ScaleOut{
					ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)},
				},
			}, offlineInstances, func(podList []*corev1.Pod) {
				By(fmt.Sprintf(`delete the specified pod "%s"`, toDeletePodName))
				deletePods(podList[2])
				By("create a new pod(ordinal:2) by replicas")
				createPods("", 2)
			}, false)
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/2"))
		})

		It("test offline the specified pod and auto-sync replicaChanges", func() {
			offlineInstanceName := fmt.Sprintf("%s-%s-%s-0", clusterName, defaultCompName, insTplName)
			offlineInstances := []string{offlineInstanceName}
			opsRes := testHScaleWithSpecifiedPod(func(cluster *appsv1.Cluster) {
				setClusterCompSpec(cluster, []appsv1.InstanceTemplate{
					{Name: insTplName, Replicas: pointer.Int32(1)},
				}, nil)
			}, opsv1alpha1.HorizontalScaling{
				ScaleIn: &opsv1alpha1.ScaleIn{
					OnlineInstancesToOffline: offlineInstances,
				},
			}, offlineInstances, func(podList []*corev1.Pod) {
				By("delete the specified pod " + offlineInstanceName)
				deletePods(podList[0])
			}, false)
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/1"))
			By("expect replicas to 2 and template " + insTplName + " replicas to 0")
			compSpec := opsRes.Cluster.Spec.GetComponentByName(defaultCompName)
			Expect(compSpec.Replicas).Should(BeEquivalentTo(2))
			Expect(*compSpec.Instances[0].Replicas).Should(BeEquivalentTo(0))
		})

		It("test online the specified pod of the instance template and auto-sync replicaChanges", func() {
			offlineInstanceName := fmt.Sprintf("%s-%s-%s-0", clusterName, defaultCompName, insTplName)
			offlineInstances := []string{offlineInstanceName}
			opsRes := testHScaleWithSpecifiedPod(func(cluster *appsv1.Cluster) {
				setClusterCompSpec(cluster, []appsv1.InstanceTemplate{
					{Name: insTplName, Replicas: pointer.Int32(1)},
				}, offlineInstances)
			}, opsv1alpha1.HorizontalScaling{
				ScaleOut: &opsv1alpha1.ScaleOut{
					OfflineInstancesToOnline: offlineInstances,
				},
			}, []string{}, func(podList []*corev1.Pod) {
				By("create the specified pod " + offlineInstanceName)
				testapps.MockInstanceSetPod(&testCtx, nil, clusterName, defaultCompName, offlineInstanceName, "follower")
			}, false)
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/1"))
			By("expect replicas to 4")
			compSpec := opsRes.Cluster.Spec.GetComponentByName(defaultCompName)
			Expect(compSpec.Replicas).Should(BeEquivalentTo(4))
			Expect(*compSpec.Instances[0].Replicas).Should(BeEquivalentTo(2))
		})

		It("test online the boundary ordinal the specified pod of the instance template and auto-sync replicaChanges", func() {
			offlineInstanceName := fmt.Sprintf("%s-%s-%s-1", clusterName, defaultCompName, insTplName)
			offlineInstanceName2 := fmt.Sprintf("%s-%s-2", clusterName, defaultCompName)
			offlineInstances := []string{offlineInstanceName, offlineInstanceName2}
			opsRes := testHScaleWithSpecifiedPod(func(cluster *appsv1.Cluster) {
				setClusterCompSpec(cluster, []appsv1.InstanceTemplate{
					{Name: insTplName, Replicas: pointer.Int32(1)},
				}, offlineInstances)
			}, opsv1alpha1.HorizontalScaling{
				ScaleOut: &opsv1alpha1.ScaleOut{
					OfflineInstancesToOnline: offlineInstances,
				},
			}, []string{}, func(podList []*corev1.Pod) {
				By("create the specified pod " + offlineInstanceName)
				testapps.MockInstanceSetPod(&testCtx, nil, clusterName, defaultCompName, offlineInstanceName, "follower")
				testapps.MockInstanceSetPod(&testCtx, nil, clusterName, defaultCompName, offlineInstanceName2, "follower")
			}, false)
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/2"))
			By("expect replicas to 4")
			compSpec := opsRes.Cluster.Spec.GetComponentByName(defaultCompName)
			Expect(compSpec.Replicas).Should(BeEquivalentTo(5))
			Expect(*compSpec.Instances[0].Replicas).Should(BeEquivalentTo(2))
		})

		It("test offline and online the specified pod and auto-sync replicaChanges", func() {
			onlinePodName := fmt.Sprintf("%s-%s-1", clusterName, defaultCompName)
			offlinePodName := fmt.Sprintf("%s-%s-%s-0", clusterName, defaultCompName, insTplName)
			opsRes := testHScaleWithSpecifiedPod(func(cluster *appsv1.Cluster) {
				setClusterCompSpec(cluster, []appsv1.InstanceTemplate{
					{Name: insTplName, Replicas: pointer.Int32(1)},
				}, []string{onlinePodName})
			}, opsv1alpha1.HorizontalScaling{
				ScaleIn: &opsv1alpha1.ScaleIn{
					OnlineInstancesToOffline: []string{offlinePodName},
				},
				ScaleOut: &opsv1alpha1.ScaleOut{
					OfflineInstancesToOnline: []string{onlinePodName},
				},
			}, []string{offlinePodName}, func(podList []*corev1.Pod) {
				By(fmt.Sprintf(`delete the specified pod"%s"`, offlinePodName))
				deletePods(podList[0])

				By(fmt.Sprintf(`create the pod "%s" which is removed from offlineInstances`, onlinePodName))
				createPods("", 1)
			}, false)
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/2"))
			By("expect replicas to 3")
			Expect(opsRes.Cluster.Spec.GetComponentByName(defaultCompName).Replicas).Should(BeEquivalentTo(3))
		})

		It("h-scale new instance templates and scale in all old replicas", func() {
			templateFoo := appsv1.InstanceTemplate{
				Name:     insTplName,
				Replicas: func() *int32 { r := int32(3); return &r }(),
			}
			templateBar := appsv1.InstanceTemplate{
				Name:     "bar",
				Replicas: func() *int32 { r := int32(3); return &r }(),
			}
			instances := []appsv1.InstanceTemplate{templateFoo, templateBar}
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, pods := commonHScaleConsensusCompTest(reqCtx, nil, opsv1alpha1.HorizontalScaling{
				ScaleOut: &opsv1alpha1.ScaleOut{
					NewInstances: []appsv1.InstanceTemplate{templateFoo, templateBar},
				},
				ScaleIn: &opsv1alpha1.ScaleIn{
					ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(3)},
				},
			}, false, false)
			By("verify cluster spec is correct")
			var targetSpec *appsv1.ClusterComponentSpec
			for i := range opsRes.Cluster.Spec.ComponentSpecs {
				spec := &opsRes.Cluster.Spec.ComponentSpecs[i]
				if spec.Name == defaultCompName {
					targetSpec = spec
				}
			}

			// auto-sync replicaChanges of the component to 6
			Expect(targetSpec.Replicas).Should(BeEquivalentTo(6))
			Expect(targetSpec.Instances).Should(HaveLen(2))
			Expect(targetSpec.Instances).Should(Equal(instances))
			By("mock six pods are created")
			createPods(insTplName, 0, 1, 2)
			createPods("bar", 0, 1, 2)
			By("delete three pods")
			deletePods(pods...)
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
		})
		createOpsAndToCreatingPhase := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, horizontalScaling opsv1alpha1.HorizontalScaling, ignoreHscalingStrictValidate bool) *opsv1alpha1.OpsRequest {
			if horizontalScaling.ComponentName == "" {
				horizontalScaling.ComponentName = defaultCompName
			}
			if horizontalScaling.Shards == nil && !hscaleFromBackup(horizontalScaling) {
				_, err := publishHScaleAllocation(ctx, k8sClient, opsRes.Cluster, defaultCompName, &opsRes.Cluster.Spec.ComponentSpecs[0])
				Expect(err).ShouldNot(HaveOccurred())
			}
			opsRes.OpsRequest = createHorizontalScaling(clusterName, horizontalScaling, ignoreHscalingStrictValidate)
			opsRes.OpsRequest.Spec.Force = true
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			mockComponentIsOperating(opsRes.Cluster, appsv1.UpdatingComponentPhase, defaultCompName)

			By("expect for opsRequest phase is Creating after doing action")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("do Action")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			return opsRes.OpsRequest
		}

		It("test offline the specified pod but it is not online with the ignore strict validate", func() {
			By("init operations resources with CLusterDefinition/Hybrid components Cluster/consensus Pods")
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}

			By("offline the specified pod but it is not exist, expect replicas not be changed")
			offlineInsName := fmt.Sprintf("%s-%s-4", clusterName, defaultCompName)
			_ = createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ScaleIn: &opsv1alpha1.ScaleIn{
					OnlineInstancesToOffline: []string{offlineInsName},
				},
			}, true)
			By("expect replicas not be changed")
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase), fmt.Sprintf("info: %v", opsRes.OpsRequest))
			Expect(opsRes.Cluster.Spec.GetComponentByName(defaultCompName).Replicas).Should(BeEquivalentTo(3))
		})

		It("test offline two specified pods with same pod name with ignore validate", func() {
			By("init operations resources with CLusterDefinition/ClusterVersion/Hybrid components Cluster/consensus Pods")
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			testPodName := fmt.Sprintf("%s-%s-1", clusterName, defaultCompName)

			By("offline two pod with same pod name")
			_ = createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ScaleIn: &opsv1alpha1.ScaleIn{
					OnlineInstancesToOffline: []string{testPodName, testPodName},
				},
			}, true)
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))
			Expect(opsRes.Cluster.Spec.GetComponentByName(defaultCompName).Replicas).Should(BeEquivalentTo(2))
			// expect the not exist pod still in opsRequest
			onlineToOfflineInstances := opsRes.OpsRequest.Spec.HorizontalScalingList[0].ScaleIn.OnlineInstancesToOffline
			Expect(onlineToOfflineInstances).Should(Equal([]string{testPodName, testPodName}), fmt.Sprintf("info: %v", opsRes.OpsRequest))
			// expect for opsRequest phase is Succeed after pods has been scaled and component phase is Running
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
		})

		It("test online two specified pods with same pod name with ignore validate", func() {
			By("init operations resources with CLusterDefinition/ClusterVersion/Hybrid components Cluster/consensus Pods")
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			testPodName := fmt.Sprintf("%s-%s-1", clusterName, defaultCompName)

			By("offline two pod with same pod name")
			_ = createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ScaleIn: &opsv1alpha1.ScaleIn{
					OnlineInstancesToOffline: []string{testPodName, testPodName},
				},
			}, true)
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))
			Expect(opsRes.Cluster.Spec.GetComponentByName(defaultCompName).Replicas).Should(BeEquivalentTo(2))
			By("expect the not exist pod still in opsRequest")
			onlineToOfflineInstances := opsRes.OpsRequest.Spec.HorizontalScalingList[0].ScaleIn.OnlineInstancesToOffline
			Expect(onlineToOfflineInstances).Should(Equal([]string{testPodName, testPodName}), fmt.Sprintf("info: %v", opsRes.OpsRequest))
			By("expect for opsRequest phase is Succeed after pods has been scaled and component phase is Running")
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/1"), fmt.Sprintf("info: %v", opsRes.OpsRequest))

		})

		It("test offline and online two pods in the same time with the ignore validate", func() {
			onlinePodName := fmt.Sprintf("%s-%s-1", clusterName, defaultCompName)
			offlinePodName := fmt.Sprintf("%s-%s-%s-0", clusterName, defaultCompName, insTplName)
			opsRes := testHScaleWithSpecifiedPod(func(cluster *appsv1.Cluster) {
				setClusterCompSpec(cluster, []appsv1.InstanceTemplate{
					{Name: insTplName, Replicas: pointer.Int32(1)},
				}, []string{onlinePodName})
			}, opsv1alpha1.HorizontalScaling{
				ScaleIn: &opsv1alpha1.ScaleIn{
					OnlineInstancesToOffline: []string{offlinePodName},
				},
				ScaleOut: &opsv1alpha1.ScaleOut{
					OfflineInstancesToOnline: []string{onlinePodName},
				},
			}, []string{offlinePodName}, func(podList []*corev1.Pod) {
				By(fmt.Sprintf(`delete the specified pod"%s"`, offlinePodName))
				deletePods(podList[0])

				By(fmt.Sprintf(`create the pod "%s" which is removed from offlineInstances`, onlinePodName))
				createPods("", 1)
			}, true)
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/2"))
		})

		It("test run multi horizontalScaling opsRequest with force flag", func() {
			By("init operations resources with CLusterDefinition/Hybrid components Cluster/consensus Pods")
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			By("create first opsRequest to add 1 replicas with `scaleOut` field and expect replicas to 4")
			createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ScaleOut: &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}},
			}, false)
			Expect(opsRes.Cluster.Spec.GetComponentByName(defaultCompName).Replicas).Should(BeEquivalentTo(4))

			By("create secondary opsRequest to add 1 replicas with `replicasToAdd` field and expect replicas to 5")
			createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ScaleOut: &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}},
			}, false)
			Expect(opsRes.Cluster.Spec.GetComponentByName(defaultCompName).Replicas).Should(BeEquivalentTo(5))

			By("a newer request supersedes previous ordinary scaling and can offline an assigned instance")
			offlineInsName := fmt.Sprintf("%s-%s-3", clusterName, defaultCompName)
			_ = createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ScaleIn: &opsv1alpha1.ScaleIn{
					ReplicaChanger:           opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)},
					OnlineInstancesToOffline: []string{offlineInsName},
				},
			}, false)
			Expect(opsRes.Cluster.Spec.GetComponentByName(defaultCompName).Replicas).Should(BeEquivalentTo(4))
			Expect(opsRes.Cluster.Spec.GetComponentByName(defaultCompName).OfflineInstances).Should(ContainElement(offlineInsName))

			By("a subsequent scale-in also supersedes the previous request without name prediction")
			_ = createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ScaleIn: &opsv1alpha1.ScaleIn{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}},
			}, false)
			Expect(opsRes.Cluster.Spec.GetComponentByName(defaultCompName).Replicas).Should(BeEquivalentTo(3))
		})

		It("horizontal scaling for shards component", func() {
			By("init operations resources")
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			// add a sharding component
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(cluster *appsv1.Cluster) {
				cluster.Spec.Shardings = []appsv1.ClusterSharding{
					{
						Name:     secondaryCompName,
						Shards:   int32(3),
						Template: cluster.Spec.ComponentSpecs[0],
					},
				}
			})).Should(Succeed())

			By("add two shards by set shards to 5")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			_ = createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: secondaryCompName},
				Shards:       pointer.Int32(5),
			}, false)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, ops *opsv1alpha1.OpsRequest) {
				g.Expect(*ops.Status.LastConfiguration.Components[secondaryCompName].Shards).Should(BeEquivalentTo(3))
			})).Should(Succeed())

			By("expect component shards to 5")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.Shardings[0].Shards).Should(BeEquivalentTo(5))
			})).Should(Succeed())

			By("mock the new components")
			createComponent := func(compName string) *appsv1.Component {
				comp := testapps.NewComponentFactory(testCtx.DefaultNamespace, opsRes.Cluster.Name+"-"+compName, compDefName).
					AddLabels(constant.AppManagedByLabelKey, constant.AppName).
					AddLabels(constant.AppInstanceLabelKey, opsRes.Cluster.Name).
					AddLabels(constant.KBAppClusterUIDKey, string(opsRes.Cluster.UID)).
					AddLabels(constant.KBAppShardingNameLabelKey, secondaryCompName).
					AddLabels(constant.KBAppComponentLabelKey, compName).
					SetReplicas(3).
					Create(&testCtx).
					GetObject()
				Expect(testapps.ChangeObjStatus(&testCtx, comp, func() {
					comp.Status.Phase = appsv1.CreatingComponentPhase
				})).Should(Succeed())
				return comp
			}
			comp4 := createComponent(secondaryCompName + "-comp4")
			comp5 := createComponent(secondaryCompName + "-comp5")
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, pobj *opsv1alpha1.OpsRequest) {
				g.Expect(pobj.Status.Progress).Should(Equal("0/2"))
				g.Expect(pobj.Status.Components[secondaryCompName].ProgressDetails).Should(HaveLen(2))
			})).Should(Succeed())

			By("expect ops phase to succeed when new components are running")
			// mock components and cluster is running
			Expect(testapps.ChangeObjStatus(&testCtx, comp4, func() {
				comp4.Status.Phase = appsv1.RunningComponentPhase
			})).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, comp5, func() {
				comp5.Status.Phase = appsv1.RunningComponentPhase
			})).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				opsRes.Cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{
					secondaryCompName: {
						Phase: appsv1.RunningComponentPhase,
					},
				}
			})).Should(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, pobj *opsv1alpha1.OpsRequest) {
				g.Expect(pobj.Status.Progress).Should(Equal("2/2"))
				g.Expect(pobj.Status.Phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
			})).Should(Succeed())

			By("delete one shard by set shards to 4")
			reqCtx = intctrlutil.RequestCtx{Ctx: ctx}
			_ = createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: secondaryCompName},
				Shards:       pointer.Int32(4),
			}, false)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, ops *opsv1alpha1.OpsRequest) {
				g.Expect(*ops.Status.LastConfiguration.Components[secondaryCompName].Shards).Should(BeEquivalentTo(5))
			})).Should(Succeed())

			By("expect component shards to 5")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.Shardings[0].Shards).Should(BeEquivalentTo(4))
			})).Should(Succeed())

			By("expect ops phase to succeed when the component is deleted")
			// Create 3 components to mock already existing components.
			createComponent(secondaryCompName + "-comp1")
			createComponent(secondaryCompName + "-comp2")
			createComponent(secondaryCompName + "-comp3")
			testapps.DeleteObject(&testCtx, client.ObjectKeyFromObject(comp5), &appsv1.Component{})
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, pobj *opsv1alpha1.OpsRequest) {
				g.Expect(pobj.Status.Progress).Should(Equal("1/1"))
				g.Expect(pobj.Status.Phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
			})).Should(Succeed())
		})
		It("should fail when scaling out beyond the max replicas", func() {
			By("Setting up the component with a replica limit of 1 to 3")
			opsRes, compDef, _ := initOperationsResources(compDefName, clusterName)
			Expect(testapps.ChangeObj(&testCtx, compDef, func(compDef *appsv1.ComponentDefinition) {
				compDef.Spec.ReplicasLimit = &appsv1.ReplicasLimit{
					MinReplicas: 1,
					MaxReplicas: 3,
				}
			})).Should(Succeed())

			By("Creating a horizontal scaling operation to scale out beyond the limit")
			ops := createHorizontalScaling(clusterName, opsv1alpha1.HorizontalScaling{
				ComponentOps: opsv1alpha1.ComponentOps{
					ComponentName: defaultCompName,
				},
				ScaleOut: &opsv1alpha1.ScaleOut{
					OfflineInstancesToOnline: []string{clusterName + defaultCompName + "-3", clusterName + defaultCompName + "-4"},
				},
			}, false)

			ops.Namespace = testCtx.DefaultNamespace
			initClusterAnnotationAndPhaseForOps(opsRes)

			By("Validating the operation")
			err := ops.Validate(ctx, k8sClient, opsRes.Cluster, true)
			Expect(err).To(HaveOccurred()) // Expect an error due to exceeding the maximum replica limit
		})

		It("should fail when scaling in below the min replicas", func() {
			By("Setting up the component with a replica limit of 1 to 3")
			opsRes, compDef, _ := initOperationsResources(compDefName, clusterName)
			Expect(testapps.ChangeObj(&testCtx, compDef, func(compDef *appsv1.ComponentDefinition) {
				compDef.Spec.ReplicasLimit = &appsv1.ReplicasLimit{
					MinReplicas: 1,
					MaxReplicas: 3,
				}
			})).Should(Succeed())

			By("Creating a horizontal scaling operation to scale in below the limit")
			ops := createHorizontalScaling(clusterName, opsv1alpha1.HorizontalScaling{
				ComponentOps: opsv1alpha1.ComponentOps{
					ComponentName: defaultCompName,
				},
				ScaleIn: &opsv1alpha1.ScaleIn{
					OnlineInstancesToOffline: []string{clusterName + defaultCompName + "-0", clusterName + defaultCompName + "-1", clusterName + defaultCompName + "-2"},
				},
			}, false)

			ops.Namespace = testCtx.DefaultNamespace
			initClusterAnnotationAndPhaseForOps(opsRes)

			By("Validating the operation")
			err := ops.Validate(ctx, k8sClient, opsRes.Cluster, true)
			Expect(err).To(HaveOccurred()) // Expect an error due to scaling in below the minimum replica limit
		})

		It("should succeed when scaling within the replicas limit", func() {
			By("Setting up the component with a replica limit of 1 to 3")
			opsRes, compDef, _ := initOperationsResources(compDefName, clusterName)
			Expect(testapps.ChangeObj(&testCtx, compDef, func(compDef *appsv1.ComponentDefinition) {
				compDef.Spec.ReplicasLimit = &appsv1.ReplicasLimit{
					MinReplicas: 1,
					MaxReplicas: 4,
				}
			})).Should(Succeed())

			By("Creating a horizontal scaling operation to scale within the limit")
			ops := createHorizontalScaling(clusterName, opsv1alpha1.HorizontalScaling{
				ComponentOps: opsv1alpha1.ComponentOps{
					ComponentName: defaultCompName,
				},
				ScaleOut: &opsv1alpha1.ScaleOut{
					OfflineInstancesToOnline: []string{clusterName + defaultCompName + "-3"},
				},
			}, false)

			ops.Namespace = testCtx.DefaultNamespace
			initClusterAnnotationAndPhaseForOps(opsRes)

			By("Validating the operation")
			err := ops.Validate(ctx, k8sClient, opsRes.Cluster, true)
			Expect(err).ToNot(HaveOccurred()) // Should pass as it's within the specified replica limit
		})

		It("should succeed when scaling within the replicas limit with shards", func() {
			By("Setting up the sharding with a shard limit of 1 to 3")
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			shardingDef := testapps.NewShardingDefinitionFactory(shardingDefName, compDefName).Create(&testCtx).GetObject()
			// add a sharding component
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(cluster *appsv1.Cluster) {
				cluster.Spec.Shardings = []appsv1.ClusterSharding{
					{
						Name:        defaultCompName,
						Shards:      int32(3),
						Template:    cluster.Spec.ComponentSpecs[0],
						ShardingDef: shardingDefName,
					},
				}
				cluster.Spec.ComponentSpecs = []appsv1.ClusterComponentSpec{}
			})).Should(Succeed())

			Expect(testapps.ChangeObj(&testCtx, shardingDef, func(shardingDef *appsv1.ShardingDefinition) {
				shardingDef.Spec.ShardsLimit = &appsv1.ShardsLimit{
					MinShards: 1,
					MaxShards: 3,
				}
			})).Should(Succeed())

			By("Testing horizontal scaling operation with different shard values")
			shardValues := []int32{0, 2, 4}               // Shard values to test
			expectedResults := []bool{false, true, false} // Expected results for each shard value: fail, pass, fail

			for i, shards := range shardValues {
				ops := createHorizontalScaling(clusterName, opsv1alpha1.HorizontalScaling{
					Shards: &shards,
					ComponentOps: opsv1alpha1.ComponentOps{
						ComponentName: defaultCompName,
					},
				}, false)

				ops.Namespace = testCtx.DefaultNamespace
				initClusterAnnotationAndPhaseForOps(opsRes)

				By("Validating the operation")
				err := ops.Validate(ctx, k8sClient, opsRes.Cluster, true)

				// Check if the result matches the expected result
				if expectedResults[i] {
					Expect(err).ToNot(HaveOccurred()) // Should pass when shards is 2 (within limit)
				} else {
					Expect(err).To(HaveOccurred()) // Should fail when shards is 4 (beyond limit) and 0 (below limit)
				}
			}

		})

	})
})

// Publish allocation through the real InstanceSet allocator and status producer.
// Deliberately do not create/update Pods: tests control when the workload applies
// the allocation, independently of when identities are published to Operations.
func publishHScaleAllocation(ctx context.Context, cli client.Client, cluster *appsv1.Cluster,
	name string, spec *appsv1.ClusterComponentSpec) (*kubebuilderx.ObjectTree, error) {
	its := &workloads.InstanceSet{}
	key := client.ObjectKey{Namespace: cluster.Namespace, Name: constant.GenerateClusterComponentName(cluster.Name, name)}
	err := cli.Get(ctx, key, its)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	create := apierrors.IsNotFound(err)
	if create {
		its.ObjectMeta = metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace, Labels: constant.GetCompLabels(cluster.Name, name)}
		its.Spec.PodManagementPolicy = k8sappsv1.ParallelPodManagement
		its.Spec.Selector = &metav1.LabelSelector{MatchLabels: its.Labels}
		its.Spec.Template = corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: its.Labels}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8"}}}}
	}
	its.Spec.Replicas, its.Spec.OfflineInstances = pointer.Int32(spec.Replicas), slices.Clone(spec.OfflineInstances)
	its.Spec.FlatInstanceOrdinal, its.Spec.Ordinals = spec.FlatInstanceOrdinal, spec.Ordinals
	its.Spec.Instances = nil
	for _, template := range spec.Instances {
		its.Spec.Instances = append(its.Spec.Instances, workloads.InstanceTemplate{
			Name: template.Name, Replicas: template.Replicas, Ordinals: template.Ordinals, Resources: template.Resources,
			Annotations: template.Annotations, Labels: template.Labels, Env: template.Env,
		})
	}
	if create {
		err = cli.Create(ctx, its)
	} else {
		err = cli.Update(ctx, its)
	}
	if err != nil {
		return nil, err
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	pods := &corev1.PodList{}
	if err := cli.List(ctx, pods, client.InNamespace(key.Namespace), client.MatchingLabels(constant.GetCompLabels(cluster.Name, name))); err != nil {
		return nil, err
	}
	for i := range pods.Items {
		if err := tree.Add(&pods.Items[i]); err != nil {
			return nil, err
		}
	}
	if _, err := instanceset.NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		return nil, err
	}
	if _, err := instanceset.NewStatusReconciler().Reconcile(tree); err != nil {
		return nil, err
	}
	return tree, cli.Status().Update(ctx, its)
}

func TestHScaleAllocatedIdentities(t *testing.T) {
	for _, flat := range []bool{false, true} {
		t.Run(fmt.Sprintf("flat=%t", flat), func(t *testing.T) {
			f := newHorizontalScalingFixture(t, scaleOutRequest("db", false))
			spec := &f.res.Cluster.Spec.ComponentSpecs[0]
			spec.FlatInstanceOrdinal = flat
			spec.Ordinals.Discrete = []int32{3, 7}
			spec.OfflineInstances = []string{"demo-db-99"} // unrelated retained identity, with no observed template
			tree := f.publishAllocation(t, "db")
			if _, err := instanceset.NewReplicasAlignmentReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			for _, obj := range tree.List(&corev1.Pod{}) {
				pod := obj.(*corev1.Pod)
				if pod.Name != "demo-db-3" {
					t.Fatalf("initial allocation = %s", pod.Name)
				}
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
				if err := f.cli.Create(f.req.Ctx, pod); err != nil {
					t.Fatal(err)
				}
			}
			f.publishAllocation(t, "db")
			f.saveConfiguration(t)
			if len(f.res.OpsRequest.Status.LastConfiguration.Components["db"].SourceInstanceAssignments) != 1 {
				t.Fatal("unrelated Offline identity entered source assignments")
			}
			trace := &horizontalScalingRuntimeTrace{OpsRuntime: f.res.Runtimes["db"], failAt: 1, err: fmt.Errorf("unexpected name planning")}
			f.res.Runtimes["db"] = trace
			if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			tree = f.publishAllocation(t, "db")
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			if got := f.res.OpsRequest.Status.Progress; got != "0/1" {
				t.Fatalf("progress = %s", got)
			}
			details := f.res.OpsRequest.Status.Components["db"].ProgressDetails
			if len(details) != 1 || details[0].ObjectKey != "Pod/demo-db-7" {
				t.Fatalf("participants = %+v", details)
			}
			if _, err := instanceset.NewReplicasAlignmentReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			for _, obj := range tree.List(&corev1.Pod{}) {
				pod := obj.(*corev1.Pod)
				if pod.Name != "demo-db-7" {
					continue
				}
				pod.Status.Phase = corev1.PodPending
				if err := f.cli.Create(f.req.Ctx, pod); err != nil {
					t.Fatal(err)
				}
			}
			f.publishAllocation(t, "db")
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			pod := &corev1.Pod{}
			if err := f.cli.Get(f.req.Ctx, client.ObjectKey{Namespace: "default", Name: "demo-db-7"}, pod); err != nil {
				t.Fatal(err)
			}
			pod.Status.Phase = corev1.PodRunning
			pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			if err := f.cli.Status().Update(f.req.Ctx, pod); err != nil {
				t.Fatal(err)
			}
			f.publishAllocation(t, "db")
			f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
			if len(trace.specs) != 0 {
				t.Fatal("ordinary forward path called name planner")
			}
			// Scale back in. Assignment removal must not bypass actual Pod deletion.
			f.res.OpsRequest.Spec.HorizontalScalingList[0].ScaleOut = nil
			f.res.OpsRequest.Spec.HorizontalScalingList[0].ScaleIn = &opsv1alpha1.ScaleIn{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}
			if err := f.cli.Update(f.req.Ctx, f.res.OpsRequest); err != nil {
				t.Fatal(err)
			}
			f.res.OpsRequest.Status.Components = nil
			f.saveConfiguration(t)
			if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			f.publishAllocation(t, "db")
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			if err := f.cli.Delete(f.req.Ctx, pod); err != nil {
				t.Fatal(err)
			}
			f.publishAllocation(t, "db")
			f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
			if got := f.res.OpsRequest.Status.Progress; got != "1/1" {
				t.Fatalf("scale-in progress = %s", got)
			}
		})
	}
}

func TestHScaleNonFlatCancellationOrdinals(t *testing.T) {
	for _, scaleIn := range []bool{false, true} {
		f := newHorizontalScalingFixture(t, scaleOutRequest("db", false))
		spec := &f.res.Cluster.Spec.ComponentSpecs[0]
		spec.Ordinals.Discrete = []int32{3, 7}
		if scaleIn {
			spec.Replicas = 2
			f.res.OpsRequest.Spec.HorizontalScalingList[0].ScaleOut = nil
			f.res.OpsRequest.Spec.HorizontalScalingList[0].ScaleIn = &opsv1alpha1.ScaleIn{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}
			if err := f.cli.Update(f.req.Ctx, f.res.OpsRequest); err != nil {
				t.Fatal(err)
			}
		}
		f.publishAllocation(t, "db")
		f.saveConfiguration(t)
		hs := horizontalScalingOpsHandler{}
		if err := hs.Action(f.req, f.cli, f.res); err != nil {
			t.Fatal(err)
		}
		if err := hs.Cancel(f.req, f.cli, f.res); err != nil {
			t.Fatal(err)
		}
		f.res.OpsRequest.Status.Phase = opsv1alpha1.OpsCancellingPhase
		// No forward progress or target status was published before cancellation.
		if scaleIn {
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
		} else {
			f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
		}
		details := f.res.OpsRequest.Status.Components["db"].ProgressDetails
		if len(details) != 1 || details[0].ObjectKey != "Pod/demo-db-7" {
			t.Fatalf("rollback names = %+v", details)
		}
		if !slices.Equal(spec.Ordinals.Discrete, []int32{3, 7}) {
			t.Fatal("rollback changed default ordinals")
		}
	}
}

func TestHScaleShardOverrides(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("group", false))
	common := f.res.Cluster.Spec.ComponentSpecs[0]
	f.res.Cluster.Spec.ComponentSpecs = nil
	f.res.Cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "group", Shards: 2, Template: common,
		ShardTemplates: []appsv1.ShardTemplate{{Name: "fixed", Replicas: pointer.Int32(2), FlatInstanceOrdinal: pointer.Bool(true)}}}}
	for _, name := range []string{"group-a", "group-b"} {
		labels := constant.GetCompLabels("demo", name, map[string]string{constant.KBAppShardingNameLabelKey: "group"})
		if name == "group-b" {
			labels[constant.KBAppShardTemplateLabelKey] = "fixed"
		}
		comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "demo-" + name, Namespace: "default", Labels: labels}}
		if err := f.cli.Create(f.req.Ctx, comp); err != nil {
			t.Fatal(err)
		}
	}
	specs, err := hscaleComponentSpecs(f.req, f.cli, f.res, "group", nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, spec := range specs {
		if _, err := publishHScaleAllocation(f.req.Ctx, f.cli, f.res.Cluster, name, spec); err != nil {
			t.Fatal(err)
		}
	}
	f.saveConfiguration(t)
	if got := len(f.res.OpsRequest.Status.LastConfiguration.Components["group"].SourceInstanceAssignments); got != 3 {
		t.Fatalf("source count = %d", got)
	}
	if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	specs, err = hscaleComponentSpecs(f.req, f.cli, f.res, "group", nil)
	if err != nil {
		t.Fatal(err)
	}
	if specs["group-a"].Replicas != 2 || specs["group-b"].Replicas != 2 || !specs["group-b"].FlatInstanceOrdinal {
		t.Fatal("lost shard override")
	}
	for name, spec := range specs {
		if _, err := publishHScaleAllocation(f.req.Ctx, f.cli, f.res.Cluster, name, spec); err != nil {
			t.Fatal(err)
		}
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	if f.res.OpsRequest.Status.Progress != "0/1" {
		t.Fatalf("progress = %s", f.res.OpsRequest.Status.Progress)
	}
	writes := f.clusterWrites
	if err := (horizontalScalingOpsHandler{}).Cancel(f.req, f.cli, f.res); !intctrlutil.IsTargetError(err, intctrlutil.ErrorIgnoreCancel) {
		t.Fatalf("flat shard cancel = %v", err)
	}
	if f.clusterWrites != writes {
		t.Fatal("partially rolled back mixed-ordinal sharding")
	}
}

func TestHScaleFlatRestrictions(t *testing.T) {
	for _, backup := range []bool{false, true} {
		f := newHorizontalScalingFixture(t, scaleOutRequest("db", backup))
		f.res.Cluster.Spec.ComponentSpecs[0].FlatInstanceOrdinal = true
		hs := horizontalScalingOpsHandler{}
		if backup {
			if err := hs.Action(f.req, f.cli, f.res); !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
				t.Fatalf("backup rejection = %v", err)
			}
		} else {
			f.publishAllocation(t, "db")
			f.saveConfiguration(t)
			if err := hs.Action(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			f.clusterWrites = 0
			if err := hs.Cancel(f.req, f.cli, f.res); !intctrlutil.IsTargetError(err, intctrlutil.ErrorIgnoreCancel) {
				t.Fatalf("cancel rejection = %v", err)
			}
		}
		if f.clusterWrites != 0 || f.backupReads != 0 || f.restoreReads != 0 {
			t.Fatal("unsupported request mutated the cluster or entered restoration")
		}
	}
}

func TestHScaleTemplateReassignment(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", false))
	spec := &f.res.Cluster.Spec.ComponentSpecs[0]
	spec.FlatInstanceOrdinal, spec.Replicas = true, 2
	tree := f.publishAllocation(t, "db")
	if _, err := instanceset.NewReplicasAlignmentReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	for _, obj := range tree.List(&corev1.Pod{}) {
		pod := obj.(*corev1.Pod)
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		if err := f.cli.Create(f.req.Ctx, pod); err != nil {
			t.Fatal(err)
		}
	}
	f.publishAllocation(t, "db")
	request := &f.res.OpsRequest.Spec.HorizontalScalingList[0]
	request.ScaleIn = &opsv1alpha1.ScaleIn{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}
	request.ScaleOut = &opsv1alpha1.ScaleOut{NewInstances: []appsv1.InstanceTemplate{{Name: "reader", Replicas: pointer.Int32(1),
		Ordinals: appsv1.Ordinals{Discrete: []int32{1}}, Env: []corev1.EnvVar{{Name: "READER", Value: "true"}}}}}
	if err := f.cli.Update(f.req.Ctx, f.res.OpsRequest); err != nil {
		t.Fatal(err)
	}
	f.saveConfiguration(t)
	if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	tree = f.publishAllocation(t, "db")
	its := tree.GetRoot().(*workloads.InstanceSet)
	for _, status := range its.Status.InstanceStatus {
		if status.PodName == "demo-db-1" && (status.TemplateName == nil || *status.TemplateName != "reader" || status.UpToDate) {
			t.Fatalf("unexpected transition observation: %+v", status)
		}
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	if got := f.res.OpsRequest.Status.Progress; got != "0/1" {
		t.Fatalf("healthy but unapplied Pod completed reassignment: %s", got)
	}
	// Re-create from the real desired template to apply the reassignment.
	pod := &corev1.Pod{}
	key := client.ObjectKey{Namespace: "default", Name: "demo-db-1"}
	if err := f.cli.Get(f.req.Ctx, key, pod); err != nil {
		t.Fatal(err)
	}
	if err := f.cli.Delete(f.req.Ctx, pod); err != nil {
		t.Fatal(err)
	}
	tree = f.publishAllocation(t, "db")
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	if _, err := instanceset.NewReplicasAlignmentReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	for _, obj := range tree.List(&corev1.Pod{}) {
		if obj.GetName() == key.Name {
			pod = obj.(*corev1.Pod)
			pod.Status.Phase = corev1.PodPending
			if err := f.cli.Create(f.req.Ctx, pod); err != nil {
				t.Fatal(err)
			}
		}
	}
	f.publishAllocation(t, "db")
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	if err := f.cli.Get(f.req.Ctx, key, pod); err != nil {
		t.Fatal(err)
	}
	pod.Status.Phase = corev1.PodRunning
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
	if err := f.cli.Status().Update(f.req.Ctx, pod); err != nil {
		t.Fatal(err)
	}
	f.publishAllocation(t, "db")
	f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
	if got := f.res.OpsRequest.Status.Progress; got != "1/1" {
		t.Fatalf("final progress = %s", got)
	}
}

func createHorizontalScaling(clusterName string, horizontalScaling opsv1alpha1.HorizontalScaling, ifIgnore bool) *opsv1alpha1.OpsRequest {
	horizontalOpsName := "horizontal-scaling-ops-" + testCtx.GetRandomStr()
	var ignoreStrictValidation string
	if ifIgnore {
		ignoreStrictValidation = "true"
	} else {
		ignoreStrictValidation = "false"
	}
	ops := testops.NewOpsRequestObj(horizontalOpsName, testCtx.DefaultNamespace,
		clusterName, opsv1alpha1.HorizontalScalingType)
	ops.Spec.HorizontalScalingList = []opsv1alpha1.HorizontalScaling{
		horizontalScaling,
	}
	ops.Annotations = map[string]string{}
	ops.Annotations[constant.IgnoreHscaleValidateAnnoKey] = ignoreStrictValidation
	opsRequest := testops.CreateOpsRequest(ctx, testCtx, ops)
	opsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
	return opsRequest
}

func TestHorizontalScalingCreateRestorePreservesPerPodStartingIndex(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, addToScheme := range []func(*runtime.Scheme) error{
		corev1.AddToScheme,
		appsv1.AddToScheme,
		dpv1alpha1.AddToScheme,
		opsv1alpha1.AddToScheme,
	} {
		if err := addToScheme(scheme); err != nil {
			t.Fatalf("add scheme: %v", err)
		}
	}

	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "default", UID: types.UID("cluster1")},
	}
	opsRequest := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "scale-out-from-backup", Namespace: "default", UID: types.UID("opsreq1")},
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-backup", Namespace: "default"},
		Status: dpv1alpha1.BackupStatus{
			Phase: dpv1alpha1.BackupPhaseCompleted,
			BackupMethod: &dpv1alpha1.BackupMethod{
				Name:          "snapshot",
				TargetVolumes: &dpv1alpha1.TargetVolumeInfo{Volumes: []string{"data"}},
			},
			Targets: []dpv1alpha1.BackupStatusTarget{{
				BackupTarget: dpv1alpha1.BackupTarget{
					Name:        "redis",
					PodSelector: &dpv1alpha1.PodSelector{Strategy: dpv1alpha1.PodSelectionStrategyAll},
				},
				SelectedTargetPods: []string{"redis-az-a-4", "redis-az-a-3"},
			}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, opsRequest, backup).Build()
	opsRes := &OpsResource{Cluster: cluster, OpsRequest: opsRequest}
	synthesizedComponent := &component.SynthesizedComponent{
		Name: "redis",
		VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
		}},
	}
	componentSpec := &appsv1.ClusterComponentSpec{
		Name: "redis",
		Instances: []appsv1.InstanceTemplate{{
			Name: "az-a",
			Ordinals: appsv1.Ordinals{
				Ranges: []appsv1.Range{{Start: 3, End: 4}},
			},
		}},
	}
	for _, ordinal := range []int32{3, 4} {
		restoreMGR := plan.NewRestoreManager(ctx, cli, cluster, scheme, map[string]string{
			constant.OpsRequestNameLabelKey: opsRequest.Name,
		}, 1, ordinal)
		err := horizontalScalingOpsHandler{}.createRestore(
			intctrlutil.RequestCtx{Ctx: ctx, Recorder: record.NewFakeRecorder(1)},
			cli, opsRes, synthesizedComponent, restoreMGR, componentSpec, backup, "az-a")
		if err != nil {
			t.Fatalf("create restore for ordinal %d: %v", ordinal, err)
		}
	}

	restoreList := &dpv1alpha1.RestoreList{}
	if err := cli.List(ctx, restoreList, client.InNamespace("default")); err != nil {
		t.Fatalf("list restores: %v", err)
	}
	if len(restoreList.Items) != 2 {
		t.Fatalf("expected two restores, got %d", len(restoreList.Items))
	}
	restoresByOrdinal := map[int32]dpv1alpha1.Restore{}
	for i := range restoreList.Items {
		restore := restoreList.Items[i]
		startingIndex := restore.Spec.PrepareDataConfig.RestoreVolumeClaimsTemplate.StartingIndex
		restoresByOrdinal[startingIndex] = restore
	}
	for _, ordinal := range []int32{3, 4} {
		restore, ok := restoresByOrdinal[ordinal]
		if !ok {
			t.Fatalf("missing restore for actual scale-out pod ordinal %d", ordinal)
		}
		claimTemplate := restore.Spec.PrepareDataConfig.RestoreVolumeClaimsTemplate
		if claimTemplate.Replicas != 1 {
			t.Fatalf("restore replicas = %d, want 1 for ordinal %d", claimTemplate.Replicas, ordinal)
		}
		if got := claimTemplate.Templates[0].Labels[constant.KBAppInstanceTemplateLabelKey]; got != "az-a" {
			t.Fatalf("instance template label = %q, want %q for ordinal %d", got, "az-a", ordinal)
		}
	}
}

func cancelOpsRequest(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, cancelTime time.Time) {
	opsRequest := opsRes.OpsRequest
	opsRequest.Spec.Cancel = true
	opsBehaviour := GetOpsManager().OpsMap[opsRequest.Spec.Type]
	Expect(testapps.ChangeObjStatus(&testCtx, opsRequest, func() {
		opsRequest.Status.CancelTimestamp = metav1.Time{Time: cancelTime}
		opsRequest.Status.Phase = opsv1alpha1.OpsCancellingPhase
	})).Should(Succeed())
	Expect(opsBehaviour.CancelFunc(reqCtx, k8sClient, opsRes)).ShouldNot(HaveOccurred())
}

func mockConsensusCompToRunning(opsRes *OpsResource) {
	// mock consensus component is Running
	compStatus := opsRes.Cluster.Status.Components[defaultCompName]
	compStatus.Phase = appsv1.RunningComponentPhase
	opsRes.Cluster.Status.Components[defaultCompName] = compStatus
}

func TestHorizontalScalingCreateRestoreReturnsFatalWhenNoRestoreBuilt(t *testing.T) {
	testCases := []struct {
		name         string
		backupMethod *dpv1alpha1.BackupMethod
		expectError  string
	}{
		{
			name:         "completed backup without backup method",
			backupMethod: nil,
			expectError:  "status.backupMethod",
		}, {
			// logical backups (e.g. TiDB BR) have no targetVolumes at all
			name:         "backup method without target volumes",
			backupMethod: &dpv1alpha1.BackupMethod{Name: "br"},
			expectError:  "has no target volumes matching component",
		}, {
			// targetVolumes exist but none match the component's volume claim templates
			name: "backup method target volumes match no component volume",
			backupMethod: &dpv1alpha1.BackupMethod{
				Name: "br",
				TargetVolumes: &dpv1alpha1.TargetVolumeInfo{
					Volumes: []string{"other-data"},
				},
			},
			expectError: "has no target volumes matching component",
		}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, addToScheme := range []func(*runtime.Scheme) error{
				corev1.AddToScheme,
				appsv1.AddToScheme,
				dpv1alpha1.AddToScheme,
				opsv1alpha1.AddToScheme,
			} {
				if err := addToScheme(scheme); err != nil {
					t.Fatalf("add scheme: %v", err)
				}
			}

			cluster := &appsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tidb",
					Namespace: "default",
					UID:       types.UID("cluster1"),
				},
			}
			opsRequest := &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "scale-out-from-backup",
					Namespace: "default",
					UID:       types.UID("opsreq1"),
				},
			}
			backup := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "br-full",
					Namespace: "default",
				},
				Status: dpv1alpha1.BackupStatus{
					Phase:        dpv1alpha1.BackupPhaseCompleted,
					BackupMethod: tc.backupMethod,
				},
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, opsRequest, backup).Build()
			opsRes := &OpsResource{
				Cluster:    cluster,
				OpsRequest: opsRequest,
			}
			synthesizedComponent := &component.SynthesizedComponent{
				Name: "tikv",
				VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{
					ObjectMeta: metav1.ObjectMeta{Name: "data"},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					},
				}},
			}
			restoreMGR := plan.NewRestoreManager(ctx, cli, cluster, scheme, map[string]string{
				constant.OpsRequestNameLabelKey: opsRequest.Name,
			}, 1, 3)

			err := horizontalScalingOpsHandler{}.createRestore(intctrlutil.RequestCtx{Ctx: ctx}, cli, opsRes,
				synthesizedComponent, restoreMGR, &appsv1.ClusterComponentSpec{Name: "tikv"}, backup, "")
			if err == nil {
				t.Fatal("expected fatal error when backup method cannot build prepareData restore")
			}
			if !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
				t.Fatalf("expected fatal error, got %T: %v", err, err)
			}
			if !strings.Contains(err.Error(), tc.expectError) {
				t.Fatalf("unexpected error: %v", err)
			}

			restoreList := &dpv1alpha1.RestoreList{}
			if err := cli.List(ctx, restoreList, client.InNamespace("default")); err != nil {
				t.Fatalf("list restores: %v", err)
			}
			if len(restoreList.Items) != 0 {
				t.Fatalf("expected no restore to be created, got %d", len(restoreList.Items))
			}
		})
	}
}
