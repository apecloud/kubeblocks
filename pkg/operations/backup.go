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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

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

const legacyBackupTimeLayout = "20060102150405"

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

	if _, retryable, err := findBackupForOpsRequest(reqCtx, cli, opsRes); err == nil {
		return nil
	} else if retryable || !apierrors.IsNotFound(err) {
		return err
	}

	// create backup
	if backup, err := buildBackup(reqCtx, cli, opsRequest, cluster); err != nil {
		return err
	} else {
		if err = cli.Create(reqCtx.Ctx, backup); !apierrors.IsAlreadyExists(err) {
			return err
		}
		if existingBackup, _, err := findBackupForOpsRequest(reqCtx, cli, opsRes); err == nil && existingBackup != nil {
			// the backup has already been created by this OpsRequest.
			return nil
		} else {
			return err
		}
	}
}

// ReconcileAction implements the backup reconcile action.
// It will check the backup status and update the OpsRequest status.
// If the backup is completed, it will return OpsSuccess
// If the backup is failed, it will return OpsFailed
func (b BackupOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	backup, retryable, err := findBackupForOpsRequest(reqCtx, cli, opsRes)
	if err != nil {
		if retryable {
			return opsv1alpha1.OpsRunningPhase, 0, err
		}
		return opsv1alpha1.OpsFailedPhase, 0, err
	}
	// check backup status
	phase := backup.Status.Phase
	switch phase {
	case dpv1alpha1.BackupPhaseCompleted:
		return opsv1alpha1.OpsSucceedPhase, 0, nil
	case dpv1alpha1.BackupPhaseFailed:
		return opsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("backup failed")
	}
	return opsv1alpha1.OpsRunningPhase, 0, nil
}

// findBackupForOpsRequest preserves in-flight Backup OpsRequests across the
// timestamp-name to UID-name protocol migration without restoring first-item lookup.
func findBackupForOpsRequest(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*dpv1alpha1.Backup, bool, error) {
	opsRequest := opsRes.OpsRequest
	cluster := opsRes.Cluster
	intent, err := normalizedBackupIntent(opsRequest)
	if err != nil {
		return nil, false, err
	}

	backupKey := client.ObjectKey{Name: intent.BackupName, Namespace: cluster.Namespace}
	backup := &dpv1alpha1.Backup{}
	if err = cli.Get(reqCtx.Ctx, backupKey, backup); err == nil {
		return validateFoundBackup(reqCtx, cli, opsRes, backup, false)
	} else if !apierrors.IsNotFound(err) {
		return nil, false, err
	}
	exactNotFound := err

	if opsRes.APIReader != nil {
		backup = &dpv1alpha1.Backup{}
		if err = opsRes.APIReader.Get(reqCtx.Ctx, backupKey, backup); err == nil {
			return validateFoundBackup(reqCtx, cli, opsRes, backup, true)
		} else if !apierrors.IsNotFound(err) {
			return nil, true, err
		}
		exactNotFound = err
	}

	rawIntent := opsRequest.Spec.GetBackup()
	if rawIntent != nil && rawIntent.BackupName != "" {
		return nil, false, exactNotFound
	}
	legacyBackup, retryable, err := findImplicitLegacyBackup(reqCtx, cli, opsRes)
	if err != nil || legacyBackup != nil {
		return legacyBackup, retryable, err
	}
	return nil, false, exactNotFound
}

func validateFoundBackup(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource,
	backup *dpv1alpha1.Backup, authoritative bool) (*dpv1alpha1.Backup, bool, error) {
	if hasBackupOwnershipProtocol(backup) {
		if err := validateBackupOwnedByOpsRequest(backup, opsRes.OpsRequest, opsRes.Cluster.Name); err != nil {
			return nil, false, backupOwnershipError(backup.Name, err)
		}
		return backup, false, nil
	}

	if authoritative {
		if err := validateLegacyBackup(backup, opsRes.OpsRequest, opsRes.Cluster, opsRes.OpsRequest.Spec.GetBackup()); err != nil {
			return nil, false, backupOwnershipError(backup.Name, err)
		}
		return backup, false, nil
	}
	confirmed, retryable, err := confirmLegacyBackup(reqCtx, cli, opsRes, backup)
	if err != nil {
		if retryable {
			return nil, true, err
		}
		return nil, false, backupOwnershipError(backup.Name, err)
	}
	return confirmed, false, nil
}

func findImplicitLegacyBackup(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*dpv1alpha1.Backup, bool, error) {
	reader := authoritativeBackupReader(cli, opsRes)
	backups := &dpv1alpha1.BackupList{}
	if err := reader.List(reqCtx.Ctx, backups, client.InNamespace(opsRes.Cluster.Namespace),
		client.MatchingLabels(getBackupLabels(opsRes.Cluster.Name, opsRes.OpsRequest.Name))); err != nil {
		return nil, true, err
	}

	var candidate *dpv1alpha1.Backup
	candidateUsesOwnershipProtocol := false
	for i := range backups.Items {
		backup := &backups.Items[i]
		if hasCompleteBackupOwnershipProtocol(backup) &&
			backup.Annotations[constant.OpsRequestUIDAnnotationKey] != string(opsRes.OpsRequest.UID) {
			// A complete protocol proves this object belongs to an older
			// same-name OpsRequest, even when API timestamps share a second.
			continue
		}
		if !backup.CreationTimestamp.IsZero() && !opsRes.OpsRequest.CreationTimestamp.IsZero() &&
			backup.CreationTimestamp.Before(&opsRes.OpsRequest.CreationTimestamp) {
			continue
		}
		usesOwnershipProtocol := hasBackupOwnershipProtocol(backup)
		if usesOwnershipProtocol {
			if err := validateBackupOwnedByOpsRequest(backup, opsRes.OpsRequest, opsRes.Cluster.Name); err != nil {
				return nil, false, intctrlutil.NewFatalError(fmt.Sprintf(
					"invalid backup ownership protocol for %q: %v", backup.Name, err))
			}
		} else if err := validateLegacyBackup(backup, opsRes.OpsRequest, opsRes.Cluster,
			opsRes.OpsRequest.Spec.GetBackup()); err != nil {
			return nil, false, intctrlutil.NewFatalError(fmt.Sprintf("invalid legacy backup %q: %v", backup.Name, err))
		}
		if candidate != nil {
			return nil, false, intctrlutil.NewFatalError(fmt.Sprintf(
				"multiple legacy backups match OpsRequest %s/%s", opsRes.OpsRequest.Namespace, opsRes.OpsRequest.Name))
		}
		candidate = backup.DeepCopy()
		candidateUsesOwnershipProtocol = usesOwnershipProtocol
	}
	if candidate == nil {
		return nil, false, nil
	}
	if candidateUsesOwnershipProtocol {
		confirmed := &dpv1alpha1.Backup{}
		if err := reader.Get(reqCtx.Ctx, client.ObjectKeyFromObject(candidate), confirmed); err != nil {
			return nil, true, err
		}
		if confirmed.UID != candidate.UID {
			return nil, true, fmt.Errorf("backup %q changed identity during lookup", candidate.Name)
		}
		if err := validateBackupOwnedByOpsRequest(confirmed, opsRes.OpsRequest, opsRes.Cluster.Name); err != nil {
			return nil, false, backupOwnershipError(candidate.Name, err)
		}
		return confirmed, false, nil
	}
	confirmed, retryable, err := confirmLegacyBackup(reqCtx, cli, opsRes, candidate)
	if err != nil {
		if retryable {
			return nil, true, err
		}
		return nil, false, intctrlutil.NewFatalError(fmt.Sprintf("invalid legacy backup %q: %v", candidate.Name, err))
	}
	return confirmed, false, nil
}

func confirmLegacyBackup(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource,
	observed *dpv1alpha1.Backup) (*dpv1alpha1.Backup, bool, error) {
	confirmed := &dpv1alpha1.Backup{}
	if err := authoritativeBackupReader(cli, opsRes).Get(reqCtx.Ctx, client.ObjectKeyFromObject(observed), confirmed); err != nil {
		return nil, true, err
	}
	if confirmed.UID != observed.UID {
		return nil, true, fmt.Errorf("legacy backup %q changed identity during adoption", observed.Name)
	}
	if err := validateLegacyBackup(confirmed, opsRes.OpsRequest, opsRes.Cluster, opsRes.OpsRequest.Spec.GetBackup()); err != nil {
		return nil, false, err
	}
	return confirmed, false, nil
}

func authoritativeBackupReader(cli client.Client, opsRes *OpsResource) client.Reader {
	if opsRes.APIReader != nil {
		return opsRes.APIReader
	}
	// The controller always supplies APIReader. The fallback keeps direct
	// handler callers working when their client itself is authoritative.
	return cli
}

func validateLegacyBackup(backup *dpv1alpha1.Backup, opsRequest *opsv1alpha1.OpsRequest,
	cluster *appsv1.Cluster, rawIntent *opsv1alpha1.Backup) error {
	if hasBackupOwnershipProtocol(backup) {
		return fmt.Errorf("backup has a partial or unexpected ownership protocol")
	}
	if backup.UID == "" {
		return fmt.Errorf("backup has no server identity")
	}
	if backup.DeletionTimestamp != nil {
		return fmt.Errorf("backup is terminating")
	}
	if backup.Labels[constant.AppInstanceLabelKey] != cluster.Name ||
		backup.Labels[constant.OpsRequestNameLabelKey] != opsRequest.Name ||
		backup.Labels[constant.OpsRequestTypeLabelKey] != string(opsv1alpha1.BackupType) {
		return fmt.Errorf("backup labels do not match OpsRequest")
	}
	if backup.CreationTimestamp.IsZero() || opsRequest.CreationTimestamp.IsZero() ||
		backup.CreationTimestamp.Before(&opsRequest.CreationTimestamp) {
		return fmt.Errorf("backup predates OpsRequest lifecycle")
	}

	if rawIntent != nil && rawIntent.BackupName != "" {
		if backup.Name != rawIntent.BackupName {
			return fmt.Errorf("backup name does not match explicit OpsRequest intent")
		}
	} else if !isLegacyGeneratedBackupName(backup.Name, cluster.Namespace, cluster.Name) {
		return fmt.Errorf("backup name does not match legacy generated name shape")
	}
	if rawIntent == nil {
		return nil
	}
	if rawIntent.BackupPolicyName != "" && backup.Spec.BackupPolicyName != rawIntent.BackupPolicyName {
		return fmt.Errorf("backup policy does not match OpsRequest intent")
	}
	if rawIntent.BackupMethod != "" && backup.Spec.BackupMethod != rawIntent.BackupMethod {
		return fmt.Errorf("backup method does not match OpsRequest intent")
	}
	if rawIntent.DeletionPolicy != "" && string(backup.Spec.DeletionPolicy) != rawIntent.DeletionPolicy {
		return fmt.Errorf("backup deletion policy does not match OpsRequest intent")
	}
	if rawIntent.RetentionPeriod != "" && string(backup.Spec.RetentionPeriod) != rawIntent.RetentionPeriod {
		return fmt.Errorf("backup retention period does not match OpsRequest intent")
	}
	if rawIntent.ParentBackupName != "" && backup.Spec.ParentBackupName != rawIntent.ParentBackupName {
		return fmt.Errorf("backup parent does not match OpsRequest intent")
	}
	if len(rawIntent.Parameters) > 0 && !equalBackupParameters(backup.Spec.Parameters, rawIntent.Parameters) {
		return fmt.Errorf("backup parameters do not match OpsRequest intent")
	}
	return nil
}

func equalBackupParameters(left, right []dpv1alpha1.ParameterPair) bool {
	if len(left) != len(right) {
		return false
	}
	leftByName := make(map[string]string, len(left))
	for _, parameter := range left {
		leftByName[parameter.Name] = parameter.Value
	}
	rightByName := make(map[string]string, len(right))
	for _, parameter := range right {
		rightByName[parameter.Name] = parameter.Value
	}
	return reflect.DeepEqual(leftByName, rightByName)
}

func hasBackupOwnershipProtocol(backup *dpv1alpha1.Backup) bool {
	if backup.Annotations == nil {
		return false
	}
	for _, key := range []string{
		constant.OpsRequestUIDAnnotationKey,
		constant.OpsRequestBackupIntentHashAnnotationKey,
		constant.OpsRequestBackupSpecHashAnnotationKey,
	} {
		if _, ok := backup.Annotations[key]; ok {
			return true
		}
	}
	return false
}

func hasCompleteBackupOwnershipProtocol(backup *dpv1alpha1.Backup) bool {
	if backup.Annotations == nil {
		return false
	}
	for _, key := range []string{
		constant.OpsRequestUIDAnnotationKey,
		constant.OpsRequestBackupIntentHashAnnotationKey,
		constant.OpsRequestBackupSpecHashAnnotationKey,
	} {
		if _, ok := backup.Annotations[key]; !ok {
			return false
		}
	}
	return true
}

func isLegacyGeneratedBackupName(name, namespace, clusterName string) bool {
	prefix := fmt.Sprintf("backup-%s-%s-", namespace, clusterName)
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	timestamp := strings.TrimPrefix(name, prefix)
	if len(timestamp) != len(legacyBackupTimeLayout) {
		return false
	}
	_, err := time.Parse(legacyBackupTimeLayout, timestamp)
	return err == nil
}

func backupOwnershipError(name string, cause error) error {
	return intctrlutil.NewFatalError(fmt.Sprintf(
		`backup "%s" already exists and is not created by this OpsRequest: %v`, name, cause))
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (b BackupOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func buildBackup(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRequest *opsv1alpha1.OpsRequest, cluster *appsv1.Cluster) (*dpv1alpha1.Backup, error) {
	var err error

	backupSpec, err := normalizedBackupIntent(opsRequest)
	if err != nil {
		return nil, err
	}

	backupSpec.BackupPolicyName, err = getDefaultBackupPolicy(reqCtx, cli, cluster, backupSpec.BackupPolicyName)
	if err != nil {
		return nil, err
	}

	backupPolicyList := &dpv1alpha1.BackupPolicyList{}
	if err = cli.List(reqCtx.Ctx, backupPolicyList, client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{
			constant.AppInstanceLabelKey: cluster.Name,
		})); err != nil {
		return nil, err
	}
	defaultBackupMethod, backupMethodMap := utils.GetBackupMethodsFromBackupPolicy(backupPolicyList, backupSpec.BackupPolicyName)
	if backupSpec.BackupMethod == "" {
		if defaultBackupMethod == "" {
			return nil, fmt.Errorf("failed to find default backup method, please check cluster's backup policy")
		}
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
			DeletionPolicy:   dpv1alpha1.BackupDeletionPolicyDelete,
			Parameters:       backupSpec.Parameters,
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
		// A Cluster name is reusable. The DP Cluster UID label keeps incremental
		// lineage from crossing into a same-name recreated Cluster.
		if parentBackup.Labels[constant.AppInstanceLabelKey] != cluster.Name {
			return nil, fmt.Errorf("parent backup %s is not belong to cluster %s", backupSpec.ParentBackupName, cluster.Name)
		}
		if parentBackup.Labels[dptypes.ClusterUIDLabelKey] != string(cluster.UID) {
			return nil, fmt.Errorf("parent backup %s has a different cluster UID", backupSpec.ParentBackupName)
		}
		backup.Spec.ParentBackupName = backupSpec.ParentBackupName
	}
	backup.Annotations, err = getBackupAnnotations(opsRequest, backup.Spec)
	if err != nil {
		return nil, err
	}

	return backup, nil
}

func validateBackupOwnedByOpsRequest(existingBackup *dpv1alpha1.Backup, opsRequest *opsv1alpha1.OpsRequest, clusterName string) error {
	intent, err := normalizedBackupIntent(opsRequest)
	if err != nil {
		return err
	}
	if existingBackup.Name != intent.BackupName {
		return fmt.Errorf("backup name does not match OpsRequest intent")
	}
	if existingBackup.Labels[constant.AppInstanceLabelKey] != clusterName ||
		existingBackup.Labels[constant.OpsRequestNameLabelKey] != opsRequest.Name ||
		existingBackup.Labels[constant.OpsRequestTypeLabelKey] != string(opsv1alpha1.BackupType) {
		return fmt.Errorf("backup labels do not match OpsRequest")
	}
	if existingBackup.Annotations[constant.OpsRequestUIDAnnotationKey] != string(opsRequest.UID) {
		return fmt.Errorf("backup UID annotation does not match OpsRequest")
	}
	intentHash, err := hashBackupObject(intent)
	if err != nil {
		return err
	}
	if existingBackup.Annotations[constant.OpsRequestBackupIntentHashAnnotationKey] != intentHash {
		return fmt.Errorf("backup intent does not match OpsRequest")
	}
	resolvedSpecHash, err := hashBackupObject(normalizedBackupSpec(existingBackup.Spec))
	if err != nil {
		return err
	}
	if existingBackup.Annotations[constant.OpsRequestBackupSpecHashAnnotationKey] != resolvedSpecHash {
		return fmt.Errorf("backup spec does not match its resolved write intent")
	}
	return nil
}

func normalizedBackupIntent(opsRequest *opsv1alpha1.OpsRequest) (*opsv1alpha1.Backup, error) {
	backupSpec := opsRequest.Spec.GetBackup()
	if backupSpec == nil {
		backupSpec = &opsv1alpha1.Backup{}
	} else {
		backupSpec = backupSpec.DeepCopy()
	}
	if backupSpec.BackupName == "" {
		if opsRequest.UID == "" {
			return nil, fmt.Errorf("opsRequest %s/%s has no UID yet", opsRequest.Namespace, opsRequest.Name)
		}
		backupSpec.BackupName = fmt.Sprintf("backup-%s", opsRequest.UID)
	}
	if backupSpec.DeletionPolicy == "" {
		backupSpec.DeletionPolicy = string(dpv1alpha1.BackupDeletionPolicyDelete)
	}
	return backupSpec, nil
}

func normalizedBackupSpec(spec dpv1alpha1.BackupSpec) dpv1alpha1.BackupSpec {
	if spec.DeletionPolicy == "" {
		spec.DeletionPolicy = dpv1alpha1.BackupDeletionPolicyDelete
	}
	return spec
}

func hashBackupObject(obj any) (string, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
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
		return "", fmt.Errorf(`not found any default backup policy for cluster "%s"`, cluster.Name)
	}
	if len(defaultBackupPolices.Items) > 1 {
		return "", fmt.Errorf(`cluster "%s" has multiple default backup policies`, cluster.Name)
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

func getBackupAnnotations(opsRequest *opsv1alpha1.OpsRequest, resolvedSpec dpv1alpha1.BackupSpec) (map[string]string, error) {
	intent, err := normalizedBackupIntent(opsRequest)
	if err != nil {
		return nil, err
	}
	intentHash, err := hashBackupObject(intent)
	if err != nil {
		return nil, err
	}
	resolvedSpecHash, err := hashBackupObject(normalizedBackupSpec(resolvedSpec))
	if err != nil {
		return nil, err
	}
	return map[string]string{
		constant.OpsRequestUIDAnnotationKey:              string(opsRequest.UID),
		constant.OpsRequestBackupIntentHashAnnotationKey: intentHash,
		constant.OpsRequestBackupSpecHashAnnotationKey:   resolvedSpecHash,
	}, nil
}
