/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
		testapps.ClearResources(&testCtx, intctrlutil.ComponentResourceConstraintSignature, ml)
		testapps.ClearResources(&testCtx, intctrlutil.ComponentClassDefinitionSignature, ml)
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

	type resourceContext struct {
		class    *appsv1alpha1.ComponentClass
		resource corev1.ResourceRequirements
	}

	type verticalScalingContext struct {
		source resourceContext
		target resourceContext
	}

	testVerticalScaleCPUAndMemory := func(workloadType testapps.ComponentDefTplType, scalingCtx verticalScalingContext) {
		const opsName = "mysql-verticalscaling"

		By("Create class related objects")
		testapps.NewComponentResourceConstraintFactory(testapps.DefaultResourceConstraintName).
			AddConstraints(testapps.GeneralResourceConstraint).
			AddSelector(appsv1alpha1.ClusterResourceConstraintSelector{
				ClusterDefRef: clusterDefName,
				Components: []appsv1alpha1.ComponentResourceConstraintSelector{
					{
						ComponentDefRef: mysqlCompDefName,
						Rules:           []string{"c1", "c2", "c3", "c4"},
					},
				},
			}).
			Create(&testCtx).GetObject()

		testapps.NewComponentClassDefinitionFactory("custom", clusterDefObj.Name, mysqlCompDefName).
			AddClasses([]appsv1alpha1.ComponentClass{testapps.Class1c1g, testapps.Class2c4g}).
			Create(&testCtx)

		By("Create a cluster obj")
		clusterFactory := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompDefName).
			SetReplicas(1)
		if scalingCtx.source.class != nil {
			clusterFactory.SetClassDefRef(&appsv1alpha1.ClassDefRef{Class: scalingCtx.source.class.Name})
		} else {
			clusterFactory.SetResources(scalingCtx.source.resource)
		}
		clusterObj = clusterFactory.Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enters creating phase")
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))

		By("mock pod/sts are available and wait for cluster enter running phase")
		podName := fmt.Sprintf("%s-%s-0", clusterObj.Name, mysqlCompName)
		pod := testapps.MockConsensusComponentStsPod(&testCtx, nil, clusterObj.Name, mysqlCompName,
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
		var mysqlSts *appsv1.StatefulSet
		var mysqlRSM *workloads.ReplicatedStateMachine
		rsmList := testk8s.ListAndCheckRSMWithComponent(&testCtx, clusterKey, mysqlCompName)
		mysqlRSM = &rsmList.Items[0]
		mysqlSts = testapps.NewStatefulSetFactory(mysqlRSM.Namespace, mysqlRSM.Name, clusterKey.Name, mysqlCompName).
			SetReplicas(*mysqlRSM.Spec.Replicas).Create(&testCtx).GetObject()
		Expect(testapps.ChangeObjStatus(&testCtx, mysqlSts, func() {
			testk8s.MockStatefulSetReady(mysqlSts)
		})).ShouldNot(HaveOccurred())
		Expect(testapps.ChangeObjStatus(&testCtx, mysqlRSM, func() {
			testk8s.MockRSMReady(mysqlRSM, pod)
		})).ShouldNot(HaveOccurred())
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))

		By("send VerticalScalingOpsRequest successfully")
		opsKey := types.NamespacedName{Name: opsName, Namespace: testCtx.DefaultNamespace}
		verticalScalingOpsRequest := testapps.NewOpsRequestObj(opsKey.Name, opsKey.Namespace,
			clusterObj.Name, appsv1alpha1.VerticalScalingType)
		if scalingCtx.target.class != nil {
			verticalScalingOpsRequest.Spec.VerticalScalingList = []appsv1alpha1.VerticalScaling{
				{
					ComponentOps: appsv1alpha1.ComponentOps{ComponentName: mysqlCompName},
					ClassDefRef: &appsv1alpha1.ClassDefRef{
						Class: scalingCtx.target.class.Name,
					},
				},
			}
		} else {
			verticalScalingOpsRequest.Spec.VerticalScalingList = []appsv1alpha1.VerticalScaling{
				{
					ComponentOps:         appsv1alpha1.ComponentOps{ComponentName: mysqlCompName},
					ResourceRequirements: scalingCtx.target.resource,
				},
			}
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
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(mysqlRSM), func(tmpRSM *workloads.ReplicatedStateMachine) {
			testk8s.MockRSMReady(tmpRSM, pod)
		})()).ShouldNot(HaveOccurred())
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, mysqlCompName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))
		// checkLatestOpsHasProcessed(clusterKey)

		By("notice opsrequest controller to run")
		Expect(testapps.ChangeObj(&testCtx, verticalScalingOpsRequest, func(lopsReq *appsv1alpha1.OpsRequest) {
			if lopsReq.Annotations == nil {
				lopsReq.Annotations = map[string]string{}
			}
			lopsReq.Annotations[constant.ReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
		})).ShouldNot(HaveOccurred())

		By("check VerticalScalingOpsRequest succeed")
		Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsSucceedPhase))

		By("check cluster resource requirements changed")
		var targetRequests corev1.ResourceList
		if scalingCtx.target.class != nil {
			targetRequests = corev1.ResourceList{
				corev1.ResourceCPU:    scalingCtx.target.class.CPU,
				corev1.ResourceMemory: scalingCtx.target.class.Memory,
			}
		} else {
			targetRequests = scalingCtx.target.resource.Requests
		}

		rsmList = testk8s.ListAndCheckRSMWithComponent(&testCtx, clusterKey, mysqlCompName)
		mysqlRSM = &rsmList.Items[0]
		Expect(reflect.DeepEqual(mysqlRSM.Spec.Template.Spec.Containers[0].Resources.Requests, targetRequests)).Should(BeTrue())

		By("check OpsRequest reclaimed after ttl")
		Expect(testapps.ChangeObj(&testCtx, verticalScalingOpsRequest, func(lopsReq *appsv1alpha1.OpsRequest) {
			lopsReq.Spec.TTLSecondsAfterSucceed = 1
		})).ShouldNot(HaveOccurred())

		Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(verticalScalingOpsRequest), verticalScalingOpsRequest, false)).Should(Succeed())
	}

	// Scenarios

	// TODO: should focus on OpsRequest control actions, and iterator through all component workload types.
	Context("with Cluster which has MySQL StatefulSet", func() {
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

		It("create cluster by class, vertical scaling by resource", func() {
			ctx := verticalScalingContext{
				source: resourceContext{class: &testapps.Class1c1g},
				target: resourceContext{resource: testapps.Class2c4g.ToResourceRequirements()},
			}
			testVerticalScaleCPUAndMemory(testapps.StatefulMySQLComponent, ctx)
		})

		// TODO(component): class ref
		PIt("create cluster by class, vertical scaling by class", func() {
			ctx := verticalScalingContext{
				source: resourceContext{class: &testapps.Class1c1g},
				target: resourceContext{class: &testapps.Class2c4g},
			}
			testVerticalScaleCPUAndMemory(testapps.StatefulMySQLComponent, ctx)
		})

		// TODO(component): class ref
		PIt("create cluster by resource, vertical scaling by class", func() {
			ctx := verticalScalingContext{
				source: resourceContext{resource: testapps.Class1c1g.ToResourceRequirements()},
				target: resourceContext{class: &testapps.Class2c4g},
			}
			testVerticalScaleCPUAndMemory(testapps.StatefulMySQLComponent, ctx)
		})

		It("create cluster by resource, vertical scaling by resource", func() {
			ctx := verticalScalingContext{
				source: resourceContext{resource: testapps.Class1c1g.ToResourceRequirements()},
				target: resourceContext{resource: testapps.Class2c4g.ToResourceRequirements()},
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
			rsmList := testk8s.ListAndCheckRSMWithComponent(&testCtx, clusterKey, mysqlCompName)
			return &rsmList.Items[0]
		}

		mockCompRunning := func(replicas int32) {
			// to wait the component object becomes stable
			compKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      component.FullName(clusterKey.Name, mysqlCompName),
			}
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *appsv1alpha1.Component) {
				g.Expect(comp.Generation).Should(Equal(comp.Status.ObservedGeneration))
			})).Should(Succeed())

			wl := componentWorkload()
			rsm, _ := wl.(*workloads.ReplicatedStateMachine)
			sts := testapps.NewStatefulSetFactory(rsm.Namespace, rsm.Name, clusterKey.Name, mysqlCompName).
				SetReplicas(*rsm.Spec.Replicas).GetObject()
			testapps.CheckedCreateK8sResource(&testCtx, sts)

			mockPods := testapps.MockConsensusComponentPods(&testCtx, sts, clusterObj.Name, mysqlCompName)
			Expect(testapps.ChangeObjStatus(&testCtx, rsm, func() {
				testk8s.MockRSMReady(rsm, mockPods...)
			})).ShouldNot(HaveOccurred())
			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				testk8s.MockStatefulSetReady(sts)
			})).ShouldNot(HaveOccurred())

			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, mysqlCompName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		}

		createMysqlCluster := func(replicas int32) {
			createBackupPolicyTpl(clusterDefObj)

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
			mockCompRunning(replicas)

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
					ComponentOps: appsv1alpha1.ComponentOps{ComponentName: mysqlCompName},
					Replicas:     replicas,
				},
			}
			// for reconciling the ops labels
			ops.Labels = nil
			Expect(testCtx.CreateObj(testCtx.Ctx, ops)).Should(Succeed())
			return ops
		}

		It("issue an VerticalScalingOpsRequest should change Cluster's resource requirements successfully", func() {
			ctx := verticalScalingContext{
				source: resourceContext{class: &testapps.Class1c1g},
				target: resourceContext{resource: testapps.Class2c4g.ToResourceRequirements()},
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
				func(g Gomega, rsm *workloads.ReplicatedStateMachine) {
					g.Expect(*rsm.Spec.Replicas).Should(Equal(replicas))
				})).Should(Succeed())
			rsm := componentWorkload()
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(rsm), func(sts *appsv1.StatefulSet) {
				sts.Spec.Replicas = &replicas
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentWorkload()),
				func(g Gomega, sts *appsv1.StatefulSet) {
					g.Expect(*sts.Spec.Replicas).Should(Equal(replicas))
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
			mockCompRunning(replicas)
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
				func(g Gomega, rsm *workloads.ReplicatedStateMachine) {
					g.Expect(*rsm.Spec.Replicas).Should(Equal(replicas))
				})).Should(Succeed())
			rsm := componentWorkload()
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(rsm), func(sts *appsv1.StatefulSet) {
				sts.Spec.Replicas = &replicas
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentWorkload()),
				func(g Gomega, sts *appsv1.StatefulSet) {
					g.Expect(*sts.Spec.Replicas).Should(Equal(replicas))
				})).Should(Succeed())

			By("mock scale down successfully by deleting one pod ")
			podName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, mysqlCompName, 2)
			dPodKeys := types.NamespacedName{Name: podName, Namespace: testCtx.DefaultNamespace}
			testapps.DeleteObject(&testCtx, dPodKeys, &corev1.Pod{})

			By("expect opsRequest phase to Succeed after cluster is Running")
			mockCompRunning(replicas)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(ops), func(g Gomega, ops *appsv1alpha1.OpsRequest) {
				g.Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsSucceedPhase))
				g.Expect(ops.Status.Progress).Should(Equal("1/1"))
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
			mockCompRunning(oldReplicas)

			By("create a horizontalScaling ops")
			ops := createClusterHscaleOps(5)
			opsKey := client.ObjectKeyFromObject(ops)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsRunningPhase))
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.UpdatingClusterPhase))

			By("create one pod")
			podName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, mysqlCompName, 3)
			pod := testapps.MockConsensusComponentStsPod(&testCtx, nil, clusterObj.Name, mysqlCompName, podName, "follower", "Readonly")

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

		It("test opsRequest queue", func() {
			By("create cluster and mock it to running")
			replicas := int32(3)
			createMysqlCluster(replicas)
			mockCompRunning(replicas)

			createRestartOps := func(clusterName string, index int) *appsv1alpha1.OpsRequest {
				opsName := fmt.Sprintf("restart-ops-%d", index)
				ops := testapps.NewOpsRequestObj(opsName, testCtx.DefaultNamespace,
					clusterName, appsv1alpha1.RestartType)
				ops.Spec.RestartList = []appsv1alpha1.ComponentOps{
					{ComponentName: mysqlCompName},
				}
				return testapps.CreateOpsRequest(ctx, testCtx, ops)
			}

			By("create first restart ops")
			ops1 := createRestartOps(clusterObj.Name, 1)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops1))).Should(Equal(appsv1alpha1.OpsRunningPhase))
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.UpdatingClusterPhase))

			By("create second horizontalScaling ops")
			ops2 := createRestartOps(clusterObj.Name, 2)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops2))).Should(Equal(appsv1alpha1.OpsPendingPhase))

			By("create third horizontalScaling ops")
			ops3 := createRestartOps(clusterObj.Name, 3)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops3))).Should(Equal(appsv1alpha1.OpsPendingPhase))

			By("expect for all opsRequests in queue")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterObj), func(g Gomega, cluster *appsv1alpha1.Cluster) {
				opsSlice, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
				g.Expect(len(opsSlice)).Should(Equal(3))
				// ops1 is running
				g.Expect(opsSlice[0].InQueue).Should(BeFalse())
				g.Expect(opsSlice[1].InQueue).Should(BeTrue())
				g.Expect(opsSlice[2].InQueue).Should(BeTrue())
			})).Should(Succeed())

			By("mock ops1 phase to Succeed")
			mockCompRunning(replicas)
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
	})
})
