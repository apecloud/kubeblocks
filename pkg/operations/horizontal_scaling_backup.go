/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

const (
	replicaRestoreInProgressMessage = "Restore Data In Progress"
	replicaRestoreCompletedMessage  = "Restore Data Completed"
)

func (hs horizontalScalingOpsHandler) getBackupObj(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	fromBackup opsv1alpha1.FromBackup) (*dpv1alpha1.Backup, error) {
	backupNamespace := opsRes.Cluster.Namespace
	if fromBackup.Namespace != "" {
		backupNamespace = fromBackup.Namespace
	}
	backupObj := &dpv1alpha1.Backup{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Namespace: backupNamespace, Name: fromBackup.Name}, backupObj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf("backup %s not found", fromBackup.Name))
		}
		return nil, err
	}
	if backupObj.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf("backup %s phase is not completed", fromBackup.Name))
	}
	return backupObj, nil
}

func buildReplicaRestoreIntent(cluster *appsv1.Cluster, fromBackup opsv1alpha1.FromBackup) *appsv1.ClusterReplicaRestore {
	namespace := fromBackup.Namespace
	if namespace == "" {
		namespace = cluster.Namespace
	}
	return &appsv1.ClusterReplicaRestore{
		Source: appsv1.ClusterRestoreSource{
			APIGroup:  dptypes.DataprotectionAPIGroup,
			Kind:      dptypes.BackupKind,
			Name:      fromBackup.Name,
			Namespace: namespace,
		},
		SourceTargetName: fromBackup.SourceTargetName,
		PITR:             fromBackup.RestorePointInTime,
		Env:              append([]corev1.EnvVar(nil), fromBackup.RestoreEnv...),
	}
}

func (hs horizontalScalingOpsHandler) observeReplicaRestore(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	resource *instanceProgressResource,
	target opsv1alpha1.HorizontalScaling,
	status *opsv1alpha1.OpsRequestComponentStatus) (instanceProgress, error) {
	progress := instanceProgress{}
	last := opsRes.OpsRequest.Status.LastConfiguration.Components[target.ComponentName]
	if last.Replicas == nil {
		return progress, intctrlutil.NewFatalError(fmt.Sprintf("missing previous replica count for component %q", target.ComponentName))
	}
	progress.expectedCount = max(0, resource.clusterComponent.Replicas-*last.Replicas)
	restoreStatus, ok := opsRes.Cluster.Status.ReplicaRestores[target.ComponentName]
	if !ok {
		status.Message = replicaRestoreInProgressMessage
		return progress, nil
	}
	progress.completedCount = min(progress.expectedCount, max(0, restoreStatus.RestoredReplicas))
	progress.succeededCount = progress.completedCount
	switch restoreStatus.Phase {
	case appsv1.ReplicaRestoreFailed:
		if err := hs.rollbackReplicaRestore(reqCtx, cli, opsRes, target.ComponentName); err != nil {
			return progress, err
		}
		message := restoreStatus.Message
		if message == "" {
			message = "owner reported replica restore failure"
		}
		return progress, intctrlutil.NewFatalError(fmt.Sprintf("replica restore for component %q failed: %s", target.ComponentName, message))
	case appsv1.ReplicaRestoreCompleted:
		if progress.completedCount < progress.expectedCount {
			return progress, nil
		}
		status.Message = replicaRestoreCompletedMessage
		if err := hs.clearReplicaRestoreIntent(reqCtx, cli, opsRes, target.ComponentName); err != nil {
			return progress, err
		}
		progress.observationsComplete = true
	default:
		status.Message = replicaRestoreInProgressMessage
	}
	return progress, nil
}

func (hs horizontalScalingOpsHandler) clearReplicaRestoreIntent(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	componentName string) error {
	compSpec := opsRes.Cluster.Spec.GetComponentByName(componentName)
	if compSpec == nil || compSpec.ReplicaRestore == nil {
		return nil
	}
	compSpec.ReplicaRestore = nil
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

func (hs horizontalScalingOpsHandler) rollbackReplicaRestore(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	componentName string) error {
	last := opsRes.OpsRequest.Status.LastConfiguration.Components[componentName]
	if last.Replicas == nil {
		return intctrlutil.NewFatalError(fmt.Sprintf("missing previous replica count for component %q", componentName))
	}
	compSpec := opsRes.Cluster.Spec.GetComponentByName(componentName)
	if compSpec == nil {
		return intctrlutil.NewFatalError(fmt.Sprintf("component %q not found", componentName))
	}
	compSpec.Replicas = *last.Replicas
	compSpec.Instances = last.InstanceTemplates
	compSpec.OfflineInstances = last.OfflineInstances
	compSpec.ReplicaRestore = nil
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

func (hs horizontalScalingOpsHandler) getBackupExpectedCompValues(
	opsRes *OpsResource,
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	horizontalScaling opsv1alpha1.HorizontalScaling) (int32, []appsv1.InstanceTemplate, []string, error) {
	if lastCompConfiguration.Replicas == nil {
		return 0, nil, nil, intctrlutil.NewFatalError(fmt.Sprintf("missing previous replica count for component %q", horizontalScaling.ComponentName))
	}
	if len(lastCompConfiguration.InstanceTemplates) != 0 || len(lastCompConfiguration.OfflineInstances) != 0 {
		return 0, nil, nil, intctrlutil.NewFatalError("scale-out from backup only supports default contiguous component replicas")
	}
	if horizontalScaling.ScaleIn != nil || horizontalScaling.ScaleOut == nil {
		return 0, nil, nil, intctrlutil.NewFatalError("scale-out from backup only supports replica scale-out")
	}
	scaleOut := horizontalScaling.ScaleOut
	if scaleOut.ReplicaChanges == nil || *scaleOut.ReplicaChanges <= 0 {
		return 0, nil, nil, intctrlutil.NewFatalError("scale-out from backup requires a positive replicaChanges")
	}
	if len(scaleOut.Instances) != 0 || len(scaleOut.NewInstances) != 0 || len(scaleOut.OfflineInstancesToOnline) != 0 {
		return 0, nil, nil, intctrlutil.NewFatalError("scale-out from backup does not support instance templates or offline instances")
	}
	return *lastCompConfiguration.Replicas + *scaleOut.ReplicaChanges, nil, nil, nil
}

func isBackupScaling(scaling opsv1alpha1.HorizontalScaling) bool {
	return scaling.ScaleOut != nil && scaling.ScaleOut.FromBackup != nil
}
