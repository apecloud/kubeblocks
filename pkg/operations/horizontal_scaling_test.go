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

	apps "k8s.io/api/apps/v1"
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

func TestHScaleCancellationBoundary(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sharding *appsv1.ClusterSharding
		allowed  bool
	}{
		{name: "mixed components"},
		{name: "flat common shard template", sharding: &appsv1.ClusterSharding{Name: "flat", Shards: 2,
			Template: appsv1.ClusterComponentSpec{FlatInstanceOrdinal: true}}},
		{name: "flat shard override", sharding: &appsv1.ClusterSharding{Name: "flat", Shards: 2,
			ShardTemplates: []appsv1.ShardTemplate{{Name: "a", Shards: pointer.Int32(1), FlatInstanceOrdinal: pointer.Bool(true)}}}},
		{name: "all shards override to non-flat", allowed: true, sharding: &appsv1.ClusterSharding{Name: "flat", Shards: 2,
			Template:       appsv1.ClusterComponentSpec{FlatInstanceOrdinal: true},
			ShardTemplates: []appsv1.ShardTemplate{{Name: "a", Shards: pointer.Int32(2), FlatInstanceOrdinal: pointer.Bool(false)}}}},
		{name: "unused flat shard template", allowed: true, sharding: &appsv1.ClusterSharding{Name: "flat", Shards: 2,
			ShardTemplates: []appsv1.ShardTemplate{{Name: "a", Shards: pointer.Int32(0), FlatInstanceOrdinal: pointer.Bool(true)}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{
				{Name: "ordinary", Replicas: 3},
			}}}
			if tc.sharding != nil {
				cluster.Spec.Shardings = []appsv1.ClusterSharding{*tc.sharding}
			} else {
				cluster.Spec.ComponentSpecs = append(cluster.Spec.ComponentSpecs, appsv1.ClusterComponentSpec{Name: "flat", Replicas: 3, FlatInstanceOrdinal: true})
			}
			ops := &opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{Cancel: true,
				SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{HorizontalScalingList: []opsv1alpha1.HorizontalScaling{
					{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "ordinary"}},
					{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "flat"}},
				}}}, Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase,
				LastConfiguration: opsv1alpha1.LastConfiguration{Components: map[string]opsv1alpha1.LastComponentConfiguration{
					"ordinary": {Replicas: pointer.Int32(2)}, "flat": {Replicas: pointer.Int32(2)},
				}}}}
			res := &OpsResource{Cluster: cluster, OpsRequest: ops}
			h := horizontalScalingOpsHandler{}
			if tc.allowed {
				if err := h.validateCancellation(res); err != nil {
					t.Fatalf("non-flat shards must remain cancellable: %v", err)
				}
				return
			}
			beforeCluster, beforeOps := cluster.DeepCopy(), ops.DeepCopy()
			// A nil client verifies that rejection happens before any write,
			// including rollback of the earlier ordinary component in this request.
			err := h.Cancel(intctrlutil.RequestCtx{Ctx: context.Background()}, nil, res)
			if !intctrlutil.IsTargetError(err, intctrlutil.ErrorIgnoreCancel) {
				t.Fatalf("want IgnoreCancel, got %v", err)
			}
			if !reflect.DeepEqual(beforeCluster, cluster) || !reflect.DeepEqual(beforeOps, ops) {
				t.Fatal("rejected cancellation changed configuration or operation state")
			}
			ops.Status.Phase = opsv1alpha1.OpsCancellingPhase
			if _, _, err := h.ReconcileAction(intctrlutil.RequestCtx{Ctx: context.Background()}, nil, res); !intctrlutil.IsTargetError(err, intctrlutil.ErrorIgnoreCancel) {
				t.Fatalf("unexpected flat rollback execution: %v", err)
			}
		})
	}
}

func TestHorizontalDiff(t *testing.T) {
	source := map[string]string{"demo-0": "", "demo-1": "big"}
	target := map[string]string{"demo-0": "", "demo-2": "big"}
	created, deleted, updated := diffAssignments(source, target)
	if len(updated) != 0 {
		t.Fatalf("unexpected template changes: %#v", updated)
	}
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
	target["demo-0"] = "reader"
	created, deleted, updated = diffAssignments(source, target)
	if len(created) != 1 || len(deleted) != 1 || len(updated) != 1 || updated["demo-0"] != "reader" {
		t.Fatalf("template reassignment must be an update, not creation/deletion: %v %v %v", created, deleted, updated)
	}
}

func TestHScaleRejectsActiveOfflineAssignments(t *testing.T) {
	comp := &appsv1.ClusterComponentSpec{Replicas: 2, OfflineInstances: []string{"demo-0"}}
	if assignmentsMatchComponent(map[string]string{"demo-0": "", "demo-1": ""}, comp) {
		t.Fatal("matching replica counts must not accept an explicitly offline instance as Active")
	}
	if !assignmentsMatchComponent(map[string]string{"demo-1": "", "demo-2": ""}, comp) {
		t.Fatal("expected the active allocation to match")
	}
}

func TestHScaleShardOverrides(t *testing.T) {
	for _, flat := range []bool{false, true} {
		t.Run(fmt.Sprintf("flat=%t", flat), func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, workloads.AddToScheme, opsv1alpha1.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
				Spec: appsv1.ClusterSpec{Shardings: []appsv1.ClusterSharding{{Name: "db", Shards: 2,
					Template: appsv1.ClusterComponentSpec{ComponentDef: "database", Replicas: 1, FlatInstanceOrdinal: flat},
					ShardTemplates: []appsv1.ShardTemplate{{Name: "large", Shards: pointer.Int32(1), Replicas: pointer.Int32(2),
						Instances: []appsv1.InstanceTemplate{{Name: "reader", Replicas: pointer.Int32(1)}}}}}}},
				Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase,
					Shardings: map[string]appsv1.ClusterShardingStatus{"db": {Phase: appsv1.RunningComponentPhase}}}}
			ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "sharding", Namespace: "default"},
				Spec: opsv1alpha1.OpsRequestSpec{Type: opsv1alpha1.HorizontalScalingType,
					SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{HorizontalScalingList: []opsv1alpha1.HorizontalScaling{{
						ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"},
						ScaleOut:     &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}}}}},
				Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase}}
			objects := []client.Object{cluster, ops, &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "database"}}}
			var common *workloads.InstanceSet
			for _, name := range []string{"db-a", "db-b"} {
				labels := constant.GetCompLabels("demo", name)
				labels[constant.KBAppShardingNameLabelKey] = "db"
				its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-" + name, Namespace: "default"},
					Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{
						PodName: "demo-" + name + "-0", TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive}},
						CurrentRevisions: map[string]string{"demo-" + name + "-0": "r"}}}
				if name == "db-a" {
					labels[constant.KBAppShardTemplateLabelKey] = "large"
					readerName := "demo-db-a-reader-0"
					if flat {
						readerName = "demo-db-a-7"
					}
					its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
						PodName: readerName, TemplateName: templateName("reader"), DesiredState: workloads.InstanceDesiredStateActive})
					its.Status.CurrentRevisions[readerName] = "r"
				} else {
					common = its
				}
				objects = append(objects, its, &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: its.Name, Namespace: "default", Labels: labels}})
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops, &workloads.InstanceSet{}).WithObjects(objects...).Build()
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
			check := func(want opsv1alpha1.OpsPhase) {
				t.Helper()
				phase, _, err := h.ReconcileAction(req, cli, res)
				if err != nil || phase != want {
					t.Fatalf("phase=%s progress=%s err=%v, want %s", phase, ops.Status.Progress, err, want)
				}
			}
			check(opsv1alpha1.OpsRunningPhase)
			// The common shard scales; the overridden shard keeps its own
			// replica count and reader assignment, and must not block progress.
			common.Status.InstanceStatus = append(common.Status.InstanceStatus, workloads.InstanceStatus{
				PodName: "demo-db-b-1", TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive})
			common.Status.CurrentRevisions["demo-db-b-1"] = "r"
			if err := cli.Status().Update(ctx, common); err != nil {
				t.Fatal(err)
			}
			check(opsv1alpha1.OpsSucceedPhase)
			if flat {
				return
			}
			if err := h.Cancel(req, cli, res); err != nil {
				t.Fatal(err)
			}
			ops.Status.Phase = opsv1alpha1.OpsCancellingPhase
			if err := cli.Status().Update(ctx, ops); err != nil {
				t.Fatal(err)
			}
			check(opsv1alpha1.OpsRunningPhase)
			delete(common.Status.CurrentRevisions, "demo-db-b-1")
			if err := cli.Status().Update(ctx, common); err != nil {
				t.Fatal(err)
			}
			check(opsv1alpha1.OpsSucceedPhase)
		})
	}
}

// Use the workload allocator and status producer, not a hand-written target
// allocation, to exercise a template taking over an existing flat ordinal.
func TestHScaleTemplateReassignment(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, workloads.AddToScheme, opsv1alpha1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	comp := appsv1.ClusterComponentSpec{Name: "db", ComponentDef: "database", Replicas: 2, FlatInstanceOrdinal: true}
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{comp}},
		Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase,
			Components: map[string]appsv1.ClusterComponentStatus{"db": {Phase: appsv1.RunningComponentPhase}}}}
	reader := appsv1.InstanceTemplate{Name: "reader", Replicas: pointer.Int32(1),
		Ordinals: appsv1.Ordinals{Discrete: []int32{1}}, Annotations: map[string]string{"template": "reader"}}
	ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "reassign", Namespace: "default"},
		Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
			HorizontalScalingList: []opsv1alpha1.HorizontalScaling{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"},
				ScaleIn:  &opsv1alpha1.ScaleIn{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}},
				ScaleOut: &opsv1alpha1.ScaleOut{NewInstances: []appsv1.InstanceTemplate{reader}}}}}},
		Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase}}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default", Generation: 1},
		Spec: workloads.InstanceSetSpec{Replicas: pointer.Int32(2), FlatInstanceOrdinal: true,
			MinReadySeconds:     60,
			PodManagementPolicy: apps.ParallelPodManagement,
			Selector:            &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
			Template:            corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "mysql:8"}}}}}}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	run := func(reconciler kubebuilderx.Reconciler) {
		t.Helper()
		if _, err := reconciler.Reconcile(tree); err != nil {
			t.Fatal(err)
		}
	}
	ready := func(pod *corev1.Pod, condition corev1.ConditionStatus) {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: condition,
			LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour))}}
	}
	run(instanceset.NewRevisionUpdateReconciler())
	run(instanceset.NewReplicasAlignmentReconciler())
	for _, obj := range tree.List(&corev1.Pod{}) {
		ready(obj.(*corev1.Pod), corev1.ConditionTrue)
	}
	run(instanceset.NewStatusReconciler())
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
	target := cluster.Spec.ComponentSpecs[0]
	its.Spec.Replicas = &target.Replicas
	its.Spec.Instances = []workloads.InstanceTemplate{{Name: reader.Name, Replicas: reader.Replicas,
		Ordinals: reader.Ordinals, Annotations: reader.Annotations}}
	its.Generation++
	if err := cli.Update(ctx, its); err != nil {
		t.Fatal(err)
	}
	run(instanceset.NewRevisionUpdateReconciler())
	check := func(want opsv1alpha1.OpsPhase, progress string, applied, healthy bool) {
		t.Helper()
		run(instanceset.NewStatusReconciler())
		status := its.FindInstanceStatus("demo-db-1")
		if status == nil || status.TemplateName == nil || *status.TemplateName != "reader" || status.UpToDate != applied || status.Ready != healthy {
			t.Fatalf("unexpected workload observation: %#v", status)
		}
		if err := cli.Status().Update(ctx, its); err != nil {
			t.Fatal(err)
		}
		phase, _, err := h.ReconcileAction(req, cli, res)
		if err != nil || phase != want || ops.Status.Progress != progress {
			t.Fatalf("phase=%s progress=%s err=%v, want %s %s", phase, ops.Status.Progress, err, want, progress)
		}
	}
	check(opsv1alpha1.OpsRunningPhase, "0/1", false, true) // Old Pod is healthy but still uses the default template.
	obj, err := tree.Get(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-db-1", Namespace: "default"}})
	if err != nil {
		t.Fatal(err)
	}
	pod := obj.(*corev1.Pod)
	ready(pod, corev1.ConditionFalse)
	check(opsv1alpha1.OpsRunningPhase, "0/1", false, false)
	deleting := metav1.Now()
	pod.DeletionTimestamp = &deleting
	check(opsv1alpha1.OpsRunningPhase, "0/1", false, false)
	// Recreate through alignment so revision and configuration are produced by
	// the workload controller. No InstanceStatus fields are patched by this test.
	if err := tree.Delete(pod); err != nil {
		t.Fatal(err)
	}
	check(opsv1alpha1.OpsRunningPhase, "0/1", false, false)
	run(instanceset.NewReplicasAlignmentReconciler())
	obj, err = tree.Get(pod)
	if err != nil {
		t.Fatal(err)
	}
	pod = obj.(*corev1.Pod)
	ready(pod, corev1.ConditionFalse)
	check(opsv1alpha1.OpsRunningPhase, "0/1", true, false)
	ready(pod, corev1.ConditionTrue)
	pod.Status.Conditions[0].LastTransitionTime = metav1.Now()
	check(opsv1alpha1.OpsRunningPhase, "0/1", true, true) // Ready but not yet Available.
	ready(pod, corev1.ConditionTrue)
	check(opsv1alpha1.OpsSucceedPhase, "1/1", true, true)
	details := ops.Status.Components["db"].ProgressDetails
	if len(details) != 1 || details[0].Group != "db/Update" || details[0].ObjectKey != "Pod/demo-db-1" {
		t.Fatalf("expected one template update, got %#v", details)
	}
}

// Exercise the real Operations runtime and cancellation handler, without a
// forward ReconcileAction publishing progress first. Desired allocation alone
// must neither complete rollback nor make unrelated objects participants.
func TestNonFlatHScaleCancellation(t *testing.T) {
	for _, operation := range []string{"scale-out", "scale-in", "named-swap", "online-high-ordinal", "default-ordinals"} {
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
				case "scale-out":
					scaling.ScaleOut = &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}
					removed = []string{"demo-db-2"}
				case "scale-in":
					scaling.ScaleIn = &opsv1alpha1.ScaleIn{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}
					restored = []string{"demo-db-1"}
				case "default-ordinals":
					comp.Replicas = 1
					comp.Ordinals = appsv1.Ordinals{Discrete: []int32{5, 6}}
					source = map[string]string{"demo-db-5": ""}
					scaling.ScaleOut = &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}
					removed = []string{"demo-db-6"}
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
				if flat {
					beforeCluster, beforeOps := cluster.DeepCopy(), ops.DeepCopy()
					if err := h.Cancel(req, cli, res); !intctrlutil.IsTargetError(err, intctrlutil.ErrorIgnoreCancel) {
						t.Fatalf("flat cancellation must be rejected: %v", err)
					}
					if !reflect.DeepEqual(cluster, beforeCluster) || !reflect.DeepEqual(ops, beforeOps) {
						t.Fatal("rejected cancellation changed the forward operation")
					}
					check(opsv1alpha1.OpsSucceedPhase) // The original scale-out/in can still finish.
					return
				}
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
	return f
}

// saveConfiguration models OpsManager persisting configuration before Action.
func (f *horizontalScalingFixture) saveConfiguration(t *testing.T) {
	t.Helper()
	if err := (horizontalScalingOpsHandler{}).SaveLastConfiguration(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	if err := f.cli.Status().Update(f.req.Ctx, f.res.OpsRequest); err != nil {
		t.Fatal(err)
	}
}

// publishAllocation runs the real workload producers only when a test explicitly advances the workload.
func (f *horizontalScalingFixture) publishAllocation(t *testing.T, spec appsv1.ClusterComponentSpec) {
	t.Helper()
	key := client.ObjectKey{Namespace: "default", Name: "demo-" + spec.Name}
	its := &workloads.InstanceSet{}
	err := f.cli.Get(f.req.Ctx, key, its)
	fresh := apierrors.IsNotFound(err)
	if err != nil && !fresh {
		t.Fatal(err)
	}
	if fresh {
		its.ObjectMeta = metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace}
	}
	its.Generation++
	its.Spec.Replicas = pointer.Int32(spec.Replicas)
	its.Spec.OfflineInstances = spec.OfflineInstances
	its.Spec.Instances = nil
	its.Spec.Selector = &metav1.LabelSelector{MatchLabels: constant.GetCompLabels("demo", spec.Name)}
	its.Spec.Template = corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "mysql:8"}}}}
	for _, template := range spec.Instances {
		its.Spec.Instances = append(its.Spec.Instances, workloads.InstanceTemplate{
			Name: template.Name, Replicas: template.Replicas, Ordinals: template.Ordinals})
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	pods := &corev1.PodList{}
	if err := f.cli.List(f.req.Ctx, pods, client.InNamespace(key.Namespace),
		client.MatchingLabels(constant.GetCompLabels("demo", spec.Name))); err != nil {
		t.Fatal(err)
	}
	for i := range pods.Items {
		if err := tree.Add(&pods.Items[i]); err != nil {
			t.Fatal(err)
		}
	}
	for _, reconciler := range []kubebuilderx.Reconciler{instanceset.NewRevisionUpdateReconciler(), instanceset.NewStatusReconciler()} {
		if _, err := reconciler.Reconcile(tree); err != nil {
			t.Fatal(err)
		}
	}
	status := its.Status.DeepCopy()
	if fresh {
		err = f.cli.Create(f.req.Ctx, its)
	} else {
		err = f.cli.Update(f.req.Ctx, its)
	}
	if err != nil {
		t.Fatal(err)
	}
	its.Status = *status
	if err := f.cli.Status().Update(f.req.Ctx, its); err != nil {
		t.Fatal(err)
	}
}

func scaleOutRequest(name string) opsv1alpha1.HorizontalScaling {
	return opsv1alpha1.HorizontalScaling{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: name},
		ScaleOut: &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}}
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
	f := newHorizontalScalingFixture(t, scaleOutRequest("db"))
	f.publishAllocation(t, f.res.Cluster.Spec.ComponentSpecs[0])
	trace := &horizontalScalingRuntimeTrace{OpsRuntime: f.res.Runtimes["db"]}
	f.res.Runtimes["db"] = trace
	f.saveConfiguration(t)
	t.Cleanup(func() {
		for _, call := range trace.calls {
			if strings.HasPrefix(call, "names(") {
				t.Fatalf("ordinary scaling invoked name planning: %s", call)
			}
		}
	})
	if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	f.replicas(t, "db", 2)
	// Until the owner publishes the new allocation, no participant is invented.
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	if len(f.res.OpsRequest.Status.Components["db"].ProgressDetails) != 0 {
		t.Fatal("unpublished target produced participants")
	}
	f.publishAllocation(t, f.res.Cluster.Spec.ComponentSpecs[0])
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
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
			requests := []opsv1alpha1.HorizontalScaling{scaleOutRequest("ordinary"), backupScaleOutRequest("restored")}
			if backupFirst {
				requests[0], requests[1] = requests[1], requests[0]
			}
			f := newHorizontalScalingFixture(t, requests...)
			f.publishAllocation(t, *f.res.Cluster.Spec.GetComponentByName("ordinary"))
			f.saveConfiguration(t)
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
			f.publishAllocation(t, *f.res.Cluster.Spec.GetComponentByName("ordinary"))
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
				if name == "ordinary" {
					f.publishAllocation(t, *f.res.Cluster.Spec.GetComponentByName(name))
				}
			}
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
	f := newHorizontalScalingFixture(t, scaleOutRequest("db"))
	spec := &f.res.Cluster.Spec.ComponentSpecs[0]
	spec.Instances = []appsv1.InstanceTemplate{{Name: "foo", Replicas: pointer.Int32(1)}}
	spec.OfflineInstances = []string{"demo-db-foo-0"}
	f.publishAllocation(t, *spec)
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
	f.publishAllocation(t, *spec)
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
		request := scaleOutRequest("sharded")
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
			if !changeCount {
				for _, name := range []string{"sharded-a", "sharded-b"} {
					spec := cluster.Spec.Shardings[0].Template
					spec.Name = name
					f.publishAllocation(t, spec)
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
				for _, name := range []string{"sharded-a", "sharded-b"} {
					spec := cluster.Spec.Shardings[0].Template
					spec.Name = name
					f.publishAllocation(t, spec)
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
