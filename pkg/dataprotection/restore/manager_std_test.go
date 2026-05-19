/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package restore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func managerTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = batchv1.AddToScheme(s)
	_ = dpv1alpha1.AddToScheme(s)
	_ = vsv1.AddToScheme(s)
	return s
}

func newMgrReqCtx() intctrlutil.RequestCtx {
	return intctrlutil.RequestCtx{
		Ctx: context.Background(),
		Req: reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "test"}},
	}
}

func newManagerTestRestore() *dpv1alpha1.Restore {
	return &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-restore",
			Namespace: "default",
			UID:       types.UID("12345678-abcd-efgh-ijkl-123456789012"),
		},
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:      "test-backup",
				Namespace: "default",
			},
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{},
		},
		Status: dpv1alpha1.RestoreStatus{
			Actions: dpv1alpha1.RestoreStatusActions{},
		},
	}
}

func newTestRestoreManager(objs ...client.Object) *RestoreManager {
	scheme := managerTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	restore := newManagerTestRestore()
	return &RestoreManager{
		OriginalRestore:       restore.DeepCopy(),
		Restore:               restore,
		PrepareDataBackupSets: []BackupActionSet{},
		PostReadyBackupSets:   []BackupActionSet{},
		Schema:                scheme,
		Recorder:              record.NewFakeRecorder(20),
		Client:                fakeClient,
	}
}

func completedBackup(name string) *dpv1alpha1.Backup {
	now := metav1.Now()
	return &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Status: dpv1alpha1.BackupStatus{
			Phase:               dpv1alpha1.BackupPhaseCompleted,
			StartTimestamp:      &metav1.Time{Time: now.Add(-1 * time.Hour)},
			CompletionTimestamp: &now,
			BackupMethod: &dpv1alpha1.BackupMethod{
				Name:          "test-method",
				ActionSetName: "test-actionset",
			},
		},
	}
}

func testActionSet() *dpv1alpha1.ActionSet {
	return &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-actionset",
		},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeFull,
			Restore: &dpv1alpha1.RestoreActionSpec{
				PrepareData: &dpv1alpha1.JobActionSpec{
					BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{
						Image:   "restore-image:latest",
						Command: []string{"restore.sh"},
					},
				},
			},
		},
	}
}

// --- NewRestoreManager ---

func TestNewRestoreManager(t *testing.T) {
	scheme := managerTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	restore := newManagerTestRestore()
	recorder := record.NewFakeRecorder(10)

	mgr := NewRestoreManager(restore, recorder, scheme, fakeClient)
	require.NotNil(t, mgr)
	assert.Equal(t, restore, mgr.Restore)
	assert.NotNil(t, mgr.OriginalRestore)
	assert.True(t, mgr.OriginalRestore != restore) // DeepCopy produces a different pointer
	assert.Empty(t, mgr.PrepareDataBackupSets)
	assert.Empty(t, mgr.PostReadyBackupSets)
}

// --- GetBackupActionSetByNamespaced ---

func TestGetBackupActionSetByNamespaced_Success(t *testing.T) {
	backup := completedBackup("my-backup")
	actionSet := testActionSet()
	mgr := newTestRestoreManager(backup, actionSet)

	reqCtx := newMgrReqCtx()
	result, err := mgr.GetBackupActionSetByNamespaced(reqCtx, mgr.Client, "my-backup", "default")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "my-backup", result.Backup.Name)
	assert.Equal(t, "test-actionset", result.ActionSet.Name)
	assert.False(t, result.UseVolumeSnapshot)
}

func TestGetBackupActionSetByNamespaced_NotFound(t *testing.T) {
	mgr := newTestRestoreManager()
	reqCtx := newMgrReqCtx()
	_, err := mgr.GetBackupActionSetByNamespaced(reqCtx, mgr.Client, "nonexistent", "default")
	require.Error(t, err)
}

func TestGetBackupActionSetByNamespaced_NilBackupMethod(t *testing.T) {
	backup := completedBackup("my-backup")
	backup.Status.BackupMethod = nil
	mgr := newTestRestoreManager(backup)

	reqCtx := newMgrReqCtx()
	_, err := mgr.GetBackupActionSetByNamespaced(reqCtx, mgr.Client, "my-backup", "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status.backupMethod")
}

func TestGetBackupActionSetByNamespaced_WithVolumeSnapshot(t *testing.T) {
	snapVols := true
	backup := completedBackup("snap-backup")
	backup.Status.BackupMethod.SnapshotVolumes = &snapVols
	actionSet := testActionSet()
	mgr := newTestRestoreManager(backup, actionSet)

	reqCtx := newMgrReqCtx()
	result, err := mgr.GetBackupActionSetByNamespaced(reqCtx, mgr.Client, "snap-backup", "default")
	require.NoError(t, err)
	assert.True(t, result.UseVolumeSnapshot)
}

// --- SetBackupSets ---

func TestSetBackupSets_PrepareData(t *testing.T) {
	mgr := newTestRestoreManager()
	backupSet := BackupActionSet{
		Backup:    completedBackup("b1"),
		ActionSet: testActionSet(),
	}
	mgr.SetBackupSets(backupSet)
	assert.Len(t, mgr.PrepareDataBackupSets, 1)
	assert.Empty(t, mgr.PostReadyBackupSets)
}

func TestSetBackupSets_PostReady(t *testing.T) {
	mgr := newTestRestoreManager()
	as := testActionSet()
	as.Spec.Restore.PrepareData = nil
	as.Spec.Restore.PostReady = []dpv1alpha1.ActionSpec{
		{Job: &dpv1alpha1.JobActionSpec{BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img", Command: []string{"cmd"}}}},
	}
	backupSet := BackupActionSet{
		Backup:    completedBackup("b1"),
		ActionSet: as,
	}
	mgr.SetBackupSets(backupSet)
	assert.Empty(t, mgr.PrepareDataBackupSets)
	assert.Len(t, mgr.PostReadyBackupSets, 1)
}

func TestSetBackupSets_VolumeSnapshot(t *testing.T) {
	mgr := newTestRestoreManager()
	backupSet := BackupActionSet{
		Backup:            completedBackup("b1"),
		ActionSet:         testActionSet(),
		UseVolumeSnapshot: true,
	}
	mgr.SetBackupSets(backupSet)
	assert.Len(t, mgr.PrepareDataBackupSets, 1)
}

func TestSetBackupSets_NilActionSet(t *testing.T) {
	mgr := newTestRestoreManager()
	backupSet := BackupActionSet{
		Backup:    completedBackup("b1"),
		ActionSet: nil,
	}
	mgr.SetBackupSets(backupSet)
	assert.Empty(t, mgr.PrepareDataBackupSets)
	assert.Empty(t, mgr.PostReadyBackupSets)
}

func TestSetBackupSets_NilRestore(t *testing.T) {
	mgr := newTestRestoreManager()
	as := testActionSet()
	as.Spec.Restore = nil
	backupSet := BackupActionSet{
		Backup:    completedBackup("b1"),
		ActionSet: as,
	}
	mgr.SetBackupSets(backupSet)
	assert.Empty(t, mgr.PrepareDataBackupSets)
	assert.Empty(t, mgr.PostReadyBackupSets)
}

func TestSetBackupSets_BothPrepareAndPostReady(t *testing.T) {
	mgr := newTestRestoreManager()
	as := testActionSet()
	as.Spec.Restore.PostReady = []dpv1alpha1.ActionSpec{
		{Job: &dpv1alpha1.JobActionSpec{BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img", Command: []string{"cmd"}}}},
	}
	backupSet := BackupActionSet{
		Backup:    completedBackup("b1"),
		ActionSet: as,
	}
	mgr.SetBackupSets(backupSet)
	assert.Len(t, mgr.PrepareDataBackupSets, 1)
	assert.Len(t, mgr.PostReadyBackupSets, 1)
}

// --- AnalysisRestoreActionsWithBackup ---

func TestAnalysisRestoreActionsWithBackup_AllCompleted(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
		RestoreVolumeClaims: []dpv1alpha1.RestoreVolumeClaim{
			{ObjectMeta: metav1.ObjectMeta{Name: "pvc-0"}},
		},
	}
	mgr.Restore.Status.Actions.PrepareData = []dpv1alpha1.RestoreStatusAction{
		{Name: "action-1", BackupName: "b1", Status: dpv1alpha1.RestoreActionCompleted},
	}
	allDone, hasFailed := mgr.AnalysisRestoreActionsWithBackup(dpv1alpha1.PrepareData, "b1", "action-1")
	assert.True(t, allDone)
	assert.False(t, hasFailed)
}

func TestAnalysisRestoreActionsWithBackup_HasFailed(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
		RestoreVolumeClaims: []dpv1alpha1.RestoreVolumeClaim{
			{ObjectMeta: metav1.ObjectMeta{Name: "pvc-0"}},
		},
	}
	mgr.Restore.Status.Actions.PrepareData = []dpv1alpha1.RestoreStatusAction{
		{Name: "action-1", BackupName: "b1", Status: dpv1alpha1.RestoreActionFailed},
	}
	allDone, hasFailed := mgr.AnalysisRestoreActionsWithBackup(dpv1alpha1.PrepareData, "b1", "action-1")
	assert.True(t, allDone)
	assert.True(t, hasFailed)
}

func TestAnalysisRestoreActionsWithBackup_Processing(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
		RestoreVolumeClaimsTemplate: &dpv1alpha1.RestoreVolumeClaimsTemplate{
			Replicas: 3,
		},
	}
	mgr.Restore.Status.Actions.PrepareData = []dpv1alpha1.RestoreStatusAction{
		{Name: "action-1", BackupName: "b1", Status: dpv1alpha1.RestoreActionCompleted},
		{Name: "action-1", BackupName: "b1", Status: dpv1alpha1.RestoreActionProcessing},
	}
	allDone, hasFailed := mgr.AnalysisRestoreActionsWithBackup(dpv1alpha1.PrepareData, "b1", "action-1")
	assert.False(t, allDone)
	assert.False(t, hasFailed)
}

func TestAnalysisRestoreActionsWithBackup_PostReady(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Status.Actions.PostReady = []dpv1alpha1.RestoreStatusAction{
		{Name: "action-1", BackupName: "b1", Status: dpv1alpha1.RestoreActionCompleted},
		{Name: "action-1", BackupName: "b1", Status: dpv1alpha1.RestoreActionCompleted},
	}
	allDone, hasFailed := mgr.AnalysisRestoreActionsWithBackup(dpv1alpha1.PostReady, "b1", "action-1")
	assert.True(t, allDone)
	assert.False(t, hasFailed)
}

func TestAnalysisRestoreActionsWithBackup_FiltersByBackupAndAction(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Status.Actions.PostReady = []dpv1alpha1.RestoreStatusAction{
		{Name: "action-1", BackupName: "b1", Status: dpv1alpha1.RestoreActionCompleted},
		{Name: "action-2", BackupName: "b1", Status: dpv1alpha1.RestoreActionProcessing},
	}
	allDone, _ := mgr.AnalysisRestoreActionsWithBackup(dpv1alpha1.PostReady, "b1", "action-1")
	assert.True(t, allDone)
}

// --- addItsManagingLabels ---

func TestAddItsManagingLabels_WithClusterAndComp(t *testing.T) {
	claim := &dpv1alpha1.RestoreVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pvc-0",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    "mycluster",
				constant.KBAppComponentLabelKey: "mycomp",
			},
		},
	}
	addItsManagingLabels(claim, 0)
	assert.Equal(t, "mycluster-mycomp-0", claim.Labels[constant.KBAppPodNameLabelKey])
}

func TestAddItsManagingLabels_WithTemplateLabel(t *testing.T) {
	claim := &dpv1alpha1.RestoreVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pvc-0",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:            "mycluster",
				constant.KBAppComponentLabelKey:         "mycomp",
				constant.KBAppInstanceTemplateLabelKey: "tpl",
			},
		},
	}
	addItsManagingLabels(claim, 2)
	assert.Equal(t, "mycluster-mycomp-tpl-2", claim.Labels[constant.KBAppPodNameLabelKey])
}

func TestAddItsManagingLabels_NoCluster(t *testing.T) {
	claim := &dpv1alpha1.RestoreVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pvc-0",
			Labels: map[string]string{
				constant.KBAppComponentLabelKey: "mycomp",
			},
		},
	}
	addItsManagingLabels(claim, 0)
	// should not set pod name label since cluster is empty
	_, has := claim.Labels[constant.KBAppPodNameLabelKey]
	assert.False(t, has)
}

func TestAddItsManagingLabels_ExistingPodName(t *testing.T) {
	claim := &dpv1alpha1.RestoreVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pvc-0",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    "mycluster",
				constant.KBAppComponentLabelKey: "mycomp",
				constant.KBAppPodNameLabelKey:   "existing-pod",
			},
		},
	}
	addItsManagingLabels(claim, 0)
	assert.Equal(t, "existing-pod", claim.Labels[constant.KBAppPodNameLabelKey])
}

// --- prepareBackupRepo ---

func TestPrepareBackupRepo_NoRepoName(t *testing.T) {
	mgr := newTestRestoreManager()
	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{
		Backup: completedBackup("b1"),
	}
	backupSet.Backup.Status.BackupRepoName = ""
	repo, err := mgr.prepareBackupRepo(reqCtx, mgr.Client, backupSet)
	require.NoError(t, err)
	assert.Nil(t, repo)
}

func TestPrepareBackupRepo_WithRepoName(t *testing.T) {
	backupRepo := &dpv1alpha1.BackupRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-repo",
		},
	}
	mgr := newTestRestoreManager(backupRepo)
	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{
		Backup: completedBackup("b1"),
	}
	backupSet.Backup.Status.BackupRepoName = "my-repo"
	repo, err := mgr.prepareBackupRepo(reqCtx, mgr.Client, backupSet)
	require.NoError(t, err)
	require.NotNil(t, repo)
	assert.Equal(t, "my-repo", repo.Name)
}

func TestPrepareBackupRepo_RepoNotFound(t *testing.T) {
	mgr := newTestRestoreManager()
	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{
		Backup: completedBackup("b1"),
	}
	backupSet.Backup.Status.BackupRepoName = "nonexistent-repo"
	_, err := mgr.prepareBackupRepo(reqCtx, mgr.Client, backupSet)
	require.Error(t, err)
}

// --- createPVCIfNotExist ---

func TestCreatePVCIfNotExist_Creates(t *testing.T) {
	mgr := newTestRestoreManager()
	reqCtx := newMgrReqCtx()
	meta := metav1.ObjectMeta{Name: "new-pvc", Namespace: "default"}
	spec := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
	}
	err := mgr.createPVCIfNotExist(reqCtx, mgr.Client, meta, spec)
	require.NoError(t, err)

	// verify PVC exists
	pvc := &corev1.PersistentVolumeClaim{}
	err = mgr.Client.Get(context.Background(), types.NamespacedName{Name: "new-pvc", Namespace: "default"}, pvc)
	require.NoError(t, err)
	assert.Equal(t, "new-pvc", pvc.Name)
}

func TestCreatePVCIfNotExist_AlreadyExists(t *testing.T) {
	existingPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "existing-pvc", Namespace: "default"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		},
	}
	mgr := newTestRestoreManager(existingPVC)
	reqCtx := newMgrReqCtx()
	meta := metav1.ObjectMeta{Name: "existing-pvc", Namespace: "default"}
	spec := corev1.PersistentVolumeClaimSpec{}
	err := mgr.createPVCIfNotExist(reqCtx, mgr.Client, meta, spec)
	require.NoError(t, err)
}

// --- CreateJobsIfNotExist ---

func TestCreateJobsIfNotExist_CreatesNew(t *testing.T) {
	mgr := newTestRestoreManager()
	reqCtx := newMgrReqCtx()

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-job-1",
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers:    []corev1.Container{{Name: "c", Image: "img"}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	owner := mgr.Restore
	fetched, err := mgr.CreateJobsIfNotExist(reqCtx, mgr.Client, owner, []*batchv1.Job{job})
	require.NoError(t, err)
	assert.Len(t, fetched, 1)
	assert.Equal(t, "restore-job-1", fetched[0].Name)
}

func TestCreateJobsIfNotExist_ExistingJob(t *testing.T) {
	existingJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-job-1",
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers:    []corev1.Container{{Name: "c", Image: "img"}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	mgr := newTestRestoreManager(existingJob)
	reqCtx := newMgrReqCtx()

	newJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-job-1",
			Namespace: "default",
		},
	}
	fetched, err := mgr.CreateJobsIfNotExist(reqCtx, mgr.Client, mgr.Restore, []*batchv1.Job{newJob})
	require.NoError(t, err)
	assert.Len(t, fetched, 1)
}

func TestCreateJobsIfNotExist_NilJobSkipped(t *testing.T) {
	mgr := newTestRestoreManager()
	reqCtx := newMgrReqCtx()
	fetched, err := mgr.CreateJobsIfNotExist(reqCtx, mgr.Client, mgr.Restore, []*batchv1.Job{nil})
	require.NoError(t, err)
	assert.Empty(t, fetched)
}

// --- CheckJobsDone ---

func TestCheckJobsDone_AllCompleted(t *testing.T) {
	mgr := newTestRestoreManager()
	backupSet := BackupActionSet{Backup: completedBackup("b1")}

	completedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "job-1", Namespace: "default"},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}

	allDone, hasFailed, err := mgr.CheckJobsDone(dpv1alpha1.PrepareData, "action-1", backupSet, []*batchv1.Job{completedJob})
	require.NoError(t, err)
	assert.True(t, allDone)
	assert.False(t, hasFailed)

	// verify status action recorded
	assert.Len(t, mgr.Restore.Status.Actions.PrepareData, 1)
	assert.Equal(t, dpv1alpha1.RestoreActionCompleted, mgr.Restore.Status.Actions.PrepareData[0].Status)
}

func TestCheckJobsDone_Failed(t *testing.T) {
	mgr := newTestRestoreManager()
	backupSet := BackupActionSet{Backup: completedBackup("b1")}

	failedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "job-1", Namespace: "default"},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded", Message: "too many retries"},
			},
		},
	}

	allDone, hasFailed, err := mgr.CheckJobsDone(dpv1alpha1.PrepareData, "action-1", backupSet, []*batchv1.Job{failedJob})
	require.NoError(t, err)
	assert.True(t, allDone)
	assert.True(t, hasFailed)
}

func TestCheckJobsDone_Processing(t *testing.T) {
	mgr := newTestRestoreManager()
	backupSet := BackupActionSet{Backup: completedBackup("b1")}

	runningJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "job-1", Namespace: "default"},
		Status:     batchv1.JobStatus{},
	}

	allDone, hasFailed, err := mgr.CheckJobsDone(dpv1alpha1.PrepareData, "action-1", backupSet, []*batchv1.Job{runningJob})
	require.NoError(t, err)
	assert.False(t, allDone)
	assert.False(t, hasFailed)
}

func TestCheckJobsDone_PostReady(t *testing.T) {
	mgr := newTestRestoreManager()
	backupSet := BackupActionSet{Backup: completedBackup("b1")}

	completedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "job-1", Namespace: "default"},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}

	allDone, hasFailed, err := mgr.CheckJobsDone(dpv1alpha1.PostReady, "action-1", backupSet, []*batchv1.Job{completedJob})
	require.NoError(t, err)
	assert.True(t, allDone)
	assert.False(t, hasFailed)
	assert.Len(t, mgr.Restore.Status.Actions.PostReady, 1)
}

// --- StopManagerContainer ---

func TestStopManagerContainer(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c", Image: "img"}},
		},
	}
	mgr := newTestRestoreManager(pod)

	err := mgr.StopManagerContainer(pod)
	require.NoError(t, err)

	updated := &corev1.Pod{}
	err = mgr.Client.Get(context.Background(), types.NamespacedName{Name: "pod-1", Namespace: "default"}, updated)
	require.NoError(t, err)
	assert.Equal(t, "true", updated.Annotations[DataProtectionStopRestoreManagerAnnotationKey])
}

func TestStopManagerContainer_AlreadyStopped(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "default",
			Annotations: map[string]string{
				DataProtectionStopRestoreManagerAnnotationKey: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c", Image: "img"}},
		},
	}
	mgr := newTestRestoreManager(pod)

	err := mgr.StopManagerContainer(pod)
	require.NoError(t, err)
}

// --- StopManagerContainerByJob ---

func TestStopManagerContainerByJob(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-pod-1",
			Namespace: "default",
			Labels:    map[string]string{"job-name": "restore-job"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c", Image: "img"}},
		},
	}
	mgr := newTestRestoreManager(pod)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "restore-job", Namespace: "default"},
	}
	err := mgr.StopManagerContainerByJob(job)
	require.NoError(t, err)

	updated := &corev1.Pod{}
	err = mgr.Client.Get(context.Background(), types.NamespacedName{Name: "job-pod-1", Namespace: "default"}, updated)
	require.NoError(t, err)
	assert.Equal(t, "true", updated.Annotations[DataProtectionStopRestoreManagerAnnotationKey])
}

// --- CheckIfRestoreContainerTerminated ---

func TestCheckIfRestoreContainerTerminated_NoPods(t *testing.T) {
	mgr := newTestRestoreManager()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "job-1", Namespace: "default"},
	}
	normal, err := mgr.CheckIfRestoreContainerTerminated(job)
	require.NoError(t, err)
	assert.False(t, normal)
}

func TestCheckIfRestoreContainerTerminated_NormalTermination(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-pod",
			Namespace: "default",
			Labels:    map[string]string{"job-name": "job-1"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: Restore, Image: "img"}},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: Restore,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
					},
				},
			},
		},
	}
	mgr := newTestRestoreManager(pod)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "job-1", Namespace: "default"},
	}
	normal, err := mgr.CheckIfRestoreContainerTerminated(job)
	require.NoError(t, err)
	assert.True(t, normal)
}

func TestCheckIfRestoreContainerTerminated_AbnormalTermination(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-pod",
			Namespace: "default",
			Labels:    map[string]string{"job-name": "job-1"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: Restore, Image: "img"}},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: Restore,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
					},
				},
			},
		},
	}
	mgr := newTestRestoreManager(pod)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "job-1", Namespace: "default"},
	}
	normal, err := mgr.CheckIfRestoreContainerTerminated(job)
	require.NoError(t, err)
	assert.False(t, normal)

	// verify manager container was stopped
	updated := &corev1.Pod{}
	err = mgr.Client.Get(context.Background(), types.NamespacedName{Name: "job-pod", Namespace: "default"}, updated)
	require.NoError(t, err)
	assert.Equal(t, "true", updated.Annotations[DataProtectionStopRestoreManagerAnnotationKey])
}

// --- Recalculation ---

func TestRecalculation_ParallelPolicy_NoOp(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
		VolumeClaimRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicyParallel,
	}
	allFinished := true
	existFailed := false
	mgr.Recalculation("b1", "action-1", &allFinished, &existFailed)
	assert.True(t, allFinished) // unchanged
}

func TestRecalculation_SerialPolicy_FailedShortCircuits(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
		VolumeClaimRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicySerial,
	}
	allFinished := false
	existFailed := true
	mgr.Recalculation("b1", "action-1", &allFinished, &existFailed)
	assert.True(t, allFinished)
}

func TestRecalculation_SerialPolicy_NotAllActionsYet(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
		VolumeClaimRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicySerial,
		RestoreVolumeClaimsTemplate: &dpv1alpha1.RestoreVolumeClaimsTemplate{
			Replicas: 3,
		},
	}
	mgr.Restore.Status.Actions.PrepareData = []dpv1alpha1.RestoreStatusAction{
		{Name: "action-1", BackupName: "b1", Status: dpv1alpha1.RestoreActionCompleted},
	}
	allFinished := true
	existFailed := false
	mgr.Recalculation("b1", "action-1", &allFinished, &existFailed)
	assert.False(t, allFinished)
}

func TestRecalculation_SerialPolicy_AllActionsComplete(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
		VolumeClaimRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicySerial,
		RestoreVolumeClaims: []dpv1alpha1.RestoreVolumeClaim{
			{ObjectMeta: metav1.ObjectMeta{Name: "pvc-0"}},
		},
	}
	mgr.Restore.Status.Actions.PrepareData = []dpv1alpha1.RestoreStatusAction{
		{Name: "action-1", BackupName: "b1", Status: dpv1alpha1.RestoreActionCompleted},
	}
	allFinished := true
	existFailed := false
	mgr.Recalculation("b1", "action-1", &allFinished, &existFailed)
	assert.True(t, allFinished)
}

// --- listCompletedBackups ---

func TestListCompletedBackups_FiltersCompleted(t *testing.T) {
	completedB := completedBackup("b-completed")
	completedB.Labels = map[string]string{
		dptypes.BackupTypeLabelKey: string(dpv1alpha1.BackupTypeFull),
		dptypes.BackupPolicyLabelKey: "my-policy",
	}
	failedB := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "b-failed",
			Namespace: "default",
			Labels: map[string]string{
				dptypes.BackupTypeLabelKey: string(dpv1alpha1.BackupTypeFull),
				dptypes.BackupPolicyLabelKey: "my-policy",
			},
		},
		Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseFailed},
	}
	mgr := newTestRestoreManager(completedB, failedB)

	continuousBackup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "continuous",
			Namespace: "default",
		},
		Spec: dpv1alpha1.BackupSpec{
			BackupPolicyName: "my-policy",
		},
	}

	reqCtx := newMgrReqCtx()
	results, err := mgr.listCompletedBackups(reqCtx, mgr.Client, continuousBackup, dpv1alpha1.BackupTypeFull)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "b-completed", results[0].Name)
}

func TestListCompletedBackups_WithClusterUIDLabel(t *testing.T) {
	backup := completedBackup("b-with-uid")
	backup.Labels = map[string]string{
		dptypes.BackupTypeLabelKey: string(dpv1alpha1.BackupTypeFull),
		dptypes.ClusterUIDLabelKey: "uid-123",
	}
	mgr := newTestRestoreManager(backup)

	continuousBackup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "continuous",
			Namespace: "default",
			Labels: map[string]string{
				dptypes.ClusterUIDLabelKey: "uid-123",
			},
		},
		Spec: dpv1alpha1.BackupSpec{
			BackupPolicyName: "my-policy",
		},
	}

	reqCtx := newMgrReqCtx()
	results, err := mgr.listCompletedBackups(reqCtx, mgr.Client, continuousBackup, dpv1alpha1.BackupTypeFull)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

// --- BuildPrepareDataJobs ---

func TestBuildPrepareDataJobs_NilConfig(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.PrepareDataConfig = nil
	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{
		Backup:    completedBackup("b1"),
		ActionSet: testActionSet(),
	}
	jobs, err := mgr.BuildPrepareDataJobs(reqCtx, mgr.Client, backupSet, &dpv1alpha1.BackupStatusTarget{}, "action-1")
	require.NoError(t, err)
	assert.Nil(t, jobs)
}

func TestBuildPrepareDataJobs_NoPrepareDataStage(t *testing.T) {
	mgr := newTestRestoreManager()
	reqCtx := newMgrReqCtx()
	as := testActionSet()
	as.Spec.Restore.PrepareData = nil
	backupSet := BackupActionSet{
		Backup:    completedBackup("b1"),
		ActionSet: as,
	}
	jobs, err := mgr.BuildPrepareDataJobs(reqCtx, mgr.Client, backupSet, &dpv1alpha1.BackupStatusTarget{}, "action-1")
	require.NoError(t, err)
	assert.Nil(t, jobs)
}

func TestBuildPrepareDataJobs_WithVolumeClaims(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	backup := completedBackup("b1")
	backup.Status.BackupMethod.TargetVolumes = &dpv1alpha1.TargetVolumeInfo{
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
	}
	as := testActionSet()
	mgr := newTestRestoreManager(backup, as)
	mgr.Restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
		RestoreVolumeClaims: []dpv1alpha1.RestoreVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pvc-0", Namespace: "default"},
				VolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				},
				VolumeConfig: dpv1alpha1.VolumeConfig{MountPath: "/data"},
			},
		},
	}
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{
				Strategy: dpv1alpha1.PodSelectionStrategyAny,
			},
		},
	}

	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{
		Backup:    backup,
		ActionSet: as,
	}
	jobs, err := mgr.BuildPrepareDataJobs(reqCtx, mgr.Client, backupSet, target, "action-1")
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Contains(t, jobs[0].Name, "restore-preparedata")
}

// --- BuildVolumePopulateJob ---

func TestBuildVolumePopulateJob_NilConfig(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.PrepareDataConfig = nil
	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{
		Backup:    completedBackup("b1"),
		ActionSet: testActionSet(),
	}
	job, err := mgr.BuildVolumePopulateJob(reqCtx, mgr.Client, backupSet, &dpv1alpha1.BackupStatusTarget{}, &corev1.PersistentVolumeClaim{}, 0)
	require.NoError(t, err)
	assert.Nil(t, job)
}

func TestBuildVolumePopulateJob_NilDataSourceRef(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{}
	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{
		Backup:    completedBackup("b1"),
		ActionSet: testActionSet(),
	}
	job, err := mgr.BuildVolumePopulateJob(reqCtx, mgr.Client, backupSet, &dpv1alpha1.BackupStatusTarget{}, &corev1.PersistentVolumeClaim{}, 0)
	require.NoError(t, err)
	assert.Nil(t, job)
}

// --- BuildPostReadyActionJobs ---

func TestBuildPostReadyActionJobs_NilReadyConfig(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.ReadyConfig = nil
	reqCtx := newMgrReqCtx()
	as := testActionSet()
	as.Spec.Restore.PostReady = []dpv1alpha1.ActionSpec{
		{Job: &dpv1alpha1.JobActionSpec{BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img", Command: []string{"cmd"}}}},
	}
	backupSet := BackupActionSet{Backup: completedBackup("b1"), ActionSet: as}
	jobs, err := mgr.BuildPostReadyActionJobs(reqCtx, mgr.Client, backupSet, &dpv1alpha1.BackupStatusTarget{}, 0)
	require.NoError(t, err)
	assert.Nil(t, jobs)
}

func TestBuildPostReadyActionJobs_NoPostReadyStage(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.ReadyConfig = &dpv1alpha1.ReadyConfig{}
	reqCtx := newMgrReqCtx()
	as := testActionSet()
	as.Spec.Restore.PostReady = nil
	backupSet := BackupActionSet{Backup: completedBackup("b1"), ActionSet: as}
	jobs, err := mgr.BuildPostReadyActionJobs(reqCtx, mgr.Client, backupSet, &dpv1alpha1.BackupStatusTarget{}, 0)
	require.NoError(t, err)
	assert.Nil(t, jobs)
}

// --- BuildDifferentialBackupActionSets ---

func TestBuildDifferentialBackupActionSets_Success(t *testing.T) {
	parentBackup := completedBackup("parent-backup")
	parentBackup.Status.BackupMethod.ActionSetName = "test-actionset"
	as := testActionSet()
	mgr := newTestRestoreManager(parentBackup, as)

	sourceBackupSet := BackupActionSet{
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{Name: "child-backup", Namespace: "default"},
			Spec:       dpv1alpha1.BackupSpec{ParentBackupName: "parent-backup"},
			Status: dpv1alpha1.BackupStatus{
				Phase: dpv1alpha1.BackupPhaseCompleted,
				BackupMethod: &dpv1alpha1.BackupMethod{
					Name:          "method",
					ActionSetName: "test-actionset",
				},
			},
		},
		ActionSet: as,
	}

	reqCtx := newMgrReqCtx()
	err := mgr.BuildDifferentialBackupActionSets(reqCtx, mgr.Client, sourceBackupSet)
	require.NoError(t, err)
	assert.Len(t, mgr.PrepareDataBackupSets, 2) // parent + child
}

// --- BuildContinuousRestoreManager ---

func TestBuildContinuousRestoreManager_InvalidTime(t *testing.T) {
	mgr := newTestRestoreManager()
	now := metav1.Now()
	continuousBackup := completedBackup("continuous")
	continuousBackup.Status.StartTimestamp = &metav1.Time{Time: now.Add(-2 * time.Hour)}
	continuousBackup.Status.CompletionTimestamp = &metav1.Time{Time: now.Add(-1 * time.Hour)}

	// restoreTime is outside the range
	mgr.Restore.Spec.RestoreTime = now.Format(time.RFC3339)

	as := testActionSet()
	as.Spec.BackupType = dpv1alpha1.BackupTypeContinuous
	backupSet := BackupActionSet{Backup: continuousBackup, ActionSet: as}

	reqCtx := newMgrReqCtx()
	err := mgr.BuildContinuousRestoreManager(reqCtx, mgr.Client, backupSet)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restore time out of the range")
}

func TestBuildContinuousRestoreManager_EmptyTimestamps(t *testing.T) {
	mgr := newTestRestoreManager()
	continuousBackup := completedBackup("continuous")
	continuousBackup.Status.StartTimestamp = nil
	continuousBackup.Status.CompletionTimestamp = nil

	mgr.Restore.Spec.RestoreTime = time.Now().Format(time.RFC3339)

	as := testActionSet()
	backupSet := BackupActionSet{Backup: continuousBackup, ActionSet: as}

	reqCtx := newMgrReqCtx()
	err := mgr.BuildContinuousRestoreManager(reqCtx, mgr.Client, backupSet)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "startTimeStamp or completeTimeStamp")
}

// --- RestorePVCFromSnapshot ---

func TestRestorePVCFromSnapshot_NilConfig(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.PrepareDataConfig = nil
	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{Backup: completedBackup("b1")}
	err := mgr.RestorePVCFromSnapshot(reqCtx, mgr.Client, backupSet, &dpv1alpha1.BackupStatusTarget{})
	require.NoError(t, err)
}

func TestRestorePVCFromSnapshot_EmptyVolumeSource(t *testing.T) {
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
		RestoreVolumeClaims: []dpv1alpha1.RestoreVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pvc-0", Namespace: "default"},
				VolumeConfig: dpv1alpha1.VolumeConfig{},
			},
		},
	}
	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{
		Backup:            completedBackup("b1"),
		UseVolumeSnapshot: true,
	}
	err := mgr.RestorePVCFromSnapshot(reqCtx, mgr.Client, backupSet, &dpv1alpha1.BackupStatusTarget{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "volumeSource can not be empty")
}

func TestRestorePVCFromSnapshot_WithVolumeSource_AnyStrategy(t *testing.T) {
	backup := completedBackup("b1")
	backup.Status.Actions = []dpv1alpha1.ActionStatus{
		{
			TargetPodName: "pod-0",
			VolumeSnapshots: []dpv1alpha1.VolumeSnapshotStatus{
				{VolumeName: "data", Name: "snap-data"},
			},
		},
	}
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
		RestoreVolumeClaims: []dpv1alpha1.RestoreVolumeClaim{
			{
				ObjectMeta:      metav1.ObjectMeta{Name: "pvc-0", Namespace: "default"},
				VolumeClaimSpec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}},
				VolumeConfig:    dpv1alpha1.VolumeConfig{VolumeSource: "data"},
			},
		},
	}
	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{
		Backup:            backup,
		UseVolumeSnapshot: true,
	}
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{
				Strategy: dpv1alpha1.PodSelectionStrategyAny,
			},
		},
	}
	err := mgr.RestorePVCFromSnapshot(reqCtx, mgr.Client, backupSet, target)
	require.NoError(t, err)

	// verify PVC was created with snapshot data source
	pvc := &corev1.PersistentVolumeClaim{}
	err = mgr.Client.Get(context.Background(), types.NamespacedName{Name: "pvc-0", Namespace: "default"}, pvc)
	require.NoError(t, err)
	require.NotNil(t, pvc.Spec.DataSource)
	assert.Equal(t, "snap-data", pvc.Spec.DataSource.Name)
}

func TestRestorePVCFromSnapshot_WithTemplate(t *testing.T) {
	backup := completedBackup("b1")
	backup.Status.Actions = []dpv1alpha1.ActionStatus{
		{
			TargetPodName: "pod-0",
			VolumeSnapshots: []dpv1alpha1.VolumeSnapshotStatus{
				{VolumeName: "data", Name: "snap-data"},
			},
		},
	}
	mgr := newTestRestoreManager()
	mgr.Restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
		RestoreVolumeClaimsTemplate: &dpv1alpha1.RestoreVolumeClaimsTemplate{
			Replicas: 1,
			Templates: []dpv1alpha1.RestoreVolumeClaim{
				{
					ObjectMeta:      metav1.ObjectMeta{Name: "data", Namespace: "default"},
					VolumeClaimSpec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}},
					VolumeConfig:    dpv1alpha1.VolumeConfig{VolumeSource: "data"},
				},
			},
		},
	}
	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{
		Backup:            backup,
		UseVolumeSnapshot: true,
	}
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{
				Strategy: dpv1alpha1.PodSelectionStrategyAny,
			},
		},
	}
	err := mgr.RestorePVCFromSnapshot(reqCtx, mgr.Client, backupSet, target)
	require.NoError(t, err)
}

// --- ValidateAndInitRestoreMGR ---

func TestValidateAndInitRestoreMGR_FullBackup(t *testing.T) {
	backup := completedBackup("test-backup")
	as := testActionSet()
	as.Spec.BackupType = dpv1alpha1.BackupTypeFull
	mgr := newTestRestoreManager(backup, as)

	reqCtx := newMgrReqCtx()
	err := ValidateAndInitRestoreMGR(reqCtx, mgr.Client, mgr)
	require.NoError(t, err)
	assert.Len(t, mgr.PrepareDataBackupSets, 1)
}

func TestValidateAndInitRestoreMGR_BackupNotFound(t *testing.T) {
	mgr := newTestRestoreManager()
	reqCtx := newMgrReqCtx()
	err := ValidateAndInitRestoreMGR(reqCtx, mgr.Client, mgr)
	require.Error(t, err)
}

func TestValidateAndInitRestoreMGR_NotCompleted(t *testing.T) {
	backup := completedBackup("test-backup")
	backup.Status.Phase = dpv1alpha1.BackupPhaseRunning
	as := testActionSet()
	mgr := newTestRestoreManager(backup, as)

	reqCtx := newMgrReqCtx()
	err := ValidateAndInitRestoreMGR(reqCtx, mgr.Client, mgr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not completed")
}

func TestValidateAndInitRestoreMGR_EmptyBackupType(t *testing.T) {
	backup := completedBackup("test-backup")
	backup.Status.BackupMethod.ActionSetName = ""
	mgr := newTestRestoreManager(backup)

	reqCtx := newMgrReqCtx()
	err := ValidateAndInitRestoreMGR(reqCtx, mgr.Client, mgr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backup type")
}

func TestValidateAndInitRestoreMGR_DifferentialBackup(t *testing.T) {
	parentBackup := completedBackup("parent-backup")
	parentBackup.Status.BackupMethod.ActionSetName = "diff-actionset"
	childBackup := completedBackup("test-backup")
	childBackup.Spec.ParentBackupName = "parent-backup"
	childBackup.Status.BackupMethod.ActionSetName = "diff-actionset"

	as := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "diff-actionset"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeDifferential,
			Restore: &dpv1alpha1.RestoreActionSpec{
				PrepareData: &dpv1alpha1.JobActionSpec{
					BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img", Command: []string{"cmd"}},
				},
			},
		},
	}
	mgr := newTestRestoreManager(parentBackup, childBackup, as)

	reqCtx := newMgrReqCtx()
	err := ValidateAndInitRestoreMGR(reqCtx, mgr.Client, mgr)
	require.NoError(t, err)
	assert.Len(t, mgr.PrepareDataBackupSets, 2)
}

// --- BuildIncrementalBackupActionSet ---

func TestBuildIncrementalBackupActionSet_SingleLevel(t *testing.T) {
	now := metav1.Now()
	fullBackup := completedBackup("full-backup")
	fullBackup.Spec.BackupMethod = "xtrabackup"
	fullBackup.Status.StartTimestamp = &metav1.Time{Time: now.Add(-3 * time.Hour)}
	fullBackup.Status.CompletionTimestamp = &metav1.Time{Time: now.Add(-2 * time.Hour)}
	fullBackup.Status.BackupMethod.ActionSetName = "full-actionset"

	incBackup := completedBackup("inc-backup")
	incBackup.Spec.BackupMethod = "xtrabackup-inc"
	incBackup.Status.ParentBackupName = "full-backup"
	incBackup.Status.BaseBackupName = "full-backup"
	incBackup.Status.StartTimestamp = &metav1.Time{Time: now.Add(-1 * time.Hour)}
	incBackup.Status.CompletionTimestamp = &now
	incBackup.Status.BackupMethod.ActionSetName = "inc-actionset"
	incBackup.Status.BackupMethod.CompatibleMethod = "xtrabackup"

	fullAS := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "full-actionset"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeFull,
			Restore: &dpv1alpha1.RestoreActionSpec{
				PrepareData: &dpv1alpha1.JobActionSpec{
					BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img", Command: []string{"cmd"}},
				},
			},
		},
	}
	incAS := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "inc-actionset"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeIncremental,
			Restore: &dpv1alpha1.RestoreActionSpec{
				PrepareData: &dpv1alpha1.JobActionSpec{
					BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img", Command: []string{"cmd"}},
				},
			},
		},
	}

	mgr := newTestRestoreManager(fullBackup, incBackup, fullAS, incAS)

	sourceSet := BackupActionSet{
		Backup:    incBackup,
		ActionSet: incAS,
	}

	reqCtx := newMgrReqCtx()
	err := mgr.BuildIncrementalBackupActionSet(reqCtx, mgr.Client, sourceSet)
	require.NoError(t, err)
	assert.NotEmpty(t, mgr.PrepareDataBackupSets)
}

func TestBuildIncrementalBackupActionSet_ParentNotFound(t *testing.T) {
	incAS := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "inc-actionset"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeIncremental,
			Restore: &dpv1alpha1.RestoreActionSpec{
				PrepareData: &dpv1alpha1.JobActionSpec{
					BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img", Command: []string{"cmd"}},
				},
			},
		},
	}

	incBackup := completedBackup("inc-backup")
	incBackup.Status.ParentBackupName = "nonexistent"
	incBackup.Status.BackupMethod.ActionSetName = "inc-actionset"

	mgr := newTestRestoreManager(incBackup, incAS)

	sourceSet := BackupActionSet{
		Backup:    incBackup,
		ActionSet: incAS,
	}

	reqCtx := newMgrReqCtx()
	err := mgr.BuildIncrementalBackupActionSet(reqCtx, mgr.Client, sourceSet)
	require.Error(t, err)
}

// --- BuildContinuousRestoreManager deeper paths ---

func TestBuildContinuousRestoreManager_BaseBackupNotRequired(t *testing.T) {
	now := metav1.Now()
	continuousBackup := completedBackup("continuous")
	continuousBackup.Status.StartTimestamp = &metav1.Time{Time: now.Add(-2 * time.Hour)}
	continuousBackup.Status.CompletionTimestamp = &metav1.Time{Time: now.Add(1 * time.Hour)}

	baseBackupRequired := false
	as := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-actionset"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeContinuous,
			Restore: &dpv1alpha1.RestoreActionSpec{
				BaseBackupRequired: &baseBackupRequired,
				PrepareData: &dpv1alpha1.JobActionSpec{
					BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img", Command: []string{"cmd"}},
				},
			},
		},
	}

	mgr := newTestRestoreManager(continuousBackup, as)
	mgr.Restore.Spec.RestoreTime = now.Format(time.RFC3339)

	backupSet := BackupActionSet{Backup: continuousBackup, ActionSet: as}
	reqCtx := newMgrReqCtx()
	err := mgr.BuildContinuousRestoreManager(reqCtx, mgr.Client, backupSet)
	require.NoError(t, err)
	assert.Len(t, mgr.PrepareDataBackupSets, 1)
}

func TestBuildContinuousRestoreManager_WithBaseBackup(t *testing.T) {
	now := metav1.Now()
	continuousBackup := completedBackup("continuous")
	continuousBackup.Status.StartTimestamp = &metav1.Time{Time: now.Add(-3 * time.Hour)}
	continuousBackup.Status.CompletionTimestamp = &metav1.Time{Time: now.Add(1 * time.Hour)}
	continuousBackup.Spec.BackupPolicyName = "my-policy"

	fullBackup := completedBackup("full-backup")
	fullBackup.Status.StartTimestamp = &metav1.Time{Time: now.Add(-4 * time.Hour)}
	fullBackup.Status.CompletionTimestamp = &metav1.Time{Time: now.Add(-2 * time.Hour)}
	fullBackup.Status.BackupMethod.ActionSetName = "full-actionset"
	fullBackup.Labels = map[string]string{
		dptypes.BackupTypeLabelKey:   string(dpv1alpha1.BackupTypeFull),
		dptypes.BackupPolicyLabelKey: "my-policy",
	}

	continuousAS := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-actionset"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeContinuous,
			Restore: &dpv1alpha1.RestoreActionSpec{
				PrepareData: &dpv1alpha1.JobActionSpec{
					BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img", Command: []string{"cmd"}},
				},
			},
		},
	}
	fullAS := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "full-actionset"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeFull,
			Restore: &dpv1alpha1.RestoreActionSpec{
				PrepareData: &dpv1alpha1.JobActionSpec{
					BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img", Command: []string{"cmd"}},
				},
			},
		},
	}

	mgr := newTestRestoreManager(continuousBackup, fullBackup, continuousAS, fullAS)
	mgr.Restore.Spec.RestoreTime = now.Add(-1 * time.Hour).Format(time.RFC3339)

	backupSet := BackupActionSet{Backup: continuousBackup, ActionSet: continuousAS}
	reqCtx := newMgrReqCtx()
	err := mgr.BuildContinuousRestoreManager(reqCtx, mgr.Client, backupSet)
	require.NoError(t, err)
	assert.Len(t, mgr.PrepareDataBackupSets, 2) // full + continuous
}

func TestBuildContinuousRestoreManager_NoBaseBackupFound(t *testing.T) {
	now := metav1.Now()
	continuousBackup := completedBackup("continuous")
	continuousBackup.Status.StartTimestamp = &metav1.Time{Time: now.Add(-2 * time.Hour)}
	continuousBackup.Status.CompletionTimestamp = &metav1.Time{Time: now.Add(1 * time.Hour)}
	continuousBackup.Spec.BackupPolicyName = "my-policy"

	continuousAS := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-actionset"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeContinuous,
			Restore: &dpv1alpha1.RestoreActionSpec{
				PrepareData: &dpv1alpha1.JobActionSpec{
					BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img", Command: []string{"cmd"}},
				},
			},
		},
	}

	mgr := newTestRestoreManager(continuousBackup, continuousAS)
	mgr.Restore.Spec.RestoreTime = now.Format(time.RFC3339)

	backupSet := BackupActionSet{Backup: continuousBackup, ActionSet: continuousAS}
	reqCtx := newMgrReqCtx()
	err := mgr.BuildContinuousRestoreManager(reqCtx, mgr.Client, backupSet)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can not found latest")
}

func TestBuildContinuousRestoreManager_SkipBaseBackup(t *testing.T) {
	now := metav1.Now()
	continuousBackup := completedBackup("continuous")
	continuousBackup.Status.StartTimestamp = &metav1.Time{Time: now.Add(-3 * time.Hour)}
	continuousBackup.Status.CompletionTimestamp = &metav1.Time{Time: now.Add(1 * time.Hour)}
	continuousBackup.Spec.BackupPolicyName = "my-policy"

	fullBackup := completedBackup("full-backup")
	fullBackup.Status.StartTimestamp = &metav1.Time{Time: now.Add(-4 * time.Hour)}
	fullBackup.Status.CompletionTimestamp = &metav1.Time{Time: now.Add(-2 * time.Hour)}
	fullBackup.Status.BackupMethod.ActionSetName = "full-actionset"
	fullBackup.Labels = map[string]string{
		dptypes.BackupTypeLabelKey:   string(dpv1alpha1.BackupTypeFull),
		dptypes.BackupPolicyLabelKey: "my-policy",
	}

	continuousAS := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-actionset",
			Annotations: map[string]string{
				constant.SkipBaseBackupRestoreInPitrAnnotationKey: "true",
			},
		},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeContinuous,
			Restore: &dpv1alpha1.RestoreActionSpec{
				PrepareData: &dpv1alpha1.JobActionSpec{
					BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img", Command: []string{"cmd"}},
				},
			},
		},
	}
	fullAS := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "full-actionset"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeFull,
			Restore: &dpv1alpha1.RestoreActionSpec{
				PrepareData: &dpv1alpha1.JobActionSpec{
					BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img", Command: []string{"cmd"}},
				},
			},
		},
	}

	mgr := newTestRestoreManager(continuousBackup, fullBackup, continuousAS, fullAS)
	mgr.Restore.Spec.RestoreTime = now.Add(-1 * time.Hour).Format(time.RFC3339)

	backupSet := BackupActionSet{Backup: continuousBackup, ActionSet: continuousAS}
	reqCtx := newMgrReqCtx()
	err := mgr.BuildContinuousRestoreManager(reqCtx, mgr.Client, backupSet)
	require.NoError(t, err)
	// only continuous set, base backup is skipped
	assert.Len(t, mgr.PrepareDataBackupSets, 1)
}

// --- BuildPostReadyActionJobs (JobAction path) ---

func TestBuildPostReadyActionJobs_JobAction(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	targetPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-pod-0",
			Namespace: "default",
			Labels:    map[string]string{"app": "mysql"},
		},
		Spec: corev1.PodSpec{
			Subdomain:  "svc",
			Containers: []corev1.Container{{Name: "main", Image: "mysql", Ports: []corev1.ContainerPort{{ContainerPort: 3306}}}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()},
			},
		},
	}
	backup := completedBackup("b1")
	backup.Status.BackupMethod.TargetVolumes = &dpv1alpha1.TargetVolumeInfo{
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
	}

	as := testActionSet()
	as.Spec.Restore.PostReady = []dpv1alpha1.ActionSpec{
		{Job: &dpv1alpha1.JobActionSpec{BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "restore:v1", Command: []string{"restore.sh"}}}},
	}

	mgr := newTestRestoreManager(targetPod, backup, as)
	mgr.Restore.Spec.ReadyConfig = &dpv1alpha1.ReadyConfig{
		JobAction: &dpv1alpha1.JobAction{
			Target: dpv1alpha1.JobActionTarget{
				PodSelector: dpv1alpha1.PodSelector{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "mysql"},
					},
					Strategy: dpv1alpha1.PodSelectionStrategyAny,
				},
			},
		},
	}

	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{
				Strategy: dpv1alpha1.PodSelectionStrategyAny,
			},
		},
	}

	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{Backup: backup, ActionSet: as}
	jobs, err := mgr.BuildPostReadyActionJobs(reqCtx, mgr.Client, backupSet, target, 0)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
}

func TestBuildPostReadyActionJobs_NilJobAction(t *testing.T) {
	backup := completedBackup("b1")
	as := testActionSet()
	as.Spec.Restore.PostReady = []dpv1alpha1.ActionSpec{
		{Job: &dpv1alpha1.JobActionSpec{BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img", Command: []string{"cmd"}}}},
	}

	mgr := newTestRestoreManager(backup, as)
	mgr.Restore.Spec.ReadyConfig = &dpv1alpha1.ReadyConfig{
		// jobAction is nil
	}

	target := &dpv1alpha1.BackupStatusTarget{}
	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{Backup: backup, ActionSet: as}
	_, err := mgr.BuildPostReadyActionJobs(reqCtx, mgr.Client, backupSet, target, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jobAction can not be empty")
}

// --- BuildPostReadyActionJobs (ExecAction path) ---

func TestBuildPostReadyActionJobs_ExecAction(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	targetPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "exec-pod-0",
			Namespace: "default",
			Labels:    map[string]string{"app": "pg"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "postgres", Image: "pg:15"}},
		},
	}

	backup := completedBackup("b1")
	as := testActionSet()
	as.Spec.Restore.PostReady = []dpv1alpha1.ActionSpec{
		{Exec: &dpv1alpha1.ExecActionSpec{
			Command: []string{"/bin/sh", "-c", "pg_restore"},
		}},
	}

	mgr := newTestRestoreManager(targetPod, backup, as)
	mgr.Restore.Spec.ReadyConfig = &dpv1alpha1.ReadyConfig{
		ExecAction: &dpv1alpha1.ExecAction{
			Target: dpv1alpha1.ExecActionTarget{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "pg"},
				},
			},
		},
	}

	target := &dpv1alpha1.BackupStatusTarget{}
	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{Backup: backup, ActionSet: as}
	jobs, err := mgr.BuildPostReadyActionJobs(reqCtx, mgr.Client, backupSet, target, 0)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Contains(t, jobs[0].Spec.Template.Spec.Containers[0].Args, "exec")
}

func TestBuildPostReadyActionJobs_ExecAction_NilExecAction(t *testing.T) {
	backup := completedBackup("b1")
	as := testActionSet()
	as.Spec.Restore.PostReady = []dpv1alpha1.ActionSpec{
		{Exec: &dpv1alpha1.ExecActionSpec{Command: []string{"cmd"}}},
	}

	mgr := newTestRestoreManager(backup, as)
	mgr.Restore.Spec.ReadyConfig = &dpv1alpha1.ReadyConfig{}

	target := &dpv1alpha1.BackupStatusTarget{}
	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{Backup: backup, ActionSet: as}
	_, err := mgr.BuildPostReadyActionJobs(reqCtx, mgr.Client, backupSet, target, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execAction can not be empty")
}

// --- BuildVolumePopulateJob with DataSourceRef ---

func TestBuildVolumePopulateJob_WithDataSourceRef(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	backup := completedBackup("b1")
	backup.Status.BackupMethod.TargetVolumes = &dpv1alpha1.TargetVolumeInfo{
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
	}
	as := testActionSet()
	mgr := newTestRestoreManager(backup, as)
	mgr.Restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
		DataSourceRef: &dpv1alpha1.VolumeConfig{MountPath: "/data"},
	}

	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{Strategy: dpv1alpha1.PodSelectionStrategyAny},
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "populate-pvc", Namespace: "default"},
	}

	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{Backup: backup, ActionSet: as}
	job, err := mgr.BuildVolumePopulateJob(reqCtx, mgr.Client, backupSet, target, pvc, 0)
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Contains(t, job.Name, "populate-pvc")
}

// --- BuildPrepareDataJobs with template ---

func TestBuildPrepareDataJobs_WithTemplate(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	backup := completedBackup("b1")
	backup.Status.BackupMethod.TargetVolumes = &dpv1alpha1.TargetVolumeInfo{
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
	}
	as := testActionSet()
	mgr := newTestRestoreManager(backup, as)
	mgr.Restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
		RestoreVolumeClaimsTemplate: &dpv1alpha1.RestoreVolumeClaimsTemplate{
			Replicas: 2,
			Templates: []dpv1alpha1.RestoreVolumeClaim{
				{
					ObjectMeta:      metav1.ObjectMeta{Name: "data", Namespace: "default"},
					VolumeClaimSpec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}},
					VolumeConfig:    dpv1alpha1.VolumeConfig{MountPath: "/data"},
				},
			},
		},
	}
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{Strategy: dpv1alpha1.PodSelectionStrategyAny},
		},
	}

	reqCtx := newMgrReqCtx()
	backupSet := BackupActionSet{Backup: backup, ActionSet: as}
	jobs, err := mgr.BuildPrepareDataJobs(reqCtx, mgr.Client, backupSet, target, "action-1")
	require.NoError(t, err)
	assert.Len(t, jobs, 2)
}
