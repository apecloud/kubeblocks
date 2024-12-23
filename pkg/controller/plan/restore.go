/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"math"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
)

// RestoreManager restores manager functions
// 1. support datafile/snapshot restore
// 2. support point in time recovery (PITR)
type RestoreManager struct {
	client.Client
	Ctx     context.Context
	Cluster *appsv1.Cluster
	Scheme  *k8sruntime.Scheme

	// private
	namespace                         string
	restoreTime                       string
	env                               []corev1.EnvVar
	parameters                        []dpv1alpha1.ParameterPair
	volumeRestorePolicy               dpv1alpha1.VolumeClaimRestorePolicy
	doReadyRestoreAfterClusterRunning bool
	startingIndex                     int32
	replicas                          int32
	restoreLabels                     map[string]string
}

func NewRestoreManager(ctx context.Context,
	cli client.Client,
	cluster *appsv1.Cluster,
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

func (r *RestoreManager) DoRestore(comp *component.SynthesizedComponent, compObj *appsv1.Component, postProvisionDone bool) error {
	backupObj, err := r.initFromAnnotation(comp, compObj)
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
	if compObj.Status.Phase != appsv1.RunningComponentPhase {
		return nil
	}
	// wait for the post-provision action to complete.
	if !postProvisionDone {
		return nil
	}
	if r.doReadyRestoreAfterClusterRunning && r.Cluster.Status.Phase != appsv1.RunningClusterPhase {
		return nil
	}
	if err = r.DoPostReady(comp, compObj, backupObj); err != nil {
		return err
	}
	// mark component restore done
	if compObj.Annotations != nil {
		compObj.Annotations[constant.RestoreDoneAnnotationKey] = "true"
	}
	// do clean up
	return r.cleanupRestoreAnnotations(comp.Name)
}

func (r *RestoreManager) DoPrepareData(comp *component.SynthesizedComponent,
	compObj *appsv1.Component,
	backupObj *dpv1alpha1.Backup) error {
	replicas := func(t appsv1.InstanceTemplate) int32 {
		if t.Replicas != nil {
			return *t.Replicas
		}
		return 1 // default replica
	}
	var restores []*dpv1alpha1.Restore
	var templateReplicas int32
	for _, v := range comp.Instances {
		r.replicas = replicas(v)
		templateReplicas += r.replicas
		restore, err := r.BuildPrepareDataRestore(comp, backupObj, v.Name)
		if err != nil {
			return err
		}
		if restore != nil {
			restores = append(restores, restore)
		}
	}
	compReplicas := comp.Replicas - templateReplicas
	if compReplicas > 0 {
		r.replicas = compReplicas
		restore, err := r.BuildPrepareDataRestore(comp, backupObj, "")
		if err != nil {
			return err
		}
		if restore != nil {
			restores = append(restores, restore)
		}
	}
	return r.createRestoreAndWait(compObj, restores...)
}

func (r *RestoreManager) BuildPrepareDataRestore(comp *component.SynthesizedComponent, backupObj *dpv1alpha1.Backup, templateName string) (*dpv1alpha1.Restore, error) {
	backupMethod := backupObj.Status.BackupMethod
	if backupMethod == nil {
		return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeRestoreFailed, `status.backupMethod of backup "%s" can not be empty`, backupObj.Name)
	}
	targetVolumes := backupMethod.TargetVolumes
	if targetVolumes == nil {
		return nil, nil
	}

	var templates []dpv1alpha1.RestoreVolumeClaim
	pvcLabels := constant.GetCompLabels(r.Cluster.Name, comp.Name)
	// TODO: create pvc by the volumeClaimTemplates of instance template if it is necessary.
	for _, v := range comp.VolumeClaimTemplates {
		if !dputils.ExistTargetVolume(targetVolumes, v.Name) {
			continue
		}
		name := fmt.Sprintf("%s-%s-%s", v.Name, r.Cluster.Name, comp.Name)
		if templateName != "" {
			name += "-" + templateName
		}
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: pvcLabels,
			},
		}
		// build pvc labels
		factory.BuildPersistentVolumeClaimLabels(comp, pvc, v.Name, templateName)
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
	sourceTargetName := comp.Annotations[constant.BackupSourceTargetAnnotationKey]
	sourceTarget := dputils.GetBackupStatusTarget(backupObj, sourceTargetName)
	restore := &dpv1alpha1.Restore{
		ObjectMeta: r.GetRestoreObjectMeta(comp, dpv1alpha1.PrepareData, templateName),
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:             backupObj.Name,
				Namespace:        r.namespace,
				SourceTargetName: sourceTargetName,
			},
			RestoreTime: r.restoreTime,
			Env:         r.env,
			Parameters:  r.parameters,
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
				RequiredPolicyForAllPodSelection: r.buildRequiredPolicy(sourceTarget),
				SchedulingSpec:                   r.buildSchedulingSpec(comp),
				VolumeClaimRestorePolicy:         r.volumeRestorePolicy,
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
	compObj *appsv1.Component,
	backupObj *dpv1alpha1.Backup) error {
	jobActionLabels := constant.GetCompLabels(r.Cluster.Name, comp.Name)
	if len(comp.Roles) > 0 {
		// HACK: assume the role with highest priority to be writable
		highestPriority := math.MinInt
		var role *appsv1.ReplicaRole
		for _, r := range comp.Roles {
			if r.UpdatePriority > highestPriority {
				highestPriority = r.UpdatePriority
				role = &r
			}
		}
		jobActionLabels[instanceset.RoleLabelKey] = role.Name
	}
	sourceTargetName := compObj.Annotations[constant.BackupSourceTargetAnnotationKey]
	restore := &dpv1alpha1.Restore{
		ObjectMeta: r.GetRestoreObjectMeta(comp, dpv1alpha1.PostReady, ""),
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:             backupObj.Name,
				Namespace:        r.namespace,
				SourceTargetName: sourceTargetName,
			},
			RestoreTime: r.restoreTime,
			Env:         r.env,
			Parameters:  r.parameters,
			ReadyConfig: &dpv1alpha1.ReadyConfig{
				ExecAction: &dpv1alpha1.ExecAction{
					Target: dpv1alpha1.ExecActionTarget{
						PodSelector: metav1.LabelSelector{
							MatchLabels: constant.GetCompLabels(r.Cluster.Name, comp.Name),
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
			},
		},
	}
	sourceTarget := dputils.GetBackupStatusTarget(backupObj, sourceTargetName)
	if sourceTarget != nil {
		// TODO: input the pod selection strategy by user.
		restore.Spec.ReadyConfig.JobAction.Target.PodSelector.Strategy = sourceTarget.PodSelector.Strategy
	}
	restore.Spec.ReadyConfig.JobAction.RequiredPolicyForAllPodSelection = r.buildRequiredPolicy(sourceTarget)
	backupMethod := backupObj.Status.BackupMethod
	if backupMethod.TargetVolumes != nil {
		restore.Spec.ReadyConfig.JobAction.Target.VolumeMounts = backupMethod.TargetVolumes.VolumeMounts
	}
	return r.createRestoreAndWait(compObj, restore)
}

func (r *RestoreManager) buildRequiredPolicy(sourceTarget *dpv1alpha1.BackupStatusTarget) *dpv1alpha1.RequiredPolicyForAllPodSelection {
	var requiredPolicy *dpv1alpha1.RequiredPolicyForAllPodSelection
	if sourceTarget != nil && sourceTarget.PodSelector.Strategy == dpv1alpha1.PodSelectionStrategyAll {
		// TODO: input the RequiredPolicyForAllPodSelection by user.
		requiredPolicy = &dpv1alpha1.RequiredPolicyForAllPodSelection{
			DataRestorePolicy: dpv1alpha1.OneToOneRestorePolicy,
		}
	}
	return requiredPolicy
}

func (r *RestoreManager) buildSchedulingSpec(comp *component.SynthesizedComponent) dpv1alpha1.SchedulingSpec {
	if comp.PodSpec == nil {
		return dpv1alpha1.SchedulingSpec{}
	}
	return dpv1alpha1.SchedulingSpec{
		Affinity:                  comp.PodSpec.Affinity,
		Tolerations:               comp.PodSpec.Tolerations,
		TopologySpreadConstraints: comp.PodSpec.TopologySpreadConstraints,
	}
}

func (r *RestoreManager) GetRestoreObjectMeta(comp *component.SynthesizedComponent, stage dpv1alpha1.RestoreStage, templateName string) metav1.ObjectMeta {
	name := fmt.Sprintf("%s-%s-%s-%s", r.Cluster.Name, comp.Name, r.Cluster.UID[:8], strings.ToLower(string(stage)))
	if templateName != "" {
		name = fmt.Sprintf("%s-%s", name, templateName)
	}
	if r.startingIndex != 0 {
		name = fmt.Sprintf("%s-%d", name, r.startingIndex)
	}
	if len(r.restoreLabels) == 0 {
		r.restoreLabels = constant.GetCompLabels(r.Cluster.Name, comp.Name)
	}
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: r.Cluster.Namespace,
		Labels:    r.restoreLabels,
	}
}

func (r *RestoreManager) initFromAnnotation(synthesizedComponent *component.SynthesizedComponent, compObj *appsv1.Component) (*dpv1alpha1.Backup, error) {
	valueString := r.Cluster.Annotations[constant.RestoreFromBackupAnnotationKey]
	if len(valueString) == 0 {
		return nil, nil
	}
	backupMap := map[string]map[string]string{}
	err := json.Unmarshal([]byte(valueString), &backupMap)
	if err != nil {
		return nil, err
	}
	compName := compObj.Labels[constant.KBAppShardingNameLabelKey]
	if len(compName) == 0 {
		compName = synthesizedComponent.Name
	}
	backupSource, ok := backupMap[compName]
	if !ok {
		return nil, nil
	}
	if namespace := backupSource[constant.BackupNamespaceKeyForRestore]; namespace != "" {
		r.namespace = namespace
	}
	if volumeRestorePolicy := backupSource[constant.VolumeRestorePolicyKeyForRestore]; volumeRestorePolicy != "" {
		r.volumeRestorePolicy = dpv1alpha1.VolumeClaimRestorePolicy(volumeRestorePolicy)
	}
	r.restoreTime = backupSource[constant.RestoreTimeKeyForRestore]
	doReadyRestoreAfterClusterRunning := backupSource[constant.DoReadyRestoreAfterClusterRunning]
	if doReadyRestoreAfterClusterRunning == "true" {
		r.doReadyRestoreAfterClusterRunning = true
	}
	if env := backupSource[constant.EnvForRestore]; env != "" {
		if err = json.Unmarshal([]byte(env), &r.env); err != nil {
			return nil, err
		}
	}
	if parameters := backupSource[constant.ParametersForRestore]; len(parameters) > 0 {
		if err = json.Unmarshal([]byte(parameters), &r.parameters); err != nil {
			return nil, err
		}
	}
	return GetBackupFromClusterAnnotation(r.Ctx, r.Client, backupSource, synthesizedComponent.Name, r.Cluster.Namespace)
}

// createRestoreAndWait create the restore CR and wait for completion.
func (r *RestoreManager) createRestoreAndWait(compObj *appsv1.Component, restores ...*dpv1alpha1.Restore) error {
	if len(restores) == 0 {
		return nil
	}
	for i := range restores {
		restore := restores[i]
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
			continue
		case dpv1alpha1.RestorePhaseFailed:
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRestoreFailed, `restore "%s" status is Failed, you can describe it and re-restore the cluster.`, restore.GetName())
		default:
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, `waiting for restore "%s" successfully`, restore.GetName())
		}
	}
	return nil
}

func (r *RestoreManager) cleanupRestoreAnnotations(compName string) error {
	if r.Cluster.Annotations != nil {
		patch := client.MergeFrom(r.Cluster.DeepCopy())
		needCleanup, err := CleanupClusterRestoreAnnotation(r.Cluster, compName)
		if err != nil {
			return err
		}
		if needCleanup {
			return r.Client.Patch(r.Ctx, r.Cluster, patch)
		}
	}
	return nil
}

func CleanupClusterRestoreAnnotation(cluster *appsv1.Cluster, compName string) (bool, error) {
	restoreInfo := cluster.Annotations[constant.RestoreFromBackupAnnotationKey]
	if restoreInfo == "" {
		return false, nil
	}
	restoreInfoMap := map[string]any{}
	if err := json.Unmarshal([]byte(restoreInfo), &restoreInfoMap); err != nil {
		return false, err
	}
	delete(restoreInfoMap, compName)
	if len(restoreInfoMap) == 0 {
		delete(cluster.Annotations, constant.RestoreFromBackupAnnotationKey)
	} else {
		restoreInfoBytes, err := json.Marshal(restoreInfoMap)
		if err != nil {
			return false, err
		}
		cluster.Annotations[constant.RestoreFromBackupAnnotationKey] = string(restoreInfoBytes)
	}
	return true, nil
}

func GetBackupFromClusterAnnotation(
	ctx context.Context,
	cli client.Reader,
	backupSource map[string]string,
	compName string,
	clusterNameSpace string) (*dpv1alpha1.Backup, error) {
	backupName := backupSource[constant.BackupNameKeyForRestore]
	if backupName == "" {
		return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeRestoreFailed,
			"failed to restore component %s, backup name is empty", compName)
	}
	namespace := backupSource[constant.BackupNamespaceKeyForRestore]
	if namespace == "" {
		namespace = clusterNameSpace
	}
	backup := &dpv1alpha1.Backup{}
	if err := cli.Get(ctx, client.ObjectKey{Name: backupName, Namespace: namespace}, backup); err != nil {
		return nil, err
	}
	return backup, nil
}
