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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/sethvargo/go-password/password"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/internal/common"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/internal/testutil/dataprotection"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
	lorry "github.com/apecloud/kubeblocks/lorry/client"
)

const (
	backupPolicyTPLName = "test-backup-policy-template-mysql"
	backupMethodName    = "test-backup-method"
	vsBackupMethodName  = "test-vs-backup-method"
	actionSetName       = "test-action-set"
	vsActionSetName     = "test-vs-action-set"
)

var (
	podAnnotationKey4Test = fmt.Sprintf("%s-test", constant.ComponentReplicasAnnotationKey)
)

type mockLorryClient struct {
	clusterKey types.NamespacedName
	compName   string
	replicas   int
}

var _ lorry.Client = &mockLorryClient{}

func (c *mockLorryClient) JoinMember(ctx context.Context) error {
	return nil
}

func (c *mockLorryClient) LeaveMember(ctx context.Context) error {
	var podList corev1.PodList
	labels := client.MatchingLabels{
		constant.AppInstanceLabelKey:    c.clusterKey.Name,
		constant.KBAppComponentLabelKey: c.compName,
	}
	if err := testCtx.Cli.List(ctx, &podList, labels, client.InNamespace(c.clusterKey.Namespace)); err != nil {
		return err
	}
	for _, pod := range podList.Items {
		if pod.Annotations == nil {
			panic(fmt.Sprintf("pod annotaions is nil: %s", pod.Name))
		}
		if pod.Annotations[podAnnotationKey4Test] == fmt.Sprintf("%d", c.replicas) {
			continue
		}
		pod.Annotations[podAnnotationKey4Test] = fmt.Sprintf("%d", c.replicas)
		if err := testCtx.Cli.Update(ctx, &pod); err != nil {
			return err
		}
	}
	return nil
}

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
		actionSetName          = "test-actionset"
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
			SetLabels(map[string]string{constant.AppInstanceLabelKey: clusterKey.Name, constant.BackupProtectionLabelKey: constant.BackupRetain}).
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
		Eventually(func(g Gomega) {
			l := testk8s.ListAndCheckRSM(&testCtx, clusterKey)
			sts := components.ConvertRSMToSTS(&l.Items[0])
			expects(g, sts, nil)
		}).Should(Succeed())
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
				g.Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(BeElementOf(appsv1alpha1.CreatingClusterPhase, appsv1alpha1.UpdatingClusterPhase))
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

	mockComponentPVCsAndBound := func(comp *appsv1alpha1.ClusterComponentSpec, replicas int, create bool, storageClassName string) {
		for i := 0; i < replicas; i++ {
			for _, vct := range comp.VolumeClaimTemplates {
				pvcKey := types.NamespacedName{
					Namespace: clusterKey.Namespace,
					Name:      getPVCName(vct.Name, comp.Name, i),
				}
				if create {
					createPVC(clusterKey.Name, pvcKey.Name, comp.Name, vct.Spec.Resources.Requests.Storage().String(), storageClassName)
				}
				Eventually(testapps.CheckObjExists(&testCtx, pvcKey,
					&corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
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

	mockPodsForTest := func(cluster *appsv1alpha1.Cluster, number int) []corev1.Pod {
		clusterDefName := cluster.Spec.ClusterDefRef
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
						constant.AppNameLabelKey:              clusterDefName,
						constant.AppInstanceLabelKey:          clusterName,
						constant.KBAppComponentLabelKey:       componentName,
						appsv1.ControllerRevisionHashLabelKey: "mock-version",
					},
					Annotations: map[string]string{
						podAnnotationKey4Test: fmt.Sprintf("%d", number),
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

	horizontalScaleComp := func(updatedReplicas int, comp *appsv1alpha1.ClusterComponentSpec,
		storageClassName string, policy *appsv1alpha1.HorizontalScalePolicy) {
		By("Mocking component PVCs to bound")
		mockComponentPVCsAndBound(comp, int(comp.Replicas), true, storageClassName)

		By("Checking rsm replicas right")
		rsmList := testk8s.ListAndCheckRSMWithComponent(&testCtx, clusterKey, comp.Name)
		Expect(int(*rsmList.Items[0].Spec.Replicas)).To(BeEquivalentTo(comp.Replicas))

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
				rsmList := testk8s.ListAndCheckRSMWithComponent(&testCtx, clusterKey, comp.Name)
				return *rsmList.Items[0].Spec.Replicas
			}).Should(BeEquivalentTo(updatedReplicas))
		}

		scaleOutCheck := func() {
			if comp.Replicas == 0 {
				return
			}

			ml := client.MatchingLabels{
				constant.AppInstanceLabelKey:    clusterKey.Name,
				constant.KBAppComponentLabelKey: comp.Name,
				constant.KBManagedByKey:         "cluster",
			}
			if policy != nil {
				By(fmt.Sprintf("Checking backup of component %s created", comp.Name))
				Eventually(testapps.List(&testCtx, generics.BackupSignature,
					ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(1))

				backupKey := types.NamespacedName{Name: fmt.Sprintf("%s-%s-scaling",
					clusterKey.Name, comp.Name),
					Namespace: testCtx.DefaultNamespace}
				By("Mocking backup status to completed")
				Expect(testapps.GetAndChangeObjStatus(&testCtx, backupKey, func(backup *dpv1alpha1.Backup) {
					backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
					backup.Status.PersistentVolumeClaimName = "backup-data"
					testdp.MockBackupStatusMethod(backup, testapps.DataVolumeName)
				})()).Should(Succeed())

				if testk8s.IsMockVolumeSnapshotEnabled(&testCtx, storageClassName) {
					By("Mocking VolumeSnapshot and set it as ReadyToUse")
					pvcName := getPVCName(testapps.DataVolumeName, comp.Name, 0)
					volumeSnapshot := &snapshotv1.VolumeSnapshot{
						ObjectMeta: metav1.ObjectMeta{
							Name:      backupKey.Name,
							Namespace: backupKey.Namespace,
							Labels: map[string]string{
								dptypes.DataProtectionLabelBackupNameKey: backupKey.Name,
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
			}

			By("Mock PVCs and set status to bound")
			mockComponentPVCsAndBound(comp, updatedReplicas, true, storageClassName)

			if policy != nil {
				checkRestoreAndSetCompleted(clusterKey, comp.Name, updatedReplicas-int(comp.Replicas))
			}

			if policy != nil {
				By("Checking Backup and Restore cleanup")
				Eventually(testapps.List(&testCtx, generics.BackupSignature, ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(0))
				Eventually(testapps.List(&testCtx, generics.RestoreSignature, ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(0))
			}

			checkUpdatedStsReplicas()

			By("Checking updated sts replicas' PVC and size")
			for _, vct := range comp.VolumeClaimTemplates {
				var volumeQuantity resource.Quantity
				for i := 0; i < updatedReplicas; i++ {
					pvcKey := types.NamespacedName{
						Namespace: clusterKey.Namespace,
						Name:      getPVCName(vct.Name, comp.Name, i),
					}
					Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
						if volumeQuantity.IsZero() {
							volumeQuantity = pvc.Spec.Resources.Requests[corev1.ResourceStorage]
						}
						Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(volumeQuantity))
						Expect(pvc.Status.Capacity[corev1.ResourceStorage]).To(Equal(volumeQuantity))
					})).Should(Succeed())
				}
			}
		}

		scaleInCheck := func() {
			if updatedReplicas == 0 {
				Consistently(func(g Gomega) {
					pvcList := corev1.PersistentVolumeClaimList{}
					g.Expect(testCtx.Cli.List(testCtx.Ctx, &pvcList, client.MatchingLabels{
						constant.AppInstanceLabelKey:    clusterKey.Name,
						constant.KBAppComponentLabelKey: comp.Name,
					})).Should(Succeed())
					for _, pvc := range pvcList.Items {
						ss := strings.Split(pvc.Name, "-")
						idx, _ := strconv.Atoi(ss[len(ss)-1])
						if idx >= updatedReplicas && idx < int(comp.Replicas) {
							g.Expect(pvc.DeletionTimestamp).Should(BeNil())
						}
					}
				}).Should(Succeed())
				return
			}

			checkUpdatedStsReplicas()

			By("Checking pvcs deleting")
			Eventually(func(g Gomega) {
				pvcList := corev1.PersistentVolumeClaimList{}
				g.Expect(testCtx.Cli.List(testCtx.Ctx, &pvcList, client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: comp.Name,
				})).Should(Succeed())
				for _, pvc := range pvcList.Items {
					ss := strings.Split(pvc.Name, "-")
					idx, _ := strconv.Atoi(ss[len(ss)-1])
					if idx >= updatedReplicas && idx < int(comp.Replicas) {
						g.Expect(pvc.DeletionTimestamp).ShouldNot(BeNil())
					}
				}
			}).Should(Succeed())

			By("Checking pod's annotation should be updated consistently")
			Eventually(func(g Gomega) {
				podList := corev1.PodList{}
				g.Expect(testCtx.Cli.List(testCtx.Ctx, &podList, client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: comp.Name,
				})).Should(Succeed())
				for _, pod := range podList.Items {
					ss := strings.Split(pod.Name, "-")
					ordinal, _ := strconv.Atoi(ss[len(ss)-1])
					if ordinal >= updatedReplicas {
						continue
					}
					g.Expect(pod.Annotations[podAnnotationKey4Test]).Should(Equal(fmt.Sprintf("%d", updatedReplicas)))
				}
			}).Should(Succeed())
		}

		if int(comp.Replicas) < updatedReplicas {
			scaleOutCheck()
		}
		if int(comp.Replicas) > updatedReplicas {
			scaleInCheck()
		}
	}

	setHorizontalScalePolicy := func(policyType appsv1alpha1.HScaleDataClonePolicyType, componentDefsWithHScalePolicy ...string) {
		By(fmt.Sprintf("Set HorizontalScalePolicy, policyType is %s", policyType))
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

					if len(policyType) == 0 {
						clusterDef.Spec.ComponentDefs[i].HorizontalScalePolicy = nil
						continue
					}

					By("Checking backup policy created from backup policy template")
					policyName := generateBackupPolicyName(clusterKey.Name, compDef.Name, "")
					clusterDef.Spec.ComponentDefs[i].HorizontalScalePolicy = &appsv1alpha1.HorizontalScalePolicy{
						Type:                     policyType,
						BackupPolicyTemplateName: backupPolicyTPLName,
					}

					Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKey{Name: policyName, Namespace: clusterKey.Namespace},
						&dpv1alpha1.BackupPolicy{}, true)).Should(Succeed())

					if policyType == appsv1alpha1.HScaleDataClonePolicyCloneVolume {
						By("creating actionSet if backup policy is backup")
						actionSet := &dpv1alpha1.ActionSet{
							ObjectMeta: metav1.ObjectMeta{
								Name:      actionSetName,
								Namespace: clusterKey.Namespace,
								Labels: map[string]string{
									constant.ClusterDefLabelKey: clusterDef.Name,
								},
							},
							Spec: dpv1alpha1.ActionSetSpec{
								Env: []corev1.EnvVar{
									{
										Name:  "test-name",
										Value: "test-value",
									},
								},
								BackupType: dpv1alpha1.BackupTypeFull,
								Backup: &dpv1alpha1.BackupActionSpec{
									BackupData: &dpv1alpha1.BackupDataActionSpec{
										JobActionSpec: dpv1alpha1.JobActionSpec{
											Image:   "xtrabackup",
											Command: []string{""},
										},
									},
								},
								Restore: &dpv1alpha1.RestoreActionSpec{
									PrepareData: &dpv1alpha1.JobActionSpec{
										Image: "xtrabackup",
										Command: []string{
											"sh",
											"-c",
											"/backup_scripts.sh",
										},
									},
								},
							},
						}
						testapps.CheckedCreateK8sResource(&testCtx, actionSet)
					}
				}
			})()).ShouldNot(HaveOccurred())
	}

	// @argument componentDefsWithHScalePolicy assign ClusterDefinition.spec.componentDefs[].horizontalScalePolicy for
	// the matching names. If not provided, will set 1st ClusterDefinition.spec.componentDefs[0].horizontalScalePolicy.
	horizontalScale := func(updatedReplicas int, storageClassName string,
		policyType appsv1alpha1.HScaleDataClonePolicyType, componentDefsWithHScalePolicy ...string) {
		defer lorry.UnsetMockClient()

		cluster := &appsv1alpha1.Cluster{}
		Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(Succeed())
		initialGeneration := int(cluster.Status.ObservedGeneration)

		setHorizontalScalePolicy(policyType, componentDefsWithHScalePolicy...)

		By("Mocking all components' PVCs to bound")
		for _, comp := range cluster.Spec.ComponentSpecs {
			mockComponentPVCsAndBound(&comp, int(comp.Replicas), true, storageClassName)
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
		for i, comp := range cluster.Spec.ComponentSpecs {
			lorry.SetMockClient(&mockLorryClient{replicas: updatedReplicas, clusterKey: clusterKey, compName: comp.Name}, nil)

			By(fmt.Sprintf("H-scale component %s with policy %s", comp.Name, hscalePolicy(comp)))
			horizontalScaleComp(updatedReplicas, &cluster.Spec.ComponentSpecs[i], storageClassName, hscalePolicy(comp))
		}

		By("Checking cluster status and the number of replicas changed")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).
			Should(BeEquivalentTo(initialGeneration + len(cluster.Spec.ComponentSpecs)))
	}

	testHorizontalScale := func(compName, compDefName string, initialReplicas, updatedReplicas int32,
		dataClonePolicy appsv1alpha1.HScaleDataClonePolicyType) {
		By("Creating a single component cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(compName, compDefName).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			AddVolumeClaimTemplate(testapps.LogVolumeName, pvcSpec).
			SetReplicas(initialReplicas).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		// REVIEW: this test flow, wait for running phase?
		testk8s.MockEnableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)
		viper.Set(constant.CfgKeyBackupPVCName, "")

		horizontalScale(int(updatedReplicas), testk8s.DefaultStorageClassName, dataClonePolicy, compDefName)
	}

	testVolumeExpansion := func(compName, compDefName string, storageClass *storagev1.StorageClass) {
		var (
			replicas          = 3
			volumeSize        = "1Gi"
			newVolumeSize     = "2Gi"
			volumeQuantity    = resource.MustParse(volumeSize)
			newVolumeQuantity = resource.MustParse(newVolumeSize)
		)

		By("Mock a StorageClass which allows resize")
		Expect(*storageClass.AllowVolumeExpansion).Should(BeTrue())

		By("Creating a cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVCSpec(volumeSize)
		pvcSpec.StorageClassName = &storageClass.Name

		By("Create cluster and waiting for the cluster initialized")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(compName, compDefName).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			AddVolumeClaimTemplate(testapps.LogVolumeName, pvcSpec).
			SetReplicas(int32(replicas)).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Checking the replicas")
		rsmList := testk8s.ListAndCheckRSM(&testCtx, clusterKey)
		rsm := &rsmList.Items[0]
		sts := testapps.NewStatefulSetFactory(rsm.Namespace, rsm.Name, clusterObj.Name, compName).
			SetReplicas(*rsm.Spec.Replicas).
			Create(&testCtx).GetObject()

		Expect(*sts.Spec.Replicas).Should(BeEquivalentTo(replicas))

		By("Mock PVCs in Bound Status")
		for i := 0; i < replicas; i++ {
			for _, vctName := range []string{testapps.DataVolumeName, testapps.LogVolumeName} {
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getPVCName(vctName, compName, i),
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
		}

		By("mock pods/sts of component are available")
		var mockPods []*corev1.Pod
		switch compDefName {
		case statelessCompDefName:
			// ignore
		case replicationCompDefName:
			mockPods = testapps.MockReplicationComponentPods(nil, testCtx, sts, clusterObj.Name, compDefName, nil)
		case statefulCompDefName, consensusCompDefName:
			mockPods = testapps.MockConsensusComponentPods(&testCtx, sts, clusterObj.Name, compName)
		}
		Expect(testapps.ChangeObjStatus(&testCtx, rsm, func() {
			testk8s.MockRSMReady(rsm, mockPods...)
		})).ShouldNot(HaveOccurred())
		Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
			testk8s.MockStatefulSetReady(sts)
		})).ShouldNot(HaveOccurred())

		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))

		By("Updating data PVC storage size")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			comp := &cluster.Spec.ComponentSpecs[0]
			for i, vct := range comp.VolumeClaimTemplates {
				if vct.Name == testapps.DataVolumeName {
					comp.VolumeClaimTemplates[i].Spec.Resources.Requests[corev1.ResourceStorage] = newVolumeQuantity
				}
			}
		})()).ShouldNot(HaveOccurred())

		By("Checking the resize operation in progress for data volume")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(appsv1alpha1.UpdatingClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.UpdatingClusterPhase))
		for i := 0; i < replicas; i++ {
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(testapps.DataVolumeName, compName, i),
			}
			Expect(k8sClient.Get(testCtx.Ctx, pvcKey, pvc)).Should(Succeed())
			Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(newVolumeQuantity))
			Expect(pvc.Status.Capacity[corev1.ResourceStorage]).To(Equal(volumeQuantity))
		}

		By("Mock resizing of data volumes finished")
		for i := 0; i < replicas; i++ {
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(testapps.DataVolumeName, compName, i),
			}
			Expect(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Status.Capacity[corev1.ResourceStorage] = newVolumeQuantity
			})()).ShouldNot(HaveOccurred())
		}

		By("Checking the resize operation finished")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))

		By("Checking data volumes are resized")
		for i := 0; i < replicas; i++ {
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(testapps.DataVolumeName, compName, i),
			}
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.Status.Capacity[corev1.ResourceStorage]).To(Equal(newVolumeQuantity))
			})).Should(Succeed())
		}

		By("Checking log volumes stay unchanged")
		for i := 0; i < replicas; i++ {
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(testapps.LogVolumeName, compName, i),
			}
			Expect(k8sClient.Get(testCtx.Ctx, pvcKey, pvc)).Should(Succeed())
			Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(volumeQuantity))
			Expect(pvc.Status.Capacity[corev1.ResourceStorage]).To(Equal(volumeQuantity))
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
		rsmList := testk8s.ListAndCheckRSM(&testCtx, clusterKey)
		numbers := *rsmList.Items[0].Spec.Replicas

		Expect(numbers).Should(BeEquivalentTo(replicas))

		By("Mock PVCs in Bound Status")
		for i := 0; i < replicas; i++ {
			tmpSpec := pvcSpec.ToV1PersistentVolumeClaimSpec()
			tmpSpec.VolumeName = getPVCName(testapps.DataVolumeName, compName, i)
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getPVCName(testapps.DataVolumeName, compName, i),
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
					Name:      getPVCName(testapps.DataVolumeName, compName, i), // use same name as pvc
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
						Name: getPVCName(testapps.DataVolumeName, compName, i),
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
		rsmList = testk8s.ListAndCheckRSM(&testCtx, clusterKey)
		numbers = *rsmList.Items[0].Spec.Replicas
		for i := numbers - 1; i >= 0; i-- {
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(testapps.DataVolumeName, compName, int(i)),
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
			rsmList = testk8s.ListAndCheckRSM(&testCtx, clusterKey)
			numbers = *rsmList.Items[0].Spec.Replicas
			for i := numbers - 1; i >= 0; i-- {
				pvc := &corev1.PersistentVolumeClaim{}
				pvcKey := types.NamespacedName{
					Namespace: clusterKey.Namespace,
					Name:      getPVCName(testapps.DataVolumeName, compName, int(i)),
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
			AddComponent(compName, compDefName).SetReplicas(1).SetComponentAffinity(compAffinity).
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
			AddComponent(compName, compDefName).SetReplicas(1).AddComponentToleration(compToleration).
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

	getStsPodsName := func(sts *appsv1.StatefulSet) []string {
		pods, err := common.GetPodListByStatefulSet(ctx, k8sClient, sts)
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

		var rsm *workloads.ReplicatedStateMachine
		Eventually(func(g Gomega) {
			rsmList := testk8s.ListAndCheckRSM(&testCtx, clusterKey)
			g.Expect(rsmList.Items).ShouldNot(BeEmpty())
			rsm = &rsmList.Items[0]
		}).Should(Succeed())
		sts := testapps.NewStatefulSetFactory(rsm.Namespace, rsm.Name, clusterKey.Name, compName).
			AddAppComponentLabel(rsm.Labels[constant.KBAppComponentLabelKey]).
			AddAppInstanceLabel(rsm.Labels[constant.AppInstanceLabelKey]).
			SetReplicas(*rsm.Spec.Replicas).Create(&testCtx).GetObject()

		By("Creating mock pods in StatefulSet, and set controller reference")
		pods := mockPodsForTest(clusterObj, replicas)
		for i, pod := range pods {
			Expect(controllerutil.SetControllerReference(sts, &pod, scheme.Scheme)).Should(Succeed())
			Expect(testCtx.CreateObj(testCtx.Ctx, &pod)).Should(Succeed())
			patch := client.MergeFrom(pod.DeepCopy())
			// mock the status to pass the isReady(pod) check in consensus_set
			pod.Status.Conditions = []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}}
			Eventually(k8sClient.Status().Patch(ctx, &pod, patch)).Should(Succeed())
			role := "follower"
			if i == 0 {
				role = "leader"
			}
			patch = client.MergeFrom(pod.DeepCopy())
			pod.Labels[constant.RoleLabelKey] = role
			Eventually(k8sClient.Patch(ctx, &pod, patch)).Should(Succeed())
		}

		By("Checking pods' role are changed accordingly")
		Eventually(func(g Gomega) {
			pods, err := common.GetPodListByStatefulSet(ctx, k8sClient, sts)
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

		// trigger rsm to reconcile as the underlying sts is not created
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(sts), func(rsm *workloads.ReplicatedStateMachine) {
			rsm.Annotations = map[string]string{"time": time.Now().Format(time.RFC3339)}
		})()).Should(Succeed())
		By("Checking pods' annotations")
		Eventually(func(g Gomega) {
			pods, err := common.GetPodListByStatefulSet(ctx, k8sClient, sts)
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(pods).Should(HaveLen(int(*sts.Spec.Replicas)))
			for _, pod := range pods {
				g.Expect(pod.Annotations).ShouldNot(BeNil())
				g.Expect(pod.Annotations[constant.ComponentReplicasAnnotationKey]).Should(Equal(strconv.Itoa(int(*sts.Spec.Replicas))))
			}
		}).Should(Succeed())
		rsmPatch := client.MergeFrom(rsm.DeepCopy())
		By("Updating RSM's status")
		rsm.Status.UpdateRevision = "mock-version"
		pods, err := common.GetPodListByStatefulSet(ctx, k8sClient, sts)
		Expect(err).Should(BeNil())
		var podList []*corev1.Pod
		for i := range pods {
			podList = append(podList, &pods[i])
		}
		testk8s.MockRSMReady(rsm, podList...)
		Expect(k8sClient.Status().Patch(ctx, rsm, rsmPatch)).Should(Succeed())

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
		testk8s.MockEnableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)

		By("Set HorizontalScalePolicy")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
			func(clusterDef *appsv1alpha1.ClusterDefinition) {
				for i, def := range clusterDef.Spec.ComponentDefs {
					if def.Name != compDefName {
						continue
					}
					clusterDef.Spec.ComponentDefs[i].HorizontalScalePolicy =
						&appsv1alpha1.HorizontalScalePolicy{Type: appsv1alpha1.HScaleDataClonePolicyCloneVolume,
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

		By("Create and Mock PVCs status to bound")
		for _, comp := range clusterObj.Spec.ComponentSpecs {
			mockComponentPVCsAndBound(&comp, int(comp.Replicas), true, testk8s.DefaultStorageClassName)
		}

		By(fmt.Sprintf("Changing replicas to %d", updatedReplicas))
		changeCompReplicas(clusterKey, updatedReplicas, &clusterObj.Spec.ComponentSpecs[0])
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))

		By("Waiting for the backup object been created")
		ml := client.MatchingLabels{
			constant.AppInstanceLabelKey:    clusterKey.Name,
			constant.KBAppComponentLabelKey: compName,
		}
		Eventually(testapps.List(&testCtx, generics.BackupSignature,
			ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(1))

		By("Mocking backup status to failed")
		backupList := dpv1alpha1.BackupList{}
		Expect(testCtx.Cli.List(testCtx.Ctx, &backupList, ml)).Should(Succeed())
		backupKey := types.NamespacedName{
			Namespace: backupList.Items[0].Namespace,
			Name:      backupList.Items[0].Name,
		}
		Expect(testapps.GetAndChangeObjStatus(&testCtx, backupKey, func(backup *dpv1alpha1.Backup) {
			backup.Status.Phase = dpv1alpha1.BackupPhaseFailed
		})()).Should(Succeed())

		By("Checking cluster status failed with backup error")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
			g.Expect(testk8s.IsMockVolumeSnapshotEnabled(&testCtx, testk8s.DefaultStorageClassName)).Should(BeTrue())
			g.Expect(cluster.Status.Conditions).ShouldNot(BeEmpty())
			var err error
			for _, cond := range cluster.Status.Conditions {
				if strings.Contains(cond.Message, "backup for horizontalScaling failed") {
					err = errors.New("has backup error")
					break
				}
			}
			g.Expect(err).Should(HaveOccurred())
		})).Should(Succeed())

		By("Expect for backup error event")
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

	testUpdateKubeBlocksToolsImage := func(compName, compDefName string) {
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(compName, compDefName).SetReplicas(1).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		oldToolsImage := viper.GetString(constant.KBToolsImage)
		newToolsImage := fmt.Sprintf("%s-%s", oldToolsImage, rand.String(4))
		defer func() {
			viper.Set(constant.KBToolsImage, oldToolsImage)
		}()

		checkWorkloadGenerationAndToolsImage := func(workloadGenerationExpected int64, oldImageCntExpected, newImageCntExpected int) {
			checkSingleWorkload(compDefName, func(g Gomega, sts *appsv1.StatefulSet, deploy *appsv1.Deployment) {
				if sts != nil {
					g.Expect(sts.Generation).Should(Equal(workloadGenerationExpected))
				}
				if deploy != nil {
					g.Expect(deploy.Generation).Should(Equal(workloadGenerationExpected))
				}
				oldImageCnt := 0
				newImageCnt := 0
				for _, c := range getPodSpec(sts, deploy).Containers {
					if c.Image == oldToolsImage {
						oldImageCnt += 1
					}
					if c.Image == newToolsImage {
						newImageCnt += 1
					}
				}
				g.Expect(oldImageCnt).Should(Equal(oldImageCntExpected))
				g.Expect(newImageCnt).Should(Equal(newImageCntExpected))
			})
		}

		By("check the workload generation as 1")
		checkWorkloadGenerationAndToolsImage(int64(1), 1, 0)

		By("update kubeblocks tools image")
		viper.Set(constant.KBToolsImage, newToolsImage)

		By("update cluster annotation to trigger cluster status reconcile")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			cluster.Annotations = map[string]string{"time": time.Now().Format(time.RFC3339)}
		})()).Should(Succeed())
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
			g.Expect(cluster.Status.ObservedGeneration).Should(Equal(int64(1)))
		})).Should(Succeed())
		checkWorkloadGenerationAndToolsImage(int64(1), 1, 0)

		By("update termination policy to trigger cluster spec reconcile, but workload not changed")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			cluster.Spec.TerminationPolicy = appsv1alpha1.DoNotTerminate
		})()).Should(Succeed())
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
			g.Expect(cluster.Status.ObservedGeneration).Should(Equal(int64(2)))
		})).Should(Succeed())
		checkWorkloadGenerationAndToolsImage(int64(1), 1, 0)

		By("update replicas to trigger cluster spec and workload reconcile")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			replicas := cluster.Spec.ComponentSpecs[0].Replicas
			cluster.Spec.ComponentSpecs[0].Replicas = replicas + 1
		})()).Should(Succeed())
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
			g.Expect(cluster.Status.ObservedGeneration).Should(Equal(int64(3)))
		})).Should(Succeed())
		checkWorkloadGenerationAndToolsImage(int64(2), 0, 1)
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
			cleanEnv()
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

			By("Check stateless workload has been created")
			Eventually(testapps.List(&testCtx, generics.RSMSignature,
				client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterKey.Name,
					constant.KBAppComponentLabelKey: statelessCompName,
				}, client.InNamespace(clusterKey.Namespace))).ShouldNot(HaveLen(0))

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
			}, false)

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey, statefulCompName, consensusCompName, replicationCompName)

			// statefulCompDefName not in componentDefsWithHScalePolicy, for nil backup policy test
			// REVIEW:
			//  1. this test flow, wait for running phase?
			horizontalScale(int(updatedReplicas), testk8s.DefaultStorageClassName, policyType, consensusCompDefName, replicationCompDefName)
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

		It("should successfully h-scale with multiple components", func() {
			testk8s.MockEnableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)
			viper.Set(constant.CfgKeyBackupPVCName, "")
			testMultiCompHScale(appsv1alpha1.HScaleDataClonePolicyCloneVolume)
		})

		It("should successfully h-scale with multiple components by backup tool", func() {
			testk8s.MockDisableVolumeSnapshot(&testCtx, testk8s.DefaultStorageClassName)
			viper.Set(constant.CfgKeyBackupPVCName, "test-backup-pvc")
			testMultiCompHScale(appsv1alpha1.HScaleDataClonePolicyCloneVolume)
		})
	})

	When("creating cluster with backup configuration", func() {
		const (
			compName                       = statefulCompName
			compDefName                    = statefulCompDefName
			backupRepoName                 = "test-backup-repo"
			backupMethodName               = "test-backup-method"
			volumeSnapshotBackupMethodName = "test-vs-backup-method"
		)
		BeforeEach(func() {
			cleanEnv()
			createAllWorkloadTypesClusterDef()
			createBackupPolicyTpl(clusterDefObj)
		})

		createClusterWithBackup := func(backup *appsv1alpha1.ClusterBackup) {
			By("Creating a cluster")
			clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).
				AddComponent(compName, compDefName).WithRandomName().SetBackup(backup).
				Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey)
		}

		It("Creating cluster without backup", func() {
			createClusterWithBackup(nil)
			Eventually(testapps.List(&testCtx, generics.BackupPolicySignature,
				client.MatchingLabels{
					constant.AppInstanceLabelKey: clusterKey.Name,
				}, client.InNamespace(clusterKey.Namespace))).ShouldNot(BeEmpty())
		})

		It("Creating cluster with backup", func() {
			var (
				boolTrue  = true
				boolFalse = false
				int64Ptr  = func(in int64) *int64 {
					return &in
				}
				retention = func(s string) dpv1alpha1.RetentionPeriod {
					return dpv1alpha1.RetentionPeriod(s)
				}
			)

			var testCases = []struct {
				desc   string
				backup *appsv1alpha1.ClusterBackup
			}{
				{
					desc: "backup with snapshot method",
					backup: &appsv1alpha1.ClusterBackup{
						Enabled:                 &boolTrue,
						RetentionPeriod:         retention("1d"),
						Method:                  vsBackupMethodName,
						CronExpression:          "*/1 * * * *",
						StartingDeadlineMinutes: int64Ptr(int64(10)),
						PITREnabled:             &boolTrue,
						RepoName:                backupRepoName,
					},
				},
				{
					desc: "disable backup",
					backup: &appsv1alpha1.ClusterBackup{
						Enabled:                 &boolFalse,
						RetentionPeriod:         retention("1d"),
						Method:                  vsBackupMethodName,
						CronExpression:          "*/1 * * * *",
						StartingDeadlineMinutes: int64Ptr(int64(10)),
						PITREnabled:             &boolTrue,
						RepoName:                backupRepoName,
					},
				},
				{
					desc: "backup with backup tool",
					backup: &appsv1alpha1.ClusterBackup{
						Enabled:                 &boolTrue,
						RetentionPeriod:         retention("2d"),
						Method:                  backupMethodName,
						CronExpression:          "*/1 * * * *",
						StartingDeadlineMinutes: int64Ptr(int64(10)),
						RepoName:                backupRepoName,
						PITREnabled:             &boolFalse,
					},
				},
				{
					desc:   "backup is nil",
					backup: nil,
				},
			}

			for _, t := range testCases {
				By(t.desc)
				backup := t.backup
				createClusterWithBackup(backup)

				checkSchedule := func(g Gomega, schedule *dpv1alpha1.BackupSchedule) {
					var policy *dpv1alpha1.SchedulePolicy
					for i, s := range schedule.Spec.Schedules {
						if s.BackupMethod == backup.Method {
							Expect(*s.Enabled).Should(BeEquivalentTo(*backup.Enabled))
							policy = &schedule.Spec.Schedules[i]
						}
					}
					if backup.Enabled != nil && *backup.Enabled {
						Expect(policy).ShouldNot(BeNil())
						Expect(policy.RetentionPeriod).Should(BeEquivalentTo(backup.RetentionPeriod))
						Expect(policy.CronExpression).Should(BeEquivalentTo(backup.CronExpression))
					}
				}

				checkPolicy := func(g Gomega, policy *dpv1alpha1.BackupPolicy) {
					if backup != nil && backup.RepoName != "" {
						Expect(*policy.Spec.BackupRepoName).Should(BeEquivalentTo(backup.RepoName))
					}
				}

				By("checking backup policy")
				backupPolicyName := generateBackupPolicyName(clusterKey.Name, compDefName, "")
				backupPolicyKey := client.ObjectKey{Name: backupPolicyName, Namespace: clusterKey.Namespace}
				backupPolicy := &dpv1alpha1.BackupPolicy{}
				Eventually(testapps.CheckObjExists(&testCtx, backupPolicyKey, backupPolicy, true)).Should(Succeed())
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, checkPolicy)).Should(Succeed())

				By("checking backup schedule")
				backupScheduleName := generateBackupScheduleName(clusterKey.Name, compDefName, "")
				backupScheduleKey := client.ObjectKey{Name: backupScheduleName, Namespace: clusterKey.Namespace}
				if backup == nil {
					Eventually(testapps.CheckObjExists(&testCtx, backupScheduleKey,
						&dpv1alpha1.BackupSchedule{}, false)).Should(Succeed())
					continue
				}
				Eventually(testapps.CheckObj(&testCtx, backupScheduleKey, checkSchedule)).Should(Succeed())
			}
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
			createBackupPolicyTpl(clusterDefObj)
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

			It(fmt.Sprintf("[comp: %s] should create/delete pods to match the desired replica number if updating cluster's replica number to a valid value", compName), func() {
				testChangeReplicas(compName, compDefName)
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

			It(fmt.Sprintf("[comp: %s] update kubeblocks-tools image", compName), func() {
				testUpdateKubeBlocksToolsImage(compName, compDefName)
			})
		}
	})

	When("creating cluster with stateful workloadTypes (being Stateful|Consensus|Replication) component", func() {
		var (
			mockStorageClass *storagev1.StorageClass
		)

		compNameNDef := map[string]string{
			statefulCompName:    statefulCompDefName,
			consensusCompName:   consensusCompDefName,
			replicationCompName: replicationCompDefName,
		}

		BeforeEach(func() {
			createAllWorkloadTypesClusterDef()
			createBackupPolicyTpl(clusterDefObj)
			mockStorageClass = testk8s.CreateMockStorageClass(&testCtx, testk8s.DefaultStorageClassName)
		})

		for compName, compDefName := range compNameNDef {
			Context(fmt.Sprintf("[comp: %s] volume expansion", compName), func() {
				It("should update PVC request storage size accordingly", func() {
					testVolumeExpansion(compName, compDefName, mockStorageClass)
				})

				It("should be able to recover if volume expansion fails", func() {
					testVolumeExpansionFailedAndRecover(compName, compDefName)
				})
			})

			Context(fmt.Sprintf("[comp: %s] horizontal scale", compName), func() {
				It("scale-out from 1 to 3 with backup(snapshot) policy normally", func() {
					testHorizontalScale(compName, compDefName, 1, 3, appsv1alpha1.HScaleDataClonePolicyCloneVolume)
				})

				It("backup error at scale-out", func() {
					testBackupError(compName, compDefName)
				})

				It("scale-out without data clone policy", func() {
					testHorizontalScale(compName, compDefName, 1, 3, "")
				})

				It("scale-in from 3 to 1", func() {
					testHorizontalScale(compName, compDefName, 3, 1, appsv1alpha1.HScaleDataClonePolicyCloneVolume)
				})

				It("scale-in to 0 and PVCs should not been deleted", func() {
					testHorizontalScale(compName, compDefName, 3, 0, appsv1alpha1.HScaleDataClonePolicyCloneVolume)
				})

				It("scale-out from 0 and should work well", func() {
					testHorizontalScale(compName, compDefName, 0, 3, appsv1alpha1.HScaleDataClonePolicyCloneVolume)
				})
			})

			Context(fmt.Sprintf("[comp: %s] scale-out after volume expansion", compName), func() {
				It("scale-out with data clone policy", func() {
					testVolumeExpansion(compName, compDefName, mockStorageClass)
					testk8s.MockEnableVolumeSnapshot(&testCtx, mockStorageClass.Name)
					viper.Set(constant.CfgKeyBackupPVCName, "")
					horizontalScale(5, mockStorageClass.Name, appsv1alpha1.HScaleDataClonePolicyCloneVolume, compDefName)
				})

				It("scale-out without data clone policy", func() {
					testVolumeExpansion(compName, compDefName, mockStorageClass)
					horizontalScale(5, mockStorageClass.Name, "", compDefName)
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
			_ = testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml",
				&dpv1alpha1.ActionSet{}, testapps.RandomizedObjName())

			By("creating backup")
			backup := testdp.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				SetBackupPolicyName(backupPolicyName).
				SetBackupMethod(testdp.BackupMethodName).
				Create(&testCtx).GetObject()

			By("mocking backup status completed, we don't need backup reconcile here")
			Eventually(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(backup), func(backup *dpv1alpha1.Backup) {
				backup.Status.PersistentVolumeClaimName = "backup-pvc"
				backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
				testdp.MockBackupStatusMethod(backup, testapps.DataVolumeName)
			})).Should(Succeed())

			By("creating cluster with backup")
			restoreFromBackup := fmt.Sprintf(`{"%s":{"name":"%s"}}`, compName, backupName)
			pvcSpec := testapps.NewPVCSpec("1Gi")
			replicas := 3
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(compName, compDefName).
				SetReplicas(int32(replicas)).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				AddAnnotations(constant.RestoreFromBackupAnnotationKey, restoreFromBackup).Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			// mock pvcs have restored
			mockComponentPVCsAndBound(clusterObj.Spec.GetComponentByName(compName), replicas, true, testk8s.DefaultStorageClassName)
			By("wait for restore created")
			ml := client.MatchingLabels{
				constant.AppInstanceLabelKey:    clusterKey.Name,
				constant.KBAppComponentLabelKey: compName,
			}
			Eventually(testapps.List(&testCtx, generics.RestoreSignature,
				ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(1))

			By("Mocking restore phase to Completed")
			// mock prepareData restore completed
			mockRestoreCompleted(ml)

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey, compName)

			rsmList := testk8s.ListAndCheckRSM(&testCtx, clusterKey)
			rsm := rsmList.Items[0]
			sts := testapps.NewStatefulSetFactory(rsm.Namespace, rsm.Name, clusterKey.Name, compName).
				SetReplicas(*rsm.Spec.Replicas).
				Create(&testCtx).GetObject()
			By("mock pod/sts are available and wait for component enter running phase")
			mockPods := testapps.MockConsensusComponentPods(&testCtx, sts, clusterObj.Name, compName)
			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				testk8s.MockStatefulSetReady(sts)
			})).ShouldNot(HaveOccurred())
			Expect(testapps.ChangeObjStatus(&testCtx, &rsm, func() {
				testk8s.MockRSMReady(&rsm, mockPods...)
			})).ShouldNot(HaveOccurred())
			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))

			By("the restore container has been removed from init containers")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(&rsm), func(g Gomega, tmpRSM *workloads.ReplicatedStateMachine) {
				g.Expect(tmpRSM.Spec.Template.Spec.InitContainers).Should(BeEmpty())
			})).Should(Succeed())

			By("clean up annotations after cluster running")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				g.Expect(tmpCluster.Status.Phase).Should(Equal(appsv1alpha1.RunningClusterPhase))
				// mock postReady restore completed
				mockRestoreCompleted(ml)
				g.Expect(tmpCluster.Annotations[constant.RestoreFromBackupAnnotationKey]).Should(BeEmpty())
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
				SetReplicas(testapps.DefaultReplicationReplicas).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey, compDefName)

			By("Checking statefulSet number")
			rsmList := testk8s.ListAndCheckRSMItemsCount(&testCtx, clusterKey, 1)
			rsm := &rsmList.Items[0]
			sts := testapps.NewStatefulSetFactory(rsm.Namespace, rsm.Name, clusterKey.Name, compName).
				SetReplicas(*rsm.Spec.Replicas).Create(&testCtx).GetObject()
			mockPods := testapps.MockReplicationComponentPods(nil, testCtx, sts, clusterObj.Name, compDefName, nil)
			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				testk8s.MockStatefulSetReady(sts)
			})).ShouldNot(HaveOccurred())
			Expect(testapps.ChangeObjStatus(&testCtx, rsm, func() {
				testk8s.MockRSMReady(rsm, mockPods...)
			})).ShouldNot(HaveOccurred())
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

func createBackupPolicyTpl(clusterDefObj *appsv1alpha1.ClusterDefinition) {
	By("Creating a BackupPolicyTemplate")
	bpt := testapps.NewBackupPolicyTemplateFactory(backupPolicyTPLName).
		AddLabels(constant.ClusterDefLabelKey, clusterDefObj.Name).
		SetClusterDefRef(clusterDefObj.Name)
	for _, v := range clusterDefObj.Spec.ComponentDefs {
		bpt = bpt.AddBackupPolicy(v.Name).
			AddBackupMethod(backupMethodName, false, actionSetName).
			SetBackupMethodVolumeMounts("data", "/data").
			AddBackupMethod(vsBackupMethodName, true, vsActionSetName).
			SetBackupMethodVolumes([]string{"data"}).
			AddSchedule(backupMethodName, "0 0 * * *", true).
			AddSchedule(vsBackupMethodName, "0 0 * * *", true)
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

func mockRestoreCompleted(ml client.MatchingLabels) {
	restoreList := dpv1alpha1.RestoreList{}
	Expect(testCtx.Cli.List(testCtx.Ctx, &restoreList, ml)).Should(Succeed())
	for _, rs := range restoreList.Items {
		err := testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(&rs), func(res *dpv1alpha1.Restore) {
			res.Status.Phase = dpv1alpha1.RestorePhaseCompleted
		})()
		Expect(client.IgnoreNotFound(err)).ShouldNot(HaveOccurred())
	}
}

func checkRestoreAndSetCompleted(clusterKey types.NamespacedName, compName string, scaleOutReplicas int) {
	By("Checking restore CR created")
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey:    clusterKey.Name,
		constant.KBAppComponentLabelKey: compName,
		constant.KBManagedByKey:         "cluster",
	}
	Eventually(testapps.List(&testCtx, generics.RestoreSignature,
		ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(scaleOutReplicas))

	By("Mocking restore phase to succeeded")
	mockRestoreCompleted(ml)
}
