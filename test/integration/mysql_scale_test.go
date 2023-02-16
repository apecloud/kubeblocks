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

package dbaastest

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("MySQL Scaling function", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterNamePrefix = "test-cluster"
	const scriptConfigName = "test-cluster-mysql-scripts"

	const mysqlCompType = "replicasets"
	const mysqlCompName = "mysql"

	// Cleanups

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.OpsRequestSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	// Testcases

	var (
		clusterDefObj     *dbaasv1alpha1.ClusterDefinition
		clusterVersionObj *dbaasv1alpha1.ClusterVersion
		clusterObj        *dbaasv1alpha1.Cluster
		clusterKey        types.NamespacedName
	)

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
			AddComponent(mysqlCompName, mysqlCompType).
			SetResources(resources).SetReplicas(1).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)
		Eventually(testdbaas.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("check cluster running")
		Eventually(testdbaas.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *dbaasv1alpha1.Cluster) {
			g.Expect(cluster.Status.Phase).To(Equal(dbaasv1alpha1.RunningPhase))
		})).Should(Succeed())

		By("send VerticalScalingOpsRequest successfully")
		opsKey := types.NamespacedName{Name: opsName, Namespace: testCtx.DefaultNamespace}
		verticalScalingOpsRequest := testdbaas.NewOpsRequestObj(opsKey.Name, opsKey.Namespace,
			clusterObj.Name, dbaasv1alpha1.VerticalScalingType)
		verticalScalingOpsRequest.Spec.TTLSecondsAfterSucceed = 0
		verticalScalingOpsRequest.Spec.VerticalScalingList = []dbaasv1alpha1.VerticalScaling{
			{
				ComponentOps: dbaasv1alpha1.ComponentOps{ComponentName: mysqlCompName},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"cpu":    resource.MustParse("400m"),
						"memory": resource.MustParse("300Mi"),
					},
				},
			},
		}
		Expect(testCtx.CreateObj(testCtx.Ctx, verticalScalingOpsRequest)).Should(Succeed())

		By("check VerticalScalingOpsRequest succeed")
		Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(verticalScalingOpsRequest),
			func(g Gomega, ops *dbaasv1alpha1.OpsRequest) {
				g.Expect(ops.Status.Phase == dbaasv1alpha1.SucceedPhase).To(BeTrue())
			})).Should(Succeed())

		By("check cluster resource requirements changed")
		Eventually(testdbaas.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *dbaasv1alpha1.Cluster) {
			g.Expect(fetched.Spec.ComponentSpecs[0].Resources.Requests).To(Equal(
				verticalScalingOpsRequest.Spec.VerticalScalingList[0].Requests))
		})).Should(Succeed())

		By("check OpsRequest reclaimed after ttl")
		Expect(testdbaas.ChangeObj(&testCtx, verticalScalingOpsRequest, func() {
			verticalScalingOpsRequest.Spec.TTLSecondsAfterSucceed = 1
		})).Should(Succeed())

		By("OpsRequest reclaimed after ttl")
		Eventually(func() error {
			return k8sClient.Get(testCtx.Ctx, client.ObjectKeyFromObject(verticalScalingOpsRequest), verticalScalingOpsRequest)
		}).Should(Satisfy(apierrors.IsNotFound))
	}

	testVerticalScaleStorage := func() {
		oldStorageValue := resource.MustParse("1Gi")
		newStorageValue := resource.MustParse("2Gi")

		By("Check StorageClass")
		defaultStorageClass := testk8s.GetDefaultStorageClass(&testCtx)
		if defaultStorageClass == nil {
			Skip("No default StorageClass found")
		} else if !(defaultStorageClass.AllowVolumeExpansion != nil && *defaultStorageClass.AllowVolumeExpansion) {
			Skip("Default StorageClass doesn't allow resize")
		}

		By("Create a cluster obj with both log and data volume of 1GB size")
		dataPvcSpec := corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: oldStorageValue,
				},
			},
		}
		logPvcSpec := dataPvcSpec
		logPvcSpec.StorageClassName = &defaultStorageClass.Name
		clusterObj = testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompType).
			AddVolumeClaimTemplate(testdbaas.DataVolumeName, &dataPvcSpec).
			AddVolumeClaimTemplate(testdbaas.LogVolumeName, &logPvcSpec).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		Eventually(testdbaas.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Check the replicas")
		Eventually(func(g Gomega) {
			stsList := &appsv1.StatefulSetList{}
			g.Expect(k8sClient.List(testCtx.Ctx, stsList, client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey: clusterKey.Name,
			}, client.InNamespace(clusterKey.Namespace))).To(Succeed())
			g.Expect(len(stsList.Items) > 0).To(BeTrue())
			sts := &stsList.Items[0]
			g.Expect(sts.Spec.Replicas).NotTo(BeNil())
			g.Expect(sts.Status.AvailableReplicas).To(Equal(*sts.Spec.Replicas))
		}).Should(Succeed())

		By("Check the pvc")
		Eventually(func() bool {
			pvcList := &corev1.PersistentVolumeClaimList{}
			Expect(k8sClient.List(testCtx.Ctx, pvcList, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
			return len(pvcList.Items) != 0
		}).Should(BeTrue())

		By("Update volume size")
		Eventually(testdbaas.GetAndChangeObj(&testCtx, clusterKey, func(fetched *dbaasv1alpha1.Cluster) {
			comp := &fetched.Spec.ComponentSpecs[0]
			comp.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue
			comp.VolumeClaimTemplates[1].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue
		})).Should(Succeed())

		Eventually(testdbaas.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))

		By("Checking the PVC")
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		for _, sts := range stsList.Items {
			for _, vct := range sts.Spec.VolumeClaimTemplates {
				for i := *sts.Spec.Replicas - 1; i >= 0; i-- {
					pvc := &corev1.PersistentVolumeClaim{}
					pvcKey := types.NamespacedName{
						Namespace: clusterKey.Namespace,
						Name:      fmt.Sprintf("%s-%s-%d", vct.Name, sts.Name, i),
					}
					Expect(k8sClient.Get(testCtx.Ctx, pvcKey, pvc)).Should(Succeed())
					Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(newStorageValue))
				}
			}
		}
	}

	// Scenarios

	Context("with MySQL defined as a stateful component", func() {
		BeforeEach(func() {
			_ = testdbaas.CreateCustomizedObj(&testCtx, "resources/mysql_scripts.yaml", &corev1.ConfigMap{},
				testdbaas.WithName(scriptConfigName), testCtx.UseDefaultNamespace())

			By("Create a clusterDef obj")
			mode := int32(0755)
			clusterDefObj = testdbaas.NewClusterDefFactory(clusterDefName).
				AddComponent(testdbaas.StatefulMySQLComponent, mysqlCompType).
				AddConfigTemplate(scriptConfigName, scriptConfigName, "", testdbaas.ScriptsVolumeName, &mode).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(mysqlCompType).AddContainerShort(testdbaas.DefaultMySQLContainerName, testdbaas.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()
		})

		It("should handle VerticalScalingOpsRequest and change Cluster's cpu&memory requirements", func() {
			testVerticalScaleCPUAndMemory()
		})

		It("should handle PVC resize requests if cluster has storage class which enables dynamic-provisioning", func() {
			testVerticalScaleStorage()
		})
	})

	Context("with MySQL defined as a consensus component", func() {
		BeforeEach(func() {
			By("Create configmap")
			_ = testdbaas.CreateCustomizedObj(&testCtx, "resources/mysql_scripts.yaml", &corev1.ConfigMap{},
				testdbaas.WithName(scriptConfigName), testCtx.UseDefaultNamespace())

			By("Create a clusterDef obj")
			mode := int32(0755)
			clusterDefObj = testdbaas.NewClusterDefFactory(clusterDefName).
				AddComponent(testdbaas.ConsensusMySQLComponent, mysqlCompType).
				AddConfigTemplate(scriptConfigName, scriptConfigName, "", testdbaas.ScriptsVolumeName, &mode).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(mysqlCompType).AddContainerShort(testdbaas.DefaultMySQLContainerName, testdbaas.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()
		})

		It("should handle VerticalScalingOpsRequest and change Cluster's cpu&memory requirements", func() {
			testVerticalScaleCPUAndMemory()
		})

		It("should handle PVC resize requests if cluster has storage class which enables dynamic-provisioning", func() {
			testVerticalScaleStorage()
		})
	})
})
