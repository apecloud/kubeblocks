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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sethvargo/go-password/password"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Cluster Controller", func() {
	const (
		clusterDefName     = "test-clusterdef"
		clusterVersionName = "test-clusterversion"
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
		clusterNameRand        string
		clusterDefNameRand     string
		clusterVersionNameRand string
		clusterDefObj          *appsv1alpha1.ClusterDefinition
		clusterVersionObj      *appsv1alpha1.ClusterVersion
		clusterObj             *appsv1alpha1.Cluster
		clusterKey             types.NamespacedName
		allSettings            map[string]interface{}
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
		randomStr := testCtx.GetRandomStr()
		clusterNameRand = "mysql-" + randomStr
		clusterDefNameRand = "mysql-definition-" + randomStr
		clusterVersionNameRand = "mysql-cluster-version-" + randomStr
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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.VolumeSnapshotSignature, true, inNS)
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

	// test function helpers
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

	type ExpectService struct {
		headless bool
		svcType  corev1.ServiceType
	}

	// getHeadlessSvcPorts returns the component's headless service ports by gathering all container's ports in the
	// ClusterComponentDefinition.PodSpec, it's a subset of the real ports as some containers can be dynamically
	// injected into the pod by the lifecycle controller, such as the probe container.
	getHeadlessSvcPorts := func(g Gomega, compDefName string) []corev1.ServicePort {
		comp, err := appsv1alpha1.GetComponentDefByCluster(testCtx.Ctx, k8sClient, *clusterObj, compDefName)
		g.Expect(err).ShouldNot(HaveOccurred())
		var headlessSvcPorts []corev1.ServicePort
		for _, container := range comp.PodSpec.Containers {
			for _, port := range container.Ports {
				// be consistent with headless_service_template.cue
				headlessSvcPorts = append(headlessSvcPorts, corev1.ServicePort{
					Name:       port.Name,
					Protocol:   port.Protocol,
					Port:       port.ContainerPort,
					TargetPort: intstr.FromString(port.Name),
				})
			}
		}
		return headlessSvcPorts
	}

	validateCompSvcList := func(g Gomega, compName string, compDefName string, expectServices map[string]ExpectService) {
		if intctrlutil.IsRSMEnabled() {
			return
		}
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		svcList := &corev1.ServiceList{}
		g.Expect(k8sClient.List(testCtx.Ctx, svcList, client.MatchingLabels{
			constant.AppInstanceLabelKey:    clusterKey.Name,
			constant.KBAppComponentLabelKey: compName,
		}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())

		for svcName, svcSpec := range expectServices {
			idx := slices.IndexFunc(svcList.Items, func(e corev1.Service) bool {
				parts := []string{clusterKey.Name, compName}
				if svcName != "" {
					parts = append(parts, svcName)
				}
				return strings.Join(parts, "-") == e.Name
			})
			g.Expect(idx >= 0).To(BeTrue())
			svc := svcList.Items[idx]
			g.Expect(svc.Spec.Type).Should(Equal(svcSpec.svcType))
			switch {
			case svc.Spec.Type == corev1.ServiceTypeLoadBalancer:
				g.Expect(svc.Spec.ExternalTrafficPolicy).Should(Equal(corev1.ServiceExternalTrafficPolicyTypeLocal))
			case svc.Spec.Type == corev1.ServiceTypeClusterIP && !svcSpec.headless:
				g.Expect(svc.Spec.ClusterIP).ShouldNot(Equal(corev1.ClusterIPNone))
			case svc.Spec.Type == corev1.ServiceTypeClusterIP && svcSpec.headless:
				g.Expect(svc.Spec.ClusterIP).Should(Equal(corev1.ClusterIPNone))
				for _, port := range getHeadlessSvcPorts(g, compDefName) {
					g.Expect(slices.Index(svc.Spec.Ports, port) >= 0).Should(BeTrue())
				}
			}
		}
		g.Expect(len(expectServices)).Should(Equal(len(svcList.Items)))
	}

	testServiceAddAndDelete := func(compName, compDefName string) {
		By("Creating a cluster with two LoadBalancer services")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(compName, compDefName).SetReplicas(1).
			AddService(testapps.ServiceVPCName, corev1.ServiceTypeLoadBalancer).
			AddService(testapps.ServiceInternetName, corev1.ServiceTypeLoadBalancer).
			WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		expectServices := map[string]ExpectService{
			testapps.ServiceHeadlessName: {svcType: corev1.ServiceTypeClusterIP, headless: true},
			testapps.ServiceDefaultName:  {svcType: corev1.ServiceTypeClusterIP, headless: false},
			testapps.ServiceVPCName:      {svcType: corev1.ServiceTypeLoadBalancer, headless: false},
			testapps.ServiceInternetName: {svcType: corev1.ServiceTypeLoadBalancer, headless: false},
		}
		Eventually(func(g Gomega) { validateCompSvcList(g, compName, compDefName, expectServices) }).Should(Succeed())

		By("Delete a LoadBalancer service")
		deleteService := testapps.ServiceVPCName
		delete(expectServices, deleteService)
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			for idx, comp := range cluster.Spec.ComponentSpecs {
				if comp.ComponentDefRef != compDefName || comp.Name != compName {
					continue
				}
				var services []appsv1alpha1.ClusterComponentService
				for _, item := range comp.Services {
					if item.Name == deleteService {
						continue
					}
					services = append(services, item)
				}
				cluster.Spec.ComponentSpecs[idx].Services = services
				return
			}
		})()).ShouldNot(HaveOccurred())
		Eventually(func(g Gomega) { validateCompSvcList(g, compName, compDefName, expectServices) }).Should(Succeed())

		By("Add the deleted LoadBalancer service back")
		expectServices[deleteService] = ExpectService{svcType: corev1.ServiceTypeLoadBalancer, headless: false}
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			for idx, comp := range cluster.Spec.ComponentSpecs {
				if comp.ComponentDefRef != compDefName || comp.Name != compName {
					continue
				}
				comp.Services = append(comp.Services, appsv1alpha1.ClusterComponentService{
					Name:        deleteService,
					ServiceType: corev1.ServiceTypeLoadBalancer,
				})
				cluster.Spec.ComponentSpecs[idx] = comp
				return
			}
		})()).ShouldNot(HaveOccurred())
		Eventually(func(g Gomega) { validateCompSvcList(g, compName, compDefName, expectServices) }).Should(Succeed())
	}

	createClusterObj := func(compName, compDefName string) {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(compName, compDefName).
			SetReplicas(1).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster enter running phase")
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

	getPodSpec := func(sts *appsv1.StatefulSet, deploy *appsv1.Deployment) *corev1.PodSpec {
		if sts != nil {
			return &sts.Spec.Template.Spec
		} else if deploy != nil {
			return &deploy.Spec.Template.Spec
		}
		panic("unreachable")
	}

	checkSingleWorkload := func(compDefName string, expects func(g Gomega, sts *appsv1.StatefulSet, deploy *appsv1.Deployment)) {
		Eventually(func(g Gomega) {
			l := testk8s.ListAndCheckRSM(&testCtx, clusterKey)
			sts := components.ConvertRSMToSTS(&l.Items[0])
			expects(g, sts, nil)
		}).Should(Succeed())
	}

	getPVCName := func(vctName, compName string, i int) string {
		return fmt.Sprintf("%s-%s-%s-%d", vctName, clusterKey.Name, compName, i)
	}

	createPVC := func(clusterName, pvcName, compName, storageSize, storageClassName string) {
		if storageSize == "" {
			storageSize = "1Gi"
		}
		clusterBytes, _ := json.Marshal(clusterObj)
		testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName,
			compName, testapps.DataVolumeName).
			AddLabelsInMap(map[string]string{
				constant.AppInstanceLabelKey:    clusterName,
				constant.KBAppComponentLabelKey: compName,
				constant.AppManagedByLabelKey:   constant.AppName,
			}).AddAnnotations(constant.LastAppliedClusterAnnotationKey, string(clusterBytes)).
			SetStorage(storageSize).
			SetStorageClass(storageClassName).
			CheckedCreate(&testCtx)
	}

	checkClusterRBACResourcesExistence := func(cluster *appsv1alpha1.Cluster, serviceAccountName string, volumeProtectionEnabled, expectExisted bool) {
		saObjKey := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      serviceAccountName,
		}
		rbObjKey := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      fmt.Sprintf("kb-%s", cluster.Name),
		}
		Eventually(testapps.CheckObjExists(&testCtx, saObjKey, &corev1.ServiceAccount{}, expectExisted)).Should(Succeed())
		Eventually(testapps.CheckObjExists(&testCtx, rbObjKey, &rbacv1.RoleBinding{}, expectExisted)).Should(Succeed())
		if volumeProtectionEnabled {
			Eventually(testapps.CheckObjExists(&testCtx, rbObjKey, &rbacv1.ClusterRoleBinding{}, expectExisted)).Should(Succeed())
		}
	}

	testClusterRBAC := func(compName, compDefName string, volumeProtectionEnabled bool) {
		Expect(compDefName).Should(BeElementOf(statelessCompDefName, statefulCompDefName, replicationCompDefName, consensusCompDefName))

		By("Creating a cluster with target service account name")
		serviceAccountName := "test-service-account"
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(compName, compDefName).SetReplicas(3).
			SetServiceAccountName(serviceAccountName).
			WithRandomName().
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		By("Checking the podSpec.serviceAccountName")
		checkSingleWorkload(compDefName, func(g Gomega, sts *appsv1.StatefulSet, deploy *appsv1.Deployment) {
			podSpec := getPodSpec(sts, deploy)
			g.Expect(podSpec.ServiceAccountName).To(Equal(serviceAccountName))
		})

		By("check the RBAC resources created exist")
		checkClusterRBACResourcesExistence(clusterObj, serviceAccountName, volumeProtectionEnabled, true)
	}

	testClusterRBACForBackup := func(compName, compDefName string) {
		// set probes and volumeProtections to nil
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj), func(clusterDef *appsv1alpha1.ClusterDefinition) {
			for i := range clusterDef.Spec.ComponentDefs {
				compDef := clusterDef.Spec.ComponentDefs[i]
				if compDef.Name == compDefName {
					compDef.Probes = nil
					compDef.VolumeProtectionSpec = nil
					clusterDef.Spec.ComponentDefs[i] = compDef
					break
				}
			}
		})()).Should(Succeed())
		testClusterRBAC(compName, compDefName, false)
	}

	testReCreateClusterWithRBAC := func(compName, compDefName string) {
		Expect(compDefName).Should(BeElementOf(statelessCompDefName, statefulCompDefName, replicationCompDefName, consensusCompDefName))

		randomStr, _ := password.Generate(6, 0, 0, true, false)
		serviceAccountName := "test-sa-" + randomStr

		By(fmt.Sprintf("Creating a cluster with random service account %s", serviceAccountName))
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(compName, compDefName).SetReplicas(3).
			SetServiceAccountName(serviceAccountName).
			WithRandomName().
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		By("check the RBAC resources created exist")
		checkClusterRBACResourcesExistence(clusterObj, serviceAccountName, true, true)

		By("Delete the cluster")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})

		By("Wait for the cluster to terminate")
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, false)).Should(Succeed())

		By("check the RBAC resources deleted")
		checkClusterRBACResourcesExistence(clusterObj, serviceAccountName, true, false)

		By("re-create cluster with same name")
		clusterObj = testapps.NewClusterFactory(clusterKey.Namespace, clusterKey.Name,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(compName, compDefName).SetReplicas(3).
			SetServiceAccountName(serviceAccountName).
			Create(&testCtx).GetObject()
		waitForCreatingResourceCompletely(clusterKey, compName)

		By("check the RBAC resources re-created exist")
		checkClusterRBACResourcesExistence(clusterObj, serviceAccountName, true, true)

		By("Delete the cluster")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})

		By("Wait for the cluster to terminate")
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, false)).Should(Succeed())
	}

	updateClusterAnnotation := func(cluster *appsv1alpha1.Cluster) {
		Expect(testapps.ChangeObj(&testCtx, cluster, func(lcluster *appsv1alpha1.Cluster) {
			lcluster.Annotations = map[string]string{
				"time": time.Now().Format(time.RFC3339),
			}
		})).ShouldNot(HaveOccurred())
	}

	// TODO: add case: empty image in cd, should report applyResourceFailed condition
	Context("when creating cluster without clusterversion", func() {
		BeforeEach(func() {
			createAllWorkloadTypesClusterDef(true)
		})

		It("should reconcile to create cluster with no error", func() {
			By("Creating a cluster")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, "").
				AddComponent(statelessCompName, statelessCompDefName).SetReplicas(3).
				AddComponent(statefulCompName, statefulCompDefName).SetReplicas(3).
				AddComponent(consensusCompName, consensusCompDefName).SetReplicas(3).
				AddComponent(replicationCompName, replicationCompDefName).SetReplicas(3).
				WithRandomName().Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey, statelessCompName, statefulCompName, consensusCompName, replicationCompName)
		})
	})

	Context("when creating cluster with multiple kinds of components", func() {
		BeforeEach(func() {
			cleanEnv()
			createAllWorkloadTypesClusterDef()
		})

		createNWaitClusterObj := func(components map[string]string,
			addedComponentProcessor func(compName string, factory *testapps.MockClusterFactory),
			withFixedName ...bool) {
			Expect(components).ShouldNot(BeEmpty())

			By("Creating a cluster")
			clusterBuilder := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name)

			compNames := make([]string, 0, len(components))
			for compName, compDefName := range components {
				clusterBuilder = clusterBuilder.AddComponent(compName, compDefName)
				if addedComponentProcessor != nil {
					addedComponentProcessor(compName, clusterBuilder)
				}
				compNames = append(compNames, compName)
			}
			if len(withFixedName) == 0 || !withFixedName[0] {
				clusterBuilder.WithRandomName()
			}
			clusterObj = clusterBuilder.Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey, compNames...)
		}

		checkAllResourcesCreated := func(compNameNDef map[string]string) {
			createNWaitClusterObj(compNameNDef, func(compName string, factory *testapps.MockClusterFactory) {
				factory.SetReplicas(3)
			}, true)

			for compName := range compNameNDef {
				By(fmt.Sprintf("Check %s workload has been created", compName))
				Eventually(testapps.List(&testCtx, generics.RSMSignature,
					client.MatchingLabels{
						constant.AppInstanceLabelKey:    clusterKey.Name,
						constant.KBAppComponentLabelKey: compName,
					}, client.InNamespace(clusterKey.Namespace))).ShouldNot(HaveLen(0))
			}
			rsmList := testk8s.ListAndCheckRSM(&testCtx, clusterKey)

			By("Check stateful pod's volumes")
			for _, sts := range rsmList.Items {
				podSpec := sts.Spec.Template
				volumeNames := map[string]struct{}{}
				for _, v := range podSpec.Spec.Volumes {
					volumeNames[v.Name] = struct{}{}
				}

				for _, cc := range [][]corev1.Container{
					podSpec.Spec.Containers,
					podSpec.Spec.InitContainers,
				} {
					for _, c := range cc {
						for _, vm := range c.VolumeMounts {
							_, ok := volumeNames[vm.Name]
							Expect(ok).Should(BeTrue())
						}
					}
				}
			}

			By("Check associated Secret has been created")
			Eventually(testapps.List(&testCtx, generics.SecretSignature,
				client.MatchingLabels{
					constant.AppInstanceLabelKey: clusterKey.Name,
				})).ShouldNot(BeEmpty())

			By("Check associated CM has been created")
			Eventually(testapps.List(&testCtx, generics.ConfigMapSignature,
				client.MatchingLabels{
					constant.AppInstanceLabelKey: clusterKey.Name,
				})).ShouldNot(BeEmpty())

			By("Check associated PDB has been created")
			Eventually(testapps.List(&testCtx, generics.PodDisruptionBudgetSignature,
				client.MatchingLabels{
					constant.AppInstanceLabelKey: clusterKey.Name,
				}, client.InNamespace(clusterKey.Namespace))).ShouldNot(BeEmpty())

			podSpec := rsmList.Items[0].Spec.Template.Spec
			By("Checking created rsm pods template with built-in toleration")
			Expect(podSpec.Tolerations).Should(HaveLen(1))
			Expect(podSpec.Tolerations[0].Key).To(Equal(testDataPlaneTolerationKey))

			By("Checking created rsm pods template with built-in Affinity")
			Expect(podSpec.Affinity.PodAntiAffinity == nil && podSpec.Affinity.PodAffinity == nil).Should(BeTrue())
			Expect(podSpec.Affinity.NodeAffinity).ShouldNot(BeNil())
			Expect(podSpec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Key).To(
				Equal(testDataPlaneNodeAffinityKey))

			By("Checking created rsm pods template without TopologySpreadConstraints")
			Expect(podSpec.TopologySpreadConstraints).Should(BeEmpty())

			By("Checking stateless services")
			statelessExpectServices := map[string]ExpectService{
				// TODO: fix me later, proxy should not have internal headless service
				testapps.ServiceHeadlessName: {svcType: corev1.ServiceTypeClusterIP, headless: true},
				testapps.ServiceDefaultName:  {svcType: corev1.ServiceTypeClusterIP, headless: false},
			}
			Eventually(func(g Gomega) {
				validateCompSvcList(g, statelessCompName, statelessCompDefName, statelessExpectServices)
			}).Should(Succeed())

			By("Checking stateful types services")
			for compName, compNameNDef := range compNameNDef {
				if compName == statelessCompName {
					continue
				}
				consensusExpectServices := map[string]ExpectService{
					testapps.ServiceHeadlessName: {svcType: corev1.ServiceTypeClusterIP, headless: true},
					testapps.ServiceDefaultName:  {svcType: corev1.ServiceTypeClusterIP, headless: false},
				}
				Eventually(func(g Gomega) {
					validateCompSvcList(g, compName, compNameNDef, consensusExpectServices)
				}).Should(Succeed())
			}
		}

		It("should create all sub-resources successfully, with terminationPolicy=Halt lifecycle", func() {
			compNameNDef := map[string]string{
				statelessCompName:   statelessCompDefName,
				consensusCompName:   consensusCompDefName,
				statefulCompName:    statefulCompDefName,
				replicationCompName: replicationCompDefName,
			}
			checkAllResourcesCreated(compNameNDef)

			By("Mocking components' PVCs to bound")
			var items []client.Object
			rsmList := testk8s.ListAndCheckRSM(&testCtx, clusterKey)
			for i := range rsmList.Items {
				items = append(items, &rsmList.Items[i])
			}
			for _, item := range items {
				compName, ok := item.GetLabels()[constant.KBAppComponentLabelKey]
				Expect(ok).Should(BeTrue())
				replicas := reflect.ValueOf(item).Elem().FieldByName("Spec").FieldByName("Replicas").Elem().Int()
				for i := int(replicas); i >= 0; i-- {
					pvcKey := types.NamespacedName{
						Namespace: clusterKey.Namespace,
						Name:      getPVCName(testapps.DataVolumeName, compName, i),
					}
					createPVC(clusterKey.Name, pvcKey.Name, compName, "", "")
					Eventually(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
					Expect(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
						pvc.Status.Phase = corev1.ClaimBound
					})()).ShouldNot(HaveOccurred())
				}
			}

			By("delete the cluster and should be preserved PVC,Secret,CM resources")
			deleteCluster := func(termPolicy appsv1alpha1.TerminationPolicyType) {
				// TODO: would be better that cluster is created with terminationPolicy=Halt instead of
				// reassign the value after created
				Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
					cluster.Spec.TerminationPolicy = termPolicy
				})()).ShouldNot(HaveOccurred())
				testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})
				Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, false)).Should(Succeed())
			}
			deleteCluster(appsv1alpha1.Halt)

			By("check should preserved PVC,Secret,CM resources")

			checkPreservedObjects := func(uid types.UID) (*corev1.PersistentVolumeClaimList, *corev1.SecretList, *corev1.ConfigMapList) {
				checkObject := func(obj client.Object) {
					clusterJSON, ok := obj.GetAnnotations()[constant.LastAppliedClusterAnnotationKey]
					Expect(ok).Should(BeTrue())
					Expect(clusterJSON).ShouldNot(BeEmpty())
					lastAppliedCluster := &appsv1alpha1.Cluster{}
					Expect(json.Unmarshal([]byte(clusterJSON), lastAppliedCluster)).ShouldNot(HaveOccurred())
					Expect(lastAppliedCluster.UID).Should(BeEquivalentTo(uid))
				}
				listOptions := []client.ListOption{
					client.InNamespace(clusterKey.Namespace),
					client.MatchingLabels{
						constant.AppInstanceLabelKey: clusterKey.Name,
					},
				}
				pvcList := &corev1.PersistentVolumeClaimList{}
				Expect(k8sClient.List(testCtx.Ctx, pvcList, listOptions...)).Should(Succeed())

				cmList := &corev1.ConfigMapList{}
				Expect(k8sClient.List(testCtx.Ctx, cmList, listOptions...)).Should(Succeed())

				secretList := &corev1.SecretList{}
				Expect(k8sClient.List(testCtx.Ctx, secretList, listOptions...)).Should(Succeed())
				if uid != "" {
					By("check pvc resources preserved")
					Expect(pvcList.Items).ShouldNot(BeEmpty())

					for _, pvc := range pvcList.Items {
						checkObject(&pvc)
					}
					By("check secret resources preserved")
					Expect(secretList.Items).ShouldNot(BeEmpty())
					for _, secret := range secretList.Items {
						checkObject(&secret)
					}
				}
				return pvcList, secretList, cmList
			}
			initPVCList, initSecretList, _ := checkPreservedObjects(clusterObj.UID)

			By("create recovering cluster")
			lastClusterUID := clusterObj.UID
			checkAllResourcesCreated(compNameNDef)

			Expect(clusterObj.UID).ShouldNot(Equal(lastClusterUID))
			lastPVCList, lastSecretList, _ := checkPreservedObjects("")

			Expect(outOfOrderEqualFunc(initPVCList.Items, lastPVCList.Items, func(i corev1.PersistentVolumeClaim, j corev1.PersistentVolumeClaim) bool {
				return i.UID == j.UID
			})).Should(BeTrue())
			Expect(outOfOrderEqualFunc(initSecretList.Items, lastSecretList.Items, func(i corev1.Secret, j corev1.Secret) bool {
				return i.UID == j.UID
			})).Should(BeTrue())

			By("delete the cluster and should be preserved PVC,Secret,CM resources but result updated the new last applied cluster UID")
			deleteCluster(appsv1alpha1.Halt)
			checkPreservedObjects(clusterObj.UID)
		})
	})

	When("creating cluster with all workloadTypes (being Stateless|Stateful|Consensus|Replication) component", func() {
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
			It(fmt.Sprintf("[comp: %s] should delete cluster resources immediately if deleting cluster with terminationPolicy=WipeOut", compName), func() {
				testWipeOut(compName, compDefName)
			})

			It(fmt.Sprintf("[comp: %s] should not terminate immediately if deleting cluster with terminationPolicy=DoNotTerminate", compName), func() {
				testDoNotTerminate(compName, compDefName)
			})

			It(fmt.Sprintf("[comp: %s] should add and delete service correctly", compName), func() {
				testServiceAddAndDelete(compName, compDefName)
			})

			It(fmt.Sprintf("[comp: %s] should create RBAC resources correctly", compName), func() {
				testClusterRBAC(compName, compDefName, true)
			})

			It(fmt.Sprintf("[comp: %s] should create RBAC resources correctly if only supports backup", compName), func() {
				testClusterRBACForBackup(compName, compDefName)
			})

			It(fmt.Sprintf("[comp: %s] should re-create cluster and RBAC resources correctly", compName), func() {
				testReCreateClusterWithRBAC(compName, compDefName)
			})
		}
	})

	Context("test cluster Failed/Abnormal phase", func() {
		It("test cluster conditions", func() {
			By("init cluster")
			cluster := testapps.CreateConsensusMysqlCluster(&testCtx, clusterDefNameRand,
				clusterVersionNameRand, clusterNameRand, consensusCompDefName, consensusCompName,
				"2Gi")
			clusterKey := client.ObjectKeyFromObject(cluster)

			By("test when clusterDefinition not found")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				g.Expect(tmpCluster.Status.ObservedGeneration).Should(BeZero())
				condition := meta.FindStatusCondition(tmpCluster.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Reason).Should(BeEquivalentTo(ReasonPreCheckFailed))
			})).Should(Succeed())

			// TODO: removed conditionsError phase need to review correct-ness of following commented off block:
			// By("test conditionsError phase")
			// Expect(testapps.GetAndChangeObjStatus(&testCtx, clusterKey, func(tmpCluster *appsv1alpha1.Cluster) {
			// 	condition := meta.FindStatusCondition(tmpCluster.Status.Conditions, ConditionTypeProvisioningStarted)
			// 	condition.LastTransitionTime = metav1.Time{Time: time.Now().Add(-(time.Millisecond*time.Duration(viper.GetInt(constant.CfgKeyCtrlrReconcileRetryDurationMS)) + time.Second))}
			// 	meta.SetStatusCondition(&tmpCluster.Status.Conditions, *condition)
			// })()).ShouldNot(HaveOccurred())

			// Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
			// 	g.Expect(tmpCluster.Status.Phase == appsv1alpha1.ConditionsErrorPhase).Should(BeTrue())
			// })).Should(Succeed())

			By("test when clusterVersion not Available")
			clusterVersion := testapps.CreateConsensusMysqlClusterVersion(&testCtx, clusterDefNameRand, clusterVersionNameRand, consensusCompDefName)
			clusterVersionKey := client.ObjectKeyFromObject(clusterVersion)
			// mock clusterVersion unavailable
			Expect(testapps.GetAndChangeObj(&testCtx, clusterVersionKey, func(clusterVersion *appsv1alpha1.ClusterVersion) {
				clusterVersion.Spec.ComponentVersions[0].ComponentDefRef = "test-n"
			})()).ShouldNot(HaveOccurred())
			_ = testapps.CreateConsensusMysqlClusterDef(&testCtx, clusterDefNameRand, consensusCompDefName)

			Eventually(testapps.CheckObj(&testCtx, clusterVersionKey, func(g Gomega, clusterVersion *appsv1alpha1.ClusterVersion) {
				g.Expect(clusterVersion.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
			})).Should(Succeed())

			// trigger reconcile
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Spec.ComponentSpecs[0].EnabledLogs = []string{"error1"}
			})()).ShouldNot(HaveOccurred())

			Eventually(func(g Gomega) {
				updateClusterAnnotation(cluster)
				g.Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
					g.Expect(cluster.Status.ObservedGeneration).Should(BeZero())
					condition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
					g.Expect(condition).ShouldNot(BeNil())
					g.Expect(condition.Reason).Should(BeEquivalentTo(ReasonPreCheckFailed))
				})).Should(Succeed())
			}).Should(Succeed())

			By("reset clusterVersion to Available")
			Expect(testapps.GetAndChangeObj(&testCtx, clusterVersionKey, func(clusterVersion *appsv1alpha1.ClusterVersion) {
				clusterVersion.Spec.ComponentVersions[0].ComponentDefRef = "consensus"
			})()).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, clusterVersionKey, func(g Gomega, clusterVersion *appsv1alpha1.ClusterVersion) {
				g.Expect(clusterVersion.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
			})).Should(Succeed())

			// trigger reconcile
			updateClusterAnnotation(cluster)
			By("test preCheckFailed")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Status.ObservedGeneration).Should(BeZero())
				condition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Reason).Should(Equal(ReasonPreCheckFailed))
			})).Should(Succeed())

			By("reset and waiting cluster to Creating")
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Spec.ComponentSpecs[0].EnabledLogs = []string{"error"}
			})()).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				g.Expect(tmpCluster.Status.Phase).Should(Equal(appsv1alpha1.CreatingClusterPhase))
				g.Expect(tmpCluster.Status.ObservedGeneration).Should(BeNumerically(">", 1))
			})).Should(Succeed())

			By("mock pvc of component to create")
			for i := 0; i < testapps.ConsensusReplicas; i++ {
				pvcName := fmt.Sprintf("%s-%s-%s-%d", testapps.DataVolumeName, clusterKey.Name, consensusCompName, i)
				pvc := testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterKey.Name,
					consensusCompName, "data").SetStorage("2Gi").Create(&testCtx).GetObject()
				// mock pvc bound
				Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(pvc), func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Phase = corev1.ClaimBound
					pvc.Status.Capacity = corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("2Gi"),
					}
				})()).ShouldNot(HaveOccurred())
			}

			By("apply smaller PVC size will should failed")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Spec.ComponentSpecs[0].VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("1Gi")
			})()).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster),
				func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
					// REVIEW/TODO: (wangyelei) following expects causing inconsistent behavior
					condition := meta.FindStatusCondition(tmpCluster.Status.Conditions, appsv1alpha1.ConditionTypeApplyResources)
					g.Expect(condition).ShouldNot(BeNil())
					g.Expect(condition.Reason).Should(Equal(ReasonApplyResourcesFailed))
				})).Should(Succeed())
		})
	})

	Context("cluster deletion", func() {
		BeforeEach(func() {
			createAllWorkloadTypesClusterDef()
		})
		It("should deleted after all the sub-resources", func() {
			createClusterObj(consensusCompName, consensusCompDefName)

			By("Waiting for the cluster enter running phase")
			Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))

			workloadKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      clusterKey.Name + "-" + consensusCompName,
			}

			By("checking workload exists")
			Eventually(testapps.CheckObjExists(&testCtx, workloadKey, &workloads.ReplicatedStateMachine{}, true)).Should(Succeed())

			finalizerName := "test/finalizer"
			By("set finalizer for workload to prevent it from deletion")
			Expect(testapps.GetAndChangeObj(&testCtx, workloadKey, func(wl *workloads.ReplicatedStateMachine) {
				wl.ObjectMeta.Finalizers = append(wl.ObjectMeta.Finalizers, finalizerName)
			})()).ShouldNot(HaveOccurred())

			By("Delete the cluster")
			testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})

			By("checking cluster keep existing")
			Consistently(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, true)).Should(Succeed())

			By("remove finalizer of sts to get it deleted")
			Expect(testapps.GetAndChangeObj(&testCtx, workloadKey, func(wl *workloads.ReplicatedStateMachine) {
				wl.ObjectMeta.Finalizers = nil
			})()).ShouldNot(HaveOccurred())

			By("Wait for the cluster to terminate")
			Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, false)).Should(Succeed())
		})
	})
})

func outOfOrderEqualFunc[E1, E2 any](s1 []E1, s2 []E2, eq func(E1, E2) bool) bool {
	if l := len(s1); l != len(s2) {
		return false
	}

	for _, v1 := range s1 {
		isEq := false
		for _, v2 := range s2 {
			if isEq = eq(v1, v2); isEq {
				break
			}
		}
		if !isEq {
			return false
		}
	}
	return true
}
