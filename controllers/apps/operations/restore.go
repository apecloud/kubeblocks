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

package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
)

type RestoreOpsHandler struct{}

var _ OpsHandler = RestoreOpsHandler{}

func init() {
	restoreBehaviour := OpsBehaviour{
		FromClusterPhases:                  appsv1alpha1.GetClusterUpRunningPhases(),
		OpsHandler:                         RestoreOpsHandler{},
		ProcessingReasonInClusterCondition: ProcessingReasonRestore,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.RestoreType, restoreBehaviour)
}

// ActionStartedCondition the started condition when handling the restore request.
func (r RestoreOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewRestoreCondition(opsRes.OpsRequest), nil
}

// Action implements the restore action.
func (r RestoreOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	clusterDef := opsRes.OpsRequest.Spec.ClusterRef

	// restore the cluster from the backup
	if cluster, err := restoreClusterFromBackup(reqCtx, cli, opsRequest, clusterDef); err != nil {
		return err
	} else {
		if err := cli.Create(reqCtx.Ctx, cluster); err != nil {
			return err
		}

		// add labels of clusterRef
		patch := client.MergeFrom(opsRequest.DeepCopy())
		if opsRequest.Labels == nil {
			opsRequest.Labels = make(map[string]string)
		}
		opsRequest.Labels[constant.AppInstanceLabelKey] = opsRequest.Spec.ClusterRef
		opsRequest.Labels[constant.OpsRequestTypeLabelKey] = string(opsRequest.Spec.Type)
		scheme, _ := appsv1alpha1.SchemeBuilder.Build()
		if err := controllerutil.SetOwnerReference(cluster, opsRequest, scheme); err != nil {
			return err
		}
		if err := cli.Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return err
		}
	}
	return nil
}

// ReconcileAction implements the restore action.
// It will check the cluster status and update the OpsRequest status.
// If the cluster is running, it will update the OpsRequest status to Complete.
// If the cluster is failed, it will update the OpsRequest status to Failed.
// If the cluster is not running, it will update the OpsRequest status to Running.
func (r RestoreOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	opsRequest := opsRes.OpsRequest
	clusterDef := opsRequest.Spec.ClusterRef

	// get cluster
	cluster := &appsv1alpha1.Cluster{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: opsRequest.GetNamespace(),
		Name:      clusterDef,
	}, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			_ = PatchClusterNotFound(reqCtx.Ctx, cli, opsRes)
		}
		return appsv1alpha1.OpsFailedPhase, 0, err
	}

	// check if the cluster is running
	if cluster.Status.Phase == appsv1alpha1.RunningClusterPhase {
		return appsv1alpha1.OpsSucceedPhase, 0, nil
	} else if cluster.Status.Phase == appsv1alpha1.FailedClusterPhase {
		return appsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("restore failed")
	}
	return appsv1alpha1.OpsRunningPhase, 0, nil
}

// SaveLastConfiguration saves last configuration to the OpsRequest.status.lastConfiguration
func (r RestoreOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error {
	return nil
}

func restoreClusterFromBackup(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRequest *appsv1alpha1.OpsRequest, clusterDef string) (*appsv1alpha1.Cluster, error) {
	backupName := opsRequest.Spec.RestoreSpec.BackupName
	restoreTimeStr := opsRequest.Spec.RestoreSpec.RestoreTimeStr
	volumeRestorePolicy := opsRequest.Spec.RestoreSpec.VolumeRestorePolicy

	// check if the backup exists
	backup := &dpv1alpha1.Backup{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{
		Name:      backupName,
		Namespace: opsRequest.Namespace,
	}, backup); err != nil {
		return nil, err
	}

	// check if the backup whether is completed
	if backup.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
		return nil, fmt.Errorf("backup %s is not completed", backupName)
	}
	if len(backup.Labels[constant.AppInstanceLabelKey]) == 0 {
		return nil, fmt.Errorf(`missing source cluster in backup "%s", "app.kubernetes.io/instance" is empty in labels`, backupName)
	}

	restoreTimeStr, err := restore.FormatRestoreTimeAndValidate(opsRequest.Spec.RestoreSpec.RestoreTimeStr, backup)
	if err != nil {
		return nil, err
	}

	// get the cluster object from backup
	clusterObj, err := getClusterObjFromBackup(cli, backup)
	if err != nil {
		return nil, err
	}
	restoreAnnotation, err := restore.GetRestoreFromBackupAnnotation(backup, volumeRestorePolicy, len(clusterObj.Spec.ComponentSpecs), clusterObj.Spec.ComponentSpecs[0].Name, restoreTimeStr)
	if err != nil {
		return nil, err
	}
	clusterObj.ObjectMeta = metav1.ObjectMeta{
		Name:        clusterDef,
		Namespace:   clusterObj.Namespace,
		Annotations: map[string]string{constant.RestoreFromBackupAnnotationKey: restoreAnnotation},
	}
	clusterObj.TypeMeta = metav1.TypeMeta{
		Kind:       types.KindCluster,
		APIVersion: types.ClusterGVR().GroupVersion().String(),
	}
	opsRequestSlice := []appsv1alpha1.OpsRecorder{
		{
			Name: opsRequest.Name,
			Type: opsRequest.Spec.Type,
		},
	}
	util.SetOpsRequestToCluster(clusterObj, opsRequestSlice)
	return clusterObj, nil
}

func getClusterObjFromBackup(cli client.Client, backup *dpv1alpha1.Backup) (*appsv1alpha1.Cluster, error) {
	// use the cluster snapshot to restore firstly
	clusterString, ok := backup.Annotations[constant.ClusterSnapshotAnnotationKey]
	if ok {
		clusterObj := &appsv1alpha1.Cluster{}
		if err := json.Unmarshal([]byte(clusterString), &clusterObj); err != nil {
			return nil, err
		}
		return clusterObj, nil
	}
	clusterName := backup.Labels[constant.AppInstanceLabelKey]
	cluster := &appsv1alpha1.Cluster{}
	if err := cli.Get(context.Background(), client.ObjectKey{
		Namespace: backup.Namespace,
		Name:      clusterName,
	}, cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}
