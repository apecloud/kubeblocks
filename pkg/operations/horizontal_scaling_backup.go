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
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// reconcileBackupScaling prepares volumes before submitting the target configuration,
// then tracks the same instance changes as ordinary replica scaling.
func (hs horizontalScalingOpsHandler) reconcileBackupScaling(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes *progressResource,
	compStatus *opsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
	horizontalScaling := pgRes.compOps.(opsv1alpha1.HorizontalScaling)
	lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[horizontalScaling.ComponentName]
	// Preserve the restore step during cancellation as well as normal reconciliation.
	if err := hs.restoreDataFromBackup(reqCtx, cli, opsRes, pgRes, pgRes.clusterComponent.DeepCopy(),
		horizontalScaling, lastCompConfiguration, compStatus); err != nil {
		return 0, 0, err
	}
	// Continue checking instance progress even while Restore is pending, as in
	// the existing lifecycle. Only the target configuration write waits for Restore.
	if err := hs.setReplicaScalingParticipants(opsRes, pgRes); err != nil {
		return 0, 0, err
	}
	return handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus)
}

func (hs horizontalScalingOpsHandler) getBackupObj(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	fromBackup opsv1alpha1.FromBackup) (*dpv1alpha1.Backup, error) {
	backupNamespace := opsRes.Cluster.Namespace
	if fromBackup.Namespace != "" {
		backupNamespace = fromBackup.Namespace
	}
	backupObj := &dpv1alpha1.Backup{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Namespace: backupNamespace, Name: fromBackup.Name}, backupObj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf("backup %s not found", fromBackup.Name))
		}
		return nil, err
	}
	if backupObj.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf("backup %s phase is not completed", fromBackup.Name))
	}
	return backupObj, nil
}

func (hs horizontalScalingOpsHandler) createRestore(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	synthesizedComponent *intctrlcomp.SynthesizedComponent,
	restoreMGR *plan.RestoreManager,
	targetCompSpec *appsv1.ClusterComponentSpec,
	backupObj *dpv1alpha1.Backup,
	templateName string) error {
	getTemplate := func(templateName string) *appsv1.InstanceTemplate {
		if templateName == "" {
			return nil
		}
		for _, template := range targetCompSpec.Instances {
			if template.Name == templateName {
				return &template
			}
		}
		return nil
	}
	// create restore
	restore, err := restoreMGR.BuildPrepareDataRestoreForPod(synthesizedComponent, backupObj, getTemplate(templateName))
	if err != nil {
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeRestoreFailed) {
			return intctrlutil.NewFatalError(err.Error())
		}
		return err
	}
	if restore == nil {
		return intctrlutil.NewFatalError(fmt.Sprintf("scale-out from backup %s/%s is not supported because backup method %q has no target volumes matching component %q",
			backupObj.Namespace, backupObj.Name, backupObj.Status.BackupMethod.Name, synthesizedComponent.Name))
	}
	scheme, _ := opsv1alpha1.SchemeBuilder.Build()
	if err := intctrlutil.SetOwnership(opsRes.OpsRequest, restore, scheme, ""); err != nil {
		return err
	}
	if err := cli.Create(reqCtx.Ctx, restore); err != nil {
		return err
	}
	reqCtx.Recorder.Eventf(opsRes.OpsRequest, corev1.EventTypeNormal, "RestoreCreated", "Restore %s created", restore.Name)
	return nil
}

func (hs horizontalScalingOpsHandler) restoreDataFromBackup(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes *progressResource,
	targetCompSpec *appsv1.ClusterComponentSpec,
	horizontalScaling opsv1alpha1.HorizontalScaling,
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	compStatus *opsv1alpha1.OpsRequestComponentStatus) error {
	restoreCompletedMsg := "Restore Data Completed"
	// check if restore completed
	if compStatus.Message == restoreCompletedMsg {
		return nil
	}
	// get and check backup
	fromBackup := horizontalScaling.ScaleOut.FromBackup
	backupObj, err := hs.getBackupObj(reqCtx, cli, opsRes, *fromBackup)
	if err != nil {
		return err
	}
	replicas, instances, offlineInstances, err := hs.getExpectedCompValues(opsRes,
		lastCompConfiguration, horizontalScaling)
	if err != nil {
		return err
	}
	targetCompSpec.Replicas = replicas
	targetCompSpec.Instances = instances
	targetCompSpec.OfflineInstances = offlineInstances
	createdPodSet, deletedPodSet, err := hs.getReplicaScalingChanges(opsRes, lastCompConfiguration, horizontalScaling, pgRes.fullComponentName)
	if err != nil {
		return err
	}
	if opsRes.OpsRequest.Status.Phase == opsv1alpha1.OpsCancellingPhase {
		// Preserve which instances the existing cancellation path restores.
		createdPodSet = deletedPodSet
	}
	comp, compDef, err := intctrlcomp.GetCompNCompDefByName(reqCtx.Ctx, cli, opsRes.Cluster.Namespace, constant.GenerateClusterComponentName(opsRes.Cluster.Name, pgRes.fullComponentName))
	if err != nil {
		return err
	}
	synthesizedComponent, err := intctrlcomp.BuildSynthesizedComponent(reqCtx.Ctx, cli, compDef, comp)
	if err != nil {
		return err
	}
	allRestoreCompleted := true
	for podName, templateName := range createdPodSet {
		idx := strings.LastIndex(podName, "-")
		podIndex := podName[idx+1:]
		podIndexInt, _ := strconv.ParseInt(podIndex, 10, 32)
		restoreMGR := plan.NewRestoreManager(reqCtx.Ctx, cli, opsRes.Cluster, model.GetScheme(), map[string]string{
			constant.OpsRequestNameLabelKey: opsRes.OpsRequest.Name,
			constant.AppInstanceLabelKey:    opsRes.Cluster.Name,
			constant.KBAppComponentLabelKey: pgRes.compOps.GetComponentName(),
		}, 1, int32(podIndexInt))
		restoreMGR.RestoreTime = fromBackup.RestorePointInTime
		restoreMGR.SetRestoreEnv(fromBackup.RestoreEnv)
		restoreMGR.RestoreNamePrefix = string(opsRes.OpsRequest.UID[:8])
		restoreMGR.SourceTargetName = fromBackup.SourceTargetName
		// check restore status
		restoreMeta := restoreMGR.GetRestoreObjectMeta(synthesizedComponent, dpv1alpha1.PrepareData, templateName)
		restore := &dpv1alpha1.Restore{}
		if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: opsRes.Cluster.Namespace, Name: restoreMeta.Name}, restore); err != nil {
			if apierrors.IsNotFound(err) {
				allRestoreCompleted = false
				if err = hs.createRestore(reqCtx, cli, opsRes, synthesizedComponent, restoreMGR, targetCompSpec, backupObj, templateName); err != nil {
					return err
				}
				continue
			}
			return err
		}
		if restore.Status.Phase == dpv1alpha1.RestorePhaseFailed {
			return intctrlutil.NewFatalError(fmt.Sprintf("restore for horizontalScaling failed: you can describe the restore resource \"%s\"", restore.Name))
		}
		if restore.Status.Phase != dpv1alpha1.RestorePhaseCompleted {
			allRestoreCompleted = false
		}
	}
	compStatus.Message = "Restore Data In Progress"
	if allRestoreCompleted {
		if err := hs.scaleOutComponentAfterRestoreData(reqCtx, cli, opsRes, targetCompSpec); err != nil {
			return err
		}
		// set restore completed message
		compStatus.Message = restoreCompletedMsg
	}
	return nil
}

func (hs horizontalScalingOpsHandler) scaleOutComponentAfterRestoreData(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	targetCompSpec *appsv1.ClusterComponentSpec) error {
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		compSpec := &opsRes.Cluster.Spec.ComponentSpecs[i]
		if compSpec.Name == targetCompSpec.Name {
			compSpec.Replicas = targetCompSpec.Replicas
			compSpec.OfflineInstances = targetCompSpec.OfflineInstances
			compSpec.Instances = targetCompSpec.Instances
		}
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}
