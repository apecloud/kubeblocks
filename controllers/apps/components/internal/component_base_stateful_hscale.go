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

package internal

import (
	"context"
	"fmt"
	"strings"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	types2 "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	controllertypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// TODO: handle unfinished jobs from previous scale in
func checkedCreateDeletePVCCronJob(reqCtx intctrlutil.RequestCtx, cli types2.ReadonlyClient,
	pvcKey types.NamespacedName, stsObj *appsv1.StatefulSet, cluster *appsv1alpha1.Cluster) (client.Object, error) {
	// hack: delete after 30 minutes
	utc := time.Now().Add(30 * time.Minute).UTC()
	schedule := fmt.Sprintf("%d %d %d %d *", utc.Minute(), utc.Hour(), utc.Day(), utc.Month())
	cronJob, err := builder.BuildCronJob(pvcKey, schedule, stsObj)
	if err != nil {
		return nil, err
	}

	job := &batchv1.CronJob{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(cronJob), job); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		reqCtx.Recorder.Eventf(cluster,
			corev1.EventTypeNormal,
			"CronJobCreate",
			"create cronjob to delete pvc/%s",
			pvcKey.Name)
		return cronJob, nil
	}
	return nil, nil
}

func isPVCExists(cli types2.ReadonlyClient, ctx context.Context,
	pvcKey types.NamespacedName) (bool, error) {
	pvc := corev1.PersistentVolumeClaim{}
	if err := cli.Get(ctx, pvcKey, &pvc); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	return true, nil
}

func isAllPVCBound(cli types2.ReadonlyClient,
	ctx context.Context,
	stsObj *appsv1.StatefulSet) (bool, error) {
	if len(stsObj.Spec.VolumeClaimTemplates) == 0 {
		return true, nil
	}
	for i := 0; i < int(*stsObj.Spec.Replicas); i++ {
		pvcKey := types.NamespacedName{
			Namespace: stsObj.Namespace,
			Name:      fmt.Sprintf("%s-%s-%d", stsObj.Spec.VolumeClaimTemplates[0].Name, stsObj.Name, i),
		}
		pvc := corev1.PersistentVolumeClaim{}
		// check pvc existence
		if err := cli.Get(ctx, pvcKey, &pvc); err != nil {
			return false, client.IgnoreNotFound(err)
		}
		if pvc.Status.Phase != corev1.ClaimBound {
			return false, nil
		}
	}
	return true, nil
}

// check volume snapshot available
func isSnapshotAvailable(cli types2.ReadonlyClient, ctx context.Context) bool {
	if !viper.GetBool("VOLUMESNAPSHOT") {
		return false
	}
	vsList := snapshotv1.VolumeSnapshotList{}
	compatClient := intctrlutil.VolumeSnapshotCompatClient{ReadonlyClient: cli, Ctx: ctx}
	getVSErr := compatClient.List(&vsList)
	return getVSErr == nil
}

func deleteSnapshot(cli types2.ReadonlyClient,
	reqCtx intctrlutil.RequestCtx,
	snapshotKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster,
	componentName string) ([]client.Object, error) {
	objs, err := deleteBackup(reqCtx.Ctx, cli, cluster.Name, componentName)
	if err != nil {
		return nil, err
	}
	if len(objs) > 0 {
		reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "BackupJobDelete", "Delete backupJob/%s", snapshotKey.Name)
	}

	vs := &snapshotv1.VolumeSnapshot{}
	compatClient := intctrlutil.VolumeSnapshotCompatClient{ReadonlyClient: cli, Ctx: reqCtx.Ctx}
	if err := compatClient.Get(snapshotKey, vs); err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	objs = append(objs, vs)
	reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "VolumeSnapshotDelete", "Delete volumeSnapshot/%s", snapshotKey.Name)

	return objs, nil
}

// deleteBackup will delete all backup related resources created during horizontal scaling,
func deleteBackup(ctx context.Context, cli types2.ReadonlyClient, clusterName string, componentName string) ([]client.Object, error) {
	ml := getBackupMatchingLabels(clusterName, componentName)
	backupList := dataprotectionv1alpha1.BackupList{}
	if err := cli.List(ctx, &backupList, ml); err != nil {
		return nil, err
	}
	objs := make([]client.Object, 0)
	for i := range backupList.Items {
		objs = append(objs, &backupList.Items[i])
	}
	return objs, nil
}

func getBackupMatchingLabels(clusterName string, componentName string) client.MatchingLabels {
	return client.MatchingLabels{
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
		constant.KBManagedByKey:         "cluster", // the resources are managed by which controller
	}
}

func doBackup(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	backupKey types.NamespacedName,
	stsProto *appsv1.StatefulSet,
	stsObj *appsv1.StatefulSet) ([]client.Object, error) {

	backupVCT := func() corev1.PersistentVolumeClaimTemplate {
		vcts := component.VolumeClaimTemplates
		vct := vcts[0]
		for _, tmpVct := range vcts {
			if tmpVct.Name == component.HorizontalScalePolicy.VolumeMountsName {
				vct = tmpVct
				break
			}
		}
		return vct
	}

	toCreatePVCKeys := func() []types.NamespacedName {
		var pvcKeys []types.NamespacedName
		vct := backupVCT()
		for i := *stsObj.Spec.Replicas; i < *stsProto.Spec.Replicas; i++ {
			pvcKey := types.NamespacedName{
				Namespace: stsObj.Namespace,
				Name: fmt.Sprintf("%s-%s-%d",
					vct.Name,
					stsObj.Name,
					i),
			}
			pvcKeys = append(pvcKeys, pvcKey)
		}
		return pvcKeys
	}

	if component.HorizontalScalePolicy == nil {
		return nil, nil
	}

	objs := make([]client.Object, 0)

	// do backup according to component's horizontal scale policy
	switch component.HorizontalScalePolicy.Type {
	// use backup tool such as xtrabackup
	case appsv1alpha1.HScaleDataClonePolicyFromBackup:
		// TODO: db core not support yet, leave it empty
		reqCtx.Recorder.Eventf(cluster,
			corev1.EventTypeWarning,
			"HorizontalScaleFailed",
			"scale with backup tool not support yet")
		// check backup ready
		backupStatus, err := CheckBackupStatus(reqCtx, cli, backupKey)
		if err != nil {
			return nil, err
		}
		switch backupStatus {
		case BackupStatusNotCreated:
			backupObjs, err := Backup(reqCtx,
				cli,
				backupKey,
				stsObj,
				cluster,
				component.ComponentDef,
				component.HorizontalScalePolicy.BackupPolicyTemplateName)
			if err != nil {
				return nil, err
			}
			objs = append(objs, backupObjs...)
			return objs, controllertypes.NewRequeueError(controllertypes.RequeueDuration, "")
		case BackupStatusProcessing:
			return objs, controllertypes.NewRequeueError(controllertypes.RequeueDuration, "")
		case BackupStatusReadyToUse:
			break
		}
		// backup's ready, then start to check restore
		pvcKeys := toCreatePVCKeys()
		needRequeue := false
		for _, pvcKey := range pvcKeys {
			restoreJobKey := types.NamespacedName{
				Namespace: pvcKey.Namespace,
				Name:      "restore-" + pvcKey.Name,
			}
			restoreStatus, err := CheckRestoreStatus(reqCtx, cli, restoreJobKey)
			if err != nil {
				return nil, err
			}
			switch restoreStatus {
			case RestoreStatusNotCreated:
				backup := dataprotectionv1alpha1.Backup{}
				if err := cli.Get(reqCtx.Ctx, backupKey, &backup); err != nil {
					return nil, err
				}
				ml := client.MatchingLabels{
					constant.ClusterDefLabelKey: component.ClusterDefName,
				}
				backupToolList := dataprotectionv1alpha1.BackupToolList{}
				if err := cli.List(reqCtx.Ctx, &backupToolList, ml); err != nil {
					return nil, err
				}
				if len(backupToolList.Items) == 0 {
					return nil, fmt.Errorf("backuptool not found for clusterdefinition: %s", component.ClusterDefName)
				}
				restoreObjs, err := Restore(restoreJobKey, stsObj, component, &backup, &backupToolList.Items[0], pvcKey)
				if err != nil {
					return nil, err
				}
				objs = append(objs, restoreObjs...)
				needRequeue = true
			case RestoreStatusProcessing:
				needRequeue = true
			case RestoreStatusReadyToUse:
				break
			}
		}
		if needRequeue {
			return objs, controllertypes.NewRequeueError(controllertypes.RequeueDuration, "")
		}
		// restore to pvcs all ready
		return objs, nil
	// use volume snapshot
	case appsv1alpha1.HScaleDataClonePolicyFromSnapshot:
		if !isSnapshotAvailable(cli, reqCtx.Ctx) {
			// TODO: add ut
			return nil, fmt.Errorf("HorizontalScaleFailed: volume snapshot not support")
		}
		vcts := component.VolumeClaimTemplates
		if len(vcts) == 0 {
			reqCtx.Recorder.Eventf(cluster,
				corev1.EventTypeNormal,
				"HorizontalScale",
				"no VolumeClaimTemplates, no need to do data clone.")
			break
		}
		vsExists, err := isVolumeSnapshotExists(cli, reqCtx.Ctx, cluster, component)
		if err != nil {
			return nil, err
		}
		// if volumesnapshot not exist, do snapshot to create it.
		if !vsExists {
			if snapshots, err := doSnapshot(cli,
				reqCtx,
				cluster,
				backupKey,
				stsObj,
				vcts,
				component.ComponentDef,
				component.HorizontalScalePolicy.BackupPolicyTemplateName); err != nil {
				return nil, err
			} else {
				objs = append(objs, snapshots...)
			}
			break
		}
		// volumesnapshot exists, then check if it is ready to use.
		ready, err := isVolumeSnapshotReadyToUse(cli, reqCtx.Ctx, cluster, component)
		if err != nil {
			return nil, err
		}
		// volumesnapshot not ready, wait for it to be ready by reconciling.
		if !ready {
			break
		}
		// if volumesnapshot ready,
		// create pvc from snapshot for every new pod
		pvcKeys := toCreatePVCKeys()
		vct := backupVCT()
		for _, pvcKey := range pvcKeys {
			if pvc, err := checkedCreatePVCFromSnapshot(cli,
				reqCtx.Ctx,
				pvcKey,
				cluster,
				component,
				vct,
				stsObj); err != nil {
				reqCtx.Log.Error(err, "checkedCreatePVCFromSnapshot failed")
				return nil, err
			} else if pvc != nil {
				objs = append(objs, pvc)
			}
		}
	// do nothing
	case appsv1alpha1.HScaleDataClonePolicyNone:
		break
	}
	return objs, nil
}

// check snapshot existence
func isVolumeSnapshotExists(cli types2.ReadonlyClient,
	ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) (bool, error) {
	ml := getBackupMatchingLabels(cluster.Name, component.Name)
	vsList := snapshotv1.VolumeSnapshotList{}
	compatClient := intctrlutil.VolumeSnapshotCompatClient{ReadonlyClient: cli, Ctx: ctx}
	if err := compatClient.List(&vsList, ml); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	for _, vs := range vsList.Items {
		// when do h-scale very shortly after last h-scale,
		// the last volume snapshot could not be deleted completely
		if vs.DeletionTimestamp.IsZero() {
			return true, nil
		}
	}
	return false, nil
}

func doSnapshot(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster,
	snapshotKey types.NamespacedName,
	stsObj *appsv1.StatefulSet,
	vcts []corev1.PersistentVolumeClaimTemplate,
	componentDef,
	backupPolicyTemplateName string) ([]client.Object, error) {
	backupPolicyTemplate := &appsv1alpha1.BackupPolicyTemplate{}
	err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: backupPolicyTemplateName}, backupPolicyTemplate)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	if err != nil {
		// no backuppolicytemplate, then try native volumesnapshot
		pvcName := strings.Join([]string{vcts[0].Name, stsObj.Name, "0"}, "-")
		snapshot, err := builder.BuildVolumeSnapshot(snapshotKey, pvcName, stsObj)
		if err != nil {
			return nil, err
		}
		reqCtx.Eventf(cluster, corev1.EventTypeNormal, "VolumeSnapshotCreate", "Create volumesnapshot/%s", snapshotKey.Name)
		return []client.Object{snapshot}, nil
	}

	// if there is backuppolicytemplate created by provider
	// create backupjob CR, will ignore error if already exists
	return createBackup(reqCtx, cli, stsObj, componentDef, backupPolicyTemplateName, snapshotKey, cluster)
}

// check snapshot ready to use
func isVolumeSnapshotReadyToUse(cli types2.ReadonlyClient,
	ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) (bool, error) {
	ml := getBackupMatchingLabels(cluster.Name, component.Name)
	vsList := snapshotv1.VolumeSnapshotList{}
	compatClient := intctrlutil.VolumeSnapshotCompatClient{ReadonlyClient: cli, Ctx: ctx}
	if err := compatClient.List(&vsList, ml); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	if len(vsList.Items) == 0 || vsList.Items[0].Status == nil {
		return false, nil
	}
	status := vsList.Items[0].Status
	if status.Error != nil {
		return false, errors.New("VolumeSnapshot/" + vsList.Items[0].Name + ": " + *status.Error.Message)
	}
	if status.ReadyToUse == nil {
		return false, nil
	}
	return *status.ReadyToUse, nil
}

func checkedCreatePVCFromSnapshot(cli types2.ReadonlyClient,
	ctx context.Context,
	pvcKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	vct corev1.PersistentVolumeClaimTemplate,
	stsObj *appsv1.StatefulSet) (client.Object, error) {
	pvc := corev1.PersistentVolumeClaim{}
	// check pvc existence
	if err := cli.Get(ctx, pvcKey, &pvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		ml := getBackupMatchingLabels(cluster.Name, component.Name)
		vsList := snapshotv1.VolumeSnapshotList{}
		compatClient := intctrlutil.VolumeSnapshotCompatClient{ReadonlyClient: cli, Ctx: ctx}
		if err := compatClient.List(&vsList, ml); err != nil {
			return nil, err
		}
		if len(vsList.Items) == 0 {
			return nil, fmt.Errorf("volumesnapshot not found in cluster %s component %s", cluster.Name, component.Name)
		}
		// exclude volumes that are deleting
		vsName := ""
		for _, vs := range vsList.Items {
			if vs.DeletionTimestamp != nil {
				continue
			}
			vsName = vs.Name
			break
		}
		return createPVCFromSnapshot(vct, stsObj, pvcKey, vsName, component)
	}
	return nil, nil
}

// createBackup create backup resources required to do backup,
func createBackup(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	sts *appsv1.StatefulSet,
	componentDef,
	backupPolicyTemplateName string,
	backupKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster) ([]client.Object, error) {
	objs := make([]client.Object, 0)
	createBackup := func(backupPolicyName string) error {
		backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Namespace: backupKey.Namespace, Name: backupPolicyName}, backupPolicy); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		// wait for backupPolicy created
		if len(backupPolicy.Name) == 0 {
			return nil
		}
		backupList := dataprotectionv1alpha1.BackupList{}
		ml := getBackupMatchingLabels(cluster.Name, sts.Labels[constant.KBAppComponentLabelKey])
		if err := cli.List(reqCtx.Ctx, &backupList, ml); err != nil {
			return err
		}
		if len(backupList.Items) > 0 {
			// check backup status, if failed return error
			if backupList.Items[0].Status.Phase == dataprotectionv1alpha1.BackupFailed {
				return intctrlutil.NewErrorf(intctrlutil.ErrorTypeBackupFailed, "backup for horizontalScaling failed: %s",
					backupList.Items[0].Status.FailureReason)
			}
			return nil
		}
		backup, err := builder.BuildBackup(sts, backupPolicyName, backupKey, "snapshot")
		if err != nil {
			return err
		}
		objs = append(objs, backup)
		return nil
	}
	backupPolicy, err := GetBackupPolicyFromTemplate(reqCtx, cli, cluster, componentDef, backupPolicyTemplateName)
	if err != nil {
		return nil, err
	}
	if backupPolicy == nil {
		return nil, intctrlutil.NewNotFound("not found any backup policy created by %s", backupPolicyTemplateName)
	}
	if err = createBackup(backupPolicy.Name); err != nil {
		return nil, err
	}

	reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "BackupJobCreate", "Create backupJob/%s", backupKey.Name)
	return objs, nil
}

func createPVCFromSnapshot(vct corev1.PersistentVolumeClaimTemplate,
	sts *appsv1.StatefulSet,
	pvcKey types.NamespacedName,
	snapshotName string,
	component *component.SynthesizedComponent) (client.Object, error) {
	pvc, err := builder.BuildPVCFromSnapshot(sts, vct, pvcKey, snapshotName, component)
	if err != nil {
		return nil, err
	}
	return pvc, nil
}
