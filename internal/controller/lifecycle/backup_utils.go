package lifecycle

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
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
	cli roclient.ReadonlyClient,
	backupKey types.NamespacedName,
	sts *appsv1.StatefulSet,
	cluster *appsv1alpha1.Cluster,
	componentDefName string,
	backupPolicyTplName string,
	dag *graph.DAG,
	root graph.Vertex) error {
	backupPolicy, err := getBackupPolicyFromTemplate(reqCtx, cli, cluster, componentDefName, backupPolicyTplName)
	if err != nil {
		return err
	}
	if backupPolicy == nil {
		return intctrlutil.NewNotFound("not found any backup policy created by %s", backupPolicyTplName)
	}
	backup, err := builder.BuildBackup(sts, backupPolicy.Name, backupKey, "datafile")
	if err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(cluster, backup, scheme); err != nil {
		return err
	}
	vertex := &lifecycleVertex{obj: backup, action: actionPtr(CREATE)}
	dag.AddVertex(vertex)
	dag.Connect(root, vertex)
	return nil
}

func CheckBackupStatus(reqCtx intctrlutil.RequestCtx,
	cli roclient.ReadonlyClient,
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
	component *component.SynthesizedComponent,
	backup *dataprotectionv1alpha1.Backup,
	backupTool *dataprotectionv1alpha1.BackupTool,
	pvcName string,
	dag *graph.DAG,
	root graph.Vertex) error {
	job, err := builder.BuildRestoreJobForFullBackup(restoreJobKey.Name, component, backup, backupTool, pvcName)
	if err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(cluster, backup, scheme); err != nil {
		return err
	}
	vertex := &lifecycleVertex{obj: job, action: actionPtr(CREATE)}
	dag.AddVertex(vertex)
	dag.Connect(root, vertex)
	return nil
}

func CheckRestoreStatus(reqCtx intctrlutil.RequestCtx,
	cli roclient.ReadonlyClient,
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
