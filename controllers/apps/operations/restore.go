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
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

type RestoreOpsHandler struct{}

var _ OpsHandler = RestoreOpsHandler{}

func init() {
	// register restore operation, it will create a new cluster
	// so set IsClusterCreationEnabled to true
	restoreBehaviour := OpsBehaviour{
		FromClusterPhases: appsv1alpha1.GetClusterUpRunningPhases(),
		OpsHandler:        RestoreOpsHandler{},
		IsClusterCreation: true,
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
	var cluster *appsv1alpha1.Cluster
	var err error

	opsRequest := opsRes.OpsRequest

	// restore the cluster from the backup
	if cluster, err = restoreClusterFromBackup(reqCtx, cli, opsRequest); err != nil {
		return err
	}

	// create cluster
	if err = cli.Create(reqCtx.Ctx, cluster); err != nil {
		return err
	}
	opsRes.Cluster = cluster

	// add labels of clusterRef and type to OpsRequest
	// and set owner reference to cluster
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Labels == nil {
		opsRequest.Labels = make(map[string]string)
	}
	opsRequest.Labels[constant.AppInstanceLabelKey] = opsRequest.Spec.ClusterRef
	opsRequest.Labels[constant.OpsRequestTypeLabelKey] = string(opsRequest.Spec.Type)
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	if err = controllerutil.SetOwnerReference(cluster, opsRequest, scheme); err != nil {
		return err
	}
	if err = cli.Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
		return err
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

func restoreClusterFromBackup(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRequest *appsv1alpha1.OpsRequest) (*appsv1alpha1.Cluster, error) {
	backupName := opsRequest.Spec.RestoreSpec.BackupName
	restoreTimeStr := opsRequest.Spec.RestoreSpec.RestoreTimeStr

	// check if the backup exists
	backup := &dpv1alpha1.Backup{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{
		Name:      backupName,
		Namespace: opsRequest.Namespace,
	}, backup); err != nil {
		return nil, err
	}

	// check if the backup is completed
	backupType := backup.Labels[dptypes.BackupTypeLabelKey]
	if backup.Status.Phase != dpv1alpha1.BackupPhaseCompleted && backupType != string(dpv1alpha1.BackupTypeContinuous) {
		return nil, fmt.Errorf("backup %s status is %s, only completed backup can be used to restore", backupName, backup.Status.Phase)
	}

	// format and validate the restore time
	restoreTimeStr, err := restore.FormatRestoreTimeAndValidate(restoreTimeStr, backup)
	if err != nil {
		return nil, err
	}
	opsRequest.Spec.RestoreSpec.RestoreTimeStr = restoreTimeStr

	// get the cluster object from backup
	clusterObj, err := getClusterObjFromBackup(backup, opsRequest)
	if err != nil {
		return nil, err
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

func getClusterObjFromBackup(backup *dpv1alpha1.Backup, opsRequest *appsv1alpha1.OpsRequest) (*appsv1alpha1.Cluster, error) {
	cluster := &appsv1alpha1.Cluster{}
	// use the cluster snapshot to restore firstly
	clusterString, ok := backup.Annotations[constant.ClusterSnapshotAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("missing snapshot annotation in backup %s, %s is empty in Annotations", backup.Name, constant.ClusterSnapshotAnnotationKey)
	}
	if err := json.Unmarshal([]byte(clusterString), &cluster); err != nil {
		return nil, err
	}
	restoreSpec := opsRequest.Spec.RestoreSpec
	// set the restore annotation to cluster
	restoreAnnotation, err := restore.GetRestoreFromBackupAnnotation(backup, cluster.Spec.ComponentSpecs, restoreSpec.VolumeRestorePolicy, restoreSpec.RestoreTimeStr, restoreSpec.EffectiveCommonComponentDef)
	if err != nil {
		return nil, err
	}
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	cluster.Annotations[constant.RestoreFromBackupAnnotationKey] = restoreAnnotation
	cluster.Name = opsRequest.Spec.ClusterRef
	return cluster, nil
}
