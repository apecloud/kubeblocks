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

package lifecycle

import (
	"context"
	"fmt"
	"strings"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	types2 "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type stsHorizontalScalingTransformer struct {
	cr  clusterRefResources
	cli types2.ReadonlyClient
	ctx intctrlutil.RequestCtx
}

func (s *stsHorizontalScalingTransformer) Transform(dag *graph.DAG) error {
	rootVertex, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	origCluster, _ := rootVertex.oriObj.(*appsv1alpha1.Cluster)
	cluster, _ := rootVertex.obj.(*appsv1alpha1.Cluster)

	if isClusterDeleting(*origCluster) {
		return nil
	}

	handleHorizontalScaling := func(vertex *lifecycleVertex) error {
		stsObj, _ := vertex.oriObj.(*appsv1.StatefulSet)
		stsProto, _ := vertex.obj.(*appsv1.StatefulSet)
		if *stsObj.Spec.Replicas == *stsProto.Spec.Replicas {
			return nil
		}

		key := client.ObjectKey{
			Namespace: stsProto.GetNamespace(),
			Name:      stsProto.GetName(),
		}
		snapshotKey := types.NamespacedName{
			Namespace: stsObj.Namespace,
			Name:      stsObj.Name + "-scaling",
		}
		// find component of current statefulset
		componentName := stsObj.Labels[constant.KBAppComponentLabelKey]
		components := mergeComponentsList(s.ctx,
			*cluster,
			s.cr.cd,
			s.cr.cd.Spec.ComponentDefs,
			cluster.Spec.ComponentSpecs)
		comp := getComponent(components, componentName)
		if comp == nil {
			s.ctx.Recorder.Eventf(cluster,
				corev1.EventTypeWarning,
				"HorizontalScaleFailed",
				"component %s not found",
				componentName)
			return nil
		}
		cleanCronJobs := func() error {
			for i := *stsObj.Spec.Replicas; i < *stsProto.Spec.Replicas; i++ {
				for _, vct := range stsObj.Spec.VolumeClaimTemplates {
					pvcKey := types.NamespacedName{
						Namespace: key.Namespace,
						Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
					}
					// delete deletion cronjob if exists
					cronJobKey := pvcKey
					cronJobKey.Name = "delete-pvc-" + pvcKey.Name
					cronJob := &batchv1.CronJob{}
					if err := s.cli.Get(s.ctx.Ctx, cronJobKey, cronJob); err != nil {
						return client.IgnoreNotFound(err)
					}
					v := &lifecycleVertex{
						obj:    cronJob,
						action: actionPtr(DELETE),
					}
					dag.AddVertex(v)
					dag.Connect(vertex, v)
				}
			}
			return nil
		}

		checkAllPVCsExist := func() (bool, error) {
			for i := *stsObj.Spec.Replicas; i < *stsProto.Spec.Replicas; i++ {
				for _, vct := range stsObj.Spec.VolumeClaimTemplates {
					pvcKey := types.NamespacedName{
						Namespace: key.Namespace,
						Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
					}
					// check pvc existence
					pvcExists, err := isPVCExists(s.cli, s.ctx.Ctx, pvcKey)
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

		checkAllPVCBoundIfNeeded := func() (bool, error) {
			if comp.HorizontalScalePolicy == nil ||
				comp.HorizontalScalePolicy.Type != appsv1alpha1.HScaleDataClonePolicyFromSnapshot ||
				!isSnapshotAvailable(s.cli, s.ctx.Ctx) {
				return true, nil
			}
			return isAllPVCBound(s.cli, s.ctx.Ctx, stsObj)
		}

		cleanBackupResourcesIfNeeded := func() error {
			if comp.HorizontalScalePolicy == nil ||
				comp.HorizontalScalePolicy.Type != appsv1alpha1.HScaleDataClonePolicyFromSnapshot ||
				!isSnapshotAvailable(s.cli, s.ctx.Ctx) {
				return nil
			}
			// if all pvc bounded, clean backup resources
			return deleteSnapshot(s.cli, s.ctx, snapshotKey, cluster, comp, dag, rootVertex)
		}

		scaleOut := func() error {
			if err := cleanCronJobs(); err != nil {
				return err
			}
			allPVCsExist, err := checkAllPVCsExist()
			if err != nil {
				return err
			}
			if !allPVCsExist {
				// do backup according to component's horizontal scale policy
				if err := doBackup(s.ctx, s.cli, comp, snapshotKey, dag, rootVertex, vertex); err != nil {
					return err
				}
				vertex.immutable = true
				return nil
			}
			// check all pvc bound, requeue if not all ready
			allPVCBounded, err := checkAllPVCBoundIfNeeded()
			if err != nil {
				return err
			}
			if !allPVCBounded {
				vertex.immutable = true
				return nil
			}
			// clean backup resources.
			// there will not be any backup resources other than scale out.
			if err := cleanBackupResourcesIfNeeded(); err != nil {
				return err
			}

			// pvcs are ready, stateful_set.replicas should be updated
			vertex.immutable = false

			return nil

		}

		scaleIn := func() error {
			// scale in, if scale in to 0, do not delete pvc
			if *stsProto.Spec.Replicas == 0 || len(stsObj.Spec.VolumeClaimTemplates) == 0 {
				return nil
			}
			for i := *stsProto.Spec.Replicas; i < *stsObj.Spec.Replicas; i++ {
				for _, vct := range stsObj.Spec.VolumeClaimTemplates {
					pvcKey := types.NamespacedName{
						Namespace: key.Namespace,
						Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
					}
					// create cronjob to delete pvc after 30 minutes
					if err := checkedCreateDeletePVCCronJob(s.cli, s.ctx, pvcKey, stsObj, cluster, dag, rootVertex); err != nil {
						return err
					}
				}
			}
			return nil
		}

		// when horizontal scaling up, sometimes db needs backup to sync data from master,
		// log is not reliable enough since it can be recycled
		var err error
		switch {
		// scale out
		case *stsObj.Spec.Replicas < *stsProto.Spec.Replicas:
			err = scaleOut()
		case *stsObj.Spec.Replicas > *stsProto.Spec.Replicas:
			err = scaleIn()
		}
		if err != nil {
			return err
		}

		if *stsObj.Spec.Replicas != *stsProto.Spec.Replicas {
			s.ctx.Recorder.Eventf(cluster,
				corev1.EventTypeNormal,
				"HorizontalScale",
				"Start horizontal scale component %s from %d to %d",
				comp.Name,
				*stsObj.Spec.Replicas,
				*stsProto.Spec.Replicas)
		}

		return nil
	}

	vertices := findAll[*appsv1.StatefulSet](dag)
	for _, vertex := range vertices {
		v, _ := vertex.(*lifecycleVertex)
		if v.obj == nil || v.oriObj == nil {
			continue
		}
		if err := handleHorizontalScaling(v); err != nil {
			return err
		}
	}
	return nil
}

func isPVCExists(cli types2.ReadonlyClient,
	ctx context.Context,
	pvcKey types.NamespacedName) (bool, error) {
	pvc := corev1.PersistentVolumeClaim{}
	if err := cli.Get(ctx, pvcKey, &pvc); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	return true, nil
}

func mergeComponentsList(reqCtx intctrlutil.RequestCtx,
	cluster appsv1alpha1.Cluster,
	clusterDef appsv1alpha1.ClusterDefinition,
	clusterCompDefList []appsv1alpha1.ClusterComponentDefinition,
	clusterCompSpecList []appsv1alpha1.ClusterComponentSpec) []component.SynthesizedComponent {
	var compList []component.SynthesizedComponent
	for _, compDef := range clusterCompDefList {
		for _, compSpec := range clusterCompSpecList {
			if compSpec.ComponentDefRef != compDef.Name {
				continue
			}
			comp := component.BuildComponent(reqCtx, cluster, clusterDef, compDef, compSpec)
			compList = append(compList, *comp)
		}
	}
	return compList
}

func getComponent(componentList []component.SynthesizedComponent, name string) *component.SynthesizedComponent {
	for _, comp := range componentList {
		if comp.Name == name {
			return &comp
		}
	}
	return nil
}

func doBackup(reqCtx intctrlutil.RequestCtx,
	cli types2.ReadonlyClient,
	component *component.SynthesizedComponent,
	snapshotKey types.NamespacedName,
	dag *graph.DAG,
	root *lifecycleVertex,
	vertex *lifecycleVertex) error {
	cluster, _ := root.obj.(*appsv1alpha1.Cluster)
	stsObj, _ := vertex.oriObj.(*appsv1.StatefulSet)
	stsProto, _ := vertex.obj.(*appsv1.StatefulSet)
	ctx := reqCtx.Ctx

	if component.HorizontalScalePolicy == nil {
		return nil
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
			return errors.Errorf("volume snapshot not support")
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
			return err
		}
		// if volumesnapshot not exist, do snapshot to create it.
		if !vsExists {
			if err := doSnapshot(cli,
				reqCtx,
				cluster,
				snapshotKey,
				stsObj,
				vcts,
				component.HorizontalScalePolicy.BackupTemplateSelector,
				dag,
				root); err != nil {
				return err
			}
			break
		}
		// volumesnapshot exists, then check if it is ready to use.
		ready, err := isVolumeSnapshotReadyToUse(cli, ctx, cluster, component)
		if err != nil {
			return err
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
			if err := checkedCreatePVCFromSnapshot(cli,
				ctx,
				pvcKey,
				cluster,
				component,
				vct,
				stsObj,
				dag,
				root); err != nil {
				reqCtx.Log.Error(err, "checkedCreatePVCFromSnapshot failed")
				return err
			}
		}
	// do nothing
	case appsv1alpha1.HScaleDataClonePolicyNone:
		break
	}
	return nil
}

// TODO: handle unfinished jobs from previous scale in
func checkedCreateDeletePVCCronJob(cli types2.ReadonlyClient,
	reqCtx intctrlutil.RequestCtx,
	pvcKey types.NamespacedName,
	stsObj *appsv1.StatefulSet,
	cluster *appsv1alpha1.Cluster,
	dag *graph.DAG,
	root graph.Vertex) error {
	ctx := reqCtx.Ctx
	now := time.Now()
	// hack: delete after 30 minutes
	t := now.Add(30 * 60 * time.Second)
	schedule := timeToSchedule(t)
	cronJob, err := builder.BuildCronJob(pvcKey, schedule, stsObj)
	if err != nil {
		return err
	}
	job := &batchv1.CronJob{}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(cronJob), job); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		vertex := &lifecycleVertex{obj: cronJob, action: actionPtr(CREATE)}
		dag.AddVertex(vertex)
		dag.Connect(root, vertex)
		reqCtx.Recorder.Eventf(cluster,
			corev1.EventTypeNormal,
			"CronJobCreate",
			"create cronjob to delete pvc/%s",
			pvcKey.Name)
	}

	return nil
}

func timeToSchedule(t time.Time) string {
	utc := t.UTC()
	return fmt.Sprintf("%d %d %d %d *", utc.Minute(), utc.Hour(), utc.Day(), utc.Month())
}

// check volume snapshot available
func isSnapshotAvailable(cli types2.ReadonlyClient, ctx context.Context) bool {
	vsList := snapshotv1.VolumeSnapshotList{}
	getVSErr := cli.List(ctx, &vsList)
	return getVSErr == nil
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

func deleteSnapshot(cli types2.ReadonlyClient,
	reqCtx intctrlutil.RequestCtx,
	snapshotKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	dag *graph.DAG,
	root graph.Vertex) error {
	ctx := reqCtx.Ctx
	if err := deleteBackup(ctx, cli, cluster.Name, component.Name, dag, root); err != nil {
		return client.IgnoreNotFound(err)
	}
	reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "BackupJobDelete", "Delete backupJob/%s", snapshotKey.Name)
	vs := &snapshotv1.VolumeSnapshot{}
	if err := cli.Get(ctx, snapshotKey, vs); err != nil {
		return client.IgnoreNotFound(err)
	}
	vertex := &lifecycleVertex{obj: vs, oriObj: vs, action: actionPtr(DELETE)}
	dag.AddVertex(vertex)
	dag.Connect(root, vertex)
	reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "VolumeSnapshotDelete", "Delete volumeSnapshot/%s", snapshotKey.Name)
	return nil
}

// deleteBackup will delete all backup related resources created during horizontal scaling,
func deleteBackup(ctx context.Context, cli types2.ReadonlyClient, clusterName string, componentName string, dag *graph.DAG, root graph.Vertex) error {

	ml := getBackupMatchingLabels(clusterName, componentName)

	deleteBackupPolicy := func() error {
		backupPolicyList := dataprotectionv1alpha1.BackupPolicyList{}
		if err := cli.List(ctx, &backupPolicyList, ml); err != nil {
			return err
		}
		for _, backupPolicy := range backupPolicyList.Items {
			vertex := &lifecycleVertex{obj: &backupPolicy, oriObj: &backupPolicy, action: actionPtr(DELETE)}
			dag.AddVertex(vertex)
			dag.Connect(root, vertex)
		}
		return nil
	}

	deleteRelatedBackups := func() error {
		backupList := dataprotectionv1alpha1.BackupList{}
		if err := cli.List(ctx, &backupList, ml); err != nil {
			return err
		}
		for _, backup := range backupList.Items {
			vertex := &lifecycleVertex{obj: &backup, oriObj: &backup, action: actionPtr(DELETE)}
			dag.AddVertex(vertex)
			dag.Connect(root, vertex)
		}
		return nil
	}

	if err := deleteBackupPolicy(); err != nil {
		return err
	}

	return deleteRelatedBackups()
}

func getBackupMatchingLabels(clusterName string, componentName string) client.MatchingLabels {
	return client.MatchingLabels{
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
		constant.KBManagedByKey:         "cluster", // the resources are managed by which controller
	}
}

// check snapshot existence
func isVolumeSnapshotExists(cli types2.ReadonlyClient,
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

func doSnapshot(cli types2.ReadonlyClient,
	reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster,
	snapshotKey types.NamespacedName,
	stsObj *appsv1.StatefulSet,
	vcts []corev1.PersistentVolumeClaimTemplate,
	backupTemplateSelector map[string]string,
	dag *graph.DAG,
	root graph.Vertex) error {

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
		err := createBackup(reqCtx, cli, stsObj, &backupPolicyTemplateList.Items[0], snapshotKey, cluster, dag, root)
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
		if err := controllerutil.SetControllerReference(cluster, snapshot, scheme); err != nil {
			return err
		}
		vertex := &lifecycleVertex{obj: snapshot, action: actionPtr(CREATE)}
		dag.AddVertex(vertex)
		dag.Connect(root, vertex)

		scheme, _ := appsv1alpha1.SchemeBuilder.Build()
		// TODO: SetOwnership
		if err := controllerutil.SetControllerReference(cluster, snapshot, scheme); err != nil {
			return err
		}
		reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "VolumeSnapshotCreate", "Create volumesnapshot/%s", snapshotKey.Name)
	}
	return nil
}

// check snapshot ready to use
func isVolumeSnapshotReadyToUse(cli types2.ReadonlyClient,
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

func checkedCreatePVCFromSnapshot(cli types2.ReadonlyClient,
	ctx context.Context,
	pvcKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	vct corev1.PersistentVolumeClaimTemplate,
	stsObj *appsv1.StatefulSet,
	dag *graph.DAG,
	root graph.Vertex) error {
	pvc := corev1.PersistentVolumeClaim{}
	// check pvc existence
	if err := cli.Get(ctx, pvcKey, &pvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		ml := getBackupMatchingLabels(cluster.Name, component.Name)
		vsList := snapshotv1.VolumeSnapshotList{}
		if err := cli.List(ctx, &vsList, ml); err != nil {
			return err
		}
		if len(vsList.Items) == 0 {
			return errors.Errorf("volumesnapshot not found in cluster %s component %s", cluster.Name, component.Name)
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
		return createPVCFromSnapshot(vct, stsObj, pvcKey, vsName, component, dag, root)
	}
	return nil
}

// createBackup create backup resources required to do backup,
func createBackup(reqCtx intctrlutil.RequestCtx,
	cli types2.ReadonlyClient,
	sts *appsv1.StatefulSet,
	backupPolicyTemplate *dataprotectionv1alpha1.BackupPolicyTemplate,
	backupKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster,
	dag *graph.DAG,
	root graph.Vertex) error {
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
		backupPolicyName = backupPolicy.Name
		vertex := &lifecycleVertex{obj: backupPolicy, action: actionPtr(CREATE)}
		dag.AddVertex(vertex)
		dag.Connect(root, vertex)
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
		if err := controllerutil.SetControllerReference(cluster, backup, scheme); err != nil {
			return err
		}
		vertex := &lifecycleVertex{obj: backup, action: actionPtr(CREATE)}
		dag.AddVertex(vertex)
		dag.Connect(root, vertex)
		return nil
	}

	backupPolicyName, err := createBackupPolicy()
	if err != nil {
		return err
	}
	if err := createBackup(backupPolicyName); err != nil {
		return err
	}

	reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "BackupJobCreate", "Create backupJob/%s", backupKey.Name)
	return nil
}

func createPVCFromSnapshot(vct corev1.PersistentVolumeClaimTemplate,
	sts *appsv1.StatefulSet,
	pvcKey types.NamespacedName,
	snapshotName string,
	component *component.SynthesizedComponent,
	dag *graph.DAG,
	root graph.Vertex) error {
	pvc, err := builder.BuildPVCFromSnapshot(sts, vct, pvcKey, snapshotName, component)
	if err != nil {
		return err
	}
	rootVertex, _ := root.(*lifecycleVertex)
	cluster, _ := rootVertex.obj.(*appsv1alpha1.Cluster)
	if err = intctrlutil.SetOwnership(cluster, pvc, scheme, dbClusterFinalizerName); err != nil {
		return err
	}
	vertex := &lifecycleVertex{obj: pvc, action: actionPtr(CREATE)}
	dag.AddVertex(vertex)
	dag.Connect(root, vertex)
	return nil
}
