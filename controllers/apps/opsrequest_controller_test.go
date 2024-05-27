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

package apps

import (
	"fmt"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("OpsRequest Controller", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterNamePrefix = "test-cluster"
	const mysqlCompDefName = "mysql"
	const mysqlCompName = "mysql"
	const defaultMinReadySeconds = 10

	var (
		_1c1g = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		}
		_2c4g = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("4Gi"),
		}
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.PodSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.OpsRequestSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.VolumeSnapshotSignature, true, inNS)

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		// TODO(review): why finalizers not removed
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)
		testapps.ClearResources(&testCtx, intctrlutil.StorageClassSignature, ml)

		// non-namespaced
		testapps.ClearResources(&testCtx, intctrlutil.BackupPolicyTemplateSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	var (
		clusterDefObj     *appsv1alpha1.ClusterDefinition
		clusterVersionObj *appsv1alpha1.ClusterVersion
		clusterObj        *appsv1alpha1.Cluster
		clusterKey        types.NamespacedName
	)

	mockSetClusterStatusPhaseToRunning := func(namespacedName types.NamespacedName) {
		Expect(testapps.GetAndChangeObjStatus(&testCtx, namespacedName,
			func(fetched *appsv1alpha1.Cluster) {
				// TODO: would be better to have hint for cluster.status.phase is mocked,
				// i.e., add annotation info for the mocked context
				fetched.Status.Phase = appsv1alpha1.RunningClusterPhase
				if len(fetched.Status.Components) == 0 {
					fetched.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{}
					for _, v := range fetched.Spec.ComponentSpecs {
						fetched.Status.SetComponentStatus(v.Name, appsv1alpha1.ClusterComponentStatus{
							Phase: appsv1alpha1.RunningClusterCompPhase,
						})
					}
					return
				}
				for componentKey, componentStatus := range fetched.Status.Components {
					componentStatus.Phase = appsv1alpha1.RunningClusterCompPhase
					fetched.Status.SetComponentStatus(componentKey, componentStatus)
				}
			})()).ShouldNot(HaveOccurred())
	}

	type verticalScalingContext struct {
		source corev1.ResourceRequirements
		target corev1.ResourceRequirements
	}

	testVerticalScaleCPUAndMemory := func(workloadType testapps.ComponentDefTplType, scalingCtx verticalScalingContext) {
		const opsName = "mysql-verticalscaling"

		By("Create a cluster obj")
		clusterFactory := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompDefName).
			SetReplicas(1).
			SetResources(scalingCtx.source)
		clusterObj = clusterFactory.Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enters creating phase")
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))

		By("mock pods are available and wait for cluster enter running phase")
		podName := fmt.Sprintf("%s-%s-0", clusterObj.Name, mysqlCompName)
		pod := testapps.MockInstanceSetPod(&testCtx, nil, clusterObj.Name, mysqlCompName,
			podName, "leader", "ReadWrite")
		// the opsRequest will use startTime to check some condition.
		// if there is no sleep for 1 second, unstable error may occur.
		time.Sleep(time.Second)
		if workloadType == testapps.StatefulMySQLComponent {
			lastTransTime := metav1.NewTime(time.Now().Add(-1 * (defaultMinReadySeconds + 1) * time.Second))
			Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
				testk8s.MockPodAvailable(pod, lastTransTime)
			})).ShouldNot(HaveOccurred())
		}
		itsList := testk8s.ListAndCheckInstanceSetWithComponent(&testCtx, clusterKey, mysqlCompName)
		mysqlIts := &itsList.Items[0]
		Expect(testapps.ChangeObjStatus(&testCtx, mysqlIts, func() {
			testk8s.MockInstanceSetReady(mysqlIts, pod)
		})).ShouldNot(HaveOccurred())
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))

		By("send VerticalScalingOpsRequest successfully")
		opsKey := types.NamespacedName{Name: opsName, Namespace: testCtx.DefaultNamespace}
		verticalScalingOpsRequest := testapps.NewOpsRequestObj(opsKey.Name, opsKey.Namespace,
			clusterObj.Name, appsv1alpha1.VerticalScalingType)
		verticalScalingOpsRequest.Spec.VerticalScalingList = []appsv1alpha1.VerticalScaling{
			{
				ComponentOps:         appsv1alpha1.ComponentOps{ComponentName: mysqlCompName},
				ResourceRequirements: scalingCtx.target,
			},
		}
		Expect(testCtx.CreateObj(testCtx.Ctx, verticalScalingOpsRequest)).Should(Succeed())

		By("wait for VerticalScalingOpsRequest is running")
		Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsRunningPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.UpdatingClusterPhase))
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, mysqlCompName)).Should(Equal(appsv1alpha1.UpdatingClusterCompPhase))
		// TODO(refactor): try to check some ephemeral states?
		// checkLatestOpsIsProcessing(clusterKey, verticalScalingOpsRequest.Spec.Type)

		// By("check Cluster and changed component phase is VerticalScaling")
		// Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
		//	g.Expect(cluster.Status.Phase).To(Equal(appsv1alpha1.SpecReconcilingClusterPhase))
		//	g.Expect(cluster.Status.Components[mysqlCompName].Phase).To(Equal(appsv1alpha1.SpecReconcilingClusterCompPhase))
		// })).Should(Succeed())

		By("mock bring Cluster and changed component back to running status")
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(mysqlIts), func(tmpIts *workloads.InstanceSet) {
			testk8s.MockInstanceSetReady(tmpIts, pod)
		})()).ShouldNot(HaveOccurred())
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, mysqlCompName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))
		// checkLatestOpsHasProcessed(clusterKey)

		By("notice opsrequest controller to run")
		testk8s.MockPodIsTerminating(ctx, testCtx, pod)
		testk8s.RemovePodFinalizer(ctx, testCtx, pod)
		testapps.MockInstanceSetPod(&testCtx, nil, clusterObj.Name, mysqlCompName,
			pod.Name, "leader", "ReadWrite", scalingCtx.target)
		Expect(testapps.ChangeObj(&testCtx, verticalScalingOpsRequest, func(lopsReq *appsv1alpha1.OpsRequest) {
			if lopsReq.Annotations == nil {
				lopsReq.Annotations = map[string]string{}
			}
			lopsReq.Annotations[constant.ReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
		})).ShouldNot(HaveOccurred())

		By("check VerticalScalingOpsRequest succeed")
		Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsSucceedPhase))

		By("check cluster resource requirements changed")
		targetRequests := scalingCtx.target.Requests
		itsList = testk8s.ListAndCheckInstanceSetWithComponent(&testCtx, clusterKey, mysqlCompName)
		mysqlIts = &itsList.Items[0]
		Expect(reflect.DeepEqual(mysqlIts.Spec.Template.Spec.Containers[0].Resources.Requests, targetRequests)).Should(BeTrue())

		By("check OpsRequest reclaimed after ttl")
		Expect(testapps.ChangeObj(&testCtx, verticalScalingOpsRequest, func(lopsReq *appsv1alpha1.OpsRequest) {
			lopsReq.Spec.TTLSecondsAfterSucceed = 1
		})).ShouldNot(HaveOccurred())

		Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(verticalScalingOpsRequest), verticalScalingOpsRequest, false)).Should(Succeed())
	}

	// Scenarios

	// TODO: should focus on OpsRequest control actions, and iterator through all component workload types.
	Context("with Cluster which has MySQL Component", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponentVersion(mysqlCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()
		})

		It("create cluster by resource, vertical scaling by resource", func() {
			ctx := verticalScalingContext{
				source: corev1.ResourceRequirements{Requests: _1c1g, Limits: _1c1g},
				target: corev1.ResourceRequirements{Requests: _2c4g, Limits: _2c4g},
			}
			testVerticalScaleCPUAndMemory(testapps.StatefulMySQLComponent, ctx)
		})
	})

	Context("with Cluster which has MySQL ConsensusSet", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			testk8s.MockEnableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ConsensusMySQLComponent, mysqlCompDefName).
				AddHorizontalScalePolicy(appsv1alpha1.HorizontalScalePolicy{
					Type:                     appsv1alpha1.HScaleDataClonePolicyCloneVolume,
					BackupPolicyTemplateName: backupPolicyTPLName,
				}).Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponentVersion(mysqlCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()

			By("Mock lorry client for the default transformer of system accounts provision")
			mockLorryClientDefault()
		})

		componentWorkload := func() client.Object {
			itsList := testk8s.ListAndCheckInstanceSetWithComponent(&testCtx, clusterKey, mysqlCompName)
			return &itsList.Items[0]
		}

		mockCompRunning := func(replicas int32, reCreatePod bool) {
			// to wait the component object becomes stable
			compKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      component.FullName(clusterKey.Name, mysqlCompName),
			}
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *appsv1alpha1.Component) {
				g.Expect(comp.Generation).Should(Equal(comp.Status.ObservedGeneration))
			})).Should(Succeed())

			wl := componentWorkload()
			its, _ := wl.(*workloads.InstanceSet)
			if reCreatePod {
				podList := &corev1.PodList{}
				Expect(k8sClient.List(ctx, podList, client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: mysqlCompName,
				})).Should(Succeed())
				for i := range podList.Items {
					testk8s.MockPodIsTerminating(ctx, testCtx, &podList.Items[i])
					testk8s.RemovePodFinalizer(ctx, testCtx, &podList.Items[i])
				}
			}
			cluster := &appsv1alpha1.Cluster{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(clusterObj), cluster)).Should(Succeed())
			mockPods := testapps.MockInstanceSetPods(&testCtx, its, cluster, mysqlCompName)
			Expect(testapps.ChangeObjStatus(&testCtx, its, func() {
				testk8s.MockInstanceSetReady(its, mockPods...)
			})).ShouldNot(HaveOccurred())

			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, mysqlCompName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		}

		createMysqlCluster := func(replicas int32) {
			createBackupPolicyTpl(clusterDefObj, mysqlCompDefName)

			By("set component to horizontal with snapshot policy and create a cluster")
			testk8s.MockEnableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)
			if clusterDefObj.Spec.ComponentDefs[0].HorizontalScalePolicy == nil {
				Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
					func(clusterDef *appsv1alpha1.ClusterDefinition) {
						clusterDef.Spec.ComponentDefs[0].HorizontalScalePolicy =
							&appsv1alpha1.HorizontalScalePolicy{Type: appsv1alpha1.HScaleDataClonePolicyCloneVolume}
					})()).ShouldNot(HaveOccurred())
			}
			pvcSpec := testapps.NewPVCSpec("1Gi")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(mysqlCompName, mysqlCompDefName).
				SetReplicas(replicas).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("mock component is Running")
			mockCompRunning(replicas, false)

			By("mock pvc created")
			for i := 0; i < int(replicas); i++ {
				pvcName := fmt.Sprintf("%s-%s-%s-%d", testapps.DataVolumeName, clusterKey.Name, mysqlCompName, i)
				pvc := testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterKey.Name,
					mysqlCompName, testapps.DataVolumeName).
					SetStorage("1Gi").
					SetStorageClass(testk8s.DefaultStorageClassName).
					Create(&testCtx).
					GetObject()
				// mock pvc bound
				Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(pvc), func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Phase = corev1.ClaimBound
				})()).ShouldNot(HaveOccurred())
			}
			// wait for cluster observed generation
			Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
			mockSetClusterStatusPhaseToRunning(clusterKey)
			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, mysqlCompName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))
			Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		}

		createClusterHscaleOps := func(replicas int32) *appsv1alpha1.OpsRequest {
			By("create a opsRequest to horizontal scale")
			opsName := "hscale-ops-" + testCtx.GetRandomStr()
			ops := testapps.NewOpsRequestObj(opsName, testCtx.DefaultNamespace,
				clusterObj.Name, appsv1alpha1.HorizontalScalingType)
			ops.Spec.HorizontalScalingList = []appsv1alpha1.HorizontalScaling{
				{
					ComponentOps:    appsv1alpha1.ComponentOps{ComponentName: mysqlCompName},
					ReplicasWrapper: appsv1alpha1.ReplicasWrapper{Replicas: pointer.Int32(replicas)},
				},
			}
			// for reconciling the ops labels
			ops.Labels = nil
			Expect(testCtx.CreateObj(testCtx.Ctx, ops)).Should(Succeed())
			return ops
		}

		It("issue an VerticalScalingOpsRequest should change Cluster's resource requirements successfully", func() {
			ctx := verticalScalingContext{
				source: corev1.ResourceRequirements{Requests: _1c1g, Limits: _1c1g},
				target: corev1.ResourceRequirements{Requests: _2c4g, Limits: _2c4g},
			}
			testVerticalScaleCPUAndMemory(testapps.ConsensusMySQLComponent, ctx)
		})

		It("HorizontalScaling when not support snapshot", func() {
			By("init backup policy template, mysql cluster and hscale ops")
			testk8s.MockDisableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)

			createMysqlCluster(3)
			cluster := &appsv1alpha1.Cluster{}
			Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(Succeed())
			initGeneration := cluster.Status.ObservedGeneration
			Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(Equal(initGeneration))

			ops := createClusterHscaleOps(5)
			opsKey := client.ObjectKeyFromObject(ops)

			By("expect component is Running if don't support volume snapshot during doing h-scale ops")
			Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsRunningPhase))
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
				// the cluster spec has been updated by ops-controller to scale out.
				g.Expect(fetched.Generation == initGeneration+1).Should(BeTrue())
				// expect cluster phase is Updating during Hscale.
				g.Expect(fetched.Generation > fetched.Status.ObservedGeneration).Should(BeTrue())
				g.Expect(fetched.Status.Phase).Should(Equal(appsv1alpha1.UpdatingClusterPhase))
				// when snapshot is not supported, the expected component phase is running.
				g.Expect(fetched.Status.Components[mysqlCompName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
				// expect preCheckFailed condition to occur.
				condition := meta.FindStatusCondition(fetched.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Status).Should(BeFalse())
				g.Expect(condition.Reason).Should(Equal(ReasonPreCheckFailed))
				g.Expect(condition.Message).Should(Equal("HorizontalScaleFailed: volume snapshot not support"))
			}))

			By("reset replicas to 3 and cluster phase should be reconciled to Running")
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
				cluster.Spec.ComponentSpecs[0].Replicas = int32(3)
			})()).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, lcluster *appsv1alpha1.Cluster) {
				g.Expect(lcluster.Generation == initGeneration+2).Should(BeTrue())
				g.Expect(lcluster.Generation == lcluster.Status.ObservedGeneration).Should(BeTrue())
				g.Expect(cluster.Status.Phase).Should(Equal(appsv1alpha1.RunningClusterPhase))
			})).Should(Succeed())
		})

		It("HorizontalScaling via volume snapshot backup", func() {
			By("init backup policy template, mysql cluster and hscale ops")
			testk8s.MockEnableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)
			oldReplicas := int32(3)
			createMysqlCluster(oldReplicas)

			replicas := int32(5)
			ops := createClusterHscaleOps(replicas)
			opsKey := client.ObjectKeyFromObject(ops)

			By("expect cluster and component is reconciling the new spec")
			Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsRunningPhase))
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Generation == 2).Should(BeTrue())
				g.Expect(cluster.Status.ObservedGeneration == 2).Should(BeTrue())
				g.Expect(cluster.Status.Components[mysqlCompName].Phase).Should(Equal(appsv1alpha1.UpdatingClusterCompPhase))
				// the expected cluster phase is Updating during Hscale.
				g.Expect(cluster.Status.Phase).Should(Equal(appsv1alpha1.UpdatingClusterPhase))
			})).Should(Succeed())

			By("mock backup status is ready, component phase should change to Updating when component is horizontally scaling.")
			backupKey := client.ObjectKey{Name: fmt.Sprintf("%s-%s-scaling",
				clusterKey.Name, mysqlCompName), Namespace: testCtx.DefaultNamespace}
			backup := &dpv1alpha1.Backup{}
			Expect(k8sClient.Get(testCtx.Ctx, backupKey, backup)).Should(Succeed())
			backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
			testdp.MockBackupStatusMethod(backup, testdp.BackupMethodName, testapps.DataVolumeName, testdp.ActionSetName)
			Expect(k8sClient.Status().Update(testCtx.Ctx, backup)).Should(Succeed())
			Consistently(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Status.Components[mysqlCompName].Phase).Should(Equal(appsv1alpha1.UpdatingClusterCompPhase))
				g.Expect(cluster.Status.Phase).Should(Equal(appsv1alpha1.UpdatingClusterPhase))
			})).Should(Succeed())

			By("mock create volumesnapshot, which should done by backup controller")
			vs := &snapshotv1.VolumeSnapshot{}
			vs.Name = backupKey.Name
			vs.Namespace = backupKey.Namespace
			vs.Labels = map[string]string{
				dptypes.BackupNameLabelKey: backupKey.Name,
			}
			pvcName := ""
			vs.Spec = snapshotv1.VolumeSnapshotSpec{
				Source: snapshotv1.VolumeSnapshotSource{
					PersistentVolumeClaimName: &pvcName,
				},
			}
			Expect(k8sClient.Create(testCtx.Ctx, vs)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, backupKey, vs, true)).Should(Succeed())

			mockComponentPVCsAndBound := func(comp *appsv1alpha1.ClusterComponentSpec) {
				for i := 0; i < int(replicas); i++ {
					for _, vct := range comp.VolumeClaimTemplates {
						pvcKey := types.NamespacedName{
							Namespace: clusterKey.Namespace,
							Name:      fmt.Sprintf("%s-%s-%s-%d", vct.Name, clusterKey.Name, comp.Name, i),
						}
						testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcKey.Name, clusterKey.Name,
							comp.Name, testapps.DataVolumeName).SetStorage(vct.Spec.Resources.Requests.Storage().String()).AddLabelsInMap(map[string]string{
							constant.AppInstanceLabelKey:    clusterKey.Name,
							constant.KBAppComponentLabelKey: comp.Name,
							constant.AppManagedByLabelKey:   constant.AppName,
						}).CheckedCreate(&testCtx)
						Eventually(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
							pvc.Status.Phase = corev1.ClaimBound
							if pvc.Status.Capacity == nil {
								pvc.Status.Capacity = corev1.ResourceList{}
							}
							pvc.Status.Capacity[corev1.ResourceStorage] = pvc.Spec.Resources.Requests[corev1.ResourceStorage]
						})).Should(Succeed())
					}
				}
			}

			// mock pvcs have restored
			mockComponentPVCsAndBound(clusterObj.Spec.GetComponentByName(mysqlCompName))
			// check restore CR and mock it to Completed
			checkRestoreAndSetCompleted(clusterKey, mysqlCompName, int(replicas-oldReplicas))

			By("check the underlying workload been updated")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentWorkload()),
				func(g Gomega, its *workloads.InstanceSet) {
					g.Expect(*its.Spec.Replicas).Should(Equal(replicas))
				})).Should(Succeed())
			its := componentWorkload()
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(its), func(its *workloads.InstanceSet) {
				its.Spec.Replicas = &replicas
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentWorkload()),
				func(g Gomega, its *workloads.InstanceSet) {
					g.Expect(*its.Spec.Replicas).Should(Equal(replicas))
				})).Should(Succeed())

			By("Checking pvc created")
			Eventually(testapps.List(&testCtx, intctrlutil.PersistentVolumeClaimSignature,
				client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: mysqlCompName,
				}, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(int(replicas)))

			By("mock all new PVCs scaled bounded")
			for i := 0; i < int(replicas); i++ {
				pvcKey := types.NamespacedName{
					Namespace: testCtx.DefaultNamespace,
					Name:      fmt.Sprintf("%s-%s-%s-%d", testapps.DataVolumeName, clusterKey.Name, mysqlCompName, i),
				}
				Expect(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Phase = corev1.ClaimBound
				})()).Should(Succeed())
			}

			By("check the backup created for scaling has been deleted")
			Eventually(testapps.CheckObjExists(&testCtx, backupKey, backup, false)).Should(Succeed())

			By("mock component workload is running and expect cluster and component are running")
			mockCompRunning(replicas, false)
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Status.Components[mysqlCompName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
				g.Expect(cluster.Status.Phase).Should(Equal(appsv1alpha1.RunningClusterPhase))
			})).Should(Succeed())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsSucceedPhase))
		})

		It("HorizontalScaling when the number of pods is inconsistent with the number of replicas", func() {
			mockLorryClient4HScale(clusterKey, mysqlCompName, 2)

			By("create a cluster with 3 pods")
			createMysqlCluster(3)

			By("mock component replicas to 4 and actual pods is 3")
			Expect(testapps.ChangeObj(&testCtx, clusterObj, func(clusterObj *appsv1alpha1.Cluster) {
				clusterObj.Spec.ComponentSpecs[0].Replicas = 4
			})).Should(Succeed())

			By("scale down the cluster replicas to 2")
			phase := appsv1alpha1.OpsPendingPhase
			replicas := int32(2)
			ops := createClusterHscaleOps(replicas)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(ops), func(g Gomega, ops *appsv1alpha1.OpsRequest) {
				phases := []appsv1alpha1.OpsPhase{appsv1alpha1.OpsRunningPhase, appsv1alpha1.OpsFailedPhase}
				g.Expect(slices.Contains(phases, ops.Status.Phase)).Should(BeTrue())
				phase = ops.Status.Phase
			})).Should(Succeed())

			// Since the component replicas is different with running pods, the cluster and component phase may be
			// Running or Updating, it depends on the timing of cluster reconciling and ops request submission.
			// If the phase is Updating, ops request will be failed because of cluster phase conflict.
			if phase == appsv1alpha1.OpsFailedPhase {
				return
			}

			By("wait for cluster and component phase are Updating")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Status.Components[mysqlCompName].Phase).Should(Equal(appsv1alpha1.UpdatingClusterCompPhase))
				g.Expect(cluster.Status.Phase).Should(Equal(appsv1alpha1.UpdatingClusterPhase))
			})).Should(Succeed())

			By("check the underlying workload been updated")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentWorkload()),
				func(g Gomega, its *workloads.InstanceSet) {
					g.Expect(*its.Spec.Replicas).Should(Equal(replicas))
				})).Should(Succeed())
			its := componentWorkload()
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(its), func(its *workloads.InstanceSet) {
				its.Spec.Replicas = &replicas
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentWorkload()),
				func(g Gomega, its *workloads.InstanceSet) {
					g.Expect(*its.Spec.Replicas).Should(Equal(replicas))
				})).Should(Succeed())

			By("mock scale down successfully by deleting one pod ")
			podName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, mysqlCompName, 2)
			dPodKeys := types.NamespacedName{Name: podName, Namespace: testCtx.DefaultNamespace}
			testapps.DeleteObject(&testCtx, dPodKeys, &corev1.Pod{})

			By("expect opsRequest phase to Succeed after cluster is Running")
			mockCompRunning(replicas, false)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(ops), func(g Gomega, ops *appsv1alpha1.OpsRequest) {
				g.Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsSucceedPhase))
				g.Expect(ops.Status.Progress).Should(Equal("2/2"))
			})).Should(Succeed())
		})

		It("delete Running opsRequest", func() {
			By("Create a horizontalScaling ops")
			testk8s.MockEnableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)
			createMysqlCluster(3)
			ops := createClusterHscaleOps(5)
			opsKey := client.ObjectKeyFromObject(ops)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsRunningPhase))

			By("check if existing horizontalScaling opsRequest annotation in cluster")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmlCluster *appsv1alpha1.Cluster) {
				opsSlice, _ := opsutil.GetOpsRequestSliceFromCluster(tmlCluster)
				g.Expect(opsSlice).Should(HaveLen(1))
				g.Expect(opsSlice[0].Name).Should(Equal(ops.Name))
			})).Should(Succeed())

			By("delete the Running ops")
			testapps.DeleteObject(&testCtx, opsKey, ops)
			Expect(testapps.ChangeObj(&testCtx, ops, func(lopsReq *appsv1alpha1.OpsRequest) {
				lopsReq.SetFinalizers([]string{})
			})).ShouldNot(HaveOccurred())

			By("check the cluster annotation")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmlCluster *appsv1alpha1.Cluster) {
				opsSlice, _ := opsutil.GetOpsRequestSliceFromCluster(tmlCluster)
				g.Expect(opsSlice).Should(HaveLen(0))
			})).Should(Succeed())
		})

		It("cancel HorizontalScaling opsRequest which is Running", func() {
			By("create cluster and mock it to running")
			testk8s.MockDisableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)
			oldReplicas := int32(3)
			createMysqlCluster(oldReplicas)
			mockCompRunning(oldReplicas, false)

			By("create a horizontalScaling ops")
			ops := createClusterHscaleOps(5)
			opsKey := client.ObjectKeyFromObject(ops)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsRunningPhase))
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.UpdatingClusterPhase))

			By("create one pod")
			podName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, mysqlCompName, 3)
			pod := testapps.MockInstanceSetPod(&testCtx, nil, clusterObj.Name, mysqlCompName, podName, "follower", "Readonly")

			By("cancel the opsRequest")
			Eventually(testapps.ChangeObj(&testCtx, ops, func(opsRequest *appsv1alpha1.OpsRequest) {
				opsRequest.Spec.Cancel = true
			})).Should(Succeed())

			By("check opsRequest is Cancelling")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(ops), func(g Gomega, opsRequest *appsv1alpha1.OpsRequest) {
				g.Expect(opsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsCancellingPhase))
				g.Expect(opsRequest.Status.CancelTimestamp.IsZero()).Should(BeFalse())
				cancelCondition := meta.FindStatusCondition(opsRequest.Status.Conditions, appsv1alpha1.ConditionTypeCancelled)
				g.Expect(cancelCondition).ShouldNot(BeNil())
				g.Expect(cancelCondition.Reason).Should(Equal(appsv1alpha1.ReasonOpsCanceling))
			})).Should(Succeed())

			By("delete the created pod")
			pod.Kind = constant.PodKind
			testk8s.MockPodIsTerminating(ctx, testCtx, pod)
			testk8s.RemovePodFinalizer(ctx, testCtx, pod)

			By("opsRequest phase should be Cancelled")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(ops), func(g Gomega, opsRequest *appsv1alpha1.OpsRequest) {
				g.Expect(opsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsCancelledPhase))
				cancelCondition := meta.FindStatusCondition(opsRequest.Status.Conditions, appsv1alpha1.ConditionTypeCancelled)
				g.Expect(cancelCondition).ShouldNot(BeNil())
				g.Expect(cancelCondition.Reason).Should(Equal(appsv1alpha1.ReasonOpsCancelSucceed))
			})).Should(Succeed())

			By("cluster phase should be Running and delete the opsRequest annotation")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmlCluster *appsv1alpha1.Cluster) {
				opsSlice, _ := opsutil.GetOpsRequestSliceFromCluster(tmlCluster)
				g.Expect(opsSlice).Should(HaveLen(0))
				g.Expect(tmlCluster.Status.Phase).Should(Equal(appsv1alpha1.RunningClusterPhase))
			})).Should(Succeed())
		})

		createRestartOps := func(clusterName string, index int, force ...bool) *appsv1alpha1.OpsRequest {
			opsName := fmt.Sprintf("restart-ops-%d", index)
			ops := testapps.NewOpsRequestObj(opsName, testCtx.DefaultNamespace,
				clusterName, appsv1alpha1.RestartType)
			ops.Spec.RestartList = []appsv1alpha1.ComponentOps{
				{ComponentName: mysqlCompName},
			}
			if len(force) > 0 {
				ops.Spec.Force = force[0]
			}
			return testapps.CreateOpsRequest(ctx, testCtx, ops)
		}

		It("test opsRequest queue", func() {
			By("create cluster and mock it to running")
			replicas := int32(3)
			createMysqlCluster(replicas)
			mockCompRunning(replicas, false)

			By("create first restart ops")
			time.Sleep(time.Second)
			ops1 := createRestartOps(clusterObj.Name, 1)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops1))).Should(Equal(appsv1alpha1.OpsRunningPhase))
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.UpdatingClusterPhase))

			By("create second restart ops")
			ops2 := createRestartOps(clusterObj.Name, 2)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops2))).Should(Equal(appsv1alpha1.OpsPendingPhase))

			By("create third restart ops")
			ops3 := createRestartOps(clusterObj.Name, 3)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops3))).Should(Equal(appsv1alpha1.OpsPendingPhase))

			By("expect for all opsRequests in the queue")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterObj), func(g Gomega, cluster *appsv1alpha1.Cluster) {
				opsSlice, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
				g.Expect(len(opsSlice)).Should(Equal(3))
				// ops1 is running
				g.Expect(opsSlice[0].InQueue).Should(BeFalse())
				g.Expect(opsSlice[1].InQueue).Should(BeTrue())
				g.Expect(opsSlice[2].InQueue).Should(BeTrue())
			})).Should(Succeed())

			By("mock ops1 phase to Succeed")
			mockCompRunning(replicas, true)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops1))).Should(Equal(appsv1alpha1.OpsSucceedPhase))

			By("expect for next ops is Running")
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops2))).Should(Equal(appsv1alpha1.OpsRunningPhase))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterObj), func(g Gomega, cluster *appsv1alpha1.Cluster) {
				// ops1 should be dequeue
				opsSlice, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
				g.Expect(len(opsSlice)).Should(Equal(2))
				g.Expect(opsSlice[0].InQueue).Should(BeFalse())
				g.Expect(opsSlice[1].InQueue).Should(BeTrue())
			})).Should(Succeed())

			// TODO: test head opsRequest phase is Failed by mocking pod is Failed
		})

		It("test opsRequest force flag", func() {
			By("create cluster and mock it to running")
			replicas := int32(3)
			createMysqlCluster(replicas)
			mockCompRunning(replicas, false)

			By("create first restart ops")
			time.Sleep(time.Second)
			ops1 := createRestartOps(clusterObj.Name, 1)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops1))).Should(Equal(appsv1alpha1.OpsRunningPhase))
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.UpdatingClusterPhase))

			By("create secondary restart ops")
			ops2 := createRestartOps(clusterObj.Name, 2)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops2))).Should(Equal(appsv1alpha1.OpsPendingPhase))

			By("create third restart ops with force flag")
			ops3 := createRestartOps(clusterObj.Name, 3, true)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops3))).Should(Equal(appsv1alpha1.OpsRunningPhase))

			By("expect for ops3 is running and ops1/ops is aborted")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterObj), func(g Gomega, cluster *appsv1alpha1.Cluster) {
				opsSlice, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
				g.Expect(len(opsSlice)).Should(Equal(1))
				g.Expect(opsSlice[0].InQueue).Should(BeFalse())
			})).Should(Succeed())

			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops1))).Should(Equal(appsv1alpha1.OpsAbortedPhase))
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops2))).Should(Equal(appsv1alpha1.OpsAbortedPhase))

			By("mock component to running and expect op3 phase to Succeed")
			mockCompRunning(replicas, true)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops3))).Should(Equal(appsv1alpha1.OpsSucceedPhase))
		})

		It("test opsRequest queue for QueueBySelf", func() {
			By("create cluster and mock it to running")
			replicas := int32(3)
			createMysqlCluster(replicas)
			mockCompRunning(replicas, false)

			By("create first restart ops")
			time.Sleep(time.Second)
			restartOps1 := createRestartOps(clusterObj.Name, 0)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(restartOps1))).Should(Equal(appsv1alpha1.OpsRunningPhase))
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.UpdatingClusterPhase))

			createExposeOps := func(clusterName string, index int, exposeSwitch appsv1alpha1.ExposeSwitch) *appsv1alpha1.OpsRequest {
				ops := testapps.NewOpsRequestObj(fmt.Sprintf("expose-ops-%d", index), testCtx.DefaultNamespace,
					clusterName, appsv1alpha1.ExposeType)
				ops.Spec.ExposeList = []appsv1alpha1.Expose{
					{
						ComponentName: mysqlCompName,
						Switch:        exposeSwitch,
						Services: []appsv1alpha1.OpsService{
							{
								Name:        "svc1",
								ServiceType: corev1.ServiceTypeLoadBalancer,
								Ports: []corev1.ServicePort{
									{Name: "port1", Port: 3306, TargetPort: intstr.IntOrString{Type: intstr.String, StrVal: "mysql"}},
								},
							},
						},
					},
				}
				return testapps.CreateOpsRequest(ctx, testCtx, ops)
			}
			By("create expose ops which needs to queue by self")
			exposeOps1 := createExposeOps(clusterObj.Name, 1, appsv1alpha1.EnableExposeSwitch)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(exposeOps1))).Should(Equal(appsv1alpha1.OpsRunningPhase))

			By("create secondary restart ops and expect it to Pending")
			restartOps2 := createRestartOps(clusterObj.Name, 2)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(restartOps2))).Should(Equal(appsv1alpha1.OpsPendingPhase))

			By("create secondary expose ops and expect it to Pending")
			exposeOps2 := createExposeOps(clusterObj.Name, 3, appsv1alpha1.DisableExposeSwitch)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(exposeOps2))).Should(Equal(appsv1alpha1.OpsPendingPhase))

			By("check opsRequest queue")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterObj), func(g Gomega, cluster *appsv1alpha1.Cluster) {
				opsSlice, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
				g.Expect(len(opsSlice)).Should(Equal(4))
				// restartOps1 is running
				g.Expect(opsSlice[0].InQueue).Should(BeFalse())
				// exposOps1 is running
				g.Expect(opsSlice[1].InQueue).Should(BeFalse())
				// restartOps2 type is pending
				g.Expect(opsSlice[2].InQueue).Should(BeTrue())
				// exposOps2 is pending
				g.Expect(opsSlice[3].InQueue).Should(BeTrue())
			})).Should(Succeed())

			By("mock component to running and expect restartOps1 phase to Succeed")
			mockCompRunning(replicas, true)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(restartOps1))).Should(Equal(appsv1alpha1.OpsSucceedPhase))

			By("mock loadBalance service is ready")
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKey{Name: fmt.Sprintf("%s-%s-svc1", clusterObj.Name, mysqlCompName), Namespace: testCtx.DefaultNamespace}, func(svc *corev1.Service) {
				svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{Hostname: "test", IP: "192.168.1.110"}}
			})()).Should(Succeed())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(exposeOps1))).Should(Equal(appsv1alpha1.OpsSucceedPhase))

			By("expect restartOps2 to Running and exposeOps2 to Succeed")
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(restartOps2))).Should(Equal(appsv1alpha1.OpsRunningPhase))
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(exposeOps2))).Should(Equal(appsv1alpha1.OpsSucceedPhase))
		})
	})
})
