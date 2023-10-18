/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package appstest

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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("MySQL Scaling function", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterNamePrefix = "test-cluster"
	const scriptConfigName = "test-cluster-mysql-scripts"
	const mysqlCompDefName = "replicasets"
	const mysqlCompName = "mysql"

	// Cleanups

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, intctrlutil.OpsRequestSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	// Testcases

	var (
		clusterDefObj     *appsv1alpha1.ClusterDefinition
		clusterVersionObj *appsv1alpha1.ClusterVersion
		clusterObj        *appsv1alpha1.Cluster
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
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompDefName).
			SetResources(resources).SetReplicas(1).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("check cluster running")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
			g.Expect(cluster.Status.Phase).To(Equal(appsv1alpha1.RunningClusterPhase))
		})).Should(Succeed())

		By("send VerticalScalingOpsRequest successfully")
		opsKey := types.NamespacedName{Name: opsName, Namespace: testCtx.DefaultNamespace}
		verticalScalingOpsRequest := testapps.NewOpsRequestObj(opsKey.Name, opsKey.Namespace,
			clusterObj.Name, appsv1alpha1.VerticalScalingType)
		verticalScalingOpsRequest.Spec.TTLSecondsAfterSucceed = 0
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

		By("check VerticalScalingOpsRequest succeed")
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(verticalScalingOpsRequest),
			func(g Gomega, ops *appsv1alpha1.OpsRequest) {
				g.Expect(ops.Status.Phase == appsv1alpha1.OpsSucceedPhase).To(BeTrue())
			})).Should(Succeed())

		By("check cluster resource requirements changed")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
			g.Expect(fetched.Spec.ComponentSpecs[0].Resources.Requests).To(Equal(
				verticalScalingOpsRequest.Spec.VerticalScalingList[0].Requests))
		})).Should(Succeed())

		By("check OpsRequest reclaimed after ttl")
		Expect(testapps.ChangeObj(&testCtx, verticalScalingOpsRequest, func(lopsReq *appsv1alpha1.OpsRequest) {
			lopsReq.Spec.TTLSecondsAfterSucceed = 1
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
		dataPvcSpec := testapps.NewPVCSpec(oldStorageValue.String())
		logPvcSpec := dataPvcSpec
		logPvcSpec.StorageClassName = &defaultStorageClass.Name
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompDefName).
			AddVolumeClaimTemplate(testapps.DataVolumeName, dataPvcSpec).
			AddVolumeClaimTemplate(testapps.LogVolumeName, logPvcSpec).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Check the replicas")
		Eventually(func(g Gomega) {
			stsList := &appsv1.StatefulSetList{}
			g.Expect(k8sClient.List(testCtx.Ctx, stsList, client.MatchingLabels{
				constant.AppInstanceLabelKey: clusterKey.Name,
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
		Eventually(testapps.GetAndChangeObj(&testCtx, clusterKey, func(fetched *appsv1alpha1.Cluster) {
			comp := &fetched.Spec.ComponentSpecs[0]
			comp.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue
			comp.VolumeClaimTemplates[1].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue
		})).Should(Succeed())

		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))

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
			_ = testapps.CreateCustomizedObj(&testCtx, "resources/mysql-scripts.yaml", &corev1.ConfigMap{},
				testapps.WithName(scriptConfigName), testCtx.UseDefaultNamespace())

			By("Create a clusterDef obj")
			mode := int32(0755)
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
				AddScriptTemplate(scriptConfigName, scriptConfigName, testCtx.DefaultNamespace, testapps.ScriptsVolumeName, &mode).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponentVersion(mysqlCompDefName).AddContainerShort(testapps.DefaultMySQLContainerName, testapps.ApeCloudMySQLImage).
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
			_ = testapps.CreateCustomizedObj(&testCtx, "resources/mysql-scripts.yaml", &corev1.ConfigMap{},
				testapps.WithName(scriptConfigName), testCtx.UseDefaultNamespace())

			By("Create a clusterDef obj")
			mode := int32(0755)
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ConsensusMySQLComponent, mysqlCompDefName).
				AddScriptTemplate(scriptConfigName, scriptConfigName, testCtx.DefaultNamespace, testapps.ScriptsVolumeName, &mode).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponentVersion(mysqlCompDefName).AddContainerShort(testapps.DefaultMySQLContainerName, testapps.ApeCloudMySQLImage).
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
