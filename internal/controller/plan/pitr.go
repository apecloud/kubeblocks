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
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// PointInTimeRecoveryManager  pitr manager functions
// 1. get the latest base backup
// 2. get the next earliest backup
// 3. add log pvc by datasource volume snapshot
// 3. update configuration
// 4. create init container to prepare log
// 5. end
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
)

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

// getSortedBackups sorted by BackupLog.StopTime
func (p *PointInTimeRecoveryManager) getSortedBackups() ([]dpv1alpha1.Backup, error) {
	backups, err := p.listCompletedBackups()
	if err != nil {
		return backups, err
	}
	sort.Slice(backups, func(i, j int) bool {
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

// getSortedBackups sorted by reverse StopTime
func (p *PointInTimeRecoveryManager) getReverseSortedBackups() ([]dpv1alpha1.Backup, error) {
	backups, err := p.listCompletedBackups()
	if err != nil {
		return backups, err
	}
	sort.Slice(backups, func(i, j int) bool {
		if backups[j].Status.Manifests.BackupLog.StopTime == nil && backups[i].Status.Manifests.BackupLog.StopTime != nil {
			return false
		}
		if backups[j].Status.Manifests.BackupLog.StopTime != nil && backups[i].Status.Manifests.BackupLog.StopTime == nil {
			return true
		}
		if backups[j].Status.Manifests.BackupLog.StopTime.Equal(backups[i].Status.Manifests.BackupLog.StopTime) {
			return backups[j].Name < backups[i].Name
		}
		return backups[j].Status.Manifests.BackupLog.StopTime.Before(backups[i].Status.Manifests.BackupLog.StopTime)
	})

	return backups, nil
}

// getLatestBaseBackup gets the latest baseBackup
func (p *PointInTimeRecoveryManager) getLatestBaseBackup() (*dpv1alpha1.Backup, error) {
	// 1. sort backups by completed timestamp
	backups, err := p.getReverseSortedBackups()
	if err != nil {
		return nil, err
	}

	// 2. get the latest backup object
	var latestBackup *dpv1alpha1.Backup
	for _, item := range backups {
		if p.restoreTime.Time.After(item.Status.CompletionTimestamp.Time) {
			latestBackup = &item
			break
		}
	}
	if latestBackup == nil {
		return nil, errors.New("can not found latest base backup")
	}

	return latestBackup, nil
}

func (p *PointInTimeRecoveryManager) getNextBackup() (*dpv1alpha1.Backup, error) {
	// 1. sort backups by reverse completed timestamp
	backups, err := p.getSortedBackups()
	if err != nil {
		return nil, err
	}

	// 2. get the next earliest backup object
	var nextBackup *dpv1alpha1.Backup
	for _, item := range backups {
		if p.restoreTime.Before(item.Status.CompletionTimestamp) {
			nextBackup = &item
			break
		}
	}
	if nextBackup == nil {
		return nil, errors.New("can not found next earliest base backup")
	}

	return nextBackup, nil
}

// checkAndInit checks if cluster need to be restored, return value: true: need, false: no need
func (p *PointInTimeRecoveryManager) checkAndInit() (need bool, err error) {
	// check args if pitr supported
	cluster := p.Cluster
	if cluster.Annotations == nil || cluster.Annotations[constant.RestoreFromTimeAnnotationKey] == "" {
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

func (p *PointInTimeRecoveryManager) getRecoveryInfo() (*dpv1alpha1.BackupToolSpec, error) {
	// get scripts from backup template
	toolList := dpv1alpha1.BackupToolList{}
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

	return spec, nil
}

func (p *PointInTimeRecoveryManager) buildResourceObjs() (objs []client.Object, err error) {
	objs = make([]client.Object, 0)
	recoveryInfo, err := p.getRecoveryInfo()
	if err != nil {
		return objs, err
	}
	for _, componentSpec := range p.Cluster.Spec.ComponentSpecs {
		if len(componentSpec.VolumeClaimTemplates) == 0 {
			continue
		}

		sts := &appsv1.StatefulSet{}
		vct := corev1.PersistentVolumeClaimTemplate{}
		vct.Name = componentSpec.VolumeClaimTemplates[0].Name
		vct.Spec = componentSpec.VolumeClaimTemplates[0].Spec.ToV1PersistentVolumeClaimSpec()

		// get data dir pvc name
		dataPVCList := corev1.PersistentVolumeClaimList{}
		dataPVCLabels := map[string]string{
			constant.AppInstanceLabelKey:    p.Cluster.Name,
			constant.KBAppComponentLabelKey: componentSpec.Name,
			constant.VolumeTypeLabelKey:     string(appsv1alpha1.VolumeTypeData),
		}
		if err = p.Client.List(p.Ctx, &dataPVCList,
			client.InNamespace(p.namespace),
			client.MatchingLabels(dataPVCLabels)); err != nil {
			return objs, err
		}
		if len(dataPVCList.Items) == 0 {
			return objs, errors.New("not found data pvc")
		}
		for i, dataPVC := range dataPVCList.Items {
			if dataPVC.Status.Phase != corev1.ClaimBound {
				return objs, errors.New("waiting PVC Bound")
			}

			nextBackup, err := p.getNextBackup()
			if err != nil {
				return objs, err
			}
			pitrPVCName := fmt.Sprintf("pitr-%s-%s-%d", p.Cluster.Name, componentSpec.Name, i)
			pitrPVCKey := types.NamespacedName{
				Namespace: p.namespace,
				Name:      pitrPVCName,
			}
			pitrPVC, err := builder.BuildPVCFromSnapshot(sts, vct, pitrPVCKey, nextBackup.Name, nil)
			if err != nil {
				return objs, err
			}
			volumes := []corev1.Volume{
				{Name: "data", VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: dataPVC.Name}}},
				{Name: "log", VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pitrPVCName}}},
			}
			volumeMounts := []corev1.VolumeMount{
				{Name: "data", MountPath: "/data"},
				{Name: "log", MountPath: "/log"},
			}

			// render the job cue template
			image := recoveryInfo.Image
			if image == "" {
				image = viper.GetString(constant.KBToolsImage)
			}
			jobName := fmt.Sprintf("pitr-phy-%s-%s-%d", p.Cluster.Name, componentSpec.Name, i)
			job, err := builder.BuildPITRJob(jobName, p.Cluster, image, []string{"sh", "-c"},
				recoveryInfo.Physical.RestoreCommands, volumes, volumeMounts, recoveryInfo.Env)
			if err != nil {
				return objs, err
			}
			// create logic restore job
			if p.Cluster.Status.Phase == appsv1alpha1.RunningClusterPhase {
				logicJobName := fmt.Sprintf("pitr-logic-%s-%s-%d", p.Cluster.Name, componentSpec.Name, i)
				logicJob, err := builder.BuildPITRJob(logicJobName, p.Cluster, image, []string{"sh", "-c"},
					recoveryInfo.Logical.RestoreCommands, volumes, volumeMounts, recoveryInfo.Env)
				if err != nil {
					return objs, err
				}
				objs = append(objs, logicJob)
			}
			// collect pvcs and jobs for later deletion
			objs = append(objs, pitrPVC)
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

// DoRecoveryJob run a physical recovery job before cluster service start
func (p *PointInTimeRecoveryManager) DoRecoveryJob() (shouldRequeue bool, err error) {
	if need, err := p.checkAndInit(); err != nil {
		return false, err
	} else if !need {
		return false, nil
	}

	// mount the data+log pvc, and run scripts job to prepare data
	if err = p.runRecoveryJob(); err != nil {
		if err.Error() == "waiting PVC Bound" {
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

// DoPrepare prepare init container and pvc before point in time recovery
func (p *PointInTimeRecoveryManager) DoPrepare(component *component.SynthesizedComponent) error {
	if need, err := p.checkAndInit(); err != nil {
		return err
	} else if !need {
		return nil
	}
	// prepare init container
	container := corev1.Container{}
	container.Name = initContainerName
	container.Image = viper.GetString(constant.KBToolsImage)
	container.Command = []string{"sleep", "infinity"}
	component.PodSpec.InitContainers = append(component.PodSpec.InitContainers, container)

	// prepare data pvc
	if len(component.VolumeClaimTemplates) == 0 {
		return errors.New("not found data pvc")
	}
	latestBackup, err := p.getLatestBaseBackup()
	if err != nil {
		return err
	}

	vct := component.VolumeClaimTemplates[0]
	snapshotAPIGroup := snapshotv1.GroupName
	vct.Spec.DataSource = &corev1.TypedLocalObjectReference{
		APIGroup: &snapshotAPIGroup,
		Kind:     constant.VolumeSnapshotKind,
		Name:     latestBackup.Name,
	}
	component.VolumeClaimTemplates[0] = vct
	return nil
}

// removeStsInitContainerForRestore removes the statefulSet's init container after recovery job completed.
func (p *PointInTimeRecoveryManager) removeStsInitContainer(
	cluster *appsv1alpha1.Cluster,
	componentName string) error {
	// get the sts list of component
	stsList := &appsv1.StatefulSetList{}
	if err := util.GetObjectListByComponentName(p.Ctx, p.Client, *cluster, stsList, componentName); err != nil {
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

// DoPITRPrepare prepare init container and pvc before point in time recovery
func DoPITRPrepare(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) error {
	if cluster.Status.ObservedGeneration >= 1 {
		return nil
	}

	// build pitr init container to wait prepare data
	// prepare data if PITR needed
	pitrMgr := PointInTimeRecoveryManager{
		Cluster: cluster,
		Client:  cli,
		Ctx:     ctx,
	}
	if err := pitrMgr.DoPrepare(component); err != nil {
		return err
	}
	return nil
}

// DoPITRIfNeed checks if run restore job and copy data for point in time recovery
func DoPITRIfNeed(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster) (shouldRequeue bool, err error) {
	if cluster.Status.ObservedGeneration != 1 {
		return false, nil
	}
	pitrMgr := PointInTimeRecoveryManager{
		Cluster: cluster,
		Client:  cli,
		Ctx:     ctx,
	}
	return pitrMgr.DoRecoveryJob()
}

// DoPITRCleanup cleanup resource and config after point in time recovery
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
