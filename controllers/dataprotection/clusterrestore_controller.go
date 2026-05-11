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

package dataprotection

import (
	"context"
	"encoding/json"
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
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	backupDataSourceRestoreLabelCluster   = "dataprotection.kubeblocks.io/target-cluster"
	backupDataSourceRestoreLabelComponent = "dataprotection.kubeblocks.io/target-component"
	backupDataSourceRestoreLabelBackup    = "dataprotection.kubeblocks.io/source-backup"
)

// ClusterRestoreReconciler reconciles ClusterRestore orchestration sessions.
type ClusterRestoreReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

type backupDataSourcePVC struct {
	pvc     *corev1.PersistentVolumeClaim
	backup  *dpv1alpha1.Backup
	options backupDataSourceContext
}

type backupDataSourceContext struct {
	restoreTime                       string
	volumeSource                      string
	mountPath                         string
	sourceTargetName                  string
	sourceTargetPodName               string
	volumeRestorePolicy               dpv1alpha1.VolumeClaimRestorePolicy
	deferPostReadyUntilClusterRunning bool
	env                               []corev1.EnvVar
	parameters                        []dpv1alpha1.ParameterPair
}

func (o backupDataSourceContext) volumeConfig() dpv1alpha1.VolumeConfig {
	return dpv1alpha1.VolumeConfig{
		VolumeSource: o.volumeSource,
		MountPath:    o.mountPath,
	}
}

func (o *backupDataSourceContext) defaultInplace() {
	if o.volumeRestorePolicy == "" {
		o.volumeRestorePolicy = dpv1alpha1.VolumeClaimRestorePolicyParallel
	}
}

func (o backupDataSourceContext) validate() error {
	switch o.volumeRestorePolicy {
	case "", dpv1alpha1.VolumeClaimRestorePolicyParallel, dpv1alpha1.VolumeClaimRestorePolicySerial:
	default:
		return fmt.Errorf("unsupported volumeRestorePolicy %q", o.volumeRestorePolicy)
	}
	return nil
}

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=clusterrestores,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=clusterrestores/status,verbs=get;patch;update
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;watch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=restores,verbs=get;list;watch;create;patch;update

func (r *ClusterRestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("cluster-restore", req.NamespacedName),
		Recorder: r.Recorder,
	}

	clusterRestore := &dpv1alpha1.ClusterRestore{}
	if err := r.Client.Get(ctx, req.NamespacedName, clusterRestore); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if !clusterRestore.DeletionTimestamp.IsZero() {
		return intctrlutil.Reconciled()
	}
	if isClusterRestoreTerminal(clusterRestore.Status.Phase) {
		return intctrlutil.Reconciled()
	}
	if clusterRestore.Status.Phase == "" {
		if err := r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhasePending, metav1.ConditionFalse, "Pending", "waiting to start cluster restore", nil); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
	}

	backup, restoreTime, err := r.getAndValidateBackup(reqCtx, clusterRestore)
	if err != nil {
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
			_ = r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseFailed, metav1.ConditionFalse, "Failed", err.Error(), nil)
			r.Recorder.Event(clusterRestore, corev1.EventTypeWarning, dprestore.ReasonRestoreFailed, err.Error())
			return intctrlutil.Reconciled()
		}
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	target := &appsv1.Cluster{}
	targetKey := types.NamespacedName{Namespace: clusterRestore.Namespace, Name: clusterRestore.Spec.TargetClusterName}
	if err = r.Client.Get(ctx, targetKey, target); err != nil {
		if !apierrors.IsNotFound(err) {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		target, err = r.buildTargetCluster(reqCtx, clusterRestore, backup, restoreTime)
		if err != nil {
			if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
				_ = r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseFailed, metav1.ConditionFalse, "Failed", err.Error(), nil)
				r.Recorder.Event(clusterRestore, corev1.EventTypeWarning, dprestore.ReasonRestoreFailed, err.Error())
				return intctrlutil.Reconciled()
			}
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		if err = r.Client.Create(ctx, target); err != nil {
			if apierrors.IsInvalid(err) {
				msg := fmt.Sprintf("failed to create target Cluster %s/%s: %s", target.Namespace, target.Name, err.Error())
				if statusErr := r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseFailed, metav1.ConditionFalse, "Failed", msg, nil); statusErr != nil {
					return intctrlutil.RequeueWithError(statusErr, reqCtx.Log, "")
				}
				r.Recorder.Event(clusterRestore, corev1.EventTypeWarning, dprestore.ReasonRestoreFailed, msg)
				return intctrlutil.Reconciled()
			}
			if !apierrors.IsAlreadyExists(err) {
				return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
			}
			existing := &appsv1.Cluster{}
			if getErr := r.Client.Get(ctx, targetKey, existing); getErr != nil {
				return intctrlutil.RequeueWithError(getErr, reqCtx.Log, "")
			}
			if !isClusterRestoreTarget(clusterRestore, existing) {
				msg := fmt.Sprintf("target Cluster %s/%s already exists and is not created by ClusterRestore %s/%s", existing.Namespace, existing.Name, clusterRestore.Namespace, clusterRestore.Name)
				if statusErr := r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseFailed, metav1.ConditionFalse, "Failed", msg, clusterRestoreTargetRef(existing)); statusErr != nil {
					return intctrlutil.RequeueWithError(statusErr, reqCtx.Log, "")
				}
				r.Recorder.Event(clusterRestore, corev1.EventTypeWarning, dprestore.ReasonRestoreFailed, msg)
				return intctrlutil.Reconciled()
			}
			target = existing
		}
		targetRef := clusterRestoreTargetRef(target)
		if err = r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseCreatingCluster, metav1.ConditionFalse, "CreatingCluster", "target Cluster is being created", targetRef); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}
	if !isClusterRestoreTarget(clusterRestore, target) {
		msg := fmt.Sprintf("target Cluster %s/%s already exists and is not created by ClusterRestore %s/%s", target.Namespace, target.Name, clusterRestore.Namespace, clusterRestore.Name)
		if err = r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseFailed, metav1.ConditionFalse, "Failed", msg, clusterRestoreTargetRef(target)); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		r.Recorder.Event(clusterRestore, corev1.EventTypeWarning, dprestore.ReasonRestoreFailed, msg)
		return intctrlutil.Reconciled()
	}
	targetRef := clusterRestoreTargetRef(target)

	if err = r.prepareSourceSystemAccounts(reqCtx, clusterRestore, target, backup); err != nil {
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
			_ = r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseFailed, metav1.ConditionFalse, "Failed", err.Error(), targetRef)
			r.Recorder.Event(clusterRestore, corev1.EventTypeWarning, dprestore.ReasonRestoreFailed, err.Error())
			return intctrlutil.Reconciled()
		}
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	items, err := r.listClusterRestorePVCs(reqCtx, clusterRestore)
	if err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	if len(items) == 0 {
		if clusterHasBackupDataSource(target) {
			if err = r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseRestoring, metav1.ConditionFalse, "Restoring", "waiting for Backup dataSource PVCs", targetRef); err != nil {
				return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
			}
			return intctrlutil.Reconciled()
		}
		restores, err := r.listInternalPostReadyRestores(reqCtx, clusterRestore)
		if err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		return r.updateClusterRestoreStatusFromRestores(reqCtx, clusterRestore, target, targetRef, restores)
	}

	allPopulated := true
	for i := range items {
		item := items[i]
		if item.pvc.Spec.VolumeName == "" {
			restoreMgr, err := r.buildRestoreManager(reqCtx, clusterRestore, item)
			if err != nil {
				if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
					_ = r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseFailed, metav1.ConditionFalse, "Failed", err.Error(), targetRef)
					r.Recorder.Event(clusterRestore, corev1.EventTypeWarning, dprestore.ReasonRestoreFailed, err.Error())
					return intctrlutil.Reconciled()
				}
				return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
			}
			if err = r.volumePopulator().Populate(reqCtx, item.pvc, restoreMgr); err != nil {
				if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
					_ = r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseFailed, metav1.ConditionFalse, "Failed", err.Error(), targetRef)
					r.Recorder.Event(clusterRestore, corev1.EventTypeWarning, dprestore.ReasonRestoreFailed, err.Error())
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
		if err = r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseRestoring, metav1.ConditionFalse, "Restoring", "populating Backup dataSource PVCs", targetRef); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	return r.reconcilePostReady(reqCtx, clusterRestore, target, targetRef, items)
}

func (r *ClusterRestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&dpv1alpha1.ClusterRestore{}).
		Watches(&appsv1.Cluster{}, handler.EnqueueRequestsFromMapFunc(r.mapClusterToClusterRestore)).
		Watches(&corev1.PersistentVolumeClaim{}, handler.EnqueueRequestsFromMapFunc(r.mapPVCToClusterRestore)).
		Watches(&batchv1.Job{}, handler.EnqueueRequestsFromMapFunc(r.mapJobToClusterRestore)).
		Watches(&dpv1alpha1.Restore{}, handler.EnqueueRequestsFromMapFunc(r.mapRestoreToClusterRestore)).
		Complete(r)
}

func (r *ClusterRestoreReconciler) volumePopulator() *VolumePopulatorReconciler {
	return &VolumePopulatorReconciler{
		Client:   r.Client,
		Scheme:   r.Scheme,
		Recorder: r.Recorder,
	}
}

func (r *ClusterRestoreReconciler) getAndValidateBackup(reqCtx intctrlutil.RequestCtx, clusterRestore *dpv1alpha1.ClusterRestore) (*dpv1alpha1.Backup, string, error) {
	backupNamespace := clusterRestore.Spec.BackupRef.Namespace
	if backupNamespace == "" {
		backupNamespace = clusterRestore.Namespace
	}
	backup := &dpv1alpha1.Backup{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: clusterRestore.Spec.BackupRef.Name, Namespace: backupNamespace}, backup); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, "", intctrlutil.NewFatalError(fmt.Sprintf("backup %s not found in namespace %s", clusterRestore.Spec.BackupRef.Name, backupNamespace))
		}
		return nil, "", err
	}
	backupType := backup.Labels[dptypes.BackupTypeLabelKey]
	if backup.Status.Phase != dpv1alpha1.BackupPhaseCompleted && backupType != string(dpv1alpha1.BackupTypeContinuous) {
		return nil, "", intctrlutil.NewFatalError(fmt.Sprintf("backup %s status is %s, only completed backup can be used to restore", backup.Name, backup.Status.Phase))
	}
	restoreTime := clusterRestore.Spec.RestoreTime
	if backupType == string(dpv1alpha1.BackupTypeContinuous) {
		formatted, err := dprestore.FormatRestoreTimeAndValidate(restoreTime, backup)
		if err != nil {
			return nil, "", intctrlutil.NewFatalError(err.Error())
		}
		restoreTime = formatted
	}
	return backup, restoreTime, nil
}

func (r *ClusterRestoreReconciler) buildTargetCluster(reqCtx intctrlutil.RequestCtx, clusterRestore *dpv1alpha1.ClusterRestore, backup *dpv1alpha1.Backup, restoreTime string) (*appsv1.Cluster, error) {
	var cluster *appsv1.Cluster
	var err error
	if clusterRestore.Spec.TargetClusterTemplate != nil {
		cluster, err = clusterFromTargetTemplate(clusterRestore.Spec.TargetClusterTemplate)
		if err != nil {
			return nil, err
		}
	} else {
		cluster, err = clusterFromBackupSnapshot(backup)
		if err != nil {
			return nil, err
		}
	}
	normalizeRestoredCluster(cluster, clusterRestore)
	if err = injectClusterRestoreDataSources(cluster, clusterRestore, backup, restoreTime); err != nil {
		return nil, err
	}
	if err = r.prepareSourceSystemAccounts(reqCtx, clusterRestore, cluster, backup); err != nil {
		return nil, err
	}
	return cluster, nil
}

func clusterFromTargetTemplate(template *dpv1alpha1.ClusterRestoreTargetClusterTemplate) (*appsv1.Cluster, error) {
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      cloneStringMap(template.Labels),
			Annotations: cloneStringMap(template.Annotations),
		},
		Spec: *template.Spec.DeepCopy(),
	}
	return cluster, nil
}

func clusterFromBackupSnapshot(backup *dpv1alpha1.Backup) (*appsv1.Cluster, error) {
	clusterString := backup.Annotations[constant.ClusterSnapshotAnnotationKey]
	if clusterString == "" {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf("missing snapshot annotation in backup %s, %s is empty in Annotations", backup.Name, constant.ClusterSnapshotAnnotationKey))
	}
	cluster := &appsv1.Cluster{}
	if err := json.Unmarshal([]byte(clusterString), cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func normalizeRestoredCluster(cluster *appsv1.Cluster, clusterRestore *dpv1alpha1.ClusterRestore) {
	cluster.TypeMeta = metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: "Cluster"}
	cluster.Name = clusterRestore.Spec.TargetClusterName
	cluster.Namespace = clusterRestore.Namespace
	cluster.ResourceVersion = ""
	cluster.UID = ""
	cluster.Generation = 0
	cluster.CreationTimestamp = metav1.Time{}
	cluster.DeletionTimestamp = nil
	cluster.DeletionGracePeriodSeconds = nil
	cluster.ManagedFields = nil
	cluster.OwnerReferences = nil
	cluster.Finalizers = nil
	cluster.Status = appsv1.ClusterStatus{}
	if cluster.Labels == nil {
		cluster.Labels = map[string]string{}
	}
	cluster.Labels[dptypes.ClusterRestoreLabelKey] = clusterRestore.Name
	cluster.Labels[constant.AppManagedByLabelKey] = constant.AppName
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	cluster.Annotations[dptypes.ClusterRestoreUIDAnnotationKey] = string(clusterRestore.UID)

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
		comp := &cluster.Spec.ComponentSpecs[i]
		comp.OfflineInstances = nil
		comp.TLS = false
		comp.Issuer = nil
		for j := range comp.SystemAccounts {
			comp.SystemAccounts[j].SecretRef = nil
		}
	}
	for i := range cluster.Spec.Shardings {
		sharding := &cluster.Spec.Shardings[i]
		sharding.Offline = nil
		for j := range sharding.Template.SystemAccounts {
			sharding.Template.SystemAccounts[j].SecretRef = nil
		}
	}
	normalizeSchedulePolicy(cluster, cluster.Spec.SchedulingPolicy)
	for i := range cluster.Spec.ComponentSpecs {
		normalizeSchedulePolicy(cluster, cluster.Spec.ComponentSpecs[i].SchedulingPolicy)
	}
	for i := range cluster.Spec.Shardings {
		normalizeSchedulePolicy(cluster, cluster.Spec.Shardings[i].Template.SchedulingPolicy)
		for j := range cluster.Spec.Shardings[i].ShardTemplates {
			normalizeSchedulePolicy(cluster, cluster.Spec.Shardings[i].ShardTemplates[j].SchedulingPolicy)
		}
	}
}

func normalizeSchedulePolicy(cluster *appsv1.Cluster, schedulePolicy *appsv1.SchedulingPolicy) {
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
			updateLabelSelector(pats[i].LabelSelector)
		}
		for i := range wpats {
			updateLabelSelector(wpats[i].PodAffinityTerm.LabelSelector)
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

func injectClusterRestoreDataSources(cluster *appsv1.Cluster, clusterRestore *dpv1alpha1.ClusterRestore, backup *dpv1alpha1.Backup, restoreTime string) error {
	inject := func(ownerName string, vct *appsv1.PersistentVolumeClaimTemplate) error {
		ref := dprestore.BackupDataSourceRef(backup.Name)
		vct.Spec.DataSourceRef = ref
		if vct.Labels == nil {
			vct.Labels = map[string]string{}
		}
		vct.Labels[dptypes.ClusterRestoreLabelKey] = clusterRestore.Name
		vct.Labels[constant.AppManagedByLabelKey] = dptypes.AppName
		if vct.Annotations == nil {
			vct.Annotations = map[string]string{}
		}
		sourceTargetName := inferBackupSourceTargetName(backup, ownerName)
		vct.Annotations[dptypes.ClusterRestoreUIDAnnotationKey] = string(clusterRestore.UID)
		vct.Annotations[dptypes.VolumeSourceAnnotationKey] = vct.Name
		vct.Annotations[dptypes.SourceTargetNameAnnotationKey] = sourceTargetName
		vct.Annotations[dptypes.SourceTargetPodNameAnnotationKey] = ""
		options := backupDataSourceContext{
			restoreTime:                       restoreTime,
			volumeSource:                      vct.Name,
			sourceTargetName:                  sourceTargetName,
			volumeRestorePolicy:               clusterRestore.Spec.VolumeRestorePolicy,
			env:                               clusterRestore.Spec.Env,
			parameters:                        clusterRestore.Spec.Parameters,
			deferPostReadyUntilClusterRunning: clusterRestore.Spec.DeferPostReadyUntilClusterRunning,
		}
		options.defaultInplace()
		return options.validate()
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

func clusterHasBackupDataSource(cluster *appsv1.Cluster) bool {
	for i := range cluster.Spec.ComponentSpecs {
		for j := range cluster.Spec.ComponentSpecs[i].VolumeClaimTemplates {
			if dprestore.IsBackupDataSourceRef(cluster.Spec.ComponentSpecs[i].VolumeClaimTemplates[j].Spec.DataSourceRef) {
				return true
			}
		}
	}
	for i := range cluster.Spec.Shardings {
		for j := range cluster.Spec.Shardings[i].Template.VolumeClaimTemplates {
			if dprestore.IsBackupDataSourceRef(cluster.Spec.Shardings[i].Template.VolumeClaimTemplates[j].Spec.DataSourceRef) {
				return true
			}
		}
		for j := range cluster.Spec.Shardings[i].ShardTemplates {
			for k := range cluster.Spec.Shardings[i].ShardTemplates[j].VolumeClaimTemplates {
				if dprestore.IsBackupDataSourceRef(cluster.Spec.Shardings[i].ShardTemplates[j].VolumeClaimTemplates[k].Spec.DataSourceRef) {
					return true
				}
			}
		}
	}
	return false
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

func (r *ClusterRestoreReconciler) prepareSourceSystemAccounts(reqCtx intctrlutil.RequestCtx, clusterRestore *dpv1alpha1.ClusterRestore, cluster *appsv1.Cluster, backup *dpv1alpha1.Backup) error {
	encryptedAccounts := backup.Annotations[constant.EncryptedSystemAccountsAnnotationKey]
	if encryptedAccounts == "" {
		return nil
	}
	accountMap := map[string]map[string]string{}
	if err := json.Unmarshal([]byte(encryptedAccounts), &accountMap); err != nil {
		return intctrlutil.NewFatalError(err.Error())
	}
	decryptor := intctrlutil.NewEncryptor(viper.GetString(constant.CfgKeyDPEncryptionKey))
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRestoreSourceAccountSecretName(cluster.Name, clusterRestore.Name),
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				dptypes.ClusterRestoreLabelKey: clusterRestore.Name,
				constant.AppManagedByLabelKey:  dptypes.AppName,
			},
			Annotations: map[string]string{
				constant.SystemAccountProvisionedAnnotationKey: "true",
				dptypes.ClusterRestoreUIDAnnotationKey:         string(clusterRestore.UID),
			},
		},
		Data: map[string][]byte{},
	}
	addPassword := func(ownerName, accountName, encryptedPassword string) (string, error) {
		password, err := decryptor.Decrypt([]byte(encryptedPassword))
		if err != nil {
			return "", intctrlutil.NewFatalError(err.Error())
		}
		passwordKey := clusterRestoreSourceAccountPasswordKey(ownerName, accountName)
		secret.Data[passwordKey] = []byte(password)
		return passwordKey, nil
	}
	injectSecretRef := func(account *appsv1.ComponentSystemAccount, passwordKey string) {
		account.SecretRef = &appsv1.ProvisionSecretRef{
			Name:      secret.Name,
			Namespace: cluster.Namespace,
			Password:  passwordKey,
		}
	}
	for i := range cluster.Spec.ComponentSpecs {
		comp := &cluster.Spec.ComponentSpecs[i]
		for j := range comp.SystemAccounts {
			account := &comp.SystemAccounts[j]
			encryptedPassword := accountMap[comp.Name][account.Name]
			if encryptedPassword == "" {
				continue
			}
			passwordKey, err := addPassword(comp.Name, account.Name, encryptedPassword)
			if err != nil {
				return err
			}
			injectSecretRef(account, passwordKey)
		}
	}
	for i := range cluster.Spec.Shardings {
		sharding := &cluster.Spec.Shardings[i]
		for j := range sharding.Template.SystemAccounts {
			account := &sharding.Template.SystemAccounts[j]
			encryptedPassword := accountMap[sharding.Name][account.Name]
			if encryptedPassword == "" {
				continue
			}
			passwordKey, err := addPassword(sharding.Name, account.Name, encryptedPassword)
			if err != nil {
				return err
			}
			injectSecretRef(account, passwordKey)
		}
	}
	if len(secret.Data) == 0 {
		return nil
	}
	if err := controllerutil.SetControllerReference(clusterRestore, secret, r.Scheme); err != nil {
		return err
	}
	current := &corev1.Secret{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(secret), current); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err = r.Client.Create(reqCtx.Ctx, secret); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
			if err = r.Client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(secret), current); err != nil {
				return err
			}
			if !isClusterRestoreManagedResource(clusterRestore, current) {
				return intctrlutil.NewFatalError(fmt.Sprintf("account source Secret %s/%s already exists and is not created by ClusterRestore %s/%s", current.Namespace, current.Name, clusterRestore.Namespace, clusterRestore.Name))
			}
			patch := client.MergeFrom(current.DeepCopy())
			current.Labels = secret.Labels
			current.Annotations = secret.Annotations
			current.Data = secret.Data
			if err = controllerutil.SetControllerReference(clusterRestore, current, r.Scheme); err != nil {
				return err
			}
			return r.Client.Patch(reqCtx.Ctx, current, patch)
		}
		return nil
	}
	if !isClusterRestoreManagedResource(clusterRestore, current) {
		return intctrlutil.NewFatalError(fmt.Sprintf("account source Secret %s/%s already exists and is not created by ClusterRestore %s/%s", current.Namespace, current.Name, clusterRestore.Namespace, clusterRestore.Name))
	}
	patch := client.MergeFrom(current.DeepCopy())
	current.Labels = secret.Labels
	current.Annotations = secret.Annotations
	current.Data = secret.Data
	if err := controllerutil.SetControllerReference(clusterRestore, current, r.Scheme); err != nil {
		return err
	}
	return r.Client.Patch(reqCtx.Ctx, current, patch)
}

func clusterRestoreSourceAccountSecretName(cluster, restore string) string {
	return constant.ShortenKubeName(fmt.Sprintf("%s-%s-account-src", cluster, restore), constant.KubeNameMaxLength)
}

func clusterRestoreSourceAccountPasswordKey(owner, account string) string {
	return constant.ShortenKubeName(fmt.Sprintf("%s-%s-%s", owner, account, constant.AccountPasswdForSecret), constant.KubeNameMaxLength)
}

func (r *ClusterRestoreReconciler) cleanupSourceSystemAccounts(reqCtx intctrlutil.RequestCtx, clusterRestore *dpv1alpha1.ClusterRestore, cluster *appsv1.Cluster) error {
	secret := &corev1.Secret{}
	key := types.NamespacedName{
		Name:      clusterRestoreSourceAccountSecretName(cluster.Name, clusterRestore.Name),
		Namespace: cluster.Namespace,
	}
	if err := r.Client.Get(reqCtx.Ctx, key, secret); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !isClusterRestoreManagedResource(clusterRestore, secret) {
		return intctrlutil.NewFatalError(fmt.Sprintf("account source Secret %s/%s already exists and is not created by ClusterRestore %s/%s", secret.Namespace, secret.Name, clusterRestore.Namespace, clusterRestore.Name))
	}
	if err := r.Client.Delete(reqCtx.Ctx, secret); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}

func (r *ClusterRestoreReconciler) cleanupClusterRestoreAccountRefs(reqCtx intctrlutil.RequestCtx, clusterRestore *dpv1alpha1.ClusterRestore, cluster *appsv1.Cluster) error {
	sourceSecretName := clusterRestoreSourceAccountSecretName(cluster.Name, clusterRestore.Name)
	patch := client.MergeFrom(cluster.DeepCopy())
	changed := false
	cleanup := func(account *appsv1.ComponentSystemAccount) {
		if account.SecretRef == nil {
			return
		}
		namespace := account.SecretRef.Namespace
		if namespace == "" {
			namespace = cluster.Namespace
		}
		if account.SecretRef.Name == sourceSecretName && namespace == cluster.Namespace {
			account.SecretRef = nil
			changed = true
		}
	}
	for i := range cluster.Spec.ComponentSpecs {
		for j := range cluster.Spec.ComponentSpecs[i].SystemAccounts {
			cleanup(&cluster.Spec.ComponentSpecs[i].SystemAccounts[j])
		}
	}
	for i := range cluster.Spec.Shardings {
		for j := range cluster.Spec.Shardings[i].Template.SystemAccounts {
			cleanup(&cluster.Spec.Shardings[i].Template.SystemAccounts[j])
		}
	}
	if !changed {
		return nil
	}
	return r.Client.Patch(reqCtx.Ctx, cluster, patch)
}

func clusterRestoreHasAccountSourceRefs(clusterRestore *dpv1alpha1.ClusterRestore, cluster *appsv1.Cluster) bool {
	sourceSecretName := clusterRestoreSourceAccountSecretName(cluster.Name, clusterRestore.Name)
	hasRef := func(account appsv1.ComponentSystemAccount) bool {
		if account.SecretRef == nil {
			return false
		}
		namespace := account.SecretRef.Namespace
		if namespace == "" {
			namespace = cluster.Namespace
		}
		return account.SecretRef.Name == sourceSecretName && namespace == cluster.Namespace
	}
	for i := range cluster.Spec.ComponentSpecs {
		for j := range cluster.Spec.ComponentSpecs[i].SystemAccounts {
			if hasRef(cluster.Spec.ComponentSpecs[i].SystemAccounts[j]) {
				return true
			}
		}
	}
	for i := range cluster.Spec.Shardings {
		for j := range cluster.Spec.Shardings[i].Template.SystemAccounts {
			if hasRef(cluster.Spec.Shardings[i].Template.SystemAccounts[j]) {
				return true
			}
		}
	}
	return false
}

func isClusterRestoreTarget(clusterRestore *dpv1alpha1.ClusterRestore, cluster *appsv1.Cluster) bool {
	if cluster.Labels[dptypes.ClusterRestoreLabelKey] != clusterRestore.Name {
		return false
	}
	if clusterRestore.UID != "" && cluster.Annotations[dptypes.ClusterRestoreUIDAnnotationKey] == string(clusterRestore.UID) {
		return true
	}
	return clusterRestore.Status.TargetClusterRef != nil &&
		clusterRestore.Status.TargetClusterRef.UID != "" &&
		clusterRestore.Status.TargetClusterRef.UID == cluster.UID
}

func isClusterRestoreManagedResource(clusterRestore *dpv1alpha1.ClusterRestore, object client.Object) bool {
	if clusterRestore.UID != "" && object.GetAnnotations()[dptypes.ClusterRestoreUIDAnnotationKey] == string(clusterRestore.UID) {
		return true
	}
	for _, ref := range object.GetOwnerReferences() {
		if ref.UID == clusterRestore.UID &&
			ref.Name == clusterRestore.Name &&
			ref.Kind == "ClusterRestore" &&
			ref.APIVersion == dpv1alpha1.GroupVersion.String() {
			return true
		}
	}
	return false
}

func isClusterRestoreTerminal(phase dpv1alpha1.ClusterRestorePhase) bool {
	switch phase {
	case dpv1alpha1.ClusterRestorePhaseCompleted, dpv1alpha1.ClusterRestorePhaseFailed:
		return true
	default:
		return false
	}
}

func (r *ClusterRestoreReconciler) listClusterRestorePVCs(reqCtx intctrlutil.RequestCtx, clusterRestore *dpv1alpha1.ClusterRestore) ([]backupDataSourcePVC, error) {
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := r.Client.List(reqCtx.Ctx, pvcs,
		client.InNamespace(clusterRestore.Namespace),
		client.MatchingLabels{dptypes.ClusterRestoreLabelKey: clusterRestore.Name}); err != nil {
		return nil, err
	}
	backupNamespace := clusterRestore.Spec.BackupRef.Namespace
	if backupNamespace == "" {
		backupNamespace = clusterRestore.Namespace
	}
	backup := &dpv1alpha1.Backup{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: clusterRestore.Spec.BackupRef.Name, Namespace: backupNamespace}, backup); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, intctrlutil.NewFatalError(err.Error())
		}
		return nil, err
	}
	var items []backupDataSourcePVC
	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		if !isClusterRestoreManagedResource(clusterRestore, pvc) {
			continue
		}
		if !dprestore.IsBackupDataSourceRef(pvc.Spec.DataSourceRef) {
			continue
		}
		options := backupDataSourceContext{
			restoreTime:                       clusterRestore.Spec.RestoreTime,
			volumeSource:                      pvc.Annotations[dptypes.VolumeSourceAnnotationKey],
			sourceTargetName:                  pvc.Annotations[dptypes.SourceTargetNameAnnotationKey],
			sourceTargetPodName:               pvc.Annotations[dptypes.SourceTargetPodNameAnnotationKey],
			volumeRestorePolicy:               clusterRestore.Spec.VolumeRestorePolicy,
			deferPostReadyUntilClusterRunning: clusterRestore.Spec.DeferPostReadyUntilClusterRunning,
			env:                               clusterRestore.Spec.Env,
			parameters:                        clusterRestore.Spec.Parameters,
		}
		options.defaultInplace()
		if err := options.validate(); err != nil {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf("failed to validate restore metadata from ClusterRestore %s/%s PVC %s: %v", clusterRestore.Namespace, clusterRestore.Name, pvc.Name, err))
		}
		items = append(items, backupDataSourcePVC{pvc: pvc, backup: backup, options: options})
	}
	return items, nil
}

func (r *ClusterRestoreReconciler) buildRestoreManager(reqCtx intctrlutil.RequestCtx, clusterRestore *dpv1alpha1.ClusterRestore, item backupDataSourcePVC) (*dprestore.RestoreManager, error) {
	restore := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constant.ShortenKubeName(fmt.Sprintf("%s-backup-ds", item.pvc.Name), constant.KubeNameMaxLength),
			Namespace: item.pvc.Namespace,
			Labels: map[string]string{
				dptypes.ClusterRestoreLabelKey: clusterRestore.Name,
			},
		},
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:             item.backup.Name,
				Namespace:        item.backup.Namespace,
				SourceTargetName: item.options.sourceTargetName,
			},
			RestoreTime: item.options.restoreTime,
			Env:         item.options.env,
			Parameters:  item.options.parameters,
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
				DataSourceRef:                    ptrVolumeConfig(item.options.volumeConfig()),
				RequiredPolicyForAllPodSelection: requiredPolicyForSourcePod(item.options.sourceTargetPodName),
				VolumeClaimRestorePolicy:         item.options.volumeRestorePolicy,
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

func (r *ClusterRestoreReconciler) reconcilePostReady(reqCtx intctrlutil.RequestCtx, clusterRestore *dpv1alpha1.ClusterRestore, cluster *appsv1.Cluster, targetRef *dpv1alpha1.ClusterRestoreTargetClusterRef, items []backupDataSourcePVC) (ctrl.Result, error) {
	for i := range items {
		if items[i].options.deferPostReadyUntilClusterRunning && cluster.Status.Phase != appsv1.RunningClusterPhase {
			if err := r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseRestoring, metav1.ConditionFalse, "Restoring", "waiting for target Cluster running before post-ready restore", targetRef); err != nil {
				return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
			}
			return intctrlutil.Reconciled()
		}
	}
	restores, err := r.ensureInternalPostReadyRestores(reqCtx, clusterRestore, cluster, items)
	if err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	if err = r.cleanupClusterRestoreInputs(reqCtx, cluster); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	return r.updateClusterRestoreStatusFromRestores(reqCtx, clusterRestore, cluster, targetRef, restores)
}

func (r *ClusterRestoreReconciler) cleanupClusterRestoreInputs(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) error {
	patch := client.MergeFrom(cluster.DeepCopy())
	changed := false
	cleanup := func(vct *appsv1.PersistentVolumeClaimTemplate) {
		if dprestore.IsBackupDataSourceRef(vct.Spec.DataSourceRef) {
			vct.Spec.DataSourceRef = nil
			changed = true
		}
		if vct.Annotations != nil {
			for _, key := range []string{
				dptypes.ClusterRestoreUIDAnnotationKey,
				dptypes.VolumeSourceAnnotationKey,
				dptypes.SourceTargetNameAnnotationKey,
				dptypes.SourceTargetPodNameAnnotationKey,
			} {
				if _, ok := vct.Annotations[key]; ok {
					delete(vct.Annotations, key)
					changed = true
				}
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

func (r *ClusterRestoreReconciler) ensureInternalPostReadyRestores(reqCtx intctrlutil.RequestCtx, clusterRestore *dpv1alpha1.ClusterRestore, cluster *appsv1.Cluster, items []backupDataSourcePVC) ([]*dpv1alpha1.Restore, error) {
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
		restore := r.buildInternalPostReadyRestore(clusterRestore, cluster, item, componentName)
		existing := &dpv1alpha1.Restore{}
		if err := r.Client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(restore), existing); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
			if err = controllerutil.SetControllerReference(clusterRestore, restore, r.Scheme); err != nil {
				return nil, err
			}
			if err = r.Client.Create(reqCtx.Ctx, restore); err != nil && !apierrors.IsAlreadyExists(err) {
				return nil, err
			}
			restores = append(restores, restore)
			continue
		}
		if !isClusterRestoreManagedResource(clusterRestore, existing) {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf("post-ready Restore %s/%s already exists and is not created by ClusterRestore %s/%s", existing.Namespace, existing.Name, clusterRestore.Namespace, clusterRestore.Name))
		}
		restores = append(restores, existing)
	}
	return restores, nil
}

func (r *ClusterRestoreReconciler) buildInternalPostReadyRestore(clusterRestore *dpv1alpha1.ClusterRestore, cluster *appsv1.Cluster, item backupDataSourcePVC, componentName string) *dpv1alpha1.Restore {
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
			RequiredPolicyForAllPodSelection: requiredPolicyForSourcePod(item.options.sourceTargetPodName),
			Target: dpv1alpha1.JobActionTarget{
				PodSelector: dpv1alpha1.PodSelector{
					LabelSelector: &podSelector,
				},
			},
		},
		ConnectionCredential: r.connectionCredential(cluster, componentName),
	}
	target := dputils.GetBackupStatusTarget(item.backup, item.options.sourceTargetName)
	if target != nil && target.PodSelector != nil {
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
				dptypes.ClusterRestoreLabelKey:        clusterRestore.Name,
				backupDataSourceRestoreLabelCluster:   cluster.Name,
				backupDataSourceRestoreLabelComponent: componentName,
				backupDataSourceRestoreLabelBackup:    item.backup.Name,
				constant.AppManagedByLabelKey:         dptypes.AppName,
			},
			Annotations: map[string]string{
				dptypes.ClusterRestoreUIDAnnotationKey: string(clusterRestore.UID),
			},
		},
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:             item.backup.Name,
				Namespace:        item.backup.Namespace,
				SourceTargetName: item.options.sourceTargetName,
			},
			RestoreTime: item.options.restoreTime,
			Env:         item.options.env,
			Parameters:  item.options.parameters,
			ReadyConfig: readyConfig,
		},
	}
}

func (r *ClusterRestoreReconciler) connectionCredential(cluster *appsv1.Cluster, componentName string) *dpv1alpha1.ConnectionCredential {
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

func (r *ClusterRestoreReconciler) listInternalPostReadyRestores(reqCtx intctrlutil.RequestCtx, clusterRestore *dpv1alpha1.ClusterRestore) ([]*dpv1alpha1.Restore, error) {
	restoreList := &dpv1alpha1.RestoreList{}
	if err := r.Client.List(reqCtx.Ctx, restoreList,
		client.InNamespace(clusterRestore.Namespace),
		client.MatchingLabels{dptypes.ClusterRestoreLabelKey: clusterRestore.Name}); err != nil {
		return nil, err
	}
	restores := make([]*dpv1alpha1.Restore, 0, len(restoreList.Items))
	for i := range restoreList.Items {
		if !isClusterRestoreManagedResource(clusterRestore, &restoreList.Items[i]) {
			continue
		}
		if restoreList.Items[i].Spec.ReadyConfig == nil {
			continue
		}
		restores = append(restores, &restoreList.Items[i])
	}
	return restores, nil
}

func (r *ClusterRestoreReconciler) updateClusterRestoreStatusFromRestores(reqCtx intctrlutil.RequestCtx, clusterRestore *dpv1alpha1.ClusterRestore, cluster *appsv1.Cluster, targetRef *dpv1alpha1.ClusterRestoreTargetClusterRef, restores []*dpv1alpha1.Restore) (ctrl.Result, error) {
	allCompleted := true
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
			if err := r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseFailed, metav1.ConditionFalse, "Failed", msg, targetRef); err != nil {
				return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
			}
			return intctrlutil.Reconciled()
		default:
			allCompleted = false
		}
	}
	if allCompleted {
		if clusterRestoreHasAccountSourceRefs(clusterRestore, cluster) && cluster.Status.Phase != appsv1.RunningClusterPhase {
			if err := r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseRestoring, metav1.ConditionFalse, "Restoring", "waiting for target Cluster running before account source cleanup", targetRef); err != nil {
				return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
			}
			return intctrlutil.Reconciled()
		}
		if err := r.cleanupClusterRestoreAccountRefs(reqCtx, clusterRestore, cluster); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		if err := r.cleanupSourceSystemAccounts(reqCtx, clusterRestore, cluster); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		if err := r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseCompleted, metav1.ConditionTrue, "Completed", "restore completed", targetRef); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}
	if err := r.patchClusterRestoreStatus(reqCtx, clusterRestore, dpv1alpha1.ClusterRestorePhaseRestoring, metav1.ConditionFalse, "Restoring", "waiting for post-ready restore", targetRef); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *ClusterRestoreReconciler) patchClusterRestoreStatus(reqCtx intctrlutil.RequestCtx, clusterRestore *dpv1alpha1.ClusterRestore, phase dpv1alpha1.ClusterRestorePhase, status metav1.ConditionStatus, reason, message string, targetRef *dpv1alpha1.ClusterRestoreTargetClusterRef) error {
	latest := &dpv1alpha1.ClusterRestore{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(clusterRestore), latest); err != nil {
		return err
	}
	patch := client.MergeFrom(latest.DeepCopy())
	latest.Status.Phase = phase
	latest.Status.ObservedGeneration = latest.Generation
	if targetRef != nil {
		latest.Status.TargetClusterRef = targetRef
	}
	meta.SetStatusCondition(&latest.Status.Conditions, metav1.Condition{
		Type:               dpv1alpha1.ClusterRestoreReadyCondition,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: latest.Generation,
		LastTransitionTime: metav1.Now(),
	})
	return r.Client.Status().Patch(reqCtx.Ctx, latest, patch)
}

func clusterRestoreTargetRef(cluster *appsv1.Cluster) *dpv1alpha1.ClusterRestoreTargetClusterRef {
	return &dpv1alpha1.ClusterRestoreTargetClusterRef{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
		UID:       cluster.UID,
	}
}

func (r *ClusterRestoreReconciler) mapClusterToClusterRestore(ctx context.Context, object client.Object) []reconcile.Request {
	name := object.GetLabels()[dptypes.ClusterRestoreLabelKey]
	if name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: object.GetNamespace(), Name: name}}}
}

func (r *ClusterRestoreReconciler) mapPVCToClusterRestore(ctx context.Context, object client.Object) []reconcile.Request {
	name := object.GetLabels()[dptypes.ClusterRestoreLabelKey]
	if name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: object.GetNamespace(), Name: name}}}
}

func (r *ClusterRestoreReconciler) mapJobToClusterRestore(ctx context.Context, object client.Object) []reconcile.Request {
	if restoreName := object.GetLabels()[dprestore.DataProtectionRestoreLabelKey]; restoreName != "" {
		restoreNamespace := object.GetLabels()[dprestore.DataProtectionRestoreNamespaceLabelKey]
		if restoreNamespace == "" {
			restoreNamespace = object.GetNamespace()
		}
		restore := &dpv1alpha1.Restore{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: restoreNamespace, Name: restoreName}, restore); err == nil {
			return r.mapRestoreToClusterRestore(ctx, restore)
		}
	}
	for _, ref := range object.GetOwnerReferences() {
		if ref.Kind != "PersistentVolumeClaim" || ref.Name == "" {
			continue
		}
		pvc := &corev1.PersistentVolumeClaim{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: object.GetNamespace(), Name: ref.Name}, pvc); err != nil {
			return nil
		}
		return r.mapPVCToClusterRestore(ctx, pvc)
	}
	return nil
}

func (r *ClusterRestoreReconciler) mapRestoreToClusterRestore(ctx context.Context, object client.Object) []reconcile.Request {
	name := object.GetLabels()[dptypes.ClusterRestoreLabelKey]
	if name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: object.GetNamespace(), Name: name}}}
}

var _ reconcile.Reconciler = &ClusterRestoreReconciler{}

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
