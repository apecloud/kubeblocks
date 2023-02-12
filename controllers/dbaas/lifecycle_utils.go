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

package dbaas

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/exp/maps"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/replicationset"
	componentutil "github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	cfgutil "github.com/apecloud/kubeblocks/controllers/dbaas/configuration"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/configmap"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type createParams struct {
	clusterDefinition *dbaasv1alpha1.ClusterDefinition
	clusterVersion    *dbaasv1alpha1.ClusterVersion
	cluster           *dbaasv1alpha1.Cluster
	component         *component.Component
	applyObjs         *[]client.Object
	cacheCtx          *map[string]interface{}
}

func (params createParams) toBuilderParams() builder.BuilderParams {
	return builder.BuilderParams{
		ClusterDefinition: params.clusterDefinition,
		ClusterVersion:    params.clusterVersion,
		Cluster:           params.cluster,
		Component:         params.component,
	}
}

func mergeComponentsList(reqCtx intctrlutil.RequestCtx,
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition,
	clusterDefCompList []dbaasv1alpha1.ClusterDefinitionComponent,
	clusterCompList []dbaasv1alpha1.ClusterComponent) []component.Component {
	var compList []component.Component
	for _, clusterDefComp := range clusterDefCompList {
		for _, clusterComp := range clusterCompList {
			if clusterComp.Type != clusterDefComp.TypeName {
				continue
			}
			comp := component.MergeComponents(reqCtx, cluster, clusterDef, &clusterDefComp, nil, &clusterComp)
			compList = append(compList, *comp)
		}
	}
	return compList
}

func getComponent(componentList []component.Component, name string) *component.Component {
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
	clusterDefinition *dbaasv1alpha1.ClusterDefinition,
	clusterVersion *dbaasv1alpha1.ClusterVersion,
	cluster *dbaasv1alpha1.Cluster) (shouldRequeue bool, err error) {

	applyObjs := make([]client.Object, 0, 3)
	cacheCtx := map[string]interface{}{}
	params := createParams{
		cluster:           cluster,
		clusterDefinition: clusterDefinition,
		applyObjs:         &applyObjs,
		cacheCtx:          &cacheCtx,
		clusterVersion:    clusterVersion,
	}
	if err := prepareSecretObjs(reqCtx, cli, &params); err != nil {
		return false, err
	}

	clusterDefComps := clusterDefinition.Spec.Components
	clusterCompMap := cluster.GetTypeMappingComponents()

	// add default component if unspecified in Cluster.spec.components
	for _, c := range clusterDefComps {
		if c.DefaultReplicas <= 0 {
			continue
		}
		if _, ok := clusterCompMap[c.TypeName]; ok {
			continue
		}
		r := c.DefaultReplicas
		cluster.Spec.Components = append(cluster.Spec.Components, dbaasv1alpha1.ClusterComponent{
			Name:     c.TypeName,
			Type:     c.TypeName,
			Replicas: &r,
		})
	}

	clusterCompMap = cluster.GetTypeMappingComponents()
	clusterVersionCompMap := clusterVersion.GetTypeMappingComponents()

	prepareComp := func(component *component.Component) error {
		iParams := params
		iParams.component = component
		return prepareComponentObjs(reqCtx, cli, &iParams)
	}

	for _, c := range clusterDefComps {
		typeName := c.TypeName
		clusterVersionComp := clusterVersionCompMap[typeName]
		clusterComps := clusterCompMap[typeName]
		for _, clusterComp := range clusterComps {
			if err := prepareComp(component.MergeComponents(reqCtx, cluster, clusterDefinition, &c, clusterVersionComp, &clusterComp)); err != nil {
				return false, err
			}
		}
	}

	return checkedCreateObjs(reqCtx, cli, &params)
}

func checkedCreateObjs(reqCtx intctrlutil.RequestCtx, cli client.Client, obj interface{}) (shouldRequeue bool, err error) {
	params, ok := obj.(*createParams)
	if !ok {
		return false, fmt.Errorf("invalid arg")
	}
	// TODO when deleting a component of the cluster, clean up the corresponding k8s resources.
	return createOrReplaceResources(reqCtx, cli, params.cluster, params.clusterDefinition, *params.applyObjs)
}

func prepareSecretObjs(reqCtx intctrlutil.RequestCtx, cli client.Client, obj interface{}) error {
	params, ok := obj.(*createParams)
	if !ok {
		return fmt.Errorf("invalid arg")
	}

	secret, err := builder.BuildConnCredential(params.toBuilderParams())
	if err != nil {
		return err
	}
	// must make sure secret resources are created before others
	*params.applyObjs = append(*params.applyObjs, secret)
	return nil
}

func existsPDBSpec(pdbSpec *policyv1.PodDisruptionBudgetSpec) bool {
	if pdbSpec == nil {
		return false
	}
	if pdbSpec.MinAvailable == nil && pdbSpec.MaxUnavailable == nil {
		return false
	}
	return true
}

// needBuildPDB check whether the PodDisruptionBudget needs to be built
func needBuildPDB(params *createParams) bool {
	if params.component.ComponentType == dbaasv1alpha1.Consensus {
		// if MinReplicas is non-zero, build pdb
		// TODO: add ut
		return params.component.MinReplicas > 0
	}
	return existsPDBSpec(params.component.PodDisruptionBudgetSpec)
}

// prepareComponentObjs generate all necessary sub-resources objects used in component,
// like Secret, ConfigMap, Service, StatefulSet, Deployment, Volume, PodDisruptionBudget etc.
// Generated resources are cached in (obj.(*createParams)).applyObjs.
func prepareComponentObjs(reqCtx intctrlutil.RequestCtx, cli client.Client, obj interface{}) error {
	params, ok := obj.(*createParams)
	if !ok {
		return fmt.Errorf("invalid arg")
	}

	workloadProcessor := func(customSetup func(*corev1.ConfigMap) (client.Object, error)) error {
		envConfig, err := builder.BuildEnvConfig(params.toBuilderParams())
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, envConfig)

		workload, err := customSetup(envConfig)
		if err != nil {
			return err
		}

		defer func() {
			// workload object should be appended last
			*params.applyObjs = append(*params.applyObjs, workload)
		}()

		svc, err := builder.BuildSvc(params.toBuilderParams(), true)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, svc)

		var podSpec *corev1.PodSpec
		sts, ok := workload.(*appsv1.StatefulSet)
		if ok {
			podSpec = &sts.Spec.Template.Spec
		} else {
			deploy, ok := workload.(*appsv1.Deployment)
			if ok {
				podSpec = &deploy.Spec.Template.Spec
			}
		}
		if podSpec == nil {
			return nil
		}

		defer func() {
			for _, cc := range []*[]corev1.Container{
				&podSpec.Containers,
				&podSpec.InitContainers,
			} {
				volumes := podSpec.Volumes
				for _, c := range *cc {
					for _, v := range c.VolumeMounts {
						// if persistence is not found, add emptyDir pod.spec.volumes[]
						volumes, _ = intctrlutil.CheckAndUpdateVolume(volumes, v.Name, func(volumeName string) corev1.Volume {
							return corev1.Volume{
								Name: v.Name,
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							}
						}, nil)
					}
				}
				podSpec.Volumes = volumes
			}
		}()

		// render config template
		configs, err := buildCfg(*params, workload, podSpec, reqCtx.Ctx, cli)
		if err != nil {
			return err
		}
		if configs != nil {
			*params.applyObjs = append(*params.applyObjs, configs...)
		}
		// end render config
		return nil
	}

	switch params.component.ComponentType {
	case dbaasv1alpha1.Stateless:
		if err := workloadProcessor(
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return builder.BuildDeploy(reqCtx, params.toBuilderParams())
			}); err != nil {
			return err
		}
	case dbaasv1alpha1.Stateful:
		if err := workloadProcessor(
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return builder.BuildSts(reqCtx, params.toBuilderParams(), envConfig.Name)
			}); err != nil {
			return err
		}
	case dbaasv1alpha1.Consensus:
		if err := workloadProcessor(
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return buildConsensusSet(reqCtx, *params, envConfig.Name)
			}); err != nil {
			return err
		}
	case dbaasv1alpha1.Replication:
		// get the maximum value of params.component.Replicas and the number of existing statefulsets under the current component,
		// then construct statefulsets for creating replicationSet or handling horizontal scaling of the replicationSet.
		var existStsList = &appsv1.StatefulSetList{}
		if err := componentutil.GetObjectListByComponentName(reqCtx.Ctx, cli, params.cluster, existStsList, params.component.Name); err != nil {
			return err
		}
		replicaCount := math.Max(float64(len(existStsList.Items)), float64(params.component.Replicas))

		for index := int32(0); index < int32(replicaCount); index++ {
			if err := workloadProcessor(
				func(envConfig *corev1.ConfigMap) (client.Object, error) {
					return buildReplicationSet(reqCtx, *params, envConfig.Name, index)
				}); err != nil {
				return err
			}
		}
	}

	if needBuildPDB(params) {
		pdb, err := builder.BuildPDB(params.toBuilderParams())
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, pdb)
	}

	if params.component.Service != nil && len(params.component.Service.Ports) > 0 {
		svc, err := builder.BuildSvc(params.toBuilderParams(), false)
		if err != nil {
			return err
		}
		if params.component.ComponentType == dbaasv1alpha1.Consensus {
			addLeaderSelectorLabels(svc, params.component)
		}
		if params.component.ComponentType == dbaasv1alpha1.Replication {
			svc.Spec.Selector[intctrlutil.RoleLabelKey] = string(replicationset.Primary)
		}
		*params.applyObjs = append(*params.applyObjs, svc)
	}

	return nil
}

// TODO multi roles with same accessMode support
func addLeaderSelectorLabels(service *corev1.Service, component *component.Component) {
	leader := component.ConsensusSpec.Leader
	if len(leader.Name) > 0 {
		service.Spec.Selector[intctrlutil.RoleLabelKey] = leader.Name
	}
}

// mergeAnnotations keeps the original annotations.
// if annotations exist and are replaced, the Deployment/StatefulSet will be updated.
func mergeAnnotations(originalAnnotations, targetAnnotations map[string]string) map[string]string {
	if restartAnnotation, ok := originalAnnotations[intctrlutil.RestartAnnotationKey]; ok {
		if targetAnnotations == nil {
			targetAnnotations = map[string]string{}
		}
		targetAnnotations[intctrlutil.RestartAnnotationKey] = restartAnnotation
	}
	return targetAnnotations
}

func createOrReplaceResources(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition,
	objs []client.Object) (shouldRequeue bool, err error) {

	ctx := reqCtx.Ctx
	logger := reqCtx.Log
	scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()

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
		componentName := stsObj.Labels[intctrlutil.AppComponentLabelKey]
		components := mergeComponentsList(reqCtx,
			cluster,
			clusterDef,
			clusterDef.Spec.Components,
			cluster.Spec.Components)
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
				component.HorizontalScalePolicy.Type != dbaasv1alpha1.HScaleDataClonePolicyFromSnapshot ||
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
				component.HorizontalScalePolicy.Type != dbaasv1alpha1.HScaleDataClonePolicyFromSnapshot ||
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
		// if configmap is env config, should update
		if len(cm.Labels[intctrlutil.AppConfigTypeLabelKey]) > 0 {
			if err := cli.Update(ctx, cm); err != nil {
				return err
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
			componentName := deployObj.Labels[intctrlutil.AppComponentLabelKey]
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
		if err := cli.Update(ctx, svcObj); err != nil {
			return err
		}
		return nil
	}

	var stsList []*appsv1.StatefulSet
	for _, obj := range objs {
		logger.Info("create or update", "objs", obj)
		if err := controllerutil.SetOwnerReference(cluster, obj, scheme); err != nil {
			return false, err
		}
		if !controllerutil.ContainsFinalizer(obj, dbClusterFinalizerName) {
			// pvc objects do not need to add finalizer
			_, ok := obj.(*corev1.PersistentVolumeClaim)
			if !ok {
				controllerutil.AddFinalizer(obj, dbClusterFinalizerName)
			}
		}
		// appendToStsList is used to handle statefulSets horizontal scaling when componentType is replication
		appendToStsList := func(stsList []*appsv1.StatefulSet) []*appsv1.StatefulSet {
			stsObj, ok := obj.(*appsv1.StatefulSet)
			if ok {
				stsList = append(stsList, stsObj)
			}
			return stsList
		}

		err := cli.Create(ctx, obj)
		if err == nil {
			stsList = appendToStsList(stsList)
			continue
		}
		if !apierrors.IsAlreadyExists(err) {
			return false, err
		} else {
			stsList = appendToStsList(stsList)
		}

		// Secret kind objects should only be applied once
		if _, ok := obj.(*corev1.Secret); ok {
			continue
		}

		// ConfigMap kind objects should only be applied once
		//
		// The Config is not allowed to be modified.
		// Once ClusterDefinition provider adjusts the ConfigTemplateRef field of CusterDefinition,
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
	}

	if err := replicationset.HandleReplicationSet(reqCtx, cli, cluster, stsList); err != nil {
		return false, err
	}

	return shouldRequeue, nil
}

// buildPersistentVolumeClaimLabels builds a pvc name label, and synchronize the labels on the sts to the pvc labels.
func buildPersistentVolumeClaimLabels(sts *appsv1.StatefulSet, pvc *corev1.PersistentVolumeClaim) {
	if pvc.Labels == nil {
		pvc.Labels = make(map[string]string)
	}
	pvc.Labels[intctrlutil.VolumeClaimTemplateNameLabelKey] = pvc.Name
	for k, v := range sts.Labels {
		if _, ok := pvc.Labels[k]; !ok {
			pvc.Labels[k] = v
		}
	}
}

// buildReplicationSet builds a replication component on statefulSet.
func buildReplicationSet(reqCtx intctrlutil.RequestCtx,
	params createParams,
	envConfigName string,
	stsIndex int32) (*appsv1.StatefulSet, error) {
	sts, err := builder.BuildSts(reqCtx, params.toBuilderParams(), envConfigName)
	if err != nil {
		return nil, err
	}
	// inject replicationSet pod env and role label.
	if sts, err = injectReplicationSetPodEnvAndLabel(params, sts, stsIndex); err != nil {
		return nil, err
	}
	// sts.Name rename and add role label.
	sts.ObjectMeta.Name = fmt.Sprintf("%s-%d", sts.ObjectMeta.Name, stsIndex)
	sts.Labels[intctrlutil.RoleLabelKey] = string(replicationset.Secondary)
	if stsIndex == *params.component.PrimaryIndex {
		sts.Labels[intctrlutil.RoleLabelKey] = string(replicationset.Primary)
	}
	sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
	// build replicationSet persistentVolumeClaim manually
	if err := buildReplicationSetPVC(params, sts); err != nil {
		return sts, err
	}
	return sts, nil
}

// buildReplicationSetPVC builds replicationSet persistentVolumeClaim manually,
// replicationSet does not manage pvc through volumeClaimTemplate defined on statefulSet,
// the purpose is convenient to convert between componentTypes in the future (TODO).
func buildReplicationSetPVC(params createParams, sts *appsv1.StatefulSet) error {
	// generate persistentVolumeClaim objects used by replicationSet's pod from component.VolumeClaimTemplates
	// TODO: The pvc objects involved in all processes in the KubeBlocks will be reconstructed into a unified generation method
	pvcMap := replicationset.GeneratePVCFromVolumeClaimTemplates(sts, params.component.VolumeClaimTemplates)
	for _, pvc := range pvcMap {
		buildPersistentVolumeClaimLabels(sts, pvc)
		*params.applyObjs = append(*params.applyObjs, pvc)
	}

	// binding persistentVolumeClaim to podSpec.Volumes
	podSpec := &sts.Spec.Template.Spec
	if podSpec == nil {
		return nil
	}
	podVolumes := podSpec.Volumes
	for _, pvc := range pvcMap {
		podVolumes, _ = intctrlutil.CheckAndUpdateVolume(podVolumes, pvc.Name, func(volumeName string) corev1.Volume {
			return corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
					},
				},
			}
		}, nil)
	}
	podSpec.Volumes = podVolumes
	return nil
}

func injectReplicationSetPodEnvAndLabel(params createParams, sts *appsv1.StatefulSet, index int32) (*appsv1.StatefulSet, error) {
	svcName := strings.Join([]string{params.cluster.Name, params.component.Name, "headless"}, "-")
	for i := range sts.Spec.Template.Spec.Containers {
		c := &sts.Spec.Template.Spec.Containers[i]
		c.Env = append(c.Env, corev1.EnvVar{
			Name:      component.KBPrefix + "_PRIMARY_POD_NAME",
			Value:     fmt.Sprintf("%s-%d-%d.%s", sts.Name, *params.component.PrimaryIndex, 0, svcName),
			ValueFrom: nil,
		})
	}
	if index != *params.component.PrimaryIndex {
		sts.Spec.Template.Labels[intctrlutil.RoleLabelKey] = string(replicationset.Secondary)
	} else {
		sts.Spec.Template.Labels[intctrlutil.RoleLabelKey] = string(replicationset.Primary)
	}
	return sts, nil
}

// buildConsensusSet build on a stateful set
func buildConsensusSet(reqCtx intctrlutil.RequestCtx,
	params createParams,
	envConfigName string) (*appsv1.StatefulSet, error) {
	sts, err := builder.BuildSts(reqCtx, params.toBuilderParams(), envConfigName)
	if err != nil {
		return sts, err
	}

	sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
	return sts, err
}

// buildCfg generate volumes for PodTemplate, volumeMount for container, and configmap for config files
func buildCfg(params createParams,
	obj client.Object,
	podSpec *corev1.PodSpec,
	ctx context.Context,
	cli client.Client) ([]client.Object, error) {
	// Need to merge configTemplateRef of ClusterVersion.Components[*].ConfigTemplateRefs and
	// ClusterDefinition.Components[*].ConfigTemplateRefs
	tpls := params.component.ConfigTemplates
	if len(tpls) == 0 {
		return nil, nil
	}

	clusterName := params.cluster.Name
	namespaceName := params.cluster.Namespace

	// New ConfigTemplateBuilder
	cfgTemplateBuilder := newCfgTemplateBuilder(clusterName, namespaceName, params.cluster, params.clusterVersion, ctx, cli)
	// Prepare built-in objects and built-in functions
	if err := cfgTemplateBuilder.injectBuiltInObjectsAndFunctions(podSpec, tpls, params.component); err != nil {
		return nil, err
	}

	configs := make([]client.Object, 0, len(tpls))
	volumes := make(map[string]dbaasv1alpha1.ConfigTemplate, len(tpls))
	// TODO Support Update ClusterVersionRef of Cluster
	scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()
	cfgLables := make(map[string]string, len(tpls))
	for _, tpl := range tpls {
		// Check config cm already exists
		cmName := cfgcore.GetInstanceCMName(obj, &tpl)
		volumes[cmName] = tpl
		// Configuration.kubeblocks.io/cfg-tpl-${ctpl-name}: ${cm-instance-name}
		cfgLables[cfgcore.GenerateTPLUniqLabelKeyWithConfig(tpl.Name)] = cmName
		isExist, err := isAlreadyExists(cmName, params.cluster.Namespace, ctx, cli)
		if err != nil {
			return nil, err
		}
		if isExist {
			continue
		}

		// Generate ConfigMap objects for config files
		cm, err := generateConfigMapFromTpl(cfgTemplateBuilder, cmName, tpl, params, ctx, cli)
		if err != nil {
			return nil, err
		}

		// The owner of the configmap object is a cluster of users,
		// in order to manage the life cycle of configmap
		if err := controllerutil.SetOwnerReference(params.cluster, cm, scheme); err != nil {
			return nil, err
		}
		configs = append(configs, cm)
	}
	if sts, ok := obj.(*appsv1.StatefulSet); ok {
		updateStatefulLabelsWithTemplate(sts, cfgLables)
	}

	// Generate Pod Volumes for ConfigMap objects
	if err := checkAndUpdatePodVolumes(podSpec, volumes); err != nil {
		return nil, cfgcore.WrapError(err, "failed to generate pod volume")
	}

	if err := updateConfigurationManagerWithComponent(params, podSpec, tpls, ctx, cli); err != nil {
		return nil, cfgcore.WrapError(err, "failed to generate sidecar for configmap's reloader")
	}

	return configs, nil
}

func updateConfigurationManagerWithComponent(
	params createParams,
	podSpec *corev1.PodSpec,
	cfgTemplates []dbaasv1alpha1.ConfigTemplate,
	ctx context.Context,
	cli client.Client) error {
	var (
		firstCfg        = 0
		usingContainers []*corev1.Container

		defaultVarRunVolumePath = "/var/run"
		criEndpointVolumeName   = "cri-runtime-endpoint"
		// criRuntimeEndpoint      = viper.GetString(cfgcore.CRIRuntimeEndpoint)
		// criType                 = viper.GetString(cfgcore.ConfigCRIType)
	)

	reloadOptions, err := cfgutil.GetReloadOptions(cli, ctx, cfgTemplates)
	if err != nil {
		return err
	}
	if reloadOptions == nil {
		return nil
	}
	if reloadOptions.UnixSignalTrigger == nil {
		// TODO support other reload type
		log.Log.Info("only unix signal type is supported!")
		return nil
	}

	// Ignore useless configtemplate
	for i, tpl := range cfgTemplates {
		usingContainers = intctrlutil.GetPodContainerWithVolumeMount(podSpec, tpl.VolumeName)
		if len(usingContainers) > 0 {
			firstCfg = i
			break
		}
	}

	// No container using any config template
	if len(usingContainers) == 0 {
		log.Log.Info(fmt.Sprintf("tpl config is not used by any container, and pass. tpl configs: %v", cfgTemplates))
		return nil
	}

	// Find first container using
	// Find out which configurations are used by the container
	volumeDirs := make([]corev1.VolumeMount, 0, len(cfgTemplates)+1)
	container := usingContainers[0]
	for i := firstCfg; i < len(cfgTemplates); i++ {
		tpl := cfgTemplates[i]
		// Ignore config template, e.g scripts configmap
		if !cfgutil.NeedReloadVolume(tpl) {
			continue
		}
		volume := intctrlutil.GetVolumeMountByVolume(container, tpl.VolumeName)
		if volume != nil {
			volumeDirs = append(volumeDirs, *volume)
		}
	}

	// If you do not need to watch any configmap volume
	if len(volumeDirs) == 0 {
		log.Log.Info(fmt.Sprintf("volume for configmap is not used by any container, and pass. cm name: %v", cfgTemplates[firstCfg]))
		return nil
	}

	unixSignalOption := reloadOptions.UnixSignalTrigger
	configManagerArgs := cfgcm.BuildSignalArgs(*unixSignalOption, volumeDirs)

	mountPath := defaultVarRunVolumePath
	managerSidecar := &cfgcm.ConfigManagerSidecar{
		ManagerName: cfgcore.ConfigSidecarName,
		Image:       viper.GetString(cfgcore.ConfigSidecarIMAGE),
		Args:        configManagerArgs,
		// add cri sock path
		Volumes: append(volumeDirs, corev1.VolumeMount{
			Name:      criEndpointVolumeName,
			MountPath: mountPath,
		}),
	}

	if container, err = builder.BuildCfgManagerContainer(managerSidecar); err != nil {
		return err
	}

	podVolumes := podSpec.Volumes
	podVolumes, _ = intctrlutil.CheckAndUpdateVolume(podVolumes, criEndpointVolumeName, func(volumeName string) corev1.Volume {
		return corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: mountPath,
				},
			},
		}
	}, nil)
	podSpec.Volumes = podVolumes

	// Add sidecar to podTemplate
	podSpec.Containers = append(podSpec.Containers, *container)

	// This sidecar container will be able to view and signal processes from other containers
	podSpec.ShareProcessNamespace = func() *bool { b := true; return &b }()
	return nil
}

func updateStatefulLabelsWithTemplate(sts *appsv1.StatefulSet, allLabels map[string]string) {
	// full configmap upgrade
	existLabels := make(map[string]string)
	for key, val := range sts.Labels {
		if strings.HasPrefix(key, cfgcore.ConfigurationTplLabelPrefixKey) {
			existLabels[key] = val
		}
	}

	// delete not exist configmap label
	deletedLabels := cfgcore.MapKeyDifference(existLabels, allLabels)
	for l := range deletedLabels.Iter() {
		delete(sts.Labels, l)
	}

	for key, val := range allLabels {
		sts.Labels[key] = val
	}
}

func checkAndUpdatePodVolumes(podSpec *corev1.PodSpec, volumes map[string]dbaasv1alpha1.ConfigTemplate) error {
	var (
		err        error
		podVolumes = podSpec.Volumes
	)
	// sort the volumes
	volumeKeys := maps.Keys(volumes)
	sort.Strings(volumeKeys)
	// Update PodTemplate Volumes
	for _, cmName := range volumeKeys {
		tpl := volumes[cmName]
		if podVolumes, err = intctrlutil.CheckAndUpdateVolume(podVolumes, tpl.VolumeName, func(volumeName string) corev1.Volume {
			return corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
						DefaultMode:          tpl.DefaultMode,
					},
				},
			}
		}, func(volume *corev1.Volume) error {
			configMap := volume.ConfigMap
			if configMap == nil {
				return fmt.Errorf("mount volume[%s] type require ConfigMap: [%+v]", volume.Name, volume)
			}
			configMap.Name = cmName
			return nil
		}); err != nil {
			return err
		}
	}
	podSpec.Volumes = podVolumes
	return nil
}

func isAlreadyExists(cmName string, namespace string, ctx context.Context, cli client.Client) (bool, error) {
	cmKey := client.ObjectKey{
		Name:      cmName,
		Namespace: namespace,
	}

	cmObj := &corev1.ConfigMap{}
	cmErr := cli.Get(ctx, cmKey, cmObj)
	if cmErr != nil && apierrors.IsNotFound(cmErr) {
		// Config is not exists
		return false, nil
	} else if cmErr != nil {
		// An unexpected error occurs
		// TODO process unexpected error
		return true, cmErr
	}

	return true, nil
}

// generateConfigMapFromTpl render config file by config template provided by provider.
func generateConfigMapFromTpl(tplBuilder *configTemplateBuilder,
	cmName string,
	tplCfg dbaasv1alpha1.ConfigTemplate,
	params createParams,
	ctx context.Context,
	cli client.Client) (*corev1.ConfigMap, error) {
	// Render config template by TplEngine
	// The template namespace must be the same as the ClusterDefinition namespace
	configs, err := processConfigMapTemplate(tplBuilder, tplCfg, ctx, cli)
	if err != nil {
		return nil, err
	}

	// Using ConfigMap cue template render to configmap of config
	return builder.BuildConfigMapWithTemplate(configs, params.toBuilderParams(), cmName, tplCfg)
}

// processConfigMapTemplate Render config file using template engine
func processConfigMapTemplate(
	tplBuilder *configTemplateBuilder,
	tplCfg dbaasv1alpha1.ConfigTemplate,
	ctx context.Context,
	cli client.Client) (map[string]string, error) {
	cfgTemplate := &dbaasv1alpha1.ConfigConstraint{}
	if len(tplCfg.ConfigConstraintRef) > 0 {
		if err := cli.Get(ctx, client.ObjectKey{
			Namespace: "",
			Name:      tplCfg.ConfigConstraintRef,
		}, cfgTemplate); err != nil {
			return nil, cfgcore.WrapError(err, "failed to get ConfigConstraint, key[%v]", tplCfg)
		}
	}

	// NOTE: not require checker configuration template status
	cfgChecker := cfgcore.NewConfigValidator(&cfgTemplate.Spec)
	cmObj := &corev1.ConfigMap{}
	//  Require template configmap exist
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: tplCfg.Namespace,
		Name:      tplCfg.ConfigTplRef,
	}, cmObj); err != nil {
		return nil, err
	}

	if len(cmObj.Data) == 0 {
		return map[string]string{}, nil
	}

	tplBuilder.setTplName(tplCfg.ConfigTplRef)
	renderedCfg, err := tplBuilder.render(cmObj.Data)
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to render configmap")
	}

	// NOTE: It is necessary to verify the correctness of the data
	if err := cfgChecker.Validate(renderedCfg); err != nil {
		return nil, cfgcore.WrapError(err, "failed to validate configmap")
	}

	return renderedCfg, nil
}

// createBackup create backup resources required to do backup,
func createBackup(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	sts *appsv1.StatefulSet,
	backupPolicyTemplate *dataprotectionv1alpha1.BackupPolicyTemplate,
	backupKey types.NamespacedName,
	cluster *dbaasv1alpha1.Cluster) error {
	ctx := reqCtx.Ctx

	createBackupPolicy := func() (backupPolicyName string, err error) {
		backupPolicyName = ""
		backupPolicyList := dataprotectionv1alpha1.BackupPolicyList{}
		ml := getBackupMatchingLabels(cluster.Name, sts.Labels[intctrlutil.AppComponentLabelKey])
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
		ml := getBackupMatchingLabels(cluster.Name, sts.Labels[intctrlutil.AppComponentLabelKey])
		if err := cli.List(ctx, &backupList, ml); err != nil {
			return err
		}
		if len(backupList.Items) > 0 {
			return nil
		}
		backup, err := builder.BuildBackup(sts, backupPolicyName, backupKey)
		if err != nil {
			return err
		}
		scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()
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
	vct corev1.PersistentVolumeClaim,
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
	cluster *dbaasv1alpha1.Cluster,
	component *component.Component) (bool, error) {
	ml := getBackupMatchingLabels(cluster.Name, component.Name)
	vsList := snapshotv1.VolumeSnapshotList{}
	if err := cli.List(ctx, &vsList, ml); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	return len(vsList.Items) > 0, nil
}

// check snapshot ready to use
func isVolumeSnapshotReadyToUse(cli client.Client,
	ctx context.Context,
	cluster *dbaasv1alpha1.Cluster,
	component *component.Component) (bool, error) {
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
	cluster *dbaasv1alpha1.Cluster,
	snapshotKey types.NamespacedName,
	stsObj *appsv1.StatefulSet,
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
		pvcName := strings.Join([]string{stsObj.Spec.VolumeClaimTemplates[0].Name, stsObj.Name, "0"}, "-")
		snapshot, err := builder.BuildVolumeSnapshot(snapshotKey, pvcName, stsObj)
		if err != nil {
			return err
		}
		if err := cli.Create(ctx, snapshot); err != nil {
			return intctrlutil.IgnoreIsAlreadyExists(err)
		}
		scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()
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
	cluster *dbaasv1alpha1.Cluster,
	componentName string,
	vct corev1.PersistentVolumeClaim,
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
			return errors.Errorf("volumesnapshot not found in cluster %s component %s", cluster.Name, component.Name)
		}
		return createPVCFromSnapshot(ctx, cli, vct, stsObj, pvcKey, vsList.Items[0].Name)
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
	cluster *dbaasv1alpha1.Cluster,
	component *component.Component) error {
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
	cluster *dbaasv1alpha1.Cluster) error {
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
	cronJob := v1.CronJob{}
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
	cluster *dbaasv1alpha1.Cluster,
	component *component.Component,
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
	case dbaasv1alpha1.HScaleDataClonePolicyFromBackup:
		// TODO: db core not support yet, leave it empty
		reqCtx.Recorder.Eventf(cluster,
			corev1.EventTypeWarning,
			"HorizontalScaleFailed",
			"scale with backup tool not support yet")
	// use volume snapshot
	case dbaasv1alpha1.HScaleDataClonePolicyFromSnapshot:
		if !isSnapshotAvailable(cli, ctx) || len(stsObj.Spec.VolumeClaimTemplates) == 0 {
			reqCtx.Recorder.Eventf(cluster,
				corev1.EventTypeWarning,
				"HorizontalScaleFailed",
				"volume snapshot not support")
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
			vct := stsObj.Spec.VolumeClaimTemplates[0]
			for _, tmpVct := range stsObj.Spec.VolumeClaimTemplates {
				if tmpVct.Name == component.HorizontalScalePolicy.VolumeMountsName {
					vct = tmpVct
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
				stsObj); err != nil {
				reqCtx.Log.Error(err, "checkedCreatePVCFromSnapshot failed")
				return shouldRequeue, err
			}
		}
	// do nothing
	case dbaasv1alpha1.HScaleDataClonePolicyNone:
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

func getBackupMatchingLabels(clusterName string, componentName string) client.MatchingLabels {
	return client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey:  clusterName,
		intctrlutil.AppComponentLabelKey: componentName,
		intctrlutil.AppCreatedByLabelKey: intctrlutil.AppName,
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
