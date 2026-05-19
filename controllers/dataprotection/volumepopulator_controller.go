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
	"reflect"
	"slices"
	"strconv"
	"strings"

	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-helpers/storage/volume"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dperrors "github.com/apecloud/kubeblocks/pkg/dataprotection/errors"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// VolumePopulatorReconciler reconciles Backup dataSource PVCs.
type VolumePopulatorReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

type pvcRestoreMode string

const (
	pvcRestoreModeRestoreData   pvcRestoreMode = "RestoreData"
	pvcRestoreModeProvisionOnly pvcRestoreMode = "ProvisionOnly"
)

type pvcRestoreContext struct {
	restoreMgr    *dprestore.RestoreManager
	mode          pvcRestoreMode
	skipPostReady bool
}

type pvcRestoreDecision struct {
	mode          pvcRestoreMode
	sourceTarget  *dpv1alpha1.BackupStatusTarget
	skipPostReady bool
}

// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=restores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=restores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentdefinitions,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *VolumePopulatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("volume-populator", req.NamespacedName),
		Recorder: r.Recorder,
	}

	// Get pvc
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, pvc); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err := r.syncPVC(reqCtx, pvc); err != nil {
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
			r.Recorder.Event(pvc, corev1.EventTypeWarning, ReasonVolumePopulateFailed, err.Error())
			if patchErr := r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingFailed, err.Error()); patchErr != nil {
				return intctrlutil.RequeueWithError(patchErr, reqCtx.Log, "")
			}
			return intctrlutil.Reconciled()
		} else if requeueErr, ok := err.(intctrlutil.RequeueError); ok {
			return intctrlutil.RequeueAfter(requeueErr.RequeueAfter(), reqCtx.Log, requeueErr.Reason())
		}
		return RecorderEventAndRequeue(reqCtx, r.Recorder, pvc, err)
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *VolumePopulatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&corev1.PersistentVolumeClaim{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *VolumePopulatorReconciler) MatchToPopulate(pvc *corev1.PersistentVolumeClaim) (bool, error) {
	dataSourceRef := pvc.Spec.DataSourceRef
	if dataSourceRef == nil {
		// Ignore PVCs without a datasource
		return false, nil
	}
	apiGroup := ""
	if dataSourceRef.APIGroup != nil {
		apiGroup = *dataSourceRef.APIGroup
	}
	if apiGroup != dptypes.DataprotectionAPIGroup || dataSourceRef.Kind != dptypes.BackupKind || dataSourceRef.Name == "" {
		// Ignore PVCs that aren't for this populator to handle
		return false, nil
	}
	return true, nil
}

func (r *VolumePopulatorReconciler) syncPVC(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim) error {
	matched, err := r.MatchToPopulate(pvc)
	if err != nil {
		return err
	}
	if !matched {
		return nil
	}
	restoreCtx, err := r.validateRestoreAndBuildMGR(reqCtx, pvc)
	if err != nil {
		return err
	}
	// if pvc has not bound pv, populate it.
	if pvc.Spec.VolumeName == "" {
		if restoreCtx.mode == pvcRestoreModeRestoreData {
			if err = r.waitForSerialPredecessors(reqCtx, pvc, restoreCtx.restoreMgr); err != nil {
				return err
			}
			return r.Populate(reqCtx, pvc, restoreCtx)
		}
		return r.ProvisionOnly(reqCtx, pvc, restoreCtx)
	}
	if err = r.completeBoundPVCIfNeeded(reqCtx, pvc, restoreCtx); err != nil {
		return err
	}
	return r.Cleanup(reqCtx, pvc)
}

func (r *VolumePopulatorReconciler) validateRestoreAndBuildMGR(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim) (*pvcRestoreContext, error) {
	backupNamespace := pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey]
	if backupNamespace == "" {
		backupNamespace = pvc.Namespace
	}
	backup := &dpv1alpha1.Backup{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: backupNamespace, Name: pvc.Spec.DataSourceRef.Name}, backup); err != nil {
		return nil, err
	}
	if backup.Status.BackupMethod == nil {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf(`status.backupMethod of backup "%s" can not be empty`, backup.Name))
	}
	parameters, err := restoreParametersMapFromPVC(pvc)
	if err != nil {
		return nil, intctrlutil.NewFatalError(err.Error())
	}
	env, err := restoreEnvFromParameters(parameters)
	if err != nil {
		return nil, intctrlutil.NewFatalError(err.Error())
	}
	volumeRestorePolicy, err := volumeRestorePolicyFromParameters(parameters)
	if err != nil {
		return nil, intctrlutil.NewFatalError(err.Error())
	}
	decision, err := r.decidePVCRestore(reqCtx, pvc, backup, parameters)
	if err != nil {
		return nil, err
	}
	restore := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getPopulatePVCName(pvc.UID),
			Namespace: pvc.Namespace,
			Labels:    internalRestoreLabels(pvc),
			Annotations: map[string]string{
				constant.RestoreSourceNamespaceAnnotationKey: backupNamespace,
				constant.RestoreComponentAnnotationKey:       pvc.Annotations[constant.RestoreComponentAnnotationKey],
				constant.RestoreVolumeTemplateAnnotationKey:  pvc.Annotations[constant.RestoreVolumeTemplateAnnotationKey],
			},
		},
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:      pvc.Spec.DataSourceRef.Name,
				Namespace: backupNamespace,
			},
			RestoreTime: pvc.Annotations[constant.RestorePITRAnnotationKey],
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
				DataSourceRef: &dpv1alpha1.VolumeConfig{
					VolumeSource: pvc.Annotations[constant.RestoreVolumeTemplateAnnotationKey],
				},
				VolumeClaimRestorePolicy: volumeRestorePolicy,
			},
			Env:        env,
			Parameters: restoreParametersToPairs(restoreActionParameters(parameters)),
		},
	}
	if restore.Spec.PrepareDataConfig.DataSourceRef.VolumeSource == "" {
		restore.Spec.PrepareDataConfig.DataSourceRef.VolumeSource = pvc.Name
	}
	if decision.sourceTarget != nil {
		restore.Spec.Backup.SourceTargetName = decision.sourceTarget.Name
		if decision.mode == pvcRestoreModeRestoreData {
			restore.Spec.PrepareDataConfig.RequiredPolicyForAllPodSelection, err = requiredPolicyForPVC(decision.sourceTarget, pvc)
			if err != nil {
				return nil, err
			}
		}
	}
	if err = r.restoreSystemAccountSecrets(reqCtx, pvc, backupNamespace); err != nil {
		return nil, err
	}
	if decision.mode == pvcRestoreModeRestoreData {
		restore, err = r.ensureInternalRestore(reqCtx, pvc, restore)
		if err != nil {
			return nil, err
		}
		if err = r.ensureInternalRestoreBackupRepoReady(reqCtx, restore); err != nil {
			return nil, err
		}
	}
	restoreMgr := dprestore.NewRestoreManager(restore, r.Recorder, r.Scheme, r.Client)
	if err = dprestore.ValidateAndInitRestoreMGR(reqCtx, r.Client, restoreMgr); err != nil {
		return nil, err
	}
	if decision.mode == pvcRestoreModeProvisionOnly {
		restoreMgr.PrepareDataBackupSets = nil
	}
	saName := restore.Spec.ServiceAccountName
	if saName == "" {
		var err error
		// TODO: update the mcMgr param
		if saName, err = EnsureWorkerServiceAccount(reqCtx, r.Client, restore.Namespace, nil); err != nil {
			return nil, err
		}
	}
	restoreMgr.WorkerServiceAccount = saName
	return &pvcRestoreContext{
		restoreMgr:    restoreMgr,
		mode:          decision.mode,
		skipPostReady: decision.skipPostReady,
	}, nil
}

func (r *VolumePopulatorReconciler) ensureInternalRestoreBackupRepoReady(reqCtx intctrlutil.RequestCtx,
	restore *dpv1alpha1.Restore) error {
	original := restore.DeepCopy()
	repoName, err := CheckBackupRepoForRestore(reqCtx, r.Client, restore)
	switch {
	case intctrlutil.IsTargetError(err, dperrors.ErrorTypeWaitForBackupRepoPreparation):
		dprestore.SetRestoreCheckBackupRepoCondition(restore, dprestore.ReasonWaitForBackupRepo, err.Error())
		if restore.Labels == nil {
			restore.Labels = map[string]string{}
		}
		restore.Labels[dataProtectionBackupRepoKey] = repoName
		restore.Labels[dataProtectionWaitRepoPreparationKey] = trueVal
		if patchErr := r.patchInternalRestoreMetaAndStatus(reqCtx, original, restore); patchErr != nil {
			return patchErr
		}
		return intctrlutil.NewRequeueError(reconcileInterval, err.Error())
	case intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal):
		dprestore.SetRestoreCheckBackupRepoCondition(restore, dprestore.ReasonCheckBackupRepoFailed, err.Error())
		if patchErr := r.patchInternalRestoreMetaAndStatus(reqCtx, original, restore); patchErr != nil {
			return patchErr
		}
		return err
	case err != nil:
		dprestore.SetRestoreCheckBackupRepoCondition(restore, ReasonUnknownError, err.Error())
		if patchErr := r.patchInternalRestoreMetaAndStatus(reqCtx, original, restore); patchErr != nil {
			return patchErr
		}
		return err
	default:
		dprestore.SetRestoreCheckBackupRepoCondition(restore, dprestore.ReasonCheckBackupRepoSuccessfully, "")
		return r.patchInternalRestoreMetaAndStatus(reqCtx, original, restore)
	}
}

func (r *VolumePopulatorReconciler) resolveSourceTarget(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim, backupNamespace string) (*dpv1alpha1.BackupStatusTarget, error) {
	backup := &dpv1alpha1.Backup{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: backupNamespace, Name: pvc.Spec.DataSourceRef.Name}, backup); err != nil {
		return nil, err
	}
	parameters, err := restoreParametersMapFromPVC(pvc)
	if err != nil {
		return nil, intctrlutil.NewFatalError(err.Error())
	}
	sourceTarget, _, err := r.resolveSourceTargetFromBackup(reqCtx, pvc, backup, parameters)
	return sourceTarget, err
}

func (r *VolumePopulatorReconciler) decidePVCRestore(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	backup *dpv1alpha1.Backup,
	parameters map[string]string) (*pvcRestoreDecision, error) {
	volumeName := restoreVolumeTemplateName(pvc)
	restoreData := backup.Status.BackupMethod.TargetVolumes != nil && utils.ExistTargetVolume(backup.Status.BackupMethod.TargetVolumes, volumeName)
	sourceTarget, skipPostReady, err := r.resolveSourceTargetFromBackup(reqCtx, pvc, backup, parameters)
	if err != nil {
		return nil, err
	}
	mode := pvcRestoreModeProvisionOnly
	if restoreData && !skipPostReady {
		mode = pvcRestoreModeRestoreData
	}
	return &pvcRestoreDecision{
		mode:          mode,
		sourceTarget:  sourceTarget,
		skipPostReady: skipPostReady,
	}, nil
}

func restoreVolumeTemplateName(pvc *corev1.PersistentVolumeClaim) string {
	if pvc.Annotations != nil {
		if volumeName := pvc.Annotations[constant.RestoreVolumeTemplateAnnotationKey]; volumeName != "" {
			return volumeName
		}
	}
	if pvc.Labels != nil {
		if volumeName := pvc.Labels[constant.VolumeClaimTemplateNameLabelKey]; volumeName != "" {
			return volumeName
		}
	}
	return pvc.Name
}

func (r *VolumePopulatorReconciler) resolveSourceTargetFromBackup(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	backup *dpv1alpha1.Backup,
	parameters map[string]string) (*dpv1alpha1.BackupStatusTarget, bool, error) {
	if sourceTargetName := restoreParameterOrAnnotation(pvc, parameters, dptypes.SourceTargetNameAnnotationKey); sourceTargetName != "" {
		target := utils.GetBackupStatusTarget(backup, sourceTargetName)
		if target == nil {
			return nil, false, intctrlutil.NewFatalError(fmt.Sprintf("backup target %s does not exist for PVC %s/%s", sourceTargetName, pvc.Namespace, pvc.Name))
		}
		return target, false, nil
	}
	if backup.Status.Target != nil || len(backup.Status.Targets) == 0 {
		return backup.Status.Target, false, nil
	}
	if len(backup.Status.Targets) == 1 {
		return &backup.Status.Targets[0], false, nil
	}
	if pvc.Labels[constant.KBAppShardingNameLabelKey] != "" {
		return r.resolveShardingSourceTarget(reqCtx, pvc, backup)
	}
	var matched []*dpv1alpha1.BackupStatusTarget
	for i := range backup.Status.Targets {
		target := &backup.Status.Targets[i]
		if backupTargetMatchesPVC(target, pvc) {
			matched = append(matched, target)
		}
	}
	if len(matched) == 1 {
		return matched[0], false, nil
	}
	if len(matched) > 1 {
		var names []string
		for _, target := range matched {
			names = append(names, target.Name)
		}
		return nil, false, intctrlutil.NewFatalError(fmt.Sprintf("multiple backup targets match PVC %s/%s: %v", pvc.Namespace, pvc.Name, names))
	}
	return nil, false, intctrlutil.NewFatalError(fmt.Sprintf("no backup target matches PVC %s/%s", pvc.Namespace, pvc.Name))
}

func (r *VolumePopulatorReconciler) resolveShardingSourceTarget(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	backup *dpv1alpha1.Backup) (*dpv1alpha1.BackupStatusTarget, bool, error) {
	clusterName := pvc.Labels[constant.AppInstanceLabelKey]
	shardingName := pvc.Labels[constant.KBAppShardingNameLabelKey]
	componentName := restoreComponentName(pvc)
	if clusterName == "" || shardingName == "" || componentName == "" {
		return nil, false, intctrlutil.NewFatalError(fmt.Sprintf("missing cluster/sharding/component labels for PVC %s/%s", pvc.Namespace, pvc.Name))
	}
	cluster := &appsv1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: pvc.Namespace, Name: clusterName}, cluster); err != nil {
		return nil, false, err
	}
	desiredShards, found := desiredShardsForClusterSharding(cluster, shardingName)
	if !found {
		return nil, false, intctrlutil.NewFatalError(fmt.Sprintf("sharding %s not found in cluster %s/%s", shardingName, cluster.Namespace, cluster.Name))
	}
	if int32(len(backup.Status.Targets)) > desiredShards {
		return nil, false, intctrlutil.NewFatalError(fmt.Sprintf(`the source targets count of the backup "%s" must be equal to or less than the count of the shard components "%s"`, backup.Name, shardingName))
	}
	components, err := r.listShardingComponents(reqCtx, pvc.Namespace, clusterName, shardingName)
	if err != nil {
		return nil, false, err
	}
	if int32(len(components)) < desiredShards {
		return nil, false, intctrlutil.NewRequeueError(reconcileInterval, fmt.Sprintf("waiting for sharding %s components to be created", shardingName))
	}
	slices.SortFunc(components, func(a, b appsv1.Component) int {
		return strings.Compare(componentLogicalName(&a), componentLogicalName(&b))
	})
	for i := range components {
		name := componentLogicalName(&components[i])
		if name != componentName {
			continue
		}
		if i >= len(backup.Status.Targets) {
			return nil, true, nil
		}
		return &backup.Status.Targets[i], false, nil
	}
	return nil, false, intctrlutil.NewRequeueError(reconcileInterval, fmt.Sprintf("waiting for sharding component %s to be created", componentName))
}

func desiredShardsForClusterSharding(cluster *appsv1.Cluster, shardingName string) (int32, bool) {
	for i := range cluster.Spec.Shardings {
		sharding := &cluster.Spec.Shardings[i]
		if sharding.Name != shardingName {
			continue
		}
		return sharding.Shards, true
	}
	return 0, false
}

func (r *VolumePopulatorReconciler) listShardingComponents(reqCtx intctrlutil.RequestCtx,
	namespace, clusterName, shardingName string) ([]appsv1.Component, error) {
	componentList := &appsv1.ComponentList{}
	if err := r.Client.List(reqCtx.Ctx, componentList, client.InNamespace(namespace),
		client.MatchingLabels(constant.GetClusterLabels(clusterName, map[string]string{constant.KBAppShardingNameLabelKey: shardingName}))); err != nil {
		return nil, err
	}
	var components []appsv1.Component
	for i := range componentList.Items {
		if !componentList.Items[i].DeletionTimestamp.IsZero() {
			continue
		}
		components = append(components, componentList.Items[i])
	}
	return components, nil
}

func componentLogicalName(component *appsv1.Component) string {
	if component.Labels != nil {
		if name := component.Labels[constant.KBAppComponentLabelKey]; name != "" {
			return name
		}
	}
	return component.Name
}

func backupTargetMatchesPVC(target *dpv1alpha1.BackupStatusTarget, pvc *corev1.PersistentVolumeClaim) bool {
	if target.PodSelector == nil || target.PodSelector.LabelSelector == nil {
		return false
	}
	selector := target.PodSelector.LabelSelector
	var effective bool
	for k, v := range selector.MatchLabels {
		if k == constant.AppInstanceLabelKey {
			continue
		}
		effective = true
		if pvc.Labels[k] != v {
			return false
		}
	}
	for _, expression := range selector.MatchExpressions {
		if expression.Key == constant.AppInstanceLabelKey {
			continue
		}
		effective = true
		value, exists := pvc.Labels[expression.Key]
		switch expression.Operator {
		case metav1.LabelSelectorOpIn:
			if !exists || !slices.Contains(expression.Values, value) {
				return false
			}
		case metav1.LabelSelectorOpNotIn:
			if exists && slices.Contains(expression.Values, value) {
				return false
			}
		case metav1.LabelSelectorOpExists:
			if !exists {
				return false
			}
		case metav1.LabelSelectorOpDoesNotExist:
			if exists {
				return false
			}
		default:
			return false
		}
	}
	return effective
}

func requiredPolicyForPVC(target *dpv1alpha1.BackupStatusTarget, pvc *corev1.PersistentVolumeClaim) (*dpv1alpha1.RequiredPolicyForAllPodSelection, error) {
	if target == nil || target.PodSelector == nil || target.PodSelector.Strategy != dpv1alpha1.PodSelectionStrategyAll {
		return nil, nil
	}
	sourceTargetPodName, err := resolveSourceTargetPodName(target, pvc)
	if err != nil {
		return nil, err
	}
	return &dpv1alpha1.RequiredPolicyForAllPodSelection{
		DataRestorePolicy: dpv1alpha1.OneToManyRestorePolicy,
		SourceOfOneToMany: &dpv1alpha1.SourceOfOneToMany{
			TargetPodName: sourceTargetPodName,
		},
	}, nil
}

func resolveSourceTargetPodName(target *dpv1alpha1.BackupStatusTarget, pvc *corev1.PersistentVolumeClaim) (string, error) {
	parameters, err := restoreParametersMapFromPVC(pvc)
	if err != nil {
		return "", intctrlutil.NewFatalError(err.Error())
	}
	if sourceTargetPodName := restoreParameterOrAnnotation(pvc, parameters, dptypes.SourceTargetPodNameAnnotationKey); sourceTargetPodName != "" {
		return sourceTargetPodName, nil
	}
	if len(target.SelectedTargetPods) == 1 {
		return target.SelectedTargetPods[0], nil
	}
	targetPodName := pvc.Labels[constant.KBAppPodNameLabelKey]
	if targetPodName == "" {
		return "", intctrlutil.NewFatalError(fmt.Sprintf("source target pod can not be inferred for PVC %s/%s", pvc.Namespace, pvc.Name))
	}
	if slices.Contains(target.SelectedTargetPods, targetPodName) {
		return targetPodName, nil
	}
	targetOrdinal, ok := podOrdinal(targetPodName)
	if !ok {
		return "", intctrlutil.NewFatalError(fmt.Sprintf("source target pod can not be inferred from target pod %s for PVC %s/%s", targetPodName, pvc.Namespace, pvc.Name))
	}
	if templateName := pvc.Labels[constant.KBAppInstanceTemplateLabelKey]; templateName != "" {
		return "", intctrlutil.NewFatalError(fmt.Sprintf("source target pod can not be inferred for instance template %s and target pod %s for PVC %s/%s, set %s explicitly", templateName, targetPodName, pvc.Namespace, pvc.Name, dptypes.SourceTargetPodNameAnnotationKey))
	}
	var candidates []string
	for _, sourcePodName := range target.SelectedTargetPods {
		sourceOrdinal, ok := podOrdinal(sourcePodName)
		if ok && sourceOrdinal == targetOrdinal {
			candidates = append(candidates, sourcePodName)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) > 1 {
		return "", intctrlutil.NewFatalError(fmt.Sprintf("multiple selected source target pods match target pod %s for PVC %s/%s: %v", targetPodName, pvc.Namespace, pvc.Name, candidates))
	}
	return "", intctrlutil.NewFatalError(fmt.Sprintf("no selected source target pod matches target pod %s for PVC %s/%s", targetPodName, pvc.Namespace, pvc.Name))
}

func podOrdinal(podName string) (int, bool) {
	index := strings.LastIndex(podName, "-")
	if index < 0 || index == len(podName)-1 {
		return 0, false
	}
	ordinal, err := strconv.Atoi(podName[index+1:])
	return ordinal, err == nil
}

func internalRestoreLabels(pvc *corev1.PersistentVolumeClaim) map[string]string {
	labels := map[string]string{
		dprestore.DataProtectionRestoreLabelKey:          getPopulatePVCName(pvc.UID),
		dprestore.DataProtectionRestoreNamespaceLabelKey: pvc.Namespace,
		dprestore.DataProtectionPopulatePVCLabelKey:      getPopulatePVCName(pvc.UID),
	}
	for _, key := range []string{
		constant.AppInstanceLabelKey,
		constant.KBAppComponentLabelKey,
		constant.KBAppShardingNameLabelKey,
		constant.VolumeClaimTemplateNameLabelKey,
	} {
		if value := pvc.Labels[key]; value != "" {
			labels[key] = value
		}
	}
	if pvc.Spec.DataSourceRef != nil {
		labels[dptypes.BackupNameLabelKey] = pvc.Spec.DataSourceRef.Name
	}
	return labels
}

func (r *VolumePopulatorReconciler) ensureInternalRestore(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	desired *dpv1alpha1.Restore) (*dpv1alpha1.Restore, error) {
	if err := controllerutil.SetOwnerReference(pvc, desired, r.Scheme); err != nil {
		return nil, err
	}
	existing := &dpv1alpha1.Restore{}
	key := types.NamespacedName{Namespace: desired.Namespace, Name: desired.Name}
	if err := r.Client.Get(reqCtx.Ctx, key, existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		if err = r.Client.Create(reqCtx.Ctx, desired); err != nil {
			return nil, err
		}
		return desired, nil
	}
	original := existing.DeepCopy()
	existing.Labels = desired.Labels
	existing.Annotations = desired.Annotations
	existing.OwnerReferences = desired.OwnerReferences
	existing.Spec = desired.Spec
	if !reflect.DeepEqual(original.Labels, existing.Labels) ||
		!reflect.DeepEqual(original.Annotations, existing.Annotations) ||
		!reflect.DeepEqual(original.OwnerReferences, existing.OwnerReferences) ||
		!reflect.DeepEqual(original.Spec, existing.Spec) {
		if err := r.Client.Patch(reqCtx.Ctx, existing, client.MergeFrom(original)); err != nil {
			return nil, err
		}
	}
	return existing, nil
}

func restoreParametersMapFromPVC(pvc *corev1.PersistentVolumeClaim) (map[string]string, error) {
	parametersJSON := pvc.Annotations[constant.RestoreParametersAnnotationKey]
	if parametersJSON == "" {
		return nil, nil
	}
	parameters := map[string]string{}
	if err := json.Unmarshal([]byte(parametersJSON), &parameters); err != nil {
		return nil, err
	}
	return parameters, nil
}

func restoreParametersToPairs(parameters map[string]string) []dpv1alpha1.ParameterPair {
	if len(parameters) == 0 {
		return nil
	}
	result := make([]dpv1alpha1.ParameterPair, 0, len(parameters))
	for k, v := range parameters {
		result = append(result, dpv1alpha1.ParameterPair{Name: k, Value: v})
	}
	return result
}

func restoreEnvFromParameters(parameters map[string]string) ([]corev1.EnvVar, error) {
	envJSON := parameters[dptypes.RestoreEnvParameterKey]
	if envJSON == "" {
		return nil, nil
	}
	env := []corev1.EnvVar{}
	if err := json.Unmarshal([]byte(envJSON), &env); err != nil {
		return nil, fmt.Errorf("invalid restore env parameter %s: %w", dptypes.RestoreEnvParameterKey, err)
	}
	return env, nil
}

func volumeRestorePolicyFromParameters(parameters map[string]string) (dpv1alpha1.VolumeClaimRestorePolicy, error) {
	policy := parameters[dptypes.VolumeRestorePolicyParameterKey]
	if policy == "" {
		return dpv1alpha1.VolumeClaimRestorePolicyParallel, nil
	}
	switch dpv1alpha1.VolumeClaimRestorePolicy(policy) {
	case dpv1alpha1.VolumeClaimRestorePolicyParallel, dpv1alpha1.VolumeClaimRestorePolicySerial:
		return dpv1alpha1.VolumeClaimRestorePolicy(policy), nil
	default:
		return "", fmt.Errorf("invalid volume restore policy %q", policy)
	}
}

func restoreActionParameters(parameters map[string]string) map[string]string {
	if len(parameters) == 0 {
		return nil
	}
	internalKeys := map[string]struct{}{
		dptypes.VolumeSourceAnnotationKey:                     {},
		dptypes.SourceTargetNameAnnotationKey:                 {},
		dptypes.SourceTargetPodNameAnnotationKey:              {},
		dptypes.VolumeRestorePolicyParameterKey:               {},
		dptypes.RestoreEnvParameterKey:                        {},
		dptypes.DeferPostReadyUntilClusterRunningParameterKey: {},
	}
	result := make(map[string]string, len(parameters))
	for k, v := range parameters {
		if _, ok := internalKeys[k]; ok {
			continue
		}
		result[k] = v
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func restoreParameterOrAnnotation(pvc *corev1.PersistentVolumeClaim, parameters map[string]string, key string) string {
	if value := parameters[key]; value != "" {
		return value
	}
	return pvc.Annotations[key]
}

func (r *VolumePopulatorReconciler) Populate(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim, restoreCtx *pvcRestoreContext) error {
	restoreMgr := restoreCtx.restoreMgr
	wait, nodeName, err := r.waitForPVCSelectedNode(reqCtx, pvc)
	if err != nil || wait {
		return err
	}
	// Make sure the PVC finalizer is present
	if !slices.Contains(pvc.Finalizers, dptypes.DataProtectionFinalizerName) {
		pvcPatch := client.MergeFrom(pvc.DeepCopy())
		controllerutil.AddFinalizer(pvc, dptypes.DataProtectionFinalizerName)
		if err = r.Client.Patch(reqCtx.Ctx, pvc, pvcPatch); err != nil {
			return err
		}
	}
	if err = r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingProcessing, "Populator started"); err != nil {
		return err
	}
	// set scheduling for restore
	restoreMgr.Restore.Spec.PrepareDataConfig.SchedulingSpec = dpv1alpha1.SchedulingSpec{
		Tolerations: []corev1.Toleration{
			{Operator: corev1.TolerationOpExists},
		},
	}
	if nodeName != "" {
		restoreMgr.Restore.Spec.PrepareDataConfig.SchedulingSpec.NodeSelector = map[string]string{
			corev1.LabelHostname: nodeName,
		}
	}
	dprestore.SetRestoreStageCondition(restoreMgr.Restore, dpv1alpha1.PrepareData, dprestore.ReasonProcessing, "processing prepareData stage.")
	var populatePVC *corev1.PersistentVolumeClaim
	for i, v := range restoreMgr.PrepareDataBackupSets {
		target := utils.GetBackupStatusTarget(v.Backup, restoreMgr.Restore.Spec.Backup.SourceTargetName)
		if target == nil {
			dprestore.SetRestoreStageCondition(restoreMgr.Restore, dpv1alpha1.PrepareData, dprestore.ReasonFailed, "can not found any source targe in backup "+v.Backup.Name)
			if patchErr := r.patchInternalRestoreStatus(reqCtx, restoreMgr); patchErr != nil {
				return patchErr
			}
			return intctrlutil.NewFatalError("can not found any source targe in backup " + v.Backup.Name)
		}
		if populatePVC == nil {
			populatePVC, err = r.getPopulatePVC(reqCtx, pvc, v,
				restoreMgr.Restore, target, nodeName)
			if err != nil {
				dprestore.SetRestoreStageCondition(restoreMgr.Restore, dpv1alpha1.PrepareData, dprestore.ReasonFailed, err.Error())
				if patchErr := r.patchInternalRestoreStatus(reqCtx, restoreMgr); patchErr != nil {
					return patchErr
				}
				return err
			}
		}

		// 1. build populate job
		job, err := restoreMgr.BuildVolumePopulateJob(reqCtx, r.Client, v, target, populatePVC, i)
		if err != nil {
			dprestore.SetRestoreStageCondition(restoreMgr.Restore, dpv1alpha1.PrepareData, dprestore.ReasonFailed, err.Error())
			if patchErr := r.patchInternalRestoreStatus(reqCtx, restoreMgr); patchErr != nil {
				return patchErr
			}
			return err
		}
		if job == nil {
			continue
		}

		// 2. create job
		jobs, err := restoreMgr.CreateJobsIfNotExist(reqCtx, r.Client, pvc, []*batchv1.Job{job})
		if err != nil {
			dprestore.SetRestoreStageCondition(restoreMgr.Restore, dpv1alpha1.PrepareData, dprestore.ReasonFailed, err.Error())
			if patchErr := r.patchInternalRestoreStatus(reqCtx, restoreMgr); patchErr != nil {
				return patchErr
			}
			return err
		}

		// 3. check if jobs are finished.
		actionFinished, actionFailed, err := restoreMgr.CheckJobsDone(dpv1alpha1.PrepareData, "prepareData", v, jobs)
		if err != nil {
			dprestore.SetRestoreStageCondition(restoreMgr.Restore, dpv1alpha1.PrepareData, dprestore.ReasonFailed, err.Error())
			if patchErr := r.patchInternalRestoreStatus(reqCtx, restoreMgr); patchErr != nil {
				return patchErr
			}
			return err
		}
		if !actionFinished {
			if err = r.patchInternalRestoreStatus(reqCtx, restoreMgr); err != nil {
				return err
			}
			return nil
		}
		if actionFailed {
			dprestore.SetRestoreStageCondition(restoreMgr.Restore, dpv1alpha1.PrepareData, dprestore.ReasonFailed, "restore prepareData job failed")
			if err = r.patchInternalRestoreStatus(reqCtx, restoreMgr); err != nil {
				return err
			}
			return intctrlutil.NewFatalError("restore prepareData job failed")
		}
	}
	// 4. if jobs are succeed, rebind the pvc and pv
	rebound, err := r.rebindPVCAndPV(reqCtx, populatePVC, pvc)
	if err != nil {
		return err
	}
	if !rebound {
		if err = r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingProcessing, "Waiting for populate PV to bind target PVC"); err != nil {
			return err
		}
		return intctrlutil.NewRequeueError(reconcileInterval, "waiting for populate PV to bind target PVC")
	}
	dprestore.SetRestoreStageCondition(restoreMgr.Restore, dpv1alpha1.PrepareData, dprestore.ReasonSucceed, "prepare data successfully")
	if err = r.patchInternalRestoreStatus(reqCtx, restoreMgr); err != nil {
		return err
	}
	return r.completeBoundPVCIfNeeded(reqCtx, pvc, restoreCtx)
}

func (r *VolumePopulatorReconciler) ProvisionOnly(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim, restoreCtx *pvcRestoreContext) error {
	wait, nodeName, err := r.waitForPVCSelectedNode(reqCtx, pvc)
	if err != nil || wait {
		return err
	}
	if !slices.Contains(pvc.Finalizers, dptypes.DataProtectionFinalizerName) {
		pvcPatch := client.MergeFrom(pvc.DeepCopy())
		controllerutil.AddFinalizer(pvc, dptypes.DataProtectionFinalizerName)
		if err = r.Client.Patch(reqCtx.Ctx, pvc, pvcPatch); err != nil {
			return err
		}
	}
	if err = r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingProcessing, "Provisioning PVC without data restore"); err != nil {
		return err
	}
	populatePVC, err := r.getProvisionOnlyPVC(reqCtx, pvc, nodeName)
	if err != nil {
		return err
	}
	rebound, err := r.rebindPVCAndPV(reqCtx, populatePVC, pvc)
	if err != nil {
		return err
	}
	if !rebound {
		if err = r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingProcessing, "Waiting for provisioned PV to bind target PVC"); err != nil {
			return err
		}
		return intctrlutil.NewRequeueError(reconcileInterval, "waiting for provisioned PV to bind target PVC")
	}
	return r.completeBoundPVCIfNeeded(reqCtx, pvc, restoreCtx)
}

func (r *VolumePopulatorReconciler) patchInternalRestoreStatus(reqCtx intctrlutil.RequestCtx, restoreMgr *dprestore.RestoreManager) error {
	if reflect.DeepEqual(restoreMgr.OriginalRestore.Status, restoreMgr.Restore.Status) {
		return nil
	}
	return r.Client.Status().Patch(reqCtx.Ctx, restoreMgr.Restore, client.MergeFrom(restoreMgr.OriginalRestore))
}

func (r *VolumePopulatorReconciler) patchInternalRestoreMetaAndStatus(reqCtx intctrlutil.RequestCtx,
	original, restore *dpv1alpha1.Restore) error {
	if !reflect.DeepEqual(original.ObjectMeta, restore.ObjectMeta) {
		if err := r.Client.Patch(reqCtx.Ctx, restore, client.MergeFrom(original)); err != nil {
			return err
		}
	}
	if reflect.DeepEqual(original.Status, restore.Status) {
		return nil
	}
	latest := &dpv1alpha1.Restore{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(restore), latest); err != nil {
		return err
	}
	statusPatch := client.MergeFrom(latest.DeepCopy())
	latest.Status = restore.Status
	return r.Client.Status().Patch(reqCtx.Ctx, latest, statusPatch)
}

func (r *VolumePopulatorReconciler) completeBoundPVCIfNeeded(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	restoreCtx *pvcRestoreContext) error {
	restoreMgr := restoreCtx.restoreMgr
	for i := range pvc.Status.Conditions {
		condition := pvc.Status.Conditions[i]
		if string(condition.Type) != appsv1.ConditionTypeRestore {
			continue
		}
		switch condition.Status {
		case corev1.ConditionTrue:
			return nil
		case corev1.ConditionFalse:
			return nil
		}
		break
	}
	postReadyCompleted, err := r.ensurePostReadyRestoreCompleted(reqCtx, pvc, restoreCtx)
	if err != nil {
		return err
	}
	if !postReadyCompleted {
		return intctrlutil.NewRequeueError(reconcileInterval, "waiting for postReady restore")
	}
	reason := ReasonPopulatingSucceed
	message := "Populator finished"
	if restoreCtx.mode == pvcRestoreModeProvisionOnly {
		reason = ReasonPopulatingProvisioned
		message = "PVC provisioned without data restore"
	}
	if err := r.UpdatePVCConditions(reqCtx, pvc, reason, message); err != nil {
		return err
	}
	if restoreCtx.mode == pvcRestoreModeProvisionOnly {
		return nil
	}
	dprestore.SetRestoreStageCondition(restoreMgr.Restore, dpv1alpha1.PrepareData, dprestore.ReasonSucceed, "prepare data successfully")
	return r.patchInternalRestoreStatus(reqCtx, restoreMgr)
}

func (r *VolumePopulatorReconciler) waitForSerialPredecessors(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	restoreMgr *dprestore.RestoreManager) error {
	if restoreMgr.Restore.Spec.PrepareDataConfig == nil ||
		restoreMgr.Restore.Spec.PrepareDataConfig.VolumeClaimRestorePolicy != dpv1alpha1.VolumeClaimRestorePolicySerial {
		return nil
	}
	pvcs, err := r.listRestorePVCsForComponent(reqCtx, pvc)
	if err != nil {
		return err
	}
	slices.SortFunc(pvcs, func(a, b corev1.PersistentVolumeClaim) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	for i := range pvcs {
		item := &pvcs[i]
		if item.UID == pvc.UID {
			return nil
		}
		restoreData, err := r.pvcNeedsDataRestore(reqCtx, item)
		if err != nil {
			return err
		}
		if !restoreData {
			continue
		}
		cond := findPVCConditionByType(item, appsv1.ConditionTypeRestore)
		if cond != nil && cond.Status == corev1.ConditionFalse {
			return intctrlutil.NewFatalError(fmt.Sprintf("previous restore PVC %s/%s failed: %s", item.Namespace, item.Name, cond.Message))
		}
		if item.Spec.VolumeName != "" {
			continue
		}
		if err = r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingProcessing,
			fmt.Sprintf("Waiting for previous restore PVC %s to finish prepareData", item.Name)); err != nil {
			return err
		}
		return intctrlutil.NewRequeueError(reconcileInterval, "waiting for previous restore PVC")
	}
	return nil
}

func (r *VolumePopulatorReconciler) pvcNeedsDataRestore(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim) (bool, error) {
	if pvc.Spec.DataSourceRef == nil {
		return false, nil
	}
	backupNamespace := pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey]
	if backupNamespace == "" {
		backupNamespace = pvc.Namespace
	}
	backup := &dpv1alpha1.Backup{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: backupNamespace, Name: pvc.Spec.DataSourceRef.Name}, backup); err != nil {
		return false, err
	}
	if backup.Status.BackupMethod == nil {
		return false, intctrlutil.NewFatalError(fmt.Sprintf(`status.backupMethod of backup "%s" can not be empty`, backup.Name))
	}
	parameters, err := restoreParametersMapFromPVC(pvc)
	if err != nil {
		return false, intctrlutil.NewFatalError(err.Error())
	}
	decision, err := r.decidePVCRestore(reqCtx, pvc, backup, parameters)
	if err != nil {
		return false, err
	}
	return decision.mode == pvcRestoreModeRestoreData, nil
}

func (r *VolumePopulatorReconciler) ensurePostReadyRestoreCompleted(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	restoreCtx *pvcRestoreContext) (bool, error) {
	restoreMgr := restoreCtx.restoreMgr
	if len(restoreMgr.PostReadyBackupSets) == 0 {
		return true, nil
	}
	if restoreCtx.skipPostReady {
		return true, nil
	}
	if pvc.Annotations[constant.RestoreSourceKindAnnotationKey] == "" {
		return true, nil
	}
	allBound, err := r.allRestorePVCsForComponentBound(reqCtx, pvc)
	if err != nil || !allBound {
		if err == nil {
			err = r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingProcessing, "Waiting for all restore PVCs to finish prepareData")
		}
		return false, err
	}
	clusterName := pvc.Labels[constant.AppInstanceLabelKey]
	componentName := restoreComponentName(pvc)
	if clusterName == "" || componentName == "" {
		return false, intctrlutil.NewFatalError(fmt.Sprintf("missing cluster/component labels for PVC %s/%s postReady restore", pvc.Namespace, pvc.Name))
	}
	comp := &appsv1.Component{}
	if err = r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: pvc.Namespace,
		Name:      constant.GenerateClusterComponentName(clusterName, componentName),
	}, comp); err != nil {
		return false, err
	}
	if comp.Status.Phase != appsv1.RunningComponentPhase || componentPostProvisionRunning(comp) {
		if err = r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingProcessing, "Waiting for component to finish post-provision"); err != nil {
			return false, err
		}
		return false, nil
	}
	parameters, err := restoreParametersMapFromPVC(pvc)
	if err != nil {
		return false, intctrlutil.NewFatalError(err.Error())
	}
	if parameters[dptypes.DeferPostReadyUntilClusterRunningParameterKey] == "true" {
		cluster := &appsv1.Cluster{}
		if err = r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: pvc.Namespace, Name: clusterName}, cluster); err != nil {
			return false, err
		}
		if cluster.Status.Phase != appsv1.RunningClusterPhase {
			if err = r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingProcessing, "Waiting for cluster to run before postReady restore"); err != nil {
				return false, err
			}
			return false, nil
		}
	}
	postReadyRestore, err := r.buildPostReadyRestore(reqCtx, pvc, restoreMgr, comp)
	if err != nil {
		return false, err
	}
	existing := &dpv1alpha1.Restore{}
	if err = r.Client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(postReadyRestore), existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		if err = r.Client.Create(reqCtx.Ctx, postReadyRestore); err != nil && !apierrors.IsAlreadyExists(err) {
			return false, err
		}
		if err = r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingProcessing, "Waiting for postReady restore to complete"); err != nil {
			return false, err
		}
		return false, nil
	}
	switch existing.Status.Phase {
	case dpv1alpha1.RestorePhaseCompleted:
		return true, nil
	case dpv1alpha1.RestorePhaseFailed:
		return false, intctrlutil.NewFatalError(fmt.Sprintf("postReady restore %s/%s failed", existing.Namespace, existing.Name))
	default:
		if err = r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingProcessing, "Waiting for postReady restore to complete"); err != nil {
			return false, err
		}
		return false, nil
	}
}

func (r *VolumePopulatorReconciler) buildPostReadyRestore(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	restoreMgr *dprestore.RestoreManager,
	comp *appsv1.Component) (*dpv1alpha1.Restore, error) {
	clusterName := pvc.Labels[constant.AppInstanceLabelKey]
	componentName := restoreComponentName(pvc)
	backupNamespace := pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey]
	if backupNamespace == "" {
		backupNamespace = pvc.Namespace
	}
	parameters, err := restoreParametersMapFromPVC(pvc)
	if err != nil {
		return nil, intctrlutil.NewFatalError(err.Error())
	}
	sourceTarget, err := r.resolveSourceTarget(reqCtx, pvc, backupNamespace)
	if err != nil {
		return nil, err
	}
	connectionCredential, err := r.postReadyConnectionCredential(reqCtx, pvc, backupNamespace, clusterName, componentName, comp)
	if err != nil {
		return nil, err
	}
	jobActionLabels := constant.GetCompLabels(clusterName, componentName)
	roleName, err := r.highestPriorityRoleName(reqCtx, comp)
	if err != nil {
		return nil, err
	}
	if roleName != "" {
		jobActionLabels[instanceset.RoleLabelKey] = roleName
	}
	readyConfig := &dpv1alpha1.ReadyConfig{
		ExecAction: &dpv1alpha1.ExecAction{
			Target: dpv1alpha1.ExecActionTarget{
				PodSelector: metav1.LabelSelector{
					MatchLabels: constant.GetCompLabels(clusterName, componentName),
				},
			},
		},
		JobAction: &dpv1alpha1.JobAction{
			Target: dpv1alpha1.JobActionTarget{
				PodSelector: dpv1alpha1.PodSelector{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: jobActionLabels,
					},
				},
			},
		},
		ConnectionCredential: connectionCredential,
	}
	if sourceTarget != nil {
		if sourceTarget.PodSelector != nil {
			readyConfig.JobAction.Target.PodSelector.Strategy = sourceTarget.PodSelector.Strategy
		}
		readyConfig.JobAction.RequiredPolicyForAllPodSelection = postReadyRequiredPolicy(sourceTarget)
		backup := &dpv1alpha1.Backup{}
		if err = r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: backupNamespace, Name: pvc.Spec.DataSourceRef.Name}, backup); err != nil {
			return nil, err
		}
		if backup.Status.BackupMethod != nil && backup.Status.BackupMethod.TargetVolumes != nil {
			readyConfig.JobAction.Target.VolumeMounts = backup.Status.BackupMethod.TargetVolumes.VolumeMounts
		}
	}
	restore := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      postReadyRestoreName(clusterName, componentName, pvc.Spec.DataSourceRef.Name),
			Namespace: pvc.Namespace,
			Labels:    internalRestoreLabels(pvc),
		},
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:      pvc.Spec.DataSourceRef.Name,
				Namespace: backupNamespace,
			},
			RestoreTime: pvc.Annotations[constant.RestorePITRAnnotationKey],
			Env:         restoreMgr.Restore.Spec.Env,
			Parameters:  restoreParametersToPairs(restoreActionParameters(parameters)),
			ReadyConfig: readyConfig,
		},
	}
	if sourceTarget != nil {
		restore.Spec.Backup.SourceTargetName = sourceTarget.Name
	}
	if err = controllerutil.SetOwnerReference(comp, restore, r.Scheme); err != nil {
		return nil, err
	}
	return restore, nil
}

func (r *VolumePopulatorReconciler) highestPriorityRoleName(reqCtx intctrlutil.RequestCtx, comp *appsv1.Component) (string, error) {
	if comp.Spec.CompDef == "" {
		return "", nil
	}
	compDef := &appsv1.ComponentDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: comp.Spec.CompDef}, compDef); err != nil {
		return "", err
	}
	if len(compDef.Spec.Roles) == 0 {
		return "", nil
	}
	role := compDef.Spec.Roles[0]
	for i := 1; i < len(compDef.Spec.Roles); i++ {
		if compDef.Spec.Roles[i].UpdatePriority > role.UpdatePriority {
			role = compDef.Spec.Roles[i]
		}
	}
	return role.Name, nil
}

func (r *VolumePopulatorReconciler) allRestorePVCsForComponentBound(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim) (bool, error) {
	pvcs, err := r.listRestorePVCsForComponent(reqCtx, pvc)
	if err != nil {
		return false, err
	}
	for i := range pvcs {
		item := &pvcs[i]
		cond := findPVCConditionByType(item, appsv1.ConditionTypeRestore)
		if cond != nil && cond.Status == corev1.ConditionFalse {
			return false, intctrlutil.NewFatalError(fmt.Sprintf("restore PVC %s/%s failed: %s", item.Namespace, item.Name, cond.Message))
		}
		if item.Spec.VolumeName == "" {
			return false, nil
		}
	}
	return true, nil
}

func findPVCConditionByType(pvc *corev1.PersistentVolumeClaim, conditionType string) *corev1.PersistentVolumeClaimCondition {
	for i := range pvc.Status.Conditions {
		if string(pvc.Status.Conditions[i].Type) == conditionType {
			return &pvc.Status.Conditions[i]
		}
	}
	return nil
}

func (r *VolumePopulatorReconciler) listRestorePVCsForComponent(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim) ([]corev1.PersistentVolumeClaim, error) {
	clusterName := pvc.Labels[constant.AppInstanceLabelKey]
	componentName := restoreComponentName(pvc)
	if clusterName == "" || componentName == "" || pvc.Spec.DataSourceRef == nil {
		return []corev1.PersistentVolumeClaim{*pvc}, nil
	}
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := r.Client.List(reqCtx.Ctx, pvcList, client.InNamespace(pvc.Namespace),
		client.MatchingLabels(constant.GetCompLabels(clusterName, componentName))); err != nil {
		return nil, err
	}
	var result []corev1.PersistentVolumeClaim
	sourceNamespace := pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey]
	for i := range pvcList.Items {
		item := pvcList.Items[i]
		if item.Spec.DataSourceRef == nil || item.Spec.DataSourceRef.Name != pvc.Spec.DataSourceRef.Name {
			continue
		}
		if item.Annotations[constant.RestoreSourceKindAnnotationKey] == "" {
			continue
		}
		if item.Annotations[constant.RestoreSourceNamespaceAnnotationKey] != sourceNamespace {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func restoreComponentName(pvc *corev1.PersistentVolumeClaim) string {
	if componentName := pvc.Labels[constant.KBAppComponentLabelKey]; componentName != "" {
		return componentName
	}
	return pvc.Annotations[constant.RestoreComponentAnnotationKey]
}

func componentPostProvisionRunning(comp *appsv1.Component) bool {
	cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ComponentConditionProgressing)
	return cond != nil && cond.Status == metav1.ConditionTrue && cond.Reason == "PostProvision"
}

func (r *VolumePopulatorReconciler) postReadyConnectionCredential(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	backupNamespace, clusterName, componentName string,
	comp *appsv1.Component) (*dpv1alpha1.ConnectionCredential, error) {
	accountName, err := r.postReadySystemAccountName(reqCtx, comp)
	if err != nil {
		return nil, err
	}
	if accountName == "" {
		backup := &dpv1alpha1.Backup{}
		if err = r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: backupNamespace, Name: pvc.Spec.DataSourceRef.Name}, backup); err != nil {
			return nil, err
		}
		accountsByComponent := map[string]map[string]string{}
		if encryptedAccounts := backup.Annotations[constant.EncryptedSystemAccountsAnnotationKey]; encryptedAccounts != "" {
			if err = json.Unmarshal([]byte(encryptedAccounts), &accountsByComponent); err != nil {
				return nil, intctrlutil.NewFatalError(err.Error())
			}
		}
		accountNames := make([]string, 0, len(accountsByComponent[componentName]))
		for name := range accountsByComponent[componentName] {
			accountNames = append(accountNames, name)
		}
		slices.Sort(accountNames)
		if len(accountNames) > 0 {
			accountName = accountNames[0]
		}
		if accountName == "" {
			return nil, nil
		}
	}
	return &dpv1alpha1.ConnectionCredential{
		SecretName:  constant.GenerateAccountSecretName(clusterName, componentName, accountName),
		PasswordKey: constant.AccountPasswdForSecret,
		UsernameKey: constant.AccountNameForSecret,
	}, nil
}

func (r *VolumePopulatorReconciler) postReadySystemAccountName(reqCtx intctrlutil.RequestCtx, comp *appsv1.Component) (string, error) {
	if comp.Spec.CompDef == "" {
		return "", nil
	}
	compDef := &appsv1.ComponentDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: comp.Spec.CompDef}, compDef); err != nil {
		return "", err
	}
	disabled := map[string]bool{}
	for i := range comp.Spec.SystemAccounts {
		if comp.Spec.SystemAccounts[i].Disabled != nil && *comp.Spec.SystemAccounts[i].Disabled {
			disabled[comp.Spec.SystemAccounts[i].Name] = true
		}
	}
	firstAccount := ""
	for i := range compDef.Spec.SystemAccounts {
		account := compDef.Spec.SystemAccounts[i]
		if disabled[account.Name] {
			continue
		}
		if firstAccount == "" {
			firstAccount = account.Name
		}
		if account.InitAccount {
			return account.Name, nil
		}
	}
	return firstAccount, nil
}

func postReadyRequiredPolicy(sourceTarget *dpv1alpha1.BackupStatusTarget) *dpv1alpha1.RequiredPolicyForAllPodSelection {
	if sourceTarget == nil || sourceTarget.PodSelector == nil || sourceTarget.PodSelector.Strategy != dpv1alpha1.PodSelectionStrategyAll {
		return nil
	}
	return &dpv1alpha1.RequiredPolicyForAllPodSelection{
		DataRestorePolicy: dpv1alpha1.OneToOneRestorePolicy,
	}
}

func postReadyRestoreName(clusterName, componentName, backupName string) string {
	return constant.ShortenKubeName(fmt.Sprintf("%s-%s-%s-post-ready", clusterName, componentName, backupName), constant.KubeNameMaxLength)
}

func (r *VolumePopulatorReconciler) Cleanup(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim) error {
	if slices.Contains(pvc.Finalizers, dptypes.DataProtectionFinalizerName) {
		pvcPatch := client.MergeFrom(pvc.DeepCopy())
		controllerutil.RemoveFinalizer(pvc, dptypes.DataProtectionFinalizerName)
		if err := r.Client.Patch(reqCtx.Ctx, pvc, pvcPatch); err != nil {
			return err
		}
	}

	jobs := &batchv1.JobList{}
	if err := r.Client.List(reqCtx.Ctx, jobs,
		client.InNamespace(pvc.Namespace), client.MatchingLabels(map[string]string{
			dprestore.DataProtectionPopulatePVCLabelKey: getPopulatePVCName(pvc.UID),
		})); err != nil {
		return err
	}

	for i := range jobs.Items {
		job := &jobs.Items[i]
		if controllerutil.ContainsFinalizer(job, dptypes.DataProtectionFinalizerName) {
			patch := client.MergeFrom(job.DeepCopy())
			controllerutil.RemoveFinalizer(job, dptypes.DataProtectionFinalizerName)
			if err := r.Patch(reqCtx.Ctx, job, patch); err != nil {
				return err
			}
		}
		if !job.DeletionTimestamp.IsZero() {
			continue
		}
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, job); err != nil {
			return err
		}
	}

	populatePVC := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: getPopulatePVCName(pvc.UID),
		Namespace: pvc.Namespace}, populatePVC); err != nil {
		return client.IgnoreNotFound(err)
	}
	return r.Client.Delete(reqCtx.Ctx, populatePVC)
}

func (r *VolumePopulatorReconciler) checkIntreeStorageClass(pvc *corev1.PersistentVolumeClaim, sc *storagev1.StorageClass) error {
	if !strings.HasPrefix(sc.Provisioner, "kubernetes.io/") {
		// This is not an in-tree StorageClass
		return nil
	}

	if pvc.Annotations != nil {
		if migrated := pvc.Annotations[volume.AnnMigratedTo]; migrated != "" {
			// The PVC is migrated to CSI
			return nil
		}
	}
	// The SC is in-tree & PVC is not migrated
	return intctrlutil.NewFatalError(fmt.Sprintf("in-tree volume volume plugin %q cannot use volume populator", sc.Provisioner))
}

func (r *VolumePopulatorReconciler) waitForPVCSelectedNode(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim) (bool, string, error) {
	var nodeName string
	if pvc.Spec.StorageClassName != nil {
		storageClassName := *pvc.Spec.StorageClassName
		storageClass := &storagev1.StorageClass{}
		if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: storageClassName}, storageClass); err != nil {
			return false, nodeName, err
		}

		if err := r.checkIntreeStorageClass(pvc, storageClass); err != nil {
			return false, nodeName, err
		}
		if storageClass.VolumeBindingMode != nil && storagev1.VolumeBindingWaitForFirstConsumer == *storageClass.VolumeBindingMode {
			nodeName = pvc.Annotations[AnnSelectedNode]
			if nodeName == "" {
				// Wait for the PVC to get a node name before continuing
				return true, nodeName, nil
			}
		}
	}
	return false, nodeName, nil
}

func (r *VolumePopulatorReconciler) getPopulatePVC(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	backupSet dprestore.BackupActionSet,
	restore *dpv1alpha1.Restore,
	target *dpv1alpha1.BackupStatusTarget,
	nodeName string) (*corev1.PersistentVolumeClaim, error) {
	populatePVCName := getPopulatePVCName(pvc.UID)
	populatePVC := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: populatePVCName,
		Namespace: pvc.Namespace}, populatePVC); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		// create populate pvc
		populatePVC = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      populatePVCName,
				Namespace: pvc.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      pvc.Spec.AccessModes,
				Resources:        pvc.Spec.Resources,
				StorageClassName: pvc.Spec.StorageClassName,
				VolumeMode:       pvc.Spec.VolumeMode,
			},
		}
		if nodeName != "" {
			populatePVC.Annotations = map[string]string{
				AnnSelectedNode: pvc.Annotations[AnnSelectedNode],
			}
		}

		if backupSet.UseVolumeSnapshot {
			// TODO: will be removed in 0.10.0, compatibility handling for version 0.8.
			prepareDataConfig := restore.Spec.PrepareDataConfig
			vsName := utils.GetOldBackupVolumeSnapshotName(backupSet.Backup.Name, prepareDataConfig.DataSourceRef.VolumeSource)
			vsCli := utils.NewCompatClient(r.Client)
			exist, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, vsCli,
				types.NamespacedName{Namespace: backupSet.Backup.Namespace, Name: vsName},
				&vsv1.VolumeSnapshot{})
			if err != nil {
				return nil, err
			}
			if !exist {
				sourceTargetPodName, err := dprestore.GetSourcePodNameFromTarget(target, prepareDataConfig.RequiredPolicyForAllPodSelection, 0)
				if err != nil {
					return nil, err
				}
				if target.PodSelector.Strategy == dpv1alpha1.PodSelectionStrategyAny || sourceTargetPodName != "" {
					snapshotGroup := dprestore.GetVolumeSnapshotsBySourcePod(backupSet.Backup, target, sourceTargetPodName)
					if snapshotGroup == nil {
						message := fmt.Sprintf(`can not found the volumeSnapshot in status.actions, sourceTargetPod is "%s"`, sourceTargetPodName)
						return nil, intctrlutil.NewFatalError(message)
					}
					vsName = snapshotGroup[prepareDataConfig.DataSourceRef.VolumeSource]
				}
			}
			// restore from volume snapshot.
			populatePVC.Spec.DataSourceRef = &corev1.TypedObjectReference{
				Name:     vsName,
				Kind:     constant.VolumeSnapshotKind,
				APIGroup: &dprestore.VolumeSnapshotGroup,
			}
		}
		if err = r.Client.Create(reqCtx.Ctx, populatePVC); err != nil && !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
	}
	return populatePVC, nil
}

func (r *VolumePopulatorReconciler) getProvisionOnlyPVC(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	nodeName string) (*corev1.PersistentVolumeClaim, error) {
	populatePVCName := getPopulatePVCName(pvc.UID)
	populatePVC := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: populatePVCName,
		Namespace: pvc.Namespace}, populatePVC); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		populatePVC = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      populatePVCName,
				Namespace: pvc.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      pvc.Spec.AccessModes,
				Resources:        pvc.Spec.Resources,
				StorageClassName: pvc.Spec.StorageClassName,
				VolumeMode:       pvc.Spec.VolumeMode,
			},
		}
		if nodeName != "" {
			populatePVC.Annotations = map[string]string{
				AnnSelectedNode: pvc.Annotations[AnnSelectedNode],
			}
		}
		if err = r.Client.Create(reqCtx.Ctx, populatePVC); err != nil && !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
	}
	return populatePVC, nil
}

func (r *VolumePopulatorReconciler) rebindPVCAndPV(reqCtx intctrlutil.RequestCtx, populatePVC, pvc *corev1.PersistentVolumeClaim) (bool, error) {
	if populatePVC.Spec.VolumeName == "" {
		return false, nil
	}
	pv := &corev1.PersistentVolume{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: populatePVC.Spec.VolumeName}, pv); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		// We'll get called again later when the PV exists
		return false, nil
	}
	// Examine the claimref for the PV and see if it's bound to the correct PVC
	claimRef := pv.Spec.ClaimRef
	if claimRef != nil && claimRef.Name == pvc.Name && claimRef.Namespace == pvc.Namespace && claimRef.UID == pvc.UID {
		return true, nil
	}
	// Make new PV with strategic patch values to perform the PV rebind
	patchPV := client.MergeFrom(pv.DeepCopy())
	pv.Spec.ClaimRef = &corev1.ObjectReference{
		Namespace:       pvc.Namespace,
		Name:            pvc.Name,
		UID:             pvc.UID,
		ResourceVersion: pvc.ResourceVersion,
	}
	if pv.Annotations == nil {
		pv.Annotations = map[string]string{}
	}
	pv.Annotations[AnnPopulateFrom] = pvc.Spec.DataSourceRef.Name
	return true, r.Client.Patch(reqCtx.Ctx, pv, patchPV)
}

func (r *VolumePopulatorReconciler) UpdatePVCConditions(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim, reason, message string) error {
	progressCondition := corev1.PersistentVolumeClaimCondition{
		Type:               PersistentVolumeClaimPopulating,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
	restoreCondition := corev1.PersistentVolumeClaimCondition{
		Type:               corev1.PersistentVolumeClaimConditionType(appsv1.ConditionTypeRestore),
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
	switch reason {
	case ReasonPopulatingSucceed, ReasonPopulatingProvisioned:
		restoreCondition.Status = corev1.ConditionTrue
	case ReasonPopulatingFailed:
		restoreCondition.Status = corev1.ConditionFalse
	}
	pvcPatch := client.MergeFrom(pvc.DeepCopy())
	var existPopulating bool
	for i, v := range pvc.Status.Conditions {
		if v.Type != PersistentVolumeClaimPopulating {
			continue
		}
		if reason == v.Reason {
			if pvcConditionMatches(pvc.Status.Conditions, restoreCondition) {
				return nil
			}
			existPopulating = true
			pvc.Status.Conditions[i] = progressCondition
			continue
		}
		if v.Reason == ReasonPopulatingSucceed {
			// ignore succeed condition
			if pvcConditionMatches(pvc.Status.Conditions, restoreCondition) {
				return nil
			}
			existPopulating = true
			continue
		}
		existPopulating = true
		pvc.Status.Conditions[i] = progressCondition
	}
	if !existPopulating {
		pvc.Status.Conditions = append(pvc.Status.Conditions, progressCondition)
	}
	upsertPVCCondition(&pvc.Status.Conditions, restoreCondition)
	switch reason {
	case ReasonPopulatingProcessing:
		r.Recorder.Event(pvc, corev1.EventTypeNormal, ReasonStartToVolumePopulate, message)
	case ReasonPopulatingSucceed, ReasonPopulatingProvisioned:
		r.Recorder.Event(pvc, corev1.EventTypeNormal, ReasonVolumePopulateSucceed, message)
	}
	return r.Client.Status().Patch(reqCtx.Ctx, pvc, pvcPatch)
}

func pvcConditionMatches(conditions []corev1.PersistentVolumeClaimCondition, condition corev1.PersistentVolumeClaimCondition) bool {
	for i := range conditions {
		existing := conditions[i]
		if existing.Type == condition.Type && existing.Status == condition.Status && existing.Reason == condition.Reason {
			return true
		}
	}
	return false
}

func (r *VolumePopulatorReconciler) restoreSystemAccountSecrets(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim, backupNamespace string) error {
	if pvc.Spec.DataSourceRef == nil || pvc.Spec.DataSourceRef.Name == "" {
		return nil
	}
	backup := &dpv1alpha1.Backup{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: backupNamespace, Name: pvc.Spec.DataSourceRef.Name}, backup); err != nil {
		return err
	}
	encryptedAccounts := backup.Annotations[constant.EncryptedSystemAccountsAnnotationKey]
	if encryptedAccounts == "" {
		return nil
	}
	accountsByComponent := map[string]map[string]string{}
	if err := json.Unmarshal([]byte(encryptedAccounts), &accountsByComponent); err != nil {
		return intctrlutil.NewFatalError(err.Error())
	}
	clusterName := pvc.Labels[constant.AppInstanceLabelKey]
	if clusterName == "" {
		return nil
	}
	componentName := pvc.Labels[constant.KBAppComponentLabelKey]
	if componentName == "" {
		componentName = pvc.Annotations[constant.RestoreComponentAnnotationKey]
	}
	encryptor := intctrlutil.NewEncryptor(viper.GetString(constant.CfgKeyDPEncryptionKey))
	if componentName != "" {
		labels := map[string]string{
			constant.AppInstanceLabelKey:        clusterName,
			constant.KBAppComponentLabelKey:     componentName,
			"apps.kubeblocks.io/system-account": "",
		}
		if err := r.restoreSystemAccountSecretSet(reqCtx, pvc, encryptor, accountsByComponent[componentName],
			systemAccountSecretScopeComponent, clusterName, componentName, labels); err != nil {
			return err
		}
	}
	if shardingName := pvc.Labels[constant.KBAppShardingNameLabelKey]; shardingName != "" {
		labels := map[string]string{
			constant.AppInstanceLabelKey:        clusterName,
			constant.KBAppShardingNameLabelKey:  shardingName,
			"apps.kubeblocks.io/system-account": "",
		}
		if err := r.restoreSystemAccountSecretSet(reqCtx, pvc, encryptor, accountsByComponent[shardingName],
			systemAccountSecretScopeSharding, clusterName, shardingName, labels); err != nil {
			return err
		}
	}
	return nil
}

type systemAccountSecretScope string

const (
	systemAccountSecretScopeComponent systemAccountSecretScope = "component"
	systemAccountSecretScopeSharding  systemAccountSecretScope = "sharding"
)

func (r *VolumePopulatorReconciler) restoreSystemAccountSecretSet(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	encryptor interface {
		Decrypt([]byte) (string, error)
	},
	accounts map[string]string,
	scope systemAccountSecretScope,
	clusterName, ownerName string,
	labels map[string]string) error {
	for accountName, encryptedPassword := range accounts {
		password, err := encryptor.Decrypt([]byte(encryptedPassword))
		if err != nil {
			return intctrlutil.NewFatalError(err.Error())
		}
		accountLabels := mapsClone(labels)
		accountLabels["apps.kubeblocks.io/system-account"] = accountName
		if err = r.upsertSystemAccountSecret(reqCtx, pvc, scope, clusterName, ownerName, accountName, []byte(password), accountLabels); err != nil {
			return err
		}
	}
	return nil
}

func mapsClone(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (r *VolumePopulatorReconciler) upsertSystemAccountSecret(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	scope systemAccountSecretScope,
	clusterName, ownerName, accountName string,
	password []byte,
	labels map[string]string) error {
	secretName := systemAccountSecretName(scope, clusterName, ownerName, accountName)
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: pvc.Namespace, Name: secretName}
	if err := r.Client.Get(reqCtx.Ctx, key, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: pvc.Namespace,
				Labels:    labels,
				Annotations: map[string]string{
					constant.SystemAccountProvisionedAnnotationKey: "true",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				constant.AccountNameForSecret:   []byte(accountName),
				constant.AccountPasswdForSecret: password,
			},
		}
		if err := r.setSystemAccountSecretOwner(reqCtx, pvc.Namespace, secret, scope, clusterName, ownerName); err != nil {
			return err
		}
		return r.Client.Create(reqCtx.Ctx, secret)
	}
	if secret.Immutable != nil && *secret.Immutable && !systemAccountSecretMatches(secret, accountName, password) {
		if err := r.Client.Delete(reqCtx.Ctx, secret); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: pvc.Namespace,
				Labels:    labels,
				Annotations: map[string]string{
					constant.SystemAccountProvisionedAnnotationKey: "true",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				constant.AccountNameForSecret:   []byte(accountName),
				constant.AccountPasswdForSecret: password,
			},
		}
		if err := r.setSystemAccountSecretOwner(reqCtx, pvc.Namespace, secret, scope, clusterName, ownerName); err != nil {
			return err
		}
		if err := r.Client.Create(reqCtx.Ctx, secret); err != nil {
			return err
		}
		return nil
	}
	patch := client.MergeFrom(secret.DeepCopy())
	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	for k, v := range labels {
		secret.Labels[k] = v
	}
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	secret.Annotations[constant.SystemAccountProvisionedAnnotationKey] = "true"
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[constant.AccountNameForSecret] = []byte(accountName)
	secret.Data[constant.AccountPasswdForSecret] = password
	if err := r.setSystemAccountSecretOwner(reqCtx, pvc.Namespace, secret, scope, clusterName, ownerName); err != nil {
		return err
	}
	return r.Client.Patch(reqCtx.Ctx, secret, patch)
}

func (r *VolumePopulatorReconciler) setSystemAccountSecretOwner(reqCtx intctrlutil.RequestCtx,
	namespace string,
	secret *corev1.Secret,
	scope systemAccountSecretScope,
	clusterName, ownerName string) error {
	switch scope {
	case systemAccountSecretScopeSharding:
		cluster := &appsv1.Cluster{}
		if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: namespace, Name: clusterName}, cluster); err != nil {
			return err
		}
		return controllerutil.SetOwnerReference(cluster, secret, r.Scheme)
	default:
		component := &appsv1.Component{}
		if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
			Namespace: namespace,
			Name:      constant.GenerateClusterComponentName(clusterName, ownerName),
		}, component); err != nil {
			return err
		}
		return controllerutil.SetOwnerReference(component, secret, r.Scheme)
	}
}

func systemAccountSecretName(scope systemAccountSecretScope, clusterName, ownerName, accountName string) string {
	if scope == systemAccountSecretScopeSharding {
		return fmt.Sprintf("%s-%s-%s", clusterName, ownerName, accountName)
	}
	return constant.GenerateAccountSecretName(clusterName, ownerName, accountName)
}

func systemAccountSecretMatches(secret *corev1.Secret, accountName string, password []byte) bool {
	return string(secret.Data[constant.AccountNameForSecret]) == accountName &&
		string(secret.Data[constant.AccountPasswdForSecret]) == string(password)
}

func upsertPVCCondition(conditions *[]corev1.PersistentVolumeClaimCondition, condition corev1.PersistentVolumeClaimCondition) {
	for i := range *conditions {
		if (*conditions)[i].Type == condition.Type {
			(*conditions)[i] = condition
			return
		}
	}
	*conditions = append(*conditions, condition)
}
