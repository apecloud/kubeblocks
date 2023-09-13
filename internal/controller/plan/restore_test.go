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
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes/scheme"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("PITR Functions", func() {
	const defaultTTL = "7d"
	const backupName = "test-backup-job"
	const sourceCluster = "source-cluster"

	var (
		randomStr   = testCtx.GetRandomStr()
		clusterName = "cluster-for-pitr-" + randomStr

		now       = metav1.Now()
		startTime = metav1.Time{Time: now.Add(-time.Hour * 2)}
		stopTime  = metav1.Time{Time: now.Add(time.Hour * 2)}
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
		testapps.ClearResources(&testCtx, generics.JobSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.CronJobSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		//
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.BackupPolicyTemplateSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test PITR", func() {
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
			clusterDef               *appsv1alpha1.ClusterDefinition
			clusterVersion           *appsv1alpha1.ClusterVersion
			cluster                  *appsv1alpha1.Cluster
			synthesizedComponent     *component.SynthesizedComponent
			pvc                      *corev1.PersistentVolumeClaim
			backup                   *dpv1alpha1.Backup
			fullBackupTool           *dpv1alpha1.BackupTool
			fullBackupToolName       string
			continuousBackupTool     *dpv1alpha1.BackupTool
			continuousBackupToolName string
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompType).
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
				SetClusterAffinity(&appsv1alpha1.Affinity{
					PodAntiAffinity: appsv1alpha1.Required,
					TopologyKeys:    []string{topologyKey},
					NodeLabels: map[string]string{
						labelKey: labelValue,
					},
				}).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				AddRestorePointInTime(metav1.Time{Time: stopTime.Time}, mysqlCompName, sourceCluster).
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
				AddAppManangedByLabel().
				AddVolume(volume).
				AddLabels(constant.ConsensusSetAccessModeLabelKey, string(appsv1alpha1.ReadWrite)).
				AddContainer(corev1.Container{Name: testapps.DefaultMySQLContainerName, Image: testapps.ApeCloudMySQLImage}).
				AddNodeName("fake-node-name").
				Create(&testCtx).GetObject()

			By("create datafile backup tool")
			fullBackupTool = testapps.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml", &dpv1alpha1.BackupTool{}, testapps.RandomizedObjName())
			fullBackupToolName = fullBackupTool.Name

			By("By creating backup tool: ")
			backupSelfDefineObj := &dpv1alpha1.BackupTool{}
			backupSelfDefineObj.SetLabels(map[string]string{
				constant.BackupToolTypeLabelKey: "pitr",
				constant.ClusterDefLabelKey:     clusterDefName,
			})
			continuousBackupTool = testapps.CreateCustomizedObj(&testCtx, "backup/pitr_backuptool.yaml",
				backupSelfDefineObj, testapps.RandomizedObjName())
			// set datafile backup relies on logfile
			Expect(testapps.ChangeObj(&testCtx, continuousBackupTool, func(tmpObj *dpv1alpha1.BackupTool) {
				tmpObj.Spec.Physical.RelyOnLogfile = true
			})).Should(Succeed())
			continuousBackupToolName = continuousBackupTool.Name

			backupObj := dpv1alpha1.BackupToolList{}
			Expect(testCtx.Cli.List(testCtx.Ctx, &backupObj)).Should(Succeed())

			By("By creating backup policyTemplate: ")
			backupTplLabels := map[string]string{
				constant.ClusterDefLabelKey: clusterDefName,
			}
			_ = testapps.NewBackupPolicyTemplateFactory("backup-policy-template").
				WithRandomName().SetLabels(backupTplLabels).
				AddBackupPolicy(mysqlCompName).
				SetClusterDefRef(clusterDefName).
				SetTTL(defaultTTL).
				AddDatafilePolicy().
				SetBackupToolName(fullBackupToolName).
				SetSchedule("0 * * * *", true).
				AddIncrementalPolicy().
				SetBackupToolName(continuousBackupToolName).
				SetSchedule("0 * * * *", true).
				Create(&testCtx).GetObject()

			clusterCompDefObj := clusterDef.Spec.ComponentDefs[0]
			synthesizedComponent = &component.SynthesizedComponent{
				PodSpec:               clusterCompDefObj.PodSpec,
				Probes:                clusterCompDefObj.Probes,
				LogConfigs:            clusterCompDefObj.LogConfigs,
				HorizontalScalePolicy: clusterCompDefObj.HorizontalScalePolicy,
				VolumeClaimTemplates:  cluster.Spec.ComponentSpecs[0].ToVolumeClaimTemplates(),
				Name:                  mysqlCompName,
				VolumeTypes:           []appsv1alpha1.VolumeTypeSpec{{Name: testapps.DataVolumeName, Type: appsv1alpha1.VolumeTypeData}},
				Replicas:              1,
			}
			By("By creating remote pvc: ")
			remotePVC := testapps.NewPersistentVolumeClaimFactory(
				testCtx.DefaultNamespace, "remote-pvc", clusterName, mysqlCompName, "log").
				SetStorage("1Gi").
				Create(&testCtx).GetObject()

			logfileRemotePVC := testapps.NewPersistentVolumeClaimFactory(
				testCtx.DefaultNamespace, "remote-pvc-logfile", clusterName, mysqlCompName, "log").
				SetStorage("1Gi").
				Create(&testCtx).GetObject()

			By("By creating base backup: ")
			backupLabels := map[string]string{
				constant.AppInstanceLabelKey:              sourceCluster,
				constant.KBAppComponentLabelKey:           mysqlCompName,
				constant.BackupTypeLabelKeyKey:            string(dpv1alpha1.BackupTypeDataFile),
				constant.DataProtectionLabelClusterUIDKey: string(cluster.UID),
			}
			backup = testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				WithRandomName().SetLabels(backupLabels).
				SetBackupPolicyName("test-fake").
				SetBackupMethod(dpv1alpha1.BackupTypeDataFile).
				Create(&testCtx).GetObject()
			baseStartTime := &startTime
			baseStopTime := &now
			backupStatus := dpv1alpha1.BackupStatus{
				Phase:                            dpv1alpha1.BackupPhaseCompleted,
				StartTimestamp:                   baseStartTime,
				CompletionTimestamp:              baseStopTime,
				BackupToolName:                   fullBackupToolName,
				SourceCluster:                    clusterName,
				PersistentVolumeClaimName:        remotePVC.Name,
				LogFilePersistentVolumeClaimName: logfileRemotePVC.Name,
				Manifests: &dpv1alpha1.ManifestsStatus{
					BackupTool: &dpv1alpha1.BackupToolManifestsStatus{
						FilePath:    fmt.Sprintf("/%s/%s", backup.Namespace, backup.Name),
						LogFilePath: fmt.Sprintf("/%s/%s", backup.Namespace, backup.Name+"-logfile"),
					},
					BackupLog: &dpv1alpha1.BackupLogStatus{
						StartTime: baseStartTime,
						StopTime:  baseStopTime,
					},
				},
			}
			patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backup))

			By("By creating continuous backup: ")
			logfileBackupLabels := map[string]string{
				constant.AppInstanceLabelKey:              sourceCluster,
				constant.KBAppComponentLabelKey:           mysqlCompName,
				constant.BackupTypeLabelKeyKey:            string(dpv1alpha1.BackupTypeLogFile),
				constant.DataProtectionLabelClusterUIDKey: string(cluster.UID),
			}
			incrStartTime := &startTime
			incrStopTime := &stopTime
			logfileBackup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				WithRandomName().SetLabels(logfileBackupLabels).
				SetBackupPolicyName("test-fake").
				SetBackupMethod(dpv1alpha1.BackupTypeLogFile).
				Create(&testCtx).GetObject()
			backupStatus = dpv1alpha1.BackupStatus{
				Phase:                     dpv1alpha1.BackupPhaseCompleted,
				StartTimestamp:            incrStartTime,
				CompletionTimestamp:       incrStopTime,
				SourceCluster:             clusterName,
				PersistentVolumeClaimName: logfileRemotePVC.Name,
				BackupToolName:            continuousBackupToolName,
				Manifests: &dpv1alpha1.ManifestsStatus{
					BackupLog: &dpv1alpha1.BackupLogStatus{
						StartTime: incrStartTime,
						StopTime:  incrStopTime,
					},
				},
			}
			patchBackupStatus(backupStatus, client.ObjectKeyFromObject(logfileBackup))
		})

		It("Test restore", func() {
			By("restore from snapshot backup")
			backupSnapshot := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				WithRandomName().
				SetBackupPolicyName("test-fake").
				SetBackupMethod(dpv1alpha1.BackupTypeSnapshot).
				Create(&testCtx).GetObject()
			restoreFromBackup := fmt.Sprintf(`{"%s":"%s"}`, mysqlCompName, backupSnapshot.Name)
			cluster.Annotations[constant.RestoreFromBackUpAnnotationKey] = restoreFromBackup
			Expect(DoRestore(ctx, testCtx.Cli, cluster, synthesizedComponent, scheme.Scheme)).Should(Succeed())

			By("restore from datafile backup")
			restoreFromBackup = fmt.Sprintf(`{"%s":"%s"}`, mysqlCompName, backup.Name)
			cluster.Annotations[constant.RestoreFromBackUpAnnotationKey] = restoreFromBackup
			err := DoRestore(ctx, testCtx.Cli, cluster, synthesizedComponent, scheme.Scheme)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting)).Should(BeTrue())
		})

		testPITR := func() {
			baseBackupPhysicalRestore := func() types.NamespacedName {
				By("create fullBackup physical restore job")
				err := DoPITR(ctx, testCtx.Cli, cluster, synthesizedComponent, scheme.Scheme)
				Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting)).Should(BeTrue())

				By("when base backup restore job completed")
				baseBackupJobName := fmt.Sprintf("base-%s", fmt.Sprintf("%s-%s-%s-%d", "data", clusterName, synthesizedComponent.Name, 0))
				baseBackupJobKey := types.NamespacedName{Namespace: cluster.Namespace, Name: baseBackupJobName}
				Eventually(testapps.CheckObj(&testCtx, baseBackupJobKey, func(g Gomega, fetched *batchv1.Job) {
					envs := fetched.Spec.Template.Spec.Containers[0].Env
					var existsTargetENV bool
					for _, env := range envs {
						if env.Name == constant.KBEnvPodName {
							existsTargetENV = true
							break
						}
					}
					g.Expect(existsTargetENV).Should(BeTrue())
				})).Should(Succeed())
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, baseBackupJobKey, func(fetched *batchv1.Job) {
					fetched.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete}}
				})).Should(Succeed())
				return baseBackupJobKey
			}

			baseBackupLogicalRestore := func() types.NamespacedName {
				By("create and wait for fullbackup logical job is completed ")
				err := DoPITR(ctx, testCtx.Cli, cluster, synthesizedComponent, scheme.Scheme)
				Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting)).Should(BeTrue())

				By("when logic full backup jobs are completed")
				logicJobName := fmt.Sprintf("restore-datafile-logic-%s-%s-0", clusterName, mysqlCompName)
				logicJobKey := types.NamespacedName{Namespace: cluster.Namespace, Name: logicJobName}
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, logicJobKey, func(fetched *batchv1.Job) {
					fetched.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete}}
				})).Should(Succeed())
				return logicJobKey
			}

			continuousPhysicalRestore := func() types.NamespacedName {
				By("create and wait for pitr physical restore job is completed ")
				err := DoPITR(ctx, testCtx.Cli, cluster, synthesizedComponent, scheme.Scheme)
				Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting)).Should(BeTrue())

				By("when physical PITR jobs are completed")
				jobName := fmt.Sprintf("pitr-phy-data-%s-%s-0", clusterName, mysqlCompName)
				jobKey := types.NamespacedName{Namespace: cluster.Namespace, Name: jobName}
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, jobKey, func(fetched *batchv1.Job) {
					fetched.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete}}
				})).Should(Succeed())
				return jobKey
			}

			continuousLogicalRestore := func() types.NamespacedName {
				By("create and wait for pitr logical job is completed ")
				err := DoPITR(ctx, testCtx.Cli, cluster, synthesizedComponent, scheme.Scheme)
				Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting)).Should(BeTrue())

				By("mock the podScope is ReadWrite for logic restore")
				Expect(testapps.ChangeObj(&testCtx, continuousBackupTool, func(tool *dpv1alpha1.BackupTool) {
					tool.Spec.Logical.PodScope = dpv1alpha1.PodRestoreScopeReadWrite
				})).Should(Succeed())
				err = DoPITR(ctx, testCtx.Cli, cluster, synthesizedComponent, scheme.Scheme)
				Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting)).Should(BeTrue())

				By("when logic PITR jobs are completed")
				logicJobName := fmt.Sprintf("restore-logfile-logic-%s-%s-0", clusterName, mysqlCompName)
				logicJobKey := types.NamespacedName{Namespace: cluster.Namespace, Name: logicJobName}
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, logicJobKey, func(fetched *batchv1.Job) {
					fetched.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete}}
				})).Should(Succeed())
				return logicJobKey
			}
			cluster.Status.ObservedGeneration = 1
			var backupJobKeys []types.NamespacedName
			// do full backup physical restore
			if fullBackupTool.Spec.Physical.GetPhysicalRestoreCommand() != nil {
				backupJobKeys = append(backupJobKeys, baseBackupPhysicalRestore())
			}

			// do continuous backup physical restore
			if continuousBackupTool.Spec.Physical.GetPhysicalRestoreCommand() != nil {
				backupJobKeys = append(backupJobKeys, continuousPhysicalRestore())
			}
			Expect(DoPITR(ctx, testCtx.Cli, cluster, synthesizedComponent, scheme.Scheme)).Should(Succeed())

			By("when logic PITR jobs are creating after cluster RUNNING")
			Eventually(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cluster), func(fetched *appsv1alpha1.Cluster) {
				fetched.Status.Phase = appsv1alpha1.RunningClusterPhase
			})).Should(Succeed())
			cluster.Status.Phase = appsv1alpha1.RunningClusterPhase

			// do full backup logical restore
			if fullBackupTool.Spec.Logical.GetLogicalRestoreCommand() != nil {
				backupJobKeys = append(backupJobKeys, baseBackupLogicalRestore())
			}

			// do continuous logical restore
			if continuousBackupTool.Spec.Logical.GetLogicalRestoreCommand() != nil {
				backupJobKeys = append(backupJobKeys, continuousLogicalRestore())
			}
			Expect(DoPITR(ctx, testCtx.Cli, cluster, synthesizedComponent, scheme.Scheme)).Should(Succeed())

			By("expect all jobs are cleaned")
			for _, v := range backupJobKeys {
				Eventually(testapps.CheckObjExists(&testCtx, v, &batchv1.Job{}, false)).Should(Succeed())
			}
		}

		It("Test PITR restore when only support physical restore for full backup", func() {
			testPITR()
		})

		It("Test PITR restore when only support physical logical for full backup", func() {
			Expect(testapps.ChangeObj(&testCtx, fullBackupTool, func(tool *dpv1alpha1.BackupTool) {
				fullBackupTool.Spec.Logical.RestoreCommands = fullBackupTool.Spec.Physical.GetPhysicalRestoreCommand()
				fullBackupTool.Spec.Physical.RestoreCommands = nil
			})).Should(Succeed())
			testPITR()
		})
	})
})

func patchBackupStatus(status dpv1alpha1.BackupStatus, key types.NamespacedName) {
	Eventually(testapps.GetAndChangeObjStatus(&testCtx, key, func(fetched *dpv1alpha1.Backup) {
		fetched.Status = status
	})).Should(Succeed())
}
