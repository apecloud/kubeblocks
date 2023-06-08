package internal

import (
	"fmt"
	"strings"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
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
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type DataClone interface {
	Succeed() (bool, error)
	CloneData(DataClone) ([]client.Object, error)
	ClearTmpResources() ([]client.Object, error)
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

func NewDataClone(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	stsObj *appsv1.StatefulSet,
	stsProto *appsv1.StatefulSet,
	key types.NamespacedName) DataClone {
	if component == nil || component.HorizontalScalePolicy == nil {
		return nil
	}
	switch component.HorizontalScalePolicy.Type {
	case appsv1alpha1.HScaleDataClonePolicyFromSnapshot:
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
		}
	case appsv1alpha1.HScaleDataClonePolicyFromBackup:
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
		}
	}
	return nil
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

func (d *baseDataClone) CloneData(realDataClone DataClone) ([]client.Object, error) {

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
		if tmpVct.Name == d.component.HorizontalScalePolicy.VolumeMountsName {
			vct = tmpVct
			break
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

var _ DataClone = &snapshotDataClone{}

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
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	// no backuppolicytemplate, then try native volumesnapshot
	if err != nil {
		pvcName := strings.Join([]string{d.backupVCT().Name, d.stsObj.Name, "0"}, "-")
		snapshot, err := builder.BuildVolumeSnapshot(d.key, pvcName, d.stsObj)
		if err != nil {
			return nil, err
		}
		d.reqCtx.Eventf(d.cluster, corev1.EventTypeNormal, "VolumeSnapshotCreate", "Create volumesnapshot/%s", d.key.Name)
		return []client.Object{snapshot}, nil
	}

	// if there is backuppolicytemplate created by provider
	backupPolicy, err := GetBackupPolicyFromTemplate(d.reqCtx, d.cli, d.cluster, d.component.ComponentDef, backupPolicyTplName)
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
	if !isSnapshotAvailable(d.cli, d.reqCtx.Ctx) {
		return backupStatusFailed, fmt.Errorf("HorizontalScaleFailed: volume snapshot not supported")
	}
	hasBackupPolicyTemplate := true
	backupPolicyTplName := d.component.HorizontalScalePolicy.BackupPolicyTemplateName
	backupPolicyTemplate := &appsv1alpha1.BackupPolicyTemplate{}
	err := d.cli.Get(d.reqCtx.Ctx, client.ObjectKey{Name: backupPolicyTplName}, backupPolicyTemplate)
	if err != nil && !errors.IsNotFound(err) {
		return backupStatusFailed, err
	}
	if errors.IsNotFound(err) {
		hasBackupPolicyTemplate = false
	}
	// if no backuppolicytemplate, do not check backup
	if hasBackupPolicyTemplate {
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
	}
	vsExists, err := d.isVolumeSnapshotExists()
	if err != nil {
		return backupStatusFailed, err
	}
	if !vsExists {
		if hasBackupPolicyTemplate {
			return backupStatusProcessing, nil
		} else {
			return backupStatusNotCreated, nil
		}
	}
	// volumesnapshot exists, check if it is ready for use.
	ready, err := d.isVolumeSnapshotReadyToUse()
	if err != nil {
		return backupStatusFailed, err
	}
	if !ready {
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

// check snapshot existence
func (d *snapshotDataClone) isVolumeSnapshotExists() (bool, error) {
	ml := d.getBackupMatchingLabels()
	vsList := snapshotv1.VolumeSnapshotList{}
	compatClient := intctrlutil.VolumeSnapshotCompatClient{ReadonlyClient: d.cli, Ctx: d.reqCtx.Ctx}
	if err := compatClient.List(&vsList, ml); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	for _, vs := range vsList.Items {
		// when do h-scale very shortly after last h-scale,
		// the last volume snapshot could not be deleted completely
		if vs.DeletionTimestamp.IsZero() {
			return true, nil
		}
	}
	return false, nil
}

// check snapshot ready to use
func (d *snapshotDataClone) isVolumeSnapshotReadyToUse() (bool, error) {
	ml := d.getBackupMatchingLabels()
	vsList := snapshotv1.VolumeSnapshotList{}
	compatClient := intctrlutil.VolumeSnapshotCompatClient{ReadonlyClient: d.cli, Ctx: d.reqCtx.Ctx}
	if err := compatClient.List(&vsList, ml); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	if len(vsList.Items) == 0 || vsList.Items[0].Status == nil {
		return false, nil
	}
	status := vsList.Items[0].Status
	if status.Error != nil {
		return false, fmt.Errorf("VolumeSnapshot/" + vsList.Items[0].Name + ": " + *status.Error.Message)
	}
	if status.ReadyToUse == nil {
		return false, nil
	}
	return *status.ReadyToUse, nil
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
		compatClient := intctrlutil.VolumeSnapshotCompatClient{ReadonlyClient: d.cli, Ctx: d.reqCtx.Ctx}
		if err := compatClient.List(&vsList, ml); err != nil {
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
	// delete volumesnapshot separately since backup may not exist if backuppolicytemplate not configured
	compatClient := intctrlutil.VolumeSnapshotCompatClient{ReadonlyClient: d.cli, Ctx: d.reqCtx.Ctx}
	vs := &snapshotv1.VolumeSnapshot{}
	err = compatClient.Get(d.key, vs)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if err == nil {
		objs = append(objs, vs)
		d.reqCtx.Recorder.Eventf(d.cluster, corev1.EventTypeNormal, "VolumeSnapshotDelete", "Delete volumeSnapshot/%s", d.key.Name)
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

var _ DataClone = &backupDataClone{}

func (d *backupDataClone) Succeed() (bool, error) {
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

func (d *backupDataClone) ClearTmpResources() ([]client.Object, error) {
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
	backupPolicy, err := GetBackupPolicyFromTemplate(d.reqCtx, d.cli, d.cluster, d.component.ComponentDef, backupPolicyTplName)
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
	restoreJobKey := d.restoreKeyFromPVCKey(pvcKey)
	backup := dataprotectionv1alpha1.Backup{}
	if err := d.cli.Get(d.reqCtx.Ctx, d.key, &backup); err != nil {
		return nil, err
	}
	ml := client.MatchingLabels{
		constant.ClusterDefLabelKey: d.component.ClusterDefName,
	}
	backupToolList := dataprotectionv1alpha1.BackupToolList{}
	if err := d.cli.List(d.reqCtx.Ctx, &backupToolList, ml); err != nil {
		return nil, err
	}
	if len(backupToolList.Items) == 0 {
		return nil, fmt.Errorf("backuptool not found for clusterdefinition: %s", d.component.ClusterDefName)
	}
	pvc, err := builder.BuildPVC(d.cluster, d.component, &d.component.VolumeClaimTemplates[0], pvcKey, "")
	if err != nil {
		return nil, err
	}
	objs = append(objs, pvc)
	// TODO: @dengshao refactor it to backup api
	job, err := builder.BuildRestoreJobForFullBackup(restoreJobKey.Name, d.component, &backup, &backupToolList.Items[0], pvcKey.Name)
	if err != nil {
		return nil, err
	}
	objs = append(objs, job)
	return objs, nil
}

func (d *backupDataClone) checkRestoreStatus(pvcKey types.NamespacedName) (backupStatus, error) {
	job := v1.Job{}
	if err := d.cli.Get(d.reqCtx.Ctx, d.restoreKeyFromPVCKey(pvcKey), &job); err != nil {
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

func (d *backupDataClone) restoreKeyFromPVCKey(pvcKey types.NamespacedName) types.NamespacedName {
	restoreJobKey := types.NamespacedName{
		Namespace: pvcKey.Namespace,
		Name:      "restore-" + pvcKey.Name,
	}
	return restoreJobKey
}

// GetBackupPolicyFromTemplate gets backup policy from template policy template.
func GetBackupPolicyFromTemplate(reqCtx intctrlutil.RequestCtx,
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
