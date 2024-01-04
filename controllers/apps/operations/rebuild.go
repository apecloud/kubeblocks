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
	"fmt"
	"time"

	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
)

type RebuildOpsHandler struct{}

var _ OpsHandler = RebuildOpsHandler{}

func init() {
	// register restore operation, it will create a new cluster
	// so set IsClusterCreationEnabled to true
	rebuildBehaviour := OpsBehaviour{
		FromClusterPhases: appsv1alpha1.GetClusterUpRunningPhases(),
		OpsHandler:        RebuildOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.RebuildType, rebuildBehaviour)
}

// ActionStartedCondition the started condition when handling the rebuild request.
func (r RebuildOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewRebuildCondition(opsRes.OpsRequest), nil
}

// Action implements the restore action.
func (r RebuildOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	rebuildSpecs := opsRes.OpsRequest.Spec.RebuildFromBackup
	var restoreObjs []*dpv1alpha1.Restore
	for _, v := range rebuildSpecs {
		// check if the backup exists
		backup := &dpv1alpha1.Backup{}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: v.BackupName, Namespace: opsRes.OpsRequest.Namespace}, backup); err != nil {
			return err
		}
		if backup.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
			return intctrlutil.NewFatalError(fmt.Sprintf("the phase of backup %s is not Completed", backup.Name))
		}
		backupMethod := backup.Status.BackupMethod
		if backupMethod == nil {
			return intctrlutil.NewFatalError(fmt.Sprintf("the backupMethod of backup %s is empty", backup.Name))
		}
		// TODO: rebuild component pvc with prepareData and volume snapshot.
		if boolptr.IsSetToTrue(backupMethod.SnapshotVolumes) {
			return intctrlutil.NewFatalError("rebuild component with volume snapshot is not supported")
		}
		restoreObjs = append(restoreObjs, r.buildPostReadyRestore(opsRes, v, backup))
	}
	return nil
}

func (r RebuildOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	//opsRequest := opsRes.OpsRequest
	//clusterDef := opsRequest.Spec.ClusterRef

	// get cluster
	// TODO: watch restore jobs and if failed, wait a moment to get failed message.
	return appsv1alpha1.OpsRunningPhase, 0, nil
}

// SaveLastConfiguration saves last configuration to the OpsRequest.status.lastConfiguration
func (r RebuildOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error {
	return nil
}

func (r RebuildOpsHandler) buildPostReadyRestore(opsRes *OpsResource,
	rebuildSpec appsv1alpha1.RebuildSpec,
	backupObj *dpv1alpha1.Backup) *dpv1alpha1.Restore {
	randomStr, _ := password.Generate(6, 2, 0, true, true)
	clusterName := opsRes.Cluster.Name
	namespace := opsRes.Cluster.Namespace
	labels := map[string]string{
		constant.AppInstanceLabelKey:    opsRes.Cluster.Name,
		constant.KBAppComponentLabelKey: rebuildSpec.ComponentName,
		constant.OpsRequestNameLabelKey: opsRes.OpsRequest.Name,
	}
	restoreObj := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", clusterName, rebuildSpec.ComponentName, randomStr),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:      rebuildSpec.BackupName,
				Namespace: namespace,
			},
			RestoreTime: rebuildSpec.RestoreTimeStr,
			ReadyConfig: &dpv1alpha1.ReadyConfig{
				ExecAction: &dpv1alpha1.ExecAction{
					Target: dpv1alpha1.ExecActionTarget{
						PodSelector: metav1.LabelSelector{
							MatchLabels: labels,
						},
					},
				},
				JobAction: &dpv1alpha1.JobAction{
					Target: dpv1alpha1.JobActionTarget{
						PodSelector: metav1.LabelSelector{
							MatchLabels: labels,
						},
					},
				},
			},
		},
	}
	backupMethod := backupObj.Status.BackupMethod
	if backupMethod.TargetVolumes != nil {
		restoreObj.Spec.ReadyConfig.JobAction.Target.VolumeMounts = backupMethod.TargetVolumes.VolumeMounts
	}
	var envs []corev1.EnvVar
	for k, v := range rebuildSpec.Parameters {
		envs = append(envs, corev1.EnvVar{Name: k, Value: v})
	}
	restoreObj.Spec.Env = envs
	return restoreObj
}
