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
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// RestoreManager restores manager functions
// 1. support datafile/snapshot restore
// 2. support point in time recovery (PITR)
type RestoreManager struct {
	client.Client
	Ctx     context.Context
	Cluster *appsv1alpha1.Cluster
	Scheme  *k8sruntime.Scheme

	// private
	namespace     string
	restoreTime   *metav1.Time
	sourceCluster string
}

func NewRestoreManager(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, scheme *k8sruntime.Scheme) *RestoreManager {
	return &RestoreManager{
		Cluster: cluster,
		Client:  cli,
		Ctx:     ctx,
		Scheme:  scheme,
	}
}

const (
	backupVolumePATH = "/backupdata"
)

// DoRestore prepares restore jobs
func DoRestore(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent, schema *k8sruntime.Scheme) error {
	if cluster.Status.ObservedGeneration > 1 {
		return nil
	}

	mgr := NewRestoreManager(ctx, cli, cluster, schema)

	// check restore from backup
	backupObj, err := mgr.getBackupObjectFromAnnotation(component)
	if err != nil {
		return err
	}
	if backupObj == nil {
		return nil
	}

	if err = mgr.createDataPVCs(component, backupObj); err != nil {
		return err
	}
	jobs := make([]client.Object, 0)
	if backupObj.Spec.BackupType == dpv1alpha1.BackupTypeDataFile {
		dataFileJobs, err := mgr.BuildDatafileRestoreJob(component, backupObj)
		if err != nil {
			return err
		}

		logicJobs, err := mgr.buildLogicRestoreJob(component, backupObj)
		if err != nil {
			return err
		}
		jobs = append(jobs, dataFileJobs...)
		jobs = append(jobs, logicJobs...)
	}

	// create and waiting job finished
	if err = mgr.createJobsAndWaiting(jobs); err != nil {
		return err
	}

	// do clean up
	if err = mgr.cleanupClusterAnnotations(); err != nil {
		return err
	}
	if err = mgr.cleanupJobs(jobs); err != nil {
		return err
	}
	return nil
}

// DoPITR prepares PITR jobs
func DoPITR(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent, schema *k8sruntime.Scheme) error {
	if cluster.Status.ObservedGeneration > 1 {
		return nil
	}
	pitrMgr := NewRestoreManager(ctx, cli, cluster, schema)
	if need, err := pitrMgr.checkPITRAndInit(component.Name); err != nil {
		return err
	} else if !need {
		return nil
	}

	// get the latest base backup from point in time
	baseBackup, err := pitrMgr.getLatestBaseBackup(component.Name)
	if err != nil {
		return err
	}

	if err = pitrMgr.createDataPVCs(component, baseBackup); err != nil {
		return err
	}

	jobs := make([]client.Object, 0)
	if baseBackup.Spec.BackupType == dpv1alpha1.BackupTypeDataFile {
		dataFilejobs, err := pitrMgr.BuildDatafileRestoreJob(component, baseBackup)
		if err != nil {
			return err
		}
		// do create datafile restore job and check completed
		if err = pitrMgr.createJobsAndWaiting(dataFilejobs); err != nil {
			return err
		}
		jobs = append(jobs, dataFilejobs...)
	}
	sourceClusterUID := baseBackup.Labels[constant.DataProtectionLabelClusterUIDKey]
	logfileBackup, err := pitrMgr.getLogfileBackup(component.Name, sourceClusterUID)
	if err != nil {
		return err
	}

	recoveryInfo, err := pitrMgr.getRecoveryInfo(baseBackup, logfileBackup)
	if err != nil {
		return err
	}
	pitrJobs := make([]client.Object, 0)
	if len(recoveryInfo.Physical.RestoreCommands) != 0 {
		pitrJobs, err = pitrMgr.buildPITRPhysicalRestoreJob(component, recoveryInfo, logfileBackup)
		if err != nil {
			return err
		}
	}

	if len(recoveryInfo.Logical.RestoreCommands) != 0 {
		logicJobs, err := pitrMgr.buildLogicRestoreJob(component, logfileBackup, recoveryInfo.Env...)
		if err != nil {
			return err
		}
		pitrJobs = append(pitrJobs, logicJobs...)
	}

	// do create PITR job and check completed
	if err = pitrMgr.createJobsAndWaiting(pitrJobs); err != nil {
		return err
	}

	// do clean up
	if err = pitrMgr.cleanupClusterAnnotations(); err != nil {
		return err
	}
	jobs = append(jobs, pitrJobs...)
	if err = pitrMgr.cleanupJobs(jobs); err != nil {
		return err
	}
	return nil
}

func (p *RestoreManager) listCompletedBackups(componentName string) (backupItems []dpv1alpha1.Backup, err error) {
	backups := dpv1alpha1.BackupList{}
	if err := p.Client.List(p.Ctx, &backups,
		client.InNamespace(p.namespace),
		client.MatchingLabels(map[string]string{
			constant.AppInstanceLabelKey:    p.sourceCluster,
			constant.KBAppComponentLabelKey: componentName,
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

// sortBackups sorts by StopTime
func (p *RestoreManager) sortBackups(backups []dpv1alpha1.Backup, reverse bool) []dpv1alpha1.Backup {
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
	return backups
}

// getLatestBaseBackup gets the latest baseBackup
func (p *RestoreManager) getLatestBaseBackup(componentName string) (*dpv1alpha1.Backup, error) {
	// 1. sorts reverse backups
	backups, err := p.listCompletedBackups(componentName)
	if err != nil {
		return nil, err
	}
	backups = p.sortBackups(backups, true)

	// 2. gets the latest backup object
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

// checkPITRAndInit checks if cluster need to be restored
func (p *RestoreManager) checkPITRAndInit(compName string) (bool, error) {
	// checks args if pitr supported
	cluster := p.Cluster
	restoreTimeStr, err := p.getComponentBackupInfoFromAnnotation(compName, constant.RestoreFromTimeAnnotationKey)
	if err != nil || restoreTimeStr == nil {
		return false, err
	}
	sourceCuster := cluster.Annotations[constant.RestoreFromSrcClusterAnnotationKey]
	if sourceCuster == "" {
		return false, errors.New("need specify a source cluster name to recovery")
	}
	restoreTime := &metav1.Time{}
	if err = restoreTime.UnmarshalQueryParameter(*restoreTimeStr); err != nil {
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

func (p *RestoreManager) getComponentBackupInfoFromAnnotation(compName, annotationKey string) (*string, error) {
	valueString := p.Cluster.Annotations[annotationKey]
	if len(valueString) == 0 {
		return nil, nil
	}
	backupMap := map[string]string{}
	err := json.Unmarshal([]byte(valueString), &backupMap)
	if err != nil {
		return nil, err
	}
	targetValue, ok := backupMap[compName]
	if !ok {
		return nil, nil
	}
	return &targetValue, nil
}

func getVolumeMount(spec *dpv1alpha1.BackupToolSpec) string {
	dataVolumeMount := "/data"
	// TODO: hack it because the mount path is not explicitly specified in cluster definition
	for _, env := range spec.Env {
		if env.Name == constant.DPVolumeDataDIR {
			dataVolumeMount = env.Value
			break
		}
	}
	return dataVolumeMount
}

// getRecoveryInfo gets the pitr recovery info from baseBackup and logfileBackup
func (p *RestoreManager) getRecoveryInfo(baseBackup, logfileBackup *dpv1alpha1.Backup) (*dpv1alpha1.BackupToolSpec, error) {
	// gets scripts from backup template
	backupTool := dpv1alpha1.BackupTool{}
	if err := p.Client.Get(p.Ctx, types.NamespacedName{
		Name: logfileBackup.Status.BackupToolName,
	}, &backupTool); err != nil {
		return nil, err
	}
	// build recovery env
	backupDIR := logfileBackup.Name
	if logfileBackup.Status.Manifests != nil && logfileBackup.Status.Manifests.BackupTool != nil {
		backupDIR = logfileBackup.Status.Manifests.BackupTool.FilePath
	}
	headEnv := []corev1.EnvVar{
		{Name: constant.DPBackupDIR, Value: backupVolumePATH + backupDIR},
		{Name: constant.DPBackupName, Value: logfileBackup.Name},
	}
	// build env of recovery time
	spec := &backupTool.Spec
	timeFormat := p.getTimeFormat(spec.Env)
	headEnv = append(headEnv, corev1.EnvVar{Name: constant.DPKBRecoveryTime, Value: p.restoreTime.UTC().Format(timeFormat)})
	headEnv = append(headEnv, corev1.EnvVar{Name: constant.DPKBRecoveryTimestamp, Value: strconv.FormatInt(p.restoreTime.Unix(), 10)})
	// build env of backup startTime and user contexts
	if baseBackup.Status.Manifests != nil {
		// inject env for backup startTime
		backupLog := baseBackup.Status.Manifests.BackupLog
		startTime := baseBackup.Status.StartTimestamp
		if backupLog != nil && backupLog.StartTime != nil {
			startTime = backupLog.StartTime
		}
		if startTime != nil {
			startTimeEnv := corev1.EnvVar{Name: constant.DPBackupStartTime, Value: startTime.UTC().Format(timeFormat)}
			startTimeTimestampEnv := corev1.EnvVar{Name: constant.DPBackupStartTimestamp, Value: strconv.FormatInt(startTime.Unix(), 10)}
			headEnv = append(headEnv, startTimeEnv, startTimeTimestampEnv)
		}
		// inject env for user contexts
		backupUserContext := baseBackup.Status.Manifests.UserContext
		for k, v := range backupUserContext {
			headEnv = append(headEnv, corev1.EnvVar{Name: strings.ToUpper(k), Value: v})
		}
	}
	spec.Env = append(headEnv, spec.Env...)
	return spec, nil
}

func (p *RestoreManager) getLogfileBackup(componentName string, sourceClusterUID string) (*dpv1alpha1.Backup, error) {
	logfileBackupList := dpv1alpha1.BackupList{}
	if err := p.Client.List(p.Ctx, &logfileBackupList,
		client.MatchingLabels{
			constant.AppInstanceLabelKey:    p.sourceCluster,
			constant.KBAppComponentLabelKey: componentName,
			constant.BackupTypeLabelKeyKey:  string(dpv1alpha1.BackupTypeLogFile),
		}); err != nil {
		return nil, err
	}
	if len(logfileBackupList.Items) == 0 {
		return nil, errors.New("not found logfile backups")
	}
	backups := p.sortBackups(logfileBackupList.Items, true)
	for _, v := range backups {
		// filter backups with cluster uid for excluding same cluster name
		if v.Labels[constant.DataProtectionLabelClusterUIDKey] == sourceClusterUID {
			return &v, nil
		}
	}
	// TODO: return an error if logfile backup is not found after v0.7.0, return the first logfile for compatibility with version v0.5.0.
	return &logfileBackupList.Items[0], nil
}

func (p *RestoreManager) getLogfilePVC(logfileBackup *dpv1alpha1.Backup) (*corev1.PersistentVolumeClaim, error) {
	pvcKey := types.NamespacedName{
		Name:      logfileBackup.Status.PersistentVolumeClaimName,
		Namespace: logfileBackup.Namespace,
	}
	pvc := corev1.PersistentVolumeClaim{}
	if err := p.Client.Get(p.Ctx, pvcKey, &pvc); err != nil {
		return nil, err
	}
	return &pvc, nil
}

func (p *RestoreManager) getDataPVCs(componentName string) ([]corev1.PersistentVolumeClaim, error) {
	dataPVCList := corev1.PersistentVolumeClaimList{}
	pvcLabels := map[string]string{
		constant.AppInstanceLabelKey:    p.Cluster.Name,
		constant.KBAppComponentLabelKey: componentName,
		constant.VolumeTypeLabelKey:     string(appsv1alpha1.VolumeTypeData),
	}
	if err := p.Client.List(p.Ctx, &dataPVCList,
		client.InNamespace(p.namespace),
		client.MatchingLabels(pvcLabels)); err != nil {
		return nil, err
	}
	return dataPVCList.Items, nil
}

// When the pvc has been bound on the determined pod,
// this is a little different from the getDataPVCs function,
// we need to get the node name of the pvc according to the pod,
// and the job must be the same as the node name of the pvc
func (p *RestoreManager) getDataPVCsAndPods(componentName string, podRestoreScope dpv1alpha1.PodRestoreScope) (map[string]corev1.Pod, error) {
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
	dataPVCsAndPodsMap := map[string]corev1.Pod{}
	for _, targetPod := range podList.Items {
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
			if podRestoreScope == dpv1alpha1.PodRestoreScopeAll {
				dataPVCsAndPodsMap[dataPVC.Name] = targetPod
				continue
			}
			if podRestoreScope == dpv1alpha1.PodRestoreScopeReadWrite {
				if targetPod.Labels[constant.ConsensusSetAccessModeLabelKey] == string(appsv1alpha1.ReadWrite) ||
					targetPod.Labels[constant.RoleLabelKey] == constant.Primary {
					dataPVCsAndPodsMap[dataPVC.Name] = targetPod
					break
				}
			}
		}
	}
	return dataPVCsAndPodsMap, nil
}

func (p *RestoreManager) getDataVCT(synthesizedComponent *component.SynthesizedComponent) corev1.PersistentVolumeClaimTemplate {
	vctMap := map[string]corev1.PersistentVolumeClaimTemplate{}
	for _, vct := range synthesizedComponent.VolumeClaimTemplates {
		vctMap[vct.Name] = vct
	}
	for _, vt := range synthesizedComponent.VolumeTypes {
		if vt.Type == appsv1alpha1.VolumeTypeData {
			return vctMap[vt.Name]

		}
	}
	if len(synthesizedComponent.VolumeClaimTemplates) != 0 {
		return synthesizedComponent.VolumeClaimTemplates[0]
	}
	return corev1.PersistentVolumeClaimTemplate{}
}

func (p *RestoreManager) createDataPVCs(synthesizedComponent *component.SynthesizedComponent, backup *dpv1alpha1.Backup) error {
	// determines the data volume type
	vct := p.getDataVCT(synthesizedComponent)
	if vct.Name == "" {
		return intctrlutil.NewNotFound("can not found any PersistentVolumeClaim of data type")
	}

	snapshotName := ""
	if backup != nil && backup.Spec.BackupType == dpv1alpha1.BackupTypeSnapshot {
		snapshotName = backup.Name
	}
	for i := int32(0); i < synthesizedComponent.Replicas; i++ {
		pvcName := fmt.Sprintf("%s-%s-%s-%d", vct.Name, p.Cluster.Name, synthesizedComponent.Name, i)
		pvcKey := types.NamespacedName{Namespace: p.Cluster.Namespace, Name: pvcName}
		pvc, err := builder.BuildPVC(p.Cluster, synthesizedComponent, &vct, pvcKey, snapshotName)
		if err != nil {
			return err
		}
		// Prevents halt recovery from checking uncleaned resources
		if pvc.Annotations == nil {
			pvc.Annotations = map[string]string{}
		}
		pvc.Annotations[constant.LastAppliedClusterAnnotationKey] =
			fmt.Sprintf(`{"metadata":{"uid":"%s","name":"%s"}}`, p.Cluster.UID, p.Cluster.Name)

		if err = p.Client.Create(p.Ctx, pvc); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (p *RestoreManager) getBackupObjectFromAnnotation(synthesizedComponent *component.SynthesizedComponent) (*dpv1alpha1.Backup, error) {
	backupSourceName, err := p.getComponentBackupInfoFromAnnotation(synthesizedComponent.Name, constant.RestoreFromBackUpAnnotationKey)
	if backupSourceName == nil || err != nil {
		return nil, err
	}
	backup := &dpv1alpha1.Backup{}
	if err = p.Client.Get(p.Ctx, types.NamespacedName{Name: *backupSourceName, Namespace: p.Cluster.Namespace}, backup); err != nil {
		return nil, err
	}
	return backup, nil
}

func (p *RestoreManager) BuildDatafileRestoreJob(synthesizedComponent *component.SynthesizedComponent, backup *dpv1alpha1.Backup) (objs []client.Object, err error) {
	pvcNames := make([]string, 0)
	vct := p.getDataVCT(synthesizedComponent)
	for i := int32(0); i < synthesizedComponent.Replicas; i++ {
		pvcNames = append(pvcNames, fmt.Sprintf("%s-%s-%s-%d", vct.Name, p.Cluster.Name, synthesizedComponent.Name, i))
	}
	return p.BuildDatafileRestoreJobByPVCS(synthesizedComponent, backup, pvcNames, p.BuildCommonLabels(synthesizedComponent))
}

func (p *RestoreManager) BuildDatafileRestoreJobByPVCS(synthesizedComponent *component.SynthesizedComponent, backup *dpv1alpha1.Backup, pvcNames []string, labels map[string]string) (objs []client.Object, err error) {
	backupToolKey := client.ObjectKey{Name: backup.Status.BackupToolName}
	backupTool := dpv1alpha1.BackupTool{}
	if err = p.Client.Get(p.Ctx, backupToolKey, &backupTool); err != nil {
		return nil, err
	}

	// builds backup volumes
	backupVolumeName := fmt.Sprintf("%s-%s", synthesizedComponent.Name, backup.Status.PersistentVolumeClaimName)
	remoteVolume := corev1.Volume{
		Name: backupVolumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: backup.Status.PersistentVolumeClaimName,
			},
		},
	}

	// builds volumeMounts
	remoteVolumeMount := corev1.VolumeMount{}
	remoteVolumeMount.Name = backupVolumeName
	remoteVolumeMount.MountPath = "/" + backup.Name
	allVolumeMounts := make([]corev1.VolumeMount, 0)
	allVolumeMounts = append(allVolumeMounts, remoteVolumeMount)
	allVolumeMounts = append(allVolumeMounts, synthesizedComponent.PodSpec.Containers[0].VolumeMounts...)
	volumeMountMap := map[string]corev1.VolumeMount{}
	for _, mount := range allVolumeMounts {
		volumeMountMap[mount.Name] = mount
	}

	// builds env
	env := []corev1.EnvVar{
		{
			Name:  constant.DPBackupName,
			Value: backup.Name,
		},
	}
	manifests := backup.Status.Manifests
	if manifests != nil && manifests.BackupTool != nil {
		env = append(env, corev1.EnvVar{Name: constant.DPBackupDIR, Value: fmt.Sprintf("/%s%s", backup.Name, manifests.BackupTool.FilePath)})
	}
	// merges env from backup tool.
	env = append(env, backupTool.Spec.Env...)
	objs = make([]client.Object, 0)
	vct := p.getDataVCT(synthesizedComponent)
	for _, pvcName := range pvcNames {
		dataVolume := corev1.Volume{
			Name: vct.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		}
		volumes := make([]corev1.Volume, 0)
		volumes = append(volumes, remoteVolume, dataVolume)
		volumes = append(volumes, synthesizedComponent.PodSpec.Volumes...)
		volumeMounts := make([]corev1.VolumeMount, 0)
		for _, volume := range volumes {
			if vmount, ok := volumeMountMap[volume.Name]; ok {
				volumeMounts = append(volumeMounts, vmount)
			}
		}
		jobName := p.GetDatafileRestoreJobName(pvcName)
		job, err := builder.BuildRestoreJob(p.Cluster, synthesizedComponent, jobName, backupTool.Spec.Image,
			backupTool.Spec.Physical.RestoreCommands, volumes, volumeMounts, env, backupTool.Spec.Resources)
		if err != nil {
			return nil, err
		}
		// if the workload uses local pv, the job's affinity should consistent with workload.
		// so datafile job should contain cluster affinity constraints.
		affinity := component.BuildAffinity(p.Cluster, p.Cluster.Spec.GetComponentByName(synthesizedComponent.Name))
		if job.Spec.Template.Spec.Affinity, err = component.BuildPodAffinity(p.Cluster, affinity, synthesizedComponent); err != nil {
			return nil, err
		}
		job.Spec.Template.ObjectMeta = metav1.ObjectMeta{
			Labels: labels,
		}
		if p.Scheme != nil {
			if err = controllerutil.SetControllerReference(p.Cluster, job, p.Scheme); err != nil {
				return nil, err
			}
		}
		job.SetLabels(labels)
		objs = append(objs, job)
	}
	return objs, nil
}

func (p *RestoreManager) buildPITRPhysicalRestoreJob(synthesizedComponent *component.SynthesizedComponent,
	recoveryInfo *dpv1alpha1.BackupToolSpec,
	logfileBackup *dpv1alpha1.Backup) (objs []client.Object, err error) {
	// gets data dir pvc name
	dataPVCs, err := p.getDataPVCs(synthesizedComponent.Name)
	if err != nil {
		return objs, err
	}
	if len(dataPVCs) == 0 {
		return objs, errors.New("not found data pvc")
	}
	// renders the pitrJob cue template
	image := recoveryInfo.Image
	if image == "" {
		image = synthesizedComponent.PodSpec.Containers[0].Image
	}
	logfilePVC, err := p.getLogfilePVC(logfileBackup)
	if err != nil {
		return objs, err
	}
	dataVolumeMount := getVolumeMount(recoveryInfo)
	volumeMounts := []corev1.VolumeMount{
		{Name: "data", MountPath: dataVolumeMount},
		{Name: "log", MountPath: backupVolumePATH},
	}
	// creates physical restore job
	for _, dataPVC := range dataPVCs {
		volumes := []corev1.Volume{
			{Name: "data", VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: dataPVC.GetName()}}},
			{Name: "log", VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: logfilePVC.GetName()}}},
		}
		pitrJobName := p.buildRestoreJobName(fmt.Sprintf("pitr-phy-%s", dataPVC.GetName()))
		pitrJob, err := builder.BuildRestoreJob(p.Cluster, synthesizedComponent, pitrJobName, image,
			recoveryInfo.Physical.RestoreCommands, volumes, volumeMounts, recoveryInfo.Env, recoveryInfo.Resources)
		if err != nil {
			return objs, err
		}
		if p.Scheme != nil {
			if err = controllerutil.SetControllerReference(p.Cluster, pitrJob, p.Scheme); err != nil {
				return nil, err
			}
		}
		pitrJob.SetLabels(p.BuildCommonLabels(synthesizedComponent))
		// collect pvcs and jobs for later deletion
		objs = append(objs, pitrJob)
	}

	return objs, nil
}

func (p *RestoreManager) buildLogicRestoreJob(synthesizedComponent *component.SynthesizedComponent, backup *dpv1alpha1.Backup, envs ...corev1.EnvVar) (objs []client.Object, err error) {
	// creates logic restore job, usually imported after the cluster service is started
	if p.Cluster.Status.Phase != appsv1alpha1.RunningClusterPhase {
		return nil, nil
	}
	backupToolKey := client.ObjectKey{Name: backup.Status.BackupToolName}
	backupTool := dpv1alpha1.BackupTool{}
	if err = p.Client.Get(p.Ctx, backupToolKey, &backupTool); err != nil {
		return nil, err
	}
	if backupTool.Spec.Logical == nil || len(backupTool.Spec.Logical.RestoreCommands) == 0 {
		return nil, nil
	}
	image := backupTool.Spec.Image
	if image == "" {
		image = synthesizedComponent.PodSpec.Containers[0].Image
	}
	dataVolumeMount := getVolumeMount(&backupTool.Spec)
	volumeMounts := []corev1.VolumeMount{
		{Name: "data", MountPath: dataVolumeMount},
		{Name: "backup-data", MountPath: backupVolumePATH},
	}
	pvcsAndPodsMap, err := p.getDataPVCsAndPods(synthesizedComponent.Name, backupTool.Spec.Logical.PodScope)
	if err != nil {
		return objs, err
	}
	jobEnv := backupTool.Spec.Env
	jobEnv = append(jobEnv, envs...)
	for pvcName, pod := range pvcsAndPodsMap {
		podENV := pod.Spec.Containers[0].Env
		podENV = append(podENV, corev1.EnvVar{Name: constant.DPDBHost, Value: intctrlutil.BuildPodHostDNS(&pod)})
		podENV = append(podENV, jobEnv...)
		volumes := []corev1.Volume{
			{Name: "data", VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}},
			{Name: "backup-data", VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: backup.Status.PersistentVolumeClaimName}}},
		}
		logicJobName := p.buildRestoreJobName(fmt.Sprintf("restore-logic-%s", pod.Name))
		logicJob, err := builder.BuildRestoreJob(p.Cluster, synthesizedComponent, logicJobName, image,
			backupTool.Spec.Logical.RestoreCommands, volumes, volumeMounts, podENV, backupTool.Spec.Resources)
		if err != nil {
			return objs, err
		}
		if p.Scheme != nil {
			if err = controllerutil.SetControllerReference(p.Cluster, logicJob, p.Scheme); err != nil {
				return nil, err
			}
		}
		logicJob.SetLabels(p.BuildCommonLabels(synthesizedComponent))
		// DO NOT use "volume.kubernetes.io/selected-node" annotation key in PVC, because it is unreliable.
		logicJob.Spec.Template.Spec.NodeName = pod.Spec.NodeName
		objs = append(objs, logicJob)
	}

	return objs, nil
}

func (p *RestoreManager) checkJobDone(key client.ObjectKey) (bool, error) {
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

func (p *RestoreManager) createJobsAndWaiting(objs []client.Object) error {
	// creates and checks into different loops to support concurrent resource creation.
	for _, job := range objs {
		fetchedJob := &batchv1.Job{}
		if err := p.Client.Get(p.Ctx, client.ObjectKeyFromObject(job), fetchedJob); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			if err = p.Client.Create(p.Ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
		}
	}
	for _, job := range objs {
		if done, err := p.checkJobDone(client.ObjectKeyFromObject(job)); err != nil {
			return err
		} else if !done {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, "waiting restore job %s", job.GetName())
		}
	}
	return nil
}

func (p *RestoreManager) cleanupJobs(objs []client.Object) error {
	if p.Cluster.Status.Phase == appsv1alpha1.RunningClusterPhase {
		for _, obj := range objs {
			if err := intctrlutil.BackgroundDeleteObject(p.Client, p.Ctx, obj); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *RestoreManager) cleanupClusterAnnotations() error {
	if p.Cluster.Status.Phase == appsv1alpha1.RunningClusterPhase && p.Cluster.Annotations != nil {
		cluster := p.Cluster
		patch := client.MergeFrom(cluster.DeepCopy())
		delete(cluster.Annotations, constant.RestoreFromSrcClusterAnnotationKey)
		delete(cluster.Annotations, constant.RestoreFromTimeAnnotationKey)
		delete(cluster.Annotations, constant.RestoreFromBackUpAnnotationKey)
		return p.Client.Patch(p.Ctx, cluster, patch)
	}
	return nil
}

// buildRestoreJobName builds the restore job name.
func (p *RestoreManager) buildRestoreJobName(jobName string) string {
	l := len(jobName)
	if l > 63 {
		return fmt.Sprintf("%s-%s", jobName[:57], jobName[l-5:l])
	}
	return jobName
}

func (p *RestoreManager) GetDatafileRestoreJobName(pvcName string) string {
	return p.buildRestoreJobName(fmt.Sprintf("base-%s", pvcName))
}

func (p *RestoreManager) BuildCommonLabels(synthesizedComponent *component.SynthesizedComponent) map[string]string {
	return map[string]string{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    p.Cluster.Name,
		constant.KBAppComponentLabelKey: synthesizedComponent.Name,
	}
}

func (p *RestoreManager) getTimeFormat(envs []corev1.EnvVar) string {
	for _, env := range envs {
		if env.Name == constant.DPTimeFormat {
			return env.Value
		}
	}
	return time.RFC3339
}
