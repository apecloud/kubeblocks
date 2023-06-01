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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replication"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	probeutil "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

const backupPolicyTPLName = "test-backup-policy-template-mysql"

var _ = Describe("Cluster Controller", func() {
	const (
		clusterDefName     = "test-clusterdef"
		clusterVersionName = "test-clusterversion"
		clusterName        = "test-cluster" // this become cluster prefix name if used with testapps.NewClusterFactory().WithRandomName()
		leader             = "leader"
		follower           = "follower"
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
		testapps.ClearClusterResources(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, generics.BackupSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.BackupPolicySignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.VolumeSnapshotSignature, inNS)
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.BackupPolicyTemplateSignature, ml)
		testapps.ClearResources(&testCtx, generics.BackupToolSignature, ml)
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
		for _, compName := range compNames {
			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(appsv1alpha1.CreatingClusterCompPhase))
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
		comp, err := util.GetComponentDefByCluster(testCtx.Ctx, k8sClient, *clusterObj, compDefName)
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

		// REVIEW: this test flow

		By("Delete the cluster")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})

		By("Wait for the cluster to terminate")
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, false)).Should(Succeed())
	}

	testDoNotTermintate := func(compName, compDefName string) {
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

	changeCompReplicas := func(clusterName types.NamespacedName, replicas int32, comp *appsv1alpha1.ClusterComponentSpec) {
		Expect(testapps.GetAndChangeObj(&testCtx, clusterName, func(cluster *appsv1alpha1.Cluster) {
			for i, clusterComp := range cluster.Spec.ComponentSpecs {
				if clusterComp.Name == comp.Name {
					cluster.Spec.ComponentSpecs[i].Replicas = replicas
				}
			}
		})()).ShouldNot(HaveOccurred())
	}

	changeComponentReplicas := func(clusterName types.NamespacedName, replicas int32) {
		Expect(testapps.GetAndChangeObj(&testCtx, clusterName, func(cluster *appsv1alpha1.Cluster) {
			Expect(cluster.Spec.ComponentSpecs).Should(HaveLen(1))
			cluster.Spec.ComponentSpecs[0].Replicas = replicas
		})()).ShouldNot(HaveOccurred())
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
		isStsWorkload := true
		switch compDefName {
		case statelessCompDefName:
			isStsWorkload = false
		case statefulCompDefName, replicationCompDefName, consensusCompDefName:
			break
		default:
			panic("unreachable")
		}

		if isStsWorkload {
			Eventually(func(g Gomega) {
				l := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
				expects(g, &l.Items[0], nil)
			}).Should(Succeed())
		} else {
			Eventually(func(g Gomega) {
				l := testk8s.ListAndCheckDeployment(&testCtx, clusterKey)
				expects(g, nil, &l.Items[0])
			}).Should(Succeed())
		}
	}

	testChangeReplicas := func(compName, compDefName string) {
		Expect(compDefName).Should(BeElementOf(statelessCompDefName, statefulCompDefName, replicationCompDefName, consensusCompDefName))
		createClusterObj(compName, compDefName)
		replicasSeq := []int32{5, 3, 1, 0, 2, 4}
		expectedOG := int64(1)
		for _, replicas := range replicasSeq {
			By(fmt.Sprintf("Change replicas to %d", replicas))
			changeComponentReplicas(clusterKey, replicas)
			expectedOG++
			By("Checking cluster status and the number of replicas changed")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
				g.Expect(fetched.Status.ObservedGeneration).To(BeEquivalentTo(expectedOG))
				g.Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))
			})).Should(Succeed())

			checkSingleWorkload(compDefName, func(g Gomega, sts *appsv1.StatefulSet, deploy *appsv1.Deployment) {
				if sts != nil {
					g.Expect(int(*sts.Spec.Replicas)).To(BeEquivalentTo(replicas))
				} else {
					g.Expect(int(*deploy.Spec.Replicas)).To(BeEquivalentTo(replicas))
				}
			})
		}
	}

	getPVCName := func(compName string, i int) string {
		return fmt.Sprintf("%s-%s-%s-%d", testapps.DataVolumeName, clusterKey.Name, compName, i)
	}

	createPVC := func(clusterName, pvcName, compName string) {
		testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName,
			compName, "data").SetStorage("1Gi").CheckedCreate(&testCtx)
	}

	mockPodsForTest := func(cluster *appsv1alpha1.Cluster, number int) []corev1.Pod {
		componentName := cluster.Spec.ComponentSpecs[0].Name
		clusterName := cluster.Name
		stsName := cluster.Name + "-" + componentName
		pods := make([]corev1.Pod, 0)
		for i := 0; i < number; i++ {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      stsName + "-" + strconv.Itoa(i),
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						constant.AppManagedByLabelKey:         constant.AppName,
						constant.AppInstanceLabelKey:          clusterName,
						constant.KBAppComponentLabelKey:       componentName,
						appsv1.ControllerRevisionHashLabelKey: "mock-version",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "mock-container",
						Image: "mock-container",
					}},
				},
			}
			pods = append(pods, *pod)
		}
		return pods
	}

	horizontalScaleComp := func(updatedReplicas int, comp *appsv1alpha1.ClusterComponentSpec, policy *appsv1alpha1.HorizontalScalePolicy) {
		By("Mocking components' PVCs to bound")
		for i := 0; i < int(comp.Replicas); i++ {
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(comp.Name, i),
			}
			createPVC(clusterKey.Name, pvcKey.Name, comp.Name)
			Eventually(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
			Expect(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Status.Phase = corev1.ClaimBound
			})()).ShouldNot(HaveOccurred())
		}

		By("Checking sts replicas right")
		stsList := testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, clusterKey, comp.Name)
		Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(comp.Replicas))

		By("Creating mock pods in StatefulSet")
		pods := mockPodsForTest(clusterObj, int(comp.Replicas))
		for i, pod := range pods {
			if comp.ComponentDefRef == replicationCompDefName && i == 0 {
				By("mocking primary for replication to pass check")
				pods[0].ObjectMeta.Labels[constant.RoleLabelKey] = "primary"
			}
			Expect(testCtx.CheckedCreateObj(testCtx.Ctx, &pod)).Should(Succeed())
			// mock the status to pass the isReady(pod) check in consensus_set
			pod.Status.Conditions = []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}}
			Expect(k8sClient.Status().Update(ctx, &pod)).Should(Succeed())
		}

		By(fmt.Sprintf("Changing replicas to %d", updatedReplicas))
		changeCompReplicas(clusterKey, int32(updatedReplicas), comp)

		checkUpdatedStsReplicas := func() {
			By("Checking updated sts replicas")
			Eventually(func() int32 {
				stsList = testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, clusterKey, comp.Name)
				return *stsList.Items[0].Spec.Replicas
			}).Should(BeEquivalentTo(updatedReplicas))
		}

		if policy == nil {
			checkUpdatedStsReplicas()
			return
		}

		By("Checking Backup created")
		Eventually(testapps.List(&testCtx, generics.BackupSignature,
			client.MatchingLabels{
				constant.AppInstanceLabelKey:    clusterKey.Name,
				constant.KBAppComponentLabelKey: comp.Name,
			}, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(1))

		backupKey := types.NamespacedName{Name: fmt.Sprintf("%s-%s-scaling",
			clusterKey.Name, comp.Name),
			Namespace: testCtx.DefaultNamespace}
		By("mocking backup status to completed")
		Expect(testapps.GetAndChangeObjStatus(&testCtx, backupKey, func(backup *dataprotectionv1alpha1.Backup) {
			backup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
			backup.Status.PersistentVolumeClaimName = "backup-data"
		})()).Should(Succeed())

		if policy.Type == appsv1alpha1.HScaleDataClonePolicyFromSnapshot {
			By("Mocking VolumeSnapshot and set it as ReadyToUse")
			pvcName := getPVCName(comp.Name, 0)
			volumeSnapshot := &snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
					Labels: map[string]string{
						constant.KBManagedByKey:         "cluster",
						constant.AppInstanceLabelKey:    clusterKey.Name,
						constant.KBAppComponentLabelKey: comp.Name,
					}},
				Spec: snapshotv1.VolumeSnapshotSpec{
					Source: snapshotv1.VolumeSnapshotSource{
						PersistentVolumeClaimName: &pvcName,
					},
				},
			}
			scheme, _ := appsv1alpha1.SchemeBuilder.Build()
			Expect(controllerruntime.SetControllerReference(clusterObj, volumeSnapshot, scheme)).Should(Succeed())
			Expect(testCtx.CreateObj(testCtx.Ctx, volumeSnapshot)).Should(Succeed())
			readyToUse := true
			volumeSnapshotStatus := snapshotv1.VolumeSnapshotStatus{ReadyToUse: &readyToUse}
			volumeSnapshot.Status = &volumeSnapshotStatus
			Expect(k8sClient.Status().Update(testCtx.Ctx, volumeSnapshot)).Should(Succeed())
		}

		if policy.Type == appsv1alpha1.HScaleDataClonePolicyFromBackup {
			By("Checking pvc created")
			Eventually(testapps.List(&testCtx, generics.PersistentVolumeClaimSignature,
				client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: comp.Name,
				}, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(updatedReplicas))
			By("Checking restore job created")
			Eventually(testapps.List(&testCtx, generics.JobSignature,
				client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: comp.Name,
					constant.KBManagedByKey:         "cluster",
				}, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(updatedReplicas - int(comp.Replicas)))
		}

		By("Mock PVCs status to bound")
		for i := 0; i < updatedReplicas; i++ {
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(comp.Name, i),
			}
			Eventually(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
			Expect(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Status.Phase = corev1.ClaimBound
			})()).ShouldNot(HaveOccurred())
		}

		By("Check backup job cleanup")
		Eventually(testapps.List(&testCtx, generics.BackupSignature,
			client.MatchingLabels{
				constant.AppInstanceLabelKey:    clusterKey.Name,
				constant.KBAppComponentLabelKey: comp.Name,
			}, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(0))
		Eventually(testapps.CheckObjExists(&testCtx, backupKey, &snapshotv1.VolumeSnapshot{}, false)).Should(Succeed())

		checkUpdatedStsReplicas()

		By("Checking updated sts replicas' PVC")
		for i := 0; i < updatedReplicas; i++ {
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(comp.Name, i),
			}
			Eventually(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
		}

		By("Checking pod env config updated")
		cmKey := types.NamespacedName{
			Namespace: clusterKey.Namespace,
			Name:      fmt.Sprintf("%s-%s-env", clusterKey.Name, comp.Name),
		}
		Eventually(testapps.CheckObj(&testCtx, cmKey, func(g Gomega, cm *corev1.ConfigMap) {
			match := func(key, prefix, suffix string) bool {
				return strings.HasPrefix(key, prefix) && strings.HasSuffix(key, suffix)
			}
			foundN := ""
			for k, v := range cm.Data {
				if match(k, constant.KBPrefix, "_N") {
					foundN = v
					break
				}
			}
			g.Expect(foundN).Should(Equal(strconv.Itoa(updatedReplicas)))
			for i := 0; i < updatedReplicas; i++ {
				foundPodHostname := ""
				suffix := fmt.Sprintf("_%d_HOSTNAME", i)
				for k, v := range cm.Data {
					if match(k, constant.KBPrefix, suffix) {
						foundPodHostname = v
						break
					}
				}
				g.Expect(foundPodHostname != "").Should(BeTrue())
			}
		})).Should(Succeed())
	}

	// @argument componentDefsWithHScalePolicy assign ClusterDefinition.spec.componentDefs[].horizontalScalePolicy for
	// the matching names. If not provided, will set 1st ClusterDefinition.spec.componentDefs[0].horizontalScalePolicy.
	horizontalScale := func(updatedReplicas int, policyType appsv1alpha1.HScaleDataClonePolicyType, componentDefsWithHScalePolicy ...string) {
		viper.Set("VOLUMESNAPSHOT", true)
		cluster := &appsv1alpha1.Cluster{}
		Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(Succeed())
		initialGeneration := int(cluster.Status.ObservedGeneration)

		By("Set HorizontalScalePolicy")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
			func(clusterDef *appsv1alpha1.ClusterDefinition) {
				// assign 1st component
				if len(componentDefsWithHScalePolicy) == 0 && len(clusterDef.Spec.ComponentDefs) > 0 {
					componentDefsWithHScalePolicy = []string{
						clusterDef.Spec.ComponentDefs[0].Name,
					}
				}
				for i, compDef := range clusterDef.Spec.ComponentDefs {
					if !slices.Contains(componentDefsWithHScalePolicy, compDef.Name) {
						continue
					}

					By("Checking backup policy created from backup policy template")
					policyName := lifecycle.DeriveBackupPolicyName(clusterKey.Name, compDef.Name, "")
					clusterDef.Spec.ComponentDefs[i].HorizontalScalePolicy =
						&appsv1alpha1.HorizontalScalePolicy{Type: policyType,
							BackupPolicyTemplateName: backupPolicyTPLName}

					Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKey{Name: policyName, Namespace: clusterKey.Namespace},
						&dataprotectionv1alpha1.BackupPolicy{}, true)).Should(Succeed())

					if policyType == appsv1alpha1.HScaleDataClonePolicyFromBackup {
						By("creating backup tool if backup policy is backup")
						backupTool := &dataprotectionv1alpha1.BackupTool{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-backup-tool",
								Namespace: clusterKey.Namespace,
								Labels: map[string]string{
									constant.ClusterDefLabelKey: clusterDef.Name,
								},
							},
							Spec: dataprotectionv1alpha1.BackupToolSpec{
								BackupCommands: []string{""},
								Image:          "xtrabackup",
								Env: []corev1.EnvVar{
									{
										Name:  "test-name",
										Value: "test-value",
									},
								},
								Physical: dataprotectionv1alpha1.BackupToolRestoreCommand{
									RestoreCommands: []string{
										"echo \"hello world\"",
									},
								},
							},
						}
						testapps.CheckedCreateK8sResource(&testCtx, backupTool)
					}
				}
			})()).ShouldNot(HaveOccurred())

		By("Mocking all components' PVCs to bound")
		for _, comp := range clusterObj.Spec.ComponentSpecs {
			for i := 0; i < int(comp.Replicas); i++ {
				pvcKey := types.NamespacedName{
					Namespace: clusterKey.Namespace,
					Name:      getPVCName(comp.Name, i),
				}
				createPVC(clusterKey.Name, pvcKey.Name, comp.Name)
				Eventually(testapps.CheckObjExists(&testCtx, pvcKey,
					&corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Phase = corev1.ClaimBound
				})).Should(Succeed())
			}
		}

		hscalePolicy := func(comp appsv1alpha1.ClusterComponentSpec) *appsv1alpha1.HorizontalScalePolicy {
			for _, componentDef := range clusterDefObj.Spec.ComponentDefs {
				if componentDef.Name == comp.ComponentDefRef {
					return componentDef.HorizontalScalePolicy
				}
			}
			return nil
		}

		By("Get the latest cluster def")
		Expect(k8sClient.Get(testCtx.Ctx, client.ObjectKeyFromObject(clusterDefObj), clusterDefObj)).Should(Succeed())
		for i, comp := range clusterObj.Spec.ComponentSpecs {
			horizontalScaleComp(updatedReplicas, &clusterObj.Spec.ComponentSpecs[i], hscalePolicy(comp))
		}

		By("Checking cluster status and the number of replicas changed")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).
			Should(BeEquivalentTo(initialGeneration + len(clusterObj.Spec.ComponentSpecs)))
	}

	testHorizontalScale := func(compName, compDefName string) {
		initialReplicas := int32(1)
		updatedReplicas := int32(3)

		By("Creating a single component cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(compName, compDefName).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			SetReplicas(initialReplicas).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		// REVIEW: this test flow, wait for running phase?
		horizontalScale(int(updatedReplicas), appsv1alpha1.HScaleDataClonePolicyFromSnapshot, compDefName)
	}

	testStorageExpansion := func(compName, compDefName string) {
		var (
			storageClassName  = "sc-mock"
			replicas          = 3
			volumeSize        = "1Gi"
			newVolumeSize     = "2Gi"
			volumeQuantity    = resource.MustParse(volumeSize)
			newVolumeQuantity = resource.MustParse(newVolumeSize)
		)

		By("Mock a StorageClass which allows resize")
		allowVolumeExpansion := true
		storageClass := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: storageClassName,
			},
			Provisioner:          "kubernetes.io/no-provisioner",
			AllowVolumeExpansion: &allowVolumeExpansion,
		}
		Expect(testCtx.CreateObj(testCtx.Ctx, storageClass)).Should(Succeed())

		By("Creating a cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec(volumeSize)
		pvcSpec.StorageClassName = &storageClass.Name

		By("Create cluster and waiting for the cluster initialized")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(compName, compDefName).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			SetReplicas(int32(replicas)).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Checking the replicas")
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		sts := &stsList.Items[0]
		Expect(*sts.Spec.Replicas).Should(BeEquivalentTo(replicas))

		By("Mock PVCs in Bound Status")
		for i := 0; i < replicas; i++ {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getPVCName(compName, i),
					Namespace: clusterKey.Namespace,
					Labels: map[string]string{
						constant.AppManagedByLabelKey:   constant.AppName,
						constant.AppInstanceLabelKey:    clusterKey.Name,
						constant.KBAppComponentLabelKey: compName,
					}},
				Spec: pvcSpec.ToV1PersistentVolumeClaimSpec(),
			}
			Expect(testCtx.CreateObj(testCtx.Ctx, pvc)).Should(Succeed())
			pvc.Status.Phase = corev1.ClaimBound // only bound pvc allows resize
			if pvc.Status.Capacity == nil {
				pvc.Status.Capacity = corev1.ResourceList{}
			}
			pvc.Status.Capacity[corev1.ResourceStorage] = volumeQuantity
			Expect(k8sClient.Status().Update(testCtx.Ctx, pvc)).Should(Succeed())
		}

		By("mock pods/sts of component are available")
		switch compDefName {
		case statelessCompDefName:
			// ignore
		case replicationCompDefName:
			testapps.MockReplicationComponentPods(nil, testCtx, sts, clusterObj.Name, compDefName, nil)
		case statefulCompDefName, consensusCompDefName:
			testapps.MockConsensusComponentPods(&testCtx, sts, clusterObj.Name, compName)
		}
		Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
			testk8s.MockStatefulSetReady(sts)
		})).ShouldNot(HaveOccurred())
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))

		By("Updating the PVC storage size")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			comp := &cluster.Spec.ComponentSpecs[0]
			comp.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = newVolumeQuantity
		})()).ShouldNot(HaveOccurred())

		By("Checking the resize operation in progress")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(appsv1alpha1.SpecReconcilingClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.SpecReconcilingClusterPhase))
		for i := 0; i < replicas; i++ {
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(compName, i),
			}
			Expect(k8sClient.Get(testCtx.Ctx, pvcKey, pvc)).Should(Succeed())
			Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(newVolumeQuantity))
			Expect(pvc.Status.Capacity[corev1.ResourceStorage]).To(Equal(volumeQuantity))
		}

		By("Mock resizing of PVCs finished")
		for i := 0; i < replicas; i++ {
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(compName, i),
			}
			Expect(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Status.Capacity[corev1.ResourceStorage] = newVolumeQuantity
			})()).ShouldNot(HaveOccurred())
		}

		By("Checking the resize operation finished")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))

		By("Checking PVCs are resized")
		for i := 0; i < replicas; i++ {
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(compName, i),
			}
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.Status.Capacity[corev1.ResourceStorage]).To(Equal(newVolumeQuantity))
			})).Should(Succeed())
		}
	}

	testVolumeExpansionFailedAndRecover := func(compName, compDefName string) {

		const storageClassName = "test-sc"
		const replicas = 3

		By("Mock a StorageClass which allows resize")
		allowVolumeExpansion := true
		storageClass := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: storageClassName,
			},
			Provisioner:          "kubernetes.io/no-provisioner",
			AllowVolumeExpansion: &allowVolumeExpansion,
		}
		Expect(testCtx.CreateObj(testCtx.Ctx, storageClass)).Should(Succeed())

		By("Creating a cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		pvcSpec.StorageClassName = &storageClass.Name

		By("Create cluster and waiting for the cluster initialized")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(compName, compDefName).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			SetReplicas(replicas).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Checking the replicas")
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		sts := &stsList.Items[0]
		Expect(*sts.Spec.Replicas).Should(BeEquivalentTo(replicas))

		By("Mock PVCs in Bound Status")
		for i := 0; i < replicas; i++ {
			tmpSpec := pvcSpec.ToV1PersistentVolumeClaimSpec()
			tmpSpec.VolumeName = getPVCName(compName, i)
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getPVCName(compName, i),
					Namespace: clusterKey.Namespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: clusterKey.Name,
					}},
				Spec: tmpSpec,
			}
			Expect(testCtx.CreateObj(testCtx.Ctx, pvc)).Should(Succeed())
			pvc.Status.Phase = corev1.ClaimBound // only bound pvc allows resize
			Expect(k8sClient.Status().Update(testCtx.Ctx, pvc)).Should(Succeed())
		}

		By("mocking PVs")
		for i := 0; i < replicas; i++ {
			pv := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getPVCName(compName, i), // use same name as pvc
					Namespace: clusterKey.Namespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: clusterKey.Name,
					}},
				Spec: corev1.PersistentVolumeSpec{
					Capacity: corev1.ResourceList{
						"storage": resource.MustParse("1Gi"),
					},
					AccessModes: []corev1.PersistentVolumeAccessMode{
						"ReadWriteOnce",
					},
					PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
					StorageClassName:              storageClassName,
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/opt/volume/nginx",
							Type: nil,
						},
					},
					ClaimRef: &corev1.ObjectReference{
						Name: getPVCName(compName, i),
					},
				},
			}
			Expect(testCtx.CreateObj(testCtx.Ctx, pv)).Should(Succeed())
		}

		By("Updating the PVC storage size")
		newStorageValue := resource.MustParse("2Gi")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			comp := &cluster.Spec.ComponentSpecs[0]
			comp.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue
		})()).ShouldNot(HaveOccurred())

		By("Checking the resize operation finished")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))

		By("Checking PVCs are resized")
		stsList = testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		sts = &stsList.Items[0]
		for i := *sts.Spec.Replicas - 1; i >= 0; i-- {
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(compName, int(i)),
			}
			Expect(k8sClient.Get(testCtx.Ctx, pvcKey, pvc)).Should(Succeed())
			Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(newStorageValue))
		}

		By("Updating the PVC storage size back")
		originStorageValue := resource.MustParse("1Gi")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			comp := &cluster.Spec.ComponentSpecs[0]
			comp.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = originStorageValue
		})()).ShouldNot(HaveOccurred())

		By("Checking the resize operation finished")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(3))

		By("Checking PVCs are resized")
		Eventually(func(g Gomega) {
			stsList = testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
			sts = &stsList.Items[0]
			for i := *sts.Spec.Replicas - 1; i >= 0; i-- {
				pvc := &corev1.PersistentVolumeClaim{}
				pvcKey := types.NamespacedName{
					Namespace: clusterKey.Namespace,
					Name:      getPVCName(compName, int(i)),
				}
				g.Expect(k8sClient.Get(testCtx.Ctx, pvcKey, pvc)).Should(Succeed())
				g.Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(originStorageValue))
			}
		}).Should(Succeed())
	}

	testClusterAffinity := func(compName, compDefName string) {
		const topologyKey = "testTopologyKey"
		const labelKey = "testNodeLabelKey"
		const labelValue = "testLabelValue"

		By("Creating a cluster with Affinity")
		Expect(compDefName).Should(BeElementOf(statelessCompDefName, statefulCompDefName, replicationCompDefName, consensusCompDefName))

		affinity := &appsv1alpha1.Affinity{
			PodAntiAffinity: appsv1alpha1.Required,
			TopologyKeys:    []string{topologyKey},
			NodeLabels: map[string]string{
				labelKey: labelValue,
			},
			Tenancy: appsv1alpha1.SharedNode,
		}

		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(compName, compDefName).SetReplicas(3).
			WithRandomName().SetClusterAffinity(affinity).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		By("Checking the Affinity and TopologySpreadConstraints")
		checkSingleWorkload(compDefName, func(g Gomega, sts *appsv1.StatefulSet, deploy *appsv1.Deployment) {
			podSpec := getPodSpec(sts, deploy)
			g.Expect(podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).To(Equal(labelKey))
			g.Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable).To(Equal(corev1.DoNotSchedule))
			g.Expect(podSpec.TopologySpreadConstraints[0].TopologyKey).To(Equal(topologyKey))
			g.Expect(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution).Should(HaveLen(1))
			g.Expect(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey).To(Equal(topologyKey))
		})
	}

	testClusterServiceAccount := func(compName, compDefName string) {
		By("Creating a cluster with target service account name")
		Expect(compDefName).Should(BeElementOf(statelessCompDefName, statefulCompDefName, replicationCompDefName, consensusCompDefName))

		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(compName, compDefName).SetReplicas(3).
			SetServiceAccountName("test-service-account").
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		By("Checking the podSpec.serviceAccountName")
		checkSingleWorkload(compDefName, func(g Gomega, sts *appsv1.StatefulSet, deploy *appsv1.Deployment) {
			podSpec := getPodSpec(sts, deploy)
			g.Expect(podSpec.ServiceAccountName).To(Equal("test-service-account"))
		})
	}

	testComponentAffinity := func(compName, compDefName string) {
		const clusterTopologyKey = "testClusterTopologyKey"
		const compTopologyKey = "testComponentTopologyKey"

		By("Creating a cluster with Affinity")
		Expect(compDefName).Should(BeElementOf(statelessCompDefName, statefulCompDefName, replicationCompDefName, consensusCompDefName))
		affinity := &appsv1alpha1.Affinity{
			PodAntiAffinity: appsv1alpha1.Required,
			TopologyKeys:    []string{clusterTopologyKey},
			Tenancy:         appsv1alpha1.SharedNode,
		}
		compAffinity := &appsv1alpha1.Affinity{
			PodAntiAffinity: appsv1alpha1.Preferred,
			TopologyKeys:    []string{compTopologyKey},
			Tenancy:         appsv1alpha1.DedicatedNode,
		}
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().SetClusterAffinity(affinity).
			AddComponent(compName, compDefName).SetComponentAffinity(compAffinity).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		By("Checking the Affinity and the TopologySpreadConstraints")
		checkSingleWorkload(compDefName, func(g Gomega, sts *appsv1.StatefulSet, deploy *appsv1.Deployment) {
			podSpec := getPodSpec(sts, deploy)
			g.Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable).To(Equal(corev1.ScheduleAnyway))
			g.Expect(podSpec.TopologySpreadConstraints[0].TopologyKey).To(Equal(compTopologyKey))
			g.Expect(podSpec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight).ShouldNot(BeNil())
			g.Expect(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution).Should(HaveLen(1))
			g.Expect(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey).To(Equal(corev1.LabelHostname))
		})
	}

	testClusterToleration := func(compName, compDefName string) {
		const tolerationKey = "testClusterTolerationKey"
		const tolerationValue = "testClusterTolerationValue"
		By("Creating a cluster with Toleration")
		Expect(compDefName).Should(BeElementOf(statelessCompDefName, statefulCompDefName, replicationCompDefName, consensusCompDefName))
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(compName, compDefName).SetReplicas(1).
			AddClusterToleration(corev1.Toleration{
				Key:      tolerationKey,
				Value:    tolerationValue,
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			}).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		By("Checking the tolerations")
		checkSingleWorkload(compDefName, func(g Gomega, sts *appsv1.StatefulSet, deploy *appsv1.Deployment) {
			podSpec := getPodSpec(sts, deploy)
			g.Expect(podSpec.Tolerations).Should(HaveLen(2))
			t := podSpec.Tolerations[0]
			g.Expect(t.Key).Should(BeEquivalentTo(tolerationKey))
			g.Expect(t.Value).Should(BeEquivalentTo(tolerationValue))
			g.Expect(t.Operator).Should(BeEquivalentTo(corev1.TolerationOpEqual))
			g.Expect(t.Effect).Should(BeEquivalentTo(corev1.TaintEffectNoSchedule))
		})
	}

	testStsWorkloadComponentToleration := func(compName, compDefName string) {
		clusterTolerationKey := "testClusterTolerationKey"
		compTolerationKey := "testcompTolerationKey"
		compTolerationValue := "testcompTolerationValue"

		By("Creating a cluster with Toleration")
		Expect(compDefName).Should(BeElementOf(statelessCompDefName, statefulCompDefName, replicationCompDefName, consensusCompDefName))
		compToleration := corev1.Toleration{
			Key:      compTolerationKey,
			Value:    compTolerationValue,
			Operator: corev1.TolerationOpEqual,
			Effect:   corev1.TaintEffectNoSchedule,
		}
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddClusterToleration(corev1.Toleration{
				Key:      clusterTolerationKey,
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			}).
			AddComponent(compName, compDefName).AddComponentToleration(compToleration).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		By("Checking the tolerations")
		checkSingleWorkload(compDefName, func(g Gomega, sts *appsv1.StatefulSet, deploy *appsv1.Deployment) {
			podSpec := getPodSpec(sts, deploy)
			Expect(podSpec.Tolerations).Should(HaveLen(2))
			t := podSpec.Tolerations[0]
			g.Expect(t.Key).Should(BeEquivalentTo(compTolerationKey))
			g.Expect(t.Value).Should(BeEquivalentTo(compTolerationValue))
			g.Expect(t.Operator).Should(BeEquivalentTo(corev1.TolerationOpEqual))
			g.Expect(t.Effect).Should(BeEquivalentTo(corev1.TaintEffectNoSchedule))
		})
	}

	mockRoleChangedEvent := func(key types.NamespacedName, sts *appsv1.StatefulSet) []corev1.Event {
		pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
		Expect(err).To(Succeed())

		events := make([]corev1.Event, 0)
		for _, pod := range pods {
			event := corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pod.Name + "-event",
					Namespace: testCtx.DefaultNamespace,
				},
				Reason:  string(probeutil.CheckRoleOperation),
				Message: `{"event":"Success","originalRole":"Leader","role":"Follower"}`,
				InvolvedObject: corev1.ObjectReference{
					Name:      pod.Name,
					Namespace: testCtx.DefaultNamespace,
					UID:       pod.UID,
					FieldPath: constant.ProbeCheckRolePath,
				},
			}
			events = append(events, event)
		}
		events[0].Message = `{"event":"Success","originalRole":"Leader","role":"Leader"}`
		return events
	}

	getStsPodsName := func(sts *appsv1.StatefulSet) []string {
		pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
		Expect(err).To(Succeed())

		names := make([]string, 0)
		for _, pod := range pods {
			names = append(names, pod.Name)
		}
		return names
	}

	testThreeReplicas := func(compName, compDefName string) {
		const replicas = 3

		By("Mock a cluster obj")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(compName, compDefName).
			SetReplicas(replicas).AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		var stsList *appsv1.StatefulSetList
		var sts *appsv1.StatefulSet
		Eventually(func(g Gomega) {
			stsList = testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
			g.Expect(stsList.Items).ShouldNot(BeEmpty())
			sts = &stsList.Items[0]
		}).Should(Succeed())

		By("Creating mock pods in StatefulSet, and set controller reference")
		pods := mockPodsForTest(clusterObj, replicas)
		for _, pod := range pods {
			Expect(controllerutil.SetControllerReference(sts, &pod, scheme.Scheme)).Should(Succeed())
			Expect(testCtx.CreateObj(testCtx.Ctx, &pod)).Should(Succeed())
			// mock the status to pass the isReady(pod) check in consensus_set
			pod.Status.Conditions = []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}}
			Expect(k8sClient.Status().Update(ctx, &pod)).Should(Succeed())
		}

		By("Creating mock role changed events")
		// pod.Labels[intctrlutil.RoleLabelKey] will be filled with the role
		events := mockRoleChangedEvent(clusterKey, sts)
		for _, event := range events {
			Expect(testCtx.CreateObj(ctx, &event)).Should(Succeed())
		}

		By("Checking pods' role are changed accordingly")
		Eventually(func(g Gomega) {
			pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
			g.Expect(err).ShouldNot(HaveOccurred())
			// should have 3 pods
			g.Expect(pods).Should(HaveLen(3))
			// 1 leader
			// 2 followers
			leaderCount, followerCount := 0, 0
			for _, pod := range pods {
				switch pod.Labels[constant.RoleLabelKey] {
				case leader:
					leaderCount++
				case follower:
					followerCount++
				}
			}
			g.Expect(leaderCount).Should(Equal(1))
			g.Expect(followerCount).Should(Equal(2))
		}).Should(Succeed())

		By("Checking pods' annotations")
		Eventually(func(g Gomega) {
			pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(pods).Should(HaveLen(int(*sts.Spec.Replicas)))
			for _, pod := range pods {
				g.Expect(pod.Annotations).ShouldNot(BeNil())
				g.Expect(pod.Annotations[constant.ComponentReplicasAnnotationKey]).Should(Equal(strconv.Itoa(int(*sts.Spec.Replicas))))
			}
		}).Should(Succeed())
		stsPatch := client.MergeFrom(sts.DeepCopy())
		By("Updating StatefulSet's status")
		sts.Status.UpdateRevision = "mock-version"
		sts.Status.Replicas = int32(replicas)
		sts.Status.AvailableReplicas = int32(replicas)
		sts.Status.CurrentReplicas = int32(replicas)
		sts.Status.ReadyReplicas = int32(replicas)
		sts.Status.ObservedGeneration = sts.Generation
		Expect(k8sClient.Status().Patch(ctx, sts, stsPatch)).Should(Succeed())

		By("Checking consensus set pods' role are updated in cluster status")
		Eventually(func(g Gomega) {
			fetched := &appsv1alpha1.Cluster{}
			g.Expect(k8sClient.Get(ctx, clusterKey, fetched)).To(Succeed())
			compName := fetched.Spec.ComponentSpecs[0].Name
			g.Expect(fetched.Status.Components != nil).To(BeTrue())
			g.Expect(fetched.Status.Components).To(HaveKey(compName))
			compStatus, ok := fetched.Status.Components[compName]
			g.Expect(ok).Should(BeTrue())
			consensusStatus := compStatus.ConsensusSetStatus
			g.Expect(consensusStatus != nil).To(BeTrue())
			g.Expect(consensusStatus.Leader.Pod).To(BeElementOf(getStsPodsName(sts)))
			g.Expect(consensusStatus.Followers).Should(HaveLen(2))
			g.Expect(consensusStatus.Followers[0].Pod).To(BeElementOf(getStsPodsName(sts)))
			g.Expect(consensusStatus.Followers[1].Pod).To(BeElementOf(getStsPodsName(sts)))
		}).Should(Succeed())

		By("Waiting the component be running")
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).
			Should(Equal(appsv1alpha1.RunningClusterCompPhase))
	}

	testBackupError := func(compName, compDefName string) {
		initialReplicas := int32(1)
		updatedReplicas := int32(3)
		viper.Set("VOLUMESNAPSHOT", true)

		By("Set HorizontalScalePolicy")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
			func(clusterDef *appsv1alpha1.ClusterDefinition) {
				for i, def := range clusterDef.Spec.ComponentDefs {
					if def.Name != compDefName {
						continue
					}
					clusterDef.Spec.ComponentDefs[i].HorizontalScalePolicy =
						&appsv1alpha1.HorizontalScalePolicy{Type: appsv1alpha1.HScaleDataClonePolicyFromSnapshot,
							BackupPolicyTemplateName: backupPolicyTPLName}
				}
			})()).ShouldNot(HaveOccurred())

		By("Creating a cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(compName, compDefName).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			SetReplicas(initialReplicas).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By(fmt.Sprintf("Changing replicas to %d", updatedReplicas))
		changeCompReplicas(clusterKey, updatedReplicas, &clusterObj.Spec.ComponentSpecs[0])
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))

		// REVIEW: this test flow, should wait/fake still Running phase?
		By("Creating backup")
		backupKey := types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      "test-backup",
		}
		backup := dataprotectionv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backupKey.Name,
				Namespace: backupKey.Namespace,
				Labels: map[string]string{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: compName,
					constant.KBManagedByKey:         "cluster",
				},
			},
			Spec: dataprotectionv1alpha1.BackupSpec{
				BackupPolicyName: lifecycle.DeriveBackupPolicyName(clusterKey.Name, compDefName, ""),
				BackupType:       "snapshot",
			},
		}
		Expect(testCtx.Create(ctx, &backup)).Should(Succeed())

		By("Checking backup status to failed, because pvc not exist")
		Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, backup *dataprotectionv1alpha1.Backup) {
			g.Expect(backup.Status.Phase).Should(Equal(dataprotectionv1alpha1.BackupFailed))
		})).Should(Succeed())

		By("Set HorizontalScalePolicy")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
			func(clusterDef *appsv1alpha1.ClusterDefinition) {
				clusterDef.Spec.ComponentDefs[0].HorizontalScalePolicy =
					&appsv1alpha1.HorizontalScalePolicy{Type: appsv1alpha1.HScaleDataClonePolicyFromSnapshot,
						BackupPolicyTemplateName: backupPolicyTPLName}
			})()).ShouldNot(HaveOccurred())

		By(fmt.Sprintf("Changing replicas to %d", updatedReplicas))
		changeCompReplicas(clusterKey, updatedReplicas, &clusterObj.Spec.ComponentSpecs[0])
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))

		By("Checking cluster status failed with backup error")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
			g.Expect(viper.GetBool("VOLUMESNAPSHOT")).Should(BeTrue())
			g.Expect(cluster.Status.Conditions).ShouldNot(BeEmpty())
			var err error
			for _, cond := range cluster.Status.Conditions {
				if strings.Contains(cond.Message, "backup for horizontalScaling failed") {
					g.Expect(cond.Message).Should(ContainSubstring("backup for horizontalScaling failed"))
					err = errors.New("has backup error")
					break
				}
			}
			if err == nil {
				// this expect is intended for print all cluster.Status.Conditions
				g.Expect(cluster.Status.Conditions).Should(BeEmpty())
			}
			g.Expect(err).Should(HaveOccurred())
		})).Should(Succeed())

		By("expect for backup error event")
		Eventually(func(g Gomega) {
			eventList := corev1.EventList{}
			Expect(k8sClient.List(ctx, &eventList, client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
			hasBackupErrorEvent := false
			for _, v := range eventList.Items {
				if v.Reason == string(intctrlutil.ErrorTypeBackupFailed) {
					hasBackupErrorEvent = true
					break
				}
			}
			g.Expect(hasBackupErrorEvent).Should(BeTrue())
		}).Should(Succeed())
	}

	updateClusterAnnotation := func(cluster *appsv1alpha1.Cluster) {
		Expect(testapps.ChangeObj(&testCtx, cluster, func(lcluster *appsv1alpha1.Cluster) {
			lcluster.Annotations = map[string]string{
				"time": time.Now().Format(time.RFC3339),
			}
		})).ShouldNot(HaveOccurred())
	}

	// Test cases
	// Scenarios
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
			createAllWorkloadTypesClusterDef()
			createBackupPolicyTpl(clusterDefObj)
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

			By("Check deployment workload has been created")
			Eventually(testapps.List(&testCtx, generics.DeploymentSignature,
				client.MatchingLabels{
					constant.AppInstanceLabelKey: clusterKey.Name,
				}, client.InNamespace(clusterKey.Namespace))).ShouldNot(HaveLen(0))

			stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)

			By("Check statefulset pod's volumes")
			for _, sts := range stsList.Items {
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

			By("Check associated PDB has been created")
			Eventually(testapps.List(&testCtx, generics.PodDisruptionBudgetSignature,
				client.MatchingLabels{
					constant.AppInstanceLabelKey: clusterKey.Name,
				}, client.InNamespace(clusterKey.Namespace))).ShouldNot(BeEmpty())

			podSpec := stsList.Items[0].Spec.Template.Spec
			By("Checking created sts pods template with built-in toleration")
			Expect(podSpec.Tolerations).Should(HaveLen(1))
			Expect(podSpec.Tolerations[0].Key).To(Equal(testDataPlaneTolerationKey))

			By("Checking created sts pods template with built-in Affinity")
			Expect(podSpec.Affinity.PodAntiAffinity == nil && podSpec.Affinity.PodAffinity == nil).Should(BeTrue())
			Expect(podSpec.Affinity.NodeAffinity).ShouldNot(BeNil())
			Expect(podSpec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Key).To(
				Equal(testDataPlaneNodeAffinityKey))

			By("Checking created sts pods template without TopologySpreadConstraints")
			Expect(podSpec.TopologySpreadConstraints).Should(BeEmpty())

			By("Check should create env configmap")
			Eventually(func(g Gomega) {
				cmList := &corev1.ConfigMapList{}
				Expect(k8sClient.List(testCtx.Ctx, cmList, client.MatchingLabels{
					constant.AppInstanceLabelKey:   clusterKey.Name,
					constant.AppConfigTypeLabelKey: "kubeblocks-env",
				}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
				Expect(cmList.Items).ShouldNot(BeEmpty())
				Expect(cmList.Items).Should(HaveLen(len(compNameNDef)))
			}).Should(Succeed())

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

		testMultiCompHScale := func(policyType appsv1alpha1.HScaleDataClonePolicyType) {
			compNameNDef := map[string]string{
				statefulCompName:    statefulCompDefName,
				consensusCompName:   consensusCompDefName,
				replicationCompName: replicationCompDefName,
			}
			initialReplicas := int32(1)
			updatedReplicas := int32(3)

			By("Creating a multi components cluster with VolumeClaimTemplate")
			pvcSpec := testapps.NewPVCSpec("1Gi")

			createNWaitClusterObj(compNameNDef, func(compName string, factory *testapps.MockClusterFactory) {
				factory.AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).SetReplicas(initialReplicas)
			})

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey, statefulCompName, consensusCompName, replicationCompName)

			// statefulCompDefName not in componentDefsWithHScalePolicy, for nil backup policy test
			// REVIEW:
			//  1. this test flow, wait for running phase?
			horizontalScale(int(updatedReplicas), policyType, consensusCompDefName, replicationCompDefName)
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
			stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
			for _, sts := range stsList.Items {
				compName, ok := sts.Labels[constant.KBAppComponentLabelKey]
				Expect(ok).Should(BeTrue())
				for i := int(*sts.Spec.Replicas); i >= 0; i-- {
					pvcKey := types.NamespacedName{
						Namespace: clusterKey.Namespace,
						Name:      getPVCName(compName, i),
					}
					createPVC(clusterKey.Name, pvcKey.Name, compName)
					Eventually(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
					Expect(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
						pvc.Status.Phase = corev1.ClaimBound
					})()).ShouldNot(HaveOccurred())
				}
			}

			By("delete the cluster and should preserved PVC,Secret,CM resources")
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
					Expect(cmList.Items).ShouldNot(BeEmpty())
					for _, secret := range secretList.Items {
						checkObject(&secret)
					}
					By("check configmap resources preserved")
					Expect(secretList.Items).ShouldNot(BeEmpty())
					for _, cm := range cmList.Items {
						checkObject(&cm)
					}
				}
				return pvcList, secretList, cmList
			}
			initPVCList, initSecretList, initCMList := checkPreservedObjects(clusterObj.UID)

			By("create recovering cluster")
			lastClusterUID := clusterObj.UID
			checkAllResourcesCreated(compNameNDef)
			Expect(clusterObj.UID).ShouldNot(Equal(lastClusterUID))
			lastPVCList, lastSecretList, lastCMList := checkPreservedObjects("")

			Expect(outOfOrderEqualFunc(initPVCList.Items, lastPVCList.Items, func(i corev1.PersistentVolumeClaim, j corev1.PersistentVolumeClaim) bool {
				return i.UID == j.UID
			})).Should(BeTrue())
			Expect(outOfOrderEqualFunc(initSecretList.Items, lastSecretList.Items, func(i corev1.Secret, j corev1.Secret) bool {
				return i.UID == j.UID
			})).Should(BeTrue())
			Expect(outOfOrderEqualFunc(initCMList.Items, lastCMList.Items, func(i corev1.ConfigMap, j corev1.ConfigMap) bool {
				return i.UID == j.UID
			})).Should(BeTrue())

			By("delete the cluster and should preserved PVC,Secret,CM resources but result updated the new last applied cluster UID")
			deleteCluster(appsv1alpha1.Halt)
			checkPreservedObjects(clusterObj.UID)
		})

		It("should successfully h-scale with multiple components", func() {
			testMultiCompHScale(appsv1alpha1.HScaleDataClonePolicyFromSnapshot)
		})

		It("should successfully h-scale with multiple components by backup tool", func() {
			testMultiCompHScale(appsv1alpha1.HScaleDataClonePolicyFromBackup)
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

		for compName, compDefName := range compNameNDef {
			It(fmt.Sprintf("[comp: %s] should delete cluster resources immediately if deleting cluster with terminationPolicy=WipeOut", compName), func() {
				testWipeOut(compName, compDefName)
			})

			It(fmt.Sprintf("[comp: %s] should not terminate immediately if deleting cluster with terminationPolicy=DoNotTerminate", compName), func() {
				testDoNotTermintate(compName, compDefName)
			})

			It(fmt.Sprintf("[comp: %s] should add and delete service correctly", compName), func() {
				testServiceAddAndDelete(compName, compDefName)
			})

			It(fmt.Sprintf("[comp: %s] should create/delete pods to match the desired replica number if updating cluster's replica number to a valid value", compName), func() {
				testChangeReplicas(compName, compDefName)
			})

			It(fmt.Sprintf("[comp: %s] should add serviceAccountName correctly", compName), func() {
				testClusterServiceAccount(compName, compDefName)
			})

			Context(fmt.Sprintf("[comp: %s] and with cluster affinity set", compName), func() {
				It("should create pod with cluster affinity", func() {
					testClusterAffinity(compName, compDefName)
				})
			})

			Context(fmt.Sprintf("[comp: %s] and with both cluster affinity and component affinity set", compName), func() {
				It("Should observe the component affinity will override the cluster affinity", func() {
					testComponentAffinity(compName, compDefName)
				})
			})

			Context(fmt.Sprintf("[comp: %s] and with cluster tolerations set", compName), func() {
				It("Should create pods with cluster tolerations", func() {
					testClusterToleration(compName, compDefName)
				})
			})

			Context(fmt.Sprintf("[comp: %s] and with both cluster tolerations and component tolerations set", compName), func() {
				It("Should observe the component tolerations will override the cluster tolerations", func() {
					testStsWorkloadComponentToleration(compName, compDefName)
				})
			})
		}
	})

	When("creating cluster with stateful workloadTypes (being Stateful|Consensus|Replication) component", func() {
		compNameNDef := map[string]string{
			statefulCompName:    statefulCompDefName,
			consensusCompName:   consensusCompDefName,
			replicationCompName: replicationCompDefName,
		}

		BeforeEach(func() {
			createAllWorkloadTypesClusterDef()
			createBackupPolicyTpl(clusterDefObj)
		})

		for compName, compDefName := range compNameNDef {
			Context(fmt.Sprintf("[comp: %s] with pvc", compName), func() {
				It("should trigger a backup process(snapshot) and create pvcs from backup for newly created replicas when horizontal scale the cluster from 1 to 3", func() {
					testHorizontalScale(compName, compDefName)
				})
			})

			Context(fmt.Sprintf("[comp: %s] with pvc and dynamic-provisioning storage class", compName), func() {
				It("should update PVC request storage size accordingly", func() {
					testStorageExpansion(compName, compDefName)
				})
			})

			It(fmt.Sprintf("[comp: %s] should be able to recover if volume expansion fails", compName), func() {
				testVolumeExpansionFailedAndRecover(compName, compDefName)
			})

			It(fmt.Sprintf("[comp: %s] should report error if backup error during horizontal scale", compName), func() {
				testBackupError(compName, compDefName)
			})

			Context(fmt.Sprintf("[comp: %s] with horizontal scale after storage expansion", compName), func() {
				It("should succeed with horizontal scale to 5 replicas", func() {
					testStorageExpansion(compName, compDefName)
					horizontalScale(5, appsv1alpha1.HScaleDataClonePolicyFromSnapshot, compDefName)
				})
			})
		}
	})

	When("creating cluster with workloadType=consensus component", func() {
		const (
			compName    = consensusCompName
			compDefName = consensusCompDefName
		)

		BeforeEach(func() {
			createAllWorkloadTypesClusterDef()
			createBackupPolicyTpl(clusterDefObj)
		})

		It("Should success with one leader pod and two follower pods", func() {
			testThreeReplicas(compName, compDefName)
		})

		It("test restore cluster from backup", func() {
			By("mock backuptool object")
			backupPolicyName := "test-backup-policy"
			backupName := "test-backup"
			backupTool := testapps.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
				&dataprotectionv1alpha1.BackupTool{}, testapps.RandomizedObjName())

			By("creating backup")
			backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				SetBackupPolicyName(backupPolicyName).
				SetBackupType(dataprotectionv1alpha1.BackupTypeDataFile).
				Create(&testCtx).GetObject()

			By("waiting for backup failed, because no backup policy exists")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup),
				func(g Gomega, tmpBackup *dataprotectionv1alpha1.Backup) {
					g.Expect(tmpBackup.Status.Phase).Should(Equal(dataprotectionv1alpha1.BackupFailed))
				})).Should(Succeed())

			By("mocking backup status completed, we don't need backup reconcile here")
			Eventually(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(backup), func(backup *dataprotectionv1alpha1.Backup) {
				backup.Status.BackupToolName = backupTool.Name
				backup.Status.PersistentVolumeClaimName = "backup-pvc"
				backup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
			})).Should(Succeed())

			By("checking backup status completed")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup),
				func(g Gomega, tmpBackup *dataprotectionv1alpha1.Backup) {
					g.Expect(tmpBackup.Status.Phase).Should(Equal(dataprotectionv1alpha1.BackupCompleted))
				})).Should(Succeed())

			By("creating cluster with backup")
			restoreFromBackup := fmt.Sprintf(`{"%s":"%s"}`, compName, backupName)
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(compName, compDefName).
				SetReplicas(3).
				AddAnnotations(constant.RestoreFromBackUpAnnotationKey, restoreFromBackup).Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey, compName)
			stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
			sts := stsList.Items[0]
			Expect(sts.Spec.Template.Spec.InitContainers).Should(HaveLen(1))

			By("mock pod/sts are available and wait for component enter running phase")
			testapps.MockConsensusComponentPods(&testCtx, &sts, clusterObj.Name, compName)
			Expect(testapps.ChangeObjStatus(&testCtx, &sts, func() {
				testk8s.MockStatefulSetReady(&sts)
			})).ShouldNot(HaveOccurred())
			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))

			By("the restore container has been removed from init containers")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(&sts), func(g Gomega, tmpSts *appsv1.StatefulSet) {
				g.Expect(tmpSts.Spec.Template.Spec.InitContainers).Should(BeEmpty())
			})).Should(Succeed())

			By("clean up annotations after cluster running")
			Expect(testapps.GetAndChangeObjStatus(&testCtx, clusterKey, func(tmpCluster *appsv1alpha1.Cluster) {
				compStatus := tmpCluster.Status.Components[compName]
				compStatus.Phase = appsv1alpha1.RunningClusterCompPhase
				tmpCluster.Status.Components[compName] = compStatus
			})()).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				g.Expect(tmpCluster.Status.Phase).Should(Equal(appsv1alpha1.RunningClusterPhase))
				g.Expect(tmpCluster.Annotations[constant.RestoreFromBackUpAnnotationKey]).Should(BeEmpty())
			})).Should(Succeed())
		})
	})

	When("creating cluster with workloadType=replication component", func() {
		const (
			compName    = replicationCompName
			compDefName = replicationCompDefName
		)
		BeforeEach(func() {
			createAllWorkloadTypesClusterDef()
			createBackupPolicyTpl(clusterDefObj)
		})

		// REVIEW/TODO: following test always failed at cluster.phase.observerGeneration=1
		//     with cluster.phase.phase=creating
		It("Should success with primary pod and secondary pod", func() {
			By("Mock a cluster obj with replication componentDefRef.")
			pvcSpec := testapps.NewPVCSpec("1Gi")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(compName, compDefName).
				SetPrimaryIndex(testapps.DefaultReplicationPrimaryIndex).
				SetReplicas(testapps.DefaultReplicationReplicas).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey, compDefName)

			By("Checking statefulSet number")
			stsList := testk8s.ListAndCheckStatefulSetItemsCount(&testCtx, clusterKey, 1)
			sts := &stsList.Items[0]

			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				testk8s.MockStatefulSetReady(sts)
			})).ShouldNot(HaveOccurred())
			for i := int32(0); i < *sts.Spec.Replicas; i++ {
				podName := fmt.Sprintf("%s-%d", sts.Name, i)
				testapps.MockReplicationComponentPod(nil, testCtx, sts, clusterObj.Name,
					compDefName, podName, replication.DefaultRole(i))
			}
			Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))
		})
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
				g.Expect(condition.Reason).Should(BeEquivalentTo(lifecycle.ReasonPreCheckFailed))
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
			_ = testapps.CreateConsensusMysqlClusterDef(&testCtx, clusterDefNameRand, consensusCompDefName)
			clusterVersion := testapps.CreateConsensusMysqlClusterVersion(&testCtx, clusterDefNameRand, clusterVersionNameRand, consensusCompDefName)
			clusterVersionKey := client.ObjectKeyFromObject(clusterVersion)
			// mock clusterVersion unavailable
			Expect(testapps.GetAndChangeObj(&testCtx, clusterVersionKey, func(clusterVersion *appsv1alpha1.ClusterVersion) {
				clusterVersion.Spec.ComponentVersions[0].ComponentDefRef = "test-n"
			})()).ShouldNot(HaveOccurred())

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
					g.Expect(condition.Reason).Should(BeEquivalentTo(lifecycle.ReasonPreCheckFailed))
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
				g.Expect(condition.Reason).Should(Equal(lifecycle.ReasonPreCheckFailed))
			})).Should(Succeed())

			By("reset and waiting cluster to Creating")
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Spec.ComponentSpecs[0].EnabledLogs = []string{"error"}
			})()).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				g.Expect(tmpCluster.Status.Phase).Should(Equal(appsv1alpha1.CreatingClusterPhase))
				g.Expect(tmpCluster.Status.ObservedGeneration).ShouldNot(BeZero())
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
					g.Expect(condition.Reason).Should(Equal(lifecycle.ReasonApplyResourcesFailed))
				})).Should(Succeed())
		})
	})
})

func createBackupPolicyTpl(clusterDefObj *appsv1alpha1.ClusterDefinition) {
	By("Creating a BackupPolicyTemplate")
	bpt := testapps.NewBackupPolicyTemplateFactory(backupPolicyTPLName).
		AddLabels(clusterDefLabelKey, clusterDefObj.Name).
		SetClusterDefRef(clusterDefObj.Name)
	for _, v := range clusterDefObj.Spec.ComponentDefs {
		bpt = bpt.AddBackupPolicy(v.Name).AddSnapshotPolicy()
		switch v.WorkloadType {
		case appsv1alpha1.Consensus:
			bpt.SetTargetRole("leader")
		case appsv1alpha1.Replication:
			bpt.SetTargetRole("primary")
		}
	}
	bpt.Create(&testCtx)
}

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
