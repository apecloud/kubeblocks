package internal

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/batch/v1"
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
	sts *appsv1.StatefulSet,
	cluster *appsv1alpha1.Cluster,
	componentDefName string,
	backupPolicyTplName string) ([]client.Object, error) {
	objs := make([]client.Object, 0)
	backupPolicy, err := GetBackupPolicyFromTemplate(reqCtx, cli, cluster, componentDefName, backupPolicyTplName)
	if err != nil {
		return nil, err
	}
	if backupPolicy == nil {
		return nil, intctrlutil.NewNotFound("not found any backup policy created by %s", backupPolicyTplName)
	}
	backup, err := builder.BuildBackup(sts, backupPolicy.Name, backupKey, "datafile")
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

func Restore(restoreJobKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster,
	sts *appsv1.StatefulSet,
	component *component.SynthesizedComponent,
	backup *dataprotectionv1alpha1.Backup,
	backupTool *dataprotectionv1alpha1.BackupTool,
	pvcKey types.NamespacedName) ([]client.Object, error) {
	objs := make([]client.Object, 0)
	// TODO: CT - need to check parameters
	pvc, err := builder.BuildPVCFromSnapshot(sts, component.VolumeClaimTemplates[0], pvcKey, "", component)
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
