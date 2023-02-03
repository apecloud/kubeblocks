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
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/consensusset"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("Cluster Controller", func() {
	const timeout = time.Second * 10
	const interval = time.Second * 1
	const waitDuration = time.Second * 3

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
		clusterObj = testdbaas.MockClusterObj(clusterDefObj.GetName(), clusterVersionObj.GetName())
		clusterKey = client.ObjectKeyFromObject(clusterObj)
	})

	AfterEach(func() {
		cleanEnv()
	})

	assureCfgTplConfigMapObj := func() *corev1.ConfigMap {
		By("Assuring an cm obj")
		cfgCM := testdbaas.CreateCustomizedObj(&testCtx, "config/configcm.yaml", &corev1.ConfigMap{},
			testCtx.UseDefaultNamespace())
		cfgTpl := testdbaas.CreateCustomizedObj(&testCtx, "config/configtpl.yaml", &dbaasv1alpha1.ConfigConstraint{},
			testCtx.UseDefaultNamespace())

		Expect(testdbaas.ChangeObjStatus(&testCtx, cfgTpl, func() {
			cfgTpl.Status.Phase = dbaasv1alpha1.AvailablePhase
		})).Should(Succeed())
		return cfgCM
	}

	// Consensus associate objs
	// ClusterDefinition with componentType = Consensus
	assureClusterDefWithConsensusObj := func() *dbaasv1alpha1.ClusterDefinition {
		By("Assuring an clusterDefinition obj with componentType = Consensus")
		return testdbaas.CreateCustomizedObj(&testCtx, "resources/mysql_cd_consensusset.yaml", &dbaasv1alpha1.ClusterDefinition{},
			testCtx.UseDefaultNamespace())
	}

	assureClusterVersionWithConsensusObj := func() *dbaasv1alpha1.ClusterVersion {
		By("Assuring an clusterVersion obj with componentType = Consensus")
		return testdbaas.CreateCustomizedObj(&testCtx, "resources/mysql_cv_consensusset.yaml", &dbaasv1alpha1.ClusterVersion{},
			testCtx.UseDefaultNamespace())
	}

	newClusterWithConsensusObj := func(
		clusterDefObj *dbaasv1alpha1.ClusterDefinition,
		clusterVersionObj *dbaasv1alpha1.ClusterVersion,
	) (*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, types.NamespacedName) {
		// setup Cluster obj required default ClusterDefinition and ClusterVersion objects if not provided
		if clusterDefObj == nil {
			assureCfgTplConfigMapObj()
			clusterDefObj = assureClusterDefWithConsensusObj()
		}
		if clusterVersionObj == nil {
			clusterVersionObj = assureClusterVersionWithConsensusObj()
		}

		clusterObj := testdbaas.MockClusterObj(clusterDefObj.GetName(), clusterVersionObj.GetName())
		clusterObj.Spec.Components = []dbaasv1alpha1.ClusterComponent{{
			Name: "wesql-test",
			Type: "replicasets",
			VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{{
				Name: volumeName,
				Spec: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
			}},
		}}
		clusterKey := client.ObjectKeyFromObject(clusterObj)
		return clusterObj, clusterDefObj, clusterVersionObj, clusterKey
	}

	listAndCheckStatefulSet := func(key types.NamespacedName) *appsv1.StatefulSetList {
		By("Check statefulset workload has been created")
		stsList := &appsv1.StatefulSetList{}
		Eventually(func() bool {
			Expect(k8sClient.List(ctx, stsList, client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey: key.Name,
			}, client.InNamespace(key.Namespace))).Should(Succeed())
			return len(stsList.Items) > 0
		}, timeout, interval).Should(BeTrue())
		return stsList
	}

	Context("after the cluster initialized", func() {
		const initializedVersion = 1

		BeforeEach(func() {
			By("Creating a cluster")
			Expect(testCtx.CreateObj(ctx, clusterObj)).Should(Succeed())

			By("Waiting for the cluster initialized")
			Eventually(testdbaas.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *dbaasv1alpha1.Cluster) {
				g.Expect(fetched.Status.ObservedGeneration == initializedVersion).To(BeTrue())
			})).Should(Succeed())
		})

		It("should create cluster and all sub-resources successfully", func() {
			By("Check deployment workload has been created")
			Eventually(func() bool {
				deployList := &appsv1.DeploymentList{}
				Expect(k8sClient.List(ctx, deployList, client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey: clusterKey.Name,
				}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
				return len(deployList.Items) != 0
			}, timeout, interval).Should(BeTrue())

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
			Eventually(func() bool {
				pdbList := &policyv1.PodDisruptionBudgetList{}
				Expect(k8sClient.List(ctx, pdbList, client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey: clusterKey.Name,
				}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
				return len(pdbList.Items) == 0
			}, timeout, interval).Should(BeTrue())

			By("Check created sts pods template without tolerations")
			Expect(len(stsList.Items[0].Spec.Template.Spec.Tolerations) == 0).Should(BeTrue())

			By("Checking the Affinity and the TopologySpreadConstraints")
			podSpec := stsList.Items[0].Spec.Template.Spec
			Expect(podSpec.Affinity).Should(BeNil())
			Expect(len(podSpec.TopologySpreadConstraints) == 0).Should(BeTrue())

			By("Check should create env configmap")
			cmList := &corev1.ConfigMapList{}
			Eventually(func() bool {
				Expect(k8sClient.List(ctx, cmList, client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey:   clusterKey.Name,
					intctrlutil.AppConfigTypeLabelKey: "kubeblocks-env",
				}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
				return len(cmList.Items) == 2
			}, timeout, interval).Should(BeTrue())
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

				comp, err := util.GetComponentDeftByCluster(ctx, k8sClient, fetched, name)
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
				intctrlutil.AppComponentLabelKey: "replicasets",
			}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
			Expect(len(svcList2.Items) == 1).Should(BeTrue())
			Expect(svcList2.Items[0].Spec.Type == corev1.ServiceTypeClusterIP).To(BeTrue())
			Expect(svcList2.Items[0].Spec.ClusterIP == corev1.ClusterIPNone).To(BeTrue())
			Expect(reflect.DeepEqual(svcList2.Items[0].Spec.Ports,
				getHeadlessSvcPorts("replicasets"))).Should(BeTrue())
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
			}), timeout, interval).Should(Succeed())

			By("Delete the cluster")
			testdbaas.DeleteObject(&testCtx, clusterKey, &dbaasv1alpha1.Cluster{})

			By("Check the cluster do not terminate immediately")
			checkClusterDoNotTerminate := func(g Gomega) {
				fetched := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, clusterKey, fetched)).To(Succeed())
				g.Expect(strings.Contains(fetched.Status.Message,
					fmt.Sprintf("spec.terminationPolicy %s is preventing deletion.", fetched.Spec.TerminationPolicy)))
				g.Expect(len(fetched.Finalizers) > 0).To(BeTrue())
			}
			Eventually(checkClusterDoNotTerminate, timeout, interval).Should(Succeed())
			Consistently(checkClusterDoNotTerminate, waitDuration, interval).Should(Succeed())

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
				changeClusterReplicas(clusterKey, replicas)
				expectedOG++

				By("Checking cluster status and the number of replicas changed")
				Eventually(testdbaas.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *dbaasv1alpha1.Cluster) {
					g.Expect(fetched.Status.ObservedGeneration == expectedOG).To(BeTrue())
				})).Should(Succeed())
				stsList := listAndCheckStatefulSet(clusterKey)
				Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(replicas))
			}
		})

		It("should fail if updating cluster's replica number to an invalid value", func() {
			invalidReplicas := int32(-1)
			By(fmt.Sprintf("Change replicas to %d", invalidReplicas))
			changeClusterReplicas(clusterKey, invalidReplicas)

			By("Checking cluster status and the number of replicas unchanged")
			Consistently(testdbaas.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *dbaasv1alpha1.Cluster) {
				g.Expect(fetched.Status.ObservedGeneration == initializedVersion).To(BeTrue())
			})).Should(Succeed())
			stsList := listAndCheckStatefulSet(clusterKey)
			Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(1))
		})
	})

	createCustomizedClusterNCheck := func(customizeCluster func(toCreate *dbaasv1alpha1.Cluster)) (
		*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, types.NamespacedName) {
		By("Creating a cluster")
		customizeCluster(clusterObj)
		Expect(testCtx.CreateObj(ctx, clusterObj)).Should(Succeed())

		fetched := &dbaasv1alpha1.Cluster{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, clusterKey, fetched)).To(Succeed())
			g.Expect(fetched.Status.ObservedGeneration == 1).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		return fetched, clusterDefObj, clusterVersionObj, clusterKey
	}

	Context("When horizontal scaling out a cluster", func() {
		It("Should trigger a backup process(snapshot) and "+
			"create pvcs from backup for newly created replicas", func() {
			compName := "replicasets"

			By("Creating a cluster with VolumeClaimTemplate")
			var pvcSpec corev1.PersistentVolumeClaimSpec
			pvcSpec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
			pvcSpec.Resources.Requests = corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			}
			initialReplicas := int32(1)
			_, clusterDef, _, key := createCustomizedClusterNCheck(func(toCreate *dbaasv1alpha1.Cluster) {
				toCreate.Spec.Components = []dbaasv1alpha1.ClusterComponent{{
					Name:     compName,
					Type:     compName,
					Replicas: &initialReplicas,
					VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{{
						Name: volumeName,
						Spec: &pvcSpec,
					}},
				}}
			})

			By("Set HorizontalScalePolicy")
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDef),
				func(clusterDef *dbaasv1alpha1.ClusterDefinition) {
					clusterDef.Spec.Components[0].HorizontalScalePolicy =
						&dbaasv1alpha1.HorizontalScalePolicy{Type: dbaasv1alpha1.HScaleDataClonePolicyFromSnapshot}
				}), timeout, interval).Should(Succeed())

			By("Creating a BackupPolicyTemplate")
			backupPolicyTplKey := types.NamespacedName{Name: "test-backup-policy-template-mysql"}
			backupPolicyTemplateYaml := fmt.Sprintf(`
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupPolicyTemplate
metadata:
  name: %s
  labels:
    clusterdefinition.kubeblocks.io/name: %s
spec:
  schedule: "0 2 * * *"
  ttl: 168h0m0s
  # !!DISCUSS Number of backup retries on fail.
  onFailAttempted: 3
  hooks:
    ContainerName: mysql
    image: rancher/kubectl:v1.23.7
    preCommands:
    - touch /data/mysql/data/.restore; sync
  backupToolName: mysql-xtrabackup
`, backupPolicyTplKey.Name, clusterDef.Name)
			backupPolicyTemplate := dataprotectionv1alpha1.BackupPolicyTemplate{}
			Expect(yaml.Unmarshal([]byte(backupPolicyTemplateYaml), &backupPolicyTemplate)).Should(Succeed())
			Expect(testCtx.CheckedCreateObj(ctx, &backupPolicyTemplate)).Should(Succeed())

			By("Mocking PVC for the first replica")
			for i := 0; i < int(initialReplicas); i++ {
				pvcYAML := fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: %s-%s-%s-%d
  namespace: default
  labels:
    app.kubernetes.io/instance: %s
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: test-sc
  volumeMode: Filesystem
  volumeName: test-pvc
`, volumeName, key.Name, compName, i, key.Name)
				pvc := corev1.PersistentVolumeClaim{}
				Expect(yaml.Unmarshal([]byte(pvcYAML), &pvc)).Should(Succeed())
				Expect(testCtx.CreateObj(ctx, &pvc)).Should(Succeed())
			}

			stsList := listAndCheckStatefulSet(key)
			Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(initialReplicas))

			updatedReplicas := int32(3)
			By(fmt.Sprintf("Changing replicas to %d", updatedReplicas))
			changeClusterReplicas(key, updatedReplicas)

			By("Checking BackupJob created")
			Eventually(func() bool {
				backupList := dataprotectionv1alpha1.BackupList{}
				Expect(k8sClient.List(ctx, &backupList, client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey: key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(backupList.Items) == 1
			}, timeout, interval).Should(BeTrue())

			By("Mocking VolumeSnapshot and set it as ReadyToUse")
			snapshotKey := types.NamespacedName{Name: fmt.Sprintf("%s-%s-scaling",
				key.Name, compName), Namespace: "default"}
			pvcName := fmt.Sprintf("%s-%s-%s-0", volumeName, key.Name, compName)
			volumeSnapshotYaml := fmt.Sprintf(`
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/created-by: kubeblocks
    app.kubernetes.io/instance: %s
    app.kubernetes.io/component-name: %s
spec:
  source:
    persistentVolumeClaimName: %s
`, snapshotKey.Name, snapshotKey.Namespace, key.Name, compName, pvcName)
			volumeSnapshot := snapshotv1.VolumeSnapshot{}
			Expect(yaml.Unmarshal([]byte(volumeSnapshotYaml), &volumeSnapshot)).Should(Succeed())
			Expect(testCtx.CheckedCreateObj(ctx, &volumeSnapshot)).Should(Succeed())
			readyToUse := true
			volumeSnapshotStatus := snapshotv1.VolumeSnapshotStatus{ReadyToUse: &readyToUse}
			volumeSnapshot.Status = &volumeSnapshotStatus
			Expect(k8sClient.Status().Update(ctx, &volumeSnapshot)).Should(Succeed())

			By("Mock PVCs status to bound")
			for i := 0; i < int(updatedReplicas); i++ {
				pvcKey := types.NamespacedName{
					Namespace: key.Namespace,
					Name:      fmt.Sprintf("%s-%s-%s-%d", volumeName, key.Name, compName, i),
				}
				Eventually(testdbaas.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true), timeout, interval).Should(Succeed())
				Eventually(testdbaas.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Phase = corev1.ClaimBound
				}), timeout, interval).Should(Succeed())
			}

			By("Check backup job cleanup")
			Eventually(func() bool {
				backupList := dataprotectionv1alpha1.BackupList{}
				Expect(k8sClient.List(ctx, &backupList, client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey: key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(backupList.Items) == 0
			}, timeout, interval).Should(BeTrue())
			Eventually(testdbaas.CheckObjExists(&testCtx, snapshotKey, &snapshotv1.VolumeSnapshot{}, false), timeout, interval).Should(Succeed())

			By("Checking cluster status and the number of replicas changed")
			Eventually(testdbaas.CheckObj(&testCtx, key, func(g Gomega, cluster *dbaasv1alpha1.Cluster) {
				g.Expect(cluster.Status.ObservedGeneration == 2).To(BeTrue())
			}), timeout, interval).Should(Succeed())
			stsList = listAndCheckStatefulSet(key)
			Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(updatedReplicas))
		})
	})

	Context("When updating cluster PVC storage size", func() {
		It("Should update PVC request storage size accordingly", func() {

			By("Mock a StorageClass which allows resize")
			StorageClassYaml := `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
   name: sc-mock
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
`
			storageClass := &storagev1.StorageClass{}
			Expect(yaml.Unmarshal([]byte(StorageClassYaml), storageClass)).Should(Succeed())
			Expect(testCtx.CheckedCreateObj(ctx, storageClass)).Should(Succeed())

			By("Creating a cluster with volume claim")
			replicas := int32(2)
			clusterObj.Spec.Components = make([]dbaasv1alpha1.ClusterComponent, 1)
			clusterObj.Spec.Components[0] = dbaasv1alpha1.ClusterComponent{
				Name:     "replicasets",
				Type:     "replicasets",
				Replicas: &replicas,
				VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{{
					Name: volumeName,
					Spec: &corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						StorageClassName: &storageClass.Name,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					}},
				},
			}
			Expect(testCtx.CreateObj(ctx, clusterObj)).Should(Succeed())

			Eventually(func(g Gomega) {
				fetchedG1 := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, clusterKey, fetchedG1)).To(Succeed())
				g.Expect(fetchedG1.Status.ObservedGeneration == 1).To(BeTrue())
			}, timeout*2, interval).Should(Succeed())

			By("Checking the replicas")
			stsList := listAndCheckStatefulSet(clusterKey)
			sts := &stsList.Items[0]
			Expect(*sts.Spec.Replicas == replicas).Should(BeTrue())

			By("Mock PVCs in Bound Status")
			for i := 0; i < int(replicas); i++ {
				pvcYAML := fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: %s-%s-%d
  namespace: default
  labels:
    app.kubernetes.io/instance: %s
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: %s
`, volumeName, sts.Name, i, clusterKey.Name, storageClass.Name)
				pvc := corev1.PersistentVolumeClaim{}
				Expect(yaml.Unmarshal([]byte(pvcYAML), &pvc)).Should(Succeed())
				Expect(testCtx.CreateObj(ctx, &pvc)).Should(Succeed())
				pvc.Status.Phase = corev1.ClaimBound // only bound pvc allows resize
				Expect(k8sClient.Status().Update(ctx, &pvc)).Should(Succeed())
			}

			By("Updating the PVC storage size")
			newStorageValue := resource.MustParse("2Gi")
			Eventually(testdbaas.GetAndChangeObj(&testCtx, clusterKey, func(cluster *dbaasv1alpha1.Cluster) {
				comp := &cluster.Spec.Components[0]
				comp.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue
			}), timeout, interval).Should(Succeed())

			By("Checking the resize operation finished")
			Eventually(func(g Gomega) {
				fetchedG2 := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, clusterKey, fetchedG2)).To(Succeed())
				g.Expect(fetchedG2.Status.ObservedGeneration == 2).To(BeTrue())
			}, timeout*2, interval).Should(Succeed())

			By("Checking PVCs are resized")
			stsList = listAndCheckStatefulSet(clusterKey)
			for _, sts := range stsList.Items {
				for _, vct := range sts.Spec.VolumeClaimTemplates {
					for i := *sts.Spec.Replicas - 1; i >= 0; i-- {
						pvc := &corev1.PersistentVolumeClaim{}
						pvcKey := types.NamespacedName{
							Namespace: clusterKey.Namespace,
							Name:      fmt.Sprintf("%s-%s-%d", vct.Name, sts.Name, i),
						}
						Expect(k8sClient.Get(ctx, pvcKey, pvc)).Should(Succeed())
						Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(newStorageValue))
					}
				}
			}
		})
	})

	mockPodsForConsensusTest := func(cluster *dbaasv1alpha1.Cluster, number int) []corev1.Pod {
		podYaml := `
apiVersion: v1
kind: Pod
metadata:
  labels:
    controller-revision-hash: mock-version
  name: my-name
  namespace: default
spec:
  containers:
  - args:
    command:
    - /bin/bash
    - -c
    env:
    - name: KB_POD_NAME
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: metadata.name
    - name: KB_REPLICASETS_N
      value: "3"
    - name: KB_REPLICASETS_0_HOSTNAME
      value: clusterepuglf-wesql-test-0
    - name: KB_REPLICASETS_1_HOSTNAME
      value: clusterepuglf-wesql-test-1
    - name: KB_REPLICASETS_2_HOSTNAME
      value: clusterepuglf-wesql-test-2
    image: docker.io/apecloud/wesql-server:latest
    imagePullPolicy: IfNotPresent
    name: mysql
    ports:
    - containerPort: 3306
      name: mysql
      protocol: TCP
    - containerPort: 13306
      name: paxos
      protocol: TCP
    volumeMounts:
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-2rhsb
      readOnly: true
  dnsPolicy: ClusterFirst
  enableServiceLinks: true
  restartPolicy: Always
  serviceAccount: default
  serviceAccountName: default

  volumes:
  - name: kube-api-access-2rhsb
    projected:
      defaultMode: 420
      sources:
      - serviceAccountToken:
          expirationSeconds: 3607
          path: token
      - configMap:
          items:
          - key: ca.crt
            path: ca.crt
          name: kube-root-ca.crt
      - downwardAPI:
          items:
          - fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
            path: namespace
`
		pods := make([]corev1.Pod, 0)
		componentName := cluster.Spec.Components[0].Name
		clusterName := cluster.Name
		stsName := cluster.Name + "-" + componentName
		for i := 0; i < number; i++ {
			pod := corev1.Pod{}
			Expect(yaml.Unmarshal([]byte(podYaml), &pod)).Should(Succeed())
			pod.Name = stsName + "-" + strconv.Itoa(i)
			pod.Labels[intctrlutil.AppInstanceLabelKey] = clusterName
			pod.Labels[intctrlutil.AppComponentLabelKey] = componentName
			pods = append(pods, pod)
		}

		return pods
	}

	mockRoleChangedEvent := func(key types.NamespacedName, sts *appsv1.StatefulSet) []corev1.Event {
		eventYaml := `
apiVersion: v1
kind: Event
metadata:
  name: myevent
  namespace: default
type: Warning
reason: Unhealthy
reportingComponent: ""
message: 'Readiness probe failed: {"event":"roleUnchanged","originalRole":"Leader","role":"Follower"}'
involvedObject:
  apiVersion: v1
  fieldPath: spec.containers{kb-rolechangedcheck}
  kind: Pod
  name: wesql-main-2
  namespace: default
`
		pods, err := consensusset.GetPodListByStatefulSet(ctx, k8sClient, sts)
		Expect(err).To(Succeed())

		events := make([]corev1.Event, 0)
		for _, pod := range pods {
			event := corev1.Event{}
			Expect(yaml.Unmarshal([]byte(eventYaml), &event)).Should(Succeed())
			event.Name = pod.Name + "-event"
			event.InvolvedObject.Name = pod.Name
			event.InvolvedObject.UID = pod.UID
			events = append(events, event)
		}
		events[0].Message = `Readiness probe failed: {"event":"roleUnchanged","originalRole":"Leader","role":"Leader"}`
		return events
	}

	getStsPodsName := func(sts *appsv1.StatefulSet) []string {
		pods, err := consensusset.GetPodListByStatefulSet(ctx, k8sClient, sts)
		Expect(err).To(Succeed())

		names := make([]string, 0)
		for _, pod := range pods {
			names = append(names, pod.Name)
		}
		return names
	}

	Context("When creating cluster with componentType = Consensus", func() {
		const leader = "leader"
		const follower = "follower"

		It("Should success with: "+
			"1 pod with 'leader' role label set, "+
			"2 pods with 'follower' role label set,"+
			"1 service routes to 'leader' pod", func() {
			By("Creating a cluster with componentType = Consensus")
			replicas := 3

			toCreate, _, _, key := newClusterWithConsensusObj(nil, nil)
			Expect(testCtx.CreateObj(ctx, toCreate)).Should(Succeed())

			By("Waiting for cluster creation")
			Eventually(func(g Gomega) {
				fetched := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, key, fetched)).To(Succeed())
				g.Expect(fetched.Status.ObservedGeneration == 1).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			stsList := listAndCheckStatefulSet(key)
			sts := &stsList.Items[0]

			By("Creating mock pods in StatefulSet")
			pods := mockPodsForConsensusTest(toCreate, replicas)
			for _, pod := range pods {
				Expect(testCtx.CreateObj(ctx, &pod)).Should(Succeed())
				// mock the status to pass the isReady(pod) check in consensus_set
				pod.Status.Conditions = []corev1.PodCondition{{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}}
				Expect(k8sClient.Status().Update(ctx, &pod)).Should(Succeed())
			}

			By("Creating mock role changed events")
			// pod.Labels[intctrlutil.ConsensusSetRoleLabelKey] will be filled with the role
			events := mockRoleChangedEvent(key, sts)
			for _, event := range events {
				Expect(testCtx.CreateObj(ctx, &event)).Should(Succeed())
			}

			By("Checking pods' role are changed accordingly")
			Eventually(func(g Gomega) {
				pods, err := consensusset.GetPodListByStatefulSet(ctx, k8sClient, sts)
				g.Expect(err).To(Succeed())
				// should have 3 pods
				g.Expect(len(pods)).To(Equal(3))
				// 1 leader
				// 2 followers
				leaderCount, followerCount := 0, 0
				for _, pod := range pods {
					switch pod.Labels[intctrlutil.ConsensusSetRoleLabelKey] {
					case leader:
						leaderCount++
					case follower:
						followerCount++
					}
				}
				g.Expect(leaderCount).Should(Equal(1))
				g.Expect(followerCount).Should(Equal(2))
			}, 2*timeout, interval).Should(Succeed())

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
				fetched := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, key, fetched)).To(Succeed())
				compName := fetched.Spec.Components[0].Name
				g.Expect(fetched.Status.Components != nil).To(BeTrue())
				g.Expect(fetched.Status.Components).To(HaveKey(compName))
				consensusStatus := fetched.Status.Components[compName].ConsensusSetStatus
				g.Expect(consensusStatus != nil).To(BeTrue())
				g.Expect(consensusStatus.Leader.Pod).To(BeElementOf(getStsPodsName(sts)))
				g.Expect(len(consensusStatus.Followers) == 2).To(BeTrue())
				g.Expect(consensusStatus.Followers[0].Pod).To(BeElementOf(getStsPodsName(sts)))
				g.Expect(consensusStatus.Followers[1].Pod).To(BeElementOf(getStsPodsName(sts)))
			}, 2*timeout, interval).Should(Succeed())

			By("Waiting the cluster be running")
			Eventually(func(g Gomega) {
				fetched := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, key, fetched)).To(Succeed())
				g.Expect(fetched.Status.Phase == dbaasv1alpha1.RunningPhase).To(BeTrue())
			}, timeout, interval).Should(Succeed())

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
				Name: "replicasets",
				Type: "replicasets",
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
				Name:        "replicasets",
				Type:        "replicasets",
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

func changeClusterReplicas(clusterName types.NamespacedName, replicas int32) {
	Eventually(testdbaas.GetAndChangeObj(&testCtx, clusterName, func(cluster *dbaasv1alpha1.Cluster) {
		if cluster.Spec.Components == nil || len(cluster.Spec.Components) == 0 {
			cluster.Spec.Components = []dbaasv1alpha1.ClusterComponent{
				{
					Name:     "replicasets",
					Type:     "replicasets",
					Replicas: &replicas,
				}}
		} else {
			cluster.Spec.Components[0].Replicas = &replicas
		}
	})).Should(Succeed())
}
