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
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	computil "github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// PointInTimeRecoveryManager  pitr manager functions
// 1. get the latest base backup
// 2. get the next earliest backup
// 3. add log pvc by datasource volume snapshot
// 4. create init container to prepare log
// 5. run recovery jobs
// 6. cleanup
type PointInTimeRecoveryManager struct {
	client.Client
	Ctx     context.Context
	Cluster *appsv1alpha1.Cluster

	// private
	namespace     string
	restoreTime   *metav1.Time
	sourceCluster string
}

const (
	initContainerName = "pitr-for-pause"
	backupVolumePATH  = "/backupdata"
)

// DoPITRPrepare prepares init container and pvc before point in time recovery
func DoPITRPrepare(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) error {
	if cluster.Status.ObservedGeneration >= 1 {
		return nil
	}

	// build pitr init container to wait for prepare data
	// prepare data if PITR needed
	pitrMgr := PointInTimeRecoveryManager{
		Cluster: cluster,
		Client:  cli,
		Ctx:     ctx,
	}
	return pitrMgr.doPrepare(component)
}

// DoPITRIfNeed if needs to run restore job and copy data for pitr
func DoPITRIfNeed(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster) (shouldRequeue bool, err error) {
	if cluster.Status.ObservedGeneration != 1 {
		return false, nil
	}
	pitrMgr := PointInTimeRecoveryManager{
		Cluster: cluster,
		Client:  cli,
		Ctx:     ctx,
	}
	return pitrMgr.doRecoveryJob()
}

// DoPITRCleanup cleanups the resources and annotations after recovery
func DoPITRCleanup(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster) error {
	if cluster.Status.ObservedGeneration < 1 {
		return nil
	}
	pitrMgr := PointInTimeRecoveryManager{
		Cluster: cluster,
		Client:  cli,
		Ctx:     ctx,
	}
	if need, err := pitrMgr.checkAndInit(); err != nil {
		return err
	} else if !need {
		return nil
	}
	// clean up job
	if err := pitrMgr.cleanupScriptsJob(); err != nil {
		return err
	}
	// clean cluster annotations
	if err := pitrMgr.cleanupClusterAnnotations(); err != nil {
		return err
	}
	return nil
}

// doRecoveryJob runs a physical recovery job before cluster service starts
func (p *PointInTimeRecoveryManager) doRecoveryJob() (shouldRequeue bool, err error) {
	if need, err := p.checkAndInit(); err != nil {
		return false, err
	} else if !need {
		return false, nil
	}

	// mount the data+log pvc, and run scripts job to prepare data
	if err = p.runRecoveryJob(); err != nil {
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting) {
			return true, nil
		}
		return false, err
	}

	// check job done
	if !p.ensureJobDone() {
		return true, nil
	}

	// remove init container
	for _, componentSpec := range p.Cluster.Spec.ComponentSpecs {
		if err = p.removeStsInitContainer(p.Cluster, componentSpec.Name); err != nil {
			return false, err
		}
	}

	return false, nil

}

// doPrepare prepares init container and pvc before recovery
func (p *PointInTimeRecoveryManager) doPrepare(component *component.SynthesizedComponent) error {
	if need, err := p.checkAndInit(); err != nil {
		return err
	} else if !need {
		return nil
	}

	// prepare data pvc
	if len(component.VolumeClaimTemplates) == 0 {
		return errors.New("not found data pvc")
	}
	latestBackup, err := p.getLatestBaseBackup()
	if err != nil {
		return err
	}

	// recovery time start time boundary processing, this scenario is converted to back up recovery function
	if latestBackup.Status.Manifests.BackupLog.StopTime.Format(time.RFC3339) == p.restoreTime.Format(time.RFC3339) {
		if latestBackup.Spec.BackupType == dpv1alpha1.BackupTypeSnapshot {
			delete(p.Cluster.Annotations, constant.RestoreFromSrcClusterAnnotationKey)
			delete(p.Cluster.Annotations, constant.RestoreFromTimeAnnotationKey)
			return p.doPrepareSnapshotBackup(component, latestBackup)
		}
		// TODO: support restore with full backup.
		return nil
	}
	// prepare init container
	container := corev1.Container{}
	container.Name = initContainerName
	container.Image = viper.GetString(constant.KBToolsImage)
	container.Command = []string{"sleep", "infinity"}
	component.PodSpec.InitContainers = append(component.PodSpec.InitContainers, container)

	if latestBackup.Spec.BackupType == dpv1alpha1.BackupTypeSnapshot {
		return p.doPrepareSnapshotBackup(component, latestBackup)
	}
	return nil
}

func (p *PointInTimeRecoveryManager) doPrepareSnapshotBackup(component *component.SynthesizedComponent, backup *dpv1alpha1.Backup) error {
	vct := component.VolumeClaimTemplates[0]
	snapshotAPIGroup := snapshotv1.GroupName
	vct.Spec.DataSource = &corev1.TypedLocalObjectReference{
		APIGroup: &snapshotAPIGroup,
		Kind:     constant.VolumeSnapshotKind,
		Name:     backup.Name,
	}
	component.VolumeClaimTemplates[0] = vct
	return nil
}

func (p *PointInTimeRecoveryManager) listCompletedBackups() (backupItems []dpv1alpha1.Backup, err error) {
	backups := dpv1alpha1.BackupList{}
	if err := p.Client.List(p.Ctx, &backups,
		client.InNamespace(p.namespace),
		client.MatchingLabels(map[string]string{
			constant.AppInstanceLabelKey: p.sourceCluster,
		}),
	); err != nil {
		return nil, err
	}

	backupItems = []dpv1alpha1.Backup{}
	for _, b := range backups.Items {
		if b.Status.Phase == dpv1alpha1.BackupCompleted && b.Status.Manifests != nil && b.Status.Manifests.BackupLog != nil {
			backupItems = append(backupItems, b)
		}
	}
	return backupItems, nil
}

// getSortedBackups sorts by StopTime
func (p *PointInTimeRecoveryManager) getSortedBackups(reverse bool) ([]dpv1alpha1.Backup, error) {
	backups, err := p.listCompletedBackups()
	if err != nil {
		return backups, err
	}
	sort.Slice(backups, func(i, j int) bool {
		if reverse {
			i, j = j, i
		}
		if backups[i].Status.Manifests.BackupLog.StopTime == nil && backups[j].Status.Manifests.BackupLog.StopTime != nil {
			return false
		}
		if backups[i].Status.Manifests.BackupLog.StopTime != nil && backups[j].Status.Manifests.BackupLog.StopTime == nil {
			return true
		}
		if backups[i].Status.Manifests.BackupLog.StopTime.Equal(backups[j].Status.Manifests.BackupLog.StopTime) {
			return backups[i].Name < backups[j].Name
		}
		return backups[i].Status.Manifests.BackupLog.StopTime.Before(backups[j].Status.Manifests.BackupLog.StopTime)
	})
	return backups, nil
}

// getLatestBaseBackup gets the latest baseBackup
func (p *PointInTimeRecoveryManager) getLatestBaseBackup() (*dpv1alpha1.Backup, error) {
	// 1. sort reverse backups
	backups, err := p.getSortedBackups(true)
	if err != nil {
		return nil, err
	}

	// 2. get the latest backup object
	var latestBackup *dpv1alpha1.Backup
	for _, item := range backups {
		if item.Spec.BackupType != dpv1alpha1.BackupTypeLogFile &&
			item.Status.Manifests.BackupLog.StopTime != nil && !p.restoreTime.Before(item.Status.Manifests.BackupLog.StopTime) {
			latestBackup = &item
			break
		}
	}
	if latestBackup == nil {
		return nil, errors.New("can not found latest base backup")
	}

	return latestBackup, nil
}

// checkAndInit checks if cluster need to be restored
func (p *PointInTimeRecoveryManager) checkAndInit() (need bool, err error) {
	// check args if pitr supported
	cluster := p.Cluster
	if cluster.Annotations[constant.RestoreFromTimeAnnotationKey] == "" {
		return false, nil
	}
	restoreTimeStr := cluster.Annotations[constant.RestoreFromTimeAnnotationKey]
	sourceCuster := cluster.Annotations[constant.RestoreFromSrcClusterAnnotationKey]
	if sourceCuster == "" {
		return false, errors.New("need specify a source cluster name to recovery")
	}
	restoreTime := &metav1.Time{}
	if err = restoreTime.UnmarshalQueryParameter(restoreTimeStr); err != nil {
		return false, err
	}
	vctCount := 0
	for _, item := range cluster.Spec.ComponentSpecs {
		vctCount += len(item.VolumeClaimTemplates)
	}
	if vctCount == 0 {
		return false, errors.New("not support pitr without any volume claim templates")
	}

	// init args
	p.restoreTime = restoreTime
	p.sourceCluster = sourceCuster
	p.namespace = cluster.Namespace
	return true, nil
}

func getVolumeMount(spec *dpv1alpha1.BackupToolSpec) string {
	dataVolumeMount := "/data"
	// TODO: hack it because the mount path is not explicitly specified in cluster definition
	for _, env := range spec.Env {
		if env.Name == "VOLUME_DATA_DIR" {
			dataVolumeMount = env.Value
			break
		}
	}
	return dataVolumeMount
}

func (p *PointInTimeRecoveryManager) getRecoveryInfo(componentName string) (*dpv1alpha1.BackupToolSpec, error) {
	// get scripts from backup template
	toolList := dpv1alpha1.BackupToolList{}
	// TODO: The reference PITR backup tool needs a stronger reference relationship, for now use label references
	if err := p.Client.List(p.Ctx, &toolList,
		client.MatchingLabels{
			constant.ClusterDefLabelKey:     p.Cluster.Spec.ClusterDefRef,
			constant.BackupToolTypeLabelKey: "pitr",
		}); err != nil {

		return nil, err
	}
	if len(toolList.Items) == 0 {
		return nil, errors.New("not support recovery because of non-existed pitr backupTool")
	}
	incrementalBackup, err := p.getIncrementalBackup(componentName)
	if err != nil {
		return nil, err
	}
	spec := &toolList.Items[0].Spec
	timeFormat := time.RFC3339
	envTimeEnvIdx := -1
	for i, env := range spec.Env {
		if env.Value == "$KB_RECOVERY_TIME" {
			envTimeEnvIdx = i
		} else if env.Name == "TIME_FORMAT" {
			timeFormat = env.Value
		}
	}
	if envTimeEnvIdx != -1 {
		spec.Env[envTimeEnvIdx].Value = p.restoreTime.Time.UTC().Format(timeFormat)
	}
	backupDIR := incrementalBackup.Name
	if incrementalBackup.Status.Manifests != nil && incrementalBackup.Status.Manifests.BackupTool != nil {
		backupDIR = incrementalBackup.Status.Manifests.BackupTool.FilePath
	}
	headEnv := []corev1.EnvVar{
		{Name: "BACKUP_DIR", Value: backupVolumePATH + "/" + backupDIR},
		{Name: "BACKUP_NAME", Value: incrementalBackup.Name}}
	spec.Env = append(headEnv, spec.Env...)
	return spec, nil
}

func (p *PointInTimeRecoveryManager) getIncrementalBackup(componentName string) (*dpv1alpha1.Backup, error) {
	incrementalBackupList := dpv1alpha1.BackupList{}
	if err := p.Client.List(p.Ctx, &incrementalBackupList,
		client.MatchingLabels{
			constant.AppInstanceLabelKey:    p.sourceCluster,
			constant.KBAppComponentLabelKey: componentName,
			constant.BackupTypeLabelKeyKey:  string(dpv1alpha1.BackupTypeLogFile),
		}); err != nil {
		return nil, err
	}
	if len(incrementalBackupList.Items) == 0 {
		return nil, errors.New("not found incremental backups")
	}
	return &incrementalBackupList.Items[0], nil
}

func (p *PointInTimeRecoveryManager) getIncrementalPVC(componentName string) (*corev1.PersistentVolumeClaim, error) {
	incrementalBackup, err := p.getIncrementalBackup(componentName)
	if err != nil {
		return nil, err
	}
	pvcKey := types.NamespacedName{
		Name:      incrementalBackup.Status.PersistentVolumeClaimName,
		Namespace: incrementalBackup.Namespace,
	}
	pvc := corev1.PersistentVolumeClaim{}
	if err := p.Client.Get(p.Ctx, pvcKey, &pvc); err != nil {
		return nil, err
	}
	return &pvc, nil
}

func (p *PointInTimeRecoveryManager) getDataPVCs(componentName string) ([]corev1.PersistentVolumeClaim, error) {
	podList := corev1.PodList{}
	podLabels := map[string]string{
		constant.AppInstanceLabelKey:    p.Cluster.Name,
		constant.KBAppComponentLabelKey: componentName,
	}
	if err := p.Client.List(p.Ctx, &podList,
		client.InNamespace(p.namespace),
		client.MatchingLabels(podLabels)); err != nil {
		return nil, err
	}
	dataPVCs := []corev1.PersistentVolumeClaim{}
	for _, targetPod := range podList.Items {
		if targetPod.Spec.NodeName == "" {
			return nil, intctrlutil.NewError(intctrlutil.ErrorTypeNeedWaiting, "waiting Pod scheduled")
		}
		for _, volume := range targetPod.Spec.Volumes {
			if volume.PersistentVolumeClaim == nil {
				continue
			}
			dataPVC := corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{Namespace: targetPod.Namespace, Name: volume.PersistentVolumeClaim.ClaimName}
			if err := p.Client.Get(p.Ctx, pvcKey, &dataPVC); err != nil {
				return nil, err
			}
			if dataPVC.Labels[constant.VolumeTypeLabelKey] != string(appsv1alpha1.VolumeTypeData) {
				continue
			}
			if dataPVC.Status.Phase != corev1.ClaimBound {
				return nil, intctrlutil.NewError(intctrlutil.ErrorTypeNeedWaiting, "waiting PVC Bound")
			}
			if dataPVC.Annotations == nil {
				dataPVC.Annotations = map[string]string{}
			}
			dataPVC.Annotations["pod-name"] = targetPod.Name
			dataPVC.Annotations["node-name"] = targetPod.Spec.NodeName
			dataPVCs = append(dataPVCs, dataPVC)
		}
	}
	return dataPVCs, nil
}

func (p *PointInTimeRecoveryManager) buildResourceObjs() (objs []client.Object, err error) {
	objs = make([]client.Object, 0)

	for _, componentSpec := range p.Cluster.Spec.ComponentSpecs {
		if len(componentSpec.VolumeClaimTemplates) == 0 {
			continue
		}

		commonLabels := map[string]string{
			constant.AppManagedByLabelKey:   constant.AppName,
			constant.AppInstanceLabelKey:    p.Cluster.Name,
			constant.KBAppComponentLabelKey: componentSpec.Name,
		}
		// get data dir pvc name
		dataPVCs, err := p.getDataPVCs(componentSpec.Name)
		if err != nil {
			return objs, err
		}
		if len(dataPVCs) == 0 {
			return objs, errors.New("not found data pvc")
		}
		recoveryInfo, err := p.getRecoveryInfo(componentSpec.Name)
		if err != nil {
			return objs, err
		}
		incrementalPVC, err := p.getIncrementalPVC(componentSpec.Name)
		if err != nil {
			return objs, err
		}
		dataVolumeMount := getVolumeMount(recoveryInfo)
		for _, dataPVC := range dataPVCs {
			volumes := []corev1.Volume{
				{Name: "data", VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: dataPVC.Name}}},
				{Name: "log", VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: incrementalPVC.Name}}},
			}
			volumeMounts := []corev1.VolumeMount{
				{Name: "data", MountPath: dataVolumeMount},
				{Name: "log", MountPath: backupVolumePATH},
			}

			// render the job cue template
			image := recoveryInfo.Image
			if image == "" {
				image = viper.GetString(constant.KBToolsImage)
			}
			jobName := fmt.Sprintf("pitr-phy-%s", dataPVC.Annotations["pod-name"])
			job, err := builder.BuildPITRJob(jobName, p.Cluster, image, []string{"sh", "-c"},
				recoveryInfo.Physical.RestoreCommands, volumes, volumeMounts, recoveryInfo.Env)
			job.SetLabels(commonLabels)
			job.Spec.Template.Spec.NodeName = dataPVC.Annotations["node-name"]
			if err != nil {
				return objs, err
			}
			// create logic restore job
			if p.Cluster.Status.Phase == appsv1alpha1.RunningClusterPhase && len(recoveryInfo.Logical.RestoreCommands) > 0 {
				logicJobName := fmt.Sprintf("pitr-logic-%s", dataPVC.Annotations["pod-name"])
				logicJob, err := builder.BuildPITRJob(logicJobName, p.Cluster, image, []string{"sh", "-c"},
					recoveryInfo.Logical.RestoreCommands, volumes, volumeMounts, recoveryInfo.Env)
				logicJob.Spec.Template.Spec.NodeName = dataPVC.Annotations["node-name"]
				if err != nil {
					return objs, err
				}
				logicJob.SetLabels(commonLabels)
				objs = append(objs, logicJob)
			}
			// collect pvcs and jobs for later deletion
			objs = append(objs, job)
		}
	}
	return objs, nil
}

func (p *PointInTimeRecoveryManager) runRecoveryJob() error {
	objs, err := p.buildResourceObjs()
	if err != nil {
		return err
	}

	for _, obj := range objs {
		err = p.Client.Create(p.Ctx, obj)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (p *PointInTimeRecoveryManager) checkJobDone(key client.ObjectKey) (bool, error) {
	result := &batchv1.Job{}
	if err := p.Client.Get(p.Ctx, key, result); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		// if err is NOT "not found", that means unknown error.
		return false, err
	}
	if result.Status.Conditions != nil && len(result.Status.Conditions) > 0 {
		jobStatusCondition := result.Status.Conditions[0]
		if jobStatusCondition.Type == batchv1.JobComplete {
			return true, nil
		} else if jobStatusCondition.Type == batchv1.JobFailed {
			return true, errors.New(jobStatusCondition.Reason)
		}
	}
	// if found, return true
	return false, nil
}

func (p *PointInTimeRecoveryManager) ensureJobDone() bool {
	var jobObj *batchv1.Job
	var ok bool
	objs, err := p.buildResourceObjs()
	if err != nil {
		return false
	}
	for _, obj := range objs {
		if jobObj, ok = obj.(*batchv1.Job); !ok {
			continue
		}
		if done, err := p.checkJobDone(client.ObjectKeyFromObject(jobObj)); err != nil {
			return false
		} else if !done {
			return false
		}
	}
	return true
}

func (p *PointInTimeRecoveryManager) cleanupScriptsJob() error {
	objs, err := p.buildResourceObjs()
	if err != nil {
		return err
	}
	if p.Cluster.Status.Phase == appsv1alpha1.RunningClusterPhase {
		for _, obj := range objs {
			if err := intctrlutil.BackgroundDeleteObject(p.Client, p.Ctx, obj); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *PointInTimeRecoveryManager) cleanupClusterAnnotations() error {
	if p.Cluster.Status.Phase == appsv1alpha1.RunningClusterPhase && p.Cluster.Annotations != nil {
		cluster := p.Cluster
		patch := client.MergeFrom(cluster.DeepCopy())
		delete(cluster.Annotations, constant.RestoreFromSrcClusterAnnotationKey)
		delete(cluster.Annotations, constant.RestoreFromTimeAnnotationKey)
		return p.Client.Patch(p.Ctx, cluster, patch)
	}
	return nil
}

// removeStsInitContainerForRestore removes the statefulSet's init container after recovery job completed.
func (p *PointInTimeRecoveryManager) removeStsInitContainer(
	cluster *appsv1alpha1.Cluster,
	componentName string) error {
	// get the sts list of component
	stsList := &appsv1.StatefulSetList{}
	if err := computil.GetObjectListByComponentName(p.Ctx, p.Client, *cluster, stsList, componentName); err != nil {
		return err
	}
	for _, sts := range stsList.Items {
		initContainers := sts.Spec.Template.Spec.InitContainers
		updateInitContainers := make([]corev1.Container, 0)
		for _, c := range initContainers {
			if c.Name != initContainerName {
				updateInitContainers = append(updateInitContainers, c)
			}
		}
		sts.Spec.Template.Spec.InitContainers = updateInitContainers
		if err := p.Client.Update(p.Ctx, &sts); err != nil {
			return err
		}
	}
	return nil
}
