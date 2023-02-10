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

package dbaas

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("OpsRequest Controller", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterNamePrefix = "test-cluster"

	const statefulCompType = "replicasets"
	const statefulCompName = "mysql"

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testdbaas.ClearResources(&testCtx, intctrlutil.OpsRequestSignature, inNS, ml)

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)
	}

	BeforeEach(func() {
		cleanEnv()

	})

	AfterEach(func() {
		cleanEnv()
	})

	var (
		clusterDefObj     *dbaasv1alpha1.ClusterDefinition
		clusterVersionObj *dbaasv1alpha1.ClusterVersion
		clusterObj        *dbaasv1alpha1.Cluster
		clusterKey        types.NamespacedName
	)

	// Testcases

	mockSetClusterStatusPhaseToRunning := func(namespacedName types.NamespacedName) {
		Eventually(testdbaas.GetAndChangeObjStatus(&testCtx, namespacedName,
			func(fetched *dbaasv1alpha1.Cluster) {
				fetched.Status.Phase = dbaasv1alpha1.RunningPhase
				for componentKey, componentStatus := range fetched.Status.Components {
					componentStatus.Phase = dbaasv1alpha1.RunningPhase
					fetched.Status.Components[componentKey] = componentStatus
				}
			})).Should(Succeed())
	}

	testVerticalScaleCPUAndMemory := func() {
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
		clusterObj = testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(statefulCompName, statefulCompType).
			SetReplicas(1).
			SetResources(resources).
			Create(&testCtx).GetCluster()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		Eventually(testdbaas.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("mock cluster status running")
		// MOCK pods are created and running, so as the cluster
		mockSetClusterStatusPhaseToRunning(clusterKey)

		By("send VerticalScalingOpsRequest successfully")
		opsKey := types.NamespacedName{Name: opsName, Namespace: testCtx.DefaultNamespace}
		verticalScalingOpsRequest := testdbaas.NewOpsRequestObj(opsKey.Name, opsKey.Namespace,
			clusterObj.Name, dbaasv1alpha1.VerticalScalingType)
		verticalScalingOpsRequest.Spec.TTLSecondsAfterSucceed = 0
		verticalScalingOpsRequest.Spec.VerticalScalingList = []dbaasv1alpha1.VerticalScaling{
			{
				ComponentOps: dbaasv1alpha1.ComponentOps{ComponentName: statefulCompName},
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
		Eventually(testdbaas.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(dbaasv1alpha1.RunningPhase))

		By("check Cluster and changed component phase is VerticalScaling")
		Eventually(testdbaas.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *dbaasv1alpha1.Cluster) {
			g.Expect(cluster.Status.Phase).To(Equal(dbaasv1alpha1.VerticalScalingPhase))
			g.Expect(cluster.Status.Components[statefulCompName].Phase).To(Equal(dbaasv1alpha1.VerticalScalingPhase))
		})).Should(Succeed())

		By("mock bring Cluster and changed component back to running status")
		mockSetClusterStatusPhaseToRunning(clusterKey)

		By("patch opsrequest controller to run")
		Eventually(testdbaas.ChangeObj(&testCtx, verticalScalingOpsRequest, func() {
			if verticalScalingOpsRequest.Annotations == nil {
				verticalScalingOpsRequest.Annotations = make(map[string]string, 1)
			}
			verticalScalingOpsRequest.Annotations[intctrlutil.OpsRequestReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
		})).Should(Succeed())

		By("check VerticalScalingOpsRequest succeed")
		Eventually(testdbaas.GetOpsRequestPhase(&testCtx, opsKey)).Should(Equal(dbaasv1alpha1.SucceedPhase))

		By("check cluster resource requirements changed")
		Eventually(testdbaas.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *dbaasv1alpha1.Cluster) {
			g.Expect(fetched.Spec.Components[0].Resources.Requests).To(Equal(
				verticalScalingOpsRequest.Spec.VerticalScalingList[0].Requests))
		})).Should(Succeed())

		By("check OpsRequest reclaimed after ttl")
		Expect(testdbaas.ChangeObj(&testCtx, verticalScalingOpsRequest, func() {
			verticalScalingOpsRequest.Spec.TTLSecondsAfterSucceed = 1
		})).Should(Succeed())

		Eventually(testdbaas.CheckObjExists(&testCtx, client.ObjectKeyFromObject(verticalScalingOpsRequest), verticalScalingOpsRequest, false)).Should(Succeed())
	}

	// Scenarios

	Context("with Cluster which has MySQL StatefulSet", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testdbaas.NewClusterDefFactory(clusterDefName, testdbaas.MySQLType).
				AddComponent(testdbaas.StatefulMySQL8, statefulCompType).
				Create(&testCtx).GetClusterDef()

			By("Create a clusterVersion obj")
			clusterVersionObj = testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(statefulCompType).AddContainerShort("mysql", testdbaas.ApeCloudMySQLImage).
				Create(&testCtx).GetClusterVersion()
		})

		It("issue an VerticalScalingOpsRequest should change Cluster's resource requirements successfully", func() {
			testVerticalScaleCPUAndMemory()
		})
	})

	Context("with Cluster which has MySQL ConsensusSet", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testdbaas.NewClusterDefFactory(clusterDefName, testdbaas.MySQLType).
				AddComponent(testdbaas.ConsensusMySQL, statefulCompType).
				Create(&testCtx).GetClusterDef()

			By("Create a clusterVersion obj")
			clusterVersionObj = testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(statefulCompType).AddContainerShort("mysql", testdbaas.ApeCloudMySQLImage).
				Create(&testCtx).GetClusterVersion()
		})

		It("issue an VerticalScalingOpsRequest should change Cluster's resource requirements successfully", func() {
			testVerticalScaleCPUAndMemory()
		})
	})
})
