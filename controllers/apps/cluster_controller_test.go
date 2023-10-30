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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Cluster Controller", func() {
	const (
		clusterDefName     = "test-clusterdef"
		clusterVersionName = "test-clusterversion"
		compDefName        = "test-compdef"
		clusterName        = "test-cluster" // this become cluster prefix name if used with testapps.NewClusterFactory().WithRandomName()
		// REVIEW:
		// - setup componentName and componentDefName as map entry pair
		statelessCompName      = "stateless"
		statelessCompDefName   = "stateless"
		statefulCompName       = "stateful"
		statefulCompDefName    = "stateful"
		consensusCompName      = "consensus"
		consensusCompDefName   = "consensus"
		replicationCompName    = "replication"
		replicationCompDefName = "replication"
	)

	var (
		clusterDefObj     *appsv1alpha1.ClusterDefinition
		clusterVersionObj *appsv1alpha1.ClusterVersion
		componentDefObj   *appsv1alpha1.ComponentDefinition
		clusterObj        *appsv1alpha1.Cluster
		clusterKey        types.NamespacedName
		allSettings       map[string]interface{}
	)

	resetViperCfg := func() {
		if allSettings != nil {
			Expect(viper.MergeConfigMap(allSettings)).ShouldNot(HaveOccurred())
			allSettings = nil
		}
	}

	resetTestContext := func() {
		clusterDefObj = nil
		clusterVersionObj = nil
		clusterObj = nil
		resetViperCfg()
	}

	// Cleanups
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.VolumeSnapshotSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ServiceSignature, true, inNS)
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.BackupPolicyTemplateSignature, ml)
		testapps.ClearResources(&testCtx, generics.ActionSetSignature, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
		resetTestContext()
	}

	BeforeEach(func() {
		cleanEnv()
		allSettings = viper.AllSettings()
	})

	AfterEach(func() {
		cleanEnv()
	})

	createAllWorkloadTypesClusterDef := func(noCreateAssociateCV ...bool) {
		By("Create a clusterDefinition obj")
		clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(testapps.StatefulMySQLComponent, statefulCompDefName).
			AddComponentDef(testapps.ConsensusMySQLComponent, consensusCompDefName).
			AddComponentDef(testapps.ReplicationRedisComponent, replicationCompDefName).
			AddComponentDef(testapps.StatelessNginxComponent, statelessCompDefName).
			Create(&testCtx).GetObject()

		if len(noCreateAssociateCV) > 0 && noCreateAssociateCV[0] {
			return
		}
		By("Create a clusterVersion obj")
		clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
			AddComponentVersion(statefulCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			AddComponentVersion(consensusCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			AddComponentVersion(replicationCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			AddComponentVersion(statelessCompDefName).AddContainerShort("nginx", testapps.NginxImage).
			Create(&testCtx).GetObject()

		By("Create a ComponentDefinition obj")
		componentDefObj = testapps.NewComponentDefinitionFactory(compDefName).
			WithRandomName().
			SetDefaultSpec().
			Create(&testCtx).
			GetObject()
	}

	waitForCreatingResourceCompletely := func(clusterKey client.ObjectKey, compNames ...string) {
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		cluster := &appsv1alpha1.Cluster{}
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, cluster, true)).Should(Succeed())
		for _, compName := range compNames {
			compPhase := appsv1alpha1.CreatingClusterCompPhase
			for _, spec := range cluster.Spec.ComponentSpecs {
				if spec.Name == compName && spec.Replicas == 0 {
					compPhase = appsv1alpha1.StoppedClusterCompPhase
				}
			}
			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(compPhase))
		}
	}

	type expectService struct {
		clusterIP string
		svcType   corev1.ServiceType
	}

	validateCompSvcList := func(g Gomega, expectServices map[string]expectService, compName string) {
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		svcList := &corev1.ServiceList{}
		g.Expect(k8sClient.List(testCtx.Ctx, svcList, client.MatchingLabels{
			constant.AppInstanceLabelKey: clusterKey.Name,
		}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())

		for svcName, svcSpec := range expectServices {
			idx := slices.IndexFunc(svcList.Items, func(e corev1.Service) bool {
				return e.Name == svcName
			})
			g.Expect(idx >= 0).To(BeTrue())
			svc := svcList.Items[idx]
			g.Expect(svc.Spec.Type).Should(Equal(svcSpec.svcType))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.KBAppComponentLabelKey, compName))
			// g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.RoleLabelKey, "leader"))
			switch {
			case svc.Spec.Type == corev1.ServiceTypeLoadBalancer:
				g.Expect(svc.Spec.ExternalTrafficPolicy).Should(Equal(corev1.ServiceExternalTrafficPolicyTypeLocal))
			case svc.Spec.Type == corev1.ServiceTypeClusterIP && len(svcSpec.clusterIP) == 0:
				g.Expect(svc.Spec.ClusterIP).ShouldNot(Equal(corev1.ClusterIPNone))
			case svc.Spec.Type == corev1.ServiceTypeClusterIP && len(svcSpec.clusterIP) != 0:
				g.Expect(svc.Spec.ClusterIP).Should(Equal(corev1.ClusterIPNone))
				// for _, port := range getHeadlessSvcPorts(g, compDefName) {
				//	g.Expect(slices.Index(svc.Spec.Ports, port) >= 0).Should(BeTrue())
				// }
			}
		}
		g.Expect(len(expectServices)).Should(Equal(len(svcList.Items)))
	}

	testServiceCreateAndDelete := func(compName, compDefName string) {
		expectServices := map[string]expectService{
			testapps.ServiceDefaultName:  {"", corev1.ServiceTypeClusterIP},
			testapps.ServiceHeadlessName: {corev1.ClusterIPNone, corev1.ServiceTypeClusterIP},
			testapps.ServiceVPCName:      {"", corev1.ServiceTypeLoadBalancer},
			testapps.ServiceInternetName: {"", corev1.ServiceTypeLoadBalancer},
		}

		clusterServices := make([]appsv1alpha1.ClusterService, 0)
		for name, svc := range expectServices {
			clusterServices = append(clusterServices, appsv1alpha1.ClusterService{
				Name: name,
				Service: corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{Port: 3306},
						},
						Type:      svc.svcType,
						ClusterIP: svc.clusterIP,
					},
				},
				ComponentSelector: compName,
				// RoleSelector:      []string{"leader"},
			})
		}

		By("creating a cluster with three services")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(compName, compDefName).SetReplicas(1).
			AddClusterService(clusterServices[0]).
			AddClusterService(clusterServices[1]).
			AddClusterService(clusterServices[2]).
			WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		deleteService := clusterServices[2]
		lastClusterService := clusterServices[3]

		By("create last cluster service manually which will not owned by cluster")
		svcObj := builder.NewServiceBuilder(clusterObj.Namespace, lastClusterService.Service.Name).
			AddLabelsInMap(constant.GetClusterWellKnownLabels(clusterObj.Name)).
			SetSpec(&lastClusterService.Service.Spec).
			AddSelector(constant.KBAppComponentLabelKey, lastClusterService.ComponentSelector).
			// AddSelector(constant.RoleLabelKey, lastClusterService.RoleSelector[0]).
			Optimize4ExternalTraffic().
			GetObject()
		Expect(testCtx.CheckedCreateObj(testCtx.Ctx, svcObj)).Should(Succeed())

		By("check all services created")
		Eventually(func(g Gomega) { validateCompSvcList(g, expectServices, compName) }).Should(Succeed())

		By("delete a cluster service")
		delete(expectServices, deleteService.Name)
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			var services []appsv1alpha1.ClusterService
			for _, item := range cluster.Spec.Services {
				if item.Name != deleteService.Name {
					services = append(services, item)
				}
			}
			cluster.Spec.Services = services
		})()).ShouldNot(HaveOccurred())

		By("check the service has been deleted, and the non-managed service has not been deleted")
		Eventually(func(g Gomega) { validateCompSvcList(g, expectServices, compName) }).Should(Succeed())

		By("add the deleted service back")
		expectServices[deleteService.Name] = expectService{deleteService.Service.Spec.ClusterIP, deleteService.Service.Spec.Type}
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			cluster.Spec.Services = append(cluster.Spec.Services, deleteService)
		})()).ShouldNot(HaveOccurred())
		Eventually(func(g Gomega) { validateCompSvcList(g, expectServices, compName) }).Should(Succeed())
	}

	createClusterObjNoWait := func(compName, compDefName string) {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefObj.Name, clusterVersionObj.Name).
			WithRandomName().
			AddComponent(compName, compDefName).
			SetReplicas(1).
			SetEnabledLogs("error").
			AddVolumeClaimTemplate(testapps.DataVolumeName, testapps.NewPVCSpec("5Gi")).
			Create(&testCtx).
			GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)
	}

	createClusterObj := func(compName, compDefName string) {
		createClusterObjNoWait(compName, compDefName)

		By("Waiting for the cluster enter provisioning phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))
	}

	// createClusterObjV2 creates cluster objects with new component definition API enabled.
	createClusterObjV2 := func(compName, compDefName string) {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "", "").
			WithRandomName().
			AddComponent(compName, "").
			SetCompDef(compDefName).
			SetReplicas(1).
			SetEnabledLogs("error").
			AddVolumeClaimTemplate(testapps.DataVolumeName, testapps.NewPVCSpec("5Gi")).
			Create(&testCtx).
			GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter provisioning phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))
	}

	testWipeOut := func(compName, compDefName string) {
		createClusterObj(compName, compDefName)

		By("Waiting for the cluster enter running phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))

		By("Mocking a retained backup")
		backupPolicyName := "test-backup-policy"
		backupName := "test-backup"
		backupMethod := "test-backup-method"
		backup := testdp.NewBackupFactory(testCtx.DefaultNamespace, backupName).
			SetBackupPolicyName(backupPolicyName).
			SetBackupMethod(backupMethod).
			SetLabels(map[string]string{
				constant.AppInstanceLabelKey:      clusterKey.Name,
				constant.BackupProtectionLabelKey: constant.BackupRetain,
			}).
			WithRandomName().
			Create(&testCtx).GetObject()
		backupKey := client.ObjectKeyFromObject(backup)

		// REVIEW: this test flow

		By("Delete the cluster")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})

		By("Wait for the cluster to terminate")
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, false)).Should(Succeed())

		By("Checking backup should exist")
		Eventually(testapps.CheckObjExists(&testCtx, backupKey, &dpv1alpha1.Backup{}, true)).Should(Succeed())
	}

	testDoNotTerminate := func(compName, compDefName string) {
		createClusterObj(compName, compDefName)

		// REVIEW: this test flow

		// REVIEW: why not set termination upon creation?
		By("Update the cluster's termination policy to DoNotTerminate")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			cluster.Spec.TerminationPolicy = appsv1alpha1.DoNotTerminate
		})()).ShouldNot(HaveOccurred())

		By("Delete the cluster")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, true)).Should(Succeed())

		By("Update the cluster's termination policy to WipeOut")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			cluster.Spec.TerminationPolicy = appsv1alpha1.WipeOut
		})()).ShouldNot(HaveOccurred())

		By("Wait for the cluster to terminate")
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, false)).Should(Succeed())
	}

	testHaltNRecovery := func(compName, compDefName string) {
		// TODO(component)
	}

	testDelete := func(compName, compDefName string) {
		// TODO(component)
	}

	// getPVCName := func(vctName, compName string, i int) string {
	//	return fmt.Sprintf("%s-%s-%s-%d", vctName, clusterKey.Name, compName, i)
	//}
	//
	// createPVC := func(clusterName, pvcName, compName, storageSize, storageClassName string) {
	//	if storageSize == "" {
	//		storageSize = "1Gi"
	//	}
	//	clusterBytes, _ := json.Marshal(clusterObj)
	//	testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName,
	//		compName, testapps.DataVolumeName).
	//		AddLabelsInMap(map[string]string{
	//			constant.AppInstanceLabelKey:    clusterName,
	//			constant.KBAppComponentLabelKey: compName,
	//			constant.AppManagedByLabelKey:   constant.AppName,
	//		}).AddAnnotations(constant.LastAppliedClusterAnnotationKey, string(clusterBytes)).
	//		SetStorage(storageSize).
	//		SetStorageClass(storageClassName).
	//		CheckedCreate(&testCtx)
	// }

	Context("provisioning cluster w/o component definition", func() {
		BeforeEach(func() {
			createAllWorkloadTypesClusterDef()
		})

		It("create cluster w/o cluster version", func() {
			By("creating a cluster w/o cluster version")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefObj.Name, "").
				AddComponent(statelessCompName, statelessCompDefName).SetReplicas(3).
				AddComponent(statefulCompName, statefulCompDefName).SetReplicas(3).
				AddComponent(consensusCompName, consensusCompDefName).SetReplicas(3).
				AddComponent(replicationCompName, replicationCompDefName).SetReplicas(3).
				WithRandomName().
				Create(&testCtx).
				GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey, statelessCompName, statefulCompName, consensusCompName, replicationCompName)
		})

		It("create cluster with component object", func() {
			createClusterObj(consensusCompName, consensusCompDefName)

			By("check component created")
			compKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      clusterKey.Name + "-" + consensusCompName,
			}
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *appsv1alpha1.Component) {
				g.Expect(comp.Generation).Should(BeEquivalentTo(1))
				for k, v := range constant.GetComponentWellKnownLabels(clusterObj.Name, consensusCompName) {
					g.Expect(comp.Labels).Should(HaveKeyWithValue(k, v))
				}
				g.Expect(comp.Spec.Cluster).Should(Equal(clusterObj.Name))
				g.Expect(comp.Spec.CompDef).Should(BeEmpty())
			})).Should(Succeed())
		})
	})

	Context("provisioning cluster w/ component definition", func() {
		BeforeEach(func() {
			createAllWorkloadTypesClusterDef()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("cluster component created", func() {
			createClusterObjV2(consensusCompName, componentDefObj.Name)

			By("check component created")
			compKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      clusterKey.Name + "-" + consensusCompName,
			}
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *appsv1alpha1.Component) {
				g.Expect(comp.Generation).Should(BeEquivalentTo(1))
				for k, v := range constant.GetComponentWellKnownLabels(clusterObj.Name, consensusCompName) {
					g.Expect(comp.Labels).Should(HaveKeyWithValue(k, v))
				}
				g.Expect(comp.Spec.Cluster).Should(Equal(clusterObj.Name))
				g.Expect(comp.Spec.CompDef).Should(Equal(componentDefObj.Name))
			})).Should(Succeed())
		})
	})

	Context("cluster deletion", func() {
		BeforeEach(func() {
			createAllWorkloadTypesClusterDef()
		})

		var (
			createObjV1 = func() { createClusterObj(consensusCompName, consensusCompDefName) }
			createObjV2 = func() { createClusterObjV2(consensusCompName, componentDefObj.Name) }
		)
		for _, createObj := range []func(){createObjV1, createObjV2} {
			It("should deleted after all the sub-resources", func() {
				createObj()

				By("check component created")
				compKey := types.NamespacedName{
					Namespace: clusterKey.Namespace,
					Name:      clusterKey.Name + "-" + consensusCompName,
				}
				Eventually(testapps.CheckObjExists(&testCtx, compKey, &appsv1alpha1.Component{}, true)).Should(Succeed())

				By("set finalizer for component to prevent it from deletion")
				finalizer := "test/finalizer"
				Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *appsv1alpha1.Component) {
					comp.Finalizers = append(comp.Finalizers, finalizer)
				})()).ShouldNot(HaveOccurred())

				By("delete the cluster")
				testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})

				By("check cluster keep existing")
				Consistently(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, true)).Should(Succeed())

				By("remove finalizer of component to get it deleted")
				Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *appsv1alpha1.Component) {
					comp.Finalizers = nil
				})()).ShouldNot(HaveOccurred())

				By("wait for the cluster to terminate")
				Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, false)).Should(Succeed())
			})
		}
	})

	Context("create cluster with all workload types", func() {
		compNameNDef := map[string]string{
			statelessCompName:   statelessCompDefName,
			statefulCompName:    statefulCompDefName,
			consensusCompName:   consensusCompDefName,
			replicationCompName: replicationCompDefName,
		}

		BeforeEach(func() {
			createAllWorkloadTypesClusterDef()
		})

		AfterEach(func() {
			cleanEnv()
		})

		for compName, compDefName := range compNameNDef {
			It(fmt.Sprintf("[comp: %s] should not terminate immediately if deleting cluster with terminationPolicy=DoNotTerminate", compName), func() {
				testDoNotTerminate(compName, compDefName)
			})

			It(fmt.Sprintf("[comp: %s] should not terminate immediately if deleting cluster with terminationPolicy=DoNotTerminate", compName), func() {
				testHaltNRecovery(compName, compDefName)
			})

			It(fmt.Sprintf("[comp: %s] should not terminate immediately if deleting cluster with terminationPolicy=DoNotTerminate", compName), func() {
				testDelete(compName, compDefName)
			})

			It(fmt.Sprintf("[comp: %s] should delete cluster resources immediately if deleting cluster with terminationPolicy=WipeOut", compName), func() {
				testWipeOut(compName, compDefName)
			})

			It(fmt.Sprintf("[comp: %s] should create and delete service correctly", compName), func() {
				testServiceCreateAndDelete(compName, compDefName)
			})
		}
	})

	Context("cluster status phase as Failed/Abnormal", func() {
		BeforeEach(func() {
			createAllWorkloadTypesClusterDef()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("test cluster conditions when cluster definition non-exist", func() {
			By("create a cluster with cluster definition non-exist")
			mockClusterDefName := fmt.Sprintf("%s-%s", consensusCompDefName, testCtx.GetRandomStr())
			createClusterObjNoWait(consensusCompName, mockClusterDefName)

			By("check conditions")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Status.ObservedGeneration).Should(BeZero())
				condition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Reason).Should(BeEquivalentTo(ReasonPreCheckFailed))
			})).Should(Succeed())
		})

		It("test cluster conditions when cluster version unavailable", func() {
			By("mock cluster version unavailable")
			mockCompDefName := "random-comp-def"
			clusterVersionKey := client.ObjectKeyFromObject(clusterVersionObj)
			Expect(testapps.GetAndChangeObj(&testCtx, clusterVersionKey, func(clusterVersion *appsv1alpha1.ClusterVersion) {
				for i, comp := range clusterVersion.Spec.ComponentVersions {
					if comp.ComponentDefRef == consensusCompDefName {
						clusterVersion.Spec.ComponentVersions[i].ComponentDefRef = mockCompDefName
						break
					}
				}
			})()).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, clusterVersionKey, func(g Gomega, clusterVersion *appsv1alpha1.ClusterVersion) {
				g.Expect(clusterVersion.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
			})).Should(Succeed())

			By("create a cluster with the unavailable cluster version")
			createClusterObjNoWait(consensusCompName, consensusCompDefName)

			By("expect the cluster provisioning condition as pre-check failed")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Status.ObservedGeneration).Should(BeZero())
				condition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Reason).Should(BeEquivalentTo(ReasonPreCheckFailed))
			})).Should(Succeed())

			By("reset cluster version to Available")
			Expect(testapps.GetAndChangeObj(&testCtx, clusterVersionKey, func(clusterVersion *appsv1alpha1.ClusterVersion) {
				for i, comp := range clusterVersion.Spec.ComponentVersions {
					if comp.ComponentDefRef == mockCompDefName {
						clusterVersion.Spec.ComponentVersions[i].ComponentDefRef = consensusCompDefName
						break
					}
				}
			})()).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, clusterVersionKey, func(g Gomega, clusterVersion *appsv1alpha1.ClusterVersion) {
				g.Expect(clusterVersion.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
			})).Should(Succeed())

			By("expect the cluster phase transit to Creating")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Status.Phase).Should(Equal(appsv1alpha1.CreatingClusterPhase))
			})).Should(Succeed())
		})
	})
})
