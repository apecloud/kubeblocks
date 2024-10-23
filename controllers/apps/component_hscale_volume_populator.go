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

package apps

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
)

type dataClone interface {
	// Succeed check if data clone succeeded
	Succeed() (bool, error)
	// CloneData do clone data, return objects that need to be created
	CloneData(dataClone) ([]client.Object, []client.Object, error)
	// GetTmpResources get all the temporary resources created during data clone, return objects that need to be deleted
	GetTmpResources() ([]client.Object, error)

	CheckBackupStatus() (backupStatus, error)
	CheckRestoreStatus(templateName string, startingIndex int32) (dpv1alpha1.RestorePhase, error)

	backup() ([]client.Object, error)

	restore(templateName string, startingIndex int32) ([]client.Object, error)
}

type backupStatus string

const (
	backupStatusNotCreated backupStatus = "NotCreated"
	backupStatusProcessing backupStatus = "Processing"
	backupStatusReadyToUse backupStatus = "ReadyToUse"
	backupStatusFailed     backupStatus = "Failed"
)

func newDataClone(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1.Cluster,
	component *component.SynthesizedComponent,
	itsObj, itsProto *workloads.InstanceSet,
	backupKey types.NamespacedName) (dataClone, error) {
	if component == nil {
		return nil, nil
	}
	desiredPodNames, err := generatePodNames(component)
	if err != nil {
		return nil, err
	}
	currentPodNames, err := generatePodNamesByITS(itsObj)
	if err != nil {
		return nil, err
	}
	if component.HorizontalScaleBackupPolicyTemplate == nil {
		return &dummyDataClone{
			baseDataClone{
				reqCtx:            reqCtx,
				cli:               cli,
				cluster:           cluster,
				component:         component,
				itsObj:            itsObj,
				itsProto:          itsProto,
				backupKey:         backupKey,
				desiredPodNames:   desiredPodNames,
				currentPodNameSet: sets.New(currentPodNames...),
			},
		}, nil
	}
	return &backupDataClone{
		baseDataClone{
			reqCtx:            reqCtx,
			cli:               cli,
			cluster:           cluster,
			component:         component,
			itsObj:            itsObj,
			itsProto:          itsProto,
			backupKey:         backupKey,
			desiredPodNames:   desiredPodNames,
			currentPodNameSet: sets.New(currentPodNames...),
		},
	}, nil
}

type baseDataClone struct {
	reqCtx            intctrlutil.RequestCtx
	cli               client.Client
	cluster           *appsv1.Cluster
	component         *component.SynthesizedComponent
	itsObj            *workloads.InstanceSet
	itsProto          *workloads.InstanceSet
	backupKey         types.NamespacedName
	desiredPodNames   []string
	currentPodNameSet sets.Set[string]
}

func (d *baseDataClone) CloneData(realDataClone dataClone) ([]client.Object, []client.Object, error) {
	objs := make([]client.Object, 0)

	// check backup ready
	status, err := realDataClone.CheckBackupStatus()
	if err != nil {
		return nil, nil, err
	}
	switch status {
	case backupStatusNotCreated:
		// create backup
		backupObjs, err := realDataClone.backup()
		if err != nil {
			return nil, nil, err
		}
		objs = append(objs, backupObjs...)
		return objs, nil, nil
	case backupStatusProcessing, backupStatusFailed:
		// requeue to waiting for backup ready
		return objs, nil, nil
	case backupStatusReadyToUse:
		break
	default:
		panic(fmt.Sprintf("unexpected backup status: %s, clustre: %s, component: %s",
			status, d.cluster.Name, d.component.Name))
	}
	for _, podName := range d.desiredPodNames {
		if _, ok := d.currentPodNameSet[podName]; ok {
			continue
		}
		// backup's ready, then start to check restore
		templateName, index, err := component.GetTemplateNameAndOrdinal(d.itsObj.Name, podName)
		if err != nil {
			return nil, nil, err
		}
		restoreStatus, err := realDataClone.CheckRestoreStatus(templateName, index)
		if err != nil {
			return nil, nil, err
		}
		switch restoreStatus {
		case "":
			restoreObjs, err := realDataClone.restore(templateName, index)
			if err != nil {
				return nil, nil, err
			}
			objs = append(objs, restoreObjs...)
		case dpv1alpha1.RestorePhaseCompleted:
			continue
		}
	}
	// create PVCs that do not need to restore
	// TODO: create pvc by the volumeClaimTemplates of instance template if it is necessary.
	pvcObjs, err := d.createPVCs(d.excludeBackupVCTs())
	if err != nil {
		return nil, nil, err
	}
	return objs, pvcObjs, nil
}

func (d *baseDataClone) isPVCExists(pvcKey types.NamespacedName) (bool, error) {
	pvc := corev1.PersistentVolumeClaim{}
	if err := d.cli.Get(d.reqCtx.Ctx, pvcKey, &pvc, inDataContext4C()); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	return true, nil
}

func (d *baseDataClone) checkAllPVCsExist() (bool, error) {
	desiredPodNames, err := generatePodNames(d.component)
	if err != nil {
		return true, err
	}
	for _, podName := range desiredPodNames {
		for _, vct := range d.component.VolumeClaimTemplates {
			pvcKey := types.NamespacedName{
				Namespace: d.itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s", vct.Name, podName),
			}
			// check pvc existence
			pvcExists, err := d.isPVCExists(pvcKey)
			if err != nil {
				return true, err
			}
			if !pvcExists {
				return false, nil
			}
		}
	}
	return true, nil
}

func (d *baseDataClone) allVCTs() []*corev1.PersistentVolumeClaimTemplate {
	vcts := make([]*corev1.PersistentVolumeClaimTemplate, 0)
	for i := range d.component.VolumeClaimTemplates {
		vcts = append(vcts, &d.component.VolumeClaimTemplates[i])
	}
	return vcts
}

func (d *baseDataClone) backupVCT() *corev1.PersistentVolumeClaimTemplate {
	return backupVCT(d.component)
}

func (d *baseDataClone) excludeBackupVCTs() []*corev1.PersistentVolumeClaimTemplate {
	vcts := make([]*corev1.PersistentVolumeClaimTemplate, 0)
	backupVCT := d.backupVCT()
	for i := range d.component.VolumeClaimTemplates {
		vct := &d.component.VolumeClaimTemplates[i]
		if vct.Name != backupVCT.Name {
			vcts = append(vcts, vct)
		}
	}
	return vcts
}

func (d *baseDataClone) createPVCs(vcts []*corev1.PersistentVolumeClaimTemplate) ([]client.Object, error) {
	objs := make([]client.Object, 0)
	for _, podName := range d.desiredPodNames {
		if _, ok := d.currentPodNameSet[podName]; ok {
			continue
		}
		templateName, _, err := component.GetTemplateNameAndOrdinal(d.itsObj.Name, podName)
		if err != nil {
			return nil, err
		}
		for _, vct := range vcts {
			pvcKey := types.NamespacedName{
				Namespace: d.itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s", vct.Name, podName),
			}
			if exist, err := d.isPVCExists(pvcKey); err != nil {
				return nil, err
			} else if exist {
				continue
			}
			pvc := factory.BuildPVC(d.cluster, d.component, vct, pvcKey, templateName, "")
			objs = append(objs, pvc)
		}
	}
	return objs, nil
}

func (d *baseDataClone) getBRLabels() map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey:    d.cluster.Name,
		constant.KBAppComponentLabelKey: d.component.Name,
		constant.AppManagedByLabelKey:   constant.AppName, // the resources are managed by which controller
	}
}

type dummyDataClone struct {
	baseDataClone
}

var _ dataClone = &dummyDataClone{}

func (d *dummyDataClone) Succeed() (bool, error) {
	return d.checkAllPVCsExist()
}

func (d *dummyDataClone) CloneData(dataClone) ([]client.Object, []client.Object, error) {
	pvcObjs, err := d.createPVCs(d.allVCTs())
	return nil, pvcObjs, err
}

func (d *dummyDataClone) GetTmpResources() ([]client.Object, error) {
	return nil, nil
}

func (d *dummyDataClone) CheckBackupStatus() (backupStatus, error) {
	return backupStatusReadyToUse, nil
}

func (d *dummyDataClone) backup() ([]client.Object, error) {
	panic("runtime error: dummyDataClone.backup called")
}

func (d *dummyDataClone) CheckRestoreStatus(templateName string, startingIndex int32) (dpv1alpha1.RestorePhase, error) {
	return dpv1alpha1.RestorePhaseCompleted, nil
}

func (d *dummyDataClone) restore(templateName string, startingIndex int32) ([]client.Object, error) {
	panic("runtime error: dummyDataClone.restore called")
}

type backupDataClone struct {
	baseDataClone
}

var _ dataClone = &backupDataClone{}

func (d *backupDataClone) Succeed() (bool, error) {
	if len(d.component.VolumeClaimTemplates) == 0 {
		d.reqCtx.Recorder.Eventf(d.cluster,
			corev1.EventTypeNormal,
			"HorizontalScale",
			"no VolumeClaimTemplates, no need to do data clone.")
		return true, nil
	}
	allPVCsExist, err := d.checkAllPVCsExist()
	if err != nil || !allPVCsExist {
		return allPVCsExist, err
	}
	for _, podName := range d.desiredPodNames {
		if _, ok := d.currentPodNameSet[podName]; ok {
			continue
		}
		templateName, index, err := component.GetTemplateNameAndOrdinal(d.itsObj.Name, podName)
		if err != nil {
			return false, err
		}
		restoreStatus, err := d.CheckRestoreStatus(templateName, index)
		if err != nil {
			return false, err
		}
		if restoreStatus != dpv1alpha1.RestorePhaseCompleted {
			return false, nil
		}
	}
	return true, nil
}

func (d *backupDataClone) GetTmpResources() ([]client.Object, error) {
	objs := make([]client.Object, 0)
	// delete backup
	brLabels := d.getBRLabels()
	backupList := dpv1alpha1.BackupList{}
	if err := d.cli.List(d.reqCtx.Ctx, &backupList, client.InNamespace(d.cluster.Namespace), client.MatchingLabels(brLabels)); err != nil {
		return nil, err
	}
	for i := range backupList.Items {
		objs = append(objs, &backupList.Items[i])
	}
	restoreList := dpv1alpha1.RestoreList{}
	if err := d.cli.List(d.reqCtx.Ctx, &restoreList, client.InNamespace(d.cluster.Namespace), client.MatchingLabels(brLabels)); err != nil {
		return nil, err
	}
	for i := range restoreList.Items {
		objs = append(objs, &restoreList.Items[i])
	}
	return objs, nil
}

func (d *backupDataClone) backup() ([]client.Object, error) {
	backupPolicyTplName := *d.component.HorizontalScaleBackupPolicyTemplate
	backupPolicy, err := getBackupPolicyFromTemplate(d.reqCtx, d.cli, d.cluster, d.component.CompDefName, backupPolicyTplName)
	if err != nil {
		return nil, err
	}
	if backupPolicy == nil {
		return nil, intctrlutil.NewNotFound("not found any backup policy created by %s", backupPolicyTplName)
	}
	volumeSnapshotEnabled, err := isVolumeSnapshotEnabled(d.reqCtx.Ctx, d.cli, d.itsObj, backupVCT(d.component))
	if err != nil {
		return nil, err
	}
	backupMethods := getBackupMethods(backupPolicy, volumeSnapshotEnabled)
	if len(backupMethods) == 0 {
		return nil, fmt.Errorf("no backup method found in backup policy %s", backupPolicy.Name)
	} else if len(backupMethods) > 1 {
		return nil, fmt.Errorf("more than one backup methods found in backup policy %s", backupPolicy.Name)
	}
	backup := factory.BuildBackup(d.cluster, d.component, backupPolicy.Name, d.backupKey, backupMethods[0])
	return []client.Object{backup}, nil
}

func (d *backupDataClone) CheckBackupStatus() (backupStatus, error) {
	backup := dpv1alpha1.Backup{}
	if err := d.cli.Get(d.reqCtx.Ctx, d.backupKey, &backup); err != nil {
		if errors.IsNotFound(err) {
			return backupStatusNotCreated, nil
		} else {
			return backupStatusFailed, err
		}
	}
	if backup.Status.Phase == dpv1alpha1.BackupPhaseFailed {
		d.reqCtx.Recorder.Event(d.cluster, corev1.EventTypeWarning, string(intctrlutil.ErrorTypeBackupFailed), fmt.Sprintf("backup for horizontalScaling failed: %s",
			backup.Status.FailureReason))
		return backupStatusFailed, nil
	}
	if backup.Status.Phase == dpv1alpha1.BackupPhaseCompleted {
		return backupStatusReadyToUse, nil
	}
	return backupStatusProcessing, nil
}

func (d *backupDataClone) restore(templateName string, startingIndex int32) ([]client.Object, error) {
	backup := &dpv1alpha1.Backup{}
	if err := d.cli.Get(d.reqCtx.Ctx, d.backupKey, backup); err != nil {
		return nil, err
	}
	restoreMGR := plan.NewRestoreManager(d.reqCtx.Ctx, d.cli, d.cluster, nil, d.getBRLabels(), int32(1), startingIndex)
	restore, err := restoreMGR.BuildPrepareDataRestore(d.component, backup, templateName)
	if err != nil || restore == nil {
		return nil, err
	}
	return []client.Object{restore}, nil
}

func (d *backupDataClone) CheckRestoreStatus(templateName string, startingIndex int32) (dpv1alpha1.RestorePhase, error) {
	restoreMGR := plan.NewRestoreManager(d.reqCtx.Ctx, d.cli, d.cluster, nil, d.getBRLabels(), int32(1), startingIndex)
	restoreMeta := restoreMGR.GetRestoreObjectMeta(d.component, dpv1alpha1.PrepareData, templateName)
	restore := &dpv1alpha1.Restore{}
	if err := d.cli.Get(d.reqCtx.Ctx, types.NamespacedName{Namespace: d.cluster.Namespace, Name: restoreMeta.Name}, restore); err != nil {
		return "", client.IgnoreNotFound(err)
	}
	if restore.Status.Phase == dpv1alpha1.RestorePhaseFailed {
		d.reqCtx.Recorder.Event(d.cluster, corev1.EventTypeWarning, string(intctrlutil.ErrorTypeRestoreFailed), fmt.Sprintf(`restore for horizontalScaling failed: you can describe the restore resource "%s"`, restore.Name))
	}
	return restore.Status.Phase, nil
}

// getBackupPolicyFromTemplate gets backup policy from template policy template.
func getBackupPolicyFromTemplate(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1.Cluster,
	componentDef, backupPolicyTemplateName string) (*dpv1alpha1.BackupPolicy, error) {
	backupPolicyList := &dpv1alpha1.BackupPolicyList{}
	if err := cli.List(reqCtx.Ctx, backupPolicyList,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			constant.AppInstanceLabelKey:         cluster.Name,
			constant.ComponentDefinitionLabelKey: componentDef,
		}); err != nil {
		return nil, err
	}
	for _, backupPolicy := range backupPolicyList.Items {
		if backupPolicy.Annotations[constant.BackupPolicyTemplateAnnotationKey] == backupPolicyTemplateName {
			return &backupPolicy, nil
		}
	}
	return nil, nil
}

func backupVCT(component *component.SynthesizedComponent) *corev1.PersistentVolumeClaimTemplate {
	if len(component.VolumeClaimTemplates) == 0 {
		return nil
	}
	vct := component.VolumeClaimTemplates[0]
	for _, tmpVct := range component.VolumeClaimTemplates {
		for _, volume := range component.Volumes {
			if volume.NeedSnapshot && volume.Name == tmpVct.Name {
				vct = tmpVct
				break
			}
		}
	}
	return &vct
}

func isVolumeSnapshotEnabled(ctx context.Context, cli client.Client,
	its *workloads.InstanceSet, vct *corev1.PersistentVolumeClaimTemplate) (bool, error) {
	if its == nil || vct == nil {
		return false, nil
	}
	pvcKey := types.NamespacedName{
		Namespace: its.Namespace,
		Name:      fmt.Sprintf("%s-%s-%d", vct.Name, its.Name, 0),
	}
	pvc := corev1.PersistentVolumeClaim{}
	if err := cli.Get(ctx, pvcKey, &pvc, inDataContext4C()); err != nil {
		return false, client.IgnoreNotFound(err)
	}

	return dputils.IsVolumeSnapshotEnabled(ctx, cli, pvc.Spec.VolumeName)
}

func getBackupMethods(backupPolicy *dpv1alpha1.BackupPolicy, useVolumeSnapshot bool) []string {
	var vsMethods []string
	var otherMethods []string
	for _, method := range backupPolicy.Spec.BackupMethods {
		if method.SnapshotVolumes != nil && *method.SnapshotVolumes {
			vsMethods = append(vsMethods, method.Name)
		} else {
			otherMethods = append(otherMethods, method.Name)
		}
	}
	if useVolumeSnapshot && len(vsMethods) > 0 {
		return vsMethods
	}
	return otherMethods
}
