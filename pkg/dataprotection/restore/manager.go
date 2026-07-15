/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	restoreManagerContainerName           = "restore-manager"
	postReadyExecutionPolicyAnnotationKey = "dataprotection.kubeblocks.io/post-ready-execution-policy"
	postReadyTargetIdentityAnnotationKey  = "dataprotection.kubeblocks.io/post-ready-target-identity"
	postReadyTargetPlanAnnotationKey      = "dataprotection.kubeblocks.io/post-ready-target-plan"
	postReadyActionContractAnnotationKey  = "dataprotection.kubeblocks.io/post-ready-action-contract"
	postReadyBackupNameAnnotationKey      = "dataprotection.kubeblocks.io/post-ready-backup-name"
	postReadyActionNameAnnotationKey      = "dataprotection.kubeblocks.io/post-ready-action-name"
	postReadyPlanNameAnnotationKey        = "dataprotection.kubeblocks.io/post-ready-plan-name"
	postReadyPlanDigestAnnotationKey      = "dataprotection.kubeblocks.io/post-ready-plan-digest"
	postReadyPlanRestoreUIDAnnotationKey  = "dataprotection.kubeblocks.io/post-ready-restore-uid"
	postReadyPlanDataKey                  = "plan.json"
	postReadyPlanVersion                  = "v2"
	postReadyPlanMaxPayloadBytes          = 1 << 20
	postReadyPlanMarkerAnnotationKey      = "dataprotection.kubeblocks.io/post-ready-stage-plan"
)

const postReadyPlanSecretType corev1.SecretType = "dataprotection.kubeblocks.io/post-ready-plan"

type postReadyActionExecutionPlan struct {
	Order      int           `json:"order"`
	BackupName string        `json:"backupName"`
	ActionName string        `json:"actionName"`
	Jobs       []batchv1.Job `json:"jobs"`
}

type postReadyExecutionPlan struct {
	Version               string                         `json:"version"`
	RestoreNamespace      string                         `json:"restoreNamespace"`
	RestoreName           string                         `json:"restoreName"`
	RestoreUID            string                         `json:"restoreUID"`
	SourceBackupNamespace string                         `json:"sourceBackupNamespace"`
	SourceBackupName      string                         `json:"sourceBackupName"`
	Actions               []postReadyActionExecutionPlan `json:"actions"`
}

type BackupActionSet struct {
	Backup *dpv1alpha1.Backup
	// set it when the backup relies on incremental backups, such as Incremental backup
	AncestorIncrementalBackups []*dpv1alpha1.Backup
	// set it when the backup relies on a base backup, such as Continuous backup
	BaseBackup              *dpv1alpha1.Backup
	ActionSet               *dpv1alpha1.ActionSet
	UseVolumeSnapshot       bool
	UseDurablePostReadyPlan bool
}

type RestoreManager struct {
	OriginalRestore       *dpv1alpha1.Restore
	Restore               *dpv1alpha1.Restore
	PrepareDataBackupSets []BackupActionSet
	PostReadyBackupSets   []BackupActionSet
	Schema                *runtime.Scheme
	Recorder              record.EventRecorder
	Client                client.Client
	WorkerServiceAccount  string
}

func NewRestoreManager(restore *dpv1alpha1.Restore, recorder record.EventRecorder, schema *runtime.Scheme, client client.Client) *RestoreManager {
	return &RestoreManager{
		OriginalRestore:       restore.DeepCopy(),
		Restore:               restore,
		PrepareDataBackupSets: []BackupActionSet{},
		PostReadyBackupSets:   []BackupActionSet{},
		Schema:                schema,
		Recorder:              recorder,
		Client:                client,
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
		if apierrors.IsNotFound(err) {
			_, foundPlan, planErr := r.loadPostReadyExecutionPlanStage(reqCtx, cli)
			if planErr != nil {
				return nil, planErr
			}
			if foundPlan {
				return &BackupActionSet{
					Backup: backup, UseVolumeSnapshot: useVolumeSnapshot, UseDurablePostReadyPlan: true,
				}, nil
			}
		}
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

// BuildIncrementalBackupActionSet builds the backupActionSet for specified incremental backup.
func (r *RestoreManager) BuildIncrementalBackupActionSet(reqCtx intctrlutil.RequestCtx, cli client.Client, sourceBackupSet BackupActionSet) error {
	childBackupSet := &sourceBackupSet
	backupMap := map[string]struct{}{}
	for childBackupSet.ActionSet != nil && childBackupSet.ActionSet.Spec.BackupType == dpv1alpha1.BackupTypeIncremental {
		// record the traversed backups
		backupMap[childBackupSet.Backup.Name] = struct{}{}
		// get the parent BackupActionSet for incremental.
		backupSet, err := r.GetBackupActionSetByNamespaced(reqCtx, cli, childBackupSet.Backup.Status.ParentBackupName, childBackupSet.Backup.Namespace)
		if err != nil || backupSet == nil {
			return intctrlutil.NewFatalError(fmt.Sprintf(`fails to get parent backup "%s" of incremental backup "%s"`,
				childBackupSet.Backup.Status.ParentBackupName, childBackupSet.Backup.Name))
		}
		if _, ok := backupMap[backupSet.Backup.Name]; ok {
			return intctrlutil.NewFatalError(fmt.Sprintf(`backup "%s" relies on child backup "%s"`,
				childBackupSet.Backup.Name, backupSet.Backup.Name))
		}
		if err := ValidateParentBackupSet(backupSet, childBackupSet); err != nil {
			return intctrlutil.NewFatalError(fmt.Sprintf(`fails to validate parent backup "%s" and child backup "%s": %v`,
				backupSet.Backup.Name, childBackupSet.Backup.Name, err))
		}
		if backupSet.ActionSet != nil && backupSet.ActionSet.Spec.BackupType == dpv1alpha1.BackupTypeIncremental {
			sourceBackupSet.AncestorIncrementalBackups = append([]*dpv1alpha1.Backup{backupSet.Backup}, sourceBackupSet.AncestorIncrementalBackups...)
		} else {
			sourceBackupSet.BaseBackup = backupSet.Backup
		}
		childBackupSet = backupSet
	}
	r.SetBackupSets(sourceBackupSet)
	return nil
}

func (r *RestoreManager) BuildContinuousRestoreManager(reqCtx intctrlutil.RequestCtx, cli client.Client, continuousBackupSet BackupActionSet) error {
	restoreTime, _ := time.Parse(time.RFC3339, r.Restore.Spec.RestoreTime)
	continuousBackup := continuousBackupSet.Backup
	checkRestoreTime := func() error {
		startTime := continuousBackup.GetStartTime()
		stopTime := continuousBackup.GetEndTime()
		if startTime.IsZero() || stopTime.IsZero() {
			return intctrlutil.NewFatalError(fmt.Sprintf(`startTimeStamp or completeTimeStamp of backup "%s" is empty`, continuousBackup.Name))
		}
		if !isTimeInRange(restoreTime, startTime.Time, stopTime.Time) {
			return intctrlutil.NewFatalError(fmt.Sprintf(`restore time out of the range for backup "%s"`, continuousBackup.Name))
		}
		return nil
	}
	// check if the restore time is valid.
	if err := checkRestoreTime(); err != nil {
		return err
	}

	if continuousBackupSet.ActionSet.Spec.Restore != nil {
		if baseBackupRequired := continuousBackupSet.ActionSet.Spec.Restore.BaseBackupRequired; boolptr.IsSetToFalse(baseBackupRequired) {
			r.SetBackupSets(continuousBackupSet)
			return nil
		}
	}

	baseBackupSet, err := r.getBaseBackupActionSetForContinuous(reqCtx, cli, continuousBackup, metav1.NewTime(restoreTime))
	if err != nil || baseBackupSet == nil {
		return err
	}

	skipBaseBackupRestoreInPitr := false
	if continuousBackupSet.ActionSet.Annotations != nil {
		if continuousBackupSet.ActionSet.Annotations[constant.SkipBaseBackupRestoreInPitrAnnotationKey] == "true" {
			skipBaseBackupRestoreInPitr = true
		}
	}

	// set base backup
	continuousBackupSet.BaseBackup = baseBackupSet.Backup
	if baseBackupSet.ActionSet != nil && baseBackupSet.ActionSet.Spec.BackupType == dpv1alpha1.BackupTypeIncremental {
		if skipBaseBackupRestoreInPitr {
			return intctrlutil.NewFatalError("unify incremental and continuous restore job is not supported")
		}
		if err = r.BuildIncrementalBackupActionSet(reqCtx, cli, *baseBackupSet); err != nil {
			return err
		}
		r.SetBackupSets(continuousBackupSet)
	} else {
		if skipBaseBackupRestoreInPitr {
			r.Recorder.Event(r.Restore, corev1.EventTypeNormal, "SkipBaseBackupRestoreInPitr", "base backup restore skipped")
			r.SetBackupSets(continuousBackupSet)
		} else {
			r.SetBackupSets(*baseBackupSet, continuousBackupSet)
		}
	}
	return nil
}

// getBaseBackupActionSetForContinuous gets full or incremental backup and actionSet for continuous.
func (r *RestoreManager) getBaseBackupActionSetForContinuous(reqCtx intctrlutil.RequestCtx, cli client.Client, continuousBackup *dpv1alpha1.Backup, restoreTime metav1.Time) (*BackupActionSet, error) {
	notFoundLatestBackup := func() (*BackupActionSet, error) {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf(`can not found latest full or incremental backup based on backupPolicy "%s" and specified restoreTime "%s"`,
			continuousBackup.Spec.BackupPolicyName, restoreTime))
	}
	if continuousBackup.GetStartTime().IsZero() {
		return notFoundLatestBackup()
	}
	// 1. list completed backups
	// full backups
	fullBackupItems, err := r.listCompletedBackups(reqCtx, cli, continuousBackup, dpv1alpha1.BackupTypeFull)
	if err != nil {
		return nil, err
	}
	// incremental backups
	incrementalBackupItems, err := r.listCompletedBackups(reqCtx, cli, continuousBackup, dpv1alpha1.BackupTypeIncremental)
	if err != nil {
		return nil, err
	}
	backupItems := []dpv1alpha1.Backup{}
	backupItems = append(backupItems, fullBackupItems...)
	backupItems = append(backupItems, incrementalBackupItems...)
	// sort by completed time in descending order
	sort.Slice(backupItems, func(i, j int) bool {
		i, j = j, i
		return utils.CompareWithBackupStopTime(backupItems[i], backupItems[j])
	})

	// 2. get the latest backup object
	var latestBackup *dpv1alpha1.Backup
	for _, item := range backupItems {
		backupStopTime := item.GetEndTime()
		// latest backup rules:
		// 1. Full or Incremental backup's stopTime must after Continuous backup's startTime.
		//    Even if the seconds are the same, the data may not be continuous.
		// 2. RestoreTime should after the Full or Incremental backup's stopTime.
		if backupStopTime != nil &&
			!restoreTime.Before(backupStopTime) &&
			!backupStopTime.Before(continuousBackup.GetStartTime()) {
			latestBackup = &item
			break
		}
	}
	if latestBackup == nil {
		return notFoundLatestBackup()
	}
	// 3. get the action set
	var actionSetName string
	if latestBackup.Status.BackupMethod != nil {
		actionSetName = latestBackup.Status.BackupMethod.ActionSetName
	}
	actionSet, err := utils.GetActionSetByName(reqCtx, cli, actionSetName)
	if err != nil {
		return nil, err
	}
	return &BackupActionSet{Backup: latestBackup, ActionSet: actionSet}, nil
}

func (r *RestoreManager) listCompletedBackups(reqCtx intctrlutil.RequestCtx, cli client.Client, continuousBackup *dpv1alpha1.Backup, backupType dpv1alpha1.BackupType) ([]dpv1alpha1.Backup, error) {
	matchingLabels := map[string]string{
		dptypes.BackupTypeLabelKey: string(backupType),
	}
	if clusterUID := continuousBackup.Labels[dptypes.ClusterUIDLabelKey]; clusterUID != "" {
		matchingLabels[dptypes.ClusterUIDLabelKey] = clusterUID
	}
	if instance := continuousBackup.Labels[constant.AppInstanceLabelKey]; instance != "" {
		matchingLabels[constant.AppInstanceLabelKey] = instance
	}
	if compName := continuousBackup.Labels[constant.KBAppComponentLabelKey]; compName != "" {
		matchingLabels[constant.KBAppComponentLabelKey] = compName
	}
	if len(matchingLabels) == 1 {
		// if only backupType label exists, need to match based on whether it is the same policy.
		matchingLabels[dptypes.BackupPolicyLabelKey] = continuousBackup.Spec.BackupPolicyName
	}
	backups := dpv1alpha1.BackupList{}
	if err := cli.List(reqCtx.Ctx, &backups,
		client.InNamespace(continuousBackup.Namespace),
		client.MatchingLabels(matchingLabels),
	); err != nil {
		return nil, err
	}
	backupItems := []dpv1alpha1.Backup{}
	for _, b := range backups.Items {
		if b.Status.Phase == dpv1alpha1.BackupPhaseCompleted {
			backupItems = append(backupItems, b)
		}
	}
	return backupItems, nil
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
	return r.AnalysisRestoreActionsWithBackupExpected(stage, backupName, actionName, 0)
}

// AnalysisRestoreActionsWithBackupExpected prevents a partially recorded
// postReady action from looking complete when its immutable plan contains more
// Jobs than Restore status has observed so far.
func (r *RestoreManager) AnalysisRestoreActionsWithBackupExpected(
	stage dpv1alpha1.RestoreStage,
	backupName string,
	actionName string,
	expectedActionCount int,
) (bool, bool) {
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
	if stage == dpv1alpha1.PostReady && expectedActionCount > 0 {
		restoreActionCount = expectedActionCount
	}

	allActionsFinished := restoreActionCount > 0 && finishedActionCount == restoreActionCount
	return allActionsFinished, existFailedAction
}

func addItsManagingLabels(claim *dpv1alpha1.RestoreVolumeClaim, index int) {
	clusterName := claim.Labels[constant.AppInstanceLabelKey]
	compName := claim.Labels[constant.KBAppComponentLabelKey]
	if clusterName == "" || compName == "" {
		return
	}
	if claim.Labels == nil {
		claim.Labels = make(map[string]string)
	}

	compObjName := constant.GenerateWorkloadNamePattern(clusterName, compName)
	itsMatchLabels := instanceset.GetMatchLabels(compObjName)
	intctrlutil.MergeMetadataMapInplace(itsMatchLabels, &claim.Labels)

	if claim.Labels[constant.KBAppPodNameLabelKey] == "" {
		templateName, exist := claim.Labels[constant.KBAppInstanceTemplateLabelKey]
		var podName string
		if exist {
			podName = fmt.Sprintf("%s-%s-%d", compObjName, templateName, index)
		} else {
			podName = fmt.Sprintf("%s-%d", compObjName, index)
		}
		claim.Labels[constant.KBAppPodNameLabelKey] = podName
	}
}

func (r *RestoreManager) RestorePVCFromSnapshot(reqCtx intctrlutil.RequestCtx, cli client.Client, backupSet BackupActionSet, target *dpv1alpha1.BackupStatusTarget) error {
	prepareDataConfig := r.Restore.Spec.PrepareDataConfig
	if prepareDataConfig == nil {
		return nil
	}
	createPVCWithSnapshot := func(claim dpv1alpha1.RestoreVolumeClaim) error {
		if claim.VolumeSource == "" {
			return intctrlutil.NewFatalError(fmt.Sprintf(`claim "%s"" volumeSource can not be empty if the backup uses volume snapshot`, claim.Name))
		}
		// TODO:  will be removed in 0.10.0, compatibility handling for version 0.8.
		volumeSnapshotName := utils.GetOldBackupVolumeSnapshotName(backupSet.Backup.Name, claim.VolumeSource)
		vsCli := utils.NewCompatClient(cli)
		if exist, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, vsCli,
			types.NamespacedName{Namespace: backupSet.Backup.Namespace, Name: volumeSnapshotName},
			&vsv1.VolumeSnapshot{}); err != nil {
			return err
		} else if !exist {
			sourceTargetPodName, err := GetSourcePodNameFromTarget(target, prepareDataConfig.RequiredPolicyForAllPodSelection, 0)
			if err != nil {
				return err
			}
			if target.PodSelector.Strategy == dpv1alpha1.PodSelectionStrategyAny || sourceTargetPodName != "" {
				snapshotGroup := GetVolumeSnapshotsBySourcePod(backupSet.Backup, target, sourceTargetPodName)
				if snapshotGroup == nil {
					message := fmt.Sprintf(`can not found the volumeSnapshot in status.actions, sourceTargetPod is "%s"`, sourceTargetPodName)
					return intctrlutil.NewFatalError(message)
				}
				volumeSnapshotName = snapshotGroup[claim.VolumeSource]
			}
		}
		if volumeSnapshotName != "" {
			// get volumeSnapshot by backup and volumeSource.
			claim.VolumeClaimSpec.DataSource = &corev1.TypedLocalObjectReference{
				Name:     volumeSnapshotName,
				Kind:     constant.VolumeSnapshotKind,
				APIGroup: &VolumeSnapshotGroup,
			}
		}
		return r.createPVCIfNotExist(reqCtx, cli, claim.ObjectMeta, claim.VolumeClaimSpec)
	}
	for i := range prepareDataConfig.RestoreVolumeClaims {
		if err := createPVCWithSnapshot(prepareDataConfig.RestoreVolumeClaims[i]); err != nil {
			return err
		}
	}
	claimTemplate := prepareDataConfig.RestoreVolumeClaimsTemplate
	if claimTemplate != nil {
		restoreJobReplicas := GetRestoreActionsCountForPrepareData(prepareDataConfig)
		for i := 0; i < restoreJobReplicas; i++ {
			//  create pvc from claims template, build volumes and volumeMounts
			for _, claim := range prepareDataConfig.RestoreVolumeClaimsTemplate.Templates {
				index := i + int(claimTemplate.StartingIndex)
				claim.Name = fmt.Sprintf("%s-%d", claim.Name, index)
				// HACK: add InstanceSet related labels to the PVC,
				// so that it can be managed by InstanceSet
				addItsManagingLabels(&claim, index)
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
func (r *RestoreManager) BuildPrepareDataJobs(reqCtx intctrlutil.RequestCtx, cli client.Client, backupSet BackupActionSet, target *dpv1alpha1.BackupStatusTarget, actionName string) ([]*batchv1.Job, error) {
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
		setServiceAccount(r.WorkerServiceAccount).
		attachBackupRepo()

	createPVCIfNotExistsAndBuildVolume := func(claim dpv1alpha1.RestoreVolumeClaim, identifier string) (*corev1.Volume, *corev1.VolumeMount, error) {
		if err := r.createPVCIfNotExist(reqCtx, cli, claim.ObjectMeta, claim.VolumeClaimSpec); err != nil {
			return nil, nil, err
		}
		return jobBuilder.buildPVCVolumeAndMount(claim.VolumeConfig, claim.Name, identifier)
	}
	for _, claim := range prepareDataConfig.RestoreVolumeClaims {
		// if only restore VolumeClaims, the sourceTargetPod must be consistent for each volumeClaims.
		// otherwise the restored data will be inconsistent.
		// create pvc from volumeClaims, set volume and volumeMount to jobBuilder
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
		// reset specific labels as addLabel does not override existing labels
		jobBuilder.resetSpecificLabels()
		if claimsTemplate != nil {
			//  create pvc from claims template, build volumes and volumeMounts
			for _, c := range claimsTemplate.Templates {
				// deepcopy to avoid modify the original object
				claim := *c.DeepCopy()
				index := i + int(claimsTemplate.StartingIndex)
				claim.Name = fmt.Sprintf("%s-%d", claim.Name, index)
				// HACK: add InstanceSet related labels to the PVC,
				// so that it can be managed by InstanceSet
				addItsManagingLabels(&claim, index)
				volume, volumeMount, err := createPVCIfNotExistsAndBuildVolume(claim, "dp-claim-tpl")
				if err != nil {
					return nil, err
				}
				for k, v := range claim.Labels {
					jobBuilder.addLabel(k, v)
				}
				jobBuilder.addToSpecificVolumesAndMounts(volume, volumeMount)
			}
		}
		sourceTargetPodName, err := GetSourcePodNameFromTarget(target, prepareDataConfig.RequiredPolicyForAllPodSelection, i)
		if err != nil {
			return nil, err
		}
		if target.PodSelector.Strategy == dpv1alpha1.PodSelectionStrategyAll && sourceTargetPodName == "" {
			// no need to recover the volume when the pod selection policy is 'All' and sourceTargetPodName is not found.
			continue
		}
		// build job and append
		job := jobBuilder.setJobName(jobBuilder.builderRestoreJobName(i)).addCommonEnv(sourceTargetPodName).build()
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
	target *dpv1alpha1.BackupStatusTarget,
	populatePVC *corev1.PersistentVolumeClaim,
	index int) (*batchv1.Job, error) {
	prepareDataConfig := r.Restore.Spec.PrepareDataConfig
	if prepareDataConfig == nil || prepareDataConfig.DataSourceRef == nil {
		return nil, nil
	}
	if !backupSet.ActionSet.HasPrepareDataStage() {
		return nil, nil
	}
	backupRepo, err := r.prepareBackupRepo(reqCtx, cli, backupSet)
	if err != nil {
		return nil, err
	}
	sourceTargetPodName, err := GetSourcePodNameFromTarget(target, prepareDataConfig.RequiredPolicyForAllPodSelection, 0)
	if err != nil {
		return nil, err
	}
	jobBuilder := newRestoreJobBuilder(r.Restore, backupSet, backupRepo, dpv1alpha1.PrepareData).
		setJobName(fmt.Sprintf("%s-%d", populatePVC.Name, index)).
		addLabel(DataProtectionPopulatePVCLabelKey, populatePVC.Name).
		setImage(backupSet.ActionSet.Spec.Restore.PrepareData.Image).
		setCommand(backupSet.ActionSet.Spec.Restore.PrepareData.Command).
		setServiceAccount(r.WorkerServiceAccount).
		attachBackupRepo().
		addCommonEnv(sourceTargetPodName)
	volume, volumeMount, err := jobBuilder.buildPVCVolumeAndMount(*prepareDataConfig.DataSourceRef, populatePVC.Name, "dp-claim")
	if err != nil {
		return nil, err
	}
	job := jobBuilder.addToSpecificVolumesAndMounts(volume, volumeMount).build()
	return job, nil
}

// GetExistingActionJobs returns jobs already recorded in Restore status for an in-flight action.
// PostReady first loads its immutable plan Secret, including completed
// predecessors, because the plan survives Job GC. Legacy restores retain the
// status-recorded Job fallback. PrepareData keeps its processing-only path.
func (r *RestoreManager) GetExistingActionJobs(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	stage dpv1alpha1.RestoreStage,
	backupName string,
	actionName string) ([]*batchv1.Job, error) {
	if stage == dpv1alpha1.PostReady {
		jobs, found, err := r.loadPostReadyExecutionPlan(reqCtx, cli, backupName, actionName)
		if err != nil {
			return nil, err
		}
		if found {
			return jobs, nil
		}
	}
	restoreActions := r.Restore.Status.Actions.PrepareData
	if stage == dpv1alpha1.PostReady {
		restoreActions = r.Restore.Status.Actions.PostReady
	}
	namespaces := []string{r.Restore.Namespace}
	if controllerNamespace := viper.GetString(constant.CfgKeyCtrlrMgrNS); controllerNamespace != "" && controllerNamespace != r.Restore.Namespace {
		namespaces = append(namespaces, controllerNamespace)
	}

	var jobs []*batchv1.Job
	missingActionJob := false
	jobKeyPrefix := fmt.Sprintf("%s/", constant.JobKind)
	for i := range restoreActions {
		action := restoreActions[i]
		if action.BackupName != backupName ||
			action.Name != actionName ||
			!strings.HasPrefix(action.ObjectKey, jobKeyPrefix) {
			continue
		}
		if stage != dpv1alpha1.PostReady && action.Status != dpv1alpha1.RestoreActionProcessing {
			continue
		}
		jobName := strings.TrimPrefix(action.ObjectKey, jobKeyPrefix)
		found := false
		for _, namespace := range namespaces {
			job := &batchv1.Job{}
			err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: jobName, Namespace: namespace}, job)
			if apierrors.IsNotFound(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if !r.isJobForRestoreAction(job) {
				continue
			}
			jobs = append(jobs, job)
			found = true
			break
		}
		if !found {
			missingActionJob = true
		}
	}
	if missingActionJob {
		return nil, nil
	}
	return jobs, nil
}

// ReconcileOrphanedPostReadyActions keeps persisted in-flight Jobs authoritative
// when a newer ActionSet removes or shortens postReady. Without this guard, the
// latest ActionSet loop can skip a still-running Job and mark postReady complete.
func (r *RestoreManager) ReconcileOrphanedPostReadyActions(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
) (bool, error) {
	stage, foundStage, err := r.loadPostReadyExecutionPlanStage(reqCtx, cli)
	if err != nil {
		return false, err
	}
	if foundStage {
		secret := &corev1.Secret{}
		if err := cli.Get(reqCtx.Ctx, types.NamespacedName{
			Namespace: r.Restore.Namespace,
			Name:      r.postReadyPlanSecretName(),
		}, secret); err != nil {
			return false, err
		}
		digest := secret.Annotations[postReadyPlanDigestAnnotationKey]
		for i := range stage.Actions {
			action := &stage.Actions[i]
			jobs := attachPostReadyPlanReference(
				action.Jobs, secret.Name, digest, string(r.Restore.UID))
			expectedCount, err := PostReadyActionExpectedJobCount(jobs)
			if err != nil {
				return false, err
			}
			completed, failed := r.AnalysisRestoreActionsWithBackupExpected(
				dpv1alpha1.PostReady, action.BackupName, action.ActionName, expectedCount)
			if failed {
				return false, intctrlutil.NewFatalError(fmt.Sprintf(
					"postReady action %s for backup %s has a terminal failed Job",
					action.ActionName, action.BackupName))
			}
			if completed {
				continue
			}
			pending := r.PendingPostReadyJobs(action.BackupName, action.ActionName, jobs)
			if len(pending) == 0 {
				return false, intctrlutil.NewFatalError(fmt.Sprintf(
					"postReady action %s for backup %s has no pending Job but is not complete",
					action.ActionName, action.BackupName))
			}
			pending, err = r.CreateJobsIfNotExist(reqCtx, cli, r.Restore, pending)
			if err != nil {
				return false, err
			}
			if err := r.ResumeNextSerialPostReadyJob(reqCtx, cli, pending); err != nil {
				return false, err
			}
			backupSet := BackupActionSet{Backup: &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{Name: action.BackupName},
			}}
			_, failed, err = r.CheckJobsDone(dpv1alpha1.PostReady, action.ActionName, backupSet, pending)
			if err != nil {
				return false, err
			}
			if failed {
				return false, intctrlutil.NewFatalError(fmt.Sprintf(
					"postReady action %s for backup %s has a terminal failed Job",
					action.ActionName, action.BackupName))
			}
			completed, _ = r.AnalysisRestoreActionsWithBackupExpected(
				dpv1alpha1.PostReady, action.BackupName, action.ActionName, expectedCount)
			if !completed {
				return false, nil
			}
		}
		return true, nil
	}

	// Before the stage marker exists, retain the legacy recovery path for an
	// upgrade with already-running frozen Jobs. New reconciles commit the full
	// stage plan before creating any Job and therefore never enter this path.
	currentActions := map[string]struct{}{}
	if r.Restore.Spec.ReadyConfig != nil {
		for i := range r.PostReadyBackupSets {
			backupSet := r.PostReadyBackupSets[i]
			if backupSet.Backup == nil || backupSet.ActionSet == nil || backupSet.ActionSet.Spec.Restore == nil {
				continue
			}
			for step := range backupSet.ActionSet.Spec.Restore.PostReady {
				currentActions[postReadyActionKey(backupSet.Backup.Name, fmt.Sprintf("%s-%d", dpv1alpha1.PostReady, step))] = struct{}{}
			}
		}
	}
	durablePlanKeys := map[string]struct{}{}

	namespaces := []string{r.Restore.Namespace}
	if controllerNamespace := viper.GetString(constant.CfgKeyCtrlrMgrNS); controllerNamespace != "" && controllerNamespace != r.Restore.Namespace {
		namespaces = append(namespaces, controllerNamespace)
	}
	orphaned := map[string][]*batchv1.Job{}
	for _, namespace := range namespaces {
		jobList := &batchv1.JobList{}
		if err := cli.List(reqCtx.Ctx, jobList,
			client.InNamespace(namespace),
			client.MatchingLabels{DataProtectionRestoreLabelKey: r.Restore.Name}); err != nil {
			return false, err
		}
		for i := range jobList.Items {
			job := &jobList.Items[i]
			if !r.isJobForRestoreAction(job) || !hasPostReadyFrozenContract(job) {
				continue
			}
			backupName, actionName := postReadyActionIdentity(job)
			if backupName == "" || actionName == "" {
				return false, intctrlutil.NewFatalError(fmt.Sprintf(
					"postReady job %s/%s has no recoverable action identity", job.Namespace, job.Name))
			}
			key := postReadyActionKey(backupName, actionName)
			if _, ok := currentActions[key]; ok {
				continue
			}
			if _, ok := durablePlanKeys[key]; ok {
				continue
			}
			orphaned[key] = append(orphaned[key], job.DeepCopy())
		}
	}

	orphanedKeys := make([]string, 0, len(orphaned))
	for key := range orphaned {
		orphanedKeys = append(orphanedKeys, key)
	}
	sort.Strings(orphanedKeys)
	for _, key := range orphanedKeys {
		jobs := orphaned[key]
		backupName, actionName := splitPostReadyActionKey(key)
		plan, err := postReadyTargetPlan(jobs[0])
		if err != nil {
			return false, err
		}
		if len(plan) != len(jobs) {
			return false, intctrlutil.NewFatalError(fmt.Sprintf(
				"in-flight postReady action %s for backup %s is incomplete and no longer exists in the ActionSet",
				actionName, backupName))
		}
		policy, err := postReadyExecutionPolicyForJob(jobs[0])
		if err != nil {
			return false, err
		}
		contract := postReadyActionContractForJob(jobs[0])
		identities := map[string]struct{}{}
		for i := range jobs {
			jobPlan, err := postReadyTargetPlan(jobs[i])
			if err != nil {
				return false, err
			}
			jobPolicy, err := postReadyExecutionPolicyForJob(jobs[i])
			if err != nil {
				return false, err
			}
			identity := postReadyTargetIdentity(jobs[i])
			index := postReadyJobIndex(jobs[i].Name)
			if strings.Join(jobPlan, "\x00") != strings.Join(plan, "\x00") ||
				jobPolicy != policy || postReadyActionContractForJob(jobs[i]) != contract ||
				index < 0 || index >= len(plan) || identity != plan[index] {
				return false, intctrlutil.NewFatalError(fmt.Sprintf(
					"in-flight postReady action %s for backup %s has an inconsistent frozen contract",
					actionName, backupName))
			}
			if _, ok := identities[identity]; ok {
				return false, intctrlutil.NewFatalError(fmt.Sprintf(
					"in-flight postReady action %s for backup %s has a duplicate frozen target",
					actionName, backupName))
			}
			identities[identity] = struct{}{}
		}
		backupSet := BackupActionSet{Backup: &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: backupName}}}
		completed, failed, err := r.CheckJobsDone(dpv1alpha1.PostReady, actionName, backupSet, jobs)
		if err != nil {
			return false, err
		}
		if failed {
			return false, intctrlutil.NewFatalError(fmt.Sprintf(
				"in-flight postReady action %s for backup %s failed after it was removed from the ActionSet",
				actionName, backupName))
		}
		if !completed {
			return false, nil
		}
	}
	return true, nil
}

func postReadyActionKey(backupName, actionName string) string {
	return backupName + "\x00" + actionName
}

func splitPostReadyActionKey(key string) (string, string) {
	parts := strings.SplitN(key, "\x00", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func (r *RestoreManager) postReadyPlanSecretName() string {
	identity := strings.Join([]string{
		r.Restore.Namespace,
		r.Restore.Name,
		string(r.Restore.UID),
		string(dpv1alpha1.PostReady),
	}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return fmt.Sprintf("postready-stage-%x", digest[:16])
}

func postReadyPlanMarkerValue(planName, digest string) string {
	return planName + "@" + digest
}

func (r *RestoreManager) postReadyPlanMarker() (string, bool) {
	if r.Restore.Annotations == nil {
		return "", false
	}
	value, ok := r.Restore.Annotations[postReadyPlanMarkerAnnotationKey]
	return value, ok
}

func (r *RestoreManager) ensurePostReadyPlanMarker(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	planName, digest string,
) error {
	expected := postReadyPlanMarkerValue(planName, digest)
	if current, ok := r.postReadyPlanMarker(); ok {
		if current != expected {
			return intctrlutil.NewFatalError(fmt.Sprintf(
				"restore %s/%s has a conflicting postReady execution plan marker",
				r.Restore.Namespace, r.Restore.Name))
		}
		return nil
	}
	original := r.Restore.DeepCopy()
	if r.Restore.Annotations == nil {
		r.Restore.Annotations = map[string]string{}
	}
	r.Restore.Annotations[postReadyPlanMarkerAnnotationKey] = expected
	return cli.Patch(reqCtx.Ctx, r.Restore, client.MergeFrom(original))
}

func postReadyPlanIdentityForJobs(jobs []*batchv1.Job) (string, string, bool, error) {
	if len(jobs) == 0 {
		return "", "", false, nil
	}
	backupName, actionName := postReadyActionIdentity(jobs[0])
	if backupName == "" && actionName == "" {
		return "", "", false, nil
	}
	if backupName == "" || actionName == "" {
		return "", "", false, intctrlutil.NewFatalError("postReady jobs have an incomplete action identity")
	}
	for i := 1; i < len(jobs); i++ {
		jobBackupName, jobActionName := postReadyActionIdentity(jobs[i])
		if jobBackupName != backupName || jobActionName != actionName {
			return "", "", false, intctrlutil.NewFatalError("postReady jobs have inconsistent action identities")
		}
	}
	return backupName, actionName, true, nil
}

func canonicalPostReadyPlanJobs(jobs []*batchv1.Job) ([]batchv1.Job, error) {
	canonical := make([]batchv1.Job, 0, len(jobs))
	for i := range jobs {
		if jobs[i] == nil {
			return nil, intctrlutil.NewFatalError("postReady execution plan contains a nil Job")
		}
		job := jobs[i].DeepCopy()
		job.TypeMeta = metav1.TypeMeta{APIVersion: batchv1.SchemeGroupVersion.String(), Kind: "Job"}
		job.ResourceVersion = ""
		job.UID = ""
		job.Generation = 0
		job.CreationTimestamp = metav1.Time{}
		job.DeletionTimestamp = nil
		job.DeletionGracePeriodSeconds = nil
		job.ManagedFields = nil
		job.OwnerReferences = nil
		job.Finalizers = []string{dptypes.DataProtectionFinalizerName}
		job.Status = batchv1.JobStatus{}
		if job.Annotations != nil {
			delete(job.Annotations, postReadyPlanNameAnnotationKey)
			delete(job.Annotations, postReadyPlanDigestAnnotationKey)
			delete(job.Annotations, postReadyPlanRestoreUIDAnnotationKey)
		}
		canonical = append(canonical, *job)
	}
	sort.Slice(canonical, func(i, j int) bool {
		return postReadyJobIndex(canonical[i].Name) < postReadyJobIndex(canonical[j].Name)
	})
	var expectedPolicy dpv1alpha1.PostReadyExecutionPolicy
	var expectedContract string
	var expectedTargetPlan string
	for i := range canonical {
		if postReadyJobIndex(canonical[i].Name) != i {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady execution plan has non-contiguous Job ordinal at %s/%s",
				canonical[i].Namespace, canonical[i].Name))
		}
		policy, err := postReadyExecutionPolicyForJob(&canonical[i])
		if err != nil {
			return nil, err
		}
		plan, err := postReadyTargetPlan(&canonical[i])
		if err != nil {
			return nil, err
		}
		if len(plan) != len(canonical) || plan[i] != postReadyTargetIdentity(&canonical[i]) {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady job %s/%s target identity does not match the complete execution plan",
				canonical[i].Namespace, canonical[i].Name))
		}
		contract := postReadyActionContractForJob(&canonical[i])
		if contract == "" {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady job %s/%s has no frozen action contract",
				canonical[i].Namespace, canonical[i].Name))
		}
		serializedTargetPlan := canonical[i].Annotations[postReadyTargetPlanAnnotationKey]
		if i == 0 {
			expectedPolicy = policy
			expectedContract = contract
			expectedTargetPlan = serializedTargetPlan
		} else if policy != expectedPolicy || contract != expectedContract || serializedTargetPlan != expectedTargetPlan {
			return nil, intctrlutil.NewFatalError("postReady execution plan contains inconsistent Job contracts")
		}
	}
	return canonical, nil
}

func postReadyPlanDigest(payload []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(payload))
}

func attachPostReadyPlanReference(jobs []batchv1.Job, planName, digest, restoreUID string) []*batchv1.Job {
	result := make([]*batchv1.Job, 0, len(jobs))
	for i := range jobs {
		job := jobs[i].DeepCopy()
		if job.Annotations == nil {
			job.Annotations = map[string]string{}
		}
		job.Annotations[postReadyPlanNameAnnotationKey] = planName
		job.Annotations[postReadyPlanDigestAnnotationKey] = digest
		job.Annotations[postReadyPlanRestoreUIDAnnotationKey] = restoreUID
		result = append(result, job)
	}
	return result
}

func canonicalPostReadyStagePlan(plan postReadyExecutionPlan) (postReadyExecutionPlan, error) {
	if len(plan.Actions) == 0 {
		return postReadyExecutionPlan{}, intctrlutil.NewFatalError("postReady stage execution plan has no actions")
	}
	seen := map[string]struct{}{}
	canonicalActions := make([]postReadyActionExecutionPlan, 0, len(plan.Actions))
	for i := range plan.Actions {
		action := plan.Actions[i]
		if action.Order != i || action.BackupName == "" || action.ActionName == "" || len(action.Jobs) == 0 {
			return postReadyExecutionPlan{}, intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady stage action %d has an invalid ordered identity", i))
		}
		key := postReadyActionKey(action.BackupName, action.ActionName)
		if _, ok := seen[key]; ok {
			return postReadyExecutionPlan{}, intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady stage execution plan has duplicate action %s/%s", action.BackupName, action.ActionName))
		}
		seen[key] = struct{}{}
		jobPointers := make([]*batchv1.Job, 0, len(action.Jobs))
		for j := range action.Jobs {
			jobPointers = append(jobPointers, action.Jobs[j].DeepCopy())
		}
		canonicalJobs, err := canonicalPostReadyPlanJobs(jobPointers)
		if err != nil {
			return postReadyExecutionPlan{}, err
		}
		for j := range canonicalJobs {
			backupName, actionName := postReadyActionIdentity(&canonicalJobs[j])
			if backupName != action.BackupName || actionName != action.ActionName {
				return postReadyExecutionPlan{}, intctrlutil.NewFatalError(fmt.Sprintf(
					"postReady stage action %s/%s contains a Job for another action",
					action.BackupName, action.ActionName))
			}
		}
		canonicalActions = append(canonicalActions, postReadyActionExecutionPlan{
			Order: action.Order, BackupName: action.BackupName, ActionName: action.ActionName, Jobs: canonicalJobs,
		})
	}
	plan.Actions = canonicalActions
	return plan, nil
}

func (r *RestoreManager) findPostReadyStageAction(
	plan *postReadyExecutionPlan,
	backupName, actionName string,
) (*postReadyActionExecutionPlan, bool) {
	for i := range plan.Actions {
		if plan.Actions[i].BackupName == backupName && plan.Actions[i].ActionName == actionName {
			return &plan.Actions[i], true
		}
	}
	return nil, false
}

func (r *RestoreManager) loadPostReadyExecutionPlanStage(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	expected ...*postReadyExecutionPlan,
) (*postReadyExecutionPlan, bool, error) {
	secretName := r.postReadyPlanSecretName()
	marker, hasMarker := r.postReadyPlanMarker()
	secret := &corev1.Secret{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: r.Restore.Namespace, Name: secretName}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			if hasMarker {
				return nil, true, intctrlutil.NewFatalError(fmt.Sprintf(
					"postReady stage execution plan %s/%s is missing", r.Restore.Namespace, secretName))
			}
			return nil, false, nil
		}
		return nil, false, err
	}
	if secret.DeletionTimestamp != nil {
		return nil, true, intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady stage execution plan %s/%s is terminating", secret.Namespace, secret.Name))
	}
	if secret.Type != postReadyPlanSecretType || secret.Immutable == nil || !*secret.Immutable {
		return nil, true, intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady stage execution plan %s/%s is not an immutable plan Secret", secret.Namespace, secret.Name))
	}
	controller := metav1.GetControllerOf(secret)
	if controller == nil ||
		controller.APIVersion != dpv1alpha1.SchemeGroupVersion.String() ||
		controller.Kind != dptypes.RestoreKind ||
		controller.Name != r.Restore.Name ||
		controller.UID != r.Restore.UID ||
		controller.Controller == nil || !*controller.Controller ||
		controller.BlockOwnerDeletion == nil || !*controller.BlockOwnerDeletion ||
		secret.Annotations[postReadyPlanRestoreUIDAnnotationKey] != string(r.Restore.UID) {
		return nil, true, intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady stage execution plan %s/%s is not owned by restore %s/%s",
			secret.Namespace, secret.Name, r.Restore.Namespace, r.Restore.Name))
	}
	payload := secret.Data[postReadyPlanDataKey]
	if len(payload) == 0 {
		return nil, true, intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady stage execution plan %s/%s has no canonical payload", secret.Namespace, secret.Name))
	}
	digest := postReadyPlanDigest(payload)
	if secret.Annotations == nil || secret.Annotations[postReadyPlanDigestAnnotationKey] != digest {
		return nil, true, intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady stage execution plan %s/%s has an invalid payload digest", secret.Namespace, secret.Name))
	}
	var plan postReadyExecutionPlan
	if err := json.Unmarshal(payload, &plan); err != nil {
		return nil, true, intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady stage execution plan %s/%s has an invalid payload: %v", secret.Namespace, secret.Name, err))
	}
	if plan.Version != postReadyPlanVersion ||
		plan.RestoreNamespace != r.Restore.Namespace || plan.RestoreName != r.Restore.Name ||
		plan.RestoreUID != string(r.Restore.UID) ||
		plan.SourceBackupNamespace != r.Restore.Spec.Backup.Namespace ||
		plan.SourceBackupName != r.Restore.Spec.Backup.Name {
		return nil, true, intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady stage execution plan %s/%s does not match restore identity", secret.Namespace, secret.Name))
	}
	canonical, err := canonicalPostReadyStagePlan(plan)
	if err != nil {
		return nil, true, err
	}
	canonicalPayload, err := json.Marshal(canonical)
	if err != nil {
		return nil, true, err
	}
	if string(canonicalPayload) != string(payload) {
		return nil, true, intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady stage execution plan %s/%s payload is not canonical", secret.Namespace, secret.Name))
	}
	expectedMarker := postReadyPlanMarkerValue(secret.Name, digest)
	if hasMarker && marker != expectedMarker {
		return nil, true, intctrlutil.NewFatalError(fmt.Sprintf(
			"restore %s/%s postReady stage execution plan marker does not match %s/%s",
			r.Restore.Namespace, r.Restore.Name, secret.Namespace, secret.Name))
	}
	if !hasMarker {
		if len(expected) != 1 || expected[0] == nil {
			return nil, true, intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady stage execution plan %s/%s has no committed marker and no complete expected stage",
				secret.Namespace, secret.Name))
		}
		expectedCanonical, err := canonicalPostReadyStagePlan(*expected[0])
		if err != nil {
			return nil, true, err
		}
		expectedPayload, err := json.Marshal(expectedCanonical)
		if err != nil {
			return nil, true, err
		}
		if string(expectedPayload) != string(payload) {
			return nil, true, intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady stage execution plan %s/%s does not match the complete expected stage",
				secret.Namespace, secret.Name))
		}
		if err := r.migratePostReadyJobsBeforeStageCommit(reqCtx, cli, &canonical, secret.Name, digest); err != nil {
			return nil, true, err
		}
	}
	if err := r.ensurePostReadyPlanMarker(reqCtx, cli, secret.Name, digest); err != nil {
		return nil, true, err
	}
	return &canonical, true, nil
}

func (r *RestoreManager) persistPostReadyExecutionPlanStage(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	actions []postReadyActionExecutionPlan,
) (*postReadyExecutionPlan, error) {
	canonical, err := canonicalPostReadyStagePlan(postReadyExecutionPlan{
		Version:               postReadyPlanVersion,
		RestoreNamespace:      r.Restore.Namespace,
		RestoreName:           r.Restore.Name,
		RestoreUID:            string(r.Restore.UID),
		SourceBackupNamespace: r.Restore.Spec.Backup.Namespace,
		SourceBackupName:      r.Restore.Spec.Backup.Name,
		Actions:               actions,
	})
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return nil, err
	}
	if len(payload) > postReadyPlanMaxPayloadBytes {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady stage execution plan payload is %d bytes and exceeds the Secret limit", len(payload)))
	}
	digest := postReadyPlanDigest(payload)
	immutable := true
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.Restore.Namespace,
			Name:      r.postReadyPlanSecretName(),
			Labels: map[string]string{
				DataProtectionRestoreLabelKey:          r.Restore.Name,
				DataProtectionRestoreNamespaceLabelKey: r.Restore.Namespace,
			},
			Annotations: map[string]string{
				postReadyPlanDigestAnnotationKey:     digest,
				postReadyPlanRestoreUIDAnnotationKey: string(r.Restore.UID),
			},
		},
		Immutable: &immutable,
		Type:      postReadyPlanSecretType,
		Data:      map[string][]byte{postReadyPlanDataKey: payload},
	}
	if err := controllerutil.SetControllerReference(r.Restore, secret, r.Schema); err != nil {
		return nil, err
	}
	if err := cli.Create(reqCtx.Ctx, secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
		persisted, found, loadErr := r.loadPostReadyExecutionPlanStage(reqCtx, cli, &canonical)
		if loadErr != nil {
			return nil, loadErr
		}
		if !found {
			return nil, intctrlutil.NewFatalError("postReady stage execution plan disappeared after AlreadyExists")
		}
		return persisted, nil
	}
	if err := r.migratePostReadyJobsBeforeStageCommit(reqCtx, cli, &canonical, secret.Name, digest); err != nil {
		return nil, err
	}
	if err := r.ensurePostReadyPlanMarker(reqCtx, cli, secret.Name, digest); err != nil {
		return nil, err
	}
	return &canonical, nil
}

func (r *RestoreManager) loadPostReadyExecutionPlan(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	backupName, actionName string,
) ([]*batchv1.Job, bool, error) {
	stage, found, err := r.loadPostReadyExecutionPlanStage(reqCtx, cli)
	if err != nil || !found {
		return nil, found, err
	}
	action, ok := r.findPostReadyStageAction(stage, backupName, actionName)
	if !ok {
		return nil, true, intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady stage execution plan does not contain action %s/%s", backupName, actionName))
	}
	secretName := r.postReadyPlanSecretName()
	secret := &corev1.Secret{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: r.Restore.Namespace, Name: secretName}, secret); err != nil {
		return nil, true, err
	}
	digest := secret.Annotations[postReadyPlanDigestAnnotationKey]
	return attachPostReadyPlanReference(action.Jobs, secret.Name, digest, string(r.Restore.UID)), true, nil
}

func (r *RestoreManager) persistPostReadyExecutionPlan(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	backupName, actionName string,
	jobs []*batchv1.Job,
) ([]*batchv1.Job, error) {
	canonical, err := canonicalPostReadyPlanJobs(jobs)
	if err != nil {
		return nil, err
	}
	stage, err := r.persistPostReadyExecutionPlanStage(reqCtx, cli, []postReadyActionExecutionPlan{{
		Order: 0, BackupName: backupName, ActionName: actionName, Jobs: canonical,
	}})
	if err != nil {
		return nil, err
	}
	action, ok := r.findPostReadyStageAction(stage, backupName, actionName)
	if !ok {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady stage execution plan does not contain action %s/%s", backupName, actionName))
	}
	secretName := r.postReadyPlanSecretName()
	secret := &corev1.Secret{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: r.Restore.Namespace, Name: secretName}, secret); err != nil {
		return nil, err
	}
	return attachPostReadyPlanReference(
		action.Jobs, secret.Name, secret.Annotations[postReadyPlanDigestAnnotationKey], string(r.Restore.UID)), nil
}

func normalizePostReadyJobSpec(job *batchv1.Job, scheme *runtime.Scheme) batchv1.JobSpec {
	copy := job.DeepCopy()
	if scheme != nil {
		scheme.Default(copy)
	}
	if copy.Spec.Parallelism != nil && *copy.Spec.Parallelism == 1 {
		copy.Spec.Parallelism = nil
	}
	if copy.Spec.Completions != nil && *copy.Spec.Completions == 1 {
		copy.Spec.Completions = nil
	}
	if copy.Spec.BackoffLimit != nil && *copy.Spec.BackoffLimit == 6 {
		copy.Spec.BackoffLimit = nil
	}
	if copy.Spec.CompletionMode != nil && *copy.Spec.CompletionMode == batchv1.NonIndexedCompletion {
		copy.Spec.CompletionMode = nil
	}
	if copy.Spec.Suspend != nil && !*copy.Spec.Suspend {
		copy.Spec.Suspend = nil
	}
	if copy.Spec.ManualSelector != nil && !*copy.Spec.ManualSelector {
		copy.Spec.ManualSelector = nil
	}
	if isGeneratedPostReadyJobSelector(copy.Spec.Selector, string(copy.UID)) {
		copy.Spec.Selector = nil
	}
	generatedLabels := map[string]string{
		"batch.kubernetes.io/controller-uid": string(copy.UID),
		"controller-uid":                     string(copy.UID),
		"batch.kubernetes.io/job-name":       copy.Name,
		"job-name":                           copy.Name,
	}
	for key, expected := range generatedLabels {
		if expected != "" && copy.Spec.Template.Labels[key] == expected {
			delete(copy.Spec.Template.Labels, key)
		}
	}
	if len(copy.Spec.Template.Labels) == 0 {
		copy.Spec.Template.Labels = nil
	}
	podSpec := &copy.Spec.Template.Spec
	if podSpec.DeprecatedServiceAccount == podSpec.ServiceAccountName {
		podSpec.DeprecatedServiceAccount = ""
	}
	for i := range podSpec.Volumes {
		normalizePostReadyVolumeDefaults(&podSpec.Volumes[i])
	}
	if podSpec.TerminationGracePeriodSeconds != nil && *podSpec.TerminationGracePeriodSeconds == 30 {
		podSpec.TerminationGracePeriodSeconds = nil
	}
	if podSpec.DNSPolicy == corev1.DNSClusterFirst {
		podSpec.DNSPolicy = ""
	}
	if podSpec.SchedulerName == corev1.DefaultSchedulerName {
		podSpec.SchedulerName = ""
	}
	if podSpec.SecurityContext != nil && apiequality.Semantic.DeepEqual(
		podSpec.SecurityContext, &corev1.PodSecurityContext{}) {
		podSpec.SecurityContext = nil
	}
	for i := range podSpec.InitContainers {
		normalizePostReadyContainerDefaults(&podSpec.InitContainers[i])
	}
	for i := range podSpec.Containers {
		normalizePostReadyContainerDefaults(&podSpec.Containers[i])
	}
	return copy.Spec
}

func isGeneratedPostReadyJobSelector(selector *metav1.LabelSelector, jobUID string) bool {
	if selector == nil || jobUID == "" || len(selector.MatchExpressions) != 0 || len(selector.MatchLabels) != 1 {
		return false
	}
	for key, value := range selector.MatchLabels {
		return (key == "controller-uid" || key == "batch.kubernetes.io/controller-uid") && value == jobUID
	}
	return false
}

func normalizePostReadyContainerDefaults(container *corev1.Container) {
	if container.TerminationMessagePath == corev1.TerminationMessagePathDefault {
		container.TerminationMessagePath = ""
	}
	if container.TerminationMessagePolicy == corev1.TerminationMessageReadFile {
		container.TerminationMessagePolicy = ""
	}
	imageName := container.Image
	if slash := strings.LastIndexByte(imageName, '/'); slash >= 0 {
		imageName = imageName[slash+1:]
	}
	defaultPullPolicy := corev1.PullIfNotPresent
	if !strings.Contains(imageName, "@") {
		colon := strings.LastIndexByte(imageName, ':')
		if colon < 0 || imageName[colon+1:] == "latest" {
			defaultPullPolicy = corev1.PullAlways
		}
	}
	if container.ImagePullPolicy == defaultPullPolicy {
		container.ImagePullPolicy = ""
	}
	for i := range container.Env {
		if container.Env[i].ValueFrom != nil && container.Env[i].ValueFrom.FieldRef != nil &&
			container.Env[i].ValueFrom.FieldRef.APIVersion == "v1" {
			container.Env[i].ValueFrom.FieldRef.APIVersion = ""
		}
	}
}

func normalizePostReadyVolumeDefaults(volume *corev1.Volume) {
	const defaultMode = int32(0o644)
	if volume.DownwardAPI != nil {
		if volume.DownwardAPI.DefaultMode != nil && *volume.DownwardAPI.DefaultMode == defaultMode {
			volume.DownwardAPI.DefaultMode = nil
		}
		for i := range volume.DownwardAPI.Items {
			if fieldRef := volume.DownwardAPI.Items[i].FieldRef; fieldRef != nil && fieldRef.APIVersion == "v1" {
				fieldRef.APIVersion = ""
			}
		}
	}
	if volume.Secret != nil && volume.Secret.DefaultMode != nil && *volume.Secret.DefaultMode == defaultMode {
		volume.Secret.DefaultMode = nil
	}
	if volume.ConfigMap != nil && volume.ConfigMap.DefaultMode != nil && *volume.ConfigMap.DefaultMode == defaultMode {
		volume.ConfigMap.DefaultMode = nil
	}
	if volume.Projected != nil {
		if volume.Projected.DefaultMode != nil && *volume.Projected.DefaultMode == defaultMode {
			volume.Projected.DefaultMode = nil
		}
		for i := range volume.Projected.Sources {
			downwardAPI := volume.Projected.Sources[i].DownwardAPI
			if downwardAPI == nil {
				continue
			}
			for j := range downwardAPI.Items {
				if fieldRef := downwardAPI.Items[j].FieldRef; fieldRef != nil && fieldRef.APIVersion == "v1" {
					fieldRef.APIVersion = ""
				}
			}
		}
	}
}

func (r *RestoreManager) postReadyJobSpecsEqual(desired, existing *batchv1.Job) bool {
	desiredSpec := normalizePostReadyJobSpec(desired, r.Schema)
	existingSpec := normalizePostReadyJobSpec(existing, r.Schema)
	return apiequality.Semantic.DeepEqual(desiredSpec, existingSpec)
}

func (r *RestoreManager) migratePostReadyJobsBeforeStageCommit(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	stage *postReadyExecutionPlan,
	planName, digest string,
) error {
	for i := range stage.Actions {
		desiredJobs := attachPostReadyPlanReference(
			stage.Actions[i].Jobs, planName, digest, string(r.Restore.UID))
		for j := range desiredJobs {
			existing := &batchv1.Job{}
			if err := cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(desiredJobs[j]), existing); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return err
			}
			if existing.DeletionTimestamp != nil || existing.UID == "" || !r.isJobForRestoreAction(existing) {
				return intctrlutil.NewFatalError(fmt.Sprintf(
					"legacy postReady job %s/%s has no stable restore ownership", existing.Namespace, existing.Name))
			}
			desiredBackupName, desiredActionName := postReadyActionIdentity(desiredJobs[j])
			existingBackupName, existingActionName := postReadyActionIdentity(existing)
			if existingBackupName != desiredBackupName || existingActionName != desiredActionName {
				return intctrlutil.NewFatalError(fmt.Sprintf(
					"legacy postReady job %s/%s action identity does not match the stage plan",
					existing.Namespace, existing.Name))
			}
			controller := metav1.GetControllerOf(existing)
			if existing.Namespace == r.Restore.Namespace && (controller == nil || controller.UID != r.Restore.UID) {
				return intctrlutil.NewFatalError(fmt.Sprintf(
					"legacy postReady job %s/%s is not owned by restore %s/%s",
					existing.Namespace, existing.Name, r.Restore.Namespace, r.Restore.Name))
			}
			existingPlanName := existing.Annotations[postReadyPlanNameAnnotationKey]
			existingDigest := existing.Annotations[postReadyPlanDigestAnnotationKey]
			existingRestoreUID := existing.Annotations[postReadyPlanRestoreUIDAnnotationKey]
			if existingPlanName != "" || existingDigest != "" || existingRestoreUID != "" {
				if existingPlanName != planName || existingDigest != digest || existingRestoreUID != string(r.Restore.UID) {
					return intctrlutil.NewFatalError(fmt.Sprintf(
						"postReady job %s/%s has a conflicting committed stage reference",
						existing.Namespace, existing.Name))
				}
				if err := r.validateExistingRestoreActionJob(desiredJobs[j], existing); err != nil {
					return err
				}
				continue
			}
			if !r.postReadyJobSpecsEqual(desiredJobs[j], existing) {
				return intctrlutil.NewFatalError(fmt.Sprintf(
					"legacy postReady job %s/%s executable spec does not match the stage plan",
					existing.Namespace, existing.Name))
			}
			original := existing.DeepCopy()
			if existing.Annotations == nil {
				existing.Annotations = map[string]string{}
			}
			existing.Annotations[postReadyPlanNameAnnotationKey] = planName
			existing.Annotations[postReadyPlanDigestAnnotationKey] = digest
			existing.Annotations[postReadyPlanRestoreUIDAnnotationKey] = string(r.Restore.UID)
			controllerutil.AddFinalizer(existing, dptypes.DataProtectionFinalizerName)
			if err := cli.Patch(reqCtx.Ctx, existing, client.MergeFrom(original)); err != nil {
				return err
			}
		}
	}
	return nil
}

func postReadyActionIdentity(job *batchv1.Job) (string, string) {
	if job.Annotations != nil {
		if backupName := job.Annotations[postReadyBackupNameAnnotationKey]; backupName != "" {
			if actionName := job.Annotations[postReadyActionNameAnnotationKey]; actionName != "" {
				return backupName, actionName
			}
		}
	}
	backupName := ""
	for _, container := range job.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if env.Name == dptypes.DPBackupName {
				backupName = env.Value
				break
			}
		}
		if backupName != "" {
			break
		}
	}
	lastSeparator := strings.LastIndexByte(job.Name, '-')
	if lastSeparator < 0 {
		return backupName, ""
	}
	stepSeparator := strings.LastIndexByte(job.Name[:lastSeparator], '-')
	if stepSeparator < 0 || stepSeparator+1 == lastSeparator {
		return backupName, ""
	}
	step := job.Name[stepSeparator+1 : lastSeparator]
	if _, err := strconv.Atoi(step); err != nil {
		return backupName, ""
	}
	return backupName, fmt.Sprintf("%s-%s", dpv1alpha1.PostReady, step)
}

func (r *RestoreManager) isJobForRestoreAction(job *batchv1.Job) bool {
	if job.Labels[DataProtectionRestoreLabelKey] != r.Restore.Name {
		return false
	}
	restoreNamespace := job.Labels[DataProtectionRestoreNamespaceLabelKey]
	if job.Namespace != r.Restore.Namespace {
		return restoreNamespace == r.Restore.Namespace
	}
	return restoreNamespace == "" || restoreNamespace == r.Restore.Namespace
}

// BuildPostReadyActionJobs builds the post ready jobs.
func (r *RestoreManager) BuildPostReadyActionJobs(reqCtx intctrlutil.RequestCtx, cli client.Client, backupSet BackupActionSet, target *dpv1alpha1.BackupStatusTarget, step int) ([]*batchv1.Job, error) {
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
	getTargetPodList := func(labelSelector metav1.LabelSelector, msgKey string) (*corev1.PodList, error) {
		targetPodList, err := utils.GetPodListByLabelSelector(reqCtx, cli, &labelSelector)
		if err != nil {
			return nil, err
		}
		if len(targetPodList.Items) == 0 {
			return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue, "can not found any pod by spec.readyConfig.%s.target.podSelector", msgKey)
		}
		return targetPodList, nil
	}

	buildJobName := func(index int) string {
		jobName := fmt.Sprintf("restore-post-ready-%s-%s-%d-%d", r.Restore.UID[:8], backupSet.Backup.Name, step, index)
		return cutJobName(jobName)
	}
	jobBuilder := newRestoreJobBuilder(r.Restore, backupSet, backupRepo, dpv1alpha1.PostReady)
	buildJobsForJobAction := func() ([]*batchv1.Job, error) {
		jobAction := r.Restore.Spec.ReadyConfig.JobAction
		if jobAction == nil {
			return nil, intctrlutil.NewFatalError("spec.readyConfig.jobAction can not be empty")
		}
		podSelector := jobAction.Target.PodSelector
		if podSelector.LabelSelector == nil {
			return nil, intctrlutil.NewFatalError("spec.readyConfig.jobAction.podSelector.labelSelector can not be empty")
		}
		targetPodList, err := getTargetPodList(*podSelector.LabelSelector, "jobAction")
		if err != nil {
			return nil, err
		}
		sort.Sort(intctrlutil.ByPodName(targetPodList.Items))
		frozenTargetByName, hasFrozenPlan, err := r.getFrozenPostReadySourceTargets(
			reqCtx, cli, types.NamespacedName{Namespace: r.Restore.Namespace, Name: buildJobName(0)})
		if err != nil {
			return nil, err
		}
		buildJob := func(targetPod *corev1.Pod, sourceTargetPodName string, index int) *batchv1.Job {
			if boolptr.IsSetToTrue(actionSpec.Job.RunOnTargetPodNode) {
				jobBuilder.resetSpecificVolumesAndMounts()
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
			return jobBuilder.setImage(actionSpec.Job.Image).
				setJobName(buildJobName(index)).
				addCommonEnv(sourceTargetPodName).
				attachBackupRepo().
				setCommand(actionSpec.Job.Command).
				setToleration(targetPod.Spec.Tolerations).
				addTargetPodAndCredentialEnv(targetPod, readyConfig.ConnectionCredential, &target.BackupTarget).
				setServiceAccount(r.WorkerServiceAccount).
				build()
		}

		if podSelector.Strategy == dpv1alpha1.PodSelectionStrategyAny && !hasFrozenPlan {
			targetPod := utils.GetFirstIndexRunningPod(targetPodList)
			if targetPod == nil {
				return nil, fmt.Errorf("can not found any running pod by spec.readyConfig.jobAction.target.podSelector")
			}
			targetPodList.Items = []corev1.Pod{*targetPod}
		}
		var jobs []*batchv1.Job
		for i := range targetPodList.Items {
			targetName := types.NamespacedName{
				Namespace: targetPodList.Items[i].Namespace,
				Name:      targetPodList.Items[i].Name,
			}.String()
			frozenTarget, selectedByFrozenPlan := frozenTargetByName[targetName]
			if hasFrozenPlan && !selectedByFrozenPlan {
				continue
			}
			sourceTargetPodName := frozenTarget.source
			jobIndex := i
			if hasFrozenPlan {
				jobIndex = frozenTarget.ordinal
			}
			if !hasFrozenPlan {
				sourceTargetPodName, err = GetSourcePodNameFromTarget(target, jobAction.RequiredPolicyForAllPodSelection, i)
				if err != nil {
					return nil, err
				}
			}
			if target.PodSelector.Strategy == dpv1alpha1.PodSelectionStrategyAll && sourceTargetPodName == "" {
				// no need to recover the volume when the pod selection policy is 'All' and sourceTargetPodName is not found.
				continue
			}
			job := buildJob(&targetPodList.Items[i], sourceTargetPodName, jobIndex)
			setPostReadyTargetIdentity(job, &targetPodList.Items[i], sourceTargetPodName)
			jobs = append(jobs, job)
		}
		if hasFrozenPlan && len(jobs) != len(frozenTargetByName) {
			return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue,
				"not all frozen postReady target pods are currently available")
		}
		return jobs, nil
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
		sort.Sort(intctrlutil.ByPodName(targetPodList.Items))
		var restoreJobs []*batchv1.Job
		for i := range targetPodList.Items {
			containerName := actionSpec.Exec.Container
			if containerName == "" {
				containerName = targetPodList.Items[i].Spec.Containers[0].Name
			}
			args := append([]string{"-n", targetPodList.Items[i].Namespace, "exec", targetPodList.Items[i].Name, "-c", containerName, "--"}, actionSpec.Exec.Command...)
			jobBuilder.setImage(viper.GetString(constant.KBToolsImage)).setCommand([]string{"kubectl"}).setArgs(args).
				setJobName(buildJobName(i)).
				setToleration(targetPodList.Items[i].Spec.Tolerations)
			job := jobBuilder.build()
			// create exec job in kubeblocks namespace for security
			kbInstalledNamespace := viper.GetString(constant.CfgKeyCtrlrMgrNS)
			if kbInstalledNamespace != "" {
				job.Namespace = kbInstalledNamespace
				// use the dedicated ServiceAccount for executing "kubectl exec"
				job.Spec.Template.Spec.ServiceAccountName = viper.GetString(dptypes.CfgKeyExecWorkerServiceAccountName)
			}
			job.Labels[DataProtectionRestoreNamespaceLabelKey] = r.Restore.Namespace
			setPostReadyTargetIdentity(job, &targetPodList.Items[i], "")
			restoreJobs = append(restoreJobs, job)
		}
		return restoreJobs, nil
	}

	var jobs []*batchv1.Job
	if actionSpec.Job != nil {
		jobs, err = buildJobsForJobAction()
	} else {
		jobs, err = buildJobsForExecAction()
	}
	if err != nil {
		return nil, err
	}
	policy := dpv1alpha1.PostReadyExecutionPolicyParallel
	if isSerialPostReady(backupSet) {
		policy = dpv1alpha1.PostReadyExecutionPolicySerial
	}
	for i := range jobs {
		setPostReadyExecutionPolicy(jobs[i], policy)
	}
	if err := setPostReadyTargetPlan(jobs); err != nil {
		return nil, err
	}
	if err := r.setPostReadyActionContract(jobs, backupSet, step); err != nil {
		return nil, err
	}
	return jobs, nil
}

func isSerialPostReady(backupSet BackupActionSet) bool {
	return backupSet.ActionSet != nil &&
		backupSet.ActionSet.Spec.Restore != nil &&
		backupSet.ActionSet.Spec.Restore.PostReadyExecutionPolicy == dpv1alpha1.PostReadyExecutionPolicySerial
}

func setPostReadyExecutionPolicy(job *batchv1.Job, policy dpv1alpha1.PostReadyExecutionPolicy) {
	if job.Annotations == nil {
		job.Annotations = map[string]string{}
	}
	job.Annotations[postReadyExecutionPolicyAnnotationKey] = string(policy)
	if policy == dpv1alpha1.PostReadyExecutionPolicySerial {
		job.Spec.Suspend = boolptr.True()
	} else {
		job.Spec.Suspend = nil
	}
}

func setPostReadyTargetIdentity(job *batchv1.Job, pod *corev1.Pod, sourceTargetPodName string) {
	if job.Annotations == nil {
		job.Annotations = map[string]string{}
	}
	target := types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Name,
	}.String()
	job.Annotations[postReadyTargetIdentityAnnotationKey] = fmt.Sprintf("%s|source=%s", target, sourceTargetPodName)
}

func postReadyTargetIdentity(job *batchv1.Job) string {
	if job.Annotations == nil {
		return ""
	}
	return job.Annotations[postReadyTargetIdentityAnnotationKey]
}

type postReadyActionContract struct {
	ReadyConfig              *dpv1alpha1.ReadyConfig `json:"readyConfig,omitempty"`
	PostReady                []dpv1alpha1.ActionSpec `json:"postReady,omitempty"`
	KBToolsImage             string                  `json:"kbToolsImage,omitempty"`
	WorkerServiceAccount     string                  `json:"workerServiceAccount,omitempty"`
	ControllerNamespace      string                  `json:"controllerNamespace,omitempty"`
	ExecWorkerServiceAccount string                  `json:"execWorkerServiceAccount,omitempty"`
}

func (r *RestoreManager) setPostReadyActionContract(jobs []*batchv1.Job, backupSet BackupActionSet, step int) error {
	if backupSet.ActionSet == nil || backupSet.ActionSet.Spec.Restore == nil {
		return intctrlutil.NewFatalError("postReady action has no ActionSet restore contract")
	}
	if step < 0 || step >= len(backupSet.ActionSet.Spec.Restore.PostReady) {
		return intctrlutil.NewFatalError(fmt.Sprintf("postReady action step %d is out of range", step))
	}
	contract := postReadyActionContract{
		ReadyConfig:              r.Restore.Spec.ReadyConfig,
		PostReady:                backupSet.ActionSet.Spec.Restore.PostReady,
		KBToolsImage:             viper.GetString(constant.KBToolsImage),
		WorkerServiceAccount:     r.WorkerServiceAccount,
		ControllerNamespace:      viper.GetString(constant.CfgKeyCtrlrMgrNS),
		ExecWorkerServiceAccount: viper.GetString(dptypes.CfgKeyExecWorkerServiceAccountName),
	}
	serialized, err := json.Marshal(contract)
	if err != nil {
		return err
	}
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(serialized))
	actionName := fmt.Sprintf("%s-%d", dpv1alpha1.PostReady, step)
	for i := range jobs {
		if jobs[i].Annotations == nil {
			jobs[i].Annotations = map[string]string{}
		}
		jobs[i].Annotations[postReadyActionContractAnnotationKey] = digest
		jobs[i].Annotations[postReadyBackupNameAnnotationKey] = backupSet.Backup.Name
		jobs[i].Annotations[postReadyActionNameAnnotationKey] = actionName
	}
	return nil
}

func postReadyActionContractForJob(job *batchv1.Job) string {
	if job.Annotations == nil {
		return ""
	}
	return job.Annotations[postReadyActionContractAnnotationKey]
}

func splitPostReadyTargetIdentity(identity string) (target, source string) {
	const sourceMarker = "|source="
	parts := strings.SplitN(identity, sourceMarker, 2)
	if len(parts) == 1 {
		return identity, ""
	}
	return parts[0], parts[1]
}

type frozenPostReadyTarget struct {
	source  string
	ordinal int
}

// getFrozenPostReadySourceTargets returns the original target-to-source and
// target-to-ordinal mappings after a partial create. JobAction must use both
// while rebuilding specs so neither the backup path nor the Job name drifts
// when the selected target Pod set changes.
func (r *RestoreManager) getFrozenPostReadySourceTargets(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	firstJobKey types.NamespacedName,
) (map[string]frozenPostReadyTarget, bool, error) {
	existing := &batchv1.Job{}
	if err := cli.Get(reqCtx.Ctx, firstJobKey, existing); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if !r.isJobForRestoreAction(existing) {
		return nil, false, intctrlutil.NewFatalError(fmt.Sprintf(
			"restore job name collision: existing job %s/%s does not belong to restore %s/%s",
			existing.Namespace, existing.Name, r.Restore.Namespace, r.Restore.Name))
	}
	if !hasPostReadyFrozenContract(existing) {
		return nil, false, nil
	}
	if _, err := postReadyExecutionPolicyForJob(existing); err != nil {
		return nil, false, err
	}
	plan, err := postReadyTargetPlan(existing)
	if err != nil {
		return nil, false, err
	}
	if len(plan) == 0 {
		return nil, false, intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s has no frozen target plan", existing.Namespace, existing.Name))
	}
	targets := make(map[string]frozenPostReadyTarget, len(plan))
	for ordinal, identity := range plan {
		target, source := splitPostReadyTargetIdentity(identity)
		if target == "" {
			return nil, false, intctrlutil.NewFatalError("postReady frozen target plan has an empty target")
		}
		if _, ok := targets[target]; ok {
			return nil, false, intctrlutil.NewFatalError(fmt.Sprintf(
				"duplicate postReady frozen target %s", target))
		}
		targets[target] = frozenPostReadyTarget{source: source, ordinal: ordinal}
	}
	return targets, true, nil
}

func hasPostReadyFrozenContract(job *batchv1.Job) bool {
	if job.Annotations == nil {
		return false
	}
	_, hasPolicy := job.Annotations[postReadyExecutionPolicyAnnotationKey]
	_, hasIdentity := job.Annotations[postReadyTargetIdentityAnnotationKey]
	_, hasPlan := job.Annotations[postReadyTargetPlanAnnotationKey]
	return hasPolicy || hasIdentity || hasPlan
}

func setPostReadyTargetPlan(jobs []*batchv1.Job) error {
	plan := make([]string, 0, len(jobs))
	for i := range jobs {
		identity := postReadyTargetIdentity(jobs[i])
		if identity == "" {
			return intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady job %s/%s has empty target identity", jobs[i].Namespace, jobs[i].Name))
		}
		plan = append(plan, identity)
	}
	serialized, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	for i := range jobs {
		jobs[i].Annotations[postReadyTargetPlanAnnotationKey] = string(serialized)
	}
	return nil
}

func postReadyTargetPlan(job *batchv1.Job) ([]string, error) {
	if job.Annotations == nil || job.Annotations[postReadyTargetPlanAnnotationKey] == "" {
		return nil, nil
	}
	var plan []string
	if err := json.Unmarshal([]byte(job.Annotations[postReadyTargetPlanAnnotationKey]), &plan); err != nil {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s has invalid frozen target plan: %v", job.Namespace, job.Name, err))
	}
	if len(plan) == 0 {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s has empty frozen target plan", job.Namespace, job.Name))
	}
	return plan, nil
}

func postReadyExecutionPolicyForJob(job *batchv1.Job) (dpv1alpha1.PostReadyExecutionPolicy, error) {
	if !hasPostReadyFrozenContract(job) {
		return dpv1alpha1.PostReadyExecutionPolicyParallel, nil
	}
	policyValue, hasPolicy := job.Annotations[postReadyExecutionPolicyAnnotationKey]
	if !hasPolicy || policyValue == "" {
		return "", intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s has a frozen target contract without an execution policy",
			job.Namespace, job.Name))
	}
	identity, hasIdentity := job.Annotations[postReadyTargetIdentityAnnotationKey]
	if !hasIdentity || identity == "" {
		return "", intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s has a frozen target contract without a target identity",
			job.Namespace, job.Name))
	}
	plan, hasPlan := job.Annotations[postReadyTargetPlanAnnotationKey]
	if !hasPlan || plan == "" {
		return "", intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s has a frozen target contract without a target plan",
			job.Namespace, job.Name))
	}
	policy := dpv1alpha1.PostReadyExecutionPolicy(policyValue)
	if policy != dpv1alpha1.PostReadyExecutionPolicyParallel && policy != dpv1alpha1.PostReadyExecutionPolicySerial {
		return "", intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s has invalid frozen execution policy %q",
			job.Namespace, job.Name, policy))
	}
	return policy, nil
}

func serialPostReadyJobs(jobs []*batchv1.Job) (bool, error) {
	if len(jobs) == 0 {
		return false, nil
	}
	policy, err := postReadyExecutionPolicyForJob(jobs[0])
	if err != nil {
		return false, err
	}
	for i := 1; i < len(jobs); i++ {
		jobPolicy, err := postReadyExecutionPolicyForJob(jobs[i])
		if err != nil {
			return false, err
		}
		if jobPolicy != policy {
			return false, intctrlutil.NewFatalError("postReady jobs have inconsistent frozen execution policies")
		}
	}
	return policy == dpv1alpha1.PostReadyExecutionPolicySerial, nil
}

// PostReadyActionExpectedJobCount returns the immutable plan cardinality when
// available. This count belongs to the plan domain and does not change as
// mutable completion status advances.
func PostReadyActionExpectedJobCount(jobs []*batchv1.Job) (int, error) {
	if len(jobs) == 0 {
		return 0, nil
	}
	plan, err := postReadyTargetPlan(jobs[0])
	if err != nil {
		return 0, err
	}
	if len(plan) > 0 {
		return len(plan), nil
	}
	return len(jobs), nil
}

// EnsurePostReadyStagePlan commits the ordered executable plan for every
// postReady action before the first Job of the stage can be created.
func (r *RestoreManager) EnsurePostReadyStagePlan(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
) (bool, error) {
	if _, hasMarker := r.postReadyPlanMarker(); hasMarker {
		if _, found, err := r.loadPostReadyExecutionPlanStage(reqCtx, cli); err != nil || found {
			return found, err
		}
	}
	if r.Restore.Spec.ReadyConfig == nil || len(r.PostReadyBackupSets) == 0 {
		return false, nil
	}
	actions := make([]postReadyActionExecutionPlan, 0)
	for i := range r.PostReadyBackupSets {
		backupSet := r.PostReadyBackupSets[i]
		if backupSet.Backup == nil || backupSet.ActionSet == nil || backupSet.ActionSet.Spec.Restore == nil {
			return false, intctrlutil.NewFatalError("postReady stage has an incomplete Backup/ActionSet definition")
		}
		target := utils.GetBackupStatusTarget(backupSet.Backup, r.Restore.Spec.Backup.SourceTargetName)
		if target == nil {
			return false, intctrlutil.NewFatalError("can not found any source targe in backup " + backupSet.Backup.Name)
		}
		for step := range backupSet.ActionSet.Spec.Restore.PostReady {
			actionName := fmt.Sprintf("%s-%d", dpv1alpha1.PostReady, step)
			jobs, err := r.BuildPostReadyActionJobs(reqCtx, cli, backupSet, target, step)
			if err != nil {
				return false, err
			}
			jobs, err = r.freezeLegacyPostReadyExecutionPlan(reqCtx, cli, jobs)
			if err != nil {
				return false, err
			}
			canonical, err := canonicalPostReadyPlanJobs(jobs)
			if err != nil {
				return false, err
			}
			if len(canonical) == 0 {
				return false, intctrlutil.NewFatalError(fmt.Sprintf(
					"postReady stage action %s/%s has no executable Jobs", backupSet.Backup.Name, actionName))
			}
			actions = append(actions, postReadyActionExecutionPlan{
				Order: len(actions), BackupName: backupSet.Backup.Name, ActionName: actionName, Jobs: canonical,
			})
		}
	}
	if len(actions) == 0 {
		return false, nil
	}
	_, err := r.persistPostReadyExecutionPlanStage(reqCtx, cli, actions)
	return err == nil, err
}

// PendingPostReadyJobs filters Jobs already recorded terminal in mutable
// Restore status. A garbage-collected Completed or Failed Job is therefore
// never recreated from the immutable plan.
func (r *RestoreManager) PendingPostReadyJobs(backupName, actionName string, jobs []*batchv1.Job) []*batchv1.Job {
	terminal := map[string]struct{}{}
	for i := range r.Restore.Status.Actions.PostReady {
		action := r.Restore.Status.Actions.PostReady[i]
		if action.BackupName == backupName && action.Name == actionName &&
			(action.Status == dpv1alpha1.RestoreActionCompleted || action.Status == dpv1alpha1.RestoreActionFailed) {
			terminal[action.ObjectKey] = struct{}{}
		}
	}
	pending := make([]*batchv1.Job, 0, len(jobs))
	for i := range jobs {
		if _, ok := terminal[BuildJobKeyForActionStatus(jobs[i].Name)]; ok {
			continue
		}
		pending = append(pending, jobs[i])
	}
	return pending
}

// FreezePostReadyExecutionPlan persists the complete immutable executable plan
// before any Job is created. Once present, that plan remains authoritative
// across Job GC and ActionSet or ReadyConfig drift.
func (r *RestoreManager) FreezePostReadyExecutionPlan(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	jobs []*batchv1.Job,
) ([]*batchv1.Job, error) {
	backupName, actionName, hasIdentity, err := postReadyPlanIdentityForJobs(jobs)
	if err != nil {
		return nil, err
	}
	if hasIdentity {
		if _, hasMarker := r.postReadyPlanMarker(); hasMarker {
			persisted, found, err := r.loadPostReadyExecutionPlan(reqCtx, cli, backupName, actionName)
			if err != nil {
				return nil, err
			}
			if found {
				return persisted, nil
			}
		}
	}

	frozen, err := r.freezeLegacyPostReadyExecutionPlan(reqCtx, cli, jobs)
	if err != nil || !hasIdentity {
		return frozen, err
	}
	frozenBackupName, frozenActionName, frozenHasIdentity, err := postReadyPlanIdentityForJobs(frozen)
	if err != nil {
		return nil, err
	}
	if !frozenHasIdentity || frozenBackupName != backupName || frozenActionName != actionName {
		return frozen, nil
	}
	return r.persistPostReadyExecutionPlan(reqCtx, cli, backupName, actionName, frozen)
}

// freezeLegacyPostReadyExecutionPlan preserves compatibility with Jobs created
// before durable plan Secrets existed. It may migrate only a complete or
// provably equivalent legacy plan; ambiguous legacy state still fails closed.
func (r *RestoreManager) freezeLegacyPostReadyExecutionPlan(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	jobs []*batchv1.Job,
) ([]*batchv1.Job, error) {
	if len(jobs) == 0 {
		return jobs, nil
	}
	policy, err := postReadyExecutionPolicyForJob(jobs[0])
	if err != nil {
		return nil, err
	}
	allInputJobsPersisted := true
	for i := range jobs {
		if jobs[i].ResourceVersion == "" {
			allInputJobsPersisted = false
			break
		}
	}
	desiredContract := postReadyActionContractForJob(jobs[0])
	if desiredContract == "" {
		if allInputJobsPersisted {
			// All executable Job specs already exist, so they remain the action
			// fact source for upgrades from the previous frozen-plan format.
			return jobs, nil
		}
		return nil, intctrlutil.NewFatalError("postReady jobs have no frozen action contract")
	}
	for i := 1; i < len(jobs); i++ {
		if postReadyActionContractForJob(jobs[i]) != desiredContract {
			return nil, intctrlutil.NewFatalError("postReady jobs have inconsistent frozen action contracts")
		}
	}
	foundExisting := false
	foundLegacyPartial := false
	var frozenPlan []string
	for i := range jobs {
		existing := &batchv1.Job{}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(jobs[i]), existing); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		if !r.isJobForRestoreAction(existing) {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(
				"restore job name collision: existing job %s/%s does not belong to restore %s/%s",
				existing.Namespace, existing.Name, r.Restore.Namespace, r.Restore.Name))
		}
		if !hasPostReadyFrozenContract(existing) {
			if allInputJobsPersisted {
				for j := range jobs {
					if hasPostReadyFrozenContract(jobs[j]) {
						return nil, intctrlutil.NewFatalError("legacy and frozen postReady jobs cannot be mixed")
					}
				}
				return jobs, nil
			}
			if foundExisting {
				return nil, intctrlutil.NewFatalError("legacy and frozen postReady jobs cannot be mixed")
			}
			if policy != dpv1alpha1.PostReadyExecutionPolicyParallel {
				return nil, intctrlutil.NewFatalError(fmt.Sprintf(
					"legacy postReady job %s/%s cannot be migrated to serial execution",
					existing.Namespace, existing.Name))
			}
			if !apiequality.Semantic.DeepDerivative(jobs[i].Spec, existing.Spec) {
				return nil, intctrlutil.NewFatalError(fmt.Sprintf(
					"legacy postReady job %s/%s executable action does not match the current ActionSet",
					existing.Namespace, existing.Name))
			}
			foundLegacyPartial = true
			continue
		}
		if foundLegacyPartial {
			return nil, intctrlutil.NewFatalError("legacy and frozen postReady jobs cannot be mixed")
		}
		existingPolicy, err := postReadyExecutionPolicyForJob(existing)
		if err != nil {
			return nil, err
		}
		if foundExisting && existingPolicy != policy {
			return nil, intctrlutil.NewFatalError("postReady jobs have inconsistent frozen execution policies")
		}
		policy = existingPolicy
		plan, err := postReadyTargetPlan(existing)
		if err != nil {
			return nil, err
		}
		if len(plan) == 0 {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady job %s/%s has no frozen target plan", existing.Namespace, existing.Name))
		}
		if len(frozenPlan) > 0 && strings.Join(plan, "\x00") != strings.Join(frozenPlan, "\x00") {
			return nil, intctrlutil.NewFatalError("postReady jobs have inconsistent frozen target plans")
		}
		existingContract := postReadyActionContractForJob(existing)
		if existingContract == "" {
			if !allInputJobsPersisted {
				return nil, intctrlutil.NewFatalError(fmt.Sprintf(
					"postReady job %s/%s has no complete frozen action contract",
					existing.Namespace, existing.Name))
			}
		} else if existingContract != desiredContract {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady job %s/%s executable action does not match its frozen contract",
				existing.Namespace, existing.Name))
		}
		index := postReadyJobIndex(existing.Name)
		if index < 0 || index >= len(plan) || postReadyTargetIdentity(existing) != plan[index] {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady job %s/%s target identity does not match its frozen plan",
				existing.Namespace, existing.Name))
		}
		frozenPlan = plan
		foundExisting = true
	}
	if foundLegacyPartial {
		for i := range jobs {
			delete(jobs[i].Annotations, postReadyExecutionPolicyAnnotationKey)
			delete(jobs[i].Annotations, postReadyTargetIdentityAnnotationKey)
			delete(jobs[i].Annotations, postReadyTargetPlanAnnotationKey)
			delete(jobs[i].Annotations, postReadyActionContractAnnotationKey)
			jobs[i].Spec.Suspend = nil
		}
		return jobs, nil
	}
	if !foundExisting {
		return jobs, nil
	}

	jobsByTarget := make(map[string]*batchv1.Job, len(jobs))
	for i := range jobs {
		identity := postReadyTargetIdentity(jobs[i])
		if identity == "" {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady job %s/%s has empty target identity", jobs[i].Namespace, jobs[i].Name))
		}
		if _, ok := jobsByTarget[identity]; ok {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf("duplicate postReady target identity %s", identity))
		}
		jobsByTarget[identity] = jobs[i]
	}
	frozenJobs := make([]*batchv1.Job, 0, len(frozenPlan))
	for i, identity := range frozenPlan {
		job, ok := jobsByTarget[identity]
		if !ok {
			return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue,
				"postReady frozen target %s is not currently available", identity)
		}
		job = job.DeepCopy()
		job.Name = postReadyJobNameForIndex(jobs[0].Name, i)
		setPostReadyExecutionPolicy(job, policy)
		job.Annotations[postReadyActionContractAnnotationKey] = desiredContract
		serializedPlan, err := json.Marshal(frozenPlan)
		if err != nil {
			return nil, err
		}
		job.Annotations[postReadyTargetPlanAnnotationKey] = string(serializedPlan)
		frozenJobs = append(frozenJobs, job)
	}
	return frozenJobs, nil
}

func (r *RestoreManager) validateExistingRestoreActionJob(desired, existing *batchv1.Job) error {
	if !r.isJobForRestoreAction(existing) {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"restore job name collision: existing job %s/%s does not belong to restore %s/%s",
			existing.Namespace, existing.Name, r.Restore.Namespace, r.Restore.Name))
	}
	desiredIdentity := postReadyTargetIdentity(desired)
	if desiredIdentity == "" {
		return nil
	}
	if postReadyTargetIdentity(existing) != desiredIdentity {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s target identity does not match desired target",
			existing.Namespace, existing.Name))
	}
	desiredHasPlanReference := desired.Annotations[postReadyPlanNameAnnotationKey] != "" ||
		desired.Annotations[postReadyPlanDigestAnnotationKey] != "" ||
		desired.Annotations[postReadyPlanRestoreUIDAnnotationKey] != ""
	if desiredHasPlanReference {
		desiredBackupName, hasDesiredBackupName := desired.Annotations[postReadyBackupNameAnnotationKey]
		desiredActionName, hasDesiredActionName := desired.Annotations[postReadyActionNameAnnotationKey]
		if !hasDesiredBackupName || desiredBackupName == "" || !hasDesiredActionName || desiredActionName == "" {
			return intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady job %s/%s desired committed action identity is incomplete",
				desired.Namespace, desired.Name))
		}
		existingBackupName, hasExistingBackupName := existing.Annotations[postReadyBackupNameAnnotationKey]
		existingActionName, hasExistingActionName := existing.Annotations[postReadyActionNameAnnotationKey]
		if !hasExistingBackupName || existingBackupName == "" ||
			!hasExistingActionName || existingActionName == "" ||
			existingBackupName != desiredBackupName || existingActionName != desiredActionName {
			return intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady job %s/%s committed action identity does not match desired action",
				existing.Namespace, existing.Name))
		}
	} else {
		desiredBackupName, desiredActionName := postReadyActionIdentity(desired)
		existingBackupName, existingActionName := postReadyActionIdentity(existing)
		if (desiredBackupName != "" || desiredActionName != "") &&
			(existingBackupName != desiredBackupName || existingActionName != desiredActionName) {
			return intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady job %s/%s action identity does not match desired action",
				existing.Namespace, existing.Name))
		}
	}
	desiredPolicy, err := postReadyExecutionPolicyForJob(desired)
	if err != nil {
		return err
	}
	existingPolicy, err := postReadyExecutionPolicyForJob(existing)
	if err != nil {
		return err
	}
	if existingPolicy != desiredPolicy {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s execution policy does not match desired policy",
			existing.Namespace, existing.Name))
	}
	desiredPlan, err := postReadyTargetPlan(desired)
	if err != nil {
		return err
	}
	existingPlan, err := postReadyTargetPlan(existing)
	if err != nil {
		return err
	}
	if len(desiredPlan) == 0 || strings.Join(existingPlan, "\x00") != strings.Join(desiredPlan, "\x00") {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s frozen target plan does not match desired plan",
			existing.Namespace, existing.Name))
	}
	desiredContract := postReadyActionContractForJob(desired)
	if desiredContract == "" || postReadyActionContractForJob(existing) != desiredContract {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s executable action does not match desired contract",
			existing.Namespace, existing.Name))
	}
	desiredPlanName := desired.Annotations[postReadyPlanNameAnnotationKey]
	desiredPlanDigest := desired.Annotations[postReadyPlanDigestAnnotationKey]
	desiredRestoreUID := desired.Annotations[postReadyPlanRestoreUIDAnnotationKey]
	if desiredPlanName == "" && desiredPlanDigest == "" && desiredRestoreUID == "" {
		if existing.Annotations[postReadyPlanNameAnnotationKey] != "" ||
			existing.Annotations[postReadyPlanDigestAnnotationKey] != "" ||
			existing.Annotations[postReadyPlanRestoreUIDAnnotationKey] != "" {
			return intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady job %s/%s has an unexpected committed stage reference",
				existing.Namespace, existing.Name))
		}
		if !r.postReadyJobSpecsEqual(desired, existing) {
			return intctrlutil.NewFatalError(fmt.Sprintf(
				"postReady job %s/%s executable spec does not match its frozen legacy plan",
				existing.Namespace, existing.Name))
		}
		return nil
	}
	if desiredPlanName == "" || desiredPlanDigest == "" || desiredRestoreUID == "" {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s desired execution plan reference is incomplete",
			desired.Namespace, desired.Name))
	}
	if existingPlanName := existing.Annotations[postReadyPlanNameAnnotationKey]; existingPlanName != desiredPlanName {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s has a missing or different immutable execution plan reference",
			existing.Namespace, existing.Name))
	}
	if existingPlanDigest := existing.Annotations[postReadyPlanDigestAnnotationKey]; existingPlanDigest != desiredPlanDigest {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s has a missing or different execution plan digest",
			existing.Namespace, existing.Name))
	}
	if existingRestoreUID := existing.Annotations[postReadyPlanRestoreUIDAnnotationKey]; existingRestoreUID != desiredRestoreUID || existingRestoreUID != string(r.Restore.UID) {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s has a missing or different restore UID reference",
			existing.Namespace, existing.Name))
	}
	if !controllerutil.ContainsFinalizer(existing, dptypes.DataProtectionFinalizerName) {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s is missing terminal-status protection",
			existing.Namespace, existing.Name))
	}
	if !r.postReadyJobSpecsEqual(desired, existing) {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady job %s/%s executable spec does not match its immutable execution plan",
			existing.Namespace, existing.Name))
	}
	return nil
}

func postReadyJobNameForIndex(name string, index int) string {
	separator := strings.LastIndexByte(name, '-')
	if separator < 0 {
		return name
	}
	return fmt.Sprintf("%s-%d", name[:separator], index)
}

// ResumeNextSerialPostReadyJob starts at most one suspended postReady job after
// every preceding job has completed successfully.
func (r *RestoreManager) ResumeNextSerialPostReadyJob(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	jobs []*batchv1.Job,
) error {
	serial, err := serialPostReadyJobs(jobs)
	if err != nil {
		return err
	}
	if !serial {
		return nil
	}
	sort.Slice(jobs, func(i, j int) bool {
		return postReadyJobIndex(jobs[i].Name) < postReadyJobIndex(jobs[j].Name)
	})
	for i := range jobs {
		done, _, errMsg := utils.IsJobFinished(jobs[i])
		if errMsg != "" {
			return nil
		}
		if done {
			continue
		}
		if jobs[i].Spec.Suspend != nil && *jobs[i].Spec.Suspend {
			updated := jobs[i].DeepCopy()
			updated.Spec.Suspend = boolptr.False()
			if err := cli.Patch(reqCtx.Ctx, updated, client.MergeFrom(jobs[i])); err != nil {
				return err
			}
			jobs[i] = updated
		}
		return nil
	}
	return nil
}

func postReadyJobIndex(name string) int {
	separator := strings.LastIndexByte(name, '-')
	if separator < 0 {
		return int(^uint(0) >> 1)
	}
	index, err := strconv.Atoi(name[separator+1:])
	if err != nil {
		return int(^uint(0) >> 1)
	}
	return index
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
			if err = cli.Create(reqCtx.Ctx, objs[i]); err != nil {
				if !apierrors.IsAlreadyExists(err) {
					return nil, err
				}
				fetchedJob = &batchv1.Job{}
				if err = cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(objs[i]), fetchedJob); err != nil {
					return nil, err
				}
				if err = r.validateExistingRestoreActionJob(objs[i], fetchedJob); err != nil {
					return nil, err
				}
				fetchedJobs = append(fetchedJobs, fetchedJob)
				continue
			}
			msg := fmt.Sprintf("created job %s/%s", objs[i].Namespace, objs[i].Name)
			r.Recorder.Event(r.Restore, corev1.EventTypeNormal, reasonCreateRestoreJob, msg)
			fetchedJobs = append(fetchedJobs, objs[i])
		} else {
			if err = r.validateExistingRestoreActionJob(objs[i], fetchedJob); err != nil {
				return nil, err
			}
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
	fetchedJobs []*batchv1.Job) (bool, bool, error) {
	var (
		allJobFinished = true
		existFailedJob bool
	)
	restoreActions := &r.Restore.Status.Actions.PrepareData
	if stage == dpv1alpha1.PostReady {
		restoreActions = &r.Restore.Status.Actions.PostReady
	}
	serialPostReady := false
	if stage == dpv1alpha1.PostReady {
		var err error
		serialPostReady, err = serialPostReadyJobs(fetchedJobs)
		if err != nil {
			return false, false, err
		}
	}
	// count the number of jobs that are completed, failed,
	// or have the normally terminated `restore` container
	finishedCount := 0
	for i := range fetchedJobs {
		statusAction := dpv1alpha1.RestoreStatusAction{
			Name:       actionName,
			ObjectKey:  BuildJobKeyForActionStatus(fetchedJobs[i].Name),
			BackupName: backupSet.Backup.Name,
		}
		done, _, errMsg := utils.IsJobFinished(fetchedJobs[i])
		switch {
		case errMsg != "":
			existFailedJob = true
			statusAction.Status = dpv1alpha1.RestoreActionFailed
			statusAction.Message = errMsg
			SetRestoreStatusAction(restoreActions, statusAction)
			finishedCount++
		case done:
			statusAction.Status = dpv1alpha1.RestoreActionCompleted
			SetRestoreStatusAction(restoreActions, statusAction)
			finishedCount++
		default:
			allJobFinished = false
			statusAction.Status = dpv1alpha1.RestoreActionProcessing
			SetRestoreStatusAction(restoreActions, statusAction)
			normalTerminated, err := r.CheckIfRestoreContainerTerminated(fetchedJobs[i])
			if err != nil {
				return false, false, err
			}
			if normalTerminated {
				finishedCount++
				if serialPostReady {
					if err := r.StopManagerContainerByJob(fetchedJobs[i]); err != nil {
						return false, false, err
					}
				}
			}
		}
	}
	if serialPostReady && existFailedJob {
		return true, true, nil
	}
	// wait until all `restore` containers are terminated normally or jobs are completed or failed
	if finishedCount == len(fetchedJobs) {
		for i := range fetchedJobs {
			err := r.StopManagerContainerByJob(fetchedJobs[i])
			if err != nil {
				return false, false, err
			}
		}
	}
	return allJobFinished, existFailedJob, nil
}

// CheckIfRestoreContainerTerminated checks if the `restore` container is terminated.
// If the `restore` container is terminated abnormally, stop the `restore manager` container.
func (r *RestoreManager) CheckIfRestoreContainerTerminated(job *batchv1.Job) (normalTerminated bool, err error) {
	podList, err := utils.GetAssociatedPodsOfJob(context.Background(), r.Client, job.Namespace, job.Name)
	if err != nil {
		return false, err
	}
	if len(podList.Items) == 0 {
		return false, nil
	}
	for i, pod := range podList.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.Name != Restore {
				continue
			}
			terminatedState := containerStatus.State.Terminated
			if terminatedState != nil {
				if terminatedState.ExitCode != 0 {
					// stop `restore manager` container if the `restore` container is terminated abnormally
					if err := r.StopManagerContainer(&podList.Items[i]); err != nil {
						return false, err
					}
				} else {
					normalTerminated = true
				}
			}
		}
	}
	return normalTerminated, nil
}

// StopManagerContainerByJob stops the `restore manager` containers by the job.
func (r *RestoreManager) StopManagerContainerByJob(job *batchv1.Job) error {
	podList, err := utils.GetAssociatedPodsOfJob(context.Background(), r.Client, job.Namespace, job.Name)
	if err != nil {
		return err
	}
	for i := range podList.Items {
		err := r.StopManagerContainer(&podList.Items[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// StopManagerContainer stops the `restore manager` container.
func (r *RestoreManager) StopManagerContainer(pod *corev1.Pod) error {
	modified := pod.DeepCopy()
	if modified.Annotations == nil {
		modified.Annotations = map[string]string{}
	}
	if val, ok := modified.Annotations[DataProtectionStopRestoreManagerAnnotationKey]; ok && val == "true" {
		return nil
	}
	// `restore manager` container will read this annotation to stop
	modified.Annotations[DataProtectionStopRestoreManagerAnnotationKey] = "true"
	if err := r.Client.Patch(context.Background(), modified, client.MergeFrom(pod)); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
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
