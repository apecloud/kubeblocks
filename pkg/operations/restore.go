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
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/operations/util"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type RestoreOpsHandler struct{}

var _ OpsHandler = RestoreOpsHandler{}

const restoredSystemAccountLabel = "apps.kubeblocks.io/system-account"

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
	var cluster *appsv1.Cluster
	var err error

	opsRequest := opsRes.OpsRequest

	// restore the cluster from the backup
	if cluster, err = r.restoreClusterFromBackup(reqCtx, cli, opsRequest); err != nil {
		return err
	}

	restoreSpec := opsRequest.Spec.GetRestore()
	backupNamespace := restoreSpec.BackupNamespace
	if backupNamespace == "" {
		backupNamespace = opsRequest.Namespace
	}
	backup := &dpv1alpha1.Backup{}
	if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: restoreSpec.BackupName, Namespace: backupNamespace}, backup); err != nil {
		return err
	}
	if err = r.prepareRestoredSystemAccounts(reqCtx, cli, cluster, backup); err != nil {
		return err
	}

	// create cluster
	if err = cli.Create(reqCtx.Ctx, cluster); err != nil {
		if apierrors.IsAlreadyExists(err) && opsRequest.Labels[constant.AppInstanceLabelKey] != "" {
			// already create by this opsRequest
			return nil
		}
		return err
	}
	opsRes.Cluster = cluster

	// add labels of clusterRef and type to OpsRequest
	// and set owner reference to cluster
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Labels == nil {
		opsRequest.Labels = make(map[string]string)
	}
	opsRequest.Labels[constant.AppInstanceLabelKey] = opsRequest.Spec.GetClusterName()
	opsRequest.Labels[constant.OpsRequestTypeLabelKey] = string(opsRequest.Spec.Type)
	scheme, _ := appsv1.SchemeBuilder.Build()
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
func (r RestoreOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	opsRequest := opsRes.OpsRequest
	clusterDef := opsRequest.Spec.GetClusterName()

	cluster := &appsv1.Cluster{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: opsRequest.GetNamespace(),
		Name:      clusterDef,
	}, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			_ = PatchClusterNotFound(reqCtx.Ctx, cli, opsRes)
		}
		return opsv1alpha1.OpsFailedPhase, 0, err
	}
	opsRes.Cluster = cluster
	if cluster.Status.Phase == appsv1.FailedClusterPhase || cluster.IsDeleting() {
		return opsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("restore failed")
	}
	restoreCond := meta.FindStatusCondition(cluster.Status.Conditions, dptypes.RestoreSessionConditionType)
	if restoreCond == nil {
		return opsv1alpha1.OpsRunningPhase, 0, nil
	}
	switch restoreCond.Reason {
	case string(dpv1alpha1.RestorePhaseCompleted):
		return opsv1alpha1.OpsSucceedPhase, 0, nil
	case string(dpv1alpha1.RestorePhaseFailed):
		return opsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("restore failed: %s", restoreCond.Message)
	}
	return opsv1alpha1.OpsRunningPhase, 0, nil
}

// SaveLastConfiguration saves last configuration to the OpsRequest.status.lastConfiguration
func (r RestoreOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error {
	return nil
}

func (r RestoreOpsHandler) restoreClusterFromBackup(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRequest *opsv1alpha1.OpsRequest) (*appsv1.Cluster, error) {
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
		opsRequest.Spec.GetRestore().RestorePointInTime = restoreTimeStr
	}
	// get the cluster object from backup
	clusterObj, err := r.getClusterObjFromBackup(backup, opsRequest)
	if err != nil {
		return nil, err
	}
	opsRequestSlice := []opsv1alpha1.OpsRecorder{
		{
			Name: opsRequest.Name,
			Type: opsRequest.Spec.Type,
		},
	}
	util.SetOpsRequestToCluster(clusterObj, opsRequestSlice)
	return clusterObj, nil
}

func (r RestoreOpsHandler) getClusterObjFromBackup(backup *dpv1alpha1.Backup, opsRequest *opsv1alpha1.OpsRequest) (*appsv1.Cluster, error) {
	cluster := &appsv1.Cluster{}
	// use the cluster snapshot to restore firstly
	clusterString, ok := backup.Annotations[constant.ClusterSnapshotAnnotationKey]
	if !ok {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf("missing snapshot annotation in backup %s, %s is empty in Annotations", backup.Name, constant.ClusterSnapshotAnnotationKey))
	}
	if err := json.Unmarshal([]byte(clusterString), &cluster); err != nil {
		return nil, err
	}
	restoreSpec := opsRequest.Spec.GetRestore()
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	delete(cluster.Annotations, constant.RestoreFromBackupAnnotationKey)
	cluster.Name = opsRequest.Spec.GetClusterName()
	cluster.Namespace = opsRequest.Namespace
	if err := injectBackupDataSourceRef(cluster, backup, restoreSpec); err != nil {
		return nil, err
	}
	// Reset cluster services
	var services []appsv1.ClusterService
	for i := range cluster.Spec.Services {
		svc := cluster.Spec.Services[i]
		if svc.Service.Spec.Type == corev1.ServiceTypeLoadBalancer {
			continue
		}
		if svc.Service.Spec.Type == corev1.ServiceTypeNodePort {
			for j := range svc.Spec.Ports {
				svc.Spec.Ports[j].NodePort = 0
			}
		}
		if svc.Service.Spec.Selector != nil {
			delete(svc.Service.Spec.Selector, constant.AppInstanceLabelKey)
		}
		services = append(services, svc)
	}
	cluster.Spec.Services = services
	for i := range cluster.Spec.ComponentSpecs {
		cluster.Spec.ComponentSpecs[i].OfflineInstances = nil
		cluster.Spec.ComponentSpecs[i].TLS = false
		cluster.Spec.ComponentSpecs[i].Issuer = nil
	}
	r.rebuildShardAccountSecrets(cluster)
	r.normalizeSchedulePolicy(cluster, cluster.Spec.SchedulingPolicy)
	for i := range cluster.Spec.ComponentSpecs {
		r.normalizeSchedulePolicy(cluster, cluster.Spec.ComponentSpecs[i].SchedulingPolicy)
	}
	for i := range cluster.Spec.Shardings {
		r.normalizeSchedulePolicy(cluster, cluster.Spec.Shardings[i].Template.SchedulingPolicy)
	}
	return cluster, nil
}

func injectBackupDataSourceRef(cluster *appsv1.Cluster, backup *dpv1alpha1.Backup, restoreSpec *opsv1alpha1.Restore) error {
	if restoreSpec == nil {
		return nil
	}
	inject := func(ownerName string, vct *appsv1.PersistentVolumeClaimTemplate) error {
		vct.Spec.DataSourceRef = restore.BackupDataSourceRef(backup.Name)
		options := restore.DefaultRestoreOptions()
		options.BackupNamespace = backup.Namespace
		if options.BackupNamespace == "" || options.BackupNamespace == cluster.Namespace {
			options.BackupNamespace = ""
		}
		options.RestoreTime = restoreSpec.RestorePointInTime
		options.VolumeSource = vct.Name
		options.SourceTargetName = inferBackupSourceTargetName(backup, ownerName)
		options.VolumeRestorePolicy = dpv1alpha1.VolumeClaimRestorePolicy(restoreSpec.VolumeRestorePolicy)
		options.DeferPostReadyUntilClusterRunning = restoreSpec.DeferPostReadyUntilClusterRunning
		options.Env = restoreSpec.Env
		options.Parameters = restoreSpec.Parameters
		annotations, err := restore.SetRestoreOptions(vct.Annotations, options)
		if err != nil {
			return err
		}
		vct.Annotations = annotations
		return nil
	}
	for i := range cluster.Spec.ComponentSpecs {
		for j := range cluster.Spec.ComponentSpecs[i].VolumeClaimTemplates {
			if err := inject(cluster.Spec.ComponentSpecs[i].Name, &cluster.Spec.ComponentSpecs[i].VolumeClaimTemplates[j]); err != nil {
				return err
			}
		}
	}
	for i := range cluster.Spec.Shardings {
		for j := range cluster.Spec.Shardings[i].Template.VolumeClaimTemplates {
			if err := inject(cluster.Spec.Shardings[i].Name, &cluster.Spec.Shardings[i].Template.VolumeClaimTemplates[j]); err != nil {
				return err
			}
		}
		for j := range cluster.Spec.Shardings[i].ShardTemplates {
			for k := range cluster.Spec.Shardings[i].ShardTemplates[j].VolumeClaimTemplates {
				if err := inject(cluster.Spec.Shardings[i].ShardTemplates[j].Name, &cluster.Spec.Shardings[i].ShardTemplates[j].VolumeClaimTemplates[k]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func inferBackupSourceTargetName(backup *dpv1alpha1.Backup, ownerName string) string {
	if backup == nil {
		return ""
	}
	if backup.Status.Target != nil {
		return backup.Status.Target.Name
	}
	if len(backup.Status.Targets) == 1 {
		return backup.Status.Targets[0].Name
	}
	for i := range backup.Status.Targets {
		if backup.Status.Targets[i].Name == ownerName {
			return backup.Status.Targets[i].Name
		}
	}
	return ""
}

func (r RestoreOpsHandler) prepareRestoredSystemAccounts(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1.Cluster, backup *dpv1alpha1.Backup) error {
	encryptedAccounts := backup.Annotations[constant.EncryptedSystemAccountsAnnotationKey]
	if encryptedAccounts == "" {
		return nil
	}
	accountMap := map[string]map[string]string{}
	if err := json.Unmarshal([]byte(encryptedAccounts), &accountMap); err != nil {
		return err
	}
	decryptor := intctrlutil.NewEncryptor(viper.GetString(constant.CfgKeyDPEncryptionKey))
	createSecret := func(secretName string, labels map[string]string, accountName, encryptedPassword string) error {
		password, err := decryptor.Decrypt([]byte(encryptedPassword))
		if err != nil {
			return err
		}
		labels[restoredSystemAccountLabel] = accountName
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: cluster.Namespace,
				Labels:    labels,
				Annotations: map[string]string{
					constant.SystemAccountProvisionedAnnotationKey: "true",
				},
			},
			Data: map[string][]byte{
				constant.AccountNameForSecret:   []byte(accountName),
				constant.AccountPasswdForSecret: []byte(password),
			},
		}
		current := &corev1.Secret{}
		key := client.ObjectKeyFromObject(secret)
		if err := cli.Get(reqCtx.Ctx, key, current); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			if err = cli.Create(reqCtx.Ctx, secret); err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
		} else {
			patch := client.MergeFrom(current.DeepCopy())
			current.Labels = secret.Labels
			current.Annotations = secret.Annotations
			current.Data = secret.Data
			if err = cli.Patch(reqCtx.Ctx, current, patch); err != nil {
				return err
			}
		}
		return nil
	}
	for i := range cluster.Spec.ComponentSpecs {
		comp := &cluster.Spec.ComponentSpecs[i]
		for accountName, encryptedPassword := range accountMap[comp.Name] {
			secretName := constant.GenerateAccountSecretName(cluster.Name, comp.Name, accountName)
			if err := createSecret(secretName, constant.GetCompLabels(cluster.Name, comp.Name), accountName, encryptedPassword); err != nil {
				return err
			}
		}
	}
	for i := range cluster.Spec.Shardings {
		sharding := &cluster.Spec.Shardings[i]
		for accountName, encryptedPassword := range accountMap[sharding.Name] {
			labels := constant.GetClusterLabels(cluster.Name, map[string]string{constant.KBAppShardingNameLabelKey: sharding.Name})
			if err := createSecret(shardingAccountSecretName(cluster.Name, sharding.Name, accountName), labels, accountName, encryptedPassword); err != nil {
				return err
			}
		}
	}
	return nil
}

func shardingAccountSecretName(cluster, sharding, account string) string {
	return constant.ShortenKubeName(fmt.Sprintf("%s-%s-%s", cluster, sharding, account), constant.KubeNameMaxLength)
}

// normalizeSchedulePolicy normalizes the schedule policy of the new cluster.
func (r RestoreOpsHandler) normalizeSchedulePolicy(cluster *appsv1.Cluster, schedulePolicy *appsv1.SchedulingPolicy) {
	if schedulePolicy == nil {
		return
	}
	updateLabelSelector := func(selector *metav1.LabelSelector) {
		if selector == nil {
			return
		}
		if _, ok := selector.MatchLabels[constant.AppInstanceLabelKey]; ok {
			selector.MatchLabels[constant.AppInstanceLabelKey] = cluster.Name
		}
		for i := range selector.MatchExpressions {
			matchExpression := &selector.MatchExpressions[i]
			if matchExpression.Key == constant.AppInstanceLabelKey {
				matchExpression.Values = []string{cluster.Name}
			}
		}
	}
	for i := range schedulePolicy.TopologySpreadConstraints {
		updateLabelSelector(schedulePolicy.TopologySpreadConstraints[i].LabelSelector)
	}
	if schedulePolicy.Affinity == nil {
		return
	}
	updatePodAffinityTerm := func(pats []corev1.PodAffinityTerm, wpats []corev1.WeightedPodAffinityTerm) {
		for i := range pats {
			podAffinityTerm := &pats[i]
			updateLabelSelector(podAffinityTerm.LabelSelector)
		}
		for i := range wpats {
			wpat := &wpats[i]
			updateLabelSelector(wpat.PodAffinityTerm.LabelSelector)
		}
	}
	if schedulePolicy.Affinity.PodAntiAffinity != nil {
		updatePodAffinityTerm(schedulePolicy.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			schedulePolicy.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution)
	}
	if schedulePolicy.Affinity.PodAffinity != nil {
		updatePodAffinityTerm(schedulePolicy.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			schedulePolicy.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution)
	}
}

func (r RestoreOpsHandler) rebuildShardAccountSecrets(cluster *appsv1.Cluster) {
	if len(cluster.Spec.Shardings) == 0 {
		return
	}
	for i := range cluster.Spec.Shardings {
		shardingSpec := &cluster.Spec.Shardings[i]
		template := &shardingSpec.Template
		for j := range template.SystemAccounts {
			account := &template.SystemAccounts[j]
			account.SecretRef = nil
		}
	}
}
