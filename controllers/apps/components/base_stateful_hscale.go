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

package components

import (
	"fmt"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type dataClone interface {
	enabled() bool
	// succeed check if data clone succeeded
	succeed() (bool, error)
	// cloneData do clone data, return objects that need to be created
	cloneData(dataClone) ([]client.Object, error)
	// clearTmpResources clear all the temporary resources created during data clone, return objects that need to be deleted
	clearTmpResources() ([]client.Object, error)
	checkBackupStatus() (backupStatus, error)
	backup() ([]client.Object, error)
	checkRestoreStatus(types.NamespacedName) (backupStatus, error)
	restore(name types.NamespacedName) ([]client.Object, error)
}

type backupStatus string

const (
	backupStatusNotCreated backupStatus = "NotCreated"
	backupStatusProcessing backupStatus = "Processing"
	backupStatusReadyToUse backupStatus = "ReadyToUse"
	backupStatusFailed     backupStatus = "Failed"
)

func newDataClone(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	stsObj *appsv1.StatefulSet,
	stsProto *appsv1.StatefulSet,
	key types.NamespacedName) (dataClone, error) {
	if component == nil || component.HorizontalScalePolicy == nil {
		return nil, nil
	}
	if component.HorizontalScalePolicy.Type == appsv1alpha1.HScaleDataClonePolicyCloneVolume {
		snapshot := &snapshotDataClone{
			baseDataClone{
				reqCtx:    reqCtx,
				cli:       cli,
				cluster:   cluster,
				component: component,
				stsObj:    stsObj,
				stsProto:  stsProto,
				key:       key,
			},
		}
		if snapshot.enabled() {
			return snapshot, nil
		}
		backupTool := &backupDataClone{
			baseDataClone{
				reqCtx:    reqCtx,
				cli:       cli,
				cluster:   cluster,
				component: component,
				stsObj:    stsObj,
				stsProto:  stsProto,
				key:       key,
			},
		}
		if backupTool.enabled() {
			return backupTool, nil
		}
		return nil, fmt.Errorf("h-scale policy is Backup but neither snapshot nor backup tool is enabled")
	}
	return nil, nil
}

type baseDataClone struct {
	reqCtx    intctrlutil.RequestCtx
	cli       client.Client
	cluster   *appsv1alpha1.Cluster
	component *component.SynthesizedComponent
	stsObj    *appsv1.StatefulSet
	stsProto  *appsv1.StatefulSet
	key       types.NamespacedName
}

func (d *baseDataClone) cloneData(realDataClone dataClone) ([]client.Object, error) {

	objs := make([]client.Object, 0)

	// check backup ready
	status, err := realDataClone.checkBackupStatus()
	if err != nil {
		return nil, err
	}
	switch status {
	case backupStatusNotCreated:
		// create backup
		backupObjs, err := realDataClone.backup()
		if err != nil {
			return nil, err
		}
		objs = append(objs, backupObjs...)
		return objs, nil
	case backupStatusProcessing:
		// requeue to waiting for backup ready
		return objs, nil
	case backupStatusReadyToUse:
		break
	}
	// backup's ready, then start to check restore
	pvcKeys := d.toCreatePVCKeys()
	for _, pvcKey := range pvcKeys {
		restoreStatus, err := realDataClone.checkRestoreStatus(pvcKey)
		if err != nil {
			return nil, err
		}
		switch restoreStatus {
		case backupStatusNotCreated:

			restoreObjs, err := realDataClone.restore(pvcKey)
			if err != nil {
				return nil, err
			}
			objs = append(objs, restoreObjs...)
		case backupStatusProcessing:
		case backupStatusReadyToUse:
			continue
		case backupStatusFailed:
			return nil, fmt.Errorf("restore failed")
		}
	}

	// restore to pvcs all ready
	return objs, nil
}

func (d *baseDataClone) isPVCExists(pvcKey types.NamespacedName) (bool, error) {
	pvc := corev1.PersistentVolumeClaim{}
	if err := d.cli.Get(d.reqCtx.Ctx, pvcKey, &pvc); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	return true, nil
}

func (d *baseDataClone) checkAllPVCsExist() (bool, error) {
	for i := *d.stsObj.Spec.Replicas; i < d.component.Replicas; i++ {
		for _, vct := range d.stsObj.Spec.VolumeClaimTemplates {
			pvcKey := types.NamespacedName{
				Namespace: d.stsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-%d", vct.Name, d.stsObj.Name, i),
			}
			// check pvc existence
			pvcExists, err := d.isPVCExists(pvcKey)
			if err != nil {
				return true, err
			}
			if !pvcExists {
				return false, nil
			}
		}
	}
	return true, nil
}

func (d *baseDataClone) backupVCT() *corev1.PersistentVolumeClaimTemplate {
	vcts := d.component.VolumeClaimTemplates
	vct := vcts[0]
	for _, tmpVct := range vcts {
		for _, volumeType := range d.component.VolumeTypes {
			if volumeType.Type == appsv1alpha1.VolumeTypeData && volumeType.Name == tmpVct.Name {
				vct = tmpVct
				break
			}
		}
	}
	return &vct
}

func (d *baseDataClone) toCreatePVCKeys() []types.NamespacedName {
	var pvcKeys []types.NamespacedName
	vct := d.backupVCT()
	for i := *d.stsObj.Spec.Replicas; i < *d.stsProto.Spec.Replicas; i++ {
		pvcKey := types.NamespacedName{
			Namespace: d.stsObj.Namespace,
			Name: fmt.Sprintf("%s-%s-%d",
				vct.Name,
				d.stsObj.Name,
				i),
		}
		pvcKeys = append(pvcKeys, pvcKey)
	}
	return pvcKeys
}

func (d *baseDataClone) getBackupMatchingLabels() client.MatchingLabels {
	return client.MatchingLabels{
		constant.AppInstanceLabelKey:    d.cluster.Name,
		constant.KBAppComponentLabelKey: d.component.Name,
		constant.KBManagedByKey:         "cluster", // the resources are managed by which controller
	}
}

type snapshotDataClone struct {
	baseDataClone
}

func (d *snapshotDataClone) enabled() bool {
	return viper.GetBool("VOLUMESNAPSHOT")
}

var _ dataClone = &snapshotDataClone{}

func (d *snapshotDataClone) succeed() (bool, error) {
	if len(d.component.VolumeClaimTemplates) == 0 {
		d.reqCtx.Recorder.Eventf(d.cluster,
			corev1.EventTypeNormal,
			"HorizontalScale",
			"no VolumeClaimTemplates, no need to do data clone.")
		return true, nil
	}
	return d.checkAllPVCsExist()
}

func (d *snapshotDataClone) clearTmpResources() ([]client.Object, error) {
	allPVCBound, err := d.isAllPVCBound()
	if err != nil {
		return nil, err
	}
	if !allPVCBound {
		return nil, nil
	}
	return d.deleteSnapshot()
}

func (d *snapshotDataClone) backup() ([]client.Object, error) {
	objs := make([]client.Object, 0)
	backupPolicyTplName := d.component.HorizontalScalePolicy.BackupPolicyTemplateName

	backupPolicyTemplate := &appsv1alpha1.BackupPolicyTemplate{}
	err := d.cli.Get(d.reqCtx.Ctx, client.ObjectKey{Name: backupPolicyTplName}, backupPolicyTemplate)
	if err != nil {
		return nil, err
	}

	// if there is backuppolicytemplate created by provider
	backupPolicy, err := getBackupPolicyFromTemplate(d.reqCtx, d.cli, d.cluster, d.component.ComponentDef, backupPolicyTplName)
	if err != nil {
		return nil, err
	}
	if backupPolicy == nil {
		return nil, intctrlutil.NewNotFound("not found any backup policy created by %s", backupPolicyTplName)
	}
	backup, err := builder.BuildBackup(d.cluster, d.component, backupPolicy.Name, d.key, "snapshot")
	if err != nil {
		return nil, err
	}
	objs = append(objs, backup)
	d.reqCtx.Recorder.Eventf(d.cluster, corev1.EventTypeNormal, "BackupJobCreate", "Create backupJob/%s", d.key.Name)
	return objs, nil
}

func (d *snapshotDataClone) checkBackupStatus() (backupStatus, error) {
	backupPolicyTplName := d.component.HorizontalScalePolicy.BackupPolicyTemplateName
	backupPolicyTemplate := &appsv1alpha1.BackupPolicyTemplate{}
	err := d.cli.Get(d.reqCtx.Ctx, client.ObjectKey{Name: backupPolicyTplName}, backupPolicyTemplate)
	if err != nil {
		return backupStatusFailed, err
	}
	backup := dataprotectionv1alpha1.Backup{}
	if err := d.cli.Get(d.reqCtx.Ctx, d.key, &backup); err != nil {
		if errors.IsNotFound(err) {
			return backupStatusNotCreated, nil
		} else {
			return backupStatusFailed, err
		}
	}
	if backup.Status.Phase == dataprotectionv1alpha1.BackupFailed {
		return backupStatusFailed, intctrlutil.NewErrorf(intctrlutil.ErrorTypeBackupFailed, "backup for horizontalScaling failed: %s",
			backup.Status.FailureReason)
	}
	if backup.Status.Phase != dataprotectionv1alpha1.BackupCompleted {
		return backupStatusProcessing, nil
	}
	return backupStatusReadyToUse, nil
}

func (d *snapshotDataClone) restore(pvcKey types.NamespacedName) ([]client.Object, error) {
	objs := make([]client.Object, 0)
	vct := d.backupVCT()
	// create pvc from snapshot for every new pod
	if pvc, err := d.checkedCreatePVCFromSnapshot(
		pvcKey,
		vct); err != nil {
		d.reqCtx.Log.Error(err, "checkedCreatePVCFromSnapshot failed")
		return nil, err
	} else if pvc != nil {
		objs = append(objs, pvc)
	}
	return objs, nil
}

func (d *snapshotDataClone) checkRestoreStatus(pvcKey types.NamespacedName) (backupStatus, error) {
	pvc := corev1.PersistentVolumeClaim{}
	if err := d.cli.Get(d.reqCtx.Ctx, pvcKey, &pvc); err != nil {
		if errors.IsNotFound(err) {
			return backupStatusNotCreated, nil
		}
		return backupStatusFailed, err
	}
	return backupStatusReadyToUse, nil
}

func (d *snapshotDataClone) listVolumeSnapshotByLabels(vsList *snapshotv1.VolumeSnapshotList, ml client.MatchingLabels) error {
	compatClient := intctrlutil.VolumeSnapshotCompatClient{ReadonlyClient: d.cli, Ctx: d.reqCtx.Ctx}
	// get vs from backup.
	backupList := dataprotectionv1alpha1.BackupList{}
	if err := d.cli.List(d.reqCtx.Ctx, &backupList, ml); err != nil {
		return err
	} else if len(backupList.Items) == 0 {
		// ignore not found
		return nil
	}
	return compatClient.List(vsList, client.MatchingLabels{
		constant.DataProtectionLabelBackupNameKey: backupList.Items[0].Name,
	})
}

func (d *snapshotDataClone) checkedCreatePVCFromSnapshot(pvcKey types.NamespacedName,
	vct *corev1.PersistentVolumeClaimTemplate) (client.Object, error) {
	pvc := corev1.PersistentVolumeClaim{}
	// check pvc existence
	if err := d.cli.Get(d.reqCtx.Ctx, pvcKey, &pvc); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		ml := d.getBackupMatchingLabels()
		vsList := snapshotv1.VolumeSnapshotList{}
		if err = d.listVolumeSnapshotByLabels(&vsList, ml); err != nil {
			return nil, err
		}
		if len(vsList.Items) == 0 {
			return nil, fmt.Errorf("volumesnapshot not found in cluster %s component %s", d.cluster.Name, d.component.Name)
		}
		// exclude volumes that are deleting
		vsName := ""
		for _, vs := range vsList.Items {
			if vs.DeletionTimestamp != nil {
				continue
			}
			vsName = vs.Name
			break
		}
		return d.createPVCFromSnapshot(vct, pvcKey, vsName)
	}
	return nil, nil
}

func (d *snapshotDataClone) createPVCFromSnapshot(
	vct *corev1.PersistentVolumeClaimTemplate,
	pvcKey types.NamespacedName,
	snapshotName string) (client.Object, error) {
	pvc, err := builder.BuildPVC(d.cluster, d.component, vct, pvcKey, snapshotName)
	if err != nil {
		return nil, err
	}
	return pvc, nil
}

func (d *snapshotDataClone) deleteSnapshot() ([]client.Object, error) {
	objs, err := d.deleteBackup()
	if err != nil {
		return nil, err
	}
	if len(objs) > 0 {
		d.reqCtx.Recorder.Eventf(d.cluster, corev1.EventTypeNormal, "BackupJobDelete", "Delete backupJob/%s", d.key.Name)
	}

	return objs, nil
}

// deleteBackup will delete all backup related resources created during horizontal scaling
func (d *snapshotDataClone) deleteBackup() ([]client.Object, error) {
	ml := d.getBackupMatchingLabels()
	backupList := dataprotectionv1alpha1.BackupList{}
	if err := d.cli.List(d.reqCtx.Ctx, &backupList, ml); err != nil {
		return nil, err
	}
	objs := make([]client.Object, 0)
	for i := range backupList.Items {
		objs = append(objs, &backupList.Items[i])
	}
	return objs, nil
}

func (d *snapshotDataClone) isAllPVCBound() (bool, error) {
	if len(d.stsObj.Spec.VolumeClaimTemplates) == 0 {
		return true, nil
	}
	for i := 0; i < int(d.component.Replicas); i++ {
		pvcKey := types.NamespacedName{
			Namespace: d.stsObj.Namespace,
			Name:      fmt.Sprintf("%s-%s-%d", d.stsObj.Spec.VolumeClaimTemplates[0].Name, d.stsObj.Name, i),
		}
		pvc := corev1.PersistentVolumeClaim{}
		// check pvc existence
		if err := d.cli.Get(d.reqCtx.Ctx, pvcKey, &pvc); err != nil {
			return false, client.IgnoreNotFound(err)
		}
		if pvc.Status.Phase != corev1.ClaimBound {
			return false, nil
		}
	}
	return true, nil
}

type backupDataClone struct {
	baseDataClone
}

func (d *backupDataClone) enabled() bool {
	return len(viper.GetString(constant.CfgKeyBackupPVCName)) > 0
}

var _ dataClone = &backupDataClone{}

func (d *backupDataClone) succeed() (bool, error) {
	allPVCsExist, err := d.checkAllPVCsExist()
	if err != nil || !allPVCsExist {
		return allPVCsExist, err
	}
	pvcKeys := d.toCreatePVCKeys()
	for _, pvcKey := range pvcKeys {
		restoreStatus, err := d.checkRestoreStatus(pvcKey)
		if err != nil {
			return false, err
		}
		if restoreStatus != backupStatusReadyToUse {
			return false, nil
		}
	}
	return true, nil
}

func (d *backupDataClone) clearTmpResources() ([]client.Object, error) {
	objs := make([]client.Object, 0)
	// delete backup
	ml := d.getBackupMatchingLabels()
	backupList := dataprotectionv1alpha1.BackupList{}
	if err := d.cli.List(d.reqCtx.Ctx, &backupList, ml); err != nil {
		return nil, err
	}
	for i := range backupList.Items {
		objs = append(objs, &backupList.Items[i])
	}
	// delete restore job
	jobList := v1.JobList{}
	if err := d.cli.List(d.reqCtx.Ctx, &jobList, ml); err != nil {
		return nil, err
	}
	for i := range jobList.Items {
		objs = append(objs, &jobList.Items[i])
	}
	return objs, nil
}

func (d *backupDataClone) backup() ([]client.Object, error) {
	objs := make([]client.Object, 0)
	backupPolicyTplName := d.component.HorizontalScalePolicy.BackupPolicyTemplateName
	backupPolicy, err := getBackupPolicyFromTemplate(d.reqCtx, d.cli, d.cluster, d.component.ComponentDef, backupPolicyTplName)
	if err != nil {
		return nil, err
	}
	if backupPolicy == nil {
		return nil, intctrlutil.NewNotFound("not found any backup policy created by %s", backupPolicyTplName)
	}
	backup, err := builder.BuildBackup(d.cluster, d.component, backupPolicy.Name, d.key, "datafile")
	if err != nil {
		return nil, err
	}
	objs = append(objs, backup)
	return objs, nil
}

func (d *backupDataClone) checkBackupStatus() (backupStatus, error) {
	backup := dataprotectionv1alpha1.Backup{}
	if err := d.cli.Get(d.reqCtx.Ctx, d.key, &backup); err != nil {
		if errors.IsNotFound(err) {
			return backupStatusNotCreated, nil
		} else {
			return backupStatusFailed, err
		}
	}
	if backup.Status.Phase == dataprotectionv1alpha1.BackupFailed {
		return backupStatusFailed, fmt.Errorf("failed to backup: %s", backup.Status.FailureReason)
	}
	if backup.Status.Phase == dataprotectionv1alpha1.BackupCompleted {
		return backupStatusReadyToUse, nil
	}
	return backupStatusProcessing, nil
}

func (d *backupDataClone) restore(pvcKey types.NamespacedName) ([]client.Object, error) {
	objs := make([]client.Object, 0)
	backup := dataprotectionv1alpha1.Backup{}
	if err := d.cli.Get(d.reqCtx.Ctx, d.key, &backup); err != nil {
		return nil, err
	}
	pvc, err := builder.BuildPVC(d.cluster, d.component, d.backupVCT(), pvcKey, "")
	if err != nil {
		return nil, err
	}
	objs = append(objs, pvc)
	restoreMgr := plan.NewRestoreManager(d.reqCtx.Ctx, d.cli, d.cluster, nil)
	restoreJobs, err := restoreMgr.BuildDatafileRestoreJobByPVCS(d.baseDataClone.component, &backup, []string{pvc.Name}, d.getBackupMatchingLabels())
	if err != nil {
		return nil, err
	}
	objs = append(objs, restoreJobs...)
	return objs, nil
}

func (d *backupDataClone) checkRestoreStatus(pvcKey types.NamespacedName) (backupStatus, error) {
	job := v1.Job{}
	restoreMgr := plan.NewRestoreManager(d.reqCtx.Ctx, d.cli, d.cluster, nil)
	jobName := restoreMgr.GetDatafileRestoreJobName(pvcKey.Name)
	if err := d.cli.Get(d.reqCtx.Ctx, types.NamespacedName{Namespace: pvcKey.Namespace, Name: jobName}, &job); err != nil {
		if errors.IsNotFound(err) {
			return backupStatusNotCreated, nil
		} else {
			return backupStatusNotCreated, err
		}
	}
	if job.Status.Succeeded == 1 {
		return backupStatusReadyToUse, nil
	}
	return backupStatusProcessing, nil
}

// getBackupPolicyFromTemplate gets backup policy from template policy template.
func getBackupPolicyFromTemplate(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentDef, backupPolicyTemplateName string) (*dataprotectionv1alpha1.BackupPolicy, error) {
	backupPolicyList := &dataprotectionv1alpha1.BackupPolicyList{}
	if err := cli.List(reqCtx.Ctx, backupPolicyList,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			constant.AppInstanceLabelKey:          cluster.Name,
			constant.KBAppComponentDefRefLabelKey: componentDef,
		}); err != nil {
		return nil, err
	}
	for _, backupPolicy := range backupPolicyList.Items {
		if backupPolicy.Annotations[constant.BackupPolicyTemplateAnnotationKey] == backupPolicyTemplateName {
			return &backupPolicy, nil
		}
	}
	return nil, nil
}
