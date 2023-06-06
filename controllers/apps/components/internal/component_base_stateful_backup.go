package internal

import (
	"fmt"

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
	CloneData() ([]client.Object, error)
}

func NewDataClone(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	stsObj *appsv1.StatefulSet,
	stsProto *appsv1.StatefulSet,
	key types.NamespacedName) DataClone {
	if component == nil || component.HorizontalScalePolicy == nil {
		return &baseDataClone{}
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
	return &baseDataClone{}
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

func (d *baseDataClone) Succeed() (bool, error) {
	return true, nil
}

func (d *baseDataClone) CloneData() ([]client.Object, error) {
	return nil, nil
}

func (d *baseDataClone) checkAllPVCsExist() (bool, error) {
	for i := *d.stsObj.Spec.Replicas; i < d.component.Replicas; i++ {
		for _, vct := range d.stsObj.Spec.VolumeClaimTemplates {
			pvcKey := types.NamespacedName{
				Namespace: d.stsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-%d", vct.Name, d.stsObj.Name, i),
			}
			// check pvc existence
			pvcExists, err := isPVCExists(d.cli, d.reqCtx.Ctx, pvcKey)
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

var _ DataClone = &baseDataClone{}

type snapshotDataClone struct {
	baseDataClone
}

var _ DataClone = &snapshotDataClone{}

func (d *snapshotDataClone) Succeed() (bool, error) {
	return d.checkAllPVCsExist()
}

func (d *snapshotDataClone) CloneData() ([]client.Object, error) {

	objs := make([]client.Object, 0)
	if !isSnapshotAvailable(d.cli, d.reqCtx.Ctx) {
		// TODO: add ut
		return nil, fmt.Errorf("HorizontalScaleFailed: volume snapshot not supported")
	}
	vcts := d.component.VolumeClaimTemplates
	if len(vcts) == 0 {
		d.reqCtx.Recorder.Eventf(d.cluster,
			corev1.EventTypeNormal,
			"HorizontalScale",
			"no VolumeClaimTemplates, no need to do data clone.")
		return objs, nil
	}
	vsExists, err := isVolumeSnapshotExists(d.cli, d.reqCtx.Ctx, d.cluster, d.component)
	if err != nil {
		return nil, err
	}
	// if volumesnapshot not exist, do snapshot to create it.
	if !vsExists {
		if snapshots, err := doSnapshot(d.cli,
			d.reqCtx,
			d.cluster,
			d.component,
			d.key,
			d.stsObj,
			vcts,
			d.component.ComponentDef,
			d.component.HorizontalScalePolicy.BackupPolicyTemplateName); err != nil {
			return nil, err
		} else {
			objs = append(objs, snapshots...)
		}
	}
	// volumesnapshot exists, check if it is ready for use.
	ready, err := isVolumeSnapshotReadyToUse(d.cli, d.reqCtx.Ctx, d.cluster, d.component)
	if err != nil {
		return nil, err
	}
	// volumesnapshot not ready, wait till it is ready after reconciling.
	if !ready {
		return objs, nil
	}
	// if volumesnapshot ready,
	// create pvc from snapshot for every new pod
	pvcKeys := d.toCreatePVCKeys()
	vct := d.backupVCT()
	for _, pvcKey := range pvcKeys {
		if pvc, err := checkedCreatePVCFromSnapshot(d.cli,
			d.reqCtx.Ctx,
			pvcKey,
			d.cluster,
			d.component,
			vct); err != nil {
			d.reqCtx.Log.Error(err, "checkedCreatePVCFromSnapshot failed")
			return nil, err
		} else if pvc != nil {
			objs = append(objs, pvc)
		}
	}
	return objs, nil
}

type backupDataClone struct {
	baseDataClone
}

var _ DataClone = &backupDataClone{}

func (d *backupDataClone) Succeed() (bool, error) {
	allPVCsExist, err := d.checkAllPVCsExist()
	if err != nil || allPVCsExist == false {
		return allPVCsExist, err
	}
	pvcKeys := d.toCreatePVCKeys()
	for _, pvcKey := range pvcKeys {
		restoreJobKey := types.NamespacedName{
			Namespace: pvcKey.Namespace,
			Name:      "restore-" + pvcKey.Name,
		}
		restoreStatus, err := CheckRestoreStatus(d.reqCtx, d.cli, restoreJobKey)
		if err != nil {
			return false, err
		}
		if restoreStatus != RestoreStatusReadyToUse {
			return false, nil
		}
	}
	return true, nil
}

func (d *backupDataClone) CloneData() ([]client.Object, error) {

	objs := make([]client.Object, 0)

	// check backup ready
	backupStatus, err := CheckBackupStatus(d.reqCtx, d.cli, d.key)
	if err != nil {
		return nil, err
	}
	switch backupStatus {
	case BackupStatusNotCreated:
		// create backup
		backupObjs, err := Backup(d.reqCtx,
			d.cli,
			d.key,
			d.cluster,
			d.component)
		if err != nil {
			return nil, err
		}
		objs = append(objs, backupObjs...)
		return objs, nil
	case BackupStatusProcessing:
		// requeue to waiting for backup ready
		return objs, nil
	case BackupStatusReadyToUse:
		break
	}
	// backup's ready, then start to check restore
	pvcKeys := d.toCreatePVCKeys()
	for _, pvcKey := range pvcKeys {
		restoreJobKey := types.NamespacedName{
			Namespace: pvcKey.Namespace,
			Name:      "restore-" + pvcKey.Name,
		}
		restoreStatus, err := CheckRestoreStatus(d.reqCtx, d.cli, restoreJobKey)
		if err != nil {
			return nil, err
		}
		switch restoreStatus {
		case RestoreStatusNotCreated:
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
			restoreObjs, err := Restore(d.cluster, d.component, restoreJobKey, &backup, &backupToolList.Items[0], pvcKey)
			if err != nil {
				return nil, err
			}
			objs = append(objs, restoreObjs...)
		case RestoreStatusProcessing:
		case RestoreStatusReadyToUse:
			break
		}
	}
	// restore to pvcs all ready
	return objs, nil
}

type BackupStatus string

const (
	BackupStatusNotCreated BackupStatus = "NotCreated"
	BackupStatusProcessing BackupStatus = "Processing"
	BackupStatusReadyToUse BackupStatus = "ReadyToUse"
)

type RestoreStatus string

const (
	RestoreStatusNotCreated RestoreStatus = "NotCreated"
	RestoreStatusProcessing RestoreStatus = "Processing"
	RestoreStatusReadyToUse RestoreStatus = "ReadyToUse"
)

func Backup(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	backupKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) ([]client.Object, error) {
	objs := make([]client.Object, 0)
	backupPolicyTplName := component.HorizontalScalePolicy.BackupPolicyTemplateName
	backupPolicy, err := GetBackupPolicyFromTemplate(reqCtx, cli, cluster, component.ComponentDef, backupPolicyTplName)
	if err != nil {
		return nil, err
	}
	if backupPolicy == nil {
		return nil, intctrlutil.NewNotFound("not found any backup policy created by %s", backupPolicyTplName)
	}
	backup, err := builder.BuildBackup(cluster, component, backupPolicy.Name, backupKey, "datafile")
	if err != nil {
		return nil, err
	}
	objs = append(objs, backup)
	return objs, nil
}

func CheckBackupStatus(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	backupKey types.NamespacedName) (BackupStatus, error) {
	backup := dataprotectionv1alpha1.Backup{}
	if err := cli.Get(reqCtx.Ctx, backupKey, &backup); err != nil {
		if errors.IsNotFound(err) {
			return BackupStatusNotCreated, nil
		} else {
			return BackupStatusNotCreated, err
		}
	}
	if backup.Status.Phase == dataprotectionv1alpha1.BackupCompleted {
		return BackupStatusReadyToUse, nil
	}
	return BackupStatusProcessing, nil
}

func Restore(cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	restoreJobKey types.NamespacedName,
	backup *dataprotectionv1alpha1.Backup,
	backupTool *dataprotectionv1alpha1.BackupTool,
	pvcKey types.NamespacedName) ([]client.Object, error) {
	objs := make([]client.Object, 0)
	// TODO: CT - need to check parameters
	pvc, err := builder.BuildPVC(cluster, component, &component.VolumeClaimTemplates[0], pvcKey, "")
	if err != nil {
		return nil, err
	}
	objs = append(objs, pvc)
	job, err := builder.BuildRestoreJobForFullBackup(restoreJobKey.Name, component, backup, backupTool, pvcKey.Name)
	if err != nil {
		return nil, err
	}
	objs = append(objs, job)
	return objs, nil
}

func CheckRestoreStatus(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	restoreJobKey types.NamespacedName) (RestoreStatus, error) {
	job := v1.Job{}
	if err := cli.Get(reqCtx.Ctx, restoreJobKey, &job); err != nil {
		if errors.IsNotFound(err) {
			return RestoreStatusNotCreated, nil
		} else {
			return RestoreStatusNotCreated, err
		}
	}
	if job.Status.Succeeded == 1 {
		return RestoreStatusReadyToUse, nil
	}
	return RestoreStatusProcessing, nil
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
