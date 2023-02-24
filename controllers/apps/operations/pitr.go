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

package operations

import (
	"context"
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// byBackupCompletionTime sorts a list of jobs by completion timestamp, using their names as a tie breaker.
type byBackupCompletionTime []dpv1alpha1.Backup

// Len return the length of byBackupCompletionTime, for the sort.Sort
func (o byBackupCompletionTime) Len() int { return len(o) }

// Swap the items, for the sort.Sort
func (o byBackupCompletionTime) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

// Less define how to compare items, for the sort.Sort
func (o byBackupCompletionTime) Less(i, j int) bool {
	if o[i].Status.CompletionTimestamp == nil && o[j].Status.CompletionTimestamp != nil {
		return false
	}
	if o[i].Status.CompletionTimestamp != nil && o[j].Status.CompletionTimestamp == nil {
		return true
	}
	if o[i].Status.CompletionTimestamp.Equal(o[j].Status.CompletionTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].Status.CompletionTimestamp.Before(o[j].Status.CompletionTimestamp)
}

// byBackupCompletionTimeReverse reverse sorts a list of jobs by completion timestamp, using their names as a tie breaker.
type byBackupCompletionTimeReverse []dpv1alpha1.Backup

// Len return the length of byBackupCompletionTimeReverse, for the sort.Sort
func (o byBackupCompletionTimeReverse) Len() int { return len(o) }

// Swap the items, for the sort.Sort
func (o byBackupCompletionTimeReverse) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

// Less define how to compare items, for the sort.Sort
func (o byBackupCompletionTimeReverse) Less(i, j int) bool {
	if o[j].Status.CompletionTimestamp == nil && o[i].Status.CompletionTimestamp != nil {
		return false
	}
	if o[j].Status.CompletionTimestamp != nil && o[i].Status.CompletionTimestamp == nil {
		return true
	}
	if o[j].Status.CompletionTimestamp.Equal(o[i].Status.CompletionTimestamp) {
		return o[j].Name < o[i].Name
	}
	return o[j].Status.CompletionTimestamp.Before(o[i].Status.CompletionTimestamp)
}

// PointInTimeRecoveryManager  pitr manager functions
// 1. get latestBaseBackup
// 2. get future backup, if not found, create it
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
	resourceObjs  []client.Object
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
		if b.Status.Phase == dpv1alpha1.BackupCompleted {
			backupItems = append(backupItems, b)
		}
	}
	return backupItems, nil
}

// getSortedBackups sorted by CompletionTimestamp
func (p *PointInTimeRecoveryManager) getSortedBackups() ([]dpv1alpha1.Backup, error) {
	backups, err := p.listCompletedBackups()
	if err != nil {
		return backups, err
	}
	sort.Sort(byBackupCompletionTime(backups))
	return backups, nil
}

// getSortedBackups sorted by reverse CompletionTimestamp
func (p *PointInTimeRecoveryManager) getReverseSortedBackups() ([]dpv1alpha1.Backup, error) {
	backups, err := p.listCompletedBackups()
	if err != nil {
		return backups, err
	}
	sort.Sort(byBackupCompletionTimeReverse(backups))

	return backups, nil
}

// getLatestBaseBackup get the latest baseBackup
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
		return nil, fmt.Errorf("can not found latest base backup via restore time %s", p.restoreTime)
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
		return nil, fmt.Errorf("can not found next earliest base backup via restore time %s", p.restoreTime)
	}

	return nextBackup, nil
}

// checkAndInit check if cluster need to be restored, return value: true: need, false: no need
func (p *PointInTimeRecoveryManager) checkAndInit(cluster *appsv1alpha1.Cluster) (bool, error) {
	// check args if pitr supported
	if cluster.Annotations == nil {
		return false, nil
	}

	restoreTime := &metav1.Time{}
	restoreTimeStr, hasKey := cluster.Annotations["restore-from-time"]
	if !hasKey {
		return false, nil
	}
	sourceCluster, hasKey := cluster.Annotations["restore-from-cluster"]
	if !hasKey {
		return false, nil
	}
	if restoreTimeStr != "" {
		if err := restoreTime.UnmarshalQueryParameter(restoreTimeStr); err != nil {
			return false, err
		}
	}

	vctCount := 0
	for _, item := range p.Cluster.Spec.ComponentSpecs {
		vctCount += len(item.VolumeClaimTemplates)
	}
	if vctCount == 0 {
		return false, fmt.Errorf("not support pitr without any volume claim templates")
	}

	// init args
	p.restoreTime = restoreTime
	p.sourceCluster = sourceCluster
	p.namespace = cluster.Namespace
	return true, nil
}

func (p *PointInTimeRecoveryManager) getRecoveryInfo() (*dpv1alpha1.BackupPointInTimeRecovery, error) {
	// get scripts from backup template
	templateList := dpv1alpha1.BackupPolicyTemplateList{}
	if err := p.Client.List(p.Ctx, &templateList,
		client.MatchingLabels{
			constant.ClusterDefLabelKey: p.Cluster.Spec.ClusterDefRef,
		}); err != nil {

		return nil, err
	}
	if len(templateList.Items) == 0 {
		return nil, fmt.Errorf("not support recovery because of non-existed backupPolicyTemplate in clusterDef: %s",
			p.Cluster.Spec.ClusterDefRef)
	}
	recoveryInfo := templateList.Items[0].Spec.PointInTimeRecovery
	if nil == recoveryInfo {
		return nil, fmt.Errorf("not support recovery because of empty pitr definition in backupPolicyTemplate")
	}
	return recoveryInfo, nil
}

func (p *PointInTimeRecoveryManager) runScriptsJob() error {
	// build volumes from datasource
	baseBackup, err := p.getLatestBaseBackup()
	if err != nil {
		return err
	}

	for _, component := range p.Cluster.Spec.ComponentSpecs {
		if len(component.VolumeClaimTemplates) == 0 {
			continue
		}

		sts := &appsv1.StatefulSet{}
		vct := corev1.PersistentVolumeClaimTemplate{}
		vct.Name = component.VolumeClaimTemplates[0].Name
		vct.Spec = *(component.VolumeClaimTemplates[0].Spec)

		dataPVCName := fmt.Sprintf("data-%s-%s-0", p.Cluster.Name, component.Name)
		dataPVCKey := types.NamespacedName{
			Namespace: p.namespace,
			Name:      dataPVCName,
		}
		dataPVC, err := builder.BuildPVCFromSnapshot(sts, vct, dataPVCKey, baseBackup.Name)
		if err != nil {
			return err
		}

		nextBackup, err := p.getNextBackup()
		if err != nil {
			return err
		}
		pitrPVCName := fmt.Sprintf("pitr-%s-%s-0", p.Cluster.Name, component.Name)
		pitrPVCKey := types.NamespacedName{
			Namespace: p.namespace,
			Name:      pitrPVCName,
		}
		pitrPVC, err := builder.BuildPVCFromSnapshot(sts, vct, pitrPVCKey, nextBackup.Name)
		if err != nil {
			return err
		}
		if pitrPVC.Annotations == nil {
			pitrPVC.Annotations = map[string]string{}
		}
		volumes := []corev1.Volume{
			{Name: "data", VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: dataPVCName}}},
			{Name: "log", VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pitrPVCName}}},
		}
		volumeMounts := []corev1.VolumeMount{
			{Name: "data", MountPath: "/data"},
			{Name: "log", MountPath: "/log"},
		}

		recoveryInfo, err := p.getRecoveryInfo()
		if err != nil {
			return err
		}

		// render the job cue template
		job, err := builder.BuildPITRJob(p.Cluster, recoveryInfo.Scripts.Image, recoveryInfo.Scripts.Command, volumes, volumeMounts)
		if err != nil {
			return err
		}
		if job.Annotations == nil {
			job.Annotations = map[string]string{}
		}
		// collect pvcs and jobs for later deletion
		p.resourceObjs = append(p.resourceObjs, pitrPVC)
		p.resourceObjs = append(p.resourceObjs, job)

		err = p.Client.Create(p.Ctx, dataPVC)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
		err = p.Client.Create(p.Ctx, pitrPVC)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
		err = p.Client.Create(p.Ctx, job)
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
			return true, fmt.Errorf(jobStatusCondition.Reason)
		}
	}
	// if found, return true
	return false, nil
}

func (p *PointInTimeRecoveryManager) ensureScriptsJobDone() bool {
	var jobObj *batchv1.Job
	var ok bool
	for _, obj := range p.resourceObjs {
		if jobObj, ok = obj.(*batchv1.Job); !ok {
			continue
		}
		done, err := p.checkJobDone(client.ObjectKeyFromObject(jobObj))
		if err != nil {
			return false
		}
		return done
	}
	return false
}

func (p *PointInTimeRecoveryManager) cleanupScriptsJob() error {
	for _, obj := range p.resourceObjs {
		if err := intctrlutil.BackgroundDeleteObject(p.Client, p.Ctx, obj); err != nil {
			return err
		}
	}
	return nil
}

// DoPrepare prepare data before point in time recovery
func (p *PointInTimeRecoveryManager) DoPrepare(cluster *appsv1alpha1.Cluster) (shouldRequeue bool, err error) {
	shouldRequeue = false
	if required, err := p.checkAndInit(cluster); err != nil {
		return shouldRequeue, err
	} else if !required {
		return shouldRequeue, nil
	}

	// mount the data+log pvc, and run scripts job to prepare data
	if err := p.runScriptsJob(); err != nil {
		return shouldRequeue, err
	}

	// check job done
	if !p.ensureScriptsJobDone() {
		return true, nil
	}

	// clean up job
	if err != p.cleanupScriptsJob() {
		return shouldRequeue, nil
	}

	return shouldRequeue, nil
}

// MergeConfigMap to merge from config when recovery to point time from cluster.
func (p *PointInTimeRecoveryManager) MergeConfigMap(configMap *corev1.ConfigMap) error {
	if required, err := p.checkAndInit(p.Cluster); err != nil {
		return err
	} else if !required {
		return nil
	}

	recoveryInfo, err := p.getRecoveryInfo()
	if err != nil {
		return nil
	}

	// replace config variables
	pitrConfigMap := recoveryInfo.Config
	timeFormat := recoveryInfo.Config["timeFormat"]
	for key, val := range pitrConfigMap {
		if v, ok := configMap.Data[key]; ok {
			restoreTimeStr := p.restoreTime.Time.UTC().Format(timeFormat)
			pitrConfigMap[key] = strings.Replace(val, "$KB_RECOVERY_TIME", restoreTimeStr, 1)
			// append pitr config map into cluster config
			configMap.Data[key] = v + "\n" + pitrConfigMap[key]
		}
	}
	return nil
}
