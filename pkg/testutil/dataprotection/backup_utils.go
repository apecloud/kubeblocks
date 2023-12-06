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

package dataprotection

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	storagev1alpha1 "github.com/apecloud/kubeblocks/apis/storage/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	"github.com/apecloud/kubeblocks/pkg/testutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func NewFakeActionSet(testCtx *testutil.TestContext) *dpv1alpha1.ActionSet {
	as := testapps.CreateCustomizedObj(testCtx, "backup/actionset.yaml",
		&dpv1alpha1.ActionSet{}, testapps.WithName(ActionSetName))
	Eventually(testapps.CheckObj(testCtx, client.ObjectKeyFromObject(as),
		func(g Gomega, as *dpv1alpha1.ActionSet) {
			g.Expect(as.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.AvailablePhase))
		})).Should(Succeed())
	return as
}

func NewFakeBackupPolicy(testCtx *testutil.TestContext,
	change func(backupPolicy *dpv1alpha1.BackupPolicy)) *dpv1alpha1.BackupPolicy {
	bp := NewBackupPolicyFactory(testCtx.DefaultNamespace, BackupPolicyName).
		SetBackupRepoName(BackupRepoName).
		SetTarget(constant.AppInstanceLabelKey, ClusterName,
			constant.KBAppComponentLabelKey, ComponentName,
			constant.RoleLabelKey, constant.Leader).
		SetPathPrefix(BackupPathPrefix).
		SetTargetConnectionCredential(ClusterName).
		AddBackupMethod(BackupMethodName, false, ActionSetName).
		SetBackupMethodVolumeMounts(DataVolumeName, DataVolumeMountPath,
			LogVolumeName, LogVolumeMountPath).
		AddBackupMethod(VSBackupMethodName, true, "").
		SetBackupMethodVolumes([]string{DataVolumeName}).
		Apply(change).
		Create(testCtx).GetObject()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClusterName,
			Namespace: testCtx.DefaultNamespace,
		},
		StringData: map[string]string{
			"password": "test-passw0rd",
		},
	}
	Expect(testCtx.CreateObj(testCtx.Ctx, secret)).Should(Succeed())
	Eventually(testapps.CheckObj(testCtx, client.ObjectKeyFromObject(bp),
		func(g Gomega, bp *dpv1alpha1.BackupPolicy) {
			g.Expect(bp.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.AvailablePhase))
		})).Should(Succeed())
	return bp
}

func NewFakeStorageProvider(testCtx *testutil.TestContext,
	change func(sp *storagev1alpha1.StorageProvider)) *storagev1alpha1.StorageProvider {
	sp := testapps.CreateCustomizedObj(testCtx, "backup/storageprovider.yaml",
		&storagev1alpha1.StorageProvider{}, func(obj *storagev1alpha1.StorageProvider) {
			obj.Name = StorageProviderName
			if change != nil {
				change(obj)
			}
		})
	// the storage provider controller is not running, so set the status manually
	Expect(testapps.ChangeObjStatus(testCtx, sp, func() {
		sp.Status.Phase = storagev1alpha1.StorageProviderReady
		meta.SetStatusCondition(&sp.Status.Conditions, metav1.Condition{
			Type:   storagev1alpha1.ConditionTypeCSIDriverInstalled,
			Status: metav1.ConditionTrue,
			Reason: "CSIDriverInstalled",
		})
	})).Should(Succeed())
	return sp
}

func NewFakeBackupRepo(testCtx *testutil.TestContext,
	change func(repo *dpv1alpha1.BackupRepo)) (*dpv1alpha1.BackupRepo, string) {
	repo := testapps.CreateCustomizedObj(testCtx, "backup/backuprepo.yaml",
		&dpv1alpha1.BackupRepo{}, func(obj *dpv1alpha1.BackupRepo) {
			obj.Name = BackupRepoName
			obj.Spec.StorageProviderRef = StorageProviderName
			if change != nil {
				change(obj)
			}
		})
	jobName := fmt.Sprintf("pre-check-%s-%s", repo.UID[:8], repo.Name)
	namespace := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	Eventually(testapps.GetAndChangeObjStatus(testCtx, types.NamespacedName{Name: jobName, Namespace: namespace},
		func(job *batchv1.Job) {
			job.Status.Conditions = append(job.Status.Conditions, batchv1.JobCondition{
				Type:   batchv1.JobComplete,
				Status: corev1.ConditionTrue,
			})
		})).Should(Succeed())
	var name string
	Eventually(testapps.CheckObj(testCtx, client.ObjectKeyFromObject(repo),
		func(g Gomega, repo *dpv1alpha1.BackupRepo) {
			g.Expect(repo.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.BackupRepoReady))
			g.Expect(repo.Status.BackupPVCName).ShouldNot(BeEmpty())
			name = repo.Status.BackupPVCName
		})).Should(Succeed())
	return repo, name
}

func NewFakeBackup(testCtx *testutil.TestContext,
	change func(backup *dpv1alpha1.Backup)) *dpv1alpha1.Backup {
	if change == nil {
		change = func(*dpv1alpha1.Backup) {} // set nop
	}
	backup := NewBackupFactory(testCtx.DefaultNamespace, BackupName).
		SetBackupPolicyName(BackupPolicyName).
		SetBackupMethod(BackupMethodName).
		Apply(change).
		Create(testCtx).GetObject()
	return backup
}

func NewFakeCluster(testCtx *testutil.TestContext) *BackupClusterInfo {
	createPVCAndPV := func(name string) *corev1.PersistentVolumeClaim {
		pvName := "pv-" + name
		pvc := testapps.NewPersistentVolumeClaimFactory(
			testCtx.DefaultNamespace, name, ClusterName, ComponentName, "data").
			SetVolumeName(pvName).
			SetStorage("1Gi").
			SetStorageClass(StorageClassName).
			Create(testCtx).GetObject()

		testapps.NewPersistentVolumeFactory(testCtx.DefaultNamespace, pvName, name).
			SetStorage("1Gi").
			SetClaimRef(pvc).SetCSIDriver(testutil.DefaultCSIDriver).Create(testCtx)
		return pvc
	}

	podFactory := func(name string) *testapps.MockPodFactory {
		return testapps.NewPodFactory(testCtx.DefaultNamespace, name).
			AddAppInstanceLabel(ClusterName).
			AddAppComponentLabel(ComponentName).
			AddContainer(corev1.Container{Name: ContainerName, Image: testapps.ApeCloudMySQLImage})
	}

	By("mocking a cluster")
	cluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, ClusterName,
		"test-cd", "test-cv").
		AddLabels(constant.AppInstanceLabelKey, ClusterName).
		Create(testCtx).GetObject()
	podName := ClusterName + "-" + ComponentName

	By("mocking a storage class")
	_ = testapps.CreateStorageClass(testCtx, StorageClassName, true)

	By("mocking a pvc belonging to the pod 0")
	pvc := createPVCAndPV("data-" + podName + "-0")

	By("mocking a pvc belonging to the pod 1")
	pvc1 := createPVCAndPV("data-" + podName + "-1")

	By("mocking pod 0 belonging to the statefulset")
	volume := corev1.Volume{Name: DataVolumeName, VolumeSource: corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}
	pod := podFactory(podName + "-0").
		AddRoleLabel("leader").
		AddVolume(volume).
		Create(testCtx).GetObject()

	By("mocking pod 1 belonging to the statefulset")
	volume2 := corev1.Volume{Name: DataVolumeName, VolumeSource: corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc1.Name}}}
	_ = podFactory(podName + "-1").
		AddVolume(volume2).
		Create(testCtx).GetObject()

	return &BackupClusterInfo{
		Cluster:   cluster,
		TargetPod: pod,
		TargetPVC: pvc.Name,
	}
}

func NewFakeBackupSchedule(testCtx *testutil.TestContext,
	change func(schedule *dpv1alpha1.BackupSchedule)) *dpv1alpha1.BackupSchedule {
	schedule := NewBackupScheduleFactory(testCtx.DefaultNamespace, BackupScheduleName).
		SetBackupPolicyName(BackupPolicyName).
		SetStartingDeadlineMinutes(StartingDeadlineMinutes).
		AddSchedulePolicy(dpv1alpha1.SchedulePolicy{
			Enabled:         boolptr.False(),
			BackupMethod:    BackupMethodName,
			CronExpression:  BackupScheduleCron,
			RetentionPeriod: BackupRetention,
		}).
		AddSchedulePolicy(dpv1alpha1.SchedulePolicy{
			Enabled:         boolptr.False(),
			BackupMethod:    VSBackupMethodName,
			CronExpression:  BackupScheduleCron,
			RetentionPeriod: BackupRetention,
		}).
		Apply(change).
		Create(testCtx).GetObject()
	return schedule
}

// EnableBackupSchedule enables the backup schedule that matches the given method.
func EnableBackupSchedule(testCtx *testutil.TestContext,
	backupSchedule *dpv1alpha1.BackupSchedule, method string) {
	Eventually(testapps.ChangeObj(testCtx, backupSchedule, func(schedule *dpv1alpha1.BackupSchedule) {
		for i := range schedule.Spec.Schedules {
			if schedule.Spec.Schedules[i].BackupMethod == method {
				schedule.Spec.Schedules[i].Enabled = boolptr.True()
				break
			}
		}
	})).Should(Succeed())
}

func MockBackupStatusMethod(backup *dpv1alpha1.Backup, backupMethodName, targetVolume, actionSetName string) {
	var snapshot bool
	if backupMethodName == VSBackupMethodName {
		snapshot = true
	}
	backup.Status.BackupMethod = &dpv1alpha1.BackupMethod{
		Name:            backupMethodName,
		SnapshotVolumes: &snapshot,
		ActionSetName:   actionSetName,
		TargetVolumes: &dpv1alpha1.TargetVolumeInfo{
			Volumes: []string{targetVolume},
			VolumeMounts: []corev1.VolumeMount{
				{Name: targetVolume, MountPath: "/"},
			},
		},
	}
}
