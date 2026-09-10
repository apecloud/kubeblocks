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
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
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
	if err := hs.restoreDataFromBackup(reqCtx, cli, opsRes, pgRes, pgRes.clusterComponent.DeepCopy(),
		horizontalScaling, lastCompConfiguration, compStatus); err != nil {
		return 0, 0, err
	}
	// Restore may still be pending; actual instance progress remains independent.
	created, deleted, err := hs.getReplicaScalingChangesForRestore(opsRes, lastCompConfiguration,
		horizontalScaling, pgRes.fullComponentName)
	if err != nil {
		return 0, 0, err
	}
	if opsRes.OpsRequest.Status.Phase == opsv1alpha1.OpsCancellingPhase {
		created, deleted = deleted, created
	}
	pgRes.createdPodSet, pgRes.deletedPodSet = created, deleted
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
	replicas, instances, offlineInstances, err := hs.getExpectedCompValuesForRestore(opsRes,
		lastCompConfiguration, horizontalScaling)
	if err != nil {
		return err
	}
	targetCompSpec.Replicas = replicas
	targetCompSpec.Instances = instances
	targetCompSpec.OfflineInstances = offlineInstances
	createdPodSet, deletedPodSet, err := hs.getReplicaScalingChangesForRestore(opsRes, lastCompConfiguration, horizontalScaling, pgRes.fullComponentName)
	if err != nil {
		return err
	}
	if opsRes.OpsRequest.Status.Phase == opsv1alpha1.OpsCancellingPhase {
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

// prepareBackupScaling validates the request without publishing its target
// configuration. The restore path submits it once the volumes are prepared.
func (hs horizontalScalingOpsHandler) prepareBackupScaling(opsRes *OpsResource,
	horizontalScaling opsv1alpha1.HorizontalScaling) error {
	last := opsRes.OpsRequest.Status.LastConfiguration.Components[horizontalScaling.ComponentName]
	if err := hs.validateHorizontalScalingForRestore(opsRes, last, horizontalScaling); err != nil {
		return err
	}
	replicas, instances, _, err := hs.getExpectedCompValuesForRestore(opsRes, last, horizontalScaling)
	if err != nil {
		return err
	}
	return validateScalingTarget(horizontalScaling.ComponentName, replicas, instances)
}

// getReplicaScalingChangesForRestore predicts forward participants only for the backup path.
func (hs horizontalScalingOpsHandler) getReplicaScalingChangesForRestore(opsRes *OpsResource,
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	horizontalScaling opsv1alpha1.HorizontalScaling,
	fullCompName string) (map[string]string, map[string]string, error) {
	clusterName := opsRes.Cluster.Name
	runtime, err := opsRes.GetRuntime(horizontalScaling.ComponentName)
	if err != nil {
		return nil, nil, err
	}
	lastPodSet, err := runtime.GenerateInstanceNameSet(clusterName, fullCompName,
		*lastCompConfiguration.Replicas, lastCompConfiguration.Instances, lastCompConfiguration.OfflineInstances)
	if err != nil {
		return nil, nil, err
	}
	expectReplicas, expectInstanceTpls, expectOfflineInstances, err := hs.getExpectedCompValuesForRestore(opsRes, lastCompConfiguration, horizontalScaling)
	if err != nil {
		return nil, nil, err
	}
	currPodSet, err := runtime.GenerateInstanceNameSet(clusterName, fullCompName,
		expectReplicas, expectInstanceTpls, expectOfflineInstances)
	if err != nil {
		return nil, nil, err
	}
	createPodSet := map[string]string{}
	deletePodSet := map[string]string{}
	for k := range currPodSet {
		if _, ok := lastPodSet[k]; !ok {
			createPodSet[k] = appsv1.GetInstanceTemplateName(clusterName, fullCompName, k)
		}
	}
	for k := range lastPodSet {
		if _, ok := currPodSet[k]; !ok {
			deletePodSet[k] = appsv1.GetInstanceTemplateName(clusterName, fullCompName, k)
		}
	}
	return createPodSet, deletePodSet, nil
}

// getExpectedCompValuesForRestore retains deterministic planning for backup restoration.
func (hs horizontalScalingOpsHandler) getExpectedCompValuesForRestore(
	opsRes *OpsResource,
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	horizontalScaling opsv1alpha1.HorizontalScaling) (int32, []appsv1.InstanceTemplate, []string, error) {
	compReplicas := *lastCompConfiguration.Replicas
	compInstanceTpls := slices.Clone(lastCompConfiguration.Instances)
	compOfflineInstances := lastCompConfiguration.OfflineInstances
	filteredHorizontal := horizontalScaling.DeepCopy()
	// Obtain the existing names before filtering, even for requests without
	// explicit online/offline lists, preserving the original lookup/error order.
	runtime, err := opsRes.GetRuntime(horizontalScaling.ComponentName)
	if err != nil {
		return 0, nil, nil, err
	}
	podSet, err := runtime.GenerateInstanceNameSet(opsRes.Cluster.Name, horizontalScaling.ComponentName,
		compReplicas, compInstanceTpls, compOfflineInstances)
	if err != nil {
		return 0, nil, nil, err
	}
	filterHorizontalScalingSpec(podSet, compOfflineInstances, filteredHorizontal)
	expectOfflineInstances := hs.getCompExpectedOfflineInstances(compOfflineInstances, *filteredHorizontal)
	err = hs.autoSyncReplicaChangesForRestore(opsRes, *filteredHorizontal, compReplicas, compInstanceTpls, expectOfflineInstances)
	if err != nil {
		return 0, nil, nil, err
	}
	return hs.getCompExpectReplicas(*filteredHorizontal, compReplicas),
		hs.getCompExpectedInstances(compInstanceTpls, *filteredHorizontal),
		expectOfflineInstances, nil
}

func (hs horizontalScalingOpsHandler) autoSyncReplicaChangesForRestore(
	opsRes *OpsResource,
	horizontalScaling opsv1alpha1.HorizontalScaling,
	compReplicas int32,
	compInstanceTpls []appsv1.InstanceTemplate,
	compExpectOfflineInstances []string) error {
	// auto sync the replicaChanges.
	scaleIn := horizontalScaling.ScaleIn
	if scaleIn != nil {
		offlineInsCountMap := opsRes.OpsRequest.CountOfflineOrOnlineInstances(opsRes.Cluster.Name, horizontalScaling.ComponentName, scaleIn.OnlineInstancesToOffline)
		scaleIn.Instances, scaleIn.ReplicaChanges = syncReplicaChangesFromCounts(offlineInsCountMap, scaleIn.ReplicaChanger, nil)
	}
	scaleOut := horizontalScaling.ScaleOut
	if scaleOut != nil {
		onlineInsCountMap, err := hs.getPlannedOnlineInstanceCounts(opsRes, horizontalScaling, compReplicas, compInstanceTpls, compExpectOfflineInstances)
		if err != nil {
			return err
		}
		scaleOut.Instances, scaleOut.ReplicaChanges = syncReplicaChangesFromCounts(onlineInsCountMap, scaleOut.ReplicaChanger, scaleOut.NewInstances)
	}
	return nil
}

func (hs horizontalScalingOpsHandler) getPlannedOnlineInstanceCounts(
	opsRes *OpsResource,
	horizontalScaling opsv1alpha1.HorizontalScaling,
	compReplicas int32,
	compInstanceTpls []appsv1.InstanceTemplate,
	compExpectOfflineInstances []string) (map[string]int32, error) {
	if horizontalScaling.ScaleOut == nil || horizontalScaling.ScaleOut.ReplicaChanges != nil || len(horizontalScaling.ScaleOut.OfflineInstancesToOnline) == 0 {
		return nil, nil
	}
	compInstanceTplsClone := slices.Clone(compInstanceTpls)
	// 1. Automatically synchronize replicaChanges based on the specified OfflineInstancesToOnline
	offlineInsMap := groupInstancesToOnline(opsRes.Cluster.Name, horizontalScaling.ComponentName, horizontalScaling.ScaleOut)
	for _, insNames := range offlineInsMap {
		compReplicas += int32(len(insNames))
	}
	for i := range compInstanceTplsClone {
		tplName := compInstanceTplsClone[i].Name
		if insNames, ok := offlineInsMap[tplName]; ok {
			compInstanceTplsClone[i].Replicas = pointer.Int32(compInstanceTplsClone[i].GetReplicas() + int32(len(insNames)))
		}
	}
	// 2. obtain the updated Pod set after synchronization replicas.
	runtime, err := opsRes.GetRuntime(horizontalScaling.ComponentName)
	if err != nil {
		return nil, err
	}
	podSet, err := runtime.GenerateInstanceNameSet(opsRes.Cluster.Name, horizontalScaling.ComponentName,
		compReplicas, compInstanceTplsClone, compExpectOfflineInstances)
	if err != nil {
		return nil, err
	}
	// 3. count the number of online instances for each instance template.
	return countPlannedOnlineInstances(offlineInsMap, podSet), nil
}

// groupInstancesToOnline uses deterministic names only for backup restoration.
func groupInstancesToOnline(clusterName, componentName string, scaleOut *opsv1alpha1.ScaleOut) map[string][]string {
	slices.Sort(scaleOut.OfflineInstancesToOnline)
	offlineInsMap := map[string][]string{}
	instanceTplChangesMap := map[string]int32{}
	for _, tplChange := range scaleOut.ReplicaChanger.Instances {
		instanceTplChangesMap[tplChange.Name] = tplChange.ReplicaChanges
	}
	for _, insName := range scaleOut.OfflineInstancesToOnline {
		insTplName := appsv1.GetInstanceTemplateName(clusterName, componentName, insName)
		if _, ok := instanceTplChangesMap[insTplName]; ok {
			// Explicit template replica changes take precedence over inferred counts.
			continue
		}
		offlineInsMap[insTplName] = append(offlineInsMap[insTplName], insName)
	}
	return offlineInsMap
}

func countPlannedOnlineInstances(offlineInsMap map[string][]string, podSet map[string]string) map[string]int32 {
	onlineInsCountMap := map[string]int32{}
	for insTplName, insNames := range offlineInsMap {
		for _, insName := range insNames {
			// Count the leading requested names present in the plan, stopping at
			// the first missing name in each template. This does not check readiness.
			if _, ok := podSet[insName]; !ok {
				break
			}
			onlineInsCountMap[insTplName]++
		}
	}
	return onlineInsCountMap
}

func (hs horizontalScalingOpsHandler) validateHorizontalScalingForRestore(
	opsRes *OpsResource,
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	horizontalScaling opsv1alpha1.HorizontalScaling,
) error {
	if opsRes.OpsRequest.Annotations[constant.IgnoreHscaleValidateAnnoKey] == "true" {
		return nil
	}
	if horizontalScaling.ScaleIn != nil {
		if err := hs.validateOnlineInstancesToOfflineForRestore(lastCompConfiguration,
			horizontalScaling.ScaleIn.OnlineInstancesToOffline, opsRes, horizontalScaling.ComponentName); err != nil {
			return err
		}
	}
	if horizontalScaling.ScaleOut != nil {
		if err := hs.validateOfflineInstancesToOnline(lastCompConfiguration,
			horizontalScaling.ScaleOut.OfflineInstancesToOnline, horizontalScaling.ComponentName); err != nil {
			return err
		}
	}
	return nil
}

func (hs horizontalScalingOpsHandler) validateOnlineInstancesToOfflineForRestore(
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	onlineInstancesToOffline []string,
	opsRes *OpsResource,
	componentName string) error {
	if len(onlineInstancesToOffline) == 0 {
		return nil
	}
	toOfflineSet := sets.New(onlineInstancesToOffline...)
	if len(toOfflineSet) < len(onlineInstancesToOffline) {
		return intctrlutil.NewFatalError("instances specified in onlineInstancesToOffline has duplicates")
	}
	runtime, err := opsRes.GetRuntime(componentName)
	if err != nil {
		return err
	}
	currPodSet, err := runtime.GenerateInstanceNameSet(opsRes.Cluster.Name, componentName,
		*lastCompConfiguration.Replicas, lastCompConfiguration.Instances, lastCompConfiguration.OfflineInstances)
	if err != nil {
		return err
	}
	for _, onlineIns := range onlineInstancesToOffline {
		if _, ok := currPodSet[onlineIns]; !ok {
			return intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" specified in onlineInstancesToOffline is not online`, onlineIns))
		}
	}
	return nil
}
