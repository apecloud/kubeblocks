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
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/scheduling"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
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
	namespace                         string
	restoreTime                       string
	volumeRestorePolicy               dpv1alpha1.VolumeClaimRestorePolicy
	doReadyRestoreAfterClusterRunning bool
	startingIndex                     int32
	replicas                          int32
	restoreLabels                     map[string]string
}

func NewRestoreManager(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	scheme *k8sruntime.Scheme,
	restoreLabels map[string]string,
	replicas, startingIndex int32,
) *RestoreManager {
	return &RestoreManager{
		Cluster:             cluster,
		Client:              cli,
		Ctx:                 ctx,
		Scheme:              scheme,
		replicas:            replicas,
		startingIndex:       startingIndex,
		namespace:           cluster.Namespace,
		volumeRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicyParallel,
		restoreLabels:       restoreLabels,
	}
}

func (r *RestoreManager) DoRestore(comp *component.SynthesizedComponent, compObj *appsv1alpha1.Component) error {
	backupObj, err := r.initFromAnnotation(comp)
	if err != nil {
		return err
	}
	if backupObj == nil {
		return nil
	}
	if backupObj.Status.BackupMethod == nil {
		return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRestoreFailed, `status.backupMethod of backup "%s" can not be empty`, backupObj.Name)
	}
	if err = r.DoPrepareData(comp, compObj, backupObj); err != nil {
		return err
	}
	if compObj.Status.Phase != appsv1alpha1.RunningClusterCompPhase {
		return nil
	}
	if r.doReadyRestoreAfterClusterRunning && r.Cluster.Status.Phase != appsv1alpha1.RunningClusterPhase {
		return nil
	}
	if err = r.DoPostReady(comp, compObj, backupObj); err != nil {
		return err
	}
	// do clean up
	return r.cleanupClusterAnnotations(comp.Name)
}

func (r *RestoreManager) DoPrepareData(comp *component.SynthesizedComponent,
	compObj *appsv1alpha1.Component,
	backupObj *dpv1alpha1.Backup) error {
	restore, err := r.BuildPrepareDataRestore(comp, backupObj)
	if err != nil {
		return err
	}
	return r.createRestoreAndWait(restore, compObj)
}

func (r *RestoreManager) BuildPrepareDataRestore(comp *component.SynthesizedComponent, backupObj *dpv1alpha1.Backup) (*dpv1alpha1.Restore, error) {
	backupMethod := backupObj.Status.BackupMethod
	if backupMethod == nil {
		return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeRestoreFailed, `status.backupMethod of backup "%s" can not be empty`, backupObj.Name)
	}
	targetVolumes := backupMethod.TargetVolumes
	if targetVolumes == nil {
		return nil, nil
	}
	getClusterJSON := func() string {
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
	pvcLabels := constant.GetKBWellKnownLabels(comp.ClusterDefName, r.Cluster.Name, comp.Name)
	for _, v := range comp.VolumeClaimTemplates {
		if !dputils.ExistTargetVolume(targetVolumes, v.Name) {
			continue
		}
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("%s-%s-%s", v.Name, r.Cluster.Name, comp.Name),
				Labels: pvcLabels,
				Annotations: map[string]string{
					// satisfy the detection of transformer_halt_recovering.
					constant.LastAppliedClusterAnnotationKey: getClusterJSON(),
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
		return nil, nil
	}
	schedulingSpec, err := r.buildSchedulingSpec(comp)
	if err != nil {
		return nil, err
	}
	restore := &dpv1alpha1.Restore{
		ObjectMeta: r.GetRestoreObjectMeta(comp, dpv1alpha1.PrepareData),
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:      backupObj.Name,
				Namespace: r.namespace,
			},
			RestoreTime: r.restoreTime,
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
				SchedulingSpec:           schedulingSpec,
				VolumeClaimRestorePolicy: r.volumeRestorePolicy,
				RestoreVolumeClaimsTemplate: &dpv1alpha1.RestoreVolumeClaimsTemplate{
					Replicas:      r.replicas,
					StartingIndex: r.startingIndex,
					Templates:     templates,
				},
			},
		},
	}
	return restore, nil
}

func (r *RestoreManager) DoPostReady(comp *component.SynthesizedComponent,
	compObj *appsv1alpha1.Component,
	backupObj *dpv1alpha1.Backup) error {
	jobActionLabels := constant.GetComponentWellKnownLabels(r.Cluster.Name, comp.Name)
	if comp.WorkloadType == appsv1alpha1.Consensus || comp.WorkloadType == appsv1alpha1.Replication {
		// TODO: use rsm constant
		rsmAccessModeLabelKey := "rsm.workloads.kubeblocks.io/access-mode"
		jobActionLabels[rsmAccessModeLabelKey] = string(appsv1alpha1.ReadWrite)
	}
	// TODO: get connect credential from backupPolicyTemplate
	restore := &dpv1alpha1.Restore{
		ObjectMeta: r.GetRestoreObjectMeta(comp, dpv1alpha1.PostReady),
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:      backupObj.Name,
				Namespace: r.namespace,
			},
			RestoreTime: r.restoreTime,
			ReadyConfig: &dpv1alpha1.ReadyConfig{
				ExecAction: &dpv1alpha1.ExecAction{
					Target: dpv1alpha1.ExecActionTarget{
						PodSelector: metav1.LabelSelector{
							MatchLabels: constant.GetKBWellKnownLabels(comp.ClusterDefName, r.Cluster.Name, comp.Name),
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
	backupMethod := backupObj.Status.BackupMethod
	if backupMethod.TargetVolumes != nil {
		restore.Spec.ReadyConfig.JobAction.Target.VolumeMounts = backupMethod.TargetVolumes.VolumeMounts
	}
	return r.createRestoreAndWait(restore, compObj)
}

func (r *RestoreManager) buildSchedulingSpec(comp *component.SynthesizedComponent) (dpv1alpha1.SchedulingSpec, error) {
	schedulingPolicy, err := scheduling.BuildSchedulingPolicy(r.Cluster, r.Cluster.Spec.GetComponentByName(comp.Name))
	if err != nil {
		return dpv1alpha1.SchedulingSpec{}, err
	}
	return dpv1alpha1.SchedulingSpec{
		Affinity:                  schedulingPolicy.Affinity,
		Tolerations:               schedulingPolicy.Tolerations,
		TopologySpreadConstraints: schedulingPolicy.TopologySpreadConstraints,
	}, nil
}

func (r *RestoreManager) GetRestoreObjectMeta(comp *component.SynthesizedComponent, stage dpv1alpha1.RestoreStage) metav1.ObjectMeta {
	name := fmt.Sprintf("%s-%s-%s-%s", r.Cluster.Name, comp.Name, r.Cluster.UID[:8], strings.ToLower(string(stage)))
	if r.startingIndex != 0 {
		name = fmt.Sprintf("%s-%d", name, r.startingIndex)
	}
	if len(r.restoreLabels) == 0 {
		r.restoreLabels = constant.GetKBWellKnownLabels(comp.ClusterDefName, r.Cluster.Name, comp.Name)
	}
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: r.Cluster.Namespace,
		Labels:    r.restoreLabels,
	}
}

func (r *RestoreManager) initFromAnnotation(synthesizedComponent *component.SynthesizedComponent) (*dpv1alpha1.Backup, error) {
	valueString := r.Cluster.Annotations[constant.RestoreFromBackupAnnotationKey]
	if len(valueString) == 0 {
		return nil, nil
	}
	backupMap := map[string]map[string]string{}
	err := json.Unmarshal([]byte(valueString), &backupMap)
	if err != nil {
		return nil, err
	}
	backupSource, ok := backupMap[synthesizedComponent.Name]
	if !ok {
		return nil, nil
	}
	if namespace := backupSource[constant.BackupNamespaceKeyForRestore]; namespace != "" {
		r.namespace = namespace
		// TODO: support restore backup to different namespace
		if namespace != r.Cluster.Namespace {
			return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeRestoreFailed,
				"unsupported restore backup to different namespace, backup namespace: %s, cluster namespace: %s", namespace, r.Cluster.Namespace)
		}
	}
	if volumeRestorePolicy := backupSource[constant.VolumeRestorePolicyKeyForRestore]; volumeRestorePolicy != "" {
		r.volumeRestorePolicy = dpv1alpha1.VolumeClaimRestorePolicy(volumeRestorePolicy)
	}
	r.restoreTime = backupSource[constant.RestoreTimeKeyForRestore]

	doReadyRestoreAfterClusterRunning := backupSource[constant.DoReadyRestoreAfterClusterRunning]
	if doReadyRestoreAfterClusterRunning == "true" {
		r.doReadyRestoreAfterClusterRunning = true
	}
	name := backupSource[constant.BackupNameKeyForRestore]
	if name == "" {
		return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeRestoreFailed,
			"failed to restore component %s, backup name is empty", synthesizedComponent.Name)
	}
	backup := &dpv1alpha1.Backup{}
	if err = r.Client.Get(r.Ctx, client.ObjectKey{Namespace: r.namespace, Name: name}, backup); err != nil {
		return nil, err
	}
	return backup, nil
}

// createRestoreAndWait create the restore CR and wait for completion.
func (r *RestoreManager) createRestoreAndWait(restore *dpv1alpha1.Restore, compObj *appsv1alpha1.Component) error {
	if restore == nil {
		return nil
	}
	if r.Scheme != nil {
		_ = controllerutil.SetControllerReference(compObj, restore, r.Scheme)
	}
	if err := r.Client.Get(r.Ctx, client.ObjectKeyFromObject(restore), restore); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err = r.Client.Create(r.Ctx, restore); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}

	switch restore.Status.Phase {
	case dpv1alpha1.RestorePhaseCompleted:
		return nil
	case dpv1alpha1.RestorePhaseFailed:
		return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRestoreFailed, `restore "%s" status is Failed, you can describe it and re-restore the cluster.`, restore.GetName())
	default:
		return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, `waiting for restore "%s" successfully`, restore.GetName())
	}
}

func (r *RestoreManager) cleanupClusterAnnotations(compName string) error {
	// TODO: Waiting for all component recovery jobs to be completed
	if r.Cluster.Annotations != nil {
		restoreInfo := r.Cluster.Annotations[constant.RestoreFromBackupAnnotationKey]
		if restoreInfo == "" {
			return nil
		}
		restoreInfoMap := map[string]any{}
		if err := json.Unmarshal([]byte(restoreInfo), &restoreInfoMap); err != nil {
			return err
		}
		delete(restoreInfoMap, compName)
		cluster := r.Cluster
		patch := client.MergeFrom(cluster.DeepCopy())
		if len(restoreInfoMap) == 0 {
			delete(cluster.Annotations, constant.RestoreFromBackupAnnotationKey)
		} else {
			restoreInfoBytes, err := json.Marshal(restoreInfoMap)
			if err != nil {
				return err
			}
			cluster.Annotations[constant.RestoreFromBackupAnnotationKey] = string(restoreInfoBytes)
		}
		return r.Client.Patch(r.Ctx, cluster, patch)
	}
	return nil
}
