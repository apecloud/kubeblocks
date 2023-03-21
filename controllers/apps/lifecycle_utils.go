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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	intctrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func mergeComponentsList(reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster,
	clusterDef *appsv1alpha1.ClusterDefinition,
	clusterCompDefList []appsv1alpha1.ClusterComponentDefinition,
	clusterCompSpecList []appsv1alpha1.ClusterComponentSpec) []component.SynthesizedComponent {
	var compList []component.SynthesizedComponent
	for _, compDef := range clusterCompDefList {
		for _, compSpec := range clusterCompSpecList {
			if compSpec.ComponentDefRef != compDef.Name {
				continue
			}
			comp := component.BuildComponent(reqCtx, *cluster, *clusterDef, compDef, compSpec)
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

func reconcileClusterWorkloads(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	clusterDef *appsv1alpha1.ClusterDefinition,
	clusterVer *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster) (shouldRequeue bool, err error) {
	clusterBackupResourceMap, err := getClusterBackupSourceMap(cluster)
	if err != nil {
		return false, err
	}
	resourcesQueue := make([]client.Object, 0, 3)
	task := intctrltypes.ReconcileTask{
		Cluster:           cluster,
		ClusterDefinition: clusterDef,
		ClusterVersion:    clusterVer,
		Resources:         &resourcesQueue,
	}
	clusterCompSpecMap := cluster.GetDefNameMappingComponents()
	clusterCompVerMap := clusterVer.GetDefNameMappingComponents()
	process1stComp := true

	prepareComp := func(synthesizedComp *component.SynthesizedComponent) error {
		iParams := task
		iParams.Component = synthesizedComp
		if process1stComp && len(synthesizedComp.Services) > 0 {
			if err := prepareConnCredential(reqCtx, cli, &iParams); err != nil {
				return err
			}
			process1stComp = false
		}

		// build info that needs to be restored from backup
		backupSourceName := clusterBackupResourceMap[synthesizedComp.Name]
		if len(backupSourceName) > 0 {
			if err := component.BuildRestoredInfo(reqCtx, cli, cluster.Namespace, synthesizedComp, backupSourceName); err != nil {
				return err
			}
		}
		return plan.PrepareComponentResources(reqCtx, cli, &iParams)
	}

	for _, c := range clusterDef.Spec.ComponentDefs {
		compDefName := c.Name
		compVer := clusterCompVerMap[compDefName]
		compSpecs := clusterCompSpecMap[compDefName]
		for _, compSpec := range compSpecs {
			if err := prepareComp(component.BuildComponent(reqCtx, *cluster, *clusterDef, c, compSpec, compVer)); err != nil {
				return false, err
			}
		}
	}

	return checkedCreateObjs(reqCtx, cli, &task)
}

func checkedCreateObjs(reqCtx intctrlutil.RequestCtx, cli client.Client, task *intctrltypes.ReconcileTask) (shouldRequeue bool, err error) {
	// TODO when deleting a component of the cluster, clean up the corresponding k8s resources.
	return createOrReplaceResources(reqCtx, cli, task.Cluster, task.ClusterDefinition, *task.Resources)
}

func prepareConnCredential(reqCtx intctrlutil.RequestCtx, cli client.Client, task *intctrltypes.ReconcileTask) error {
	secret, err := builder.BuildConnCredential(task.GetBuilderParams())
	if err != nil {
		return err
	}
	// must make sure secret resources are created before others
	task.InsertResource(secret)
	return nil
}

// mergeAnnotations keeps the original annotations.
// if annotations exist and are replaced, the Deployment/StatefulSet will be updated.
func mergeAnnotations(originalAnnotations, targetAnnotations map[string]string) map[string]string {
	if restartAnnotation, ok := originalAnnotations[constant.RestartAnnotationKey]; ok {
		if targetAnnotations == nil {
			targetAnnotations = map[string]string{}
		}
		targetAnnotations[constant.RestartAnnotationKey] = restartAnnotation
	}
	return targetAnnotations
}

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

func createOrReplaceResources(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterDef *appsv1alpha1.ClusterDefinition,
	objs []client.Object) (shouldRequeue bool, err error) {

	ctx := reqCtx.Ctx
	logger := reqCtx.Log
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()

	handleSts := func(stsProto *appsv1.StatefulSet) (shouldRequeue bool, err error) {
		key := client.ObjectKey{
			Namespace: stsProto.GetNamespace(),
			Name:      stsProto.GetName(),
		}
		stsObj := &appsv1.StatefulSet{}
		if err := cli.Get(ctx, key, stsObj); err != nil {
			return false, err
		}
		snapshotKey := types.NamespacedName{
			Namespace: stsObj.Namespace,
			Name:      stsObj.Name + "-scaling",
		}
		// find component of current statefulset
		componentName := stsObj.Labels[constant.KBAppComponentLabelKey]
		components := mergeComponentsList(reqCtx,
			cluster,
			clusterDef,
			clusterDef.Spec.ComponentDefs,
			cluster.Spec.ComponentSpecs)
		component := getComponent(components, componentName)
		if component == nil {
			reqCtx.Recorder.Eventf(cluster,
				corev1.EventTypeWarning,
				"HorizontalScaleFailed",
				"component %s not found",
				componentName)
			return false, nil
		}

		cleanCronJobs := func() error {
			for i := *stsObj.Spec.Replicas; i < *stsProto.Spec.Replicas; i++ {
				for _, vct := range stsObj.Spec.VolumeClaimTemplates {
					pvcKey := types.NamespacedName{
						Namespace: key.Namespace,
						Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
					}
					// delete deletion cronjob if exists
					if err := deleteDeletePVCCronJob(cli, ctx, pvcKey); err != nil {
						return err
					}
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
					pvcExists, err := isPVCExists(cli, ctx, pvcKey)
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

		scaleOut := func() (shouldRequeue bool, err error) {
			shouldRequeue = false
			if err = cleanCronJobs(); err != nil {
				return
			}
			allPVCsExist, err := checkAllPVCsExist()
			if err != nil {
				return
			}
			if allPVCsExist {
				return
			}
			// do backup according to component's horizontal scale policy
			return doBackup(reqCtx,
				cli,
				cluster,
				component,
				stsObj,
				stsProto,
				snapshotKey)
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
					if err := createDeletePVCCronJob(cli, reqCtx, pvcKey, stsObj, cluster); err != nil {
						return err
					}
				}
			}
			return nil
		}

		checkAllPVCBoundIfNeeded := func() (shouldRequeue bool, err error) {
			shouldRequeue = false
			err = nil
			if component.HorizontalScalePolicy == nil ||
				component.HorizontalScalePolicy.Type != appsv1alpha1.HScaleDataClonePolicyFromSnapshot ||
				!isSnapshotAvailable(cli, ctx) {
				return
			}
			allPVCBound, err := isAllPVCBound(cli, ctx, stsObj)
			if err != nil {
				return
			}
			if !allPVCBound {
				// requeue waiting pvc phase become bound
				return true, nil
			}
			// all pvc bounded, can do next step
			return
		}

		cleanBackupResourcesIfNeeded := func() error {
			if component.HorizontalScalePolicy == nil ||
				component.HorizontalScalePolicy.Type != appsv1alpha1.HScaleDataClonePolicyFromSnapshot ||
				!isSnapshotAvailable(cli, ctx) {
				return nil
			}
			// if all pvc bounded, clean backup resources
			return deleteSnapshot(cli, reqCtx, snapshotKey, cluster, component)
		}

		// when horizontal scaling up, sometimes db needs backup to sync data from master,
		// log is not reliable enough since it can be recycled
		if *stsObj.Spec.Replicas < *stsProto.Spec.Replicas {
			shouldRequeue, err = scaleOut()
			if err != nil {
				return false, err
			}
			if shouldRequeue {
				return true, nil
			}
		} else if *stsObj.Spec.Replicas > *stsProto.Spec.Replicas {
			if err := scaleIn(); err != nil {
				return false, err
			}
		}
		if *stsObj.Spec.Replicas != *stsProto.Spec.Replicas {
			reqCtx.Recorder.Eventf(cluster,
				corev1.EventTypeNormal,
				"HorizontalScale",
				"Start horizontal scale component %s from %d to %d",
				component.Name,
				*stsObj.Spec.Replicas,
				*stsProto.Spec.Replicas)
		}
		stsObjCopy := stsObj.DeepCopy()
		// keep the original template annotations.
		// if annotations exist and are replaced, the statefulSet will be updated.
		stsProto.Spec.Template.Annotations = mergeAnnotations(stsObj.Spec.Template.Annotations,
			stsProto.Spec.Template.Annotations)
		stsObj.Spec.Template = stsProto.Spec.Template
		stsObj.Spec.Replicas = stsProto.Spec.Replicas
		stsObj.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
		if err := cli.Update(ctx, stsObj); err != nil {
			return false, err
		}
		if !reflect.DeepEqual(&stsObjCopy.Spec, &stsObj.Spec) {
			// sync component phase
			syncComponentPhaseWhenSpecUpdating(cluster, componentName)
		}

		// check all pvc bound, requeue if not all ready
		shouldRequeue, err = checkAllPVCBoundIfNeeded()
		if err != nil {
			return false, err
		}
		if shouldRequeue {
			return true, err
		}
		// clean backup resources.
		// there will not be any backup resources other than scale out.
		if err := cleanBackupResourcesIfNeeded(); err != nil {
			return false, err
		}

		// check stsObj.Spec.VolumeClaimTemplates storage
		// request size and find attached PVC and patch request
		// storage size
		for _, vct := range stsObj.Spec.VolumeClaimTemplates {
			var vctProto *corev1.PersistentVolumeClaim
			for _, v := range stsProto.Spec.VolumeClaimTemplates {
				if v.Name == vct.Name {
					vctProto = &v
					break
				}
			}

			// REVIEW: how could VCT proto is nil?
			if vctProto == nil {
				continue
			}

			for i := *stsObj.Spec.Replicas - 1; i >= 0; i-- {
				pvc := &corev1.PersistentVolumeClaim{}
				pvcKey := types.NamespacedName{
					Namespace: key.Namespace,
					Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
				}
				var err error
				if err = cli.Get(ctx, pvcKey, pvc); err != nil {
					return false, err
				}
				if pvc.Spec.Resources.Requests[corev1.ResourceStorage] == vctProto.Spec.Resources.Requests[corev1.ResourceStorage] {
					continue
				}
				patch := client.MergeFrom(pvc.DeepCopy())
				pvc.Spec.Resources.Requests[corev1.ResourceStorage] = vctProto.Spec.Resources.Requests[corev1.ResourceStorage]
				if err := cli.Patch(ctx, pvc, patch); err != nil {
					return false, err
				}
			}
		}

		return false, nil
	}

	handleConfigMap := func(cm *corev1.ConfigMap) error {
		switch {
		case len(cm.Labels[constant.AppConfigTypeLabelKey]) > 0:
			// if configmap is env config, should update
			if err := cli.Update(ctx, cm); err != nil {
				return err
			}
		case len(cm.Labels[constant.CMConfigurationSpecProviderLabelKey]) > 0:
			// if tls settings updated, do Update
			// FIXME: very hacky way. should allow config to be updated
			oldCm := &corev1.ConfigMap{}
			if err := cli.Get(ctx, client.ObjectKeyFromObject(cm), oldCm); err != nil {
				return err
			}
			compName := cm.Labels[constant.KBAppComponentLabelKey]
			clusterDefComp := component.GetClusterDefCompByName(*clusterDef, *cluster, compName)
			if clusterDefComp == nil {
				return errors.New("clusterDefComp not found")
			}
			if plan.IsTLSSettingsUpdated(clusterDefComp.CharacterType, *oldCm, *cm) {
				if err := cli.Update(ctx, cm); err != nil {
					return err
				}
			}
		}
		return nil
	}

	handleDeploy := func(deployProto *appsv1.Deployment) error {
		key := client.ObjectKey{
			Namespace: deployProto.GetNamespace(),
			Name:      deployProto.GetName(),
		}
		deployObj := &appsv1.Deployment{}
		if err := cli.Get(ctx, key, deployObj); err != nil {
			return err
		}
		deployObjCopy := deployObj.DeepCopy()
		deployProto.Spec.Template.Annotations = mergeAnnotations(deployObj.Spec.Template.Annotations,
			deployProto.Spec.Template.Annotations)
		deployObj.Spec = deployProto.Spec
		if err := cli.Update(ctx, deployObj); err != nil {
			return err
		}
		if !reflect.DeepEqual(&deployObjCopy.Spec, &deployObj.Spec) {
			// sync component phase
			componentName := deployObj.Labels[constant.KBAppComponentLabelKey]
			syncComponentPhaseWhenSpecUpdating(cluster, componentName)
		}
		return nil
	}

	handleSvc := func(svcProto *corev1.Service) error {
		key := client.ObjectKey{
			Namespace: svcProto.GetNamespace(),
			Name:      svcProto.GetName(),
		}
		svcObj := &corev1.Service{}
		if err := cli.Get(ctx, key, svcObj); err != nil {
			return err
		}
		svcObj.Spec = svcProto.Spec
		svcObj.Annotations = mergeServiceAnnotations(svcObj.Annotations, svcProto.Annotations)
		if err := cli.Update(ctx, svcObj); err != nil {
			return err
		}
		return nil
	}

	handlePVC := func(pvcProto *corev1.PersistentVolumeClaim) error {
		key := client.ObjectKey{
			Namespace: pvcProto.GetNamespace(),
			Name:      pvcProto.GetName(),
		}
		pvcObj := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(ctx, key, pvcObj); err != nil {
			return err
		}
		if pvcObj.Spec.Resources.Requests[corev1.ResourceStorage] == pvcProto.Spec.Resources.Requests[corev1.ResourceStorage] {
			return nil
		}
		patch := client.MergeFrom(pvcObj.DeepCopy())
		pvcObj.Spec.Resources.Requests[corev1.ResourceStorage] = pvcProto.Spec.Resources.Requests[corev1.ResourceStorage]
		if err := cli.Patch(ctx, pvcObj, patch); err != nil {
			return err
		}
		return nil
	}

	cleanUselessServices := func() error {
		var (
			allSvcList = corev1.ServiceList{}
			ml         = getServiceMatchingLabels(cluster.Name, "")
		)
		if err = cli.List(reqCtx.Ctx, &allSvcList, ml); err != nil {
			return err
		}

		buildSvcName := func(clusterName, componentName, suffix string) string {
			parts := []string{clusterName, componentName}
			if suffix != "" {
				parts = append(parts, suffix)
			}
			return strings.Join(parts, "-")
		}

		expectSvcMap := make(map[string]*appsv1alpha1.ClusterComponentService)
		for _, comp := range cluster.Spec.ComponentSpecs {
			// default ClusterIP service, name should be consistent with name in service_template.cue
			expectSvcMap[buildSvcName(cluster.Name, comp.Name, "")] = nil

			// default headless service, name should be consistent with name in headless_service_template.cue
			expectSvcMap[buildSvcName(cluster.Name, comp.Name, "headless")] = nil

			// extra user exposed services, name should be consistent with name in service_template.cue
			for _, svc := range comp.Services {
				expectSvcMap[buildSvcName(cluster.Name, comp.Name, svc.Name)] = &svc
			}
		}

		for _, svc := range allSvcList.Items {
			if _, ok := expectSvcMap[svc.Name]; ok {
				continue
			}
			patch := client.MergeFrom(svc.DeepCopy())
			controllerutil.RemoveFinalizer(&svc, dbClusterFinalizerName)
			if err = cli.Patch(reqCtx.Ctx, &svc, patch); err != nil {
				return client.IgnoreNotFound(err)
			}
			if err = cli.Delete(reqCtx.Ctx, &svc); err != nil {
				return client.IgnoreNotFound(err)
			}
		}
		return nil
	}

	// why create tls certs here? or why not use prepare-checkedCreate pattern?
	// tls certs generation is very time-consuming, if using prepare-checkedCreate pattern,
	// we shall generate certs in every component Update which will slow down the cluster reconcile loop
	if err := plan.CreateOrCheckTLSCerts(reqCtx, cli, cluster, scheme, dbClusterFinalizerName); err != nil {
		return false, err
	}

	objsByKind := make(map[string][]client.Object)
	for _, obj := range objs {
		logger.V(1).Info("create or update", "objs", obj)
		if err := intctrlutil.SetOwnership(cluster, obj, scheme, dbClusterFinalizerName); err != nil {
			return false, err
		}
		kind := obj.GetObjectKind().GroupVersionKind().Kind
		objsByKind[kind] = append(objsByKind[kind], obj)

		if err := cli.Create(ctx, obj); err == nil {
			continue
		} else if !apierrors.IsAlreadyExists(err) {
			return false, err
		}

		// Secret kind objects should only be applied once
		if _, ok := obj.(*corev1.Secret); ok {
			continue
		}

		// ConfigMap kind objects should only be applied once
		//
		// The Config is not allowed to be modified.
		// Once ClusterDefinition provider adjusts the TemplateRef field of CusterDefinition,
		// or provider modifies the wrong config file, it may cause the application cluster may fail.
		if cm, ok := obj.(*corev1.ConfigMap); ok {
			if err := handleConfigMap(cm); err != nil {
				return false, err
			}
			continue
		}

		stsProto, ok := obj.(*appsv1.StatefulSet)
		if ok {
			requeue, err := handleSts(stsProto)
			if err != nil {
				return false, err
			}
			if requeue {
				shouldRequeue = true
			}
			continue
		}
		deployProto, ok := obj.(*appsv1.Deployment)
		if ok {
			if err := handleDeploy(deployProto); err != nil {
				return false, err
			}
			continue
		}
		svcProto, ok := obj.(*corev1.Service)
		if ok {
			if err := handleSvc(svcProto); err != nil {
				return false, err
			}
			continue
		}
		// do volume expansion when workload type is `replication`
		pvcProto, ok := obj.(*corev1.PersistentVolumeClaim)
		if ok {
			if err := handlePVC(pvcProto); err != nil {
				return false, err
			}
			continue
		}
	}

	if err = cleanUselessServices(); err != nil {
		return false, err
	}

	// stsList is used to handle statefulSets horizontal scaling when workloadType is replication
	var stsList []*appsv1.StatefulSet
	for _, obj := range objsByKind[constant.StatefulSetKind] {
		stsList = append(stsList, obj.(*appsv1.StatefulSet))
	}
	if err := replicationset.HandleReplicationSet(reqCtx.Ctx, cli, cluster, stsList); err != nil {
		return false, err
	}

	return shouldRequeue, nil
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
	snapshotName string,
	component *component.SynthesizedComponent) error {
	pvc, err := builder.BuildPVCFromSnapshot(sts, vct, pvcKey, snapshotName, component)
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
	component *component.SynthesizedComponent,
	vct corev1.PersistentVolumeClaimTemplate,
	stsObj *appsv1.StatefulSet) error {
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
		return createPVCFromSnapshot(ctx, cli, vct, stsObj, pvcKey, vsName, component)
	}
	return nil
}

func isAllPVCBound(cli client.Client,
	ctx context.Context,
	stsObj *appsv1.StatefulSet) (bool, error) {
	allPVCBound := true
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
	return allPVCBound, nil
}

func deleteSnapshot(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	snapshotKey types.NamespacedName,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) error {
	ctx := reqCtx.Ctx
	if err := deleteBackup(ctx, cli, cluster.Name, component.Name); err != nil {
		return client.IgnoreNotFound(err)
	}
	reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "BackupJobDelete", "Delete backupjob/%s", snapshotKey.Name)
	vs := snapshotv1.VolumeSnapshot{}
	if err := cli.Get(ctx, snapshotKey, &vs); err != nil {
		return client.IgnoreNotFound(err)
	}
	if err := cli.Delete(ctx, &vs); err != nil {
		return client.IgnoreNotFound(err)
	}
	reqCtx.Recorder.Eventf(cluster, corev1.EventTypeNormal, "VolumeSnapshotDelete", "Delete volumesnapshot/%s", snapshotKey.Name)
	return nil
}

func createDeletePVCCronJob(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	pvcKey types.NamespacedName,
	stsObj *appsv1.StatefulSet,
	cluster *appsv1alpha1.Cluster) error {
	ctx := reqCtx.Ctx
	now := time.Now()
	// hack: delete after 30 minutes
	t := now.Add(30 * 60 * time.Second)
	schedule := timeToSchedule(t)
	cronJob, err := builder.BuildCronJob(pvcKey, schedule, stsObj)
	if err != nil {
		return err
	}
	if err := cli.Create(ctx, cronJob); err != nil {
		return intctrlutil.IgnoreIsAlreadyExists(err)
	}
	reqCtx.Recorder.Eventf(cluster,
		corev1.EventTypeNormal,
		"CronJobCreate",
		"create cronjob to delete pvc/%s",
		pvcKey.Name)
	return nil
}

func deleteDeletePVCCronJob(cli client.Client,
	ctx context.Context,
	pvcKey types.NamespacedName) error {
	cronJobKey := pvcKey
	cronJobKey.Name = "delete-pvc-" + pvcKey.Name
	cronJob := batchv1.CronJob{}
	if err := cli.Get(ctx, cronJobKey, &cronJob); err != nil {
		return client.IgnoreNotFound(err)
	}
	if err := cli.Delete(ctx, &cronJob); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}

func timeToSchedule(t time.Time) string {
	utc := t.UTC()
	return fmt.Sprintf("%d %d %d %d *", utc.Minute(), utc.Hour(), utc.Day(), utc.Month())
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
				component,
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

func isPVCExists(cli client.Client,
	ctx context.Context,
	pvcKey types.NamespacedName) (bool, error) {
	pvc := corev1.PersistentVolumeClaim{}
	if err := cli.Get(ctx, pvcKey, &pvc); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	return true, nil
}

func getServiceMatchingLabels(clusterName string, componentName string) client.MatchingLabels {
	result := map[string]string{
		constant.AppInstanceLabelKey:  clusterName,
		constant.AppManagedByLabelKey: constant.AppName,
	}
	if componentName != "" {
		result[constant.KBAppComponentLabelKey] = componentName
	}
	return result
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

// getClusterBackupSourceMap gets the backup source map from cluster.annotations
func getClusterBackupSourceMap(cluster *appsv1alpha1.Cluster) (map[string]string, error) {
	compBackupMapString := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]
	if len(compBackupMapString) == 0 {
		return nil, nil
	}
	compBackupMap := map[string]string{}
	err := json.Unmarshal([]byte(compBackupMapString), &compBackupMap)
	return compBackupMap, err
}
