/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apps

import (
	"context"
	"fmt"
	"strings"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// mergeServiceAnnotations keeps the original annotations except prometheus scrape annotations.
// if annotations exist and are replaced, the Service will be updated.
func mergeServiceAnnotations(originalAnnotations, targetAnnotations map[string]string) map[string]string {
	if len(originalAnnotations) == 0 {
		return targetAnnotations
	}
	tmpAnnotations := make(map[string]string, len(originalAnnotations)+len(targetAnnotations))
	for k, v := range originalAnnotations {
		if !strings.HasPrefix(k, "prometheus.io") {
			tmpAnnotations[k] = v
		}
	}
	maps.Copy(tmpAnnotations, targetAnnotations)
	return tmpAnnotations
}

// createBackup create backup resources required to do backup,
func createBackup(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	sts *appsv1.StatefulSet,
	backupPolicyTemplate *dataprotectionv1alpha1.BackupPolicyTemplate,
	backupKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster) error {
	ctx := reqCtx.Ctx

	createBackupPolicy := func() (backupPolicyName string, err error) {
		backupPolicyName = ""
		backupPolicyList := dataprotectionv1alpha1.BackupPolicyList{}
		ml := getBackupMatchingLabels(cluster.Name, sts.Labels[constant.KBAppComponentLabelKey])
		if err = cli.List(ctx, &backupPolicyList, ml); err != nil {
			return
		}
		if len(backupPolicyList.Items) > 0 {
			backupPolicyName = backupPolicyList.Items[0].Name
			return
		}
		backupPolicy, err := builder.BuildBackupPolicy(sts, backupPolicyTemplate, backupKey)
		if err != nil {
			return
		}
		if err = cli.Create(ctx, backupPolicy); err != nil {
			return backupPolicyName, intctrlutil.IgnoreIsAlreadyExists(err)
		}
		// wait 1 second in order to list the newly created backuppolicy
		time.Sleep(time.Second)
		if err = cli.List(ctx, &backupPolicyList, ml); err != nil {
			return
		}
		if len(backupPolicyList.Items) == 0 ||
			len(backupPolicyList.Items[0].Name) == 0 {
			err = errors.Errorf("Can not find backuppolicy name for cluster %s", cluster.Name)
			return
		}
		backupPolicyName = backupPolicyList.Items[0].Name
		return
	}

	createBackup := func(backupPolicyName string) error {
		backupList := dataprotectionv1alpha1.BackupList{}
		ml := getBackupMatchingLabels(cluster.Name, sts.Labels[constant.KBAppComponentLabelKey])
		if err := cli.List(ctx, &backupList, ml); err != nil {
			return err
		}
		if len(backupList.Items) > 0 {
			// check backup status, if failed return error
			if backupList.Items[0].Status.Phase == dataprotectionv1alpha1.BackupFailed {
				reqCtx.Recorder.Eventf(cluster, corev1.EventTypeWarning,
					"HorizontalScaleFailed", "backup %s status failed", backupKey.Name)
				return errors.Errorf("cluster %s h-scale failed, backup error: %s",
					cluster.Name, backupList.Items[0].Status.FailureReason)
			}
			return nil
		}
		backup, err := builder.BuildBackup(sts, backupPolicyName, backupKey)
		if err != nil {
			return err
		}
		scheme, _ := appsv1alpha1.SchemeBuilder.Build()
		if err := controllerutil.SetOwnerReference(cluster, backup, scheme); err != nil {
			return err
		}
		if err := cli.Create(ctx, backup); err != nil {
			return intctrlutil.IgnoreIsAlreadyExists(err)
		}
		return nil
	}

	backupPolicyName, err := createBackupPolicy()
	if err != nil {
		return err
	}
	if err := createBackup(backupPolicyName); err != nil {
		return err
	}

	reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "BackupJobCreate", "Create backupjob/%s", backupKey.Name)
	return nil
}

// deleteBackup will delete all backup related resources created during horizontal scaling,
func deleteBackup(ctx context.Context, cli client.Client, clusterName string, componentName string) error {

	ml := getBackupMatchingLabels(clusterName, componentName)

	deleteBackupPolicy := func() error {
		backupPolicyList := dataprotectionv1alpha1.BackupPolicyList{}
		if err := cli.List(ctx, &backupPolicyList, ml); err != nil {
			return err
		}
		for _, backupPolicy := range backupPolicyList.Items {
			if err := cli.Delete(ctx, &backupPolicy); err != nil {
				return client.IgnoreNotFound(err)
			}
		}
		return nil
	}

	deleteRelatedBackups := func() error {
		backupList := dataprotectionv1alpha1.BackupList{}
		if err := cli.List(ctx, &backupList, ml); err != nil {
			return err
		}
		for _, backup := range backupList.Items {
			if err := cli.Delete(ctx, &backup); err != nil {
				return client.IgnoreNotFound(err)
			}
		}
		return nil
	}

	if err := deleteBackupPolicy(); err != nil {
		return err
	}

	return deleteRelatedBackups()
}

func createPVCFromSnapshot(ctx context.Context,
	cli client.Client,
	vct corev1.PersistentVolumeClaimTemplate,
	sts *appsv1.StatefulSet,
	pvcKey types.NamespacedName,
	snapshotName string) error {
	pvc, err := builder.BuildPVCFromSnapshot(sts, vct, pvcKey, snapshotName)
	if err != nil {
		return err
	}
	if err := cli.Create(ctx, pvc); err != nil {
		return intctrlutil.IgnoreIsAlreadyExists(err)
	}
	return nil
}

// check volume snapshot available
func isSnapshotAvailable(cli client.Client, ctx context.Context) bool {
	vsList := snapshotv1.VolumeSnapshotList{}
	getVSErr := cli.List(ctx, &vsList)
	return getVSErr == nil
}

// check snapshot existence
func isVolumeSnapshotExists(cli client.Client,
	ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) (bool, error) {
	ml := getBackupMatchingLabels(cluster.Name, component.Name)
	vsList := snapshotv1.VolumeSnapshotList{}
	if err := cli.List(ctx, &vsList, ml); err != nil {
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

// check snapshot ready to use
func isVolumeSnapshotReadyToUse(cli client.Client,
	ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) (bool, error) {
	ml := getBackupMatchingLabels(cluster.Name, component.Name)
	vsList := snapshotv1.VolumeSnapshotList{}
	if err := cli.List(ctx, &vsList, ml); err != nil {
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

func doSnapshot(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster,
	snapshotKey types.NamespacedName,
	stsObj *appsv1.StatefulSet,
	vcts []corev1.PersistentVolumeClaimTemplate,
	backupTemplateSelector map[string]string) error {

	ctx := reqCtx.Ctx

	ml := client.MatchingLabels(backupTemplateSelector)
	backupPolicyTemplateList := dataprotectionv1alpha1.BackupPolicyTemplateList{}
	// find backuppolicytemplate by clusterdefinition
	if err := cli.List(ctx, &backupPolicyTemplateList, ml); err != nil {
		return err
	}
	if len(backupPolicyTemplateList.Items) > 0 {
		// if there is backuppolicytemplate created by provider
		// create backupjob CR, will ignore error if already exists
		err := createBackup(reqCtx, cli, stsObj, &backupPolicyTemplateList.Items[0], snapshotKey, cluster)
		if err != nil {
			return err
		}
	} else {
		// no backuppolicytemplate, then try native volumesnapshot
		pvcName := strings.Join([]string{vcts[0].Name, stsObj.Name, "0"}, "-")
		snapshot, err := builder.BuildVolumeSnapshot(snapshotKey, pvcName, stsObj)
		if err != nil {
			return err
		}
		if err := cli.Create(ctx, snapshot); err != nil {
			return intctrlutil.IgnoreIsAlreadyExists(err)
		}
		scheme, _ := appsv1alpha1.SchemeBuilder.Build()
		if err := controllerutil.SetOwnerReference(cluster, snapshot, scheme); err != nil {
			return err
		}
		reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "VolumeSnapshotCreate", "Create volumesnapshot/%s", snapshotKey.Name)
	}
	return nil
}

func checkedCreatePVCFromSnapshot(cli client.Client,
	ctx context.Context,
	pvcKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster,
	componentName string,
	vct corev1.PersistentVolumeClaimTemplate,
	stsObj *appsv1.StatefulSet) error {
	pvc := corev1.PersistentVolumeClaim{}
	// check pvc existence
	if err := cli.Get(ctx, pvcKey, &pvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		ml := getBackupMatchingLabels(cluster.Name, componentName)
		vsList := snapshotv1.VolumeSnapshotList{}
		if err := cli.List(ctx, &vsList, ml); err != nil {
			return err
		}
		if len(vsList.Items) == 0 {
			return errors.Errorf("volumesnapshot not found in cluster %s component %s", cluster.Name, componentName)
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
		return createPVCFromSnapshot(ctx, cli, vct, stsObj, pvcKey, vsName)
	}
	return nil
}

func doBackup(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	stsObj *appsv1.StatefulSet,
	stsProto *appsv1.StatefulSet,
	snapshotKey types.NamespacedName) (shouldRequeue bool, err error) {
	ctx := reqCtx.Ctx
	shouldRequeue = false
	if component.HorizontalScalePolicy == nil {
		return shouldRequeue, nil
	}
	// do backup according to component's horizontal scale policy
	switch component.HorizontalScalePolicy.Type {
	// use backup tool such as xtrabackup
	case appsv1alpha1.HScaleDataClonePolicyFromBackup:
		// TODO: db core not support yet, leave it empty
		reqCtx.Recorder.Eventf(cluster,
			corev1.EventTypeWarning,
			"HorizontalScaleFailed",
			"scale with backup tool not support yet")
	// use volume snapshot
	case appsv1alpha1.HScaleDataClonePolicyFromSnapshot:
		if !isSnapshotAvailable(cli, ctx) {
			reqCtx.Recorder.Eventf(cluster,
				corev1.EventTypeWarning,
				"HorizontalScaleFailed",
				"volume snapshot not support")
			// TODO: add ut
			return false, errors.Errorf("volume snapshot not support")
		}
		vcts := component.VolumeClaimTemplates
		if len(vcts) == 0 {
			reqCtx.Recorder.Eventf(cluster,
				corev1.EventTypeNormal,
				"HorizontalScale",
				"no VolumeClaimTemplates, no need to do data clone.")
			break
		}
		vsExists, err := isVolumeSnapshotExists(cli, ctx, cluster, component)
		if err != nil {
			return false, err
		}
		// if volumesnapshot not exist, do snapshot to create it.
		if !vsExists {
			if err := doSnapshot(cli,
				reqCtx,
				cluster,
				snapshotKey,
				stsObj,
				vcts,
				component.HorizontalScalePolicy.BackupTemplateSelector); err != nil {
				return shouldRequeue, err
			}
			shouldRequeue = true
			break
		}
		// volumesnapshot exists, then check if it is ready to use.
		ready, err := isVolumeSnapshotReadyToUse(cli, ctx, cluster, component)
		if err != nil {
			return shouldRequeue, err
		}
		// volumesnapshot not ready, wait for it to be ready by reconciling.
		if !ready {
			shouldRequeue = true
			break
		}
		// if volumesnapshot ready,
		// create pvc from snapshot for every new pod
		for i := *stsObj.Spec.Replicas; i < *stsProto.Spec.Replicas; i++ {
			vct := vcts[0]
			for _, tmpVct := range vcts {
				if tmpVct.Name == component.HorizontalScalePolicy.VolumeMountsName {
					vct = tmpVct
					break
				}
			}
			// sync vct.spec.resources from component
			for _, tmpVct := range component.VolumeClaimTemplates {
				if vct.Name == tmpVct.Name {
					vct.Spec.Resources = tmpVct.Spec.Resources
					break
				}
			}
			pvcKey := types.NamespacedName{
				Namespace: stsObj.Namespace,
				Name: fmt.Sprintf("%s-%s-%d",
					vct.Name,
					stsObj.Name,
					i),
			}
			if err := checkedCreatePVCFromSnapshot(cli,
				ctx,
				pvcKey,
				cluster,
				component.Name,
				vct,
				stsObj); err != nil {
				reqCtx.Log.Error(err, "checkedCreatePVCFromSnapshot failed")
				return shouldRequeue, err
			}
		}
	// do nothing
	case appsv1alpha1.HScaleDataClonePolicyNone:
		break
	}
	return shouldRequeue, nil
}

func getBackupMatchingLabels(clusterName string, componentName string) client.MatchingLabels {
	return client.MatchingLabels{
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
		constant.KBManagedByKey:         "cluster", // the resources are managed by which controller
	}
}

// deleteObjectOrphan delete the object with cascade=orphan.
func deleteObjectOrphan(cli client.Client, ctx context.Context, obj client.Object) error {
	deletePropagation := metav1.DeletePropagationOrphan
	deleteOptions := &client.DeleteOptions{
		PropagationPolicy: &deletePropagation,
	}

	if err := cli.Delete(ctx, obj, deleteOptions); err != nil {
		return err
	}
	return nil
}
