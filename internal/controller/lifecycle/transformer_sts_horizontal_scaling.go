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

package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type StsHorizontalScalingTransformer struct{}

func (t *StsHorizontalScalingTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	rootVertex, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	origCluster, _ := rootVertex.oriObj.(*appsv1alpha1.Cluster)
	cluster, _ := rootVertex.obj.(*appsv1alpha1.Cluster)

	handleHorizontalScaling := func(vertex *lifecycleVertex) error {
		stsObj, _ := vertex.oriObj.(*appsv1.StatefulSet)
		stsProto, _ := vertex.obj.(*appsv1.StatefulSet)

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
		components := mergeComponentsList(reqCtx,
			*cluster,
			*transCtx.ClusterDef,
			transCtx.ClusterDef.Spec.ComponentDefs,
			cluster.Spec.ComponentSpecs)
		comp := getComponent(components, componentName)
		if comp == nil {
			if *stsObj.Spec.Replicas != *stsProto.Spec.Replicas {
				transCtx.EventRecorder.Eventf(cluster,
					corev1.EventTypeWarning,
					"HorizontalScaleFailed",
					"component %s not found",
					componentName)
			}
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
					if err := transCtx.Client.Get(transCtx.Context, cronJobKey, cronJob); err != nil {
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
					pvcExists, err := isPVCExists(transCtx.Client, transCtx.Context, pvcKey)
					if err != nil {
						return false, err
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
				!isSnapshotAvailable(transCtx.Client, transCtx.Context) {
				return true, nil
			}
			return isAllPVCBound(transCtx.Client, transCtx.Context, stsProto)
		}

		cleanBackupResourcesIfNeeded := func() error {
			if comp.HorizontalScalePolicy == nil ||
				comp.HorizontalScalePolicy.Type != appsv1alpha1.HScaleDataClonePolicyFromSnapshot ||
				!isSnapshotAvailable(transCtx.Client, transCtx.Context) {
				return nil
			}
			// if all pvc bounded, clean backup resources
			return deleteSnapshot(transCtx.Client, reqCtx, snapshotKey, cluster, comp, dag, rootVertex)
		}

		emitHorizontalScalingEvent := func() {
			if cluster.Status.Components == nil {
				return
			}
			if *stsObj.Spec.Replicas == *stsProto.Spec.Replicas {
				return
			}
			if componentStatus, ok := cluster.Status.Components[componentName]; ok {
				if componentStatus.Phase == appsv1alpha1.SpecReconcilingClusterCompPhase {
					return
				}
				transCtx.EventRecorder.Eventf(cluster,
					corev1.EventTypeNormal,
					"HorizontalScale",
					"Start horizontal scale component %s from %d to %d",
					comp.Name,
					*stsObj.Spec.Replicas,
					*stsProto.Spec.Replicas)
			}
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
				if comp.HorizontalScalePolicy == nil {
					vertex.immutable = false
					return nil
				}
				// do backup according to component's horizontal scale policy
				vertex.immutable = true
				if err := doBackup(reqCtx, transCtx.Client, comp, snapshotKey, dag, rootVertex, vertex); err != nil {
					return err
				}
				return nil
			}
			// pvcs are ready, stateful_set.replicas should be updated
			vertex.immutable = false

			return nil
		}

		postScaleOut := func() error {
			// check all pvc bound, wait next reconciliation if not all ready
			allPVCBounded, err := checkAllPVCBoundIfNeeded()
			if err != nil {
				return err
			}
			if !allPVCBounded {
				return nil
			}
			// clean backup resources.
			// there will not be any backup resources other than scale out.
			if err := cleanBackupResourcesIfNeeded(); err != nil {
				return err
			}
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
					if err := checkedCreateDeletePVCCronJob(transCtx.Client, reqCtx, pvcKey, stsObj, cluster, dag, rootVertex); err != nil {
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
		emitHorizontalScalingEvent()

		if err = postScaleOut(); err != nil {
			return err
		}

		if err = postScaleOut(); err != nil {
			return err
		}

		return nil
	}
	findPVCsToBeDeleted := func(pvcSnapshot clusterSnapshot) []*corev1.PersistentVolumeClaim {
		stsToBeDeleted := make([]*appsv1.StatefulSet, 0)
		// list sts to be deleted
		for _, vertex := range dag.Vertices() {
			v, _ := vertex.(*lifecycleVertex)
			// find sts to be deleted
			if sts, ok := v.obj.(*appsv1.StatefulSet); ok && (v.action != nil && *v.action == DELETE) {
				stsToBeDeleted = append(stsToBeDeleted, sts)
			}
		}
		// compose all pvc names that owned by sts to be deleted
		pvcNameSet := sets.New[string]()
		for _, sts := range stsToBeDeleted {
			for _, template := range sts.Spec.VolumeClaimTemplates {
				for i := 0; i < int(*sts.Spec.Replicas); i++ {
					name := fmt.Sprintf("%s-%s-%d", template.Name, sts.Name, i)
					pvcNameSet.Insert(name)
				}
			}
		}
		// pvcs that not owned by any deleting sts should be filtered
		orphanPVCs := make([]*corev1.PersistentVolumeClaim, 0)
		for _, obj := range pvcSnapshot {
			pvc, _ := obj.(*corev1.PersistentVolumeClaim)
			if pvcNameSet.Has(pvc.Name) {
				orphanPVCs = append(orphanPVCs, pvc)
			}
		}
		return orphanPVCs
	}

	// if cluster is deleting, no need h-scale
	if !isClusterDeleting(*origCluster) {
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
	}

	// find all pvcs that should be deleted when parent sts is deleting:
	// 1. cluster is deleting
	// 2. component is deleting by a cluster Update
	//
	// why handle pvc deletion here?
	// two types of pvc should be handled: generated by sts and by our h-scale transformer.
	// by sts: we only handle the pvc deletion which occurs in cluster deletion.
	// by h-scale transformer: we handle the pvc creation and deletion, the creation is handled in h-scale funcs.
	// so all in all, here we should only handle the pvc deletion of both types.
	ml := getAppInstanceML(*cluster)
	oldSnapshot, err := readCacheSnapshot(transCtx, *cluster, ml, &corev1.PersistentVolumeClaimList{})
	if err != nil {
		return err
	}
	pvcs := findPVCsToBeDeleted(oldSnapshot)
	for _, pvc := range pvcs {
		vertex := &lifecycleVertex{obj: pvc, action: actionPtr(DELETE)}
		dag.AddVertex(vertex)
		dag.Connect(rootVertex, vertex)
	}

	return nil
}

func isPVCExists(cli roclient.ReadonlyClient,
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
	cli roclient.ReadonlyClient,
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
			return fmt.Errorf("volume snapshot not support")
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
				component.ComponentDef,
				component.HorizontalScalePolicy.BackupPolicyTemplateName,
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
func checkedCreateDeletePVCCronJob(cli roclient.ReadonlyClient,
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
func isSnapshotAvailable(cli roclient.ReadonlyClient, ctx context.Context) bool {
	if !viper.GetBool("VOLUMESNAPSHOT") {
		return false
	}
	vsList := snapshotv1.VolumeSnapshotList{}
	compatClient := intctrlutil.VolumeSnapshotCompatClient{ReadonlyClient: cli, Ctx: ctx}
	getVSErr := compatClient.List(&vsList)
	return getVSErr == nil
}

func isAllPVCBound(cli roclient.ReadonlyClient,
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

func deleteSnapshot(cli roclient.ReadonlyClient,
	reqCtx intctrlutil.RequestCtx,
	snapshotKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	dag *graph.DAG,
	root graph.Vertex) error {
	ctx := reqCtx.Ctx
	if err := deleteBackup(reqCtx, cli, cluster, component.Name, snapshotKey.Name, dag, root); err != nil {
		return client.IgnoreNotFound(err)
	}
	vs := &snapshotv1.VolumeSnapshot{}
	compatClient := intctrlutil.VolumeSnapshotCompatClient{ReadonlyClient: cli, Ctx: ctx}
	if err := compatClient.Get(snapshotKey, vs); err != nil {
		return client.IgnoreNotFound(err)
	}
	vertex := &lifecycleVertex{obj: vs, oriObj: vs, action: actionPtr(DELETE)}
	dag.AddVertex(vertex)
	dag.Connect(root, vertex)
	reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "VolumeSnapshotDelete", "Delete volumeSnapshot/%s", snapshotKey.Name)
	return nil
}

// deleteBackup will delete all backup related resources created during horizontal scaling,
func deleteBackup(reqCtx intctrlutil.RequestCtx, cli roclient.ReadonlyClient,
	cluster *appsv1alpha1.Cluster, componentName, snapshotName string,
	dag *graph.DAG, root graph.Vertex) error {
	ml := getBackupMatchingLabels(cluster.Name, componentName)
	backupList := dataprotectionv1alpha1.BackupList{}
	if err := cli.List(reqCtx.Ctx, &backupList, ml); err != nil {
		return err
	}
	if len(backupList.Items) == 0 {
		return nil
	}
	for _, backup := range backupList.Items {
		vertex := &lifecycleVertex{obj: &backup, oriObj: &backup, action: actionPtr(DELETE)}
		dag.AddVertex(vertex)
		dag.Connect(root, vertex)
	}
	reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "BackupJobDelete", "Delete backupJob/%s", snapshotName)
	return nil
}

func getBackupMatchingLabels(clusterName string, componentName string) client.MatchingLabels {
	return client.MatchingLabels{
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
		constant.KBManagedByKey:         "cluster", // the resources are managed by which controller
	}
}

// check snapshot existence
func isVolumeSnapshotExists(cli roclient.ReadonlyClient,
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

func doSnapshot(cli roclient.ReadonlyClient,
	reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster,
	snapshotKey types.NamespacedName,
	stsObj *appsv1.StatefulSet,
	vcts []corev1.PersistentVolumeClaimTemplate,
	componentDef,
	backupPolicyTemplateName string,
	dag *graph.DAG,
	root graph.Vertex) error {

	ctx := reqCtx.Ctx

	backupPolicyTemplate := &appsv1alpha1.BackupPolicyTemplate{}
	if err := cli.Get(ctx, client.ObjectKey{Name: backupPolicyTemplateName}, backupPolicyTemplate); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if len(backupPolicyTemplate.Name) > 0 {
		// if there is backuppolicytemplate created by provider
		// create backupjob CR, will ignore error if already exists
		err := createBackup(reqCtx, cli, stsObj, componentDef, backupPolicyTemplateName, snapshotKey, cluster, dag, root)
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

		reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "VolumeSnapshotCreate", "Create volumesnapshot/%s", snapshotKey.Name)
	}
	return nil
}

// check snapshot ready to use
func isVolumeSnapshotReadyToUse(cli roclient.ReadonlyClient,
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

func checkedCreatePVCFromSnapshot(cli roclient.ReadonlyClient,
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
		compatClient := intctrlutil.VolumeSnapshotCompatClient{ReadonlyClient: cli, Ctx: ctx}
		if err := compatClient.List(&vsList, ml); err != nil {
			return err
		}
		if len(vsList.Items) == 0 {
			return fmt.Errorf("volumesnapshot not found in cluster %s component %s", cluster.Name, component.Name)
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
	cli roclient.ReadonlyClient,
	sts *appsv1.StatefulSet,
	componentDef,
	backupPolicyTemplateName string,
	backupKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster,
	dag *graph.DAG,
	root graph.Vertex) error {
	ctx := reqCtx.Ctx

	createBackup := func(backupPolicyName string) error {
		backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
		if err := cli.Get(ctx, client.ObjectKey{Namespace: backupKey.Namespace, Name: backupPolicyName}, backupPolicy); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		// wait for backupPolicy created
		if len(backupPolicy.Name) == 0 {
			return nil
		}
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
		vertex := &lifecycleVertex{obj: backup, action: actionPtr(CREATE)}
		dag.AddVertex(vertex)
		dag.Connect(root, vertex)
		return nil
	}
	backupPolicy, err := getBackupPolicyFromTemplate(reqCtx, cli, cluster, componentDef, backupPolicyTemplateName)
	if err != nil {
		return err
	}
	if backupPolicy == nil {
		return intctrlutil.NewNotFound("not found any backup policy created by %s", backupPolicyTemplateName)
	}
	if err = createBackup(backupPolicy.Name); err != nil {
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

var _ graph.Transformer = &StsHorizontalScalingTransformer{}
