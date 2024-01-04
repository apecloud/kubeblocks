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

package plan

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

var _ = Describe("Restore", func() {
	const backupName = "test-backup-job"
	const sourceCluster = "source-cluster"

	var (
		randomStr   = testCtx.GetRandomStr()
		clusterName = "cluster-" + randomStr

		now       = metav1.Now()
		startTime = metav1.Time{Time: now.Add(-time.Hour * 2)}
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		deletionPropagation := metav1.DeletePropagationBackground
		deletionGracePeriodSeconds := int64(0)
		opts := client.DeleteAllOfOptions{
			DeleteOptions: client.DeleteOptions{
				GracePeriodSeconds: &deletionGracePeriodSeconds,
				PropagationPolicy:  &deletionPropagation,
			},
		}
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, &opts)
		testapps.ClearResources(&testCtx, generics.BackupSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.BackupPolicySignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.RestoreSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.ComponentSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		//
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.BackupPolicyTemplateSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Cluster Restore", func() {
		const (
			clusterDefName     = "test-clusterdef"
			clusterVersionName = "test-clusterversion"
			mysqlCompType      = "replicasets"
			mysqlCompName      = "mysql"
			nginxCompType      = "proxy"
			topologyKey        = "testTopologyKey"
			labelKey           = "testNodeLabelKey"
			labelValue         = "testLabelValue"
		)

		var (
			clusterDef              *appsv1alpha1.ClusterDefinition
			clusterVersion          *appsv1alpha1.ClusterVersion
			cluster                 *appsv1alpha1.Cluster
			synthesizedComponent    *component.SynthesizedComponent
			compObj                 *appsv1alpha1.Component
			pvc                     *corev1.PersistentVolumeClaim
			backup                  *dpv1alpha1.Backup
			fullBackupActionSet     *dpv1alpha1.ActionSet
			fullBackupActionSetName string
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ConsensusMySQLComponent, mysqlCompType).
				AddComponentDef(testapps.StatelessNginxComponent, nginxCompType).
				Create(&testCtx).GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponentVersion(mysqlCompType).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				AddComponentVersion(nginxCompType).
				AddInitContainerShort("nginx-init", testapps.NginxImage).
				AddContainerShort("nginx", testapps.NginxImage).
				Create(&testCtx).GetObject()
			pvcSpec := testapps.NewPVCSpec("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompType).
				SetReplicas(3).
				SetClusterAffinity(&appsv1alpha1.Affinity{
					PodAntiAffinity: appsv1alpha1.Required,
					TopologyKeys:    []string{topologyKey},
					NodeLabels: map[string]string{
						labelKey: labelValue,
					},
				}).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				Create(&testCtx).GetObject()

			By("By mocking a pvc")
			pvc = testapps.NewPersistentVolumeClaimFactory(
				testCtx.DefaultNamespace, "data-"+clusterName+"-"+mysqlCompName+"-0", clusterName, mysqlCompName, "data").
				SetStorage("1Gi").
				Create(&testCtx).GetObject()

			By("By mocking a pod")
			volume := corev1.Volume{Name: pvc.Name, VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}
			_ = testapps.NewPodFactory(testCtx.DefaultNamespace, clusterName+"-"+mysqlCompName+"-0").
				AddAppInstanceLabel(clusterName).
				AddAppComponentLabel(mysqlCompName).
				AddAppManagedByLabel().
				AddVolume(volume).
				AddLabels(constant.ConsensusSetAccessModeLabelKey, string(appsv1alpha1.ReadWrite)).
				AddContainer(corev1.Container{Name: testapps.DefaultMySQLContainerName, Image: testapps.ApeCloudMySQLImage}).
				AddNodeName("fake-node-name").
				Create(&testCtx).GetObject()

			By("create actionset of full backup")
			fullBackupActionSet = testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml", &dpv1alpha1.ActionSet{}, testapps.RandomizedObjName())
			fullBackupActionSetName = fullBackupActionSet.Name

			By("By creating backup policyTemplate: ")
			backupTplLabels := map[string]string{
				constant.ClusterDefLabelKey: clusterDefName,
			}
			_ = testapps.NewBackupPolicyTemplateFactory("backup-policy-template").
				WithRandomName().SetLabels(backupTplLabels).
				AddBackupPolicy(mysqlCompName).
				SetClusterDefRef(clusterDefName).
				AddBackupMethod(testdp.BackupMethodName, false, fullBackupActionSetName).
				SetBackupMethodVolumeMounts(testapps.DataVolumeName, "/data")

			clusterCompDefObj := clusterDef.Spec.ComponentDefs[0]
			synthesizedComponent = &component.SynthesizedComponent{
				WorkloadType:          appsv1alpha1.Consensus,
				PodSpec:               clusterCompDefObj.PodSpec,
				Probes:                clusterCompDefObj.Probes,
				LogConfigs:            clusterCompDefObj.LogConfigs,
				HorizontalScalePolicy: clusterCompDefObj.HorizontalScalePolicy,
				VolumeClaimTemplates:  cluster.Spec.ComponentSpecs[0].ToVolumeClaimTemplates(),
				Name:                  mysqlCompName,
				VolumeTypes:           []appsv1alpha1.VolumeTypeSpec{{Name: testapps.DataVolumeName, Type: appsv1alpha1.VolumeTypeData}},
				Replicas:              1,
			}
			By("create component object")
			compObj = testapps.NewComponentFactory(testCtx.DefaultNamespace, cluster.Name+"-"+synthesizedComponent.Name, "").
				AddLabels(constant.AppInstanceLabelKey, cluster.Name).
				AddLabels(constant.KBAppClusterUIDLabelKey, string(cluster.UID)).
				SetReplicas(1).
				Create(&testCtx).
				GetObject()

			By("By creating remote pvc: ")
			remotePVC := testapps.NewPersistentVolumeClaimFactory(
				testCtx.DefaultNamespace, "remote-pvc", clusterName, mysqlCompName, "log").
				SetStorage("1Gi").
				Create(&testCtx).GetObject()

			By("By creating base backup: ")
			backupLabels := map[string]string{
				constant.AppInstanceLabelKey:    sourceCluster,
				constant.KBAppComponentLabelKey: mysqlCompName,
			}
			backup = testdp.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				WithRandomName().SetLabels(backupLabels).
				SetBackupPolicyName("test-fake").
				SetBackupMethod(testdp.VSBackupMethodName).
				Create(&testCtx).GetObject()
			baseStartTime := &startTime
			baseStopTime := &now
			backup.Status = dpv1alpha1.BackupStatus{
				Phase:                     dpv1alpha1.BackupPhaseCompleted,
				StartTimestamp:            baseStartTime,
				CompletionTimestamp:       baseStopTime,
				PersistentVolumeClaimName: remotePVC.Name,
			}
			testdp.MockBackupStatusMethod(backup, testdp.VSBackupMethodName, testapps.DataVolumeName, testdp.ActionSetName)
			patchBackupStatus(backup.Status, client.ObjectKeyFromObject(backup))
		})

		It("Test restore", func() {
			By("restore from backup")
			restoreFromBackup := fmt.Sprintf(`{"%s": {"name":"%s"}}`, mysqlCompName, backup.Name)
			Expect(testapps.ChangeObj(&testCtx, cluster, func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Annotations = map[string]string{
					constant.RestoreFromBackupAnnotationKey: restoreFromBackup,
				}
			})).Should(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).Should(Succeed())
			restoreMGR := NewRestoreManager(ctx, k8sClient, cluster, scheme.Scheme, nil, 3, 0)
			err := restoreMGR.DoRestore(synthesizedComponent, compObj)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting)).Should(BeTrue())

			By("mock restore of prepareData stage to Completed")
			restoreMeta := restoreMGR.GetRestoreObjectMeta(synthesizedComponent, dpv1alpha1.PrepareData)
			namedspace := types.NamespacedName{Name: restoreMeta.Name, Namespace: restoreMeta.Namespace}
			Expect(testapps.GetAndChangeObjStatus(&testCtx, namedspace, func(restore *dpv1alpha1.Restore) {
				restore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
			})()).ShouldNot(HaveOccurred())

			By("mock component and cluster phase to Running")
			Expect(testapps.ChangeObjStatus(&testCtx, cluster, func() {
				cluster.Status.Phase = appsv1alpha1.RunningClusterPhase
				cluster.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
					mysqlCompName: {
						Phase: appsv1alpha1.RunningClusterCompPhase,
					},
				}
			})).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, compObj, func() {
				compObj.Status.Phase = appsv1alpha1.RunningClusterCompPhase
			})).Should(Succeed())

			By("wait for postReady restore created and mock it to Completed")
			restoreMGR.Cluster = cluster
			_ = restoreMGR.DoRestore(synthesizedComponent, compObj)

			// check if restore CR of postReady stage is created.
			restoreMeta = restoreMGR.GetRestoreObjectMeta(synthesizedComponent, dpv1alpha1.PostReady)
			namedspace = types.NamespacedName{Name: restoreMeta.Name, Namespace: restoreMeta.Namespace}
			Eventually(testapps.CheckObjExists(&testCtx, namedspace,
				&dpv1alpha1.Restore{}, true)).Should(Succeed())
			// set restore to Completed
			Expect(testapps.GetAndChangeObjStatus(&testCtx, namedspace, func(restore *dpv1alpha1.Restore) {
				restore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
			})()).ShouldNot(HaveOccurred())

			By("clean up annotations after cluster running")
			_ = restoreMGR.DoRestore(synthesizedComponent, compObj)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				g.Expect(tmpCluster.Annotations[constant.RestoreFromBackupAnnotationKey]).Should(BeEmpty())
			})).Should(Succeed())
		})

		It("unsupported restore to different namespace", func() {
			const fakeNamespace = "fake-namespace"
			restoreFromBackup := fmt.Sprintf(`{"%s": {"name":"%s", "namespace":"%s"}}`, mysqlCompName, backup.Name, fakeNamespace)
			Expect(testapps.ChangeObj(&testCtx, cluster, func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Annotations = map[string]string{
					constant.RestoreFromBackupAnnotationKey: restoreFromBackup,
				}
			})).Should(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).Should(Succeed())
			restoreMGR := NewRestoreManager(ctx, k8sClient, cluster, scheme.Scheme, nil, 3, 0)
			err := restoreMGR.DoRestore(synthesizedComponent, compObj)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeRestoreFailed)).Should(BeTrue())
		})
	})
})

func patchBackupStatus(status dpv1alpha1.BackupStatus, key types.NamespacedName) {
	Eventually(testapps.GetAndChangeObjStatus(&testCtx, key, func(fetched *dpv1alpha1.Backup) {
		fetched.Status = status
	})).Should(Succeed())
}
