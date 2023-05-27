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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replication"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("OpsRequest Controller", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterNamePrefix = "test-cluster"
	const mysqlCompDefName = "mysql"
	const mysqlCompName = "mysql"
	const defaultMinReadySeconds = 10

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.OpsRequestSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.VolumeSnapshotSignature, inNS)

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)
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

	// Testcases

	// checkLatestOpsIsProcessing := func(clusterKey client.ObjectKey, opsType appsv1alpha1.OpsType) {
	//	Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
	//		con := meta.FindStatusCondition(fetched.Status.Conditions, appsv1alpha1.ConditionTypeLatestOpsRequestProcessed)
	//		g.Expect(con).ShouldNot(BeNil())
	//		g.Expect(con.Status).Should(Equal(metav1.ConditionFalse))
	//		g.Expect(con.Reason).Should(Equal(appsv1alpha1.OpsRequestBehaviourMapper[opsType].ProcessingReasonInClusterCondition))
	//	})).Should(Succeed())
	// }
	//
	// checkLatestOpsHasProcessed := func(clusterKey client.ObjectKey) {
	//	Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
	//		con := meta.FindStatusCondition(fetched.Status.Conditions, appsv1alpha1.ConditionTypeLatestOpsRequestProcessed)
	//		g.Expect(con).ShouldNot(BeNil())
	//		g.Expect(con.Status).Should(Equal(metav1.ConditionTrue))
	//		g.Expect(con.Reason).Should(Equal(lifecycle.ReasonOpsRequestProcessed))
	//	})).Should(Succeed())
	// }

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
		constraint := testapps.NewComponentResourceConstraintFactory(testapps.DefaultResourceConstraintName).
			AddConstraints(testapps.GeneralResourceConstraint).
			Create(&testCtx).GetObject()

		testapps.NewComponentClassDefinitionFactory("custom", clusterDefObj.Name, mysqlCompDefName).
			AddClasses(constraint.Name, []string{testapps.Class1c1gName, testapps.Class2c4gName}).
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
		stsList := testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, clusterKey, mysqlCompName)
		mysqlSts := stsList.Items[0]
		Expect(testapps.ChangeObjStatus(&testCtx, &mysqlSts, func() {
			testk8s.MockStatefulSetReady(&mysqlSts)
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
					Class:        scalingCtx.target.class.Name,
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
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.SpecReconcilingClusterPhase))
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, mysqlCompName)).Should(Equal(appsv1alpha1.SpecReconcilingClusterCompPhase))
		// TODO(refactor): try to check some ephemeral states?
		// checkLatestOpsIsProcessing(clusterKey, verticalScalingOpsRequest.Spec.Type)

		// By("check Cluster and changed component phase is VerticalScaling")
		// Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
		//	g.Expect(cluster.Status.Phase).To(Equal(appsv1alpha1.SpecReconcilingClusterPhase))
		//	g.Expect(cluster.Status.Components[mysqlCompName].Phase).To(Equal(appsv1alpha1.SpecReconcilingClusterCompPhase))
		// })).Should(Succeed())

		By("mock bring Cluster and changed component back to running status")
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(&mysqlSts), func(tmpSts *appsv1.StatefulSet) {
			testk8s.MockStatefulSetReady(tmpSts)
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
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
			g.Expect(fetched.Spec.ComponentSpecs[0].Resources.Requests).To(Equal(targetRequests))
		})).Should(Succeed())

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

		It("create cluster by class, vertical scaling by class", func() {
			ctx := verticalScalingContext{
				source: resourceContext{class: &testapps.Class1c1g},
				target: resourceContext{class: &testapps.Class2c4g},
			}
			testVerticalScaleCPUAndMemory(testapps.StatefulMySQLComponent, ctx)
		})

		It("create cluster by resource, vertical scaling by class", func() {
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
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ConsensusMySQLComponent, mysqlCompDefName).
				AddHorizontalScalePolicy(appsv1alpha1.HorizontalScalePolicy{
					Type:                     appsv1alpha1.HScaleDataClonePolicyFromSnapshot,
					BackupPolicyTemplateName: backupPolicyTPLName,
				}).Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponentVersion(mysqlCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()
		})

		componentWorkload := func() *appsv1.StatefulSet {
			stsList := testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, clusterKey, mysqlCompName)
			return &stsList.Items[0]
		}

		mockCompRunning := func(replicas int32) {
			sts := componentWorkload()
			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				testk8s.MockStatefulSetReady(sts)
			})).ShouldNot(HaveOccurred())
			for i := 0; i < int(replicas); i++ {
				podName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, mysqlCompName, i)
				podRole := "follower"
				accessMode := "Readonly"
				if i == 0 {
					podRole = "leader"
					accessMode = "ReadWrite"
				}
				testapps.MockConsensusComponentStsPod(&testCtx, sts, clusterObj.Name, mysqlCompName, podName, podRole, accessMode)
			}
			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, mysqlCompName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		}

		createMysqlCluster := func(replicas int32) {
			createBackupPolicyTpl(clusterDefObj)

			By("set component to horizontal with snapshot policy and create a cluster")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
				func(clusterDef *appsv1alpha1.ClusterDefinition) {
					clusterDef.Spec.ComponentDefs[0].HorizontalScalePolicy =
						&appsv1alpha1.HorizontalScalePolicy{Type: appsv1alpha1.HScaleDataClonePolicyFromSnapshot}
				})()).ShouldNot(HaveOccurred())
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
					mysqlCompName, testapps.DataVolumeName).SetStorage("1Gi").Create(&testCtx).GetObject()
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
			viper.Set("VOLUMESNAPSHOT", false)

			createMysqlCluster(3)
			cluster := &appsv1alpha1.Cluster{}
			Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(Succeed())
			initGeneration := cluster.Status.ObservedGeneration
			Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(Equal(initGeneration))

			ops := createClusterHscaleOps(5)
			opsKey := client.ObjectKeyFromObject(ops)

			By("expect component is Running if don't support volume snapshot during doing h-scale ops")
			Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsRunningPhase))
			// Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, lcluster *appsv1alpha1.Cluster) {
			//	// the cluster spec has been updated by ops-controller to scale out.
			//	g.Expect(lcluster.Generation == initGeneration+1).Should(BeTrue())
			//	// but the new spec can't be reconciled successfully because of volume snapshot is disabled.
			//	g.Expect(lcluster.Generation > lcluster.Status.ObservedGeneration).Should(BeTrue())
			//	g.Expect(cluster.Status.Components[mysqlCompName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
			//	g.Expect(cluster.Status.Phase).Should(Equal(appsv1alpha1.RunningClusterPhase))
			// })).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
				// the cluster spec has been updated by ops-controller to scale out.
				g.Expect(fetched.Generation == initGeneration+1).Should(BeTrue())
				// expect cluster phase is Updating during Hscale.
				g.Expect(fetched.Generation > fetched.Status.ObservedGeneration).Should(BeTrue())
				g.Expect(fetched.Status.Phase).Should(Equal(appsv1alpha1.SpecReconcilingClusterPhase))
				// when snapshot is not supported, the expected component phase is running.
				g.Expect(fetched.Status.Components[mysqlCompName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
				// expect preCheckFailed condition to occur.
				condition := meta.FindStatusCondition(fetched.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Status).Should(BeFalse())
				g.Expect(condition.Reason).Should(Equal(lifecycle.ReasonPreCheckFailed))
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

		// TODO(refactor): review the case before merge.
		It("HorizontalScaling via volume snapshot backup", func() {
			By("init backup policy template, mysql cluster and hscale ops")
			viper.Set("VOLUMESNAPSHOT", true)
			createMysqlCluster(3)

			replicas := int32(5)
			ops := createClusterHscaleOps(replicas)
			opsKey := client.ObjectKeyFromObject(ops)

			By("expect cluster and component is reconciling the new spec")
			Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsRunningPhase))
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Generation == 2).Should(BeTrue())
				g.Expect(cluster.Status.ObservedGeneration == 2).Should(BeTrue())
				// component phase should be running during snapshot backup
				// g.Expect(cluster.Status.Components[mysqlCompName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
				g.Expect(cluster.Status.Components[mysqlCompName].Phase).Should(Equal(appsv1alpha1.SpecReconcilingClusterCompPhase))
				// the expected cluster phase is Updating during Hscale.
				g.Expect(cluster.Status.Phase).Should(Equal(appsv1alpha1.RunningClusterPhase))
			})).Should(Succeed())

			By("mock VolumeSnapshot status is ready, component phase should change to Updating when component is horizontally scaling.")
			snapshotKey := types.NamespacedName{Name: fmt.Sprintf("%s-%s-scaling",
				clusterKey.Name, mysqlCompName), Namespace: testCtx.DefaultNamespace}
			volumeSnapshot := &snapshotv1.VolumeSnapshot{}
			Expect(k8sClient.Get(testCtx.Ctx, snapshotKey, volumeSnapshot)).Should(Succeed())
			readyToUse := true
			volumeSnapshot.Status = &snapshotv1.VolumeSnapshotStatus{ReadyToUse: &readyToUse}
			Expect(k8sClient.Status().Update(testCtx.Ctx, volumeSnapshot)).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Status.Components[mysqlCompName].Phase).Should(Equal(appsv1alpha1.SpecReconcilingClusterCompPhase))
				// g.Expect(cluster.Status.Phase).Should(Equal(appsv1alpha1.SpecReconcilingClusterPhase))
				g.Expect(cluster.Status.Phase).Should(Equal(appsv1alpha1.RunningClusterPhase))
			})).Should(Succeed())

			By("wait the underlying workload been updated")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentWorkload()),
				func(g Gomega, sts *appsv1.StatefulSet) {
					g.Expect(*sts.Spec.Replicas).Should(Equal(replicas))
				})).Should(Succeed())

			By("mock component workload is running and expect cluster and component are running")
			mockCompRunning(replicas)
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Status.Components[mysqlCompName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
				g.Expect(cluster.Status.Phase).Should(Equal(appsv1alpha1.RunningClusterPhase))
			})).Should(Succeed())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsSucceedPhase))
		})
	})

	Context("with Cluster which has redis Replication", func() {
		var podList []*corev1.Pod
		var stsList = &appsv1.StatefulSetList{}

		createStsPodAndMockStsReady := func() {
			Eventually(testapps.List(&testCtx, intctrlutil.StatefulSetSignature,
				client.MatchingLabels{
					constant.AppInstanceLabelKey: clusterObj.Name,
				}, client.InNamespace(clusterObj.Namespace))).Should(HaveLen(1))
			stsList = testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, client.ObjectKeyFromObject(clusterObj),
				testapps.DefaultRedisCompName)
			Expect(stsList.Items).Should(HaveLen(1))
			sts := &stsList.Items[0]
			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				testk8s.MockStatefulSetReady(sts)
			})).ShouldNot(HaveOccurred())
			for i := int32(0); i < *sts.Spec.Replicas; i++ {
				podName := fmt.Sprintf("%s-%d", sts.Name, i)
				pod := testapps.MockReplicationComponentPod(nil, testCtx, sts, clusterObj.Name,
					testapps.DefaultRedisCompName, podName, replication.DefaultRole(i))
				podList = append(podList, pod)
			}
		}
		BeforeEach(func() {
			By("init replication cluster")
			// init storageClass
			storageClassName := "standard"
			testapps.CreateStorageClass(&testCtx, storageClassName, true)
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompDefName).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication workloadType.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponentVersion(testapps.DefaultRedisCompDefName).AddContainerShort(testapps.DefaultRedisContainerName,
				testapps.DefaultRedisImageName).
				Create(&testCtx).GetObject()

			By("Creating a cluster with replication workloadType.")
			pvcSpec := testapps.NewPVCSpec("1Gi")
			pvcSpec.StorageClassName = &storageClassName
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompDefName).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).SetPrimaryIndex(0).
				SetReplicas(testapps.DefaultReplicationReplicas).
				Create(&testCtx).GetObject()
			// mock sts ready and create pod
			createStsPodAndMockStsReady()
			// mock pvc creation
			for i := 0; i < testapps.DefaultReplicationReplicas; i++ {
				pvcName := fmt.Sprintf("%s-%s-%s-%d", testapps.DataVolumeName, clusterObj.Name, mysqlCompName, i)
				testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterObj.Name,
					mysqlCompName, testapps.DataVolumeName).AddLabels(constant.AppInstanceLabelKey, clusterObj.Name,
					constant.VolumeClaimTemplateNameLabelKey, testapps.DataVolumeName,
					constant.KBAppComponentLabelKey, testapps.DefaultRedisCompName).
					SetStorage("1Gi").SetStorageClass(storageClassName).Create(&testCtx).GetObject()
			}
			// wait for cluster to running
			Eventually(testapps.GetClusterPhase(&testCtx, client.ObjectKeyFromObject(clusterObj))).Should(Equal(appsv1alpha1.RunningClusterPhase))
		})

		It("delete Running opsRequest", func() {
			By("Create a volume-expand ops")
			opsName := "volume-expand" + testCtx.GetRandomStr()
			volumeExpandOps := testapps.NewOpsRequestObj(opsName, clusterObj.Namespace,
				clusterObj.Name, appsv1alpha1.VolumeExpansionType)
			volumeExpandOps.Spec.VolumeExpansionList = []appsv1alpha1.VolumeExpansion{
				{
					ComponentOps: appsv1alpha1.ComponentOps{ComponentName: testapps.DefaultRedisCompName},
					VolumeClaimTemplates: []appsv1alpha1.OpsRequestVolumeClaimTemplate{
						{
							Name:    testapps.DataVolumeName,
							Storage: resource.MustParse("3Gi"),
						},
					},
				},
			}
			Expect(testCtx.CreateObj(testCtx.Ctx, volumeExpandOps)).Should(Succeed())
			clusterKey = client.ObjectKeyFromObject(clusterObj)
			opsKey := client.ObjectKeyFromObject(volumeExpandOps)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsRunningPhase))

			By("delete the Running ops")
			testapps.DeleteObject(&testCtx, opsKey, volumeExpandOps)
			Expect(testapps.ChangeObj(&testCtx, volumeExpandOps, func(lopsReq *appsv1alpha1.OpsRequest) {
				lopsReq.SetFinalizers([]string{})
			})).ShouldNot(HaveOccurred())

			By("check the cluster annotation")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmlCluster *appsv1alpha1.Cluster) {
				opsSlice, _ := opsutil.GetOpsRequestSliceFromCluster(tmlCluster)
				g.Expect(opsSlice).Should(HaveLen(0))
			})).Should(Succeed())
		})
	})
})
