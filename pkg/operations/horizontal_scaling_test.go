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
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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

// Publish both assignments before taking 7 offline so its retained template is
// supplied by the workload producer, not inferred or fabricated by the test.
func prepareHScaleIdentitySwap(t *testing.T, flat bool) *horizontalScalingFixture {
	t.Helper()
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", false))
	spec := &f.res.Cluster.Spec.ComponentSpecs[0]
	spec.FlatInstanceOrdinal, spec.Replicas = flat, 2
	spec.Ordinals.Discrete = []int32{3, 7}
	f.publishAllocation(t, "db")
	spec.Replicas, spec.OfflineInstances = 1, []string{"demo-db-7"}
	createHScaleAlignedPod(t, f, f.publishAllocation(t, "db"), "demo-db-3", true)
	f.publishAllocation(t, "db")
	request := &f.res.OpsRequest.Spec.HorizontalScalingList[0]
	request.ScaleIn = &opsv1alpha1.ScaleIn{OnlineInstancesToOffline: []string{"demo-db-3"}}
	request.ScaleOut = &opsv1alpha1.ScaleOut{OfflineInstancesToOnline: []string{"demo-db-7"}}
	if err := f.cli.Update(f.req.Ctx, f.res.OpsRequest); err != nil {
		t.Fatal(err)
	}
	f.saveConfiguration(t)
	if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	return f
}

func createHScaleAlignedPod(t *testing.T, f *horizontalScalingFixture, tree *kubebuilderx.ObjectTree, name string, ready bool) *corev1.Pod {
	t.Helper()
	if _, err := instanceset.NewReplicasAlignmentReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	for _, obj := range tree.List(&corev1.Pod{}) {
		pod := obj.(*corev1.Pod)
		if pod.Name != name {
			continue
		}
		pod.Status.Phase = corev1.PodPending
		if ready {
			pod.Status.Phase = corev1.PodRunning
			pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		}
		if err := f.cli.Create(f.req.Ctx, pod); err != nil {
			t.Fatal(err)
		}
		return pod
	}
	t.Fatalf("allocator did not create %s", name)
	return nil
}

func TestHScaleExplicitIdentitySwap(t *testing.T) {
	for _, flat := range []bool{false, true} {
		t.Run(fmt.Sprintf("flat=%t", flat), func(t *testing.T) {
			f := prepareHScaleIdentitySwap(t, flat)
			// Equal counts do not make the old allocation the requested target.
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			if flat {
				writes := f.clusterWrites
				if err := (horizontalScalingOpsHandler{}).Cancel(f.req, f.cli, f.res); !intctrlutil.IsTargetError(err, intctrlutil.ErrorIgnoreCancel) {
					t.Fatalf("cancel = %v", err)
				}
				if writes != f.clusterWrites {
					t.Fatal("rejected cancel modified the forward target")
				}
			}
			pod := createHScaleAlignedPod(t, f, f.publishAllocation(t, "db"), "demo-db-7", false)
			f.publishAllocation(t, "db")
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			if f.res.OpsRequest.Status.Progress != "0/2" {
				t.Fatalf("swap progress = %s", f.res.OpsRequest.Status.Progress)
			}
			if err := f.cli.Delete(f.req.Ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "demo-db-3"}}); err != nil {
				t.Fatal(err)
			}
			f.publishAllocation(t, "db")
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			pod.Status.Phase = corev1.PodRunning
			pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			if err := f.cli.Status().Update(f.req.Ctx, pod); err != nil {
				t.Fatal(err)
			}
			f.publishAllocation(t, "db")
			f.reconcile(t, opsv1alpha1.OpsSucceedPhase) // A rejected flat cancel does not stop forward progress.
			if f.res.OpsRequest.Status.Progress != "2/2" {
				t.Fatalf("completed swap progress = %s", f.res.OpsRequest.Status.Progress)
			}
		})
	}
}

func TestHScaleNonFlatSwapCancellation(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprintf("restored-instance-failed=%t", failed), func(t *testing.T) {
			f := prepareHScaleIdentitySwap(t, false)
			pod7 := createHScaleAlignedPod(t, f, f.publishAllocation(t, "db"), "demo-db-7", false)
			if err := f.cli.Delete(f.req.Ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "demo-db-3"}}); err != nil {
				t.Fatal(err)
			}
			// An unrelated Released Pod must not participate in rollback.
			unrelated := pod7.DeepCopy()
			unrelated.Name, unrelated.ResourceVersion, unrelated.UID = "demo-db-99", "", ""
			if err := f.cli.Create(f.req.Ctx, unrelated); err != nil {
				t.Fatal(err)
			}
			f.publishAllocation(t, "db")
			if len(f.res.OpsRequest.Status.Components) != 0 {
				t.Fatal("test requires cancellation before forward progress was recorded")
			}
			if err := (horizontalScalingOpsHandler{}).Cancel(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			f.res.OpsRequest.Status.Phase = opsv1alpha1.OpsCancellingPhase
			if err := f.cli.Status().Update(f.req.Ctx, f.res.OpsRequest); err != nil {
				t.Fatal(err)
			}
			f.publishAllocation(t, "db")
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			if err := f.cli.Delete(f.req.Ctx, pod7); err != nil {
				t.Fatal(err)
			}
			pod3 := createHScaleAlignedPod(t, f, f.publishAllocation(t, "db"), "demo-db-3", false)
			f.publishAllocation(t, "db")
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			want := opsv1alpha1.OpsSucceedPhase
			pod3.Status.Phase = corev1.PodRunning
			pod3.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			if failed {
				pod3.Status.Phase, pod3.Status.Conditions, want = corev1.PodFailed, nil, opsv1alpha1.OpsFailedPhase
			}
			if err := f.cli.Status().Update(f.req.Ctx, pod3); err != nil {
				t.Fatal(err)
			}
			f.publishAllocation(t, "db")
			f.reconcile(t, want)
			if f.res.OpsRequest.Status.Progress != "2/2" {
				t.Fatalf("rollback progress = %s", f.res.OpsRequest.Status.Progress)
			}
			for _, detail := range f.res.OpsRequest.Status.Components["db"].ProgressDetails {
				if detail.ObjectKey == "Pod/demo-db-99" {
					t.Fatal("unrelated Released instance entered rollback")
				}
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
	specs, err := hscaleComponentSpecs(f.req, f.cli, f.res, "group")
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
	specs, err = hscaleComponentSpecs(f.req, f.cli, f.res, "group")
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
		f := newHorizontalScalingFixture(t, scaleOutRequest("ordinary", false), scaleOutRequest("db", backup))
		f.res.Cluster.Spec.ComponentSpecs[1].FlatInstanceOrdinal = true
		hs := horizontalScalingOpsHandler{}
		if backup {
			if err := hs.Action(f.req, f.cli, f.res); !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
				t.Fatalf("backup rejection = %v", err)
			}
		} else {
			f.publishAllocation(t, "ordinary")
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

func prepareHScaleTemplateReassignment(t *testing.T) (*horizontalScalingFixture, *kubebuilderx.ObjectTree) {
	t.Helper()
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", false))
	spec := &f.res.Cluster.Spec.ComponentSpecs[0]
	spec.FlatInstanceOrdinal, spec.Replicas = true, 2
	tree := f.publishAllocation(t, "db")
	its := tree.GetRoot().(*workloads.InstanceSet)
	its.Spec.MinReadySeconds = 60
	if err := f.cli.Update(f.req.Ctx, its); err != nil {
		t.Fatal(err)
	}
	definition := &appsv1.ComponentDefinition{}
	if err := f.cli.Get(f.req.Ctx, client.ObjectKey{Name: "database"}, definition); err != nil {
		t.Fatal(err)
	}
	definition.Spec.Roles = []appsv1.ReplicaRole{{Name: "leader"}}
	if err := f.cli.Update(f.req.Ctx, definition); err != nil {
		t.Fatal(err)
	}
	if _, err := instanceset.NewReplicasAlignmentReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	for _, obj := range tree.List(&corev1.Pod{}) {
		pod := obj.(*corev1.Pod)
		pod.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Hour))
		pod.Labels[constant.RoleLabelKey] = "leader"
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour))}}
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
	return f, f.publishAllocation(t, "db")
}

func TestHScaleTemplateReassignment(t *testing.T) {
	f, tree := prepareHScaleTemplateReassignment(t)
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
	pod.Finalizers = []string{"test.kubeblocks.io/hold-deletion"}
	if err := f.cli.Update(f.req.Ctx, pod); err != nil {
		t.Fatal(err)
	}
	if err := f.cli.Delete(f.req.Ctx, pod); err != nil {
		t.Fatal(err)
	}
	tree = f.publishAllocation(t, "db")
	if status := tree.GetRoot().(*workloads.InstanceSet).FindInstanceStatus(key.Name); status == nil || status.CurrentState != workloads.InstanceCurrentStateTerminating {
		t.Fatalf("expected terminating reassignment: %+v", status)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	if err := f.cli.Get(f.req.Ctx, key, pod); err != nil {
		t.Fatal(err)
	}
	pod.Finalizers = nil
	if err := f.cli.Update(f.req.Ctx, pod); err != nil {
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
	pod.Labels[constant.RoleLabelKey] = "leader"
	if err := f.cli.Update(f.req.Ctx, pod); err != nil {
		t.Fatal(err)
	}
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
	if err := f.cli.Status().Update(f.req.Ctx, pod); err != nil {
		t.Fatal(err)
	}
	tree = f.publishAllocation(t, "db")
	status := tree.GetRoot().(*workloads.InstanceSet).FindInstanceStatus(key.Name)
	if status == nil || !status.UpToDate || !status.Ready || status.Available {
		t.Fatalf("expected applied and Ready, but not Available: %+v", status)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	pod.Status.Conditions[0].LastTransitionTime = metav1.NewTime(time.Now().Add(-time.Hour))
	if err := f.cli.Status().Update(f.req.Ctx, pod); err != nil {
		t.Fatal(err)
	}
	f.publishAllocation(t, "db")
	f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
	if got := f.res.OpsRequest.Status.Progress; got != "1/1" {
		t.Fatalf("final progress = %s", got)
	}
	details := f.res.OpsRequest.Status.Components["db"].ProgressDetails
	if len(details) != 1 || details[0].Group != "db/Update" || details[0].ObjectKey != "Pod/demo-db-1" {
		t.Fatalf("unexpected update details: %+v", details)
	}
}

func TestHScaleUnappliedTemplateFailure(t *testing.T) {
	f, _ := prepareHScaleTemplateReassignment(t)
	pod := &corev1.Pod{}
	if err := f.cli.Get(f.req.Ctx, client.ObjectKey{Namespace: "default", Name: "demo-db-1"}, pod); err != nil {
		t.Fatal(err)
	}
	pod.Status.Conditions = []corev1.PodCondition{
		{Type: corev1.PodReady, Status: corev1.ConditionFalse},
		{Type: corev1.ContainersReady, Status: corev1.ConditionFalse, LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour))},
	}
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "mysql", State: corev1.ContainerState{
		Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff", Message: "container failed"},
	}}}
	if err := f.cli.Status().Update(f.req.Ctx, pod); err != nil {
		t.Fatal(err)
	}
	tree := f.publishAllocation(t, "db")
	status := tree.GetRoot().(*workloads.InstanceSet).FindInstanceStatus(pod.Name)
	if status == nil || !status.Failed || status.UpToDate {
		t.Fatalf("expected failed old configuration: %+v", status)
	}
	f.res.OpsRequest.Status.StartTimestamp = metav1.Now()
	f.reconcile(t, opsv1alpha1.OpsFailedPhase)
	details := f.res.OpsRequest.Status.Components["db"].ProgressDetails
	if len(details) != 1 || details[0].Group != "db/Update" || details[0].Status != opsv1alpha1.FailedProgressStatus {
		t.Fatalf("unapplied failure was hidden: %+v", details)
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

// Use the real naming and workload runtime; only API storage is in memory.
type horizontalScalingFixture struct {
	cli                                      client.Client
	res                                      *OpsResource
	req                                      intctrlutil.RequestCtx
	clusterWrites, backupReads, restoreReads int
}

// Observe the real runtime and inject failures only to check short-circuit order.
type horizontalScalingRuntimeTrace struct {
	OpsRuntime
	calls  []string
	specs  []appsv1.ClusterComponentSpec
	failAt int
	err    error
}

func (r *horizontalScalingRuntimeTrace) GenerateInstanceNameSet(clusterName, compName string, replicas int32,
	instances []appsv1.InstanceTemplate, offline []string) (map[string]string, error) {
	r.calls = append(r.calls, fmt.Sprintf("names(%s,%d)", compName, replicas))
	spec := appsv1.ClusterComponentSpec{Name: compName, Replicas: replicas, Instances: instances, OfflineInstances: offline}
	r.specs = append(r.specs, *spec.DeepCopy())
	if r.failAt > 0 && len(r.calls) == r.failAt {
		return nil, r.err
	}
	return r.OpsRuntime.GenerateInstanceNameSet(clusterName, compName, replicas, instances, offline)
}

func (r *horizontalScalingRuntimeTrace) GetWorkload(namespace, clusterName, compName string) (Workload, error) {
	r.calls = append(r.calls, "workload("+compName+")")
	return r.OpsRuntime.GetWorkload(namespace, clusterName, compName)
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
		WithStatusSubresource(ops, &dpv1alpha1.Restore{}, &workloads.InstanceSet{}).WithInterceptorFuncs(interceptor.Funcs{
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
	if len(requests) == 1 && hscaleFromBackup(requests[0]) {
		f.saveConfiguration(t)
	}
	return f
}

func (f *horizontalScalingFixture) saveConfiguration(t *testing.T) {
	t.Helper()
	if err := (horizontalScalingOpsHandler{}).SaveLastConfiguration(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	if err := f.cli.Status().Update(f.req.Ctx, f.res.OpsRequest); err != nil {
		t.Fatal(err)
	}
}

func (f *horizontalScalingFixture) publishAllocation(t *testing.T, name string) *kubebuilderx.ObjectTree {
	t.Helper()
	spec := f.res.Cluster.Spec.GetComponentByName(name)
	tree, err := publishHScaleAllocation(f.req.Ctx, f.cli, f.res.Cluster, name, spec)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func scaleOutRequest(name string, backup bool) opsv1alpha1.HorizontalScaling {
	r := opsv1alpha1.HorizontalScaling{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: name},
		ScaleOut: &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}}
	if backup {
		r.ScaleOut.FromBackup = &opsv1alpha1.FromBackup{Name: "snapshot"}
	}
	return r
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
		t.Fatalf("reconcile phase = %s, want %s; progress=%s details=%+v", phase, want, f.res.OpsRequest.Status.Progress, f.res.OpsRequest.Status.Components)
	}
}

func TestHorizontalScalingOrdinaryPathDoesNotRestore(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", false))
	f.publishAllocation(t, "db")
	f.saveConfiguration(t)
	trace := &horizontalScalingRuntimeTrace{OpsRuntime: f.res.Runtimes["db"], failAt: 1, err: errors.New("ordinary scaling must not plan names")}
	f.res.Runtimes["db"] = trace
	if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	f.replicas(t, "db", 2)
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	if len(f.res.OpsRequest.Status.Components["db"].ProgressDetails) != 0 {
		t.Fatal("invented participants before allocation publication")
	}
	f.publishAllocation(t, "db")
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	if len(trace.specs) != 0 {
		t.Fatal("ordinary scaling used name planning")
	}
	if f.backupReads != 0 || f.restoreReads != 0 || f.clusterWrites != 1 {
		t.Fatalf("ordinary path API calls: backup=%d restore=%d cluster writes=%d", f.backupReads, f.restoreReads, f.clusterWrites)
	}
	details := f.res.OpsRequest.Status.Components["db"].ProgressDetails
	if f.res.OpsRequest.Status.Progress != "0/1" || len(details) != 1 || details[0].ObjectKey != "Pod/demo-db-1" || details[0].Status != opsv1alpha1.PendingProgressStatus {
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
			f.publishAllocation(t, "ordinary")
			f.saveConfiguration(t)
			f.addBackup(t)
			if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			f.replicas(t, "ordinary", 2)
			f.replicas(t, "restored", 1)
			f.publishAllocation(t, "ordinary")
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
			for _, name := range []string{"ordinary", "restored"} {
				pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-" + name + "-1", Namespace: "default",
					Labels: constant.GetCompLabels("demo", name)},
					Status: corev1.PodStatus{Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}}
				if err := f.cli.Create(f.req.Ctx, pod); err != nil {
					t.Fatal(err)
				}
			}
			f.publishAllocation(t, "ordinary")
			f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
			if got := f.res.OpsRequest.Status.Progress; got != "2/2" {
				t.Fatalf("completed progress = %q", got)
			}
			for _, name := range []string{"ordinary", "restored"} {
				details := f.res.OpsRequest.Status.Components[name].ProgressDetails
				if len(details) != 1 || details[0].ObjectKey != "Pod/demo-"+name+"-1" || details[0].Status != opsv1alpha1.SucceedProgressStatus {
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

func TestHorizontalScalingCancelRestoresConfigurationAndDirection(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", false))
	spec := &f.res.Cluster.Spec.ComponentSpecs[0]
	spec.Instances = []appsv1.InstanceTemplate{{Name: "foo", Replicas: pointer.Int32(1)}}
	spec.OfflineInstances = []string{"demo-db-foo-0"}
	f.publishAllocation(t, "db")
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
	f.publishAllocation(t, "db")
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	details := f.res.OpsRequest.Status.Components["db"].ProgressDetails
	if f.res.OpsRequest.Status.Progress != "0/1" || len(details) != 1 || details[0].ObjectKey != "Pod/demo-db-0" ||
		details[0].Status != opsv1alpha1.PendingProgressStatus || !strings.Contains(details[0].Message, "create") {
		t.Fatalf("unexpected create participants: %+v", f.res.OpsRequest.Status)
	}
	f.res.OpsRequest.Status.Phase = opsv1alpha1.OpsCancellingPhase
	if err := hs.Cancel(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	if spec.Replicas != *last.Replicas || !reflect.DeepEqual(spec.Instances, last.Instances) || !reflect.DeepEqual(spec.OfflineInstances, last.OfflineInstances) {
		t.Fatal("Cancel did not restore the original configuration")
	}
	// ReconcileAction reports successful rollback; OpsManager maps it to Cancelled.
	f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
	details = f.res.OpsRequest.Status.Components["db"].ProgressDetails
	if f.res.OpsRequest.Status.Progress != "1/1" || len(details) != 1 || details[0].ObjectKey != "Pod/demo-db-0" || !strings.Contains(details[0].Message, "delete") {
		t.Fatalf("unexpected cancel progress: %+v", f.res.OpsRequest.Status)
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
				if !changeCount {
					if _, err := publishHScaleAllocation(f.req.Ctx, f.cli, cluster, shard, &cluster.Spec.Shardings[0].Template); err != nil {
						t.Fatal(err)
					}
				}
			}
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
			wantShards, wantReplicas, wantProgress := int32(2), int32(2), "0/2"
			if changeCount {
				wantShards, wantReplicas, wantProgress = 3, 1, "0/1"
			}
			if got := cluster.Spec.Shardings[0]; got.Shards != wantShards || got.Template.Replicas != wantReplicas {
				t.Fatalf("unexpected sharding: %+v", got)
			}
			if !changeCount {
				for _, shard := range []string{"sharded-a", "sharded-b"} {
					if _, err := publishHScaleAllocation(f.req.Ctx, f.cli, cluster, shard, &cluster.Spec.Shardings[0].Template); err != nil {
						t.Fatal(err)
					}
				}
			}
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			if got := f.res.OpsRequest.Status.Progress; got != wantProgress {
				t.Fatalf("progress = %s, want %s", got, wantProgress)
			}
			details := f.res.OpsRequest.Status.Components["sharded"].ProgressDetails
			if len(details) != 2 {
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
