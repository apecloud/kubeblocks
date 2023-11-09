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

package restore

import (
	"fmt"
	"sort"

	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type BackupActionSet struct {
	Backup            *dpv1alpha1.Backup
	ActionSet         *dpv1alpha1.ActionSet
	UseVolumeSnapshot bool
}

type RestoreManager struct {
	OriginalRestore       *dpv1alpha1.Restore
	Restore               *dpv1alpha1.Restore
	PrepareDataBackupSets []BackupActionSet
	PostReadyBackupSets   []BackupActionSet
	Schema                *runtime.Scheme
	Recorder              record.EventRecorder
}

func NewRestoreManager(restore *dpv1alpha1.Restore, recorder record.EventRecorder, schema *runtime.Scheme) *RestoreManager {
	return &RestoreManager{
		OriginalRestore:       restore.DeepCopy(),
		Restore:               restore,
		PrepareDataBackupSets: []BackupActionSet{},
		PostReadyBackupSets:   []BackupActionSet{},
		Schema:                schema,
		Recorder:              recorder,
	}
}

// GetBackupActionSetByNamespaced gets the BackupActionSet by name and namespace of backup.
func (r *RestoreManager) GetBackupActionSetByNamespaced(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	backupName,
	namespace string) (*BackupActionSet, error) {
	backup := &dpv1alpha1.Backup{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: namespace, Name: backupName}, backup); err != nil {
		if apierrors.IsNotFound(err) {
			err = intctrlutil.NewFatalError(err.Error())
		}
		return nil, err
	}
	backupMethod := backup.Status.BackupMethod
	if backupMethod == nil {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf(`status.backupMethod of backup "%s" is empty`, backupName))
	}
	useVolumeSnapshot := backupMethod.SnapshotVolumes != nil && *backupMethod.SnapshotVolumes
	actionSet, err := utils.GetActionSetByName(reqCtx, cli, backup.Status.BackupMethod.ActionSetName)
	if err != nil {
		return nil, err
	}
	return &BackupActionSet{Backup: backup, ActionSet: actionSet, UseVolumeSnapshot: useVolumeSnapshot}, nil
}

// BuildDifferentialBackupActionSets builds the backupActionSets for specified incremental backup.
func (r *RestoreManager) BuildDifferentialBackupActionSets(reqCtx intctrlutil.RequestCtx, cli client.Client, sourceBackupSet BackupActionSet) error {
	parentBackupSet, err := r.GetBackupActionSetByNamespaced(reqCtx, cli, sourceBackupSet.Backup.Spec.ParentBackupName, sourceBackupSet.Backup.Namespace)
	if err != nil || parentBackupSet == nil {
		return err
	}
	r.SetBackupSets(*parentBackupSet, sourceBackupSet)
	return nil
}

// BuildIncrementalBackupActionSets builds the backupActionSets for specified incremental backup.
func (r *RestoreManager) BuildIncrementalBackupActionSets(reqCtx intctrlutil.RequestCtx, cli client.Client, sourceBackupSet BackupActionSet) error {
	r.SetBackupSets(sourceBackupSet)
	if sourceBackupSet.ActionSet != nil && sourceBackupSet.ActionSet.Spec.BackupType == dpv1alpha1.BackupTypeIncremental {
		// get the parent BackupActionSet for incremental.
		backupSet, err := r.GetBackupActionSetByNamespaced(reqCtx, cli, sourceBackupSet.Backup.Spec.ParentBackupName, sourceBackupSet.Backup.Namespace)
		if err != nil || backupSet == nil {
			return err
		}
		return r.BuildIncrementalBackupActionSets(reqCtx, cli, *backupSet)
	}
	// if reaches full backup, sort the BackupActionSets and return
	sortBackupSets := func(backupSets []BackupActionSet, reverse bool) []BackupActionSet {
		sort.Slice(backupSets, func(i, j int) bool {
			if reverse {
				i, j = j, i
			}
			backupI := backupSets[i].Backup
			backupJ := backupSets[j].Backup
			if backupI == nil {
				return false
			}
			if backupJ == nil {
				return true
			}
			return compareWithBackupStopTime(*backupI, *backupJ)
		})
		return backupSets
	}
	r.PrepareDataBackupSets = sortBackupSets(r.PrepareDataBackupSets, false)
	r.PostReadyBackupSets = sortBackupSets(r.PostReadyBackupSets, false)
	return nil
}

func (r *RestoreManager) SetBackupSets(backupSets ...BackupActionSet) {
	for i := range backupSets {
		if backupSets[i].UseVolumeSnapshot {
			r.PrepareDataBackupSets = append(r.PrepareDataBackupSets, backupSets[i])
			continue
		}
		if backupSets[i].ActionSet == nil || backupSets[i].ActionSet.Spec.Restore == nil {
			continue
		}
		if backupSets[i].ActionSet.Spec.Restore.PrepareData != nil {
			r.PrepareDataBackupSets = append(r.PrepareDataBackupSets, backupSets[i])
		}

		if len(backupSets[i].ActionSet.Spec.Restore.PostReady) > 0 {
			r.PostReadyBackupSets = append(r.PostReadyBackupSets, backupSets[i])
		}
	}
}

// AnalysisRestoreActionsWithBackup analysis the restore actions progress group by backup.
// check if the restore jobs are completed or failed or processing.
func (r *RestoreManager) AnalysisRestoreActionsWithBackup(stage dpv1alpha1.RestoreStage, backupName string, actionName string) (bool, bool) {
	var (
		restoreActionCount  int
		finishedActionCount int
		existFailedAction   bool
	)
	restoreActions := r.Restore.Status.Actions.PostReady
	if stage == dpv1alpha1.PrepareData {
		restoreActions = r.Restore.Status.Actions.PrepareData
		// if the stage is prepareData, actionCount keeps up with pvc count.
		restoreActionCount = GetRestoreActionsCountForPrepareData(r.Restore.Spec.PrepareDataConfig)
	}
	for i := range restoreActions {
		if restoreActions[i].BackupName != backupName || restoreActions[i].Name != actionName {
			continue
		}
		// if the stage is PostReady, actionCount keeps up with actions
		if stage == dpv1alpha1.PostReady {
			restoreActionCount += 1
		}
		switch restoreActions[i].Status {
		case dpv1alpha1.RestoreActionFailed:
			finishedActionCount += 1
			existFailedAction = true
		case dpv1alpha1.RestoreActionCompleted:
			finishedActionCount += 1
		}
	}

	allActionsFinished := restoreActionCount > 0 && finishedActionCount == restoreActionCount
	return allActionsFinished, existFailedAction
}

func (r *RestoreManager) RestorePVCFromSnapshot(reqCtx intctrlutil.RequestCtx, cli client.Client, backupSet BackupActionSet) error {
	prepareDataConfig := r.Restore.Spec.PrepareDataConfig
	if prepareDataConfig == nil {
		return nil
	}
	createPVCWithSnapshot := func(claim dpv1alpha1.RestoreVolumeClaim) error {
		if claim.VolumeSource == "" {
			return intctrlutil.NewFatalError(fmt.Sprintf(`claim "%s"" volumeSource can not be empty if the backup uses volume snapshot`, claim.Name))
		}

		// TODO: compatibility handling for version 0.6/0.5, will be removed in 0.8.
		volumeSnapshotName := backupSet.Backup.Name
		vsCli := &intctrlutil.VolumeSnapshotCompatClient{
			Client: cli,
			Ctx:    reqCtx.Ctx,
		}
		if exist, err := vsCli.CheckResourceExists(types.NamespacedName{Namespace: backupSet.Backup.Namespace, Name: volumeSnapshotName}, &vsv1.VolumeSnapshot{}); err != nil {
			return err
		} else if !exist {
			volumeSnapshotName = utils.GetBackupVolumeSnapshotName(backupSet.Backup.Name, claim.VolumeSource)
		}
		// get volumeSnapshot by backup and volumeSource.
		claim.VolumeClaimSpec.DataSource = &corev1.TypedLocalObjectReference{
			Name:     volumeSnapshotName,
			Kind:     constant.VolumeSnapshotKind,
			APIGroup: &VolumeSnapshotGroup,
		}
		return r.createPVCIfNotExist(reqCtx, cli, claim.ObjectMeta, claim.VolumeClaimSpec)
	}

	for _, claim := range prepareDataConfig.RestoreVolumeClaims {
		if err := createPVCWithSnapshot(claim); err != nil {
			return err
		}
	}
	claimTemplate := prepareDataConfig.RestoreVolumeClaimsTemplate

	if claimTemplate != nil {
		restoreJobReplicas := GetRestoreActionsCountForPrepareData(prepareDataConfig)
		for i := 0; i < restoreJobReplicas; i++ {
			//  create pvc from claims template, build volumes and volumeMounts
			for _, claim := range prepareDataConfig.RestoreVolumeClaimsTemplate.Templates {
				claim.Name = fmt.Sprintf("%s-%d", claim.Name, i+int(claimTemplate.StartingIndex))
				if err := createPVCWithSnapshot(claim); err != nil {
					return err
				}
			}
		}
	}
	// NOTE: do not to record status action for restoring from snapshot. it is not defined in ActionSet.
	return nil
}

func (r *RestoreManager) prepareBackupRepo(reqCtx intctrlutil.RequestCtx, cli client.Client, backupSet BackupActionSet) (*dpv1alpha1.BackupRepo, error) {
	if backupSet.Backup.Status.BackupRepoName != "" {
		backupRepo := &dpv1alpha1.BackupRepo{}
		err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: backupSet.Backup.Status.BackupRepoName}, backupRepo)
		if err != nil {
			if apierrors.IsNotFound(err) {
				err = intctrlutil.NewFatalError(err.Error())
			}
			return nil, err
		}
		return backupRepo, nil
	}
	return nil, nil
}

// BuildPrepareDataJobs builds the restore jobs for prepare pvc's data, and will create the target pvcs if not exist.
func (r *RestoreManager) BuildPrepareDataJobs(reqCtx intctrlutil.RequestCtx, cli client.Client, backupSet BackupActionSet, actionName string) ([]*batchv1.Job, error) {
	prepareDataConfig := r.Restore.Spec.PrepareDataConfig
	if prepareDataConfig == nil {
		return nil, nil
	}
	if !backupSet.ActionSet.HasPrepareDataStage() {
		return nil, nil
	}
	backupRepo, err := r.prepareBackupRepo(reqCtx, cli, backupSet)
	if err != nil {
		return nil, err
	}
	jobBuilder := newRestoreJobBuilder(r.Restore, backupSet, backupRepo, dpv1alpha1.PrepareData).
		setImage(backupSet.ActionSet.Spec.Restore.PrepareData.Image).
		setCommand(backupSet.ActionSet.Spec.Restore.PrepareData.Command).
		addCommonEnv().
		attachBackupRepo()

	createPVCIfNotExistsAndBuildVolume := func(claim dpv1alpha1.RestoreVolumeClaim, identifier string) (*corev1.Volume, *corev1.VolumeMount, error) {
		if err := r.createPVCIfNotExist(reqCtx, cli, claim.ObjectMeta, claim.VolumeClaimSpec); err != nil {
			return nil, nil, err
		}
		return jobBuilder.buildPVCVolumeAndMount(claim.VolumeConfig, claim.Name, identifier)
	}

	// create pvc from volumeClaims, set volume and volumeMount to jobBuilder
	for _, claim := range prepareDataConfig.RestoreVolumeClaims {
		volume, volumeMount, err := createPVCIfNotExistsAndBuildVolume(claim, "dp-claim")
		if err != nil {
			return nil, err
		}
		jobBuilder.addToCommonVolumesAndMounts(volume, volumeMount)
	}

	var (
		restoreJobs        []*batchv1.Job
		restoreJobReplicas = GetRestoreActionsCountForPrepareData(prepareDataConfig)
		claimsTemplate     = prepareDataConfig.RestoreVolumeClaimsTemplate
	)

	if prepareDataConfig.IsSerialPolicy() {
		// obtain the PVC serial number that needs to be restored
		currentOrder := 1
		prepareActions := r.Restore.Status.Actions.PrepareData
		for i := range prepareActions {
			if prepareActions[i].BackupName != backupSet.Backup.Name || prepareActions[i].Name != actionName {
				continue
			}
			if prepareActions[i].Status == dpv1alpha1.RestoreActionCompleted && currentOrder < restoreJobReplicas {
				currentOrder += 1
				if prepareDataConfig.IsSerialPolicy() {
					// if the restore policy is Serial, should delete the completed job to release the pvc.
					if err := deleteRestoreJob(reqCtx, cli, prepareActions[i].ObjectKey, r.Restore.Namespace); err != nil {
						return nil, err
					}
				}
			}
		}
		restoreJobReplicas = currentOrder
	}

	// build restore job to prepare pvc's data
	for i := 0; i < restoreJobReplicas; i++ {
		// reset specific volumes and volumeMounts
		jobBuilder.resetSpecificVolumesAndMounts()
		if claimsTemplate != nil {
			//  create pvc from claims template, build volumes and volumeMounts
			for _, claim := range claimsTemplate.Templates {
				claim.Name = fmt.Sprintf("%s-%d", claim.Name, i+int(claimsTemplate.StartingIndex))
				volume, volumeMount, err := createPVCIfNotExistsAndBuildVolume(claim, "dp-claim-tpl")
				if err != nil {
					return nil, err
				}
				jobBuilder.addToSpecificVolumesAndMounts(volume, volumeMount)
			}
		}
		// build job and append
		job := jobBuilder.setJobName(jobBuilder.builderRestoreJobName(i)).build()
		if prepareDataConfig.IsSerialPolicy() &&
			restoreJobHasCompleted(r.Restore.Status.Actions.PrepareData, job.Name) {
			// if the job has completed and the restore policy is Serial, continue
			continue
		}
		restoreJobs = append(restoreJobs, job)
	}
	return restoreJobs, nil
}

func (r *RestoreManager) BuildVolumePopulateJob(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	backupSet BackupActionSet,
	populatePVC *corev1.PersistentVolumeClaim,
	index int) (*batchv1.Job, error) {
	prepareDataConfig := r.Restore.Spec.PrepareDataConfig
	if prepareDataConfig == nil && prepareDataConfig.DataSourceRef == nil {
		return nil, nil
	}
	if !backupSet.ActionSet.HasPrepareDataStage() {
		return nil, nil
	}
	backupRepo, err := r.prepareBackupRepo(reqCtx, cli, backupSet)
	if err != nil {
		return nil, err
	}
	jobBuilder := newRestoreJobBuilder(r.Restore, backupSet, backupRepo, dpv1alpha1.PrepareData).
		setJobName(fmt.Sprintf("%s-%d", populatePVC.Name, index)).
		addLabel(DataProtectionLabelPopulatePVCKey, populatePVC.Name).
		setImage(backupSet.ActionSet.Spec.Restore.PrepareData.Image).
		setCommand(backupSet.ActionSet.Spec.Restore.PrepareData.Command).
		attachBackupRepo().
		addCommonEnv()
	volume, volumeMount, err := jobBuilder.buildPVCVolumeAndMount(*prepareDataConfig.DataSourceRef, populatePVC.Name, "dp-claim")
	if err != nil {
		return nil, err
	}
	job := jobBuilder.addToSpecificVolumesAndMounts(volume, volumeMount).build()
	return job, nil
}

// BuildPostReadyActionJobs builds the post ready jobs.
func (r *RestoreManager) BuildPostReadyActionJobs(reqCtx intctrlutil.RequestCtx, cli client.Client, backupSet BackupActionSet, step int) ([]*batchv1.Job, error) {
	readyConfig := r.Restore.Spec.ReadyConfig
	if readyConfig == nil {
		return nil, nil
	}
	if !backupSet.ActionSet.HasPostReadyStage() {
		return nil, nil
	}
	backupRepo, err := r.prepareBackupRepo(reqCtx, cli, backupSet)
	if err != nil {
		return nil, err
	}
	actionSpec := backupSet.ActionSet.Spec.Restore.PostReady[step]
	getTargetPodList := func(labelSelector metav1.LabelSelector, msgKey string) ([]corev1.Pod, error) {
		targetPodList, err := utils.GetPodListByLabelSelector(reqCtx, cli, labelSelector)
		if err != nil {
			return nil, err
		}
		if len(targetPodList.Items) == 0 {
			return nil, fmt.Errorf("can not found any pod by spec.readyConfig.%s.target.podSelector", msgKey)
		}
		return targetPodList.Items, nil
	}

	jobBuilder := newRestoreJobBuilder(r.Restore, backupSet, backupRepo, dpv1alpha1.PostReady).addCommonEnv()

	buildJobName := func(index int) string {
		jobName := fmt.Sprintf("restore-post-ready-%s-%s-%d-%d", r.Restore.UID[:8], backupSet.Backup.Name, step, index)
		return cutJobName(jobName)
	}

	buildJobsForJobAction := func() ([]*batchv1.Job, error) {
		jobAction := r.Restore.Spec.ReadyConfig.JobAction
		if jobAction == nil {
			return nil, intctrlutil.NewFatalError("spec.readyConfig.jobAction can not be empty")
		}
		targetPodList, err := getTargetPodList(jobAction.Target.PodSelector, "jobAction")
		if err != nil {
			return nil, err
		}
		targetPod := targetPodList[0]
		if boolptr.IsSetToTrue(actionSpec.Job.RunOnTargetPodNode) {
			jobBuilder.setNodeNameToNodeSelector(targetPod.Spec.NodeName)
			// mount the targe pod's volumes when RunOnTargetPodNode is true
			for _, volumeMount := range jobAction.Target.VolumeMounts {
				for _, volume := range targetPod.Spec.Volumes {
					if volume.Name != volumeMount.Name {
						continue
					}
					jobBuilder.addToSpecificVolumesAndMounts(&volume, &volumeMount)
				}
			}
		}
		job := jobBuilder.setImage(actionSpec.Job.Image).
			setJobName(buildJobName(0)).
			attachBackupRepo().
			setCommand(actionSpec.Job.Command).
			setToleration(targetPod.Spec.Tolerations).
			addTargetPodAndCredentialEnv(&targetPod, r.Restore.Spec.ReadyConfig.ConnectionCredential).
			build()
		return []*batchv1.Job{job}, nil
	}

	buildJobsForExecAction := func() ([]*batchv1.Job, error) {
		execAction := r.Restore.Spec.ReadyConfig.ExecAction
		if execAction == nil {
			return nil, intctrlutil.NewFatalError("spec.readyConfig.execAction can not be empty")
		}
		targetPodList, err := getTargetPodList(execAction.Target.PodSelector, "execAction")
		if err != nil {
			return nil, err
		}
		var restoreJobs []*batchv1.Job
		for i := range targetPodList {
			containerName := actionSpec.Exec.Container
			if containerName == "" {
				containerName = targetPodList[i].Spec.Containers[0].Name
			}
			args := append([]string{"-n", targetPodList[i].Namespace, "exec", targetPodList[i].Name, "-c", containerName, "--"}, actionSpec.Exec.Command...)
			jobBuilder.setImage(viper.GetString(constant.KBToolsImage)).setCommand([]string{"kubectl"}).setArgs(args).
				setJobName(buildJobName(i)).
				setToleration(targetPodList[i].Spec.Tolerations)
			job := jobBuilder.build()
			// create exec job in kubeblocks namespace for security
			job.Namespace = viper.GetString(constant.CfgKeyCtrlrMgrNS)
			job.Labels[DataProtectionLabelRestoreNamespaceKey] = r.Restore.Namespace
			// use the kubeblocks's serviceAccount
			job.Spec.Template.Spec.ServiceAccountName = viper.GetString(constant.KBServiceAcccountName)
			restoreJobs = append(restoreJobs, job)
		}
		return restoreJobs, nil
	}

	if actionSpec.Job != nil {
		return buildJobsForJobAction()
	}
	return buildJobsForExecAction()
}

func (r *RestoreManager) createPVCIfNotExist(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	claimMetadata metav1.ObjectMeta,
	claimSpec corev1.PersistentVolumeClaimSpec) error {
	claimMetadata.Namespace = reqCtx.Req.Namespace
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: claimMetadata,
		Spec:       claimSpec,
	}
	tmpPVC := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: claimMetadata.Name, Namespace: claimMetadata.Namespace}, tmpPVC); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		msg := fmt.Sprintf("created pvc %s/%s", pvc.Namespace, pvc.Name)
		r.Recorder.Event(r.Restore, corev1.EventTypeNormal, reasonCreateRestorePVC, msg)
		if err = cli.Create(reqCtx.Ctx, pvc); err != nil {
			return client.IgnoreAlreadyExists(err)
		}
	}
	return nil
}

// CreateJobsIfNotExist creates the jobs if not exist.
func (r *RestoreManager) CreateJobsIfNotExist(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	ownerObj client.Object,
	objs []*batchv1.Job) ([]*batchv1.Job, error) {
	// creates jobs if not exist
	var fetchedJobs []*batchv1.Job
	for i := range objs {
		if objs[i] == nil {
			continue
		}
		fetchedJob := &batchv1.Job{}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(objs[i]), fetchedJob); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
			if ownerObj.GetNamespace() == objs[i].Namespace {
				if err = controllerutil.SetControllerReference(ownerObj, objs[i], r.Schema); err != nil {
					return nil, err
				}
			}
			if err = cli.Create(reqCtx.Ctx, objs[i]); err != nil && !apierrors.IsAlreadyExists(err) {
				return nil, err
			}
			msg := fmt.Sprintf("created job %s/%s", objs[i].Namespace, objs[i].Name)
			r.Recorder.Event(r.Restore, corev1.EventTypeNormal, reasonCreateRestoreJob, msg)
			fetchedJobs = append(fetchedJobs, objs[i])
		} else {
			fetchedJobs = append(fetchedJobs, fetchedJob)
		}
	}
	return fetchedJobs, nil
}

// CheckJobsDone checks if jobs are completed or failed.
func (r *RestoreManager) CheckJobsDone(
	stage dpv1alpha1.RestoreStage,
	actionName string,
	backupSet BackupActionSet,
	fetchedJobs []*batchv1.Job) (bool, bool) {
	var (
		allJobFinished = true
		existFailedJob bool
	)
	restoreActions := &r.Restore.Status.Actions.PrepareData
	if stage == dpv1alpha1.PostReady {
		restoreActions = &r.Restore.Status.Actions.PostReady
	}
	for i := range fetchedJobs {
		statusAction := dpv1alpha1.RestoreStatusAction{
			Name:       actionName,
			ObjectKey:  buildJobKeyForActionStatus(fetchedJobs[i].Name),
			BackupName: backupSet.Backup.Name,
		}
		done, _, errMsg := utils.IsJobFinished(fetchedJobs[i])
		switch {
		case errMsg != "":
			existFailedJob = true
			statusAction.Status = dpv1alpha1.RestoreActionFailed
			statusAction.Message = errMsg
			SetRestoreStatusAction(restoreActions, statusAction)
		case done:
			statusAction.Status = dpv1alpha1.RestoreActionCompleted
			SetRestoreStatusAction(restoreActions, statusAction)
		default:
			allJobFinished = false
			statusAction.Status = dpv1alpha1.RestoreActionProcessing
			SetRestoreStatusAction(restoreActions, statusAction)
		}
	}
	return allJobFinished, existFailedJob
}

// Recalculation whether all actions have been completed.
func (r *RestoreManager) Recalculation(backupName, actionName string, allActionsFinished, existFailedAction *bool) {
	prepareDataConfig := r.Restore.Spec.PrepareDataConfig
	if !prepareDataConfig.IsSerialPolicy() {
		return
	}

	if *existFailedAction {
		// under the Serial policy, restore will be failed if any action is failed.
		*allActionsFinished = true
		return
	}
	var actionCount int
	for _, v := range r.Restore.Status.Actions.PrepareData {
		if v.Name == actionName && v.BackupName == backupName {
			actionCount += 1
		}
	}
	if actionCount != GetRestoreActionsCountForPrepareData(prepareDataConfig) {
		// if the number of actions is not equal to the number of target actions, the recovery has not yet ended
		*allActionsFinished = false
	}
}
