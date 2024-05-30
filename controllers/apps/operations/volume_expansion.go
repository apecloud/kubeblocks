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

package operations

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type volumeExpansionOpsHandler struct {
}

type volumeExpansionHelper struct {
	compOps           ComponentOpsInteface
	fullComponentName string
	templateName      string
	vctName           string
	expectCount       int
}

var _ OpsHandler = volumeExpansionOpsHandler{}

var pvcNameRegex = regexp.MustCompile("(.*)-([0-9]+)$")

const (
	// VolumeExpansionTimeOut volume expansion timeout.
	VolumeExpansionTimeOut = 30 * time.Minute
)

func init() {
	// the volume expansion operation only supports online expansion now
	volumeExpansionBehaviour := OpsBehaviour{
		OpsHandler:  volumeExpansionOpsHandler{},
		QueueBySelf: true,
	}
	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.VolumeExpansionType, volumeExpansionBehaviour)
}

// ActionStartedCondition the started condition when handle the volume expansion request.
func (ve volumeExpansionOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewVolumeExpandingCondition(opsRes.OpsRequest), nil
}

// Action modifies Cluster.spec.components[*].VolumeClaimTemplates[*].spec.resources
func (ve volumeExpansionOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	applyVolumeExpansion := func(compSpec *appsv1alpha1.ClusterComponentSpec, obj ComponentOpsInteface) {
		setVolumeStorage := func(volumeExpansionVCTs []appsv1alpha1.OpsRequestVolumeClaimTemplate,
			targetVCTs []appsv1alpha1.ClusterComponentVolumeClaimTemplate) {
			for _, v := range volumeExpansionVCTs {
				for i, vct := range targetVCTs {
					if vct.Name != v.Name {
						continue
					}
					targetVCTs[i].Spec.Resources.Requests[corev1.ResourceStorage] = v.Storage
				}
			}
		}
		volumeExpansion := obj.(appsv1alpha1.VolumeExpansion)
		setVolumeStorage(volumeExpansion.VolumeClaimTemplates, compSpec.VolumeClaimTemplates)
		// update the vct of the instances.
		for _, v := range volumeExpansion.Instances {
			for i := range compSpec.Instances {
				if compSpec.Instances[i].Name == v.Name {
					setVolumeStorage(v.VolumeClaimTemplates, compSpec.Instances[i].VolumeClaimTemplates)
					break
				}
			}
		}
	}
	compOpsSet := newComponentOpsHelper(opsRes.OpsRequest.Spec.VolumeExpansionList)
	compOpsSet.updateClusterComponentsAndShardings(opsRes.Cluster, applyVolumeExpansion)
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for volume expansion opsRequest.
func (ve volumeExpansionOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		opsRequest             = opsRes.OpsRequest
		requeueAfter           time.Duration
		err                    error
		opsRequestPhase        = appsv1alpha1.OpsRunningPhase
		oldOpsRequestStatus    = opsRequest.Status.DeepCopy()
		expectProgressCount    int
		succeedProgressCount   int
		completedProgressCount int
	)
	getTemplateReplicas := func(templates []appsv1alpha1.InstanceTemplate) int32 {
		var replicaCount int32
		for _, v := range templates {
			replicaCount += intctrlutil.TemplateReplicas(v)
		}
		return replicaCount
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Status.Components == nil {
		ve.initComponentStatus(opsRequest)
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.VolumeExpansionList)
	storageMap := ve.getRequestStorageMap(opsRequest)
	var veHelpers []volumeExpansionHelper
	setVeHelpers := func(compSpec appsv1alpha1.ClusterComponentSpec, compOps ComponentOpsInteface, fullComponentName string) {
		volumeExpansion := compOps.(appsv1alpha1.VolumeExpansion)
		if len(volumeExpansion.VolumeClaimTemplates) > 0 {
			expectReplicas := compSpec.Replicas - getTemplateReplicas(compSpec.Instances)
			for _, vct := range volumeExpansion.VolumeClaimTemplates {
				veHelpers = append(veHelpers, volumeExpansionHelper{
					compOps:           compOps,
					fullComponentName: fullComponentName,
					expectCount:       int(expectReplicas),
					vctName:           vct.Name,
				})
			}
		}
		if len(volumeExpansion.Instances) > 0 {
			for _, ins := range compSpec.Instances {
				for _, vct := range ins.VolumeClaimTemplates {
					veHelpers = append(veHelpers, volumeExpansionHelper{
						compOps:           compOps,
						fullComponentName: fullComponentName,
						expectCount:       int(intctrlutil.TemplateReplicas(ins)),
						vctName:           vct.Name,
					})
				}
			}
		}
	}
	for _, compSpec := range opsRes.Cluster.Spec.ComponentSpecs {
		compOps, ok := compOpsHelper.componentOpsSet[compSpec.Name]
		if !ok {
			continue
		}
		setVeHelpers(compSpec, compOps, compSpec.Name)
	}
	for _, shardingSpec := range opsRes.Cluster.Spec.ShardingSpecs {
		compOps, ok := compOpsHelper.componentOpsSet[shardingSpec.Name]
		if !ok {
			continue
		}
		shardingComps, err := intctrlutil.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, shardingSpec.Name)
		if err != nil {
			return opsRequestPhase, 0, err
		}
		for _, v := range shardingComps {
			setVeHelpers(shardingSpec.Template, compOps, v.Labels[constant.KBAppComponentLabelKey])
		}
	}
	// reconcile the status.components. when the volume expansion is successful,
	// sync the volumeClaimTemplate status and component phase On the OpsRequest and Cluster.
	for _, veHelper := range veHelpers {
		opsCompStatus := opsRequest.Status.Components[veHelper.compOps.GetComponentName()]
		key := getComponentVCTKey(veHelper.compOps.GetComponentName(), veHelper.templateName, veHelper.vctName)
		requestStorage, ok := storageMap[key]
		if !ok {
			continue
		}
		succeedCount, completedCount, err := ve.handleVCTExpansionProgress(reqCtx, cli, opsRes,
			&opsCompStatus, requestStorage, veHelper)
		if err != nil {
			return "", requeueAfter, err
		}
		expectProgressCount += veHelper.expectCount
		succeedProgressCount += succeedCount
		completedProgressCount += completedCount
		opsRequest.Status.Components[veHelper.compOps.GetComponentName()] = opsCompStatus
	}
	if completedProgressCount != expectProgressCount {
		requeueAfter = time.Minute
	}
	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", completedProgressCount, expectProgressCount)
	// patch OpsRequest.status.components
	if !reflect.DeepEqual(*oldOpsRequestStatus, opsRequest.Status) {
		if err = cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, requeueAfter, err
		}
	}

	// check all PVCs of volumeClaimTemplate are successful
	if expectProgressCount == completedProgressCount {
		if expectProgressCount == succeedProgressCount {
			opsRequestPhase = appsv1alpha1.OpsSucceedPhase
		} else {
			opsRequestPhase = appsv1alpha1.OpsFailedPhase
		}
		return opsRequestPhase, requeueAfter, err
	}
	// check whether the volume expansion operation has timed out
	if time.Now().After(opsRequest.Status.StartTimestamp.Add(VolumeExpansionTimeOut)) {
		// if volume expansion timed out
		opsRequestPhase = appsv1alpha1.OpsFailedPhase
		err = errors.New(fmt.Sprintf("Timed out waiting for volume expansion to complete, the timeout value is %g minutes", VolumeExpansionTimeOut.Minutes()))
	}
	return opsRequestPhase, requeueAfter, err
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (ve volumeExpansionOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	compOpsHelper := newComponentOpsHelper(opsRequest.Spec.VolumeExpansionList)
	storageMap := ve.getRequestStorageMap(opsRequest)
	compOpsHelper.saveLastConfigurations(opsRes, func(compSpec appsv1alpha1.ClusterComponentSpec, comOps ComponentOpsInteface) appsv1alpha1.LastComponentConfiguration {
		getLastVCTs := func(vcts []appsv1alpha1.ClusterComponentVolumeClaimTemplate, templateName string) []appsv1alpha1.ClusterComponentVolumeClaimTemplate {
			lastVCTs := make([]appsv1alpha1.ClusterComponentVolumeClaimTemplate, 0)
			for _, vct := range vcts {
				key := getComponentVCTKey(comOps.GetComponentName(), comOps.GetComponentName(), templateName)
				if _, ok := storageMap[key]; !ok {
					continue
				}
				lastVCTs = append(lastVCTs, vct)
			}
			return lastVCTs
		}
		volumeExpansion := comOps.(appsv1alpha1.VolumeExpansion)
		// save the last vcts of the instances
		var instanceTemplates []appsv1alpha1.InstanceTemplate
		for _, v := range volumeExpansion.Instances {
			for _, ins := range compSpec.Instances {
				if ins.Name != v.Name {
					continue
				}
				instanceTemplates = append(instanceTemplates, appsv1alpha1.InstanceTemplate{
					VolumeClaimTemplates: getLastVCTs(ins.VolumeClaimTemplates, ins.Name),
				})
			}
		}
		// save the last vcts of the componnet
		lastVCTS := getLastVCTs(compSpec.VolumeClaimTemplates, "")
		var convertedLastVCTs []appsv1alpha1.OpsRequestVolumeClaimTemplate
		for _, v := range lastVCTS {
			convertedLastVCTs = append(convertedLastVCTs, appsv1alpha1.OpsRequestVolumeClaimTemplate{
				Name:    v.Name,
				Storage: v.Spec.Resources.Requests[corev1.ResourceStorage],
			})
		}
		return appsv1alpha1.LastComponentConfiguration{
			VolumeClaimTemplates: convertedLastVCTs,
			Instances:            instanceTemplates,
		}
	})
	return nil
}

// pvcIsResizing when pvc start resizing, it will set conditions type to Resizing/FileSystemResizePending
func (ve volumeExpansionOpsHandler) pvcIsResizing(pvc *corev1.PersistentVolumeClaim) bool {
	for _, condition := range pvc.Status.Conditions {
		if condition.Type == corev1.PersistentVolumeClaimResizing || condition.Type == corev1.PersistentVolumeClaimFileSystemResizePending {
			return true
		}
	}
	return false
}

func (ve volumeExpansionOpsHandler) getRequestStorageMap(opsRequest *appsv1alpha1.OpsRequest) map[string]resource.Quantity {
	storageMap := map[string]resource.Quantity{}
	setStorageMap := func(vct appsv1alpha1.OpsRequestVolumeClaimTemplate, compOps appsv1alpha1.ComponentOps, templateName string) {
		key := getComponentVCTKey(compOps.GetComponentName(), templateName, vct.Name)
		storageMap[key] = vct.Storage
	}
	for _, v := range opsRequest.Spec.VolumeExpansionList {
		for _, vct := range v.VolumeClaimTemplates {
			setStorageMap(vct, v.ComponentOps, "")
		}
		for _, ins := range v.Instances {
			for _, vct := range ins.VolumeClaimTemplates {
				setStorageMap(vct, v.ComponentOps, ins.Name)
			}
		}
	}
	return storageMap
}

// initComponentStatus inits status.components for the VolumeExpansion OpsRequest
func (ve volumeExpansionOpsHandler) initComponentStatus(opsRequest *appsv1alpha1.OpsRequest) {
	opsRequest.Status.Components = map[string]appsv1alpha1.OpsRequestComponentStatus{}
	for _, v := range opsRequest.Spec.VolumeExpansionList {
		opsRequest.Status.Components[v.ComponentName] = appsv1alpha1.OpsRequestComponentStatus{}
	}
}

// handleVCTExpansionProgress checks whether the pvc of the volume claim template is in (resizing, expansion succeeded, expansion completed).
func (ve volumeExpansionOpsHandler) handleVCTExpansionProgress(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	requestStorage resource.Quantity,
	veHelper volumeExpansionHelper) (int, int, error) {
	var (
		succeedCount   int
		completedCount int
		err            error
	)
	matchingLabels := client.MatchingLabels{
		constant.AppInstanceLabelKey:             opsRes.Cluster.Name,
		constant.VolumeClaimTemplateNameLabelKey: veHelper.vctName,
		constant.KBAppComponentLabelKey:          veHelper.fullComponentName,
	}
	if veHelper.templateName != "" {
		matchingLabels[constant.KBAppComponentInstanceTemplatelabelKey] = veHelper.templateName
	}
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err = cli.List(reqCtx.Ctx, pvcList, matchingLabels, client.InNamespace(opsRes.Cluster.Namespace)); err != nil {
		return 0, 0, err
	}
	var ordinal int
	for _, v := range pvcList.Items {
		// filter PVC(s) with ordinal no larger than comp.Replicas - 1, which left by scale-in
		ordinal, err = getPVCOrdinal(v.Name)
		if err != nil {
			return 0, 0, err
		}
		if ordinal > veHelper.expectCount-1 {
			continue
		}
		if v.Labels[constant.KBAppComponentInstanceTemplatelabelKey] != veHelper.templateName {
			continue
		}
		objectKey := getPVCProgressObjectKey(v.Name)
		progressDetail := findStatusProgressDetail(compStatus.ProgressDetails, objectKey)
		if progressDetail == nil {
			progressDetail = &appsv1alpha1.ProgressStatusDetail{ObjectKey: objectKey, Group: veHelper.vctName}
		}
		if progressDetail.Status == appsv1alpha1.FailedProgressStatus {
			completedCount += 1
			continue
		}
		currStorageSize := v.Status.Capacity.Storage()
		// should check if the spec.resources.requests.storage equals to the requested storage
		// and current storage size is greater than request storage size.
		// and pvc is bound if the pvc is re-created for recovery.
		if currStorageSize.Cmp(requestStorage) >= 0 &&
			v.Spec.Resources.Requests.Storage().Cmp(requestStorage) == 0 &&
			v.Status.Phase == corev1.ClaimBound {
			succeedCount += 1
			completedCount += 1
			message := fmt.Sprintf("Successfully expand volume: %s in component: %s", objectKey, veHelper.compOps.GetComponentName())
			progressDetail.SetStatusAndMessage(appsv1alpha1.SucceedProgressStatus, message)
			setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, *progressDetail)
			continue
		}
		if ve.pvcIsResizing(&v) {
			message := fmt.Sprintf("Start expanding volume: %s in component: %s", objectKey, veHelper.compOps.GetComponentName())
			progressDetail.SetStatusAndMessage(appsv1alpha1.ProcessingProgressStatus, message)
		} else {
			message := fmt.Sprintf("Waiting for an external controller to process the pvc: %s in component: %s", objectKey, veHelper.compOps.GetComponentName())
			progressDetail.SetStatusAndMessage(appsv1alpha1.PendingProgressStatus, message)
		}
		setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, *progressDetail)
	}
	return succeedCount, completedCount, nil
}

func getComponentVCTKey(compoName, insTemplateName, vctName string) string {
	var instanceNameKey string
	if insTemplateName != "" {
		instanceNameKey = "." + insTemplateName
	}
	return fmt.Sprintf("%s%s.%s", compoName, instanceNameKey, vctName)
}

func getPVCProgressObjectKey(pvcName string) string {
	return fmt.Sprintf("PVC/%s", pvcName)
}

func getPVCOrdinal(pvcName string) (int, error) {
	subMatches := pvcNameRegex.FindStringSubmatch(pvcName)
	if len(subMatches) < 3 {
		return 0, fmt.Errorf("wrong pvc name: %s", pvcName)
	}
	return strconv.Atoi(subMatches[2])
}
