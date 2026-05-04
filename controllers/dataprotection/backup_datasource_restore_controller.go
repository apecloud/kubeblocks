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

package dataprotection

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
)

const (
	backupDataSourceRestoreLabelCluster   = "dataprotection.kubeblocks.io/target-cluster"
	backupDataSourceRestoreLabelComponent = "dataprotection.kubeblocks.io/target-component"
	backupDataSourceRestoreLabelBackup    = "dataprotection.kubeblocks.io/source-backup"
)

// BackupDataSourceRestoreReconciler reconciles Cluster-level restore sessions
// described by PVC dataSourceRef.kind=Backup and DP restore-options.
type BackupDataSourceRestoreReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

type backupDataSourcePVC struct {
	pvc     *corev1.PersistentVolumeClaim
	backup  *dpv1alpha1.Backup
	options dprestore.RestoreOptions
}

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/status,verbs=get;patch;update
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;watch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=restores,verbs=get;list;watch;create;patch;update

func (r *BackupDataSourceRestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("backup-datasource-restore", req.NamespacedName),
		Recorder: r.Recorder,
	}

	cluster := &appsv1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if cluster.IsDeleting() {
		return intctrlutil.Reconciled()
	}
	if !clusterHasBackupDataSource(cluster) {
		restores, err := r.listInternalPostReadyRestores(reqCtx, cluster)
		if err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		if len(restores) == 0 {
			return intctrlutil.Reconciled()
		}
		return r.updateClusterConditionFromRestores(reqCtx, cluster, restores)
	}

	items, err := r.listBackupDataSourcePVCs(reqCtx, cluster)
	if err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	if len(items) == 0 {
		if err := r.patchClusterRestoreCondition(reqCtx, cluster, metav1.ConditionFalse, string(dpv1alpha1.RestorePhaseRunning), "waiting for Backup dataSource PVCs"); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	allPopulated := true
	for i := range items {
		item := items[i]
		if item.pvc.Spec.VolumeName == "" {
			restoreMgr, err := r.buildRestoreManager(reqCtx, item)
			if err != nil {
				if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
					_ = r.patchClusterRestoreCondition(reqCtx, cluster, metav1.ConditionFalse, string(dpv1alpha1.RestorePhaseFailed), err.Error())
					r.Recorder.Event(cluster, corev1.EventTypeWarning, dprestore.ReasonRestoreFailed, err.Error())
					return intctrlutil.Reconciled()
				}
				return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
			}
			populator := r.volumePopulator()
			if err = populator.Populate(reqCtx, item.pvc, restoreMgr); err != nil {
				if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
					_ = r.patchClusterRestoreCondition(reqCtx, cluster, metav1.ConditionFalse, string(dpv1alpha1.RestorePhaseFailed), err.Error())
					r.Recorder.Event(cluster, corev1.EventTypeWarning, dprestore.ReasonRestoreFailed, err.Error())
					return intctrlutil.Reconciled()
				}
				return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
			}
			allPopulated = false
			continue
		}
		if err = r.volumePopulator().Cleanup(reqCtx, item.pvc); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
	}

	if !allPopulated {
		if err := r.patchClusterRestoreCondition(reqCtx, cluster, metav1.ConditionFalse, string(dpv1alpha1.RestorePhaseRunning), "populating Backup dataSource PVCs"); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	return r.reconcilePostReady(reqCtx, cluster, items)
}

func (r *BackupDataSourceRestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&appsv1.Cluster{}).
		Watches(&appsv1.Component{}, handler.EnqueueRequestsFromMapFunc(r.mapComponentToCluster)).
		Watches(&corev1.PersistentVolumeClaim{}, handler.EnqueueRequestsFromMapFunc(r.mapPVCToCluster)).
		Watches(&batchv1.Job{}, handler.EnqueueRequestsFromMapFunc(r.mapJobToCluster)).
		Watches(&dpv1alpha1.Restore{}, handler.EnqueueRequestsFromMapFunc(r.mapRestoreToCluster)).
		Complete(r)
}

func (r *BackupDataSourceRestoreReconciler) volumePopulator() *VolumePopulatorReconciler {
	return &VolumePopulatorReconciler{
		Client:   r.Client,
		Scheme:   r.Scheme,
		Recorder: r.Recorder,
	}
}

func (r *BackupDataSourceRestoreReconciler) listBackupDataSourcePVCs(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) ([]backupDataSourcePVC, error) {
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := r.Client.List(reqCtx.Ctx, pvcs,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{constant.AppInstanceLabelKey: cluster.Name}); err != nil {
		return nil, err
	}
	var items []backupDataSourcePVC
	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		if !dprestore.IsBackupDataSourceRef(pvc.Spec.DataSourceRef) {
			continue
		}
		options, err := dprestore.ParseRestoreOptions(pvc.Annotations)
		if err != nil {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf("failed to parse restore options from PVC %s/%s: %v", pvc.Namespace, pvc.Name, err))
		}
		backupNamespace := options.BackupNamespace
		if backupNamespace == "" {
			backupNamespace = pvc.Namespace
		}
		backup := &dpv1alpha1.Backup{}
		if err = r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: pvc.Spec.DataSourceRef.Name, Namespace: backupNamespace}, backup); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, intctrlutil.NewFatalError(err.Error())
			}
			return nil, err
		}
		items = append(items, backupDataSourcePVC{pvc: pvc, backup: backup, options: options})
	}
	return items, nil
}

func (r *BackupDataSourceRestoreReconciler) buildRestoreManager(reqCtx intctrlutil.RequestCtx, item backupDataSourcePVC) (*dprestore.RestoreManager, error) {
	requiredPolicy := requiredPolicyForSourcePod(item.options.SourceTargetPodName)
	restore := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constant.ShortenKubeName(fmt.Sprintf("%s-backup-ds", item.pvc.Name), constant.KubeNameMaxLength),
			Namespace: item.pvc.Namespace,
		},
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:             item.backup.Name,
				Namespace:        item.backup.Namespace,
				SourceTargetName: item.options.SourceTargetName,
			},
			RestoreTime: item.options.RestoreTime,
			Env:         item.options.Env,
			Parameters:  item.options.Parameters,
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
				DataSourceRef:                    ptrVolumeConfig(item.options.VolumeConfig()),
				RequiredPolicyForAllPodSelection: requiredPolicy,
				VolumeClaimRestorePolicy:         item.options.VolumeRestorePolicy,
			},
		},
	}
	restoreMgr := dprestore.NewRestoreManager(restore, r.Recorder, r.Scheme, r.Client)
	if err := dprestore.ValidateAndInitRestoreMGR(reqCtx, r.Client, restoreMgr); err != nil {
		return nil, err
	}
	saName, err := EnsureWorkerServiceAccount(reqCtx, r.Client, restore.Namespace, nil)
	if err != nil {
		return nil, err
	}
	restoreMgr.WorkerServiceAccount = saName
	return restoreMgr, nil
}

func (r *BackupDataSourceRestoreReconciler) cleanupClusterRestoreInputs(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) error {
	patch := client.MergeFrom(cluster.DeepCopy())
	changed := false
	cleanup := func(vct *appsv1.PersistentVolumeClaimTemplate) {
		if dprestore.IsBackupDataSourceRef(vct.Spec.DataSourceRef) {
			vct.Spec.DataSourceRef = nil
			changed = true
		}
		if vct.Annotations != nil {
			if _, ok := vct.Annotations[dptypes.RestoreOptionsAnnotationKey]; ok {
				delete(vct.Annotations, dptypes.RestoreOptionsAnnotationKey)
				changed = true
			}
		}
	}
	for i := range cluster.Spec.ComponentSpecs {
		for j := range cluster.Spec.ComponentSpecs[i].VolumeClaimTemplates {
			cleanup(&cluster.Spec.ComponentSpecs[i].VolumeClaimTemplates[j])
		}
	}
	for i := range cluster.Spec.Shardings {
		for j := range cluster.Spec.Shardings[i].Template.VolumeClaimTemplates {
			cleanup(&cluster.Spec.Shardings[i].Template.VolumeClaimTemplates[j])
		}
		for j := range cluster.Spec.Shardings[i].ShardTemplates {
			for k := range cluster.Spec.Shardings[i].ShardTemplates[j].VolumeClaimTemplates {
				cleanup(&cluster.Spec.Shardings[i].ShardTemplates[j].VolumeClaimTemplates[k])
			}
		}
	}
	if !changed {
		return nil
	}
	return r.Client.Patch(reqCtx.Ctx, cluster, patch)
}

func (r *BackupDataSourceRestoreReconciler) reconcilePostReady(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster, items []backupDataSourcePVC) (ctrl.Result, error) {
	for i := range items {
		if items[i].options.DeferPostReadyUntilClusterRunning && cluster.Status.Phase != appsv1.RunningClusterPhase {
			if err := r.patchClusterRestoreCondition(reqCtx, cluster, metav1.ConditionFalse, string(dpv1alpha1.RestorePhaseRunning), "waiting for target Cluster running before post-ready restore"); err != nil {
				return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
			}
			return intctrlutil.Reconciled()
		}
	}
	restores, err := r.ensureInternalPostReadyRestores(reqCtx, cluster, items)
	if err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	if err = r.cleanupClusterRestoreInputs(reqCtx, cluster); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	return r.updateClusterConditionFromRestores(reqCtx, cluster, restores)
}

func (r *BackupDataSourceRestoreReconciler) updateClusterConditionFromRestores(reqCtx intctrlutil.RequestCtx,
	cluster *appsv1.Cluster,
	restores []*dpv1alpha1.Restore) (ctrl.Result, error) {
	allCompleted := true
	var err error
	for i := range restores {
		restore := restores[i]
		switch restore.Status.Phase {
		case dpv1alpha1.RestorePhaseCompleted:
			continue
		case dpv1alpha1.RestorePhaseFailed:
			msg := fmt.Sprintf("post-ready restore %s failed", restore.Name)
			if cond := meta.FindStatusCondition(restore.Status.Conditions, dprestore.ConditionTypeRestorePostReady); cond != nil && cond.Message != "" {
				msg = cond.Message
			}
			if err = r.patchClusterRestoreCondition(reqCtx, cluster, metav1.ConditionFalse, string(dpv1alpha1.RestorePhaseFailed), msg); err != nil {
				return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
			}
			return intctrlutil.Reconciled()
		default:
			allCompleted = false
		}
	}
	if allCompleted {
		if err = r.patchClusterRestoreCondition(reqCtx, cluster, metav1.ConditionTrue, string(dpv1alpha1.RestorePhaseCompleted), "restore completed"); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}
	if err = r.patchClusterRestoreCondition(reqCtx, cluster, metav1.ConditionFalse, string(dpv1alpha1.RestorePhaseRunning), "waiting for post-ready restore"); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *BackupDataSourceRestoreReconciler) listInternalPostReadyRestores(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) ([]*dpv1alpha1.Restore, error) {
	restoreList := &dpv1alpha1.RestoreList{}
	if err := r.Client.List(reqCtx.Ctx, restoreList,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{backupDataSourceRestoreLabelCluster: cluster.Name}); err != nil {
		return nil, err
	}
	restores := make([]*dpv1alpha1.Restore, 0, len(restoreList.Items))
	for i := range restoreList.Items {
		restores = append(restores, &restoreList.Items[i])
	}
	return restores, nil
}

func (r *BackupDataSourceRestoreReconciler) ensureInternalPostReadyRestores(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster, items []backupDataSourcePVC) ([]*dpv1alpha1.Restore, error) {
	seen := map[string]struct{}{}
	var restores []*dpv1alpha1.Restore
	for i := range items {
		item := items[i]
		componentName := item.pvc.Labels[constant.KBAppComponentLabelKey]
		if componentName == "" {
			continue
		}
		key := fmt.Sprintf("%s/%s/%s", componentName, item.backup.Namespace, item.backup.Name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		restore := r.buildInternalPostReadyRestore(cluster, item, componentName)
		existing := &dpv1alpha1.Restore{}
		if err := r.Client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(restore), existing); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
			if err = controllerutil.SetControllerReference(cluster, restore, r.Scheme); err != nil {
				return nil, err
			}
			if err = r.Client.Create(reqCtx.Ctx, restore); err != nil && !apierrors.IsAlreadyExists(err) {
				return nil, err
			}
			restores = append(restores, restore)
			continue
		}
		restores = append(restores, existing)
	}
	return restores, nil
}

func (r *BackupDataSourceRestoreReconciler) buildInternalPostReadyRestore(cluster *appsv1.Cluster, item backupDataSourcePVC, componentName string) *dpv1alpha1.Restore {
	podSelector := metav1.LabelSelector{
		MatchLabels: constant.GetCompLabels(cluster.Name, componentName),
	}
	readyConfig := &dpv1alpha1.ReadyConfig{
		ExecAction: &dpv1alpha1.ExecAction{
			Target: dpv1alpha1.ExecActionTarget{
				PodSelector: podSelector,
			},
		},
		JobAction: &dpv1alpha1.JobAction{
			RequiredPolicyForAllPodSelection: requiredPolicyForSourcePod(item.options.SourceTargetPodName),
			Target: dpv1alpha1.JobActionTarget{
				PodSelector: dpv1alpha1.PodSelector{
					LabelSelector: &podSelector,
				},
			},
		},
		ConnectionCredential: r.connectionCredential(cluster, componentName),
	}
	target := dputils.GetBackupStatusTarget(item.backup, item.options.SourceTargetName)
	if target != nil {
		readyConfig.JobAction.Target.PodSelector.Strategy = target.PodSelector.Strategy
	}
	if item.backup.Status.BackupMethod != nil && item.backup.Status.BackupMethod.TargetVolumes != nil {
		readyConfig.JobAction.Target.VolumeMounts = item.backup.Status.BackupMethod.TargetVolumes.VolumeMounts
	}
	name := constant.ShortenKubeName(fmt.Sprintf("%s-%s-%s-post-ready", cluster.Name, componentName, item.backup.Name), constant.KubeNameMaxLength)
	return &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				backupDataSourceRestoreLabelCluster:   cluster.Name,
				backupDataSourceRestoreLabelComponent: componentName,
				backupDataSourceRestoreLabelBackup:    item.backup.Name,
				constant.AppManagedByLabelKey:         dptypes.AppName,
			},
		},
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:             item.backup.Name,
				Namespace:        item.backup.Namespace,
				SourceTargetName: item.options.SourceTargetName,
			},
			RestoreTime: item.options.RestoreTime,
			Env:         item.options.Env,
			Parameters:  item.options.Parameters,
			ReadyConfig: readyConfig,
		},
	}
}

func (r *BackupDataSourceRestoreReconciler) connectionCredential(cluster *appsv1.Cluster, componentName string) *dpv1alpha1.ConnectionCredential {
	for i := range cluster.Spec.ComponentSpecs {
		comp := &cluster.Spec.ComponentSpecs[i]
		if comp.Name != componentName || len(comp.SystemAccounts) == 0 {
			continue
		}
		return &dpv1alpha1.ConnectionCredential{
			SecretName:  constant.GenerateAccountSecretName(cluster.Name, componentName, comp.SystemAccounts[0].Name),
			PasswordKey: constant.AccountPasswdForSecret,
			UsernameKey: constant.AccountNameForSecret,
		}
	}
	return nil
}

func (r *BackupDataSourceRestoreReconciler) patchClusterRestoreCondition(reqCtx intctrlutil.RequestCtx,
	cluster *appsv1.Cluster,
	status metav1.ConditionStatus,
	reason string,
	message string) error {
	latest := &appsv1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(cluster), latest); err != nil {
		return err
	}
	patch := client.MergeFrom(latest.DeepCopy())
	meta.SetStatusCondition(&latest.Status.Conditions, metav1.Condition{
		Type:               dptypes.RestoreSessionConditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
	return r.Client.Status().Patch(reqCtx.Ctx, latest, patch)
}

func (r *BackupDataSourceRestoreReconciler) mapPVCToCluster(ctx context.Context, object client.Object) []reconcile.Request {
	clusterName := object.GetLabels()[constant.AppInstanceLabelKey]
	if clusterName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: object.GetNamespace(), Name: clusterName}}}
}

func (r *BackupDataSourceRestoreReconciler) mapComponentToCluster(ctx context.Context, object client.Object) []reconcile.Request {
	clusterName := object.GetLabels()[constant.AppInstanceLabelKey]
	if clusterName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: object.GetNamespace(), Name: clusterName}}}
}

func (r *BackupDataSourceRestoreReconciler) mapJobToCluster(ctx context.Context, object client.Object) []reconcile.Request {
	for _, ref := range object.GetOwnerReferences() {
		if ref.Kind != "PersistentVolumeClaim" || ref.Name == "" {
			continue
		}
		pvc := &corev1.PersistentVolumeClaim{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: object.GetNamespace(), Name: ref.Name}, pvc); err != nil {
			return nil
		}
		return r.mapPVCToCluster(ctx, pvc)
	}
	return nil
}

func (r *BackupDataSourceRestoreReconciler) mapRestoreToCluster(ctx context.Context, object client.Object) []reconcile.Request {
	clusterName := object.GetLabels()[backupDataSourceRestoreLabelCluster]
	if clusterName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: object.GetNamespace(), Name: clusterName}}}
}

func clusterHasBackupDataSource(cluster *appsv1.Cluster) bool {
	has := func(vcts []appsv1.PersistentVolumeClaimTemplate) bool {
		for i := range vcts {
			if dprestore.IsBackupDataSourceRef(vcts[i].Spec.DataSourceRef) {
				return true
			}
		}
		return false
	}
	for i := range cluster.Spec.ComponentSpecs {
		if has(cluster.Spec.ComponentSpecs[i].VolumeClaimTemplates) {
			return true
		}
	}
	for i := range cluster.Spec.Shardings {
		if has(cluster.Spec.Shardings[i].Template.VolumeClaimTemplates) {
			return true
		}
		for j := range cluster.Spec.Shardings[i].ShardTemplates {
			if has(cluster.Spec.Shardings[i].ShardTemplates[j].VolumeClaimTemplates) {
				return true
			}
		}
	}
	return false
}

func requiredPolicyForSourcePod(sourcePodName string) *dpv1alpha1.RequiredPolicyForAllPodSelection {
	if sourcePodName == "" {
		return &dpv1alpha1.RequiredPolicyForAllPodSelection{
			DataRestorePolicy: dpv1alpha1.OneToOneRestorePolicy,
		}
	}
	return &dpv1alpha1.RequiredPolicyForAllPodSelection{
		DataRestorePolicy: dpv1alpha1.OneToManyRestorePolicy,
		SourceOfOneToMany: &dpv1alpha1.SourceOfOneToMany{
			TargetPodName: sourcePodName,
		},
	}
}

func ptrVolumeConfig(v dpv1alpha1.VolumeConfig) *dpv1alpha1.VolumeConfig {
	return &v
}
