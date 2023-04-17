/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apps

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	const mysqlCompDefName = "consensus"
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
		testapps.ClearResources(&testCtx, intctrlutil.OpsRequestSignature, inNS, ml)

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)
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

	// Testcases

	checkLatestOpsIsProcessing := func(clusterKey client.ObjectKey, opsType appsv1alpha1.OpsType) {
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
			con := meta.FindStatusCondition(fetched.Status.Conditions, appsv1alpha1.ConditionTypeLatestOpsRequestProcessed)
			g.Expect(con).ShouldNot(BeNil())
			g.Expect(con.Status).Should(Equal(metav1.ConditionFalse))
			g.Expect(con.Reason).Should(Equal(appsv1alpha1.OpsRequestBehaviourMapper[opsType].ProcessingReasonInClusterCondition))
		})).Should(Succeed())
	}

	checkLatestOpsHasProcessed := func(clusterKey client.ObjectKey) {
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
			con := meta.FindStatusCondition(fetched.Status.Conditions, appsv1alpha1.ConditionTypeLatestOpsRequestProcessed)
			g.Expect(con).ShouldNot(BeNil())
			g.Expect(con.Status).Should(Equal(metav1.ConditionTrue))
			g.Expect(con.Reason).Should(Equal(lifecycle.ReasonOpsRequestProcessed))
		})).Should(Succeed())
	}

	mockSetClusterStatusPhaseToRunning := func(namespacedName types.NamespacedName) {
		Expect(testapps.GetAndChangeObjStatus(&testCtx, namespacedName,
			func(fetched *appsv1alpha1.Cluster) {
				// TODO: whould be better to have hint for cluster.status.phase is mocked,
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

	testVerticalScaleCPUAndMemory := func(workloadType testapps.ComponentDefTplType) {
		const opsName = "mysql-verticalscaling"

		By("Create a cluster obj")
		resources := corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"cpu":    resource.MustParse("800m"),
				"memory": resource.MustParse("512Mi"),
			},
			Requests: corev1.ResourceList{
				"cpu":    resource.MustParse("500m"),
				"memory": resource.MustParse("256Mi"),
			},
		}
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompDefName).
			SetReplicas(1).
			SetResources(resources).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))

		By("mock pod/sts are available and wait for cluster enter running phase")
		podName := fmt.Sprintf("%s-%s-0", clusterObj.Name, mysqlCompName)
		pod := testapps.MockConsensusComponentStsPod(testCtx, nil, clusterObj.Name, mysqlCompName,
			podName, "leader", "ReadWrite")
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
		verticalScalingOpsRequest.Spec.VerticalScalingList = []appsv1alpha1.VerticalScaling{
			{
				ComponentOps: appsv1alpha1.ComponentOps{ComponentName: mysqlCompName},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"cpu":    resource.MustParse("400m"),
						"memory": resource.MustParse("300Mi"),
					},
				},
			},
		}
		Expect(testCtx.CreateObj(testCtx.Ctx, verticalScalingOpsRequest)).Should(Succeed())

		By("check VerticalScalingOpsRequest running")
		Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsRunningPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.SpecReconcilingClusterPhase))
		Eventually(testapps.GetClusterComponentPhase(testCtx, clusterObj.Name, mysqlCompName)).Should(Equal(appsv1alpha1.SpecReconcilingClusterCompPhase))
		checkLatestOpsIsProcessing(clusterKey, verticalScalingOpsRequest.Spec.Type)

		By("check Cluster and changed component phase is VerticalScaling")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
			g.Expect(cluster.Status.Phase).To(Equal(appsv1alpha1.SpecReconcilingClusterPhase))                               // VerticalScalingPhase
			g.Expect(cluster.Status.Components[mysqlCompName].Phase).To(Equal(appsv1alpha1.SpecReconcilingClusterCompPhase)) // VerticalScalingPhase
		})).Should(Succeed())

		By("mock bring Cluster and changed component back to running status")
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(&mysqlSts), func(tmpSts *appsv1.StatefulSet) {
			testk8s.MockStatefulSetReady(tmpSts)
		})()).ShouldNot(HaveOccurred())
		Eventually(testapps.GetClusterComponentPhase(testCtx, clusterObj.Name, mysqlCompName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))
		checkLatestOpsHasProcessed(clusterKey)

		By("patch opsrequest controller to run")
		Expect(testapps.ChangeObj(&testCtx, verticalScalingOpsRequest, func(lopsReq *appsv1alpha1.OpsRequest) {
			if lopsReq.Annotations == nil {
				lopsReq.Annotations = map[string]string{}
			}
			lopsReq.Annotations[constant.ReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
		})).ShouldNot(HaveOccurred())

		By("check VerticalScalingOpsRequest succeed")
		Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsSucceedPhase))

		By("check cluster resource requirements changed")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
			g.Expect(fetched.Spec.ComponentSpecs[0].Resources.Requests).To(Equal(
				verticalScalingOpsRequest.Spec.VerticalScalingList[0].Requests))
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
				AddComponent(mysqlCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()
		})

		It("issue an VerticalScalingOpsRequest should change Cluster's resource requirements successfully", func() {
			testVerticalScaleCPUAndMemory(testapps.StatefulMySQLComponent)
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
				AddComponent(mysqlCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()
		})

		It("issue an VerticalScalingOpsRequest should change Cluster's resource requirements successfully", func() {
			testVerticalScaleCPUAndMemory(testapps.ConsensusMySQLComponent)
		})

		It("HorizontalScaling when not support snapshot", func() {
			By("init backup policy template")
			viper.Set("VOLUMESNAPSHOT", false)
			createBackupPolicyTpl(clusterDefObj)
			replicas := int32(3)

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
			stsList := testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, clusterKey, mysqlCompName)
			sts := &stsList.Items[0]
			Expect(int(*sts.Spec.Replicas)).To(BeEquivalentTo(replicas))
			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				testk8s.MockStatefulSetReady(sts)
			})).ShouldNot(HaveOccurred())
			testapps.MockConsensusComponentPods(testCtx, sts, clusterKey.Name, mysqlCompName)
			Eventually(testapps.GetClusterComponentPhase(testCtx, clusterKey.Name, mysqlCompName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))

			By("mock pvc created")
			for i := 0; i < int(replicas); i++ {
				pvcName := fmt.Sprintf("%s-%s-%s-%d", testapps.DataVolumeName, clusterKey.Name, mysqlCompName, i)
				pvc := testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterKey.Name,
					mysqlCompName, "data").SetStorage("1Gi").Create(&testCtx).GetObject()
				// mock pvc bound
				Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(pvc), func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Phase = corev1.ClaimBound
				})()).ShouldNot(HaveOccurred())
			}
			// wait for cluster observed generation
			Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
			mockSetClusterStatusPhaseToRunning(clusterKey)

			By("create a opsRequest to horizontal scale")
			opsName := "hscale-ops-" + testCtx.GetRandomStr()
			ops := testapps.NewOpsRequestObj(opsName, testCtx.DefaultNamespace,
				clusterObj.Name, appsv1alpha1.HorizontalScalingType)
			ops.Spec.HorizontalScalingList = []appsv1alpha1.HorizontalScaling{
				{
					ComponentOps: appsv1alpha1.ComponentOps{ComponentName: mysqlCompName},
					Replicas:     int32(5),
				},
			}
			opsKey := client.ObjectKeyFromObject(ops)
			Expect(testCtx.CreateObj(testCtx.Ctx, ops)).Should(Succeed())

			By("expect component is Running if don't support volume snapshot during doing h-scale ops")
			Eventually(testapps.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(appsv1alpha1.OpsRunningPhase))
			// cluster phase changes to HorizontalScalingPhase first. then, it will be ConditionsError because it does not support snapshot backup after a period of time.
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.SpecReconcilingClusterPhase)) // HorizontalScalingPhase
			Eventually(testapps.GetClusterComponentPhase(testCtx, clusterKey.Name, mysqlCompName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))

			By("delete h-scale ops")
			testapps.DeleteObject(&testCtx, opsKey, ops)
			Expect(testapps.ChangeObj(&testCtx, ops, func(lopsReq *appsv1alpha1.OpsRequest) {
				lopsReq.SetFinalizers([]string{})
			})).ShouldNot(HaveOccurred())

			By("reset replicas to 1 and cluster should reconcile to Running")
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
				cluster.Spec.ComponentSpecs[0].Replicas = int32(3)
			})()).ShouldNot(HaveOccurred())
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))
		})
	})

	Context("with Cluster which has redis Replication", func() {
		var podList []*corev1.Pod
		var stsList = &appsv1.StatefulSetList{}

		createStsPodAndMockStsReady := func() {
			Eventually(testapps.GetListLen(&testCtx, intctrlutil.StatefulSetSignature,
				client.MatchingLabels{
					constant.AppInstanceLabelKey: clusterObj.Name,
				}, client.InNamespace(clusterObj.Namespace))).Should(BeEquivalentTo(1))
			stsList = testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, client.ObjectKeyFromObject(clusterObj),
				testapps.DefaultRedisCompName)
			Expect(stsList.Items).Should(HaveLen(1))
			sts := &stsList.Items[0]
			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				testk8s.MockStatefulSetReady(sts)
			})).ShouldNot(HaveOccurred())
			for i := int32(0); i < *sts.Spec.Replicas; i++ {
				podName := fmt.Sprintf("%s-%d", sts.Name, i)
				pod := testapps.MockReplicationComponentStsPod(nil, testCtx, sts, clusterObj.Name,
					testapps.DefaultRedisCompName, podName, replication.DefaultRole(i))
				podList = append(podList, pod)
			}
		}
		BeforeEach(func() {
			By("init replication cluster")
			// init storageClass
			storageClassName := "standard"
			testapps.CreateStorageClass(testCtx, storageClassName, true)
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompDefName).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication workloadType.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponent(testapps.DefaultRedisCompDefName).AddContainerShort(testapps.DefaultRedisContainerName,
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
			// wait for cluster to running
			Eventually(testapps.GetClusterPhase(&testCtx, client.ObjectKeyFromObject(clusterObj))).
				Should(Equal(appsv1alpha1.RunningClusterPhase))
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
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmlCluster *appsv1alpha1.Cluster) {
				opsSlice, _ := opsutil.GetOpsRequestSliceFromCluster(tmlCluster)
				g.Expect(opsSlice).Should(HaveLen(1))
				g.Expect(tmlCluster.Status.Components[testapps.DefaultRedisCompName].Phase).Should(Equal(appsv1alpha1.SpecReconcilingClusterCompPhase)) // VolumeExpandingPhase
				// TODO: status conditions for VolumeExpandingPhase
			})).Should(Succeed())

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
