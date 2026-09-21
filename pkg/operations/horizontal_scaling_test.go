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
	"errors"
	"fmt"
	"reflect"
	"slices"
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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
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

	mockScalingAssignments := func(opsRes *OpsResource) {
		mockHorizontalScalingProgress(opsRes.Cluster, defaultCompName)
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
			if hasExplicitScalingInstances(horizontalScaling) && !isBackupScaling(horizontalScaling) {
				mockScalingAssignments(opsRes)
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
			mockHorizontalScalingProgress(opsRes.Cluster, defaultCompName)
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
		}

		checkCancelledSucceed := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource) {
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsCancelledPhase))
			opsProgressDetails := opsRes.OpsRequest.Status.Components[defaultCompName].ProgressDetails
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("3/3"))
			Expect(len(opsProgressDetails)).Should(Equal(3))
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
			mockHorizontalScalingProgress(opsRes.Cluster, defaultCompName)
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
			mockHorizontalScalingProgress(opsRes.Cluster, defaultCompName)
			By("cancel HScale opsRequest after one pod has been deleted")
			cancelOpsRequest(reqCtx, opsRes, time.Now().Add(-1*time.Second))
			if isScaleDown {
				By("re-create the pod for rollback")
				createPods("", 2)
			} else {
				By("delete the pod for rollback")
				deletePods(pod)
			}
			mockHorizontalScalingProgress(opsRes.Cluster, defaultCompName)
			By("expect for opsRequest phase is Succeed after pods has been scaled and component phase is Running")
			mockConsensusCompToRunning(opsRes)
			checkCancelledSucceed(reqCtx, opsRes)

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

			By("complete the created restores and submit the replica target")
			for i := range restoreList.Items {
				key := client.ObjectKeyFromObject(&restoreList.Items[i])
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, key, func(restore *dpv1alpha1.Restore) {
					Expect(restore.Spec.Env).Should(Equal(restoreEnv))
					restore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
				})).Should(Succeed())
			}
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.Cluster.Spec.GetComponentByName(defaultCompName).Replicas).Should(Equal(int32(5)))

			By("mock pods to created and expect opsRequest phase to Succeed")
			createPods("", 3, 4)
			mockHorizontalScalingProgress(opsRes.Cluster, defaultCompName)
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
			mockHorizontalScalingProgress(opsRes.Cluster, defaultCompName)
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
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("3/3"))
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
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("4/4"))
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
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("3/3"))
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
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("4/4"))
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
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("5/5"))
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
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("4/4"))
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
			mockHorizontalScalingProgress(opsRes.Cluster, defaultCompName)
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
		})
		createOpsAndToCreatingPhase := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, horizontalScaling opsv1alpha1.HorizontalScaling, ignoreHscalingStrictValidate bool) *opsv1alpha1.OpsRequest {
			if horizontalScaling.ComponentName == "" {
				horizontalScaling.ComponentName = defaultCompName
			}
			if hasExplicitScalingInstances(horizontalScaling) && !isBackupScaling(horizontalScaling) {
				mockScalingAssignments(opsRes)
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
			By("mock the remaining active instances")
			its := &workloads.InstanceSet{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: opsRes.Cluster.Namespace, Name: constant.GenerateClusterComponentName(clusterName, defaultCompName)}, its)).Should(Succeed())
			testapps.MockInstanceSetPods(&testCtx, its, opsRes.Cluster, defaultCompName)
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
		})

		It("test offline duplicate instance names with ignore validation", func() {
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
			By("mock the remaining active instances")
			its := &workloads.InstanceSet{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: opsRes.Cluster.Namespace, Name: constant.GenerateClusterComponentName(clusterName, defaultCompName)}, its)).Should(Succeed())
			testapps.MockInstanceSetPods(&testCtx, its, opsRes.Cluster, defaultCompName)
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("3/3"), fmt.Sprintf("info: %v", opsRes.OpsRequest))

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
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("4/4"))
		})

		It("test run multi horizontalScaling opsRequest with force flag", func() {
			By("init operations resources with CLusterDefinition/Hybrid components Cluster/consensus Pods")
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			createPods("", 0, 1, 2)
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			By("create first opsRequest to add 1 replicas with `scaleOut` field and expect replicas to 4")
			first := createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ScaleOut: &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}},
			}, false)
			Expect(opsRes.Cluster.Spec.GetComponentByName(defaultCompName).Replicas).Should(BeEquivalentTo(4))

			By("create secondary opsRequest to add 1 replicas with `replicasToAdd` field and expect replicas to 5")
			second := createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ScaleOut: &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}},
			}, false)
			Expect(opsRes.Cluster.Spec.GetComponentByName(defaultCompName).Replicas).Should(BeEquivalentTo(5))

			createPods("", 3, 4)

			By("create third opsRequest to offline a pod which is created by another running opsRequest and expect it to fail")
			offlineInsName := fmt.Sprintf("%s-%s-3", clusterName, defaultCompName)
			_ = createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ScaleIn: &opsv1alpha1.ScaleIn{
					ReplicaChanger:           opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)},
					OnlineInstancesToOffline: []string{offlineInsName},
				},
			}, false)
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsFailedPhase))
			conditions := opsRes.OpsRequest.Status.Conditions
			Expect(conditions[len(conditions)-1].Message).Should(ContainSubstring(fmt.Sprintf(`instance "%s" cannot be taken offline as it has been created by another running opsRequest`, offlineInsName)))

			By("create a opsRequest to delete 1 replicas which is created by another running opsRequest and expect it to fail")
			_ = createOpsAndToCreatingPhase(reqCtx, opsRes, opsv1alpha1.HorizontalScaling{
				ScaleIn: &opsv1alpha1.ScaleIn{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}},
			}, false)
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsFailedPhase))
			conditions = opsRes.OpsRequest.Status.Conditions
			Expect(conditions[len(conditions)-1].Message).Should(ContainSubstring(`cannot be taken offline as it has been created by another running opsRequest`))
			By("complete both accepted scale-outs against the current five-replica target")
			for _, accepted := range []*opsv1alpha1.OpsRequest{first, second} {
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(accepted), accepted)).Should(Succeed())
				opsRes.OpsRequest = accepted
				checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
				Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("5/5"))
			}

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
			Expect(PatchOpsStatus(ctx, k8sClient, opsRes, opsv1alpha1.OpsRunningPhase)).Should(Succeed())
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
			comp1 := createComponent(secondaryCompName + "-comp1")
			comp2 := createComponent(secondaryCompName + "-comp2")
			comp3 := createComponent(secondaryCompName + "-comp3")
			comp4 := createComponent(secondaryCompName + "-comp4")
			comp5 := createComponent(secondaryCompName + "-comp5")
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, pobj *opsv1alpha1.OpsRequest) {
				g.Expect(pobj.Status.Progress).Should(Equal("0/2"))
				g.Expect(pobj.Status.Components[secondaryCompName].ProgressDetails).Should(HaveLen(5))
			})).Should(Succeed())

			By("expect ops phase to succeed when new components are running")
			for _, comp := range []*appsv1.Component{comp1, comp2, comp3, comp4, comp5} {
				Expect(testapps.ChangeObjStatus(&testCtx, comp, func() {
					comp.Status.Phase = appsv1.RunningComponentPhase
					comp.Status.ObservedGeneration = comp.Generation
				})).Should(Succeed())
			}
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/2"))
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
			mockShardingRunning := func() {
				Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
					opsRes.Cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{
						secondaryCompName: {Phase: appsv1.RunningComponentPhase,
							ObservedGeneration: opsRes.Cluster.Generation, UpToDate: true},
					}
				})).Should(Succeed())
			}
			mockShardingRunning()
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
			testapps.DeleteObject(&testCtx, client.ObjectKeyFromObject(comp5), &appsv1.Component{})
			mockShardingRunning()
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
	compStatus.UpToDate = true
	compStatus.ObservedGeneration = opsRes.Cluster.Generation
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

// Use the real naming and workload runtime; only API storage is in memory.
type horizontalScalingFixture struct {
	cli                                      client.Client
	res                                      *OpsResource
	req                                      intctrlutil.RequestCtx
	clusterWrites, backupReads, restoreReads int
}

// Record runtime calls to prove replica observations use the domain API.
type horizontalScalingRuntimeTrace struct {
	OpsRuntime
	calls []string
}

func (r *horizontalScalingRuntimeTrace) GetWorkload(namespace, clusterName, compName string) (Workload, error) {
	r.calls = append(r.calls, "workload("+compName+")")
	return r.OpsRuntime.GetWorkload(namespace, clusterName, compName)
}

func TestHorizontalScalingDoesNotReadRuntimeProgress(t *testing.T) {
	for _, fromBackup := range []bool{false, true} {
		t.Run(fmt.Sprintf("backup=%t", fromBackup), func(t *testing.T) {
			f := newHorizontalScalingFixture(t, scaleOutRequest("db", fromBackup))
			if fromBackup {
				f.addBackup(t)
			}
			trace := &horizontalScalingRuntimeTrace{OpsRuntime: f.res.Runtimes["db"]}
			f.res.Runtimes["db"] = trace
			hs := horizontalScalingOpsHandler{}
			if err := hs.Action(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			if len(trace.calls) > 0 {
				t.Fatalf("runtime progress calls: %v", trace.calls)
			}
		})
	}
}

func TestHorizontalScalingOnlineInferenceInputs(t *testing.T) {
	for _, tc := range []struct {
		name                                             string
		template, explicitTotal, explicitTemplate, empty bool
		wantCalls                                        []string
		wantReplicas                                     int32
	}{
		{name: "infer-default", wantReplicas: 2},
		{name: "infer-template", template: true, wantReplicas: 2},
		{name: "explicit-total-skips-inference", explicitTotal: true, wantReplicas: 2},
		{name: "explicit-template", template: true, explicitTemplate: true, wantReplicas: 2},
		{name: "empty-list-skips-inference", empty: true, wantReplicas: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := scaleOutRequest("db", false)
			request.ScaleOut.ReplicaChanges = nil
			instance := "demo-db-0"
			if tc.template {
				instance = "demo-db-foo-0"
			}
			if !tc.empty {
				request.ScaleOut.OfflineInstancesToOnline = []string{instance}
			}
			if tc.explicitTotal {
				request.ScaleOut.ReplicaChanges = pointer.Int32(1)
			}
			if tc.explicitTemplate {
				request.ScaleOut.Instances = []opsv1alpha1.InstanceReplicasTemplate{{Name: "foo", ReplicaChanges: 1}}
			}
			f := newHorizontalScalingFixture(t, scaleOutRequest("db", false))
			f.res.OpsRequest.Spec.HorizontalScalingList = []opsv1alpha1.HorizontalScaling{request}
			spec := &f.res.Cluster.Spec.ComponentSpecs[0]
			spec.OfflineInstances = []string{instance}
			if tc.template {
				spec.Instances = []appsv1.InstanceTemplate{{Name: "foo", Replicas: pointer.Int32(1)}}
			}
			publishHorizontalScalingAssignments(t, f)
			hs := horizontalScalingOpsHandler{}
			if err := hs.SaveLastConfiguration(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			original := spec.DeepCopy()
			trace := &horizontalScalingRuntimeTrace{OpsRuntime: f.res.Runtimes["db"]}
			f.res.Runtimes["db"] = trace
			if err := hs.Action(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(trace.calls, tc.wantCalls) {
				t.Fatalf("calls = %v, want %v", trace.calls, tc.wantCalls)
			}
			last := f.res.OpsRequest.Status.LastConfiguration.Components["db"]
			if !reflect.DeepEqual(last.InstanceTemplates, original.Instances) || !reflect.DeepEqual(last.OfflineInstances, original.OfflineInstances) {
				t.Fatalf("Action changed the saved configuration: %+v", last)
			}
			backupRequest := request.DeepCopy()
			backupRequest.ScaleOut.FromBackup = &opsv1alpha1.FromBackup{Name: "snapshot"}
			replicas, instances, offline, err := hs.getExpectedCompValues(f.res,
				f.res.OpsRequest.Status.LastConfiguration.Components["db"], *backupRequest)
			if err != nil {
				t.Fatal(err)
			}
			if replicas != spec.Replicas || !reflect.DeepEqual(instances, spec.Instances) || !reflect.DeepEqual(offline, spec.OfflineInstances) {
				t.Fatal("backup and ordinary planning produced different targets")
			}
			f.replicas(t, "db", tc.wantReplicas)
			if tc.template && (len(spec.Instances) != 1 || spec.Instances[0].GetReplicas() != 2) {
				t.Fatalf("target instances = %+v", spec.Instances)
			}
			if !tc.empty && len(spec.OfflineInstances) != 0 {
				t.Fatalf("target offline = %v", spec.OfflineInstances)
			}
		})
	}
}

func newHorizontalScalingFixture(t *testing.T, requests ...opsv1alpha1.HorizontalScaling) *horizontalScalingFixture {
	t.Helper()
	f := &horizontalScalingFixture{}
	f.req = intctrlutil.RequestCtx{Ctx: context.Background(), Recorder: record.NewFakeRecorder(100)}
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme,
		dpv1alpha1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "cluster1"},
		Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase}}
	ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "scale", Namespace: "default", UID: "opsreq123"},
		Spec: opsv1alpha1.OpsRequestSpec{Type: opsv1alpha1.HorizontalScalingType,
			SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{HorizontalScalingList: requests}},
		Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase}}
	objects := []client.Object{cluster, ops, &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "database"}}}
	for _, request := range requests {
		spec := appsv1.ClusterComponentSpec{Name: request.ComponentName, ComponentDef: "database", Replicas: 1,
			VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{Name: "data"}}}
		cluster.Spec.ComponentSpecs = append(cluster.Spec.ComponentSpecs, spec)
		comp, err := component.BuildComponent(cluster, &spec, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, comp)
	}
	f.cli = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).
		WithStatusSubresource(ops, &dpv1alpha1.Restore{}).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, cli client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			switch obj.(type) {
			case *dpv1alpha1.Backup:
				f.backupReads++
			case *dpv1alpha1.Restore:
				f.restoreReads++
			}
			return cli.Get(ctx, key, obj, opts...)
		},
		Update: func(ctx context.Context, cli client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*appsv1.Cluster); ok {
				f.clusterWrites++
			}
			return cli.Update(ctx, obj, opts...)
		},
	}).Build()
	f.res = &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: f.req.Recorder}
	var err error
	f.res.Runtimes, err = buildOpsRuntimes(f.req.Ctx, f.cli, f.res)
	if err != nil {
		t.Fatal(err)
	}
	publishHorizontalScalingAssignments(t, f)
	if err := (horizontalScalingOpsHandler{}).SaveLastConfiguration(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	// OpsManager persists this snapshot before invoking Action/ReconcileAction.
	if err := f.cli.Status().Update(f.req.Ctx, ops); err != nil {
		t.Fatal(err)
	}
	return f
}

func scaleOutRequest(name string, backup bool) opsv1alpha1.HorizontalScaling {
	r := opsv1alpha1.HorizontalScaling{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: name},
		ScaleOut: &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}}
	if backup {
		r.ScaleOut.FromBackup = &opsv1alpha1.FromBackup{Name: "snapshot"}
	}
	return r
}

func (f *horizontalScalingFixture) addBackup(t *testing.T) {
	t.Helper()
	backup := &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "snapshot", Namespace: "default"},
		Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseCompleted,
			BackupMethod: &dpv1alpha1.BackupMethod{Name: "snapshot", SnapshotVolumes: pointer.Bool(true),
				TargetVolumes: &dpv1alpha1.TargetVolumeInfo{Volumes: []string{"data"}}},
			Targets: []dpv1alpha1.BackupStatusTarget{{BackupTarget: dpv1alpha1.BackupTarget{
				Name: "db", PodSelector: &dpv1alpha1.PodSelector{}}}}}}
	if err := f.cli.Create(f.req.Ctx, backup); err != nil {
		t.Fatal(err)
	}
}

func (f *horizontalScalingFixture) replicas(t *testing.T, name string, want int32) {
	t.Helper()
	cluster := &appsv1.Cluster{}
	if err := f.cli.Get(f.req.Ctx, client.ObjectKeyFromObject(f.res.Cluster), cluster); err != nil {
		t.Fatal(err)
	}
	if got := cluster.Spec.GetComponentByName(name).Replicas; got != want {
		t.Fatalf("%s replicas = %d, want %d", name, got, want)
	}
}

func (f *horizontalScalingFixture) reconcile(t *testing.T, want opsv1alpha1.OpsPhase) {
	t.Helper()
	phase, _, err := (horizontalScalingOpsHandler{}).ReconcileAction(f.req, f.cli, f.res)
	if err != nil {
		t.Fatal(err)
	}
	if phase != want {
		t.Fatalf("reconcile phase = %s, want %s", phase, want)
	}
}

func TestHorizontalScalingOrdinaryPathDoesNotRestore(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", false))
	if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	f.replicas(t, "db", 2)
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	if f.backupReads != 0 || f.restoreReads != 0 || f.clusterWrites != 1 {
		t.Fatalf("ordinary path API calls: backup=%d restore=%d cluster writes=%d", f.backupReads, f.restoreReads, f.clusterWrites)
	}
	details := f.res.OpsRequest.Status.Components["db"].ProgressDetails
	if f.res.OpsRequest.Status.Progress != "0/1" || len(details) != 1 || details[0].ObjectKey != "Pod/demo-db-0" || details[0].Status != opsv1alpha1.ProcessingProgressStatus {
		t.Fatalf("unexpected create progress: %+v", f.res.OpsRequest.Status)
	}
}

func TestHorizontalScalingMixedComponentsRestoreTiming(t *testing.T) {
	for _, backupFirst := range []bool{false, true} {
		name := "ordinary-first"
		if backupFirst {
			name = "backup-first"
		}
		t.Run(name, func(t *testing.T) {
			requests := []opsv1alpha1.HorizontalScaling{scaleOutRequest("ordinary", false), scaleOutRequest("restored", true)}
			if backupFirst {
				requests[0], requests[1] = requests[1], requests[0]
			}
			f := newHorizontalScalingFixture(t, requests...)
			f.addBackup(t)
			if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			f.replicas(t, "ordinary", 2)
			f.replicas(t, "restored", 1)
			if f.backupReads != 0 || f.restoreReads != 0 {
				t.Fatal("Action must defer Restore work")
			}
			for i := 0; i < 2; i++ {
				f.reconcile(t, opsv1alpha1.OpsRunningPhase)
				f.replicas(t, "restored", 1)
				if f.clusterWrites != 1 {
					t.Fatal("pending Restore wrote target configuration")
				}
				if got := f.res.OpsRequest.Status.Components["restored"].Message; got != "Restore Data In Progress" {
					t.Fatalf("message = %q", got)
				}
				if got := f.res.OpsRequest.Status.Progress; got != "0/2" {
					t.Fatalf("progress = %q", got)
				}
			}
			restores := &dpv1alpha1.RestoreList{}
			if err := f.cli.List(f.req.Ctx, restores); err != nil {
				t.Fatal(err)
			}
			if len(restores.Items) != 1 {
				t.Fatalf("repeated reconcile created %d restores", len(restores.Items))
			}
			restore := &restores.Items[0]
			if restore.Spec.PrepareDataConfig.RestoreVolumeClaimsTemplate.StartingIndex != 1 {
				t.Fatal("unexpected restore ordinal")
			}
			restore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
			if err := f.cli.Status().Update(f.req.Ctx, restore); err != nil {
				t.Fatal(err)
			}
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			f.replicas(t, "restored", 2)
			if f.clusterWrites != 2 || f.res.OpsRequest.Status.Components["restored"].Message != "Restore Data Completed" {
				t.Fatal("missing restore completion write/message")
			}
			reads := f.backupReads + f.restoreReads
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			if f.clusterWrites != 2 || reads != f.backupReads+f.restoreReads {
				t.Fatal("completed marker did not skip repeated restore work")
			}
			// Restore completion alone does not complete the Ops: the newly
			// requested instances must become available on both paths.
			publishHorizontalScalingResult(t, f)
			f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
			if got := f.res.OpsRequest.Status.Progress; got != "4/4" {
				t.Fatalf("completed progress = %q", got)
			}
			for _, name := range []string{"ordinary", "restored"} {
				details := f.res.OpsRequest.Status.Components[name].ProgressDetails
				if len(details) != 2 || details[0].ObjectKey != "Pod/demo-"+name+"-0" || details[0].Status != opsv1alpha1.SucceedProgressStatus {
					t.Fatalf("unexpected completed participants for %s: %+v", name, details)
				}
			}
			f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
			if f.clusterWrites != 2 || reads != f.backupReads+f.restoreReads {
				t.Fatal("completed Ops repeated restore work")
			}
		})
	}
}

func TestHorizontalScalingRestoreFailureAndCancellation(t *testing.T) {
	t.Run("failed-restore", func(t *testing.T) {
		f := newHorizontalScalingFixture(t, scaleOutRequest("db", true))
		f.addBackup(t)
		if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
			t.Fatal(err)
		}
		f.reconcile(t, opsv1alpha1.OpsRunningPhase)
		restores := &dpv1alpha1.RestoreList{}
		if err := f.cli.List(f.req.Ctx, restores); err != nil {
			t.Fatal(err)
		}
		if len(restores.Items) != 1 {
			t.Fatalf("restores = %d", len(restores.Items))
		}
		restore := &restores.Items[0]
		restore.Status.Phase = dpv1alpha1.RestorePhaseFailed
		if err := f.cli.Status().Update(f.req.Ctx, restore); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			_, _, err := (horizontalScalingOpsHandler{}).ReconcileAction(f.req, f.cli, f.res)
			if !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) || !strings.Contains(err.Error(), "restore for horizontalScaling failed") {
				t.Fatalf("unexpected error: %v", err)
			}
			f.replicas(t, "db", 1)
			if f.clusterWrites != 1 {
				t.Fatal("failed Restore wrote target configuration")
			}
		}
	})
	t.Run("cancelling-still-validates-backup", func(t *testing.T) {
		f := newHorizontalScalingFixture(t, scaleOutRequest("db", true))
		f.res.OpsRequest.Status.Phase = opsv1alpha1.OpsCancellingPhase
		if err := (horizontalScalingOpsHandler{}).Cancel(f.req, f.cli, f.res); err != nil {
			t.Fatal(err)
		}
		_, _, err := (horizontalScalingOpsHandler{}).ReconcileAction(f.req, f.cli, f.res)
		if !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) || !strings.Contains(err.Error(), "backup snapshot not found") {
			t.Fatalf("unexpected cancellation error: %v", err)
		}
		if f.backupReads != 1 {
			t.Fatal("Cancelling bypassed the existing backup path")
		}
	})
}

func TestHorizontalScalingCancelRestoresConfigurationAndDirection(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", false))
	spec := &f.res.Cluster.Spec.ComponentSpecs[0]
	spec.Instances = []appsv1.InstanceTemplate{{Name: "foo", Replicas: pointer.Int32(1)}}
	spec.OfflineInstances = []string{"demo-db-foo-0"}
	hs := horizontalScalingOpsHandler{}
	if err := hs.SaveLastConfiguration(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	last := f.res.OpsRequest.Status.LastConfiguration.Components["db"]
	if err := f.cli.Status().Update(f.req.Ctx, f.res.OpsRequest); err != nil {
		t.Fatal(err)
	}
	if err := hs.Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	f.res.OpsRequest.Status.Phase = opsv1alpha1.OpsCancellingPhase
	if err := hs.Cancel(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	if spec.Replicas != *last.Replicas || !reflect.DeepEqual(spec.Instances, last.InstanceTemplates) || !reflect.DeepEqual(spec.OfflineInstances, last.OfflineInstances) {
		t.Fatal("Cancel did not restore the original configuration")
	}
	// ReconcileAction reports successful rollback; OpsManager maps it to Cancelled.
	publishHorizontalScalingResult(t, f)
	f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
	if f.res.OpsRequest.Status.Progress != "1/1" {
		t.Fatalf("unexpected rollback observation: %+v", f.res.OpsRequest.Status)
	}
}

func TestHorizontalScalingShardCountAndShardReplicas(t *testing.T) {
	for _, changeCount := range []bool{false, true} {
		name := "replicas-per-shard"
		request := scaleOutRequest("sharded", false)
		if changeCount {
			name = "shard-count"
			request.ScaleOut = nil
			request.Shards = pointer.Int32(3)
		}
		t.Run(name, func(t *testing.T) {
			f := newHorizontalScalingFixture(t, request)
			cluster := f.res.Cluster
			cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "sharded", Shards: 2, Template: cluster.Spec.ComponentSpecs[0]}}
			cluster.Spec.ComponentSpecs = nil
			for _, shard := range []string{"sharded-a", "sharded-b"} {
				comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "demo-" + shard, Namespace: "default",
					Labels: constant.GetCompLabels("demo", shard, map[string]string{constant.KBAppShardingNameLabelKey: "sharded"})}}
				if err := f.cli.Create(f.req.Ctx, comp); err != nil {
					t.Fatal(err)
				}
			}
			publishHorizontalScalingAssignments(t, f)
			hs := horizontalScalingOpsHandler{}
			if err := hs.SaveLastConfiguration(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			if err := f.cli.Status().Update(f.req.Ctx, f.res.OpsRequest); err != nil {
				t.Fatal(err)
			}
			if err := hs.Action(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			wantShards, wantReplicas, wantProgress := int32(2), int32(2), "0/4"
			if changeCount {
				wantShards, wantReplicas, wantProgress = 3, 1, "0/1"
			}
			if got := cluster.Spec.Shardings[0]; got.Shards != wantShards || got.Template.Replicas != wantReplicas {
				t.Fatalf("unexpected sharding: %+v", got)
			}
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			if got := f.res.OpsRequest.Status.Progress; got != wantProgress {
				t.Fatalf("progress = %s, want %s", got, wantProgress)
			}
			details := f.res.OpsRequest.Status.Components["sharded"].ProgressDetails
			if changeCount && len(details) != 2 || !changeCount && len(details) != 0 {
				t.Fatalf("progress details = %+v", details)
			}
			for _, detail := range details {
				prefix := "Pod/demo-sharded-"
				if changeCount {
					prefix = "Component/demo-sharded-"
				}
				if !strings.HasPrefix(detail.ObjectKey, prefix) {
					t.Fatalf("wrong progress path: %+v", detail)
				}
			}
			if f.backupReads != 0 || f.restoreReads != 0 {
				t.Fatal("sharding request entered Restore path")
			}
		})
	}
}

func TestHorizontalScalingCancellingBackupKeepsOriginalScale(t *testing.T) {
	for _, tc := range []struct {
		name               string
		startRestore       bool
		replicaChanges     int32
		wantRestoreCount   int
		completeOneRestore bool
	}{
		{name: "before-first-restore", replicaChanges: 1},
		{name: "after-partial-restore", startRestore: true, replicaChanges: 2, wantRestoreCount: 2, completeOneRestore: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := scaleOutRequest("db", true)
			request.ScaleOut.ReplicaChanges = pointer.Int32(tc.replicaChanges)
			f := newHorizontalScalingFixture(t, request)
			f.addBackup(t)
			hs := horizontalScalingOpsHandler{}
			if err := hs.Action(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			if tc.startRestore {
				f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			}

			restores := &dpv1alpha1.RestoreList{}
			if err := f.cli.List(f.req.Ctx, restores); err != nil {
				t.Fatal(err)
			}
			if len(restores.Items) != tc.wantRestoreCount {
				t.Fatalf("restores before cancellation = %d, want %d", len(restores.Items), tc.wantRestoreCount)
			}
			if tc.completeOneRestore {
				restore := &restores.Items[0]
				restore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
				if err := f.cli.Status().Update(f.req.Ctx, restore); err != nil {
					t.Fatal(err)
				}
			}

			f.res.OpsRequest.Status.Phase = opsv1alpha1.OpsCancellingPhase
			if err := f.cli.Status().Update(f.req.Ctx, f.res.OpsRequest); err != nil {
				t.Fatal(err)
			}
			if err := hs.Cancel(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			f.replicas(t, "db", 1)
			publishHorizontalScalingResult(t, f)
			f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
			f.replicas(t, "db", 1)

			afterCancel := &dpv1alpha1.RestoreList{}
			if err := f.cli.List(f.req.Ctx, afterCancel); err != nil {
				t.Fatal(err)
			}
			if len(afterCancel.Items) != tc.wantRestoreCount {
				t.Fatalf("restores after cancellation = %d, want %d", len(afterCancel.Items), tc.wantRestoreCount)
			}
			if tc.completeOneRestore {
				completed := 0
				for i := range afterCancel.Items {
					if afterCancel.Items[i].Status.Phase == dpv1alpha1.RestorePhaseCompleted {
						completed++
					}
				}
				if completed != 1 {
					t.Fatalf("completed restores after cancellation = %d, want 1", completed)
				}
			}
		})
	}
}

func TestHorizontalScalingMixedBackupConflicts(t *testing.T) {
	for _, earlierBackup := range []bool{false, true} {
		for _, currentBackup := range []bool{false, true} {
			t.Run(fmt.Sprintf("earlier-backup=%t/current-backup=%t", earlierBackup, currentBackup), func(t *testing.T) {
				current := scaleOutRequest("db", currentBackup)
				current.ScaleOut.ReplicaChanges = pointer.Int32(0)
				current.ScaleIn = &opsv1alpha1.ScaleIn{
					ReplicaChanger:           opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)},
					OnlineInstancesToOffline: []string{"demo-db-1"},
				}
				f := newHorizontalScalingFixture(t, scaleOutRequest("db", currentBackup))
				f.res.OpsRequest.Spec.HorizontalScalingList = []opsv1alpha1.HorizontalScaling{current}
				earlier := f.res.OpsRequest.DeepCopy()
				earlier.Name, earlier.UID, earlier.ResourceVersion = "earlier", "earlier1", ""
				earlier.Spec.HorizontalScalingList = []opsv1alpha1.HorizontalScaling{scaleOutRequest("db", earlierBackup)}
				if err := f.cli.Create(f.req.Ctx, earlier); err != nil {
					t.Fatal(err)
				}
				f.res.Cluster.Spec.ComponentSpecs[0].Replicas = 2
				publishHorizontalScalingAssignments(t, f)
				if err := (horizontalScalingOpsHandler{}).SaveLastConfiguration(f.req, f.cli, f.res); err != nil {
					t.Fatal(err)
				}
				f.res.Cluster.Annotations = map[string]string{constant.OpsRequestAnnotationKey: fmt.Sprintf(`[{"name":"earlier","type":%q},{"name":"scale","type":%q}]`, opsv1alpha1.HorizontalScalingType, opsv1alpha1.HorizontalScalingType)}
				err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res)
				if !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) || !strings.Contains(err.Error(), `instance "demo-db-1" cannot be taken offline`) {
					t.Fatalf("expected conflict with earlier request, got %v", err)
				}
				if f.clusterWrites != 0 || f.backupReads != 0 || f.restoreReads != 0 {
					t.Fatal("conflict performed scaling or restore work")
				}
			})
		}
	}
}

func TestHorizontalScalingBackupRejectsInvalidPlanBeforeWork(t *testing.T) {
	request := scaleOutRequest("db", true)
	request.ScaleOut.NewInstances = []appsv1.InstanceTemplate{{Name: "invalid", Replicas: pointer.Int32(1),
		Ordinals: appsv1.Ordinals{Ranges: []appsv1.Range{{Start: 3, End: 2}}}}}
	actionFixture := newHorizontalScalingFixture(t, scaleOutRequest("db", true))
	actionFixture.res.OpsRequest.Status.LastConfiguration.Components["db"] = opsv1alpha1.LastComponentConfiguration{
		Replicas: pointer.Int32(1), InstanceTemplates: request.ScaleOut.NewInstances,
	}
	hs := horizontalScalingOpsHandler{}
	if err := hs.Action(actionFixture.req, actionFixture.cli, actionFixture.res); err == nil {
		t.Fatal("Action accepted invalid existing instance ordinals")
	}
	if actionFixture.clusterWrites != 0 || actionFixture.backupReads != 0 || actionFixture.restoreReads != 0 {
		t.Fatal("invalid existing instance ordinals performed work during Action")
	}

	f := newHorizontalScalingFixture(t, request)
	f.addBackup(t)
	for i := 0; i < 2; i++ {
		if _, _, err := hs.ReconcileAction(f.req, f.cli, f.res); err == nil {
			t.Fatal("invalid instance plan was accepted")
		}
	}
	if f.clusterWrites != 0 || f.restoreReads != 0 {
		t.Fatal("invalid plan performed scaling or restore work")
	}
}

func publishHorizontalScalingAssignments(t *testing.T, f *horizontalScalingFixture) {
	t.Helper()
	for _, target := range f.res.OpsRequest.Spec.HorizontalScalingList {
		if target.Shards != nil {
			continue
		}
		spec := getComponentSpecOrShardingTemplate(f.res.Cluster, target.ComponentName)
		if spec == nil {
			continue
		}
		names, err := generateAllPodNamesToSet(spec.Replicas, spec.Instances, spec.OfflineInstances, f.res.Cluster.Name, target.ComponentName)
		if err != nil {
			t.Fatal(err)
		}
		its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: constant.GenerateClusterComponentName(f.res.Cluster.Name, target.ComponentName), Namespace: f.res.Cluster.Namespace}, Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](spec.Replicas)}}
		for name := range names {
			template := appsv1.GetInstanceTemplateName(f.res.Cluster.Name, target.ComponentName, name)
			its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: name, TemplateName: &template, DesiredState: workloads.InstanceDesiredStateActive})
		}
		for _, name := range spec.OfflineInstances {
			template := appsv1.GetInstanceTemplateName(f.res.Cluster.Name, target.ComponentName, name)
			its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: name, TemplateName: &template, DesiredState: workloads.InstanceDesiredStateOffline, CurrentState: workloads.InstanceCurrentStateAbsent})
		}
		current := &workloads.InstanceSet{}
		if err := f.cli.Get(f.req.Ctx, client.ObjectKeyFromObject(its), current); client.IgnoreNotFound(err) != nil {
			t.Fatal(err)
		} else if err != nil {
			if err := f.cli.Create(f.req.Ctx, its); err != nil {
				t.Fatal(err)
			}
		} else {
			current.Status = its.Status
			current.Spec = its.Spec
			if err := f.cli.Update(f.req.Ctx, current); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestHorizontalScalingPersistsOwnerAssignmentsBeforeAction(t *testing.T) {
	for _, invalidTemplateCount := range []bool{false, true} {
		t.Run(fmt.Sprintf("invalid-template-count=%t", invalidTemplateCount), func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "cluster"},
				Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "db", Replicas: 2,
					Instances: []appsv1.InstanceTemplate{{Name: "blue", Replicas: pointer.Int32(1)}}}}},
				Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase}}
			ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "scale", Namespace: "default", UID: "request"},
				Spec: opsv1alpha1.OpsRequestSpec{Type: opsv1alpha1.HorizontalScalingType,
					SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{HorizontalScalingList: []opsv1alpha1.HorizontalScaling{{
						ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"},
						ScaleIn:      &opsv1alpha1.ScaleIn{OnlineInstancesToOffline: []string{"owner-chosen"}},
					}}}}, Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsPendingPhase}}
			if invalidTemplateCount {
				ops.Spec.HorizontalScalingList[0].ScaleIn.Instances = []opsv1alpha1.InstanceReplicasTemplate{{Name: "blue", ReplicaChanges: 0}}
			}
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"},
				Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{
					{PodName: "owner-chosen", TemplateName: pointer.String("blue"), DesiredState: workloads.InstanceDesiredStateActive},
					{PodName: "another-identity", TemplateName: pointer.String(""), DesiredState: workloads.InstanceDesiredStateActive},
				}}}
			failPatch := true
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, ops, its).
				WithStatusSubresource(ops).WithInterceptorFuncs(interceptor.Funcs{
				SubResourcePatch: func(ctx context.Context, c client.Client, subresource string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
					if request, ok := obj.(*opsv1alpha1.OpsRequest); ok && request.Status.Phase == opsv1alpha1.OpsCreatingPhase && failPatch {
						failPatch = false
						return errors.New("injected allocation persistence failure")
					}
					return c.SubResource(subresource).Patch(ctx, obj, patch, opts...)
				},
			}).Build()
			req := intctrlutil.RequestCtx{Ctx: context.Background(), Recorder: record.NewFakeRecorder(32)}
			load := func() *OpsResource {
				c, o := &appsv1.Cluster{}, &opsv1alpha1.OpsRequest{}
				if err := cli.Get(req.Ctx, client.ObjectKeyFromObject(cluster), c); err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(req.Ctx, client.ObjectKeyFromObject(ops), o); err != nil {
					t.Fatal(err)
				}
				return &OpsResource{Cluster: c, OpsRequest: o, Recorder: req.Recorder}
			}
			var patchErr error
			for range 4 {
				_, patchErr = GetOpsManager().Do(req, cli, load())
				if patchErr != nil {
					break
				}
			}
			if patchErr == nil || !strings.Contains(patchErr.Error(), "allocation persistence") {
				t.Fatalf("expected persistence failure, got %v", patchErr)
			}
			res := load()
			if res.OpsRequest.Status.Phase != opsv1alpha1.OpsPendingPhase || res.Cluster.Spec.ComponentSpecs[0].Replicas != 2 {
				t.Fatalf("failed baseline patch changed operation or target: %+v", res)
			}
			for range 4 {
				res = load()
				if res.OpsRequest.Status.Phase == opsv1alpha1.OpsCreatingPhase {
					break
				}
				if _, err := GetOpsManager().Do(req, cli, res); err != nil {
					t.Fatal(err)
				}
			}
			res = load()
			instances := res.OpsRequest.Status.LastConfiguration.Components["db"].Instances
			if res.OpsRequest.Status.Phase != opsv1alpha1.OpsCreatingPhase || !reflect.DeepEqual(instances, []opsv1alpha1.LastInstanceConfiguration{{Name: "owner-chosen", TemplateName: "blue"}}) {
				t.Fatalf("owner assignments were not persisted: %+v", res.OpsRequest.Status)
			}
			if len(instances) != 1 {
				t.Fatalf("persisted unrelated owner assignments: %v", instances)
			}
			if err := cli.Delete(req.Ctx, its); err != nil {
				t.Fatal(err)
			}
			if invalidTemplateCount {
				if _, err := GetOpsManager().Do(req, cli, load()); err != nil {
					t.Fatal(err)
				}
				res = load()
				if res.OpsRequest.Status.Phase != opsv1alpha1.OpsFailedPhase ||
					!strings.Contains(fmt.Sprint(res.OpsRequest.Status.Conditions), "replicaChanges") {
					t.Fatalf("Creating did not reject the template count: %+v", res.OpsRequest.Status)
				}
				if !reflect.DeepEqual(res.Cluster.Spec, cluster.Spec) {
					t.Fatalf("invalid template count changed Cluster spec: %+v", res.Cluster.Spec)
				}
				return
			}
			for range 2 {
				if _, err := GetOpsManager().Do(req, cli, load()); err != nil {
					t.Fatalf("Creating replay must reuse assignments: %v", err)
				}
			}
			res = load()
			target := res.Cluster.Spec.ComponentSpecs[0]
			if target.Replicas != 1 || target.Instances[0].GetReplicas() != 0 || !reflect.DeepEqual(target.OfflineInstances, []string{"owner-chosen"}) {
				t.Fatalf("wrong target after replay: %+v", target)
			}
		})
	}
}

func TestHorizontalScalingRejectsInvalidInstanceTargets(t *testing.T) {
	for _, tc := range []struct {
		name         string
		targets      []string
		ignore       bool
		wantReplicas int32
	}{
		{name: "invalid", targets: []string{"missing"}, wantReplicas: 3},
		{name: "mixed", targets: []string{"owner-chosen", "missing"}, wantReplicas: 3},
		{name: "ignore-invalid", targets: []string{"missing"}, ignore: true, wantReplicas: 3},
		{name: "ignore-mixed", targets: []string{"owner-chosen", "missing"}, ignore: true, wantReplicas: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "cluster"},
				Spec:   appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "db", Replicas: 3}}},
				Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase}}
			ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "scale", Namespace: "default", UID: "request"},
				Spec: opsv1alpha1.OpsRequestSpec{Type: opsv1alpha1.HorizontalScalingType,
					SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{HorizontalScalingList: []opsv1alpha1.HorizontalScaling{{
						ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"},
						ScaleIn:      &opsv1alpha1.ScaleIn{OnlineInstancesToOffline: tc.targets},
					}}}}, Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsPendingPhase}}
			if tc.ignore {
				ops.Annotations = map[string]string{constant.IgnoreHscaleValidateAnnoKey: "true"}
			}
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"},
				Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{
					{PodName: "owner-chosen", TemplateName: pointer.String(""), DesiredState: workloads.InstanceDesiredStateActive},
					{PodName: "another-identity", TemplateName: pointer.String(""), DesiredState: workloads.InstanceDesiredStateActive},
					{PodName: "third-identity", TemplateName: pointer.String(""), DesiredState: workloads.InstanceDesiredStateActive},
				}}}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, ops, its).WithStatusSubresource(ops).Build()
			req := intctrlutil.RequestCtx{Ctx: context.Background(), Recorder: record.NewFakeRecorder(32)}
			for range 4 {
				if err := cli.Get(req.Ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(req.Ctx, client.ObjectKeyFromObject(ops), ops); err != nil {
					t.Fatal(err)
				}
				if ops.Status.Phase == opsv1alpha1.OpsFailedPhase {
					break
				}
				res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: req.Recorder}
				if _, err := GetOpsManager().Do(req, cli, res); err != nil {
					t.Fatalf("invalid target must be rejected, not retried: %v", err)
				}
			}
			if err := cli.Get(req.Ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(req.Ctx, client.ObjectKeyFromObject(ops), ops); err != nil {
				t.Fatal(err)
			}
			if !tc.ignore && (ops.Status.Phase != opsv1alpha1.OpsFailedPhase ||
				!strings.Contains(fmt.Sprint(ops.Status.Conditions), `instance "missing" specified in onlineInstancesToOffline is not online`)) {
				t.Fatalf("expected invalid-instance failure, got %+v", ops.Status)
			}
			if tc.ignore && ops.Status.Phase != opsv1alpha1.OpsCreatingPhase {
				t.Fatalf("ignore mode did not reach Action: %+v", ops.Status)
			}
			spec := cluster.Spec.ComponentSpecs[0]
			if spec.Replicas != tc.wantReplicas {
				t.Fatalf("replicas = %d, want %d", spec.Replicas, tc.wantReplicas)
			}
			wantOffline := []string(nil)
			if tc.wantReplicas == 2 {
				wantOffline = []string{"owner-chosen"}
			}
			if !reflect.DeepEqual(spec.OfflineInstances, wantOffline) {
				t.Fatalf("offline instances = %v, want %v", spec.OfflineInstances, wantOffline)
			}
			if !tc.ignore {
				return
			}
			cluster.Status.Components = map[string]appsv1.ClusterComponentStatus{
				"db": {ObservedGeneration: cluster.Generation, UpToDate: true, Phase: appsv1.RunningComponentPhase},
			}
			its.Spec.Replicas = ptr.To[int32](spec.Replicas)
			its.Status.ObservedGeneration = its.Generation
			for i := range its.Status.InstanceStatus {
				status := &its.Status.InstanceStatus[i]
				status.CurrentState = workloads.InstanceCurrentStatePresent
				status.UpToDate, status.Ready, status.Available = true, true, true
				if slices.Contains(wantOffline, status.PodName) {
					status.DesiredState = workloads.InstanceDesiredStateOffline
					status.CurrentState = workloads.InstanceCurrentStateAbsent
				}
			}
			if err := cli.Update(req.Ctx, its); err != nil {
				t.Fatal(err)
			}
			res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: req.Recorder}
			if _, err := GetOpsManager().Reconcile(req, cli, res); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(req.Ctx, client.ObjectKeyFromObject(ops), ops); err != nil {
				t.Fatal(err)
			}
			if ops.Status.Phase != opsv1alpha1.OpsSucceedPhase || ops.Status.Progress != "3/3" {
				t.Fatalf("ignored names prevented completion: %+v", ops.Status)
			}
		})
	}
}

// Publish the apps result and workload observations separately from the request.
func publishHorizontalScalingResult(t *testing.T, f *horizontalScalingFixture) {
	t.Helper()
	publishHorizontalScalingAssignments(t, f)
	if f.res.Cluster.Status.Components == nil {
		f.res.Cluster.Status.Components = map[string]appsv1.ClusterComponentStatus{}
	}
	for _, spec := range f.res.Cluster.Spec.ComponentSpecs {
		f.res.Cluster.Status.Components[spec.Name] = appsv1.ClusterComponentStatus{Phase: appsv1.RunningComponentPhase,
			ObservedGeneration: f.res.Cluster.Generation, UpToDate: true}
		its := &workloads.InstanceSet{}
		if err := f.cli.Get(f.req.Ctx, client.ObjectKey{Namespace: f.res.Cluster.Namespace, Name: constant.GenerateClusterComponentName(f.res.Cluster.Name, spec.Name)}, its); err != nil {
			t.Fatal(err)
		}
		its.Status.ObservedGeneration = its.Generation
		for i := range its.Status.InstanceStatus {
			status := &its.Status.InstanceStatus[i]
			if status.EffectiveDesiredState() == workloads.InstanceDesiredStateActive {
				status.CurrentState = workloads.InstanceCurrentStatePresent
				status.UpToDate, status.Ready, status.Available = true, true, true
			}
		}
		if err := f.cli.Update(f.req.Ctx, its); err != nil {
			t.Fatal(err)
		}
	}
}

// Model owner observations, including terminating identities outside the target allocation.
func mockHorizontalScalingProgress(cluster *appsv1.Cluster, name string) {
	mockRunningInstanceStatus(cluster, name)
	key := client.ObjectKey{Namespace: cluster.Namespace, Name: constant.GenerateClusterComponentName(cluster.Name, name)}
	its := &workloads.InstanceSet{}
	Expect(k8sClient.Get(testCtx.Ctx, key, its)).Should(Succeed())
	spec := cluster.Spec.GetComponentByName(name)
	its.Spec.Replicas = ptr.To[int32](spec.Replicas)
	Expect(k8sClient.Update(testCtx.Ctx, its)).Should(Succeed())
	pods := &corev1.PodList{}
	Expect(k8sClient.List(testCtx.Ctx, pods, client.InNamespace(cluster.Namespace), client.MatchingLabels{constant.AppInstanceLabelKey: cluster.Name, constant.KBAppComponentLabelKey: name})).Should(Succeed())
	Eventually(testapps.GetAndChangeObjStatus(&testCtx, key, func(its *workloads.InstanceSet) {
		its.Status.ObservedGeneration = its.Generation
		for _, offline := range spec.OfflineInstances {
			template := appsv1.GetInstanceTemplateName(cluster.Name, name, offline)
			its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: offline, TemplateName: &template, DesiredState: workloads.InstanceDesiredStateOffline, CurrentState: workloads.InstanceCurrentStateAbsent})
		}
		for _, pod := range pods.Items {
			if its.FindInstanceStatus(pod.Name) != nil {
				continue
			}
			template := appsv1.GetInstanceTemplateName(cluster.Name, name, pod.Name)
			currentState := workloads.InstanceCurrentStatePresent
			if !pod.DeletionTimestamp.IsZero() {
				currentState = workloads.InstanceCurrentStateTerminating
			}
			its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: pod.Name, TemplateName: &template, DesiredState: workloads.InstanceDesiredStateReleased, CurrentState: currentState})
		}
	})).Should(Succeed())
}

func TestHorizontalScalingResultAndCurrentProgressConverge(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", false))
	hs := horizontalScalingOpsHandler{}
	if err := hs.Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	publishHorizontalScalingResult(t, f)
	key := client.ObjectKey{Namespace: "default", Name: "demo-db"}
	its := &workloads.InstanceSet{}
	if err := f.cli.Get(f.req.Ctx, key, its); err != nil {
		t.Fatal(err)
	}
	// A complete instance projection does not decide the apps result.
	status := f.res.Cluster.Status.Components["db"]
	status.UpToDate = false
	f.res.Cluster.Status.Components["db"] = status
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	if f.res.OpsRequest.Status.Progress != "2/2" {
		t.Fatalf("progress=%s", f.res.OpsRequest.Status.Progress)
	}
	status.UpToDate = true
	f.res.Cluster.Status.Components["db"] = status
	// Apps can converge before all instance observations arrive.
	original := its.DeepCopy()
	its.Status.InstanceStatus = its.Status.InstanceStatus[:1]
	if err := f.cli.Update(f.req.Ctx, its); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	if f.res.OpsRequest.Status.Progress != "1/2" || len(f.res.OpsRequest.Status.Components["db"].ProgressDetails) != 1 {
		t.Fatalf("missing observation was filled: %+v", f.res.OpsRequest.Status)
	}
	// A cached workload for the previous allocation is also insufficient.
	its.Spec.Replicas = ptr.To[int32](1)
	if err := f.cli.Update(f.req.Ctx, its); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	if f.res.OpsRequest.Status.Progress != "1/1" {
		t.Fatalf("progress=%s", f.res.OpsRequest.Status.Progress)
	}
	its.Spec = original.Spec
	its.Status = original.Status
	its.Status.InstanceStatus[0].Failed = true
	objectKey := getProgressObjectKey(constant.PodKind, its.Status.InstanceStatus[0].PodName)
	if err := f.cli.Update(f.req.Ctx, its); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	failed := findStatusProgressDetail(f.res.OpsRequest.Status.Components["db"].ProgressDetails, objectKey)
	if failed == nil || failed.Status != opsv1alpha1.FailedProgressStatus || failed.EndTime.IsZero() {
		t.Fatalf("failed observation=%+v", failed)
	}
	its.Status.InstanceStatus[0].Failed = false
	its.Status.InstanceStatus[0].Ready = false
	if err := f.cli.Update(f.req.Ctx, its); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	recovering := findStatusProgressDetail(f.res.OpsRequest.Status.Components["db"].ProgressDetails, objectKey)
	if recovering == nil || recovering.Status != opsv1alpha1.ProcessingProgressStatus || !recovering.EndTime.IsZero() {
		t.Fatalf("failed detail did not recover=%+v", recovering)
	}
	its.Status.InstanceStatus[0].Ready = true
	if err := f.cli.Update(f.req.Ctx, its); err != nil {
		t.Fatal(err)
	}
	// Simulate restarting the ops process from its persisted status.
	resumed := &opsv1alpha1.OpsRequest{}
	if err := f.cli.Get(f.req.Ctx, client.ObjectKeyFromObject(f.res.OpsRequest), resumed); err != nil {
		t.Fatal(err)
	}
	f.res.OpsRequest = resumed
	f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
	if f.res.OpsRequest.Status.Progress != "2/2" {
		t.Fatalf("progress=%s", f.res.OpsRequest.Status.Progress)
	}
}

func TestHorizontalScalingBackupPreparationIgnoresOldRunningTopology(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", true))
	f.addBackup(t)
	publishHorizontalScalingResult(t, f)
	if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	details := f.res.OpsRequest.Status.Components["db"].ProgressDetails
	if f.res.OpsRequest.Status.Progress != "0/1" || len(details) != 1 || !strings.HasPrefix(details[0].ObjectKey, "Restore/") {
		t.Fatalf("preparation projected old topology: %+v", f.res.OpsRequest.Status)
	}
	f.replicas(t, "db", 1)
}

func TestHorizontalScalingProgressPatchFailureRetries(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", false))
	if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	publishHorizontalScalingResult(t, f)
	failedPatch := errors.New("status patch failed")
	interrupted := interceptor.NewClient(f.cli.(client.WithWatch), interceptor.Funcs{SubResourcePatch: func(ctx context.Context, cli client.Client, subResource string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
		return failedPatch
	}})
	phase, _, err := (horizontalScalingOpsHandler{}).ReconcileAction(f.req, interrupted, f.res)
	if phase != opsv1alpha1.OpsRunningPhase || !errors.Is(err, failedPatch) {
		t.Fatalf("phase=%s err=%v", phase, err)
	}
	resumed := &opsv1alpha1.OpsRequest{}
	if err := f.cli.Get(f.req.Ctx, client.ObjectKeyFromObject(f.res.OpsRequest), resumed); err != nil {
		t.Fatal(err)
	}
	f.res.OpsRequest = resumed
	f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
}

func TestHorizontalScalingFailureWaitsForOtherAcceptedTargets(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", false), scaleOutRequest("other", false))
	if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	publishHorizontalScalingResult(t, f)
	status := f.res.Cluster.Status.Components["db"]
	status.Phase = appsv1.FailedComponentPhase
	f.res.Cluster.Status.Components["db"] = status
	if err := f.cli.Delete(f.req.Ctx, &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"}}); err != nil {
		t.Fatal(err)
	}
	other := f.res.Cluster.Status.Components["other"]
	other.UpToDate = false
	f.res.Cluster.Status.Components["other"] = other
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	status.Phase = appsv1.RunningComponentPhase
	f.res.Cluster.Status.Components["db"] = status
	other.UpToDate = true
	f.res.Cluster.Status.Components["other"] = other
	if _, err := GetOpsManager().Reconcile(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	if f.res.OpsRequest.Status.Phase != opsv1alpha1.OpsFailedPhase {
		t.Fatalf("terminal branch failure was lost: %s", f.res.OpsRequest.Status.Phase)
	}
}

func TestHorizontalScalingCancellationReobservesFailedBranch(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", false), scaleOutRequest("other", false))
	hs := horizontalScalingOpsHandler{}
	if err := hs.Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	publishHorizontalScalingResult(t, f)
	status := f.res.Cluster.Status.Components["db"]
	status.Phase = appsv1.FailedComponentPhase
	f.res.Cluster.Status.Components["db"] = status
	other := f.res.Cluster.Status.Components["other"]
	other.UpToDate = false
	f.res.Cluster.Status.Components["other"] = other
	if _, err := GetOpsManager().Reconcile(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	if f.res.OpsRequest.Status.Phase != opsv1alpha1.OpsRunningPhase || f.res.OpsRequest.Status.Components["db"].Reason != horizontalScalingFailedReason {
		t.Fatalf("expected a recorded branch failure: %+v", f.res.OpsRequest.Status)
	}
	previous := f.res.OpsRequest.DeepCopy()
	if err := hs.Cancel(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	if err := PatchOpsStatusWithOpsDeepCopy(f.req.Ctx, f.cli, f.res, previous,
		opsv1alpha1.OpsCancellingPhase, opsv1alpha1.NewCancelingCondition(f.res.OpsRequest)); err != nil {
		t.Fatal(err)
	}
	resumed := &opsv1alpha1.OpsRequest{}
	if err := f.cli.Get(f.req.Ctx, client.ObjectKeyFromObject(f.res.OpsRequest), resumed); err != nil {
		t.Fatal(err)
	}
	f.res.OpsRequest = resumed
	publishHorizontalScalingResult(t, f)
	its := &workloads.InstanceSet{}
	if err := f.cli.Get(f.req.Ctx, client.ObjectKey{Namespace: "default", Name: "demo-db"}, its); err != nil {
		t.Fatal(err)
	}
	its.Status.InstanceStatus[0].Ready = false
	if err := f.cli.Update(f.req.Ctx, its); err != nil {
		t.Fatal(err)
	}
	if _, err := GetOpsManager().Reconcile(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	if f.res.OpsRequest.Status.Phase != opsv1alpha1.OpsCancellingPhase {
		t.Fatalf("cancel ended before rollback converged: %s", f.res.OpsRequest.Status.Phase)
	}
	publishHorizontalScalingResult(t, f)
	if _, err := GetOpsManager().Reconcile(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	if f.res.OpsRequest.Status.Phase != opsv1alpha1.OpsCancelledPhase || !slices.ContainsFunc(f.res.OpsRequest.Status.Conditions, func(c metav1.Condition) bool {
		return c.Type == opsv1alpha1.ConditionTypeCancelled && c.Reason == opsv1alpha1.ReasonOpsCancelSucceed
	}) {
		t.Fatalf("rollback did not complete successfully: %+v", f.res.OpsRequest.Status)
	}
}

func TestHorizontalScalingEmptyShardingRequiresObservedResult(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("sharded", false))
	cluster := f.res.Cluster
	cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "sharded", Shards: 0, Template: cluster.Spec.ComponentSpecs[0]}}
	cluster.Spec.ComponentSpecs = nil
	hs := horizontalScalingOpsHandler{}
	if err := hs.SaveLastConfiguration(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	if err := hs.Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{
		"sharded": {ObservedGeneration: cluster.Generation, UpToDate: true, Phase: appsv1.RunningComponentPhase},
	}
	f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
	if f.res.OpsRequest.Status.Progress != "0/0" {
		t.Fatalf("progress=%s", f.res.OpsRequest.Status.Progress)
	}
}

func TestHorizontalScalingWaitsForEveryShardObservation(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("sharded", false))
	cluster := f.res.Cluster
	cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "sharded", Shards: 2, Template: cluster.Spec.ComponentSpecs[0]}}
	cluster.Spec.ComponentSpecs = nil
	cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{
		"sharded": {ObservedGeneration: cluster.Generation, UpToDate: true, Phase: appsv1.RunningComponentPhase},
	}
	for _, name := range []string{"sharded-a", "sharded-b"} {
		comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "demo-" + name, Namespace: "default",
			Labels: constant.GetCompLabels("demo", name, map[string]string{constant.KBAppShardingNameLabelKey: "sharded"})}}
		its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: comp.Name, Namespace: comp.Namespace},
			Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](1)},
			Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{
				PodName: comp.Name + "-chosen", TemplateName: ptr.To(""),
				DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent,
				UpToDate: true, Ready: true, Available: true,
			}}}}
		for _, obj := range []client.Object{comp, its} {
			if err := f.cli.Create(f.req.Ctx, obj); err != nil {
				t.Fatal(err)
			}
		}
		want := opsv1alpha1.OpsRunningPhase
		if name == "sharded-b" {
			want = opsv1alpha1.OpsSucceedPhase
		}
		f.reconcile(t, want)
	}
	if f.res.OpsRequest.Status.Progress != "2/2" {
		t.Fatalf("progress=%s", f.res.OpsRequest.Status.Progress)
	}
}

func TestHorizontalScalingFailedRestoreDoesNotRestartWhilePeersFinish(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("broken", true), scaleOutRequest("healthy", true))
	f.addBackup(t)
	if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	restores := &dpv1alpha1.RestoreList{}
	if err := f.cli.List(f.req.Ctx, restores); err != nil {
		t.Fatal(err)
	}
	if len(restores.Items) != 2 {
		t.Fatalf("restores=%d", len(restores.Items))
	}
	var broken, healthy *dpv1alpha1.Restore
	for i := range restores.Items {
		restore := &restores.Items[i]
		if restore.Labels[constant.KBAppComponentLabelKey] == "broken" {
			broken = restore
		} else {
			healthy = restore
		}
	}
	broken.Status.Phase = dpv1alpha1.RestorePhaseFailed
	if err := f.cli.Status().Update(f.req.Ctx, broken); err != nil {
		t.Fatal(err)
	}
	if _, err := GetOpsManager().Reconcile(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	resumed := &opsv1alpha1.OpsRequest{}
	if err := f.cli.Get(f.req.Ctx, client.ObjectKeyFromObject(f.res.OpsRequest), resumed); err != nil {
		t.Fatal(err)
	}
	if resumed.Status.Phase != opsv1alpha1.OpsRunningPhase || resumed.Status.Components["broken"].Reason != horizontalScalingFailedReason {
		t.Fatalf("failed branch not persisted while peer waits: %+v", resumed.Status)
	}
	f.res.OpsRequest = resumed
	if err := f.cli.Delete(f.req.Ctx, broken); err != nil {
		t.Fatal(err)
	}
	healthy.Status.Phase = dpv1alpha1.RestorePhaseCompleted
	if err := f.cli.Status().Update(f.req.Ctx, healthy); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	f.replicas(t, "broken", 1)
	f.replicas(t, "healthy", 2)
	if err := f.cli.List(f.req.Ctx, restores); err != nil {
		t.Fatal(err)
	}
	if len(restores.Items) != 1 {
		t.Fatal("failed restore was recreated")
	}
	publishHorizontalScalingResult(t, f)
	if _, err := GetOpsManager().Reconcile(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	if f.res.OpsRequest.Status.Phase != opsv1alpha1.OpsFailedPhase {
		t.Fatalf("phase=%s", f.res.OpsRequest.Status.Phase)
	}
}

func TestHorizontalScalingPersistsFailureBeforeLaterRestoreReadError(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("broken", true), scaleOutRequest("healthy", true))
	f.addBackup(t)
	if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	restores := &dpv1alpha1.RestoreList{}
	if err := f.cli.List(f.req.Ctx, restores); err != nil {
		t.Fatal(err)
	}
	var broken *dpv1alpha1.Restore
	for i := range restores.Items {
		if restores.Items[i].Labels[constant.KBAppComponentLabelKey] == "broken" {
			broken = &restores.Items[i]
		}
	}
	if broken == nil {
		t.Fatal("missing broken restore")
	}
	broken.Status.Phase = dpv1alpha1.RestorePhaseFailed
	if err := f.cli.Status().Update(f.req.Ctx, broken); err != nil {
		t.Fatal(err)
	}
	failing := interceptor.NewClient(f.cli.(client.WithWatch), interceptor.Funcs{Get: func(ctx context.Context, cli client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		if _, ok := obj.(*dpv1alpha1.Restore); ok && key.Name != broken.Name {
			return errors.New("injected later Restore read failure")
		}
		return cli.Get(ctx, key, obj, opts...)
	}})
	_, err := GetOpsManager().Reconcile(f.req, failing, f.res)
	if err == nil || !strings.Contains(err.Error(), "injected later") {
		t.Fatalf("unexpected error: %v", err)
	}
	reloaded := &opsv1alpha1.OpsRequest{}
	if err := f.cli.Get(f.req.Ctx, client.ObjectKeyFromObject(f.res.OpsRequest), reloaded); err != nil {
		t.Fatal(err)
	}
	if reloaded.Status.Components["broken"].Reason != horizontalScalingFailedReason {
		t.Fatalf("failed branch was not persisted: %+v", reloaded.Status)
	}
	f.res.OpsRequest = reloaded
	if err := f.cli.Delete(f.req.Ctx, broken); err != nil {
		t.Fatal(err)
	}
	if _, err := GetOpsManager().Reconcile(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	if err := f.cli.Get(f.req.Ctx, client.ObjectKeyFromObject(broken), &dpv1alpha1.Restore{}); err == nil {
		t.Fatal("failed Restore was recreated")
	}
}

func TestHorizontalScalingCancellationCompletesAfterForwardTimeout(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", true))
	f.addBackup(t)
	if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	f.res.OpsRequest.Spec.TimeoutSeconds = ptr.To(int32(1))
	if err := f.cli.Update(f.req.Ctx, f.res.OpsRequest); err != nil {
		t.Fatal(err)
	}
	previous := f.res.OpsRequest.DeepCopy()
	if err := (horizontalScalingOpsHandler{}).Cancel(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	f.res.OpsRequest.Status.StartTimestamp = metav1.NewTime(time.Now().Add(-time.Minute))
	if err := PatchOpsStatusWithOpsDeepCopy(f.req.Ctx, f.cli, f.res, previous, opsv1alpha1.OpsCancellingPhase, opsv1alpha1.NewCancelingCondition(f.res.OpsRequest)); err != nil {
		t.Fatal(err)
	}
	publishHorizontalScalingResult(t, f)
	if _, err := GetOpsManager().Reconcile(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	reloaded := &opsv1alpha1.OpsRequest{}
	if err := f.cli.Get(f.req.Ctx, client.ObjectKeyFromObject(f.res.OpsRequest), reloaded); err != nil {
		t.Fatal(err)
	}
	if reloaded.Status.Phase != opsv1alpha1.OpsCancelledPhase {
		t.Fatalf("completed rollback was aborted: %s", reloaded.Status.Phase)
	}
}

func TestHorizontalScalingTimeoutDoesNotDispatchBackupWork(t *testing.T) {
	for _, started := range []bool{false, true} {
		t.Run(fmt.Sprintf("restore-started-%t", started), func(t *testing.T) {
			f := newHorizontalScalingFixture(t, scaleOutRequest("db", true))
			f.addBackup(t)
			if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			if started {
				f.reconcile(t, opsv1alpha1.OpsRunningPhase)
				restores := &dpv1alpha1.RestoreList{}
				if err := f.cli.List(f.req.Ctx, restores); err != nil {
					t.Fatal(err)
				}
				for i := range restores.Items {
					restores.Items[i].Status.Phase = dpv1alpha1.RestorePhaseCompleted
					if err := f.cli.Status().Update(f.req.Ctx, &restores.Items[i]); err != nil {
						t.Fatal(err)
					}
				}
			}
			f.res.OpsRequest.Spec.TimeoutSeconds = ptr.To[int32](1)
			if err := f.cli.Update(f.req.Ctx, f.res.OpsRequest); err != nil {
				t.Fatal(err)
			}
			f.res.OpsRequest.Status.StartTimestamp = metav1.NewTime(time.Now().Add(-time.Minute))
			if err := f.cli.Status().Update(f.req.Ctx, f.res.OpsRequest); err != nil {
				t.Fatal(err)
			}
			if _, err := GetOpsManager().Reconcile(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			if f.res.OpsRequest.Status.Phase != opsv1alpha1.OpsAbortedPhase {
				t.Fatalf("phase=%s", f.res.OpsRequest.Status.Phase)
			}
			f.replicas(t, "db", 1)
			restores := &dpv1alpha1.RestoreList{}
			if err := f.cli.List(f.req.Ctx, restores); err != nil {
				t.Fatal(err)
			}
			if !started && len(restores.Items) != 0 || f.clusterWrites != 1 {
				t.Fatal("expired request dispatched new backup work or topology")
			}
		})
	}
}
