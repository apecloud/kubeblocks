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
	"reflect"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Cluster Controller", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterNamePrefix = "test-cluster"

	const mysqlCompType = "replicasets"
	const mysqlCompName = "mysql"

	const nginxCompType = "proxy"
	const nginxCompName = "nginx"

	const leader = "leader"
	const follower = "follower"

	// Cleanups

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
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
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
		// non-namespaced
		testapps.ClearResources(&testCtx, intctrlutil.BackupPolicyTemplateSignature, ml)
		testapps.ClearResources(&testCtx, intctrlutil.StorageClassSignature, ml)
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

	// Test cases

	checkAllResourcesCreated := func() {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(mysqlCompName, mysqlCompType).SetReplicas(3).
			AddComponent(nginxCompName, nginxCompType).SetReplicas(3).
			WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster initialized")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Check deployment workload has been created")
		Eventually(testapps.GetListLen(&testCtx, intctrlutil.DeploymentSignature,
			client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey: clusterKey.Name,
			}, client.InNamespace(clusterKey.Namespace))).ShouldNot(Equal(0))

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
		Eventually(testapps.GetListLen(&testCtx, intctrlutil.PodDisruptionBudgetSignature,
			client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey: clusterKey.Name,
			}, client.InNamespace(clusterKey.Namespace))).Should(Equal(0))

		podSpec := stsList.Items[0].Spec.Template.Spec
		By("Checking created sts pods template without tolerations")
		Expect(len(podSpec.Tolerations) == 0).Should(BeTrue())

		By("Checking created sts pods template without Affinity")
		Expect(podSpec.Affinity).Should(BeNil())

		By("Checking created sts pods template without TopologySpreadConstraints")
		Expect(len(podSpec.TopologySpreadConstraints) == 0).Should(BeTrue())

		By("Check should create env configmap")
		Eventually(testapps.GetListLen(&testCtx, intctrlutil.ConfigMapSignature,
			client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey:   clusterKey.Name,
				intctrlutil.AppConfigTypeLabelKey: "kubeblocks-env",
			}, client.InNamespace(clusterKey.Namespace))).Should(Equal(2))
	}

	checkAllServicesCreate := func() {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(mysqlCompName, mysqlCompType).SetReplicas(1).
			AddComponent(nginxCompName, nginxCompType).SetReplicas(3).
			WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster initialized")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Checking proxy should have external ClusterIP service")
		svcList1 := &corev1.ServiceList{}
		Expect(k8sClient.List(testCtx.Ctx, svcList1, client.MatchingLabels{
			intctrlutil.AppInstanceLabelKey:  clusterKey.Name,
			intctrlutil.AppComponentLabelKey: nginxCompName,
		}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
		// TODO fix me later, proxy should not have internal headless service
		// Expect(len(svcList1.Items) == 1).Should(BeTrue())
		Expect(len(svcList1.Items) > 0).Should(BeTrue())
		var existsExternalClusterIP bool
		for _, svc := range svcList1.Items {
			Expect(svc.Spec.Type == corev1.ServiceTypeClusterIP).To(BeTrue())
			if svc.Spec.ClusterIP == corev1.ClusterIPNone {
				continue
			}
			existsExternalClusterIP = true
		}
		Expect(existsExternalClusterIP).To(BeTrue())

		By("Checking replicasets should have internal headless service")
		getHeadlessSvcPorts := func(compDefName string) []corev1.ServicePort {
			fetched := &appsv1alpha1.Cluster{}
			Expect(k8sClient.Get(testCtx.Ctx, clusterKey, fetched)).To(Succeed())

			comp, err := util.GetComponentDefByCluster(testCtx.Ctx, k8sClient, fetched, compDefName)
			Expect(err).ShouldNot(HaveOccurred())

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

		svcList2 := &corev1.ServiceList{}
		Expect(k8sClient.List(testCtx.Ctx, svcList2, client.MatchingLabels{
			intctrlutil.AppInstanceLabelKey:  clusterKey.Name,
			intctrlutil.AppComponentLabelKey: mysqlCompName,
		}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
		Expect(len(svcList2.Items)).Should(BeEquivalentTo(1))
		Expect(svcList2.Items[0].Spec.Type == corev1.ServiceTypeClusterIP).To(BeTrue())
		Expect(svcList2.Items[0].Spec.ClusterIP == corev1.ClusterIPNone).To(BeTrue())
		Expect(reflect.DeepEqual(svcList2.Items[0].Spec.Ports,
			getHeadlessSvcPorts(mysqlCompType))).Should(BeTrue())
	}

	testWipeOut := func() {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster initialized")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Delete the cluster")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})

		By("Wait for the cluster to terminate")
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, false)).Should(Succeed())
	}

	testDoNotTermintate := func() {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster initialized")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Update the cluster's termination policy to DoNotTerminate")
		Eventually(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			cluster.Spec.TerminationPolicy = appsv1alpha1.DoNotTerminate
		})).Should(Succeed())

		By("Delete the cluster")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})

		By("Check the cluster do not terminate immediately")
		checkClusterDoNotTerminate := testapps.CheckObj(&testCtx, clusterKey,
			func(g Gomega, fetched *appsv1alpha1.Cluster) {
				g.Expect(fetched.Status.Message).To(ContainSubstring(
					fmt.Sprintf("spec.terminationPolicy %s is preventing deletion.", fetched.Spec.TerminationPolicy)))
				g.Expect(len(fetched.Finalizers) > 0).To(BeTrue())
			})
		Eventually(checkClusterDoNotTerminate).Should(Succeed())
		Consistently(checkClusterDoNotTerminate).Should(Succeed())

		By("Update the cluster's termination policy to WipeOut")
		Eventually(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			cluster.Spec.TerminationPolicy = appsv1alpha1.WipeOut
		})).Should(Succeed())

		By("Wait for the cluster to terminate")
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, false)).Should(Succeed())
	}

	changeCompReplicas := func(clusterName types.NamespacedName, replicas int32, comp *appsv1alpha1.ClusterComponentSpec) {
		Eventually(testapps.GetAndChangeObj(&testCtx, clusterName, func(cluster *appsv1alpha1.Cluster) {
			for i, clusterComp := range cluster.Spec.ComponentSpecs {
				if clusterComp.Name == comp.Name {
					cluster.Spec.ComponentSpecs[i].Replicas = replicas
				}
			}
		})).Should(Succeed())
	}

	changeStatefulSetReplicas := func(clusterName types.NamespacedName, replicas int32) {
		Eventually(testapps.GetAndChangeObj(&testCtx, clusterName, func(cluster *appsv1alpha1.Cluster) {
			if cluster.Spec.ComponentSpecs == nil || len(cluster.Spec.ComponentSpecs) == 0 {
				cluster.Spec.ComponentSpecs = []appsv1alpha1.ClusterComponentSpec{
					{
						Name:            mysqlCompName,
						ComponentDefRef: mysqlCompType,
						Replicas:        replicas,
					}}
			} else {
				cluster.Spec.ComponentSpecs[0].Replicas = replicas
			}
		})).Should(Succeed())
	}

	testChangeReplicas := func() {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster initialized")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		replicasSeq := []int32{5, 3, 1, 0, 2, 4}
		expectedOG := int64(1)
		for _, replicas := range replicasSeq {
			By(fmt.Sprintf("Change replicas to %d", replicas))
			changeStatefulSetReplicas(clusterKey, replicas)
			expectedOG++

			By("Checking cluster status and the number of replicas changed")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
				g.Expect(fetched.Status.ObservedGeneration).To(BeEquivalentTo(expectedOG))
			})).Should(Succeed())
			stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
			Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(replicas))
		}
	}

	testChangeReplicasInvalidValue := func() {
		By("Creating a cluster")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompType).SetReplicas(1).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster initialized")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		invalidReplicas := int32(-1)
		By(fmt.Sprintf("Change replicas to %d", invalidReplicas))
		changeStatefulSetReplicas(clusterKey, invalidReplicas)

		By("Checking cluster status and the number of replicas unchanged")
		Consistently(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
			g.Expect(fetched.Status.ObservedGeneration).To(BeEquivalentTo(1))
		})).Should(Succeed())
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(1))
	}

	getPVCName := func(compName string, i int) string {
		return fmt.Sprintf("%s-%s-%s-%d", testapps.DataVolumeName, clusterKey.Name, compName, i)
	}

	createPVC := func(clusterName, pvcName, compName string) {
		testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName,
			compName, "data").SetStorage("1Gi").CheckedCreate(&testCtx)
	}

	horizontalScaleComp := func(updatedReplicas int, comp *appsv1alpha1.ClusterComponentSpec) {
		By("Mocking components' PVCs to bound")
		for i := 0; i < int(comp.Replicas); i++ {
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(comp.Name, i),
			}
			createPVC(clusterKey.Name, pvcKey.Name, comp.Name)
			Eventually(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
			Eventually(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Status.Phase = corev1.ClaimBound
			})).Should(Succeed())
		}

		By("Checking sts replicas right")
		stsList := testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, clusterKey, comp.Name)
		Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(comp.Replicas))

		By(fmt.Sprintf("Changing replicas to %d", updatedReplicas))
		changeCompReplicas(clusterKey, int32(updatedReplicas), comp)

		By("Checking Backup created")
		Eventually(testapps.GetListLen(&testCtx, intctrlutil.BackupSignature,
			client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey:  clusterKey.Name,
				intctrlutil.AppComponentLabelKey: comp.Name,
			}, client.InNamespace(clusterKey.Namespace))).Should(Equal(1))

		By("Mocking VolumeSnapshot and set it as ReadyToUse")
		snapshotKey := types.NamespacedName{Name: fmt.Sprintf("%s-%s-scaling",
			clusterKey.Name, comp.Name),
			Namespace: testCtx.DefaultNamespace}
		pvcName := getPVCName(comp.Name, 0)
		volumeSnapshot := &snapshotv1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      snapshotKey.Name,
				Namespace: snapshotKey.Namespace,
				Labels: map[string]string{
					intctrlutil.AppCreatedByLabelKey: intctrlutil.AppName,
					intctrlutil.AppInstanceLabelKey:  clusterKey.Name,
					intctrlutil.AppComponentLabelKey: comp.Name,
				}},
			Spec: snapshotv1.VolumeSnapshotSpec{
				Source: snapshotv1.VolumeSnapshotSource{
					PersistentVolumeClaimName: &pvcName,
				},
			},
		}
		Expect(testCtx.CreateObj(testCtx.Ctx, volumeSnapshot)).Should(Succeed())
		readyToUse := true
		volumeSnapshotStatus := snapshotv1.VolumeSnapshotStatus{ReadyToUse: &readyToUse}
		volumeSnapshot.Status = &volumeSnapshotStatus
		Expect(k8sClient.Status().Update(testCtx.Ctx, volumeSnapshot)).Should(Succeed())

		By("Mock PVCs status to bound")
		for i := 0; i < updatedReplicas; i++ {
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(comp.Name, i),
			}
			Eventually(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
			Eventually(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
				pvc.Status.Phase = corev1.ClaimBound
			})).Should(Succeed())
		}

		By("Check backup job cleanup")
		Eventually(testapps.GetListLen(&testCtx, intctrlutil.BackupSignature,
			client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey:  clusterKey.Name,
				intctrlutil.AppComponentLabelKey: comp.Name,
			}, client.InNamespace(clusterKey.Namespace))).Should(Equal(0))
		Eventually(testapps.CheckObjExists(&testCtx, snapshotKey, &snapshotv1.VolumeSnapshot{}, false)).Should(Succeed())

		By("Checking updated sts replicas")
		stsList = testk8s.ListAndCheckStatefulSetWithComponent(&testCtx, clusterKey, comp.Name)
		Expect(*stsList.Items[0].Spec.Replicas).To(BeEquivalentTo(updatedReplicas))
	}

	horizontalScale := func(updatedReplicas int) {

		cluster := &appsv1alpha1.Cluster{}
		Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(Succeed())
		initialGeneration := int(cluster.Status.ObservedGeneration)

		By("Set HorizontalScalePolicy")
		Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
			func(clusterDef *appsv1alpha1.ClusterDefinition) {
				clusterDef.Spec.ComponentDefs[0].HorizontalScalePolicy =
					&appsv1alpha1.HorizontalScalePolicy{Type: appsv1alpha1.HScaleDataClonePolicyFromSnapshot}
			})).Should(Succeed())

		By("Creating a BackupPolicyTemplate")
		backupPolicyTplKey := types.NamespacedName{Name: "test-backup-policy-template-mysql"}
		backupPolicyTpl := &dataprotectionv1alpha1.BackupPolicyTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name: backupPolicyTplKey.Name,
				Labels: map[string]string{
					clusterDefLabelKey: clusterDefObj.Name,
				},
			},
			Spec: dataprotectionv1alpha1.BackupPolicyTemplateSpec{
				BackupToolName: "mysql-xtrabackup",
			},
		}
		Expect(testCtx.CreateObj(testCtx.Ctx, backupPolicyTpl)).Should(Succeed())

		for i := range clusterObj.Spec.ComponentSpecs {
			horizontalScaleComp(updatedReplicas, &clusterObj.Spec.ComponentSpecs[i])
		}

		By("Checking cluster status and the number of replicas changed")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(initialGeneration + len(clusterObj.Spec.ComponentSpecs)))
	}

	testHorizontalScale := func() {
		initialReplicas := int32(1)
		updatedReplicas := int32(3)

		secondMysqlCompName := mysqlCompName + "1"

		By("Creating a multi components cluster with VolumeClaimTemplate")
		pvcSpec := testapps.NewPVC("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompType).
			AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
			SetReplicas(initialReplicas).
			AddComponent(secondMysqlCompName, mysqlCompType).
			AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
			SetReplicas(initialReplicas).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		horizontalScale(int(updatedReplicas))
	}

	testVerticalScale := func() {
		const storageClassName = "sc-mock"
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
		pvcSpec := testapps.NewPVC("1Gi")
		pvcSpec.StorageClassName = &storageClass.Name

		By("Create cluster and waiting for the cluster initialized")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompType).
			AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
			SetReplicas(replicas).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Checking the replicas")
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		sts := &stsList.Items[0]
		Expect(*sts.Spec.Replicas == replicas).Should(BeTrue())

		By("Mock PVCs in Bound Status")
		for i := 0; i < replicas; i++ {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getPVCName(mysqlCompName, i),
					Namespace: clusterKey.Namespace,
					Labels: map[string]string{
						intctrlutil.AppInstanceLabelKey: clusterKey.Name,
					}},
				Spec: pvcSpec,
			}
			Expect(testCtx.CreateObj(testCtx.Ctx, pvc)).Should(Succeed())
			pvc.Status.Phase = corev1.ClaimBound // only bound pvc allows resize
			Expect(k8sClient.Status().Update(testCtx.Ctx, pvc)).Should(Succeed())
		}

		By("Updating the PVC storage size")
		newStorageValue := resource.MustParse("2Gi")
		Eventually(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
			comp := &cluster.Spec.ComponentSpecs[0]
			comp.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue
		})).Should(Succeed())

		By("Checking the resize operation finished")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))

		By("Checking PVCs are resized")
		stsList = testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		sts = &stsList.Items[0]
		for i := *sts.Spec.Replicas - 1; i >= 0; i-- {
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      getPVCName(mysqlCompName, int(i)),
			}
			Expect(k8sClient.Get(testCtx.Ctx, pvcKey, pvc)).Should(Succeed())
			Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(newStorageValue))
		}
	}

	testClusterAffinity := func() {
		const topologyKey = "testTopologyKey"
		const lableKey = "testNodeLabelKey"
		const labelValue = "testLabelValue"

		By("Creating a cluster with Affinity")
		affinity := &appsv1alpha1.Affinity{
			PodAntiAffinity: appsv1alpha1.Required,
			TopologyKeys:    []string{topologyKey},
			NodeLabels: map[string]string{
				lableKey: labelValue,
			},
			Tenancy: appsv1alpha1.SharedNode,
		}

		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(mysqlCompName, mysqlCompType).SetReplicas(3).
			WithRandomName().SetClusterAffinity(affinity).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster initialized")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Checking the Affinity and TopologySpreadConstraints")
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		podSpec := stsList.Items[0].Spec.Template.Spec
		Expect(podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).To(Equal(lableKey))
		Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable).To(Equal(corev1.DoNotSchedule))
		Expect(podSpec.TopologySpreadConstraints[0].TopologyKey).To(Equal(topologyKey))
		Expect(len(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution)).To(Equal(1))
		Expect(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey).To(Equal(topologyKey))
	}

	testComponentAffinity := func() {
		const clusterTopologyKey = "testClusterTopologyKey"
		const compTopologyKey = "testComponentTopologyKey"

		By("Creating a cluster with Affinity")
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
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().SetClusterAffinity(affinity).
			AddComponent(mysqlCompName, mysqlCompType).SetComponentAffinity(compAffinity).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster initialized")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Checking the Affinity and the TopologySpreadConstraints")
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		podSpec := stsList.Items[0].Spec.Template.Spec
		Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable).To(Equal(corev1.ScheduleAnyway))
		Expect(podSpec.TopologySpreadConstraints[0].TopologyKey).To(Equal(compTopologyKey))
		Expect(podSpec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight).ShouldNot(BeNil())
		Expect(len(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution)).To(Equal(1))
		Expect(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey).To(Equal(corev1.LabelHostname))
	}

	testClusterToleration := func() {
		const tolerationKey = "testClusterTolerationKey"
		const tolerationValue = "testClusterTolerationValue"
		By("Creating a cluster with Toleration")
		toleration := corev1.Toleration{
			Key:      tolerationKey,
			Value:    tolerationValue,
			Operator: corev1.TolerationOpEqual,
			Effect:   corev1.TaintEffectNoSchedule,
		}
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompType).SetReplicas(1).
			AddClusterToleration(toleration).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster initialized")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Checking the tolerations")
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		podSpec := stsList.Items[0].Spec.Template.Spec
		Expect(len(podSpec.Tolerations) == 1).Should(BeTrue())
		toleration = podSpec.Tolerations[0]
		Expect(toleration.Key == tolerationKey &&
			toleration.Value == tolerationValue).Should(BeTrue())
		Expect(toleration.Operator == corev1.TolerationOpEqual &&
			toleration.Effect == corev1.TaintEffectNoSchedule).Should(BeTrue())
	}

	testComponentToleration := func() {
		clusterTolerationKey := "testClusterTolerationKey"
		compTolerationKey := "testcompTolerationKey"
		compTolerationValue := "testcompTolerationValue"

		By("Creating a cluster with Toleration")
		toleration := corev1.Toleration{
			Key:      clusterTolerationKey,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoExecute,
		}
		compToleration := corev1.Toleration{
			Key:      compTolerationKey,
			Value:    compTolerationValue,
			Operator: corev1.TolerationOpEqual,
			Effect:   corev1.TaintEffectNoSchedule,
		}
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().AddClusterToleration(toleration).
			AddComponent(mysqlCompName, mysqlCompType).AddComponentToleration(compToleration).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for the cluster initialized")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Checking the tolerations")
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		podSpec := stsList.Items[0].Spec.Template.Spec
		Expect(len(podSpec.Tolerations) == 1).Should(BeTrue())
		toleration = podSpec.Tolerations[0]
		Expect(toleration.Key == compTolerationKey &&
			toleration.Value == compTolerationValue).Should(BeTrue())
		Expect(toleration.Operator == corev1.TolerationOpEqual &&
			toleration.Effect == corev1.TaintEffectNoSchedule).Should(BeTrue())
	}

	mockPodsForConsensusTest := func(cluster *appsv1alpha1.Cluster, number int) []corev1.Pod {
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
						intctrlutil.AppInstanceLabelKey:       clusterName,
						intctrlutil.AppComponentLabelKey:      componentName,
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
				Reason:  "Unhealthy",
				Message: `Readiness probe failed: {"event":"roleUnchanged","originalRole":"Leader","role":"Follower"}`,
				InvolvedObject: corev1.ObjectReference{
					Name:      pod.Name,
					Namespace: testCtx.DefaultNamespace,
					UID:       pod.UID,
					FieldPath: "spec.containers{kb-rolechangedcheck}",
				},
			}
			events = append(events, event)
		}
		events[0].Message = `Readiness probe failed: {"event":"roleUnchanged","originalRole":"Leader","role":"Leader"}`
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

	testThreeReplicas := func() {
		const replicas = 3

		By("Mock a cluster obj")
		pvcSpec := &corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		}
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompType).
			SetReplicas(replicas).AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for cluster creation")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		sts := &stsList.Items[0]

		By("Creating mock pods in StatefulSet")
		pods := mockPodsForConsensusTest(clusterObj, replicas)
		for _, pod := range pods {
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
			g.Expect(err).To(Succeed())
			// should have 3 pods
			g.Expect(len(pods)).To(Equal(3))
			// 1 leader
			// 2 followers
			leaderCount, followerCount := 0, 0
			for _, pod := range pods {
				switch pod.Labels[intctrlutil.RoleLabelKey] {
				case leader:
					leaderCount++
				case follower:
					followerCount++
				}
			}
			g.Expect(leaderCount).Should(Equal(1))
			g.Expect(followerCount).Should(Equal(2))
		}).Should(Succeed())

		By("Updating StatefulSet's status")
		sts.Status.UpdateRevision = "mock-version"
		sts.Status.Replicas = int32(replicas)
		sts.Status.AvailableReplicas = int32(replicas)
		sts.Status.CurrentReplicas = int32(replicas)
		sts.Status.ReadyReplicas = int32(replicas)
		sts.Status.ObservedGeneration = sts.Generation
		Expect(k8sClient.Status().Update(ctx, sts)).Should(Succeed())

		By("Checking pods' role are updated in cluster status")
		Eventually(func(g Gomega) {
			fetched := &appsv1alpha1.Cluster{}
			g.Expect(k8sClient.Get(ctx, clusterKey, fetched)).To(Succeed())
			compName := fetched.Spec.ComponentSpecs[0].Name
			g.Expect(fetched.Status.Components != nil).To(BeTrue())
			g.Expect(fetched.Status.Components).To(HaveKey(compName))
			consensusStatus := fetched.Status.Components[compName].ConsensusSetStatus
			g.Expect(consensusStatus != nil).To(BeTrue())
			g.Expect(consensusStatus.Leader.Pod).To(BeElementOf(getStsPodsName(sts)))
			g.Expect(len(consensusStatus.Followers) == 2).To(BeTrue())
			g.Expect(consensusStatus.Followers[0].Pod).To(BeElementOf(getStsPodsName(sts)))
			g.Expect(consensusStatus.Followers[1].Pod).To(BeElementOf(getStsPodsName(sts)))
		}).Should(Succeed())

		By("Waiting the cluster be running")
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningPhase))
	}

	mockPodsForReplicationTest := func(cluster *appsv1alpha1.Cluster, stsList []appsv1.StatefulSet) []corev1.Pod {
		componentName := cluster.Spec.ComponentSpecs[0].Name
		clusterName := cluster.Name
		pods := make([]corev1.Pod, 0)
		for _, sts := range stsList {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sts.Name + "-0",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						intctrlutil.RoleLabelKey:              sts.Labels[intctrlutil.RoleLabelKey],
						intctrlutil.AppInstanceLabelKey:       clusterName,
						intctrlutil.AppComponentLabelKey:      componentName,
						appsv1.ControllerRevisionHashLabelKey: sts.Status.UpdateRevision,
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

	getReplicationSetStsPodsName := func(stsList []appsv1.StatefulSet) []string {
		names := make([]string, 0)
		for _, sts := range stsList {
			pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, &sts)
			Expect(err).To(Succeed())
			Expect(len(pods)).To(BeEquivalentTo(1))
			names = append(names, pods[0].Name)
		}
		return names
	}

	testReplicationCreation := func() {
		By("Mock a cluster obj with replication componentDefRef.")
		pvcSpec := testapps.NewPVC("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompType).
			SetPrimaryIndex(testapps.DefaultReplicationPrimaryIndex).
			SetReplicas(testapps.DefaultReplicationReplicas).
			AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for cluster creation")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(0))

		By("Checking statefulSet number")
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		Expect(len(stsList.Items)).Should(BeEquivalentTo(2))

		By("Checking statefulSet role label")
		for _, sts := range stsList.Items {
			if strings.HasSuffix(sts.Name, strconv.Itoa(testapps.DefaultReplicationPrimaryIndex)) {
				Expect(sts.Labels[intctrlutil.RoleLabelKey]).Should(BeEquivalentTo(replicationset.Primary))
			} else {
				Expect(sts.Labels[intctrlutil.RoleLabelKey]).Should(BeEquivalentTo(replicationset.Secondary))
			}
		}

		By("Updating StatefulSet's status")
		status := appsv1.StatefulSetStatus{
			AvailableReplicas:  1,
			ObservedGeneration: 1,
			Replicas:           1,
			ReadyReplicas:      1,
			UpdatedReplicas:    1,
			CurrentRevision:    "mock-revision",
			UpdateRevision:     "mock-revision",
		}
		for _, sts := range stsList.Items {
			status.ObservedGeneration = sts.Generation
			testk8s.PatchStatefulSetStatus(&testCtx, sts.Name, status)
		}

		By("Creating mock pods in StatefulSet")
		stsList = testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		pods := mockPodsForReplicationTest(clusterObj, stsList.Items)
		for _, pod := range pods {
			Expect(testCtx.CreateObj(testCtx.Ctx, &pod)).Should(Succeed())
			pod.Status.Conditions = []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}}
			Expect(k8sClient.Status().Update(ctx, &pod)).Should(Succeed())
		}

		By("Checking pods' role are updated in cluster status")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
			compName := fetched.Spec.ComponentSpecs[0].Name
			g.Expect(fetched.Status.Components).NotTo(BeNil())
			g.Expect(fetched.Status.Components).To(HaveKey(compName))
			replicationStatus := fetched.Status.Components[compName].ReplicationSetStatus
			g.Expect(replicationStatus).NotTo(BeNil())
			g.Expect(replicationStatus.Primary.Pod).To(BeElementOf(getReplicationSetStsPodsName(stsList.Items)))
			g.Expect(len(replicationStatus.Secondaries)).To(BeEquivalentTo(1))
			g.Expect(replicationStatus.Secondaries[0].Pod).To(BeElementOf(getReplicationSetStsPodsName(stsList.Items)))
		})).Should(Succeed())

	}

	// Scenarios

	Context("when creating cluster with multiple kinds of components", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatefulMySQLComponent, mysqlCompType).
				AddComponent(testapps.StatelessNginxComponent, nginxCompType).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(mysqlCompType).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				AddComponent(nginxCompType).AddContainerShort("nginx", testapps.NginxImage).
				Create(&testCtx).GetObject()
		})

		It("should create all sub-resources successfully", func() {
			checkAllResourcesCreated()
		})

		It("should create corresponding services correctly", func() {
			checkAllServicesCreate()
		})
	})

	Context("when creating cluster with MySQL as stateful component", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatefulMySQLComponent, mysqlCompType).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(mysqlCompType).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()
		})

		It("should delete cluster resources immediately if deleting cluster with WipeOut termination policy", func() {
			testWipeOut()
		})

		It("should not terminate immediately if deleting cluster with DoNotTerminate termination policy", func() {
			testDoNotTermintate()
		})

		It("should create/delete pods to match the desired replica number if updating cluster's replica number to a valid value", func() {
			testChangeReplicas()
		})

		It("should fail if updating cluster's replica number to an invalid value", func() {
			testChangeReplicasInvalidValue()
		})

		Context("and with cluster affinity set", func() {
			It("should create pod with cluster affinity", func() {
				testClusterAffinity()
			})
		})

		Context("and with both cluster affinity and component affinity set", func() {
			It("Should observe the component affinity will override the cluster affinity", func() {
				testComponentAffinity()
			})
		})

		Context("and with cluster tolerations set", func() {
			It("Should create pods with cluster tolerations", func() {
				testClusterToleration()
			})
		})

		Context("and with both cluster tolerations and component tolerations set", func() {
			It("Should observe the component tolerations will override the cluster tolerations", func() {
				testComponentToleration()
			})
		})

		Context("with pvc", func() {
			It("should trigger a backup process(snapshot) and create pvcs from backup for newly created replicas when horizontal scale the cluster from 1 to 3", func() {
				testHorizontalScale()
			})
		})

		Context("with pvc and dynamic-provisioning storage class", func() {
			It("should update PVC request storage size accordingly when vertical scale the cluster", func() {
				testVerticalScale()
			})
		})
	})

	Context("when creating cluster with MySQL as consensus component", func() {
		BeforeEach(func() {
			By("Create a clusterDef obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.ConsensusMySQLComponent, mysqlCompType).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(mysqlCompType).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()
		})

		It("Should success with one leader pod and two follower pods", func() {
			testThreeReplicas()
		})

		It("should create/delete pods to match the desired replica number if updating cluster's replica number to a valid value", func() {
			testChangeReplicas()
		})

		It("should fail if updating cluster's replica number to an invalid value", func() {
			testChangeReplicasInvalidValue()
		})

		Context("with pvc", func() {
			It("should trigger a backup process(snapshot) and create pvcs from backup for newly created replicas when horizontal scale the cluster from 1 to 3", func() {
				testHorizontalScale()
			})
		})

		Context("with pvc and dynamic-provisioning storage class", func() {
			It("should update PVC request storage size accordingly when vertical scale the cluster", func() {
				testVerticalScale()
			})
		})

		Context("with horizontalScale after verticalScale", func() {
			It("should succeed", func() {
				testVerticalScale()
				horizontalScale(5)
			})
		})
	})

	Context("when creating cluster with Redis as replication component", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj with replication componentDefRef.")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompType).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication componentDefRef.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponent(testapps.DefaultRedisCompType).
				AddContainerShort(testapps.DefaultRedisContainerName, testapps.DefaultRedisImageName).
				Create(&testCtx).GetObject()
		})

		It("Should success with primary sts and secondary sts", func() {
			testReplicationCreation()
		})
	})
})
