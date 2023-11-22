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
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
)

const backupTimeLayout = "20060102150405"

type BackupOpsHandler struct{}

var _ OpsHandler = BackupOpsHandler{}

func init() {
	// ToClusterPhase is not defined, because 'backup' does not affect the cluster phase.
	backupBehaviour := OpsBehaviour{
		FromClusterPhases: appsv1alpha1.GetClusterUpRunningPhases(),
		OpsHandler:        BackupOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.BackupType, backupBehaviour)
}

// ActionStartedCondition the started condition when handling the backup request.
func (b BackupOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewBackupCondition(opsRes.OpsRequest), nil
}

// Action implements the backup action.
// It will create a backup resource for cluster.
func (b BackupOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	cluster := opsRes.Cluster

	// create backup
	if backup, err := buildBackup(reqCtx, cli, opsRequest, cluster); err != nil {
		return err
	} else {
		return cli.Create(reqCtx.Ctx, backup)
	}
}

// ReconcileAction implements the backup reconcile action.
// It will check the backup status and update the OpsRequest status.
// If the backup is completed, it will return OpsSuccess
// If the backup is failed, it will return OpsFailed
func (b BackupOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	opsRequest := opsRes.OpsRequest
	cluster := opsRes.Cluster

	// get backup
	backups := &dpv1alpha1.BackupList{}
	if err := cli.List(reqCtx.Ctx, backups, client.InNamespace(cluster.Namespace), client.MatchingLabels(getBackupLabels(cluster.Name, opsRequest.Name))); err != nil {
		return appsv1alpha1.OpsFailedPhase, 0, err
	}

	if len(backups.Items) == 0 {
		return appsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("backup not found")
	}
	// check backup status
	phase := backups.Items[0].Status.Phase
	if phase == dpv1alpha1.BackupPhaseCompleted {
		return appsv1alpha1.OpsSucceedPhase, 0, nil
	} else if phase == dpv1alpha1.BackupPhaseFailed {
		return appsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("backup failed")
	}
	return appsv1alpha1.OpsRunningPhase, 0, nil
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (b BackupOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func buildBackup(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRequest *appsv1alpha1.OpsRequest, cluster *appsv1alpha1.Cluster) (*dpv1alpha1.Backup, error) {
	var err error

	backupSpec := opsRequest.Spec.BackupSpec
	if backupSpec == nil {
		backupSpec = &appsv1alpha1.BackupSpec{}
	}

	if len(backupSpec.BackupName) == 0 {
		backupSpec.BackupName = strings.Join([]string{"backup", cluster.Namespace, cluster.Name, time.Now().Format(backupTimeLayout)}, "-")
	}

	backupSpec.BackupPolicyName, err = getDefaultBackupPolicy(reqCtx, cli, cluster, backupSpec.BackupPolicyName)
	if err != nil {
		return nil, err
	}

	backupPolicyList := &dpv1alpha1.BackupPolicyList{}
	if err := cli.List(reqCtx.Ctx, backupPolicyList, client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{
			constant.AppInstanceLabelKey: cluster.Name,
		})); err != nil {
		return nil, err
	}
	defaultBackupMethod, backupMethodMap, err := utils.GetBackupMethodsFromBackupPolicy(backupPolicyList, backupSpec.BackupPolicyName)
	if err != nil {
		return nil, err
	}
	if backupSpec.BackupMethod == "" {
		backupSpec.BackupMethod = defaultBackupMethod
	}
	if _, ok := backupMethodMap[backupSpec.BackupMethod]; !ok {
		return nil, fmt.Errorf("backup method %s is not supported, please check cluster's backup policy", backupSpec.BackupMethod)
	}

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupSpec.BackupName,
			Namespace: cluster.Namespace,
			Labels:    getBackupLabels(cluster.Name, opsRequest.Name),
		},
		Spec: dpv1alpha1.BackupSpec{
			BackupPolicyName: backupSpec.BackupPolicyName,
			BackupMethod:     backupSpec.BackupMethod,
		},
	}

	if backupSpec.DeletionPolicy != "" {
		backup.Spec.DeletionPolicy = dpv1alpha1.BackupDeletionPolicy(backupSpec.DeletionPolicy)
	}
	if backupSpec.RetentionPeriod != "" {
		retentionPeriod := dpv1alpha1.RetentionPeriod(backupSpec.RetentionPeriod)
		if _, err := retentionPeriod.ToDuration(); err != nil {
			return nil, err
		}
		backup.Spec.RetentionPeriod = retentionPeriod
	}
	if backupSpec.ParentBackupName != "" {
		parentBackup := dpv1alpha1.Backup{}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: backupSpec.ParentBackupName, Namespace: cluster.Namespace}, &parentBackup); err != nil {
			return nil, err
		}
		// check parent backup exists and completed
		if parentBackup.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
			return nil, fmt.Errorf("parent backup %s is not completed", backupSpec.ParentBackupName)
		}
		// check parent backup belongs to the cluster of the backup
		if parentBackup.Labels[constant.AppInstanceLabelKey] != cluster.Name {
			return nil, fmt.Errorf("parent backup %s is not belong to cluster %s", backupSpec.ParentBackupName, cluster.Name)
		}
		backup.Spec.ParentBackupName = backupSpec.ParentBackupName
	}

	return backup, nil
}

func getDefaultBackupPolicy(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1alpha1.Cluster, backupPolicy string) (string, error) {
	// if backupPolicy is not empty, return it directly
	if backupPolicy != "" {
		return backupPolicy, nil
	}

	backupPolicyList := &dpv1alpha1.BackupPolicyList{}
	if err := cli.List(reqCtx.Ctx, backupPolicyList, client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{
			constant.AppInstanceLabelKey: cluster.Name,
		})); err != nil {
		return "", err
	}
	defaultBackupPolices := &dpv1alpha1.BackupPolicyList{}
	for _, backupPolicy := range backupPolicyList.Items {
		if backupPolicy.GetAnnotations()[dptypes.DefaultBackupPolicyAnnotationKey] == "true" {
			defaultBackupPolices.Items = append(defaultBackupPolices.Items, backupPolicy)
		}
	}

	if len(defaultBackupPolices.Items) == 0 {
		return "", fmt.Errorf(`not found any default backup policy for cluster "%s"`, cluster.Name)
	}
	if len(defaultBackupPolices.Items) > 1 {
		return "", fmt.Errorf(`cluster "%s" has multiple default backup policies`, cluster.Name)
	}

	return defaultBackupPolices.Items[0].GetName(), nil
}

func getBackupLabels(cluster, request string) map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey:      cluster,
		constant.BackupProtectionLabelKey: constant.BackupRetain,
		constant.OpsRequestNameLabelKey:   request,
		constant.OpsRequestTypeLabelKey:   string(appsv1alpha1.BackupType),
	}
}
