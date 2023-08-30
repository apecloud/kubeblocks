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

package plan

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/factory"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// RestoreManager restores manager functions
// 1. support datafile/snapshot restore
// 2. support point in time recovery (PITR)
type RestoreManager struct {
	client.Client
	Ctx     context.Context
	Cluster *appsv1alpha1.Cluster
	Scheme  *k8sruntime.Scheme

	// private
	namespace     string
	restoreTime   *metav1.Time
	sourceCluster string
}

func NewRestoreManager(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, scheme *k8sruntime.Scheme) *RestoreManager {
	return &RestoreManager{
		Cluster: cluster,
		Client:  cli,
		Ctx:     ctx,
		Scheme:  scheme,
	}
}

func (r *RestoreManager) DoRestore(comp *component.SynthesizedComponent) error {
	backupObj, err := r.getBackupObjectFromAnnotation(comp)
	if err != nil {
		return err
	}
	if backupObj == nil {
		return nil
	}
	if backupObj.Status.BackupMethod == nil {
		return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRestoreFailed, `status.backupMethod of backup "%s" can not be empty`, backupObj.Name)
	}
	if err = r.DoPrepareData(comp, backupObj); err != nil {
		return err
	}
	if err = r.DoPostReady(comp, backupObj); err != nil {
		return err
	}
	// do clean up
	if err = r.cleanupClusterAnnotations(); err != nil {
		return err
	}
	return r.cleanupRestores(comp)
}

func (r *RestoreManager) DoPrepareData(comp *component.SynthesizedComponent, backupObj *dpv1alpha1.Backup) error {
	targetVolumes := backupObj.Status.BackupMethod.TargetVolumes
	if targetVolumes == nil {
		return nil
	}
	getClusterJson := func() string {
		clusterSpec := r.Cluster.DeepCopy()
		clusterSpec.ObjectMeta = metav1.ObjectMeta{
			Name: clusterSpec.GetName(),
			UID:  clusterSpec.GetUID(),
		}
		clusterSpec.Status = appsv1alpha1.ClusterStatus{}
		b, _ := json.Marshal(*clusterSpec)
		return string(b)
	}

	var templates []dpv1alpha1.RestoreVolumeClaim
	pvcLabels := factory.BuildCommonLabels(r.Cluster, comp)
	for _, v := range comp.VolumeClaimTemplates {
		if !r.existVolumeSource(targetVolumes, v.Name) {
			continue
		}
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("%s-%s-%s", v.Name, r.Cluster.Name, comp.Name),
				Labels: pvcLabels,
				Annotations: map[string]string{
					// satisfy the detection of transformer_halt_recovering.
					constant.LastAppliedClusterAnnotationKey: getClusterJson(),
				},
			},
		}
		// build pvc labels
		factory.BuildPersistentVolumeClaimLabels(comp, pvc, v.Name)
		claimTemplate := dpv1alpha1.RestoreVolumeClaim{
			ObjectMeta:      pvc.ObjectMeta,
			VolumeClaimSpec: v.Spec,
			VolumeConfig: dpv1alpha1.VolumeConfig{
				VolumeSource: v.Name,
			},
		}
		templates = append(templates, claimTemplate)
	}
	if len(templates) == 0 {
		return nil
	}
	restore := &dpv1alpha1.Restore{
		ObjectMeta: r.getRestoreObjectMeta(comp, dpv1alpha1.PrepareData),
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupConfig{
				Name: backupObj.Name,
				// TODO: get namespace from annotations
				Namespace: r.namespace,
			},
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
				// TODO: get from annotations
				VolumeClaimManagementPolicy: dpv1alpha1.ParallelManagementPolicy,
				RestoreVolumeClaimsTemplate: &dpv1alpha1.RestoreVolumeClaimsTemplate{
					Replicas:  comp.Replicas,
					Templates: templates,
				},
			},
		},
	}
	return r.createRestoreAndWait(restore)
}

func (r *RestoreManager) DoPostReady(comp *component.SynthesizedComponent, backupObj *dpv1alpha1.Backup) error {
	compStatus := r.Cluster.Status.Components[comp.Name]
	if compStatus.Phase != appsv1alpha1.RunningClusterCompPhase {
		return nil
	}
	jobActionLabels := factory.BuildCommonLabels(r.Cluster, comp)
	if comp.WorkloadType == appsv1alpha1.Consensus || comp.WorkloadType == appsv1alpha1.Replication {
		// TODO: use rsm constant
		rsmAccessModeLabelKey := "rsm.workloads.kubeblocks.io/access-mode"
		jobActionLabels[rsmAccessModeLabelKey] = string(appsv1alpha1.ReadWrite)
	}
	restore := &dpv1alpha1.Restore{
		ObjectMeta: r.getRestoreObjectMeta(comp, dpv1alpha1.PostReady),
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupConfig{
				Name:      backupObj.Name,
				Namespace: r.namespace,
			},
			ReadyConfig: &dpv1alpha1.ReadyConfig{
				ExecAction: &dpv1alpha1.ExecAction{
					Target: dpv1alpha1.ExecActionTarget{
						PodSelector: metav1.LabelSelector{
							MatchLabels: factory.BuildCommonLabels(r.Cluster, comp),
						},
					},
				},
				JobAction: &dpv1alpha1.JobAction{
					Target: dpv1alpha1.JobActionTarget{
						PodSelector: metav1.LabelSelector{
							MatchLabels: jobActionLabels,
						},
					},
				},
			},
		},
	}
	return r.createRestoreAndWait(restore)
}

func (r *RestoreManager) getRestoreObjectMeta(comp *component.SynthesizedComponent, stage dpv1alpha1.RestoreStage) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      fmt.Sprintf("%s-%s-%s", r.Cluster.Name, r.Cluster.UID[:8], strings.ToLower(string(stage))),
		Namespace: r.Cluster.Namespace,
		Labels:    factory.BuildCommonLabels(r.Cluster, comp),
	}
}

// existVolumeSource checks if the backup.status.backupMethod.targetVolumes exists the target volume which should be restored.
func (r *RestoreManager) existVolumeSource(targetVolumes *dpv1alpha1.TargetVolumeInfo, volumeName string) bool {
	for _, v := range targetVolumes.Volumes {
		if v == volumeName {
			return true
		}
	}
	for _, v := range targetVolumes.VolumeMounts {
		if v.Name == volumeName {
			return true
		}
	}
	return false
}

// getBackupObjectFromAnnotation gets the backup object from the cluster annotations.
func (r *RestoreManager) getBackupObjectFromAnnotation(synthesizedComponent *component.SynthesizedComponent) (*dpv1alpha1.Backup, error) {
	valueString := r.Cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]
	if len(valueString) == 0 {
		return nil, nil
	}
	backupMap := map[string]string{}
	err := json.Unmarshal([]byte(valueString), &backupMap)
	if err != nil {
		return nil, err
	}
	backupSourceName, ok := backupMap[synthesizedComponent.Name]
	if !ok {
		return nil, nil
	}
	backup := &dpv1alpha1.Backup{}
	if err = r.Client.Get(r.Ctx, types.NamespacedName{Name: backupSourceName, Namespace: r.Cluster.Namespace}, backup); err != nil {
		return nil, err
	}
	return backup, nil
}

// createRestoreAndWait create the restore CR and wait for completion.
func (r *RestoreManager) createRestoreAndWait(restore *dpv1alpha1.Restore) error {
	controllerutil.SetControllerReference(r.Cluster, restore, r.Scheme)
	if err := r.Client.Get(r.Ctx, client.ObjectKeyFromObject(restore), restore); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err = r.Client.Create(r.Ctx, restore); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	if restore.Status.Phase == dpv1alpha1.RestorePhaseCompleted {
		return nil
	}
	if restore.Status.Phase == dpv1alpha1.RestorePhaseFailed {
		return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRestoreFailed, `restore "%s" is Failed, you can describe it and re-restore the cluster.`, restore.GetName())
	}
	return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, `waiting for restore "%s" successfully`, restore.GetName())
}

func (r *RestoreManager) cleanupClusterAnnotations() error {
	if r.Cluster.Status.Phase == appsv1alpha1.RunningClusterPhase && r.Cluster.Annotations != nil {
		cluster := r.Cluster
		patch := client.MergeFrom(cluster.DeepCopy())
		delete(cluster.Annotations, constant.RestoreFromSrcClusterAnnotationKey)
		delete(cluster.Annotations, constant.RestoreFromTimeAnnotationKey)
		delete(cluster.Annotations, constant.RestoreFromBackUpAnnotationKey)
		return r.Client.Patch(r.Ctx, cluster, patch)
	}
	return nil
}

func (r *RestoreManager) cleanupRestores(comp *component.SynthesizedComponent) error {
	if r.Cluster.Status.Phase != appsv1alpha1.RunningClusterPhase {
		return nil
	}
	restoreList := &dpv1alpha1.RestoreList{}
	if err := r.Client.List(r.Ctx, restoreList, client.MatchingLabels(factory.BuildCommonLabels(r.Cluster, comp))); err != nil {
		return err
	}
	for i := range restoreList.Items {
		if err := intctrlutil.BackgroundDeleteObject(r.Client, r.Ctx, &restoreList.Items[i]); err != nil {
			return err
		}
	}
	return nil
}
