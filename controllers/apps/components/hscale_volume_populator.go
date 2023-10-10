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
	"context"
	"fmt"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/factory"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type dataClone interface {
	// Succeed check if data clone succeeded
	Succeed() (bool, error)
	// CloneData do clone data, return objects that need to be created
	CloneData(dataClone) ([]client.Object, error)
	// ClearTmpResources clear all the temporary resources created during data clone, return objects that need to be deleted
	ClearTmpResources() ([]client.Object, error)

	checkBackupStatus() (backupStatus, error)
	backup() ([]client.Object, error)
	pvcKeysToRestore() []types.NamespacedName
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

func NewDataClone(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	stsObj *appsv1.StatefulSet,
	stsProto *appsv1.StatefulSet,
	key types.NamespacedName) (dataClone, error) {
	if component == nil {
		return nil, nil
	}
	if component.HorizontalScalePolicy == nil {
		return &dummyDataClone{
			baseDataClone{
				reqCtx:    reqCtx,
				cli:       cli,
				cluster:   cluster,
				component: component,
				stsObj:    stsObj,
				stsProto:  stsProto,
				key:       key,
			},
		}, nil
	}
	if component.HorizontalScalePolicy.Type == appsv1alpha1.HScaleDataClonePolicyCloneVolume {
		volumeSnapshotEnabled, err := isVolumeSnapshotEnabled(reqCtx.Ctx, cli, stsObj, backupVCT(component))
		if err != nil {
			return nil, err
		}
		if volumeSnapshotEnabled {
			return &snapshotDataClone{
				baseDataClone{
					reqCtx:    reqCtx,
					cli:       cli,
					cluster:   cluster,
					component: component,
					stsObj:    stsObj,
					stsProto:  stsProto,
					key:       key,
				},
			}, nil
		}
		return &backupDataClone{
			baseDataClone{
				reqCtx:    reqCtx,
				cli:       cli,
				cluster:   cluster,
				component: component,
				stsObj:    stsObj,
				stsProto:  stsProto,
				key:       key,
			},
		}, nil
	}
	// TODO: how about policy None and Snapshot?
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

func (d *baseDataClone) CloneData(realDataClone dataClone) ([]client.Object, error) {
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
	default:
		panic(fmt.Sprintf("unexpected backup status: %s, clustre: %s, component: %s",
			status, d.cluster.Name, d.component.Name))
	}

	// backup's ready, then start to check restore
	for _, pvcKey := range d.pvcKeysToRestore() {
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
		default:
			panic(fmt.Sprintf("unexpected restore status: %s, clustre: %s, component: %s",
				status, d.cluster.Name, d.component.Name))
		}
	}

	// create PVCs that do not need to restore
	pvcObjs, err := d.createPVCs(d.excludeBackupVCTs())
	if err != nil {
		return nil, err
	}
	objs = append(objs, pvcObjs...)

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
		for _, vct := range d.component.VolumeClaimTemplates {
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

func (d *baseDataClone) allVCTs() []*corev1.PersistentVolumeClaimTemplate {
	vcts := make([]*corev1.PersistentVolumeClaimTemplate, 0)
	for i := range d.component.VolumeClaimTemplates {
		vcts = append(vcts, &d.component.VolumeClaimTemplates[i])
	}
	return vcts
}

func (d *baseDataClone) backupVCT() *corev1.PersistentVolumeClaimTemplate {
	return backupVCT(d.component)
}

func (d *baseDataClone) excludeBackupVCTs() []*corev1.PersistentVolumeClaimTemplate {
	vcts := make([]*corev1.PersistentVolumeClaimTemplate, 0)
	backupVCT := d.backupVCT()
	for i := range d.component.VolumeClaimTemplates {
		vct := &d.component.VolumeClaimTemplates[i]
		if vct.Name != backupVCT.Name {
			vcts = append(vcts, vct)
		}
	}
	return vcts
}

func (d *baseDataClone) pvcKeysToRestore() []types.NamespacedName {
	var pvcKeys []types.NamespacedName
	backupVct := d.backupVCT()
	for i := *d.stsObj.Spec.Replicas; i < d.component.Replicas; i++ {
		pvcKey := types.NamespacedName{
			Namespace: d.stsObj.Namespace,
			Name:      fmt.Sprintf("%s-%s-%d", backupVct.Name, d.stsObj.Name, i),
		}
		pvcKeys = append(pvcKeys, pvcKey)
	}
	return pvcKeys
}

func (d *baseDataClone) createPVCs(vcts []*corev1.PersistentVolumeClaimTemplate) ([]client.Object, error) {
	objs := make([]client.Object, 0)
	for i := *d.stsObj.Spec.Replicas; i < d.component.Replicas; i++ {
		for _, vct := range vcts {
			pvcKey := types.NamespacedName{
				Namespace: d.stsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-%d", vct.Name, d.stsObj.Name, i),
			}
			if exist, err := d.isPVCExists(pvcKey); err != nil {
				return nil, err
			} else if exist {
				continue
			}
			pvc := factory.BuildPVC(d.cluster, d.component, vct, pvcKey, "")
			objs = append(objs, pvc)
		}
	}
	return objs, nil
}

func (d *baseDataClone) getBackupMatchingLabels() client.MatchingLabels {
	return client.MatchingLabels{
		constant.AppInstanceLabelKey:    d.cluster.Name,
		constant.KBAppComponentLabelKey: d.component.Name,
		constant.KBManagedByKey:         "cluster", // the resources are managed by which controller
	}
}

type dummyDataClone struct {
	baseDataClone
}

var _ dataClone = &dummyDataClone{}

func (d *dummyDataClone) Succeed() (bool, error) {
	return d.checkAllPVCsExist()
}

func (d *dummyDataClone) CloneData(dataClone) ([]client.Object, error) {
	return d.createPVCs(d.allVCTs())
}

func (d *dummyDataClone) ClearTmpResources() ([]client.Object, error) {
	return nil, nil
}

func (d *dummyDataClone) checkBackupStatus() (backupStatus, error) {
	return backupStatusReadyToUse, nil
}

func (d *dummyDataClone) backup() ([]client.Object, error) {
	panic("runtime error: dummyDataClone.backup called")
}

func (d *dummyDataClone) checkRestoreStatus(types.NamespacedName) (backupStatus, error) {
	return backupStatusReadyToUse, nil
}

func (d *dummyDataClone) restore(name types.NamespacedName) ([]client.Object, error) {
	panic("runtime error: dummyDataClone.restore called")
}

type snapshotDataClone struct {
	baseDataClone
}

var _ dataClone = &snapshotDataClone{}

func (d *snapshotDataClone) Succeed() (bool, error) {
	if len(d.component.VolumeClaimTemplates) == 0 {
		d.reqCtx.Recorder.Eventf(d.cluster,
			corev1.EventTypeNormal,
			"HorizontalScale",
			"no VolumeClaimTemplates, no need to do data clone.")
		return true, nil
	}
	return d.checkAllPVCsExist()
}

func (d *snapshotDataClone) ClearTmpResources() ([]client.Object, error) {
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
	backupPolicy, err := getBackupPolicyFromTemplate(d.reqCtx, d.cli, d.cluster, d.component.ClusterCompDefName, backupPolicyTplName)
	if err != nil {
		return nil, err
	}
	if backupPolicy == nil {
		return nil, intctrlutil.NewNotFound("not found any backup policy created by %s", backupPolicyTplName)
	}
	backup := factory.BuildBackup(d.cluster, d.component, backupPolicy.Name, d.key, "snapshot")
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
	if err := d.cli.List(d.reqCtx.Ctx, &backupList, client.InNamespace(d.cluster.Namespace), ml); err != nil {
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
			return nil, fmt.Errorf("volumesnapshot not found for cluster %s component %s", d.cluster.Name, d.component.Name)
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
	pvc := factory.BuildPVC(d.cluster, d.component, vct, pvcKey, snapshotName)
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
	if err := d.cli.List(d.reqCtx.Ctx, &backupList, client.InNamespace(d.cluster.Namespace), ml); err != nil {
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

var _ dataClone = &backupDataClone{}

func (d *backupDataClone) Succeed() (bool, error) {
	if len(d.component.VolumeClaimTemplates) == 0 {
		d.reqCtx.Recorder.Eventf(d.cluster,
			corev1.EventTypeNormal,
			"HorizontalScale",
			"no VolumeClaimTemplates, no need to do data clone.")
		return true, nil
	}
	allPVCsExist, err := d.checkAllPVCsExist()
	if err != nil || !allPVCsExist {
		return allPVCsExist, err
	}
	for _, pvcKey := range d.pvcKeysToRestore() {
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

func (d *backupDataClone) ClearTmpResources() ([]client.Object, error) {
	objs := make([]client.Object, 0)
	// delete backup
	ml := d.getBackupMatchingLabels()
	backupList := dataprotectionv1alpha1.BackupList{}
	if err := d.cli.List(d.reqCtx.Ctx, &backupList, client.InNamespace(d.cluster.Namespace), ml); err != nil {
		return nil, err
	}
	for i := range backupList.Items {
		objs = append(objs, &backupList.Items[i])
	}
	// delete restore job
	jobList := v1.JobList{}
	if err := d.cli.List(d.reqCtx.Ctx, &jobList, client.InNamespace(d.cluster.Namespace), ml); err != nil {
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
	backupPolicy, err := getBackupPolicyFromTemplate(d.reqCtx, d.cli, d.cluster, d.component.ClusterCompDefName, backupPolicyTplName)
	if err != nil {
		return nil, err
	}
	if backupPolicy == nil {
		return nil, intctrlutil.NewNotFound("not found any backup policy created by %s", backupPolicyTplName)
	}
	backup := factory.BuildBackup(d.cluster, d.component, backupPolicy.Name, d.key, "datafile")
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
	pvc := factory.BuildPVC(d.cluster, d.component, d.backupVCT(), pvcKey, "")
	objs = append(objs, pvc)
	backupTool := &dataprotectionv1alpha1.BackupTool{}
	if err := d.cli.Get(d.reqCtx.Ctx, client.ObjectKey{Name: backup.Status.BackupToolName}, backupTool); err != nil {
		return nil, err
	}
	restoreMgr := plan.NewRestoreManager(d.reqCtx.Ctx, d.cli, d.cluster, nil)
	restoreJobs, err := restoreMgr.BuildDatafileRestoreJobByPVCS(d.baseDataClone.component, &backup, backupTool, []string{pvc.Name}, d.getBackupMatchingLabels())
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

func backupVCT(component *component.SynthesizedComponent) *corev1.PersistentVolumeClaimTemplate {
	if len(component.VolumeClaimTemplates) == 0 {
		return nil
	}
	vct := component.VolumeClaimTemplates[0]
	for _, tmpVct := range component.VolumeClaimTemplates {
		for _, volumeType := range component.VolumeTypes {
			if volumeType.Type == appsv1alpha1.VolumeTypeData && volumeType.Name == tmpVct.Name {
				vct = tmpVct
				break
			}
		}
	}
	return &vct
}

func isVolumeSnapshotEnabled(ctx context.Context, cli client.Client,
	sts *appsv1.StatefulSet, vct *corev1.PersistentVolumeClaimTemplate) (bool, error) {
	if sts == nil || vct == nil {
		return false, nil
	}
	pvcKey := types.NamespacedName{
		Namespace: sts.Namespace,
		Name:      fmt.Sprintf("%s-%s-%d", vct.Name, sts.Name, 0),
	}
	pvc := corev1.PersistentVolumeClaim{}
	if err := cli.Get(ctx, pvcKey, &pvc); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	if pvc.Spec.StorageClassName == nil {
		return false, nil
	}

	storageClass := storagev1.StorageClass{}
	if err := cli.Get(ctx, types.NamespacedName{Name: *pvc.Spec.StorageClassName}, &storageClass); err != nil {
		return false, client.IgnoreNotFound(err)
	}

	vscList := snapshotv1.VolumeSnapshotClassList{}
	if err := cli.List(ctx, &vscList); err != nil {
		return false, err
	}
	for _, vsc := range vscList.Items {
		if vsc.Driver == storageClass.Provisioner {
			return true, nil
		}
	}
	return false, nil
}
