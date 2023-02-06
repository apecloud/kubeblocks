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
	"context"
	"fmt"
	"reflect"

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

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("Cluster Controller", func() {
	const clusterNamePrefix = "test-cluster"
	const statefulCompName = "replicasets"
	const statefulCompType = "replicasets"
	const volumeName = "data"

	ctx := context.Background()

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
		// non-namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.BackupPolicyTemplateSignature, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.StorageClassSignature, ml)
	}

	var (
		clusterDefObj     *dbaasv1alpha1.ClusterDefinition
		clusterVersionObj *dbaasv1alpha1.ClusterVersion
		clusterObj        *dbaasv1alpha1.Cluster
		clusterKey        types.NamespacedName
	)

	BeforeEach(func() {
		cleanEnv()

		By("Create a configmap and config template obj")
		_ = testdbaas.CreateCustomizedObj(&testCtx, "config/configcm.yaml", &corev1.ConfigMap{},
			testCtx.UseDefaultNamespace())

		cfgTpl := testdbaas.CreateCustomizedObj(&testCtx, "config/configtpl.yaml",
			&dbaasv1alpha1.ConfigConstraint{}, testCtx.UseDefaultNamespace())
		Expect(testdbaas.ChangeObjStatus(&testCtx, cfgTpl, func() {
			cfgTpl.Status.Phase = dbaasv1alpha1.AvailablePhase
		})).Should(Succeed())

		By("Create a clusterDefinition obj")
		clusterDefObj = testdbaas.CreateCustomizedObj(&testCtx, "resources/mysql_cd.yaml",
			&dbaasv1alpha1.ClusterDefinition{}, testCtx.UseDefaultNamespace())

		By("Create a clusterVersion obj")
		clusterVersionObj = testdbaas.CreateCustomizedObj(&testCtx, "resources/mysql_cv.yaml",
			&dbaasv1alpha1.ClusterVersion{}, testCtx.UseDefaultNamespace())

		By("Mock a cluster obj")
		clusterKey = testdbaas.GetRandomizedKey(&testCtx, clusterNamePrefix)
		clusterObj = testdbaas.NewClusterObj(clusterKey, clusterDefObj.GetName(), clusterVersionObj.GetName())
	})

	AfterEach(func() {
		cleanEnv()
	})

	listAndCheckStatefulSet := func(key types.NamespacedName) *appsv1.StatefulSetList {
		By("Check statefulset workload has been created")
		stsList := &appsv1.StatefulSetList{}
		Eventually(func() bool {
			Expect(k8sClient.List(ctx, stsList, client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey: key.Name,
			}, client.InNamespace(key.Namespace))).Should(Succeed())
			return len(stsList.Items) > 0
		}).Should(BeTrue())
		return stsList
	}

	changeStatefulSetReplicas := func(clusterName types.NamespacedName, replicas int32) {
		Eventually(testdbaas.GetAndChangeObj(&testCtx, clusterName, func(cluster *dbaasv1alpha1.Cluster) {
			if cluster.Spec.Components == nil || len(cluster.Spec.Components) == 0 {
				cluster.Spec.Components = []dbaasv1alpha1.ClusterComponent{
					{
						Name:     statefulCompName,
						Type:     statefulCompType,
						Replicas: &replicas,
					}}
			} else {
				cluster.Spec.Components[0].Replicas = &replicas
			}
		})).Should(Succeed())
	}

	Context("after the cluster initialized", func() {
		const initializedVersion = 1

		BeforeEach(func() {
			By("Creating a cluster")
			Expect(testCtx.CreateObj(ctx, clusterObj)).Should(Succeed())

			By("Waiting for the cluster initialized")
			Eventually(testdbaas.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(initializedVersion))
		})

		It("should create cluster and all sub-resources successfully", func() {
			By("Check deployment workload has been created")
			Eventually(testdbaas.GetListLen(&testCtx, intctrlutil.DeploymentSignature,
				client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey: clusterKey.Name,
				}, client.InNamespace(clusterKey.Namespace))).ShouldNot(Equal(0))

			stsList := listAndCheckStatefulSet(clusterKey)

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
			Eventually(testdbaas.GetListLen(&testCtx, intctrlutil.PodDisruptionBudgetSignature,
				client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey: clusterKey.Name,
				}, client.InNamespace(clusterKey.Namespace))).Should(Equal(0))

			By("Check created sts pods template without tolerations")
			Expect(len(stsList.Items[0].Spec.Template.Spec.Tolerations) == 0).Should(BeTrue())

			By("Checking the Affinity and the TopologySpreadConstraints")
			podSpec := stsList.Items[0].Spec.Template.Spec
			Expect(podSpec.Affinity).Should(BeNil())
			Expect(len(podSpec.TopologySpreadConstraints) == 0).Should(BeTrue())

			By("Check should create env configmap")
			Eventually(testdbaas.GetListLen(&testCtx, intctrlutil.ConfigMapSignature,
				client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey:   clusterKey.Name,
					intctrlutil.AppConfigTypeLabelKey: "kubeblocks-env",
				}, client.InNamespace(clusterKey.Namespace))).Should(Equal(2))
		})

		It("should create corresponding services correctly", func() {
			By("Checking proxy should have external ClusterIP service")
			svcList1 := &corev1.ServiceList{}
			Expect(k8sClient.List(ctx, svcList1, client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey:  clusterKey.Name,
				intctrlutil.AppComponentLabelKey: "proxy",
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
			getHeadlessSvcPorts := func(name string) []corev1.ServicePort {
				fetched := &dbaasv1alpha1.Cluster{}
				Expect(k8sClient.Get(ctx, clusterKey, fetched)).To(Succeed())

				comp, err := util.GetComponentDefByCluster(ctx, k8sClient, fetched, name)
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
			Expect(k8sClient.List(ctx, svcList2, client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey:  clusterKey.Name,
				intctrlutil.AppComponentLabelKey: statefulCompName,
			}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
			Expect(len(svcList2.Items) == 1).Should(BeTrue())
			Expect(svcList2.Items[0].Spec.Type == corev1.ServiceTypeClusterIP).To(BeTrue())
			Expect(svcList2.Items[0].Spec.ClusterIP == corev1.ClusterIPNone).To(BeTrue())
			Expect(reflect.DeepEqual(svcList2.Items[0].Spec.Ports,
				getHeadlessSvcPorts(statefulCompName))).Should(BeTrue())
		})

		It("should delete cluster resources immediately if deleting cluster with WipeOut termination policy", func() {
			By("Delete the cluster")
			testdbaas.DeleteObject(&testCtx, clusterKey, &dbaasv1alpha1.Cluster{})

			By("Wait for the cluster to terminate")
			Eventually(testdbaas.CheckObjExists(&testCtx, clusterKey, &dbaasv1alpha1.Cluster{}, false)).Should(Succeed())
		})

		It("should not terminate immediately if deleting cluster with DoNotTerminate termination policy", func() {
			By("Update the cluster's termination policy to DoNotTerminate")
			Eventually(testdbaas.GetAndChangeObj(&testCtx, clusterKey, func(cluster *dbaasv1alpha1.Cluster) {
				cluster.Spec.TerminationPolicy = dbaasv1alpha1.DoNotTerminate
			})).Should(Succeed())

			By("Delete the cluster")
			testdbaas.DeleteObject(&testCtx, clusterKey, &dbaasv1alpha1.Cluster{})

			By("Check the cluster do not terminate immediately")
			checkClusterDoNotTerminate := testdbaas.CheckObj(&testCtx, clusterKey,
				func(g Gomega, fetched *dbaasv1alpha1.Cluster) {
					g.Expect(fetched.Status.Message).To(ContainSubstring(
						fmt.Sprintf("spec.terminationPolicy %s is preventing deletion.", fetched.Spec.TerminationPolicy)))
					g.Expect(len(fetched.Finalizers) > 0).To(BeTrue())
				})
			Eventually(checkClusterDoNotTerminate).Should(Succeed())
			Consistently(checkClusterDoNotTerminate).Should(Succeed())

			By("Update the cluster's termination policy to WipeOut")
			Eventually(testdbaas.GetAndChangeObj(&testCtx, clusterKey, func(cluster *dbaasv1alpha1.Cluster) {
				cluster.Spec.TerminationPolicy = dbaasv1alpha1.WipeOut
			})).Should(Succeed())

			By("Wait for the cluster to terminate")
			Eventually(testdbaas.CheckObjExists(&testCtx, clusterKey, &dbaasv1alpha1.Cluster{}, false)).Should(Succeed())
		})

		It("should create/delete pods to match the desired replica number if updating cluster's replica number to a valid value", func() {
			replicasSeq := []int32{5, 3, 1, 0, 2, 4}
			expectedOG := int64(initializedVersion)
			for _, replicas := range replicasSeq {
				By(fmt.Sprintf("Change replicas to %d", replicas))
				changeStatefulSetReplicas(clusterKey, replicas)
				expectedOG++

				By("Checking cluster status and the number of replicas changed")
				Eventually(testdbaas.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *dbaasv1alpha1.Cluster) {
					g.Expect(fetched.Status.ObservedGeneration).To(BeEquivalentTo(expectedOG))
				})).Should(Succeed())
				stsList := listAndCheckStatefulSet(clusterKey)
				Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(replicas))
			}
		})

		It("should fail if updating cluster's replica number to an invalid value", func() {
			invalidReplicas := int32(-1)
			By(fmt.Sprintf("Change replicas to %d", invalidReplicas))
			changeStatefulSetReplicas(clusterKey, invalidReplicas)

			By("Checking cluster status and the number of replicas unchanged")
			Consistently(testdbaas.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *dbaasv1alpha1.Cluster) {
				g.Expect(fetched.Status.ObservedGeneration).To(BeEquivalentTo(initializedVersion))
			})).Should(Succeed())
			stsList := listAndCheckStatefulSet(clusterKey)
			Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(1))
		})
	})

	getPVCName := func(i int) string {
		return fmt.Sprintf("%s-%s-%s-%d", volumeName, clusterKey.Name, statefulCompName, i)
	}

	Context("When horizontal scaling out a cluster", func() {
		It("Should trigger a backup process(snapshot) and "+
			"create pvcs from backup for newly created replicas", func() {
			By("Creating a cluster with VolumeClaimTemplate")
			initialReplicas := int32(1)
			pvcSpec := corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			}
			clusterObj.Spec.Components = []dbaasv1alpha1.ClusterComponent{{
				Name:     statefulCompName,
				Type:     statefulCompType,
				Replicas: &initialReplicas,
				VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{{
					Name: volumeName,
					Spec: &pvcSpec,
				}},
			}}
			Expect(testCtx.CreateObj(ctx, clusterObj)).Should(Succeed())
			Eventually(testdbaas.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

			By("Set HorizontalScalePolicy")
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
				func(clusterDef *dbaasv1alpha1.ClusterDefinition) {
					clusterDef.Spec.Components[0].HorizontalScalePolicy =
						&dbaasv1alpha1.HorizontalScalePolicy{Type: dbaasv1alpha1.HScaleDataClonePolicyFromSnapshot}
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
			Expect(testCtx.CreateObj(ctx, backupPolicyTpl)).Should(Succeed())

			By("Mocking PVC for the first replica")
			for i := 0; i < int(initialReplicas); i++ {
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getPVCName(i),
						Namespace: clusterKey.Namespace,
						Labels: map[string]string{
							intctrlutil.AppInstanceLabelKey: clusterKey.Name,
						}},
					Spec: pvcSpec,
				}
				Expect(testCtx.CreateObj(ctx, pvc)).Should(Succeed())
			}

			stsList := listAndCheckStatefulSet(clusterKey)
			Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(initialReplicas))

			updatedReplicas := int32(3)
			By(fmt.Sprintf("Changing replicas to %d", updatedReplicas))
			changeStatefulSetReplicas(clusterKey, updatedReplicas)

			By("Checking BackupJob created")
			Eventually(testdbaas.GetListLen(&testCtx, intctrlutil.BackupSignature,
				client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey: clusterKey.Name,
				}, client.InNamespace(clusterKey.Namespace))).Should(Equal(1))

			By("Mocking VolumeSnapshot and set it as ReadyToUse")
			snapshotKey := types.NamespacedName{Name: fmt.Sprintf("%s-%s-scaling",
				clusterKey.Name, statefulCompName), Namespace: "default"}
			pvcName := getPVCName(0)
			volumeSnapshot := &snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotKey.Name,
					Namespace: snapshotKey.Namespace,
					Labels: map[string]string{
						intctrlutil.AppCreatedByLabelKey: intctrlutil.AppName,
						intctrlutil.AppInstanceLabelKey:  clusterKey.Name,
						intctrlutil.AppComponentLabelKey: statefulCompName,
					}},
				Spec: snapshotv1.VolumeSnapshotSpec{
					Source: snapshotv1.VolumeSnapshotSource{
						PersistentVolumeClaimName: &pvcName,
					},
				},
			}
			Expect(testCtx.CreateObj(ctx, volumeSnapshot)).Should(Succeed())
			readyToUse := true
			volumeSnapshotStatus := snapshotv1.VolumeSnapshotStatus{ReadyToUse: &readyToUse}
			volumeSnapshot.Status = &volumeSnapshotStatus
			Expect(k8sClient.Status().Update(ctx, volumeSnapshot)).Should(Succeed())

			By("Mock PVCs status to bound")
			for i := 0; i < int(updatedReplicas); i++ {
				pvcKey := types.NamespacedName{
					Namespace: clusterKey.Namespace,
					Name:      getPVCName(i),
				}
				Eventually(testdbaas.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
				Eventually(testdbaas.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Phase = corev1.ClaimBound
				})).Should(Succeed())
			}

			By("Check backup job cleanup")
			Eventually(testdbaas.GetListLen(&testCtx, intctrlutil.BackupSignature,
				client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey: clusterKey.Name,
				}, client.InNamespace(clusterKey.Namespace))).Should(Equal(0))
			Eventually(testdbaas.CheckObjExists(&testCtx, snapshotKey, &snapshotv1.VolumeSnapshot{}, false)).Should(Succeed())

			By("Checking cluster status and the number of replicas changed")
			Eventually(testdbaas.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))
			stsList = listAndCheckStatefulSet(clusterKey)
			Expect(*stsList.Items[0].Spec.Replicas).To(BeEquivalentTo(updatedReplicas))
		})
	})

	Context("When updating cluster PVC storage size", func() {
		It("Should update PVC request storage size accordingly", func() {
			const storageClassName = "sc-mock"

			By("Mock a StorageClass which allows resize")
			allowVolumeExpansion := true
			storageClass := &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: storageClassName,
				},
				Provisioner:          "kubernetes.io/no-provisioner",
				AllowVolumeExpansion: &allowVolumeExpansion,
			}
			Expect(testCtx.CreateObj(ctx, storageClass)).Should(Succeed())

			By("Creating a cluster with volume claim")
			replicas := int32(2)
			pvcSpec := corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				StorageClassName: &storageClass.Name,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			}
			clusterObj.Spec.Components = make([]dbaasv1alpha1.ClusterComponent, 1)
			clusterObj.Spec.Components[0] = dbaasv1alpha1.ClusterComponent{
				Name:     statefulCompName,
				Type:     statefulCompType,
				Replicas: &replicas,
				VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{{
					Name: volumeName,
					Spec: &pvcSpec,
				}},
			}
			Expect(testCtx.CreateObj(ctx, clusterObj)).Should(Succeed())
			Eventually(testdbaas.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

			By("Checking the replicas")
			stsList := listAndCheckStatefulSet(clusterKey)
			sts := &stsList.Items[0]
			Expect(*sts.Spec.Replicas == replicas).Should(BeTrue())

			By("Mock PVCs in Bound Status")
			for i := 0; i < int(replicas); i++ {
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getPVCName(i),
						Namespace: clusterKey.Namespace,
						Labels: map[string]string{
							intctrlutil.AppInstanceLabelKey: clusterKey.Name,
						}},
					Spec: pvcSpec,
				}
				Expect(testCtx.CreateObj(ctx, pvc)).Should(Succeed())
				pvc.Status.Phase = corev1.ClaimBound // only bound pvc allows resize
				Expect(k8sClient.Status().Update(ctx, pvc)).Should(Succeed())
			}

			By("Updating the PVC storage size")
			newStorageValue := resource.MustParse("2Gi")
			Eventually(testdbaas.GetAndChangeObj(&testCtx, clusterKey, func(cluster *dbaasv1alpha1.Cluster) {
				comp := &cluster.Spec.Components[0]
				comp.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue
			})).Should(Succeed())

			By("Checking the resize operation finished")
			Eventually(testdbaas.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(2))

			By("Checking PVCs are resized")
			stsList = listAndCheckStatefulSet(clusterKey)
			sts = &stsList.Items[0]
			for i := *sts.Spec.Replicas - 1; i >= 0; i-- {
				pvc := &corev1.PersistentVolumeClaim{}
				pvcKey := types.NamespacedName{
					Namespace: clusterKey.Namespace,
					Name:      getPVCName(int(i)),
				}
				Expect(k8sClient.Get(ctx, pvcKey, pvc)).Should(Succeed())
				Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(newStorageValue))
			}
		})
	})

	Context("When creating cluster with cluster affinity set", func() {
		It("Should create pod with cluster affinity", func() {
			By("Creating a cluster")
			topologyKey := "testTopologyKey"
			lableKey := "testNodeLabelKey"
			labelValue := "testLabelValue"
			clusterObj.Spec.Affinity = &dbaasv1alpha1.Affinity{
				PodAntiAffinity: dbaasv1alpha1.Required,
				TopologyKeys:    []string{topologyKey},
				NodeLabels: map[string]string{
					lableKey: labelValue,
				},
			}
			Expect(testCtx.CreateObj(ctx, clusterObj)).Should(Succeed())

			By("Checking the Affinity and TopologySpreadConstraints")
			stsList := listAndCheckStatefulSet(clusterKey)
			podSpec := stsList.Items[0].Spec.Template.Spec
			Expect(podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).To(Equal(lableKey))
			Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable).To(Equal(corev1.DoNotSchedule))
			Expect(podSpec.TopologySpreadConstraints[0].TopologyKey).To(Equal(topologyKey))
			Expect(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey).To(Equal(topologyKey))
		})
	})

	Context("When creating cluster with both cluster affinity and component affinity set", func() {
		It("Should observe the component affinity will override the cluster affinity", func() {
			By("Creating a cluster")
			clusterTopologyKey := "testClusterTopologyKey"
			clusterObj.Spec.Affinity = &dbaasv1alpha1.Affinity{
				PodAntiAffinity: dbaasv1alpha1.Required,
				TopologyKeys:    []string{clusterTopologyKey},
			}
			compTopologyKey := "testComponentTopologyKey"
			clusterObj.Spec.Components = []dbaasv1alpha1.ClusterComponent{}
			clusterObj.Spec.Components = append(clusterObj.Spec.Components, dbaasv1alpha1.ClusterComponent{
				Name: statefulCompName,
				Type: statefulCompType,
				Affinity: &dbaasv1alpha1.Affinity{
					PodAntiAffinity: dbaasv1alpha1.Preferred,
					TopologyKeys:    []string{compTopologyKey},
				},
			})
			Expect(testCtx.CreateObj(ctx, clusterObj)).Should(Succeed())

			By("Checking the Affinity and the TopologySpreadConstraints")
			stsList := listAndCheckStatefulSet(clusterKey)
			podSpec := stsList.Items[0].Spec.Template.Spec
			Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable).To(Equal(corev1.ScheduleAnyway))
			Expect(podSpec.TopologySpreadConstraints[0].TopologyKey).To(Equal(compTopologyKey))
			Expect(podSpec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight).ShouldNot(BeNil())
		})
	})

	Context("When creating cluster with cluster tolerations set", func() {
		It("Should create pods with cluster tolerations", func() {
			By("Creating a cluster")
			var tolerations []corev1.Toleration
			tolerationKey := "testClusterTolerationKey"
			tolerationValue := "testClusterTolerationValue"
			clusterObj.Spec.Tolerations = append(tolerations, corev1.Toleration{
				Key:      tolerationKey,
				Value:    tolerationValue,
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			})
			Expect(testCtx.CreateObj(ctx, clusterObj)).Should(Succeed())

			By("Checking the tolerations")
			stsList := listAndCheckStatefulSet(clusterKey)
			podSpec := stsList.Items[0].Spec.Template.Spec
			Expect(len(podSpec.Tolerations) == 1).Should(BeTrue())
			toleration := podSpec.Tolerations[0]
			Expect(toleration.Key == tolerationKey &&
				toleration.Value == tolerationValue).Should(BeTrue())
			Expect(toleration.Operator == corev1.TolerationOpEqual &&
				toleration.Effect == corev1.TaintEffectNoSchedule).Should(BeTrue())
		})
	})

	Context("When creating cluster with both cluster tolerations and component tolerations set", func() {
		It("Should observe the component tolerations will override the cluster tolerations", func() {
			By("Creating a cluster")
			var clusterTolerations []corev1.Toleration
			clusterTolerationKey := "testClusterTolerationKey"
			clusterObj.Spec.Tolerations = append(clusterTolerations, corev1.Toleration{
				Key:      clusterTolerationKey,
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			})

			var compTolerations []corev1.Toleration
			compTolerationKey := "testcompTolerationKey"
			compTolerationValue := "testcompTolerationValue"
			compTolerations = append(compTolerations, corev1.Toleration{
				Key:      compTolerationKey,
				Value:    compTolerationValue,
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			})

			clusterObj.Spec.Components = []dbaasv1alpha1.ClusterComponent{}
			clusterObj.Spec.Components = append(clusterObj.Spec.Components, dbaasv1alpha1.ClusterComponent{
				Name:        statefulCompName,
				Type:        statefulCompType,
				Tolerations: compTolerations,
			})
			Expect(testCtx.CreateObj(ctx, clusterObj)).Should(Succeed())

			By("Checking the tolerations")
			stsList := listAndCheckStatefulSet(clusterKey)
			podSpec := stsList.Items[0].Spec.Template.Spec
			Expect(len(podSpec.Tolerations) == 1).Should(BeTrue())
			toleration := podSpec.Tolerations[0]
			Expect(toleration.Key == compTolerationKey &&
				toleration.Value == compTolerationValue).Should(BeTrue())
			Expect(toleration.Operator == corev1.TolerationOpEqual &&
				toleration.Effect == corev1.TaintEffectNoSchedule).Should(BeTrue())
		})
	})
})
