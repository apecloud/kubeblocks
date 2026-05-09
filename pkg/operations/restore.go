/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
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
		OpsHandler:        RestoreOpsHandler{},
		IsClusterCreation: true,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.RestoreType, restoreBehaviour)
}

// ActionStartedCondition the started condition when handling the restore request.
func (r RestoreOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewRestoreCondition(opsRes.OpsRequest), nil
}

// Action implements the restore action.
func (r RestoreOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest

	clusterRestore, err := r.buildClusterRestore(reqCtx, cli, opsRequest)
	if err != nil {
		return err
	}
	if err = intctrlutil.SetControllerReference(opsRequest, clusterRestore); err != nil {
		return err
	}

	if err = cli.Create(reqCtx.Ctx, clusterRestore); apierrors.IsAlreadyExists(err) {
		existing := &dpv1alpha1.ClusterRestore{}
		if getErr := cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(clusterRestore), existing); getErr != nil {
			return getErr
		}
		if !isClusterRestoreOwnedByOpsRequest(existing, opsRequest) {
			return intctrlutil.NewFatalError(fmt.Sprintf("cluster restore %s/%s already exists and is not owned by OpsRequest %s/%s", clusterRestore.Namespace, clusterRestore.Name, opsRequest.Namespace, opsRequest.Name))
		}
	} else if err != nil {
		return err
	}

	// add labels of clusterRef and type to OpsRequest
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Labels == nil {
		opsRequest.Labels = make(map[string]string)
	}
	opsRequest.Labels[constant.AppInstanceLabelKey] = opsRequest.Spec.GetClusterName()
	opsRequest.Labels[constant.OpsRequestTypeLabelKey] = string(opsRequest.Spec.Type)
	if err = cli.Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
		return err
	}
	return nil
}

// ReconcileAction implements the restore action.
// It waits for ClusterRestore completion before checking the target Cluster phase.
func (r RestoreOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	opsRequest := opsRes.OpsRequest
	clusterRestore := &dpv1alpha1.ClusterRestore{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: opsRequest.Namespace,
		Name:      clusterRestoreName(opsRequest),
	}, clusterRestore); err != nil {
		if apierrors.IsNotFound(err) {
			return opsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("cluster restore %s not found", clusterRestoreName(opsRequest))
		}
		return opsv1alpha1.OpsFailedPhase, 0, err
	}

	switch clusterRestore.Status.Phase {
	case dpv1alpha1.ClusterRestorePhaseFailed:
		return opsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("cluster restore %s failed", clusterRestore.Name)
	case dpv1alpha1.ClusterRestorePhaseCompleted:
	default:
		return opsv1alpha1.OpsRunningPhase, 0, nil
	}

	targetClusterKey := client.ObjectKey{
		Namespace: clusterRestore.Namespace,
		Name:      clusterRestore.Spec.TargetClusterName,
	}
	if clusterRestore.Status.TargetClusterRef != nil {
		if clusterRestore.Status.TargetClusterRef.Namespace != "" {
			targetClusterKey.Namespace = clusterRestore.Status.TargetClusterRef.Namespace
		}
		if clusterRestore.Status.TargetClusterRef.Name != "" {
			targetClusterKey.Name = clusterRestore.Status.TargetClusterRef.Name
		}
	}

	cluster := &appsv1.Cluster{}
	if err := cli.Get(reqCtx.Ctx, targetClusterKey, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return opsv1alpha1.OpsRunningPhase, 0, nil
		}
		return opsv1alpha1.OpsFailedPhase, 0, err
	}
	opsRes.Cluster = cluster
	if cluster.Status.Phase == appsv1.FailedClusterPhase || cluster.IsDeleting() {
		return opsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("restore failed")
	}
	if cluster.Status.Phase != appsv1.RunningClusterPhase {
		return opsv1alpha1.OpsRunningPhase, 0, nil
	}
	return opsv1alpha1.OpsSucceedPhase, 0, nil
}

// SaveLastConfiguration saves last configuration to the OpsRequest.status.lastConfiguration
func (r RestoreOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error {
	return nil
}

func (r RestoreOpsHandler) buildClusterRestore(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRequest *opsv1alpha1.OpsRequest) (*dpv1alpha1.ClusterRestore, error) {
	restoreSpec := opsRequest.Spec.GetRestore()
	if restoreSpec == nil {
		return nil, intctrlutil.NewFatalError("spec.restore can not be empty")
	}
	backupName := restoreSpec.BackupName
	backupNamespace := restoreSpec.BackupNamespace
	if backupNamespace == "" {
		backupNamespace = opsRequest.Namespace
	}
	// check if the backup exists
	backup := &dpv1alpha1.Backup{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{
		Name:      backupName,
		Namespace: backupNamespace,
	}, backup); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf("backup %s not found in namespace %s", backupName, backupNamespace))
		}
		return nil, err
	}

	// check if the backup is completed
	backupType := backup.Labels[dptypes.BackupTypeLabelKey]
	if backup.Status.Phase != dpv1alpha1.BackupPhaseCompleted && backupType != string(dpv1alpha1.BackupTypeContinuous) {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf("backup %s status is %s, only completed backup can be used to restore", backupName, backup.Status.Phase))
	}

	// format and validate the restore time
	if backupType == string(dpv1alpha1.BackupTypeContinuous) {
		restoreTimeStr, err := restore.FormatRestoreTimeAndValidate(restoreSpec.RestorePointInTime, backup)
		if err != nil {
			return nil, intctrlutil.NewFatalError(err.Error())
		}
		restoreSpec.RestorePointInTime = restoreTimeStr
	}

	return &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRestoreName(opsRequest),
			Namespace: opsRequest.Namespace,
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    opsRequest.Spec.GetClusterName(),
				constant.OpsRequestTypeLabelKey: string(opsRequest.Spec.Type),
			},
		},
		Spec: dpv1alpha1.ClusterRestoreSpec{
			TargetClusterName: opsRequest.Spec.GetClusterName(),
			BackupRef: dpv1alpha1.ClusterRestoreBackupRef{
				Name:      backupName,
				Namespace: backupNamespace,
			},
			RestoreTime:                       restoreSpec.RestorePointInTime,
			VolumeRestorePolicy:               dpv1alpha1.VolumeClaimRestorePolicy(restoreSpec.VolumeRestorePolicy),
			DeferPostReadyUntilClusterRunning: restoreSpec.DeferPostReadyUntilClusterRunning,
			Env:                               restoreSpec.Env,
			Parameters:                        restoreSpec.Parameters,
		},
	}, nil
}

func clusterRestoreName(opsRequest *opsv1alpha1.OpsRequest) string {
	return opsRequest.Name
}

func isClusterRestoreOwnedByOpsRequest(clusterRestore *dpv1alpha1.ClusterRestore, opsRequest *opsv1alpha1.OpsRequest) bool {
	for _, ref := range clusterRestore.OwnerReferences {
		if ref.UID == opsRequest.UID &&
			ref.Name == opsRequest.Name &&
			ref.Kind == "OpsRequest" &&
			ref.APIVersion == opsv1alpha1.GroupVersion.String() {
			return true
		}
	}
	return false
}
