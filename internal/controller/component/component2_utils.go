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

package component

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/apecloud/kubeblocks/internal/generics"
	"reflect"
	"strings"
	"time"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	types2 "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func listObjWithLabelsInNamespace[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](
	reqCtx intctrlutil.RequestCtx, cli client.Client, _ func(T, L), namespace string, labels map[string]string) ([]PT, error) {
	var objList L
	if err := cli.List(reqCtx.Ctx, PL(&objList), client.MatchingLabels(labels), client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	objs := make([]PT, 0)
	for _, obj := range reflect.ValueOf(&objList).Elem().FieldByName("Items").Interface().([]T) {
		objs = append(objs, &obj)
	}
	return objs, nil
}

func listPodOwnedByComponent(reqCtx intctrlutil.RequestCtx, cli client.Client,
	namespace, clusterName, compName string) ([]*corev1.Pod, error) {
	labels := map[string]string{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: compName,
	}
	return listObjWithLabelsInNamespace(reqCtx, cli, generics.PodSignature, namespace, labels)
}

func listStsOwnedByComponent(reqCtx intctrlutil.RequestCtx, cli client.Client,
	namespace, clusterName, compName string) ([]*appsv1.StatefulSet, error) {
	labels := map[string]string{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: compName,
	}
	return listObjWithLabelsInNamespace(reqCtx, cli, generics.StatefulSetSignature, namespace, labels)
}

func listDeployOwnedByComponent(reqCtx intctrlutil.RequestCtx, cli client.Client,
	namespace, clusterName, compName string) ([]*appsv1.Deployment, error) {
	labels := map[string]string{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: compName,
	}
	return listObjWithLabelsInNamespace(reqCtx, cli, generics.DeploymentSignature, namespace, labels)
}

func getClusterBackupSourceMap(cluster appsv1alpha1.Cluster) (map[string]string, error) {
	compBackupMapString := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]
	if len(compBackupMapString) == 0 {
		return nil, nil
	}
	compBackupMap := map[string]string{}
	err := json.Unmarshal([]byte(compBackupMapString), &compBackupMap)
	return compBackupMap, err
}

func getComponentBackupSource(cluster appsv1alpha1.Cluster, compName string) (string, error) {
	backupSources, err := getClusterBackupSourceMap(cluster)
	if err != nil {
		return "", err
	}
	if source, ok := backupSources[compName]; ok {
		return source, nil
	}
	return "", nil
}

func buildRestoreInfoFromBackup(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster appsv1alpha1.Cluster,
	component *SynthesizedComponent) error {
	// build info that needs to be restored from backup
	backupSourceName, err := getComponentBackupSource(cluster, component.Name)
	if err != nil {
		return err
	}
	if len(backupSourceName) == 0 {
		return nil
	}

	backup, backupTool, err := getBackupObjects(reqCtx, cli, cluster.Namespace, backupSourceName)
	if err != nil {
		return err
	}
	return BuildRestoredInfo2(component, backup, backupTool)
}

func updateTLSVolumeAndVolumeMount(podSpec *corev1.PodSpec, clusterName string, component SynthesizedComponent) error {
	if !component.TLS {
		return nil
	}

	// update volume
	volumes := podSpec.Volumes
	volume, err := composeTLSVolume(clusterName, component)
	if err != nil {
		return err
	}
	volumes = append(volumes, *volume)
	podSpec.Volumes = volumes

	// update volumeMount
	for index, container := range podSpec.Containers {
		volumeMounts := container.VolumeMounts
		volumeMount := composeTLSVolumeMount()
		volumeMounts = append(volumeMounts, volumeMount)
		podSpec.Containers[index].VolumeMounts = volumeMounts
	}

	return nil
}

func composeTLSVolume(clusterName string, component SynthesizedComponent) (*corev1.Volume, error) {
	if !component.TLS {
		return nil, errors.New("can't compose TLS volume when TLS not enabled")
	}
	if component.Issuer == nil {
		return nil, errors.New("issuer shouldn't be nil when TLS enabled")
	}
	if component.Issuer.Name == appsv1alpha1.IssuerUserProvided && component.Issuer.SecretRef == nil {
		return nil, errors.New("secret ref shouldn't be nil when issuer is UserProvided")
	}

	var secretName, ca, cert, key string
	switch component.Issuer.Name {
	case appsv1alpha1.IssuerKubeBlocks:
		secretName = plan.GenerateTLSSecretName(clusterName, component.Name)
		ca = builder.CAName
		cert = builder.CertName
		key = builder.KeyName
	case appsv1alpha1.IssuerUserProvided:
		secretName = component.Issuer.SecretRef.Name
		ca = component.Issuer.SecretRef.CA
		cert = component.Issuer.SecretRef.Cert
		key = component.Issuer.SecretRef.Key
	}
	volume := corev1.Volume{
		Name: builder.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
				Items: []corev1.KeyToPath{
					{Key: ca, Path: builder.CAName},
					{Key: cert, Path: builder.CertName},
					{Key: key, Path: builder.KeyName},
				},
				Optional: func() *bool { o := false; return &o }(),
			},
		},
	}

	return &volume, nil
}

func composeTLSVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      builder.VolumeName,
		MountPath: builder.MountPath,
		ReadOnly:  true,
	}
}

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
			return false, err
		}
		if pvc.Status.Phase != corev1.ClaimBound {
			return false, nil
		}
	}
	return true, nil
}

// check volume snapshot available
func isSnapshotAvailable(cli types2.ReadonlyClient, ctx context.Context) bool {
	vsList := snapshotv1.VolumeSnapshotList{}
	getVSErr := cli.List(ctx, &vsList)
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
	if err := cli.Get(reqCtx.Ctx, snapshotKey, vs); err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	objs = append(objs, vs)
	reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "VolumeSnapshotDelete", "Delete volumeSnapshot/%s", snapshotKey.Name)

	return objs, nil
}

// deleteBackup will delete all backup related resources created during horizontal scaling,
func deleteBackup(ctx context.Context, cli types2.ReadonlyClient, clusterName string, componentName string) ([]client.Object, error) {

	objs := make([]client.Object, 0)

	ml := getBackupMatchingLabels(clusterName, componentName)

	deleteBackupPolicy := func() error {
		backupPolicyList := dataprotectionv1alpha1.BackupPolicyList{}
		if err := cli.List(ctx, &backupPolicyList, ml); err != nil {
			return client.IgnoreNotFound(err)
		}
		for _, backupPolicy := range backupPolicyList.Items {
			objs = append(objs, &backupPolicy)
		}
		return nil
	}

	deleteRelatedBackups := func() error {
		backupList := dataprotectionv1alpha1.BackupList{}
		if err := cli.List(ctx, &backupList, ml); err != nil {
			return client.IgnoreNotFound(err)
		}
		for _, backup := range backupList.Items {
			objs = append(objs, &backup)
		}
		return nil
	}

	if err := deleteBackupPolicy(); err != nil {
		return nil, err
	}
	if err := deleteRelatedBackups(); err != nil {
		return nil, err
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
	cli types2.ReadonlyClient,
	cluster *appsv1alpha1.Cluster,
	component *SynthesizedComponent,
	snapshotKey types.NamespacedName,
	stsProto *appsv1.StatefulSet,
	stsObj *appsv1.StatefulSet) ([]client.Object, error) {
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
	// use volume snapshot
	case appsv1alpha1.HScaleDataClonePolicyFromSnapshot:
		if !isSnapshotAvailable(cli, reqCtx.Ctx) {
			reqCtx.Recorder.Eventf(cluster,
				corev1.EventTypeWarning,
				"HorizontalScaleFailed",
				"volume snapshot not support")
			// TODO: add ut
			return nil, fmt.Errorf("volume snapshot not support")
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
				snapshotKey,
				stsObj,
				vcts,
				component.HorizontalScalePolicy.BackupTemplateSelector); err != nil {
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
			if pvc, err := checkedCreatePVCFromSnapshot(cli,
				reqCtx.Ctx,
				pvcKey,
				cluster,
				component.Name,
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
	component *SynthesizedComponent) (bool, error) {
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

func doSnapshot(cli types2.ReadonlyClient,
	reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster,
	snapshotKey types.NamespacedName,
	stsObj *appsv1.StatefulSet,
	vcts []corev1.PersistentVolumeClaimTemplate,
	backupTemplateSelector map[string]string) ([]client.Object, error) {
	ml := client.MatchingLabels(backupTemplateSelector)
	backupPolicyTemplateList := dataprotectionv1alpha1.BackupPolicyTemplateList{}
	// find backuppolicytemplate by clusterdefinition
	if err := cli.List(reqCtx.Ctx, &backupPolicyTemplateList, ml); err != nil {
		return nil, err
	}

	objs := make([]client.Object, 0)
	if len(backupPolicyTemplateList.Items) > 0 {
		// if there is backuppolicytemplate created by provider
		// create backupjob CR, will ignore error if already exists
		backups, err := createBackup(reqCtx, cli, stsObj, &backupPolicyTemplateList.Items[0], snapshotKey, cluster)
		if err != nil {
			return nil, err
		}
		objs = append(objs, backups...)
	} else {
		// no backuppolicytemplate, then try native volumesnapshot
		pvcName := strings.Join([]string{vcts[0].Name, stsObj.Name, "0"}, "-")
		snapshot, err := builder.BuildVolumeSnapshot(snapshotKey, pvcName, stsObj)
		if err != nil {
			return nil, err
		}
		if err := controllerutil.SetControllerReference(cluster, snapshot, scheme); err != nil {
			return nil, err
		}
		objs = append(objs, snapshot)

		// TODO: why set it again?
		scheme, _ := appsv1alpha1.SchemeBuilder.Build()
		// TODO: SetOwnership
		if err := controllerutil.SetControllerReference(cluster, snapshot, scheme); err != nil {
			return nil, err
		}
		reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "VolumeSnapshotCreate", "Create volumesnapshot/%s", snapshotKey.Name)
	}
	return objs, nil
}

// check snapshot ready to use
func isVolumeSnapshotReadyToUse(cli types2.ReadonlyClient,
	ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	component *SynthesizedComponent) (bool, error) {
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

func checkedCreatePVCFromSnapshot(cli types2.ReadonlyClient,
	ctx context.Context,
	pvcKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster,
	componentName string,
	vct corev1.PersistentVolumeClaimTemplate,
	stsObj *appsv1.StatefulSet) (client.Object, error) {
	pvc := corev1.PersistentVolumeClaim{}
	// check pvc existence
	if err := cli.Get(ctx, pvcKey, &pvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		ml := getBackupMatchingLabels(cluster.Name, componentName)
		vsList := snapshotv1.VolumeSnapshotList{}
		if err := cli.List(ctx, &vsList, ml); err != nil {
			return nil, err
		}
		if len(vsList.Items) == 0 {
			return nil, fmt.Errorf("volumesnapshot not found in cluster %s component %s", cluster.Name, componentName)
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
		return createPVCFromSnapshot(vct, cluster, stsObj, pvcKey, vsName)
	}
	return nil, nil
}

// createBackup create backup resources required to do backup,
func createBackup(reqCtx intctrlutil.RequestCtx,
	cli types2.ReadonlyClient,
	sts *appsv1.StatefulSet,
	backupPolicyTemplate *dataprotectionv1alpha1.BackupPolicyTemplate,
	backupKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster) ([]client.Object, error) {

	objs := make([]client.Object, 0)

	createBackupPolicy := func() (backupPolicyName string, err error) {
		backupPolicyName = ""
		backupPolicyList := dataprotectionv1alpha1.BackupPolicyList{}
		ml := getBackupMatchingLabels(cluster.Name, sts.Labels[constant.KBAppComponentLabelKey])
		if err = cli.List(reqCtx.Ctx, &backupPolicyList, ml); err != nil {
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
		backupPolicyName = backupPolicy.Name
		objs = append(objs, backupPolicy)
		return
	}

	createBackup := func(backupPolicyName string) error {
		backupList := dataprotectionv1alpha1.BackupList{}
		ml := getBackupMatchingLabels(cluster.Name, sts.Labels[constant.KBAppComponentLabelKey])
		if err := cli.List(reqCtx.Ctx, &backupList, ml); err != nil {
			return err
		}
		if len(backupList.Items) > 0 {
			// check backup status, if failed return error
			if backupList.Items[0].Status.Phase == dataprotectionv1alpha1.BackupFailed {
				reqCtx.Recorder.Eventf(cluster, corev1.EventTypeWarning,
					"HorizontalScaleFailed", "backup %s status failed", backupKey.Name)
				return fmt.Errorf("cluster %s h-scale failed, backup error: %s",
					cluster.Name, backupList.Items[0].Status.FailureReason)
			}
			return nil
		}
		backup, err := builder.BuildBackup(sts, backupPolicyName, backupKey)
		if err != nil {
			return err
		}
		if err := controllerutil.SetControllerReference(cluster, backup, scheme); err != nil {
			return err
		}
		objs = append(objs, backup)
		return nil
	}

	backupPolicyName, err := createBackupPolicy()
	if err != nil {
		return nil, err
	}
	if err := createBackup(backupPolicyName); err != nil {
		return nil, err
	}

	reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "BackupJobCreate", "Create backupJob/%s", backupKey.Name)
	return objs, nil
}

func createPVCFromSnapshot(vct corev1.PersistentVolumeClaimTemplate,
	cluster *appsv1alpha1.Cluster,
	sts *appsv1.StatefulSet,
	pvcKey types.NamespacedName,
	snapshotName string) (client.Object, error) {
	pvc, err := builder.BuildPVCFromSnapshot(sts, vct, pvcKey, snapshotName)
	if err != nil {
		return nil, err
	}
	intctrlutil.SetOwnership(cluster, pvc, scheme, dbClusterFinalizerName)
	return pvc, nil
}
