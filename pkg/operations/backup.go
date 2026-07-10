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
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
)

type BackupOpsHandler struct{}

var _ OpsHandler = BackupOpsHandler{}

func init() {
	// ToClusterPhase is not defined, because 'backup' does not affect the cluster phase.
	backupBehaviour := OpsBehaviour{
		FromClusterPhases: []appsv1.ClusterPhase{appsv1.RunningClusterPhase,
			appsv1.UpdatingClusterPhase, appsv1.AbnormalClusterPhase},
		OpsHandler: BackupOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.BackupType, backupBehaviour)
}

// ActionStartedCondition the started condition when handling the backup request.
func (b BackupOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewBackupCondition(opsRes.OpsRequest), nil
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
		if err = cli.Create(reqCtx.Ctx, backup); !apierrors.IsAlreadyExists(err) {
			return err
		}
		existingBackup := &dpv1alpha1.Backup{}
		if getErr := cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(backup), existingBackup); getErr != nil {
			return getErr
		}
		if validateBackupOwnedByOpsRequest(existingBackup, backup, opsRequest) == nil {
			// the backup has already been created by this OpsRequest.
			return nil
		}
		return intctrlutil.NewFatalError(fmt.Sprintf(`backup "%s" already exists and is not created by this OpsRequest`, backup.Name))
	}
}

// ReconcileAction implements the backup reconcile action.
// It will check the backup status and update the OpsRequest status.
// If the backup is completed, it will return OpsSuccess
// If the backup is failed, it will return OpsFailed
func (b BackupOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	opsRequest := opsRes.OpsRequest
	cluster := opsRes.Cluster

	// get backup
	backups := &dpv1alpha1.BackupList{}
	if err := cli.List(reqCtx.Ctx, backups, client.InNamespace(cluster.Namespace), client.MatchingLabels(getBackupLabels(cluster.Name, opsRequest.Name))); err != nil {
		return opsv1alpha1.OpsFailedPhase, 0, err
	}

	if len(backups.Items) == 0 {
		return opsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("backup not found")
	}
	// check backup status
	phase := backups.Items[0].Status.Phase
	switch phase {
	case dpv1alpha1.BackupPhaseCompleted:
		return opsv1alpha1.OpsSucceedPhase, 0, nil
	case dpv1alpha1.BackupPhaseFailed:
		return opsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("backup failed")
	}
	return opsv1alpha1.OpsRunningPhase, 0, nil
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (b BackupOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func buildBackup(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRequest *opsv1alpha1.OpsRequest, cluster *appsv1.Cluster) (*dpv1alpha1.Backup, error) {
	var err error

	backupSpec := opsRequest.Spec.GetBackup()
	if backupSpec == nil {
		backupSpec = &opsv1alpha1.Backup{}
	}

	if len(backupSpec.BackupName) == 0 {
		if opsRequest.UID == "" {
			return nil, fmt.Errorf("opsRequest %s/%s has no UID yet", opsRequest.Namespace, opsRequest.Name)
		}
		backupSpec.BackupName = fmt.Sprintf("backup-%s", opsRequest.UID)
	}

	explicitPolicyName := backupSpec.BackupPolicyName != ""
	backupSpec.BackupPolicyName, err = getDefaultBackupPolicy(reqCtx, cli, cluster, backupSpec.BackupPolicyName)
	if err != nil {
		return nil, err
	}

	backupPolicy := &dpv1alpha1.BackupPolicy{}
	if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: backupSpec.BackupPolicyName, Namespace: cluster.Namespace}, backupPolicy); err != nil {
		if apierrors.IsNotFound(err) && explicitPolicyName {
			// the user explicitly referenced a nonexistent backup policy, which is deterministic.
			return nil, intctrlutil.NewFatalError(err.Error())
		}
		return nil, err
	}
	if explicitPolicyName && backupPolicy.Labels[constant.AppInstanceLabelKey] != cluster.Name {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf("backup policy %s is not belong to cluster %s", backupPolicy.Name, cluster.Name))
	}
	if backupPolicy.Status.Phase != dpv1alpha1.AvailablePhase {
		// the backup policy exists but has not been reconciled to Available yet, retry until it converges.
		return nil, fmt.Errorf(`backup policy "%s" is not available yet, phase: "%s"`, backupPolicy.Name, backupPolicy.Status.Phase)
	}
	defaultBackupMethod, backupMethodMap := utils.GetBackupMethodsFromBackupPolicy(
		&dpv1alpha1.BackupPolicyList{Items: []dpv1alpha1.BackupPolicy{*backupPolicy}}, backupSpec.BackupPolicyName)
	// the backup policy is known to be Available here, so a missing backup method is deterministic.
	if backupSpec.BackupMethod == "" {
		if defaultBackupMethod == "" {
			return nil, intctrlutil.NewFatalError("failed to find default backup method, please check cluster's backup policy")
		}
		backupSpec.BackupMethod = defaultBackupMethod
	}
	if _, ok := backupMethodMap[backupSpec.BackupMethod]; !ok {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf("backup method %s is not supported, please check cluster's backup policy", backupSpec.BackupMethod))
	}

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:        backupSpec.BackupName,
			Namespace:   cluster.Namespace,
			Labels:      getBackupLabels(cluster.Name, opsRequest.Name),
			Annotations: getBackupAnnotations(opsRequest),
		},
		Spec: dpv1alpha1.BackupSpec{
			BackupPolicyName: backupSpec.BackupPolicyName,
			BackupMethod:     backupSpec.BackupMethod,
			Parameters:       backupSpec.Parameters,
		},
	}

	if backupSpec.DeletionPolicy != "" {
		backup.Spec.DeletionPolicy = dpv1alpha1.BackupDeletionPolicy(backupSpec.DeletionPolicy)
	}
	if backupSpec.RetentionPeriod != "" {
		retentionPeriod := dpv1alpha1.RetentionPeriod(backupSpec.RetentionPeriod)
		if _, err := retentionPeriod.ToDuration(); err != nil {
			return nil, intctrlutil.NewFatalError(err.Error())
		}
		backup.Spec.RetentionPeriod = retentionPeriod
	}
	if backupSpec.ParentBackupName != "" {
		parentBackup := dpv1alpha1.Backup{}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: backupSpec.ParentBackupName, Namespace: cluster.Namespace}, &parentBackup); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, intctrlutil.NewFatalError(err.Error())
			}
			return nil, err
		}
		// check parent backup exists and completed
		if parentBackup.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
			if parentBackup.Status.Phase == dpv1alpha1.BackupPhaseFailed || parentBackup.Status.Phase == dpv1alpha1.BackupPhaseDeleting {
				return nil, intctrlutil.NewFatalError(fmt.Sprintf("parent backup %s is %s", backupSpec.ParentBackupName, parentBackup.Status.Phase))
			}
			return nil, fmt.Errorf("parent backup %s is not completed", backupSpec.ParentBackupName)
		}
		// check parent backup belongs to the cluster of the backup
		if parentBackup.Labels[constant.AppInstanceLabelKey] != cluster.Name {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf("parent backup %s is not belong to cluster %s", backupSpec.ParentBackupName, cluster.Name))
		}
		backup.Spec.ParentBackupName = backupSpec.ParentBackupName
	}

	return backup, nil
}

func validateBackupOwnedByOpsRequest(existingBackup, desiredBackup *dpv1alpha1.Backup, opsRequest *opsv1alpha1.OpsRequest) error {
	if existingBackup.Labels[constant.OpsRequestNameLabelKey] != opsRequest.Name ||
		existingBackup.Labels[constant.OpsRequestTypeLabelKey] != string(opsv1alpha1.BackupType) {
		return fmt.Errorf("backup labels do not match OpsRequest")
	}
	if existingBackup.Annotations[constant.OpsRequestUIDAnnotationKey] != string(opsRequest.UID) {
		return fmt.Errorf("backup UID annotation does not match OpsRequest")
	}
	if !apiequality.Semantic.DeepEqual(existingBackup.Spec, desiredBackup.Spec) {
		return fmt.Errorf("backup spec does not match OpsRequest")
	}
	return nil
}

func getDefaultBackupPolicy(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1.Cluster, backupPolicy string) (string, error) {
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
		// the default backup policy is generated by the dataprotection controller; it may not have
		// been created yet when the ops is submitted, so keep this retryable instead of fatal.
		return "", fmt.Errorf(`not found any default backup policy for cluster "%s"`, cluster.Name)
	}
	if len(defaultBackupPolices.Items) > 1 {
		return "", intctrlutil.NewFatalError(fmt.Sprintf(`cluster "%s" has multiple default backup policies`, cluster.Name))
	}

	return defaultBackupPolices.Items[0].GetName(), nil
}

func getBackupLabels(cluster, request string) map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey:    cluster,
		constant.OpsRequestNameLabelKey: request,
		constant.OpsRequestTypeLabelKey: string(opsv1alpha1.BackupType),
	}
}

func getBackupAnnotations(opsRequest *opsv1alpha1.OpsRequest) map[string]string {
	return map[string]string{
		constant.OpsRequestUIDAnnotationKey: string(opsRequest.UID),
	}
}
