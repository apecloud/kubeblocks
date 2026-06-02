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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
)

type RestoreOpsHandler struct{}

var _ OpsHandler = RestoreOpsHandler{}

const (
	restoreFailureGateRequeueAfter   = 30 * time.Second
	restoreCRPreCreateFailureTimeout = 5 * time.Minute
)

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
//
// Failure-gate ordering:
//
//  1. cluster.Status.Phase == Running -> OpsSucceedPhase. The cluster
//     Running phase is authoritative for restore success even when a
//     Restore CR is still in progress.
//  2. cluster.IsDeleting() -> OpsFailedPhase. A deleting cluster is a
//     terminal restore failure regardless of Restore CR state.
//  3. cluster.Status.Phase == Failed -> consult Restore CRs through the
//     restore-aware failure gate. A transiently Failed cluster while
//     Restore CRs are still running is not a terminal restore failure;
//     it can recover once the restore workflow completes. The failure
//     gate distinguishes:
//     - any Restore CR Failed: terminal restore failure.
//     - all Restore CRs Completed but cluster still Failed: terminal
//     restore failure.
//     - some Restore CR still in progress: keep OpsRequest Running.
//     - empty Restore CR list: short pre-create window only; after the
//     bounded window expires it is a terminal restore failure.
//     - Restore CR list API error: non-terminal explicit error,
//     controller-runtime will requeue.
//  4. otherwise -> OpsRunningPhase.
func (r RestoreOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	opsRequest := opsRes.OpsRequest
	clusterDef := opsRequest.Spec.GetClusterName()

	// get cluster
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

	// Step 1: cluster Running -> Succeed (success contract preserved).
	if cluster.Status.Phase == appsv1.RunningClusterPhase {
		return opsv1alpha1.OpsSucceedPhase, 0, nil
	}
	// Step 2: cluster IsDeleting -> Failed (existing deleting contract).
	if cluster.IsDeleting() {
		return opsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("restore failed")
	}
	// Step 3: cluster Failed -> D restore-semantic-aware failure gate.
	if cluster.Status.Phase == appsv1.FailedClusterPhase {
		restoreCRs, listErr := r.listRestoreCRsForRestoreOps(reqCtx.Ctx, cli, opsRequest, cluster)
		if listErr != nil {
			// List API error: non-terminal + explicit error so the controller
			// re-queues loudly. Avoids silent allow.
			return opsv1alpha1.OpsRunningPhase, 0, fmt.Errorf("list Restore CRs for restore failure gate failed: %w", listErr)
		}
		if len(restoreCRs) == 0 {
			if r.restorePreCreateWindowExpired(opsRequest) {
				return opsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("restore failed: no Restore CRs found within %s after OpsRequest started", restoreCRPreCreateFailureTimeout)
			}
			// Empty list: possibly a normal pre-create window. Non-terminal
			// retry with bounded backoff plus an explicit log so the state is
			// observable. Not silent Running, not terminal Failed.
			reqCtx.Log.Info("restore failure-gate: cluster Failed but no Restore CRs found yet; requeue",
				"cluster", cluster.Name, "namespace", cluster.Namespace, "timeout", restoreCRPreCreateFailureTimeout.String())
			return opsv1alpha1.OpsRunningPhase, restoreFailureGateRequeueAfter, nil
		}
		anyRestoreFailed := false
		allRestoresCompleted := true
		for i := range restoreCRs {
			switch restoreCRs[i].Status.Phase {
			case dpv1alpha1.RestorePhaseFailed:
				anyRestoreFailed = true
			case dpv1alpha1.RestorePhaseCompleted:
				// counts toward all-completed
			default:
				// Running, AsDataSource, or empty -> not yet terminal-completed.
				allRestoresCompleted = false
			}
		}
		if anyRestoreFailed {
			return opsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("restore failed")
		}
		if allRestoresCompleted {
			return opsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("restore failed")
		}
		// Race case: at least one Restore CR is still in progress; cluster
		// transient Failed during restore is NOT a terminal restore failure.
		return opsv1alpha1.OpsRunningPhase, 0, nil
	}
	// Step 4: other cluster phases -> Running.
	return opsv1alpha1.OpsRunningPhase, 0, nil
}

// listRestoreCRsForRestoreOps returns the Restore CRs that belong to the
// cluster being restored by this OpsRequest. It is used by the restore
// failure gate to distinguish "in-progress restore" from "terminal restore
// failure" when the cluster Status.Phase is Failed.
//
// Lookup convention: same namespace as the cluster, first label-selected by
// `app.kubernetes.io/instance=<cluster.Name>`, then scoped to the current
// restore run. Current Restore CRs generated from the restore annotation carry
// the current cluster UID prefix as a hyphen-delimited name segment. The start
// timestamp guard prevents older Restore CRs for the same cluster name from
// deciding the current OpsRequest outcome.
func (r RestoreOpsHandler) listRestoreCRsForRestoreOps(
	ctx context.Context,
	cli client.Client,
	opsRequest *opsv1alpha1.OpsRequest,
	cluster *appsv1.Cluster,
) ([]dpv1alpha1.Restore, error) {
	restoreList := &dpv1alpha1.RestoreList{}
	if err := cli.List(ctx, restoreList,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{constant.AppInstanceLabelKey: cluster.Name},
	); err != nil {
		return nil, err
	}
	restoreCRs := make([]dpv1alpha1.Restore, 0, len(restoreList.Items))
	for i := range restoreList.Items {
		if r.restoreCRBelongsToRestoreOps(&restoreList.Items[i], opsRequest, cluster) {
			restoreCRs = append(restoreCRs, restoreList.Items[i])
		}
	}
	return restoreCRs, nil
}

func (r RestoreOpsHandler) restoreCRBelongsToRestoreOps(
	restoreCR *dpv1alpha1.Restore,
	opsRequest *opsv1alpha1.OpsRequest,
	cluster *appsv1.Cluster,
) bool {
	if opsName := restoreCR.Labels[constant.OpsRequestNameLabelKey]; opsName != "" {
		return opsName == opsRequest.Name
	}
	if !restoreNameContainsClusterUIDPrefix(restoreCR.Name, cluster) {
		return false
	}
	if startTime, ok := restoreFailureGateStartTime(opsRequest); ok {
		start := metav1.Time{Time: startTime}
		if restoreCR.CreationTimestamp.Before(&start) {
			return false
		}
	}
	return true
}

func (r RestoreOpsHandler) restorePreCreateWindowExpired(opsRequest *opsv1alpha1.OpsRequest) bool {
	startTime, ok := restoreFailureGateStartTime(opsRequest)
	return ok && time.Now().After(startTime.Add(restoreCRPreCreateFailureTimeout))
}

func restoreFailureGateStartTime(opsRequest *opsv1alpha1.OpsRequest) (time.Time, bool) {
	if !opsRequest.Status.StartTimestamp.IsZero() {
		return opsRequest.Status.StartTimestamp.Time, true
	}
	if !opsRequest.CreationTimestamp.IsZero() {
		return opsRequest.CreationTimestamp.Time, true
	}
	return time.Time{}, false
}

func restoreClusterUIDPrefix(cluster *appsv1.Cluster) string {
	uid := string(cluster.UID)
	if len(uid) > 8 {
		return uid[:8]
	}
	return uid
}

func restoreNameContainsClusterUIDPrefix(restoreName string, cluster *appsv1.Cluster) bool {
	uidPrefix := restoreClusterUIDPrefix(cluster)
	return uidPrefix == "" || strings.Contains(restoreName, "-"+uidPrefix+"-")
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
	// set the restore annotation to cluster
	restoreAnnotation, err := restore.GetRestoreFromBackupAnnotation(backup, restoreSpec.VolumeRestorePolicy, restoreSpec.RestorePointInTime,
		restoreSpec.Env, restoreSpec.DeferPostReadyUntilClusterRunning, restoreSpec.Parameters)
	if err != nil {
		return nil, err
	}
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	cluster.Annotations[constant.RestoreFromBackupAnnotationKey] = restoreAnnotation
	cluster.Name = opsRequest.Spec.GetClusterName()
	cluster.Namespace = opsRequest.Namespace
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
