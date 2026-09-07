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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
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
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
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
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			return opsRes, pods
		}

		checkOpsRequestPhaseIsSucceed := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource) {
			By("expect for opsRequest phase is Succeed after pods has been scaled and component phase is Running")
			// mock consensus component is Running
			mockConsensusCompToRunning(opsRes)
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
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
			if horizontalScaling.Shards == nil {
				testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			}
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
			opsRes.OpsRequest = createHorizontalScaling(clusterName, horizontalScaling, ignoreHscalingStrictValidate)
			opsRes.OpsRequest.Spec.Force = true
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			mockComponentIsOperating(opsRes.Cluster, appsv1.UpdatingComponentPhase, defaultCompName)
			if horizontalScaling.Shards == nil {
				testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			}

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

		It("a new horizontalScaling request supersedes an earlier request for the same component", func() {
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

			By("create third opsRequest to offline an instance allocated by the current InstanceSet")
			offlineInsName := fmt.Sprintf("%s-%s-3", clusterName, defaultCompName)
			_ = createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ScaleIn: &opsv1alpha1.ScaleIn{
					ReplicaChanger:           opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)},
					OnlineInstancesToOffline: []string{offlineInsName},
				},
			}, false)
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("create a fourth request and let it supersede the third request")
			_ = createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ScaleIn: &opsv1alpha1.ScaleIn{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}},
			}, false)
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))
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

func TestHScaleRejectsFlatFromBackupBeforeMutation(t *testing.T) {
	cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{
		{Name: "ordinary", Replicas: 1}, {Name: "flat", Replicas: 1, FlatInstanceOrdinal: true},
	}}}
	ops := &opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{HorizontalScalingList: []opsv1alpha1.HorizontalScaling{
		{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "ordinary"}, ScaleOut: &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}},
		{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "flat"}, ScaleOut: &opsv1alpha1.ScaleOut{FromBackup: &opsv1alpha1.FromBackup{Name: "backup"}}},
	}}}}
	before := cluster.DeepCopy()
	// A nil client also guarantees validation precedes updates to earlier Ops.
	err := (horizontalScalingOpsHandler{}).Action(intctrlutil.RequestCtx{Ctx: context.Background()}, nil, &OpsResource{Cluster: cluster, OpsRequest: ops})
	if !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
		t.Fatalf("want Fatal, got %v", err)
	}
	if !reflect.DeepEqual(before, cluster) {
		t.Fatal("unsupported request mutated cluster")
	}
}

func TestHScaleRollbackParticipants(t *testing.T) {
	source := map[string]string{"demo-db-7": "", "demo-db-42": "large"}
	details := []opsv1alpha1.ProgressStatusDetail{
		{Group: "db/Create", ObjectKey: "Pod/demo-db-99"},
		{Group: "db/Delete", ObjectKey: "Pod/demo-db-42"},
		{Group: "other/Create", ObjectKey: "Pod/other-0"},
		{Group: "db/OtherAction", ObjectKey: "Pod/demo-db-100"},
		{Group: "db/Create", ObjectKey: "PVC/data-demo-db-99"},
	}
	created, deleted := rollbackInstanceSets(source, details, "db")
	if !reflect.DeepEqual(created, source) ||
		!reflect.DeepEqual(deleted, map[string]string{"demo-db-99": ""}) {
		t.Fatalf("rollback must check source health and only recorded deletions: created=%v deleted=%v", created, deleted)
	}
	// With no recorded participants, current runtime objects must not be
	// guessed to be operation-created instances. Source health is still checked.
	created, deleted = rollbackInstanceSets(source, nil, "db")
	if !reflect.DeepEqual(created, source) || len(deleted) != 0 {
		t.Fatalf("empty progress lost source health or invented deletions: created=%v deleted=%v", created, deleted)
	}
	delete(created, "demo-db-7")
	if len(source) != 2 {
		t.Fatal("rollback mutated the saved source")
	}
}

func TestHorizontalDiff(t *testing.T) {
	source := map[string]string{"demo-0": "", "demo-1": "big"}
	target := map[string]string{"demo-0": "", "demo-2": "big"}
	created, deleted := diffAssignments(source, target)
	if len(created) != 1 || created["demo-2"] != "big" || len(deleted) != 1 || deleted["demo-1"] != "big" {
		t.Fatalf("unexpected assignment diff: created=%#v deleted=%#v", created, deleted)
	}
	horizontalScaling := opsv1alpha1.HorizontalScaling{
		ScaleOut: &opsv1alpha1.ScaleOut{OfflineInstancesToOnline: []string{"demo-2"}},
		ScaleIn:  &opsv1alpha1.ScaleIn{OnlineInstancesToOffline: []string{"demo-1"}},
	}
	if !horizontalDiffMatchesOperation(horizontalScaling, created, deleted) {
		t.Fatal("expected explicit online/offline transition to match diff")
	}
	horizontalScaling.ScaleOut.OfflineInstancesToOnline = []string{"demo-3"}
	if horizontalDiffMatchesOperation(horizontalScaling, created, deleted) {
		t.Fatal("stale allocation must not satisfy an explicit identity transition")
	}
	if horizontalDiffMatchesOperation(opsv1alpha1.HorizontalScaling{ScaleOut: &opsv1alpha1.ScaleOut{}}, created, deleted) {
		t.Fatal("scale-out-only operation must not accept an unexpected deletion")
	}
}

// Exercise the real Operations runtime and cancellation handler, without a
// forward ReconcileAction publishing progress first. Desired allocation alone
// must neither complete rollback nor make unrelated objects participants.
func TestNonFlatHScaleCancellation(t *testing.T) {
	for _, operation := range []string{"scale-out", "scale-in", "named-swap", "online-high-ordinal", "from-backup"} {
		for _, unrelatedReleased := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/unrelated-released=%t", operation, unrelatedReleased), func(t *testing.T) {
				ctx := context.Background()
				scheme := runtime.NewScheme()
				for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, workloads.AddToScheme, opsv1alpha1.AddToScheme, dpv1alpha1.AddToScheme} {
					if err := add(scheme); err != nil {
						t.Fatal(err)
					}
				}
				comp := appsv1.ClusterComponentSpec{Name: "db", ComponentDef: "database", Replicas: 2}
				source := map[string]string{"demo-db-0": "", "demo-db-1": ""}
				scaling := opsv1alpha1.HorizontalScaling{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}}
				var restored, removed []string
				offline := map[string]string{}
				switch operation {
				case "scale-out", "from-backup":
					scaling.ScaleOut = &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}
					removed = []string{"demo-db-2"}
					if operation == "from-backup" {
						// No Backup exists in the client: cancellation must not invoke
						// restore or depend on its state.
						scaling.ScaleOut.FromBackup = &opsv1alpha1.FromBackup{Name: "not-needed-for-cancel"}
					}
				case "scale-in":
					scaling.ScaleIn = &opsv1alpha1.ScaleIn{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}
					restored = []string{"demo-db-1"}
				case "named-swap":
					source = map[string]string{"demo-db-0": "", "demo-db-large-0": "large"}
					comp.Instances = []appsv1.InstanceTemplate{{Name: "large", Replicas: pointer.Int32(1)}}
					comp.OfflineInstances = []string{"demo-db-large-1"}
					offline["demo-db-large-1"] = "large"
					scaling.ScaleIn = &opsv1alpha1.ScaleIn{OnlineInstancesToOffline: []string{"demo-db-large-0"}}
					scaling.ScaleOut = &opsv1alpha1.ScaleOut{OfflineInstancesToOnline: []string{"demo-db-large-1"}}
					restored, removed = []string{"demo-db-large-0"}, []string{"demo-db-large-1"}
				case "online-high-ordinal":
					comp.Replicas = 1
					source = map[string]string{"demo-db-0": ""}
					comp.OfflineInstances = []string{"demo-db-5"}
					offline["demo-db-5"] = ""
					scaling.ScaleOut = &opsv1alpha1.ScaleOut{OfflineInstancesToOnline: []string{"demo-db-5"}}
					// Action adds one replica. The non-flat allocator selects 1,
					// not 5; rollback must reverse the actual forward configuration.
					removed = []string{"demo-db-1"}
				}
				cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
					Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{comp}},
					Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase,
						Components: map[string]appsv1.ClusterComponentStatus{"db": {Phase: appsv1.RunningComponentPhase}}}}
				ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "cancel", Namespace: "default"},
					Spec:   opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{HorizontalScalingList: []opsv1alpha1.HorizontalScaling{scaling}}},
					Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase}}
				its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"},
					Status: workloads.InstanceSetStatus{CurrentRevisions: map[string]string{"demo-db-99": "r"}}}
				for name, template := range source {
					its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
						PodName: name, TemplateName: templateName(template), DesiredState: workloads.InstanceDesiredStateActive})
					its.Status.CurrentRevisions[name] = "r"
				}
				for name, template := range offline {
					its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
						PodName: name, TemplateName: templateName(template), DesiredState: workloads.InstanceDesiredStateOffline})
				}
				its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
					PodName: "demo-db-99", DesiredState: workloads.InstanceDesiredStateReleased, CurrentState: workloads.InstanceCurrentStateTerminating})
				if !unrelatedReleased {
					delete(its.Status.CurrentRevisions, "demo-db-99")
					its.Status.InstanceStatus = its.Status.InstanceStatus[:len(its.Status.InstanceStatus)-1]
				}
				cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops, its).WithObjects(cluster, ops, its,
					&appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "database"}}).Build()
				res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(100),
					Runtimes: map[string]OpsRuntime{"db": newOpsRuntime(ctx, cli, "")}}
				h, req := horizontalScalingOpsHandler{}, intctrlutil.RequestCtx{Ctx: ctx}
				if err := h.SaveLastConfiguration(req, cli, res); err != nil {
					t.Fatal(err)
				}
				if err := cli.Status().Update(ctx, ops); err != nil {
					t.Fatal(err)
				}
				if err := h.Action(req, cli, res); err != nil {
					t.Fatal(err)
				}
				if len(ops.Status.Components) != 0 {
					t.Fatal("test requires cancellation without forward progress records")
				}
				if err := h.Cancel(req, cli, res); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(cluster.Spec.ComponentSpecs[0], comp) {
					t.Fatal("cancel did not restore the original component configuration")
				}
				ops.Status.Phase = opsv1alpha1.OpsCancellingPhase
				if err := cli.Status().Update(ctx, ops); err != nil {
					t.Fatal(err)
				}
				for _, name := range removed {
					its.Status.CurrentRevisions[name] = "r"
				}
				conditions := func(kind workloads.ConditionType, names []string) {
					t.Helper()
					message, err := json.Marshal(names)
					if err != nil {
						t.Fatal(err)
					}
					its.Status.Conditions = []metav1.Condition{{Type: string(kind), Status: metav1.ConditionFalse,
						Message: string(message)}}
				}
				check := func(want opsv1alpha1.OpsPhase) {
					t.Helper()
					if err := cli.Status().Update(ctx, its); err != nil {
						t.Fatal(err)
					}
					phase, _, err := h.ReconcileAction(req, cli, res)
					if err != nil || phase != want {
						t.Fatalf("phase=%s, want=%s, err=%v, progress=%s", phase, want, err, ops.Status.Progress)
					}
					for _, detail := range ops.Status.Components["db"].ProgressDetails {
						if detail.ObjectKey == "Pod/demo-db-99" || detail.ObjectKey == "Pod/demo-db-0" {
							t.Fatalf("unrelated instance became a cancellation participant: %#v", detail)
						}
					}
				}
				// Restored names already have revisions, but may still be unready.
				conditions(workloads.InstanceReady, restored)
				check(opsv1alpha1.OpsRunningPhase)
				for _, name := range removed {
					delete(its.Status.CurrentRevisions, name)
				}
				if len(restored) > 0 {
					check(opsv1alpha1.OpsRunningPhase)
					conditions(workloads.InstanceAvailable, restored)
					check(opsv1alpha1.OpsRunningPhase)
					conditions(workloads.InstanceFailure, restored)
					check(opsv1alpha1.OpsFailedPhase)
				}
				// Unchanged source 0 is also unready: as before this PR, only the
				// affected identities control cancellation completion.
				conditions(workloads.InstanceReady, []string{"demo-db-0", "demo-db-99"})
				check(opsv1alpha1.OpsSucceedPhase)
				if ops.Status.Progress != fmt.Sprintf("%d/%d", len(restored)+len(removed), len(restored)+len(removed)) {
					t.Fatalf("unexpected cancellation progress: %s", ops.Status.Progress)
				}
			})
		}
	}
}

func TestHScaleInstanceStatusLifecycle(t *testing.T) {
	for _, flat := range []bool{false, true} {
		for _, operation := range []string{"scale-out", "scale-in", "offline-online", "cancel-out", "cancel-in"} {
			t.Run(fmt.Sprintf("%s/flat=%v", operation, flat), func(t *testing.T) {
				ctx := context.Background()
				scheme := runtime.NewScheme()
				for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, workloads.AddToScheme, opsv1alpha1.AddToScheme} {
					if err := add(scheme); err != nil {
						t.Fatal(err)
					}
				}
				const componentName = "db"
				defaultName, largeName, offlineName := "demo-db-7", "demo-db-42", "demo-db-8"
				newDefaultName, laterDefaultName := "demo-db-99", "demo-db-98"
				if !flat {
					// Non-flat names must obey the template naming rule. Flat
					// allocations deliberately retain non-contiguous ordinals.
					defaultName, largeName, offlineName = "demo-db-0", "demo-db-large-0", "demo-db-large-1"
					newDefaultName, laterDefaultName = "demo-db-1", "demo-db-2"
				}
				source := []workloads.InstanceStatus{
					{PodName: defaultName, TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive},
					{PodName: largeName, TemplateName: templateName("large"), DesiredState: workloads.InstanceDesiredStateActive},
					{PodName: offlineName, TemplateName: templateName("large"), DesiredState: workloads.InstanceDesiredStateOffline},
					{PodName: "demo-db-9", DesiredState: workloads.InstanceDesiredStateOffline},
				}
				comp := appsv1.ClusterComponentSpec{Name: componentName, ComponentDef: "database", Replicas: 2,
					FlatInstanceOrdinal: flat, Instances: []appsv1.InstanceTemplate{{Name: "large", Replicas: pointer.Int32(1)}},
					OfflineInstances: []string{offlineName, "demo-db-9"}}
				cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
					Spec:   appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{comp}},
					Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase, Components: map[string]appsv1.ClusterComponentStatus{componentName: {Phase: appsv1.RunningComponentPhase}}}}
				hs := opsv1alpha1.HorizontalScaling{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: componentName}}
				switch operation {
				case "scale-out", "cancel-out":
					hs.ScaleOut = &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}
				case "scale-in", "cancel-in":
					hs.ScaleIn = &opsv1alpha1.ScaleIn{ReplicaChanger: opsv1alpha1.ReplicaChanger{Instances: []opsv1alpha1.InstanceReplicasTemplate{{Name: "large", ReplicaChanges: 1}}}}
				case "offline-online":
					hs.ScaleIn = &opsv1alpha1.ScaleIn{OnlineInstancesToOffline: []string{largeName}}
					hs.ScaleOut = &opsv1alpha1.ScaleOut{OfflineInstancesToOnline: []string{offlineName}}
				}
				ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "hscale", Namespace: "default"},
					Spec:   opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{HorizontalScalingList: []opsv1alpha1.HorizontalScaling{hs}}},
					Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase, StartTimestamp: metav1.NewTime(time.Now().Add(-time.Minute))}}
				its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"},
					Status: workloads.InstanceSetStatus{InstanceStatus: source, CurrentRevisions: map[string]string{defaultName: "r", largeName: "r"}}}
				cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops, its).WithObjects(cluster, ops, its,
					&appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "database"}}).Build()
				res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(100), Runtimes: map[string]OpsRuntime{componentName: newOpsRuntime(ctx, cli, "")}}
				h := horizontalScalingOpsHandler{}
				req := intctrlutil.RequestCtx{Ctx: ctx}
				its.Status.InstanceStatus = nil
				if err := cli.Status().Update(ctx, its); err != nil {
					t.Fatal(err)
				}
				if err := h.SaveLastConfiguration(req, cli, res); !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting) {
					t.Fatalf("unpublished source allocation must wait: %v", err)
				}
				its.Status.InstanceStatus = source
				if err := cli.Status().Update(ctx, its); err != nil {
					t.Fatal(err)
				}
				if err := h.SaveLastConfiguration(req, cli, res); err != nil {
					t.Fatal(err)
				}
				// OpsManager persists LastConfiguration before invoking Action.
				if err := cli.Status().Update(ctx, ops); err != nil {
					t.Fatal(err)
				}
				saved := ops.DeepCopy().Status.LastConfiguration
				assignments := saved.Components[componentName].SourceInstanceAssignments
				want := 2
				if operation == "offline-online" {
					want = 3
				}
				if len(assignments) != want {
					t.Fatalf("unexpected captured assignments: %#v", assignments)
				}
				if err := h.Action(req, cli, res); err != nil {
					t.Fatal(err)
				}
				check := func(want opsv1alpha1.OpsPhase) {
					t.Helper()
					phase, _, err := h.ReconcileAction(req, cli, res)
					if err != nil {
						t.Fatal(err)
					}
					if phase != want {
						t.Fatalf("phase=%s progress=%s want=%s details=%#v", phase, ops.Status.Progress, want, ops.Status.Components)
					}
					if !reflect.DeepEqual(saved, ops.Status.LastConfiguration) {
						t.Fatal("reconcile changed source configuration")
					}
				}
				publish := func() {
					t.Helper()
					if err := cli.Status().Update(ctx, its); err != nil {
						t.Fatal(err)
					}
				}
				check(opsv1alpha1.OpsRunningPhase) // Neither stale identities nor equal-count stale swaps may complete.
				if operation == "scale-out" {
					// A later direct spec edit and its allocation are not this
					// operation's target, even when they agree with each other.
					cluster.Spec.ComponentSpecs[0].Replicas++
					its.Status.InstanceStatus = append(append([]workloads.InstanceStatus(nil), source...),
						workloads.InstanceStatus{PodName: laterDefaultName, TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive},
						workloads.InstanceStatus{PodName: newDefaultName, TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive})
					its.Status.CurrentRevisions[laterDefaultName] = "r"
					its.Status.CurrentRevisions[newDefaultName] = "r"
					publish()
					check(opsv1alpha1.OpsRunningPhase)
					cluster.Spec.ComponentSpecs[0].Replicas--
					delete(its.Status.CurrentRevisions, laterDefaultName)
					delete(its.Status.CurrentRevisions, newDefaultName)
				}
				target := append([]workloads.InstanceStatus(nil), source...)
				switch operation {
				case "scale-out", "cancel-out":
					target = append(target, workloads.InstanceStatus{PodName: newDefaultName, TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive})
				case "scale-in", "cancel-in":
					target[1].DesiredState = workloads.InstanceDesiredStateReleased
				case "offline-online":
					target[1].DesiredState = workloads.InstanceDesiredStateOffline
					target[2].DesiredState = workloads.InstanceDesiredStateActive
				}
				its.Status.InstanceStatus = target
				publish()
				check(opsv1alpha1.OpsRunningPhase) // Allocation alone does not create/delete actual instances.
				switch operation {
				case "scale-out", "cancel-out":
					its.Status.CurrentRevisions[newDefaultName] = "r"
				case "scale-in", "cancel-in":
					delete(its.Status.CurrentRevisions, largeName)
				case "offline-online":
					delete(its.Status.CurrentRevisions, largeName)
					its.Status.CurrentRevisions[offlineName] = "r"
				}
				publish()
				if operation != "cancel-out" && operation != "cancel-in" {
					check(opsv1alpha1.OpsSucceedPhase)
					return
				}
				ops.Spec.Cancel = true
				ops.Status.Phase = opsv1alpha1.OpsCancellingPhase
				if err := cli.Status().Update(ctx, ops); err != nil {
					t.Fatal(err)
				}
				if err := h.Cancel(req, cli, res); err != nil {
					t.Fatal(err)
				}
				check(opsv1alpha1.OpsRunningPhase) // Target still describes the forward operation.
				its.Status.InstanceStatus = append([]workloads.InstanceStatus(nil), source...)
				if operation == "cancel-out" {
					its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: newDefaultName, DesiredState: workloads.InstanceDesiredStateReleased, CurrentState: workloads.InstanceCurrentStateTerminating})
				}
				publish()
				check(opsv1alpha1.OpsRunningPhase) // Restored allocation is not proof that rollback has finished.
				if operation == "cancel-out" {
					delete(its.Status.CurrentRevisions, newDefaultName)
					its.Status.InstanceStatus = source
				} else {
					its.Status.CurrentRevisions[largeName] = "r"
				}
				publish()
				check(opsv1alpha1.OpsSucceedPhase)
			})
		}
	}
}
