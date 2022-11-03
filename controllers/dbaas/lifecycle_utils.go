/*
Copyright ApeCloud Inc.

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
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/leaanthony/debme"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type createParams struct {
	clusterDefinition *dbaasv1alpha1.ClusterDefinition
	appVersion        *dbaasv1alpha1.AppVersion
	cluster           *dbaasv1alpha1.Cluster
	component         *Component
	applyObjs         *[]client.Object
	cacheCtx          *map[string]interface{}
}

const (
	dbaasPrefix = "KB"
)

var (
	//go:embed cue/*
	cueTemplates embed.FS
)

func init() {
	viper.SetDefault(cmNamespaceKey, "default")
}

func (c createParams) getCacheBytesValue(key string, valueCreator func() ([]byte, error)) ([]byte, error) {
	vIf, ok := (*c.cacheCtx)[key]
	if ok {
		return vIf.([]byte), nil
	}
	v, err := valueCreator()
	if err != nil {
		return nil, err
	}
	(*c.cacheCtx)[key] = v
	return v, err
}

func (c createParams) getCacheCUETplValue(key string, valueCreator func() (*intctrlutil.CUETpl, error)) (*intctrlutil.CUETpl, error) {
	vIf, ok := (*c.cacheCtx)[key]
	if ok {
		return vIf.(*intctrlutil.CUETpl), nil
	}
	v, err := valueCreator()
	if err != nil {
		return nil, err
	}
	(*c.cacheCtx)[key] = v
	return v, err
}

func (c createParams) getConfigTemplates() ([]dbaasv1alpha1.ConfigTemplate, error) {
	var appVersionTpl []dbaasv1alpha1.ConfigTemplate
	for _, component := range c.appVersion.Spec.Components {
		if component.Type == c.component.Type {
			appVersionTpl = component.ConfigTemplateRefs
			break
		}
	}
	return mergeConfigTemplates(appVersionTpl, c.getComponentConfigTemplates())
}

// mergeConfigTemplates merge AppVersion.Components[*].ConfigTemplateRefs and ClusterDefinition.Components[*].ConfigTemplateRefs
func mergeConfigTemplates(appVersionTpl []dbaasv1alpha1.ConfigTemplate, cdTpl []dbaasv1alpha1.ConfigTemplate) ([]dbaasv1alpha1.ConfigTemplate, error) {
	if len(appVersionTpl) == 0 {
		return cdTpl, nil
	}

	if len(cdTpl) == 0 {
		return appVersionTpl, nil
	}

	mergedCfgTpl := make([]dbaasv1alpha1.ConfigTemplate, 0, len(appVersionTpl)+len(cdTpl))
	mergedTplMap := make(map[string]bool, cap(mergedCfgTpl))

	for i := range appVersionTpl {
		if _, ok := (mergedTplMap)[appVersionTpl[i].VolumeName]; ok {
			return nil, fmt.Errorf("ConfigTemplate require not same volumeName [%s]", appVersionTpl[i].Name)
		}
		mergedCfgTpl = append(mergedCfgTpl, appVersionTpl[i])
		mergedTplMap[appVersionTpl[i].VolumeName] = true
	}

	for i := range cdTpl {
		// AppVersion replace clusterDefinition
		if _, ok := (mergedTplMap)[cdTpl[i].VolumeName]; ok {
			continue
		}
		mergedCfgTpl = append(mergedCfgTpl, cdTpl[i])
		mergedTplMap[cdTpl[i].VolumeName] = true
	}

	return mergedCfgTpl, nil
}

func (c createParams) getComponentConfigTemplates() []dbaasv1alpha1.ConfigTemplate {
	for _, component := range c.clusterDefinition.Spec.Components {
		if component.TypeName == c.component.Type {
			return component.ConfigTemplateRefs
		}
	}
	return nil
}

func getAppVersionComponentByType(components []dbaasv1alpha1.AppVersionComponent, typeName string) *dbaasv1alpha1.AppVersionComponent {
	for _, component := range components {
		if component.Type == typeName {
			return &component
		}
	}
	return nil
}

func getClusterComponentsByType(components []dbaasv1alpha1.ClusterComponent, typeName string) []*dbaasv1alpha1.ClusterComponent {
	comps := []*dbaasv1alpha1.ClusterComponent{}
	for _, component := range components {
		if component.Type == typeName {
			comps = append(comps, &component)
		}
	}
	return comps
}

func getContainerByName(containers []corev1.Container, name string) (int, *corev1.Container) {
	for i, container := range containers {
		if container.Name == name {
			return i, &container
		}
	}
	return -1, nil
}

func toK8sVolumeClaimTemplate(template dbaasv1alpha1.ClusterComponentVolumeClaimTemplate) corev1.PersistentVolumeClaimTemplate {
	t := corev1.PersistentVolumeClaimTemplate{}
	t.ObjectMeta.Name = template.Name
	if template.Spec != nil {
		t.Spec = *template.Spec
	}
	return t
}

func toK8sVolumeClaimTemplates(templates []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate) []corev1.PersistentVolumeClaimTemplate {
	ts := []corev1.PersistentVolumeClaimTemplate{}
	for _, template := range templates {
		ts = append(ts, toK8sVolumeClaimTemplate(template))
	}
	return ts
}

func buildAffinityLabelSelector(clusterName string, componentName string) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			appInstanceLabelKey:  clusterName,
			appComponentLabelKey: componentName,
		},
	}
}

func buildPodTopologySpreadConstraints(
	cluster *dbaasv1alpha1.Cluster,
	comAffinity *dbaasv1alpha1.Affinity,
	component *Component,
) []corev1.TopologySpreadConstraint {
	var topologySpreadConstraints []corev1.TopologySpreadConstraint

	var whenUnsatisfiable corev1.UnsatisfiableConstraintAction
	if comAffinity.PodAntiAffinity == dbaasv1alpha1.Required {
		whenUnsatisfiable = corev1.DoNotSchedule
	} else {
		whenUnsatisfiable = corev1.ScheduleAnyway
	}
	for _, topologyKey := range comAffinity.TopologyKeys {
		topologySpreadConstraints = append(topologySpreadConstraints, corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			WhenUnsatisfiable: whenUnsatisfiable,
			TopologyKey:       topologyKey,
			LabelSelector:     buildAffinityLabelSelector(cluster.Name, component.Name),
		})
	}
	return topologySpreadConstraints
}

func buildPodAffinity(
	cluster *dbaasv1alpha1.Cluster,
	comAffinity *dbaasv1alpha1.Affinity,
	component *Component,
) *corev1.Affinity {
	affinity := new(corev1.Affinity)
	// Build NodeAffinity
	var matchExpressions []corev1.NodeSelectorRequirement
	for key, value := range comAffinity.NodeLabels {
		values := strings.Split(value, ",")
		matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
			Key:      key,
			Operator: corev1.NodeSelectorOpIn,
			Values:   values,
		})
	}
	if len(matchExpressions) > 0 {
		nodeSelectorTerm := corev1.NodeSelectorTerm{
			MatchExpressions: matchExpressions,
		}
		affinity.NodeAffinity = &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{nodeSelectorTerm},
			},
		}
	}
	// Build PodAntiAffinity
	var podAntiAffinity *corev1.PodAntiAffinity
	var podAffinityTerms []corev1.PodAffinityTerm
	for _, topologyKey := range comAffinity.TopologyKeys {
		podAffinityTerms = append(podAffinityTerms, corev1.PodAffinityTerm{
			TopologyKey:   topologyKey,
			LabelSelector: buildAffinityLabelSelector(cluster.Name, component.Name),
		})
	}
	if comAffinity.PodAntiAffinity == dbaasv1alpha1.Required {
		podAntiAffinity = &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: podAffinityTerms,
		}
	} else {
		var weightedPodAffinityTerms []corev1.WeightedPodAffinityTerm
		for _, podAffinityTerm := range podAffinityTerms {
			weightedPodAffinityTerms = append(weightedPodAffinityTerms, corev1.WeightedPodAffinityTerm{
				Weight:          100,
				PodAffinityTerm: podAffinityTerm,
			})
		}
		podAntiAffinity = &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: weightedPodAffinityTerms,
		}
	}
	affinity.PodAntiAffinity = podAntiAffinity
	return affinity
}

func disableMonitor(component *Component) {
	component.Monitor = MonitorConfig{
		Enable: false,
	}
}

func mergeMonitorConfig(
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition,
	clusterDefComp *dbaasv1alpha1.ClusterDefinitionComponent,
	clusterComp *dbaasv1alpha1.ClusterComponent,
	component *Component) {
	monitorEnable := false
	if clusterComp != nil {
		monitorEnable = clusterComp.Monitor
	}

	monitorConfig := clusterDefComp.Monitor
	if !monitorEnable || monitorConfig == nil {
		disableMonitor(component)
		return
	}

	if !monitorConfig.BuiltIn {
		if monitorConfig.Exporter == nil {
			disableMonitor(component)
			return
		}
		component.Monitor = MonitorConfig{
			Enable:     true,
			ScrapePath: monitorConfig.Exporter.ScrapePath,
			ScrapePort: monitorConfig.Exporter.ScrapePort,
		}
		return
	}

	characterType := clusterDefComp.CharacterType
	if len(characterType) == 0 {
		characterType = CalcCharacterType(clusterDef.Spec.Type)
	}
	if !IsWellKnownCharacterType(characterType) {
		disableMonitor(component)
		return
	}

	switch characterType {
	case KMysql:
		err := WellKnownCharacterTypeFunc[KMysql](cluster, component)
		if err != nil {
			disableMonitor(component)
		}
	default:
		disableMonitor(component)
	}
}

func mergeComponents(
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition,
	clusterDefComp *dbaasv1alpha1.ClusterDefinitionComponent,
	appVerComp *dbaasv1alpha1.AppVersionComponent,
	clusterComp *dbaasv1alpha1.ClusterComponent) *Component {
	if clusterDefComp == nil {
		return nil
	}
	component := &Component{
		ClusterDefName:  clusterDef.Name,
		ClusterType:     clusterDef.Spec.Type,
		Name:            clusterDefComp.TypeName,
		Type:            clusterDefComp.TypeName,
		MinAvailable:    clusterDefComp.MinAvailable,
		MaxAvailable:    clusterDefComp.MaxAvailable,
		DefaultReplicas: clusterDefComp.DefaultReplicas,
		Replicas:        clusterDefComp.DefaultReplicas,
		AntiAffinity:    clusterDefComp.AntiAffinity,
		ComponentType:   clusterDefComp.ComponentType,
		ConsensusSpec:   clusterDefComp.ConsensusSpec,
		ReplicationSpec: clusterDefComp.ReplicationSpec,
		PrimaryStsIndex: clusterDefComp.PrimaryStsIndex,
		PodSpec:         clusterDefComp.PodSpec,
		Service:         clusterDefComp.Service,
		Scripts:         clusterDefComp.Scripts,
		Probes:          clusterDefComp.Probes,
	}

	if appVerComp != nil && appVerComp.PodSpec != nil {
		for _, container := range appVerComp.PodSpec.Containers {
			i, c := getContainerByName(component.PodSpec.Containers, container.Name)
			if c != nil {
				if container.Image != "" {
					component.PodSpec.Containers[i].Image = container.Image
				}
				if len(container.Command) != 0 {
					component.PodSpec.Containers[i].Command = container.Command
				}
				if len(container.Args) != 0 {
					component.PodSpec.Containers[i].Args = container.Args
				}
				if container.WorkingDir != "" {
					component.PodSpec.Containers[i].WorkingDir = container.WorkingDir
				}
				if len(container.Ports) != 0 {
					component.PodSpec.Containers[i].Ports = container.Ports
				}
				if len(container.EnvFrom) != 0 {
					component.PodSpec.Containers[i].EnvFrom = container.EnvFrom
				}
				if len(container.Env) != 0 {
					component.PodSpec.Containers[i].Env = container.Env
				}
				if container.Resources.Limits != nil || container.Resources.Requests != nil {
					component.PodSpec.Containers[i].Resources = container.Resources
				}
				if len(container.VolumeMounts) != 0 {
					component.PodSpec.Containers[i].VolumeMounts = container.VolumeMounts
				}
				if len(container.VolumeDevices) != 0 {
					component.PodSpec.Containers[i].VolumeDevices = container.VolumeDevices
				}
				if container.LivenessProbe != nil {
					component.PodSpec.Containers[i].LivenessProbe = container.LivenessProbe
				}
				if container.ReadinessProbe != nil {
					component.PodSpec.Containers[i].ReadinessProbe = container.ReadinessProbe
				}
				if container.StartupProbe != nil {
					component.PodSpec.Containers[i].StartupProbe = container.StartupProbe
				}
				if container.Lifecycle != nil {
					component.PodSpec.Containers[i].Lifecycle = container.Lifecycle
				}
				if container.TerminationMessagePath != "" {
					component.PodSpec.Containers[i].TerminationMessagePath = container.TerminationMessagePath
				}
				if container.TerminationMessagePolicy != "" {
					component.PodSpec.Containers[i].TerminationMessagePolicy = container.TerminationMessagePolicy
				}
				if container.ImagePullPolicy != "" {
					component.PodSpec.Containers[i].ImagePullPolicy = container.ImagePullPolicy
				}
				if container.SecurityContext != nil {
					component.PodSpec.Containers[i].SecurityContext = container.SecurityContext
				}
			} else {
				component.PodSpec.Containers = append(component.PodSpec.Containers, container)
			}
		}
	}
	affinity := cluster.Spec.Affinity
	if clusterComp != nil {
		component.Name = clusterComp.Name

		// respect user's declaration
		if clusterComp.Replicas > 0 {
			component.Replicas = clusterComp.Replicas
		}

		if clusterComp.VolumeClaimTemplates != nil {
			component.VolumeClaimTemplates = toK8sVolumeClaimTemplates(clusterComp.VolumeClaimTemplates)
		}
		if clusterComp.Resources.Requests != nil || clusterComp.Resources.Limits != nil {
			component.PodSpec.Containers[0].Resources = clusterComp.Resources
		}

		// respect user's declaration
		if clusterComp.ServiceType != "" {
			component.Service.Type = clusterComp.ServiceType
		}

		if clusterComp.Affinity != nil {
			affinity = clusterComp.Affinity
		}

		if clusterComp.PrimaryStsIndex != nil {
			component.PrimaryStsIndex = clusterComp.PrimaryStsIndex
		}
	}
	if component.PodSpec.Affinity == nil && affinity != nil {
		component.PodSpec.Affinity = buildPodAffinity(cluster, affinity, component)
	}
	if len(component.PodSpec.TopologySpreadConstraints) == 0 && affinity != nil {
		component.PodSpec.TopologySpreadConstraints = buildPodTopologySpreadConstraints(cluster, affinity, component)
	}

	// TODO(zhixu.zt) We need to reserve the VolumeMounts of the container for ConfigMap or Secret,
	// At present, it is possible to distinguish between ConfigMap volume and normal volume,
	// Compare the VolumeName of configTemplateRef and Name of VolumeMounts
	//
	// if component.VolumeClaimTemplates == nil {
	//	 for i := range component.PodSpec.Containers {
	//	 	component.PodSpec.Containers[i].VolumeMounts = nil
	//	 }
	// }

	mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)

	return component
}

func buildClusterCreationTasks(
	clusterDefinition *dbaasv1alpha1.ClusterDefinition,
	appVersion *dbaasv1alpha1.AppVersion,
	cluster *dbaasv1alpha1.Cluster) (*intctrlutil.Task, error) {
	rootTask := intctrlutil.NewTask()

	applyObjs := make([]client.Object, 0, 3)
	cacheCtx := map[string]interface{}{}

	prepareSecretsTask := intctrlutil.NewTask()
	prepareSecretsTask.ExecFunction = prepareSecretObjs
	params := createParams{
		cluster:           cluster,
		clusterDefinition: clusterDefinition,
		applyObjs:         &applyObjs,
		cacheCtx:          &cacheCtx,
		appVersion:        appVersion,
	}
	prepareSecretsTask.Context["exec"] = &params
	rootTask.SubTasks = append(rootTask.SubTasks, prepareSecretsTask)

	buildTask := func(component *Component) {
		componentTask := intctrlutil.NewTask()
		componentTask.ExecFunction = prepareComponentObjs
		iParams := params
		iParams.component = component
		componentTask.Context["exec"] = &iParams
		rootTask.SubTasks = append(rootTask.SubTasks, componentTask)
	}

	components := clusterDefinition.Spec.Components
	useDefaultComp := len(cluster.Spec.Components) == 0
	for _, component := range components {
		componentName := component.TypeName
		appVersionComponent := getAppVersionComponentByType(appVersion.Spec.Components, componentName)

		if useDefaultComp {
			buildTask(mergeComponents(cluster, clusterDefinition, &component, appVersionComponent, nil))
		} else {
			clusterComps := getClusterComponentsByType(cluster.Spec.Components, componentName)
			for _, clusterComp := range clusterComps {
				buildTask(mergeComponents(cluster, clusterDefinition, &component, appVersionComponent, clusterComp))
			}
		}
	}

	createObjsTask := intctrlutil.NewTask()
	createObjsTask.ExecFunction = checkedCreateObjs
	createObjsTask.Context["exec"] = &params
	rootTask.SubTasks = append(rootTask.SubTasks, createObjsTask)
	return &rootTask, nil
}

func checkedCreateObjs(reqCtx intctrlutil.RequestCtx, cli client.Client, obj interface{}) error {
	params, ok := obj.(*createParams)
	if !ok {
		return fmt.Errorf("invalid arg")
	}

	if err := createOrReplaceResources(reqCtx, cli, params.cluster, *params.applyObjs); err != nil {
		return err
	}
	return nil
}

func prepareSecretObjs(reqCtx intctrlutil.RequestCtx, cli client.Client, obj interface{}) error {
	params, ok := obj.(*createParams)
	if !ok {
		return fmt.Errorf("invalid arg")
	}

	secret, err := buildSecret(*params)
	if err != nil {
		return err
	}
	// must make sure secret resources are created before others
	*params.applyObjs = append(*params.applyObjs, secret)
	return nil
}

// TODO: @free6om handle config of all component types
func prepareComponentObjs(reqCtx intctrlutil.RequestCtx, cli client.Client, obj interface{}) error {
	params, ok := obj.(*createParams)
	if !ok {
		return fmt.Errorf("invalid arg")
	}

	switch params.component.ComponentType {
	case dbaasv1alpha1.Stateless:
		sts, err := buildDeploy(*params)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, sts)
	case dbaasv1alpha1.Stateful:
		sts, err := buildSts(reqCtx, *params)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, sts)

		svcs, err := buildHeadlessSvcs(*params, sts)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, svcs...)

		// render config
		configs, err := buildCfg(*params, sts, reqCtx.Ctx, cli)
		if err != nil {
			return err
		}
		if configs != nil {
			*params.applyObjs = append(*params.applyObjs, configs...)
		}
		// end render config
	case dbaasv1alpha1.Consensus:
		css, err := buildConsensusSet(reqCtx, *params)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, css)

		svcs, err := buildHeadlessSvcs(*params, css)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, svcs...)

		// render config
		configs, err := buildCfg(*params, css, reqCtx.Ctx, cli)
		if err != nil {
			return err
		}
		if configs != nil {
			*params.applyObjs = append(*params.applyObjs, configs...)
		}
	case dbaasv1alpha1.Replication:
		rstsList, err := buildReplicationSet(reqCtx, cli, *params)
		if err != nil {
			return err
		}
		for _, rsts := range rstsList {
			*params.applyObjs = append(*params.applyObjs, rsts)

			svcs, err := buildHeadlessSvcs(*params, rsts)
			if err != nil {
				return err
			}
			*params.applyObjs = append(*params.applyObjs, svcs...)

			configs, err := buildCfg(*params, rsts, reqCtx.Ctx, cli)
			if err != nil {
				return err
			}
			if configs != nil {
				*params.applyObjs = append(*params.applyObjs, configs...)
			}
		}
	}

	pdb, err := buildPDB(*params)
	if err != nil {
		return err
	}
	*params.applyObjs = append(*params.applyObjs, pdb)

	if params.component.Service.Ports != nil {
		svc, err := buildSvc(*params)
		if err != nil {
			return err
		}
		if params.component.ComponentType == dbaasv1alpha1.Consensus {
			addSelectorLabels(svc, params.component, dbaasv1alpha1.ReadWrite)
		}
		*params.applyObjs = append(*params.applyObjs, svc)
	}

	return nil
}

// TODO multi roles with same accessMode support
func addSelectorLabels(service *corev1.Service, component *Component, accessMode dbaasv1alpha1.AccessMode) {
	addSelector := func(service *corev1.Service, member dbaasv1alpha1.ConsensusMember, accessMode dbaasv1alpha1.AccessMode) {
		if member.AccessMode == accessMode && len(member.Name) > 0 {
			service.Spec.Selector[consensusSetRoleLabelKey] = member.Name
		}
	}

	addSelector(service, component.ConsensusSpec.Leader, accessMode)
	if component.ConsensusSpec.Learner != nil {
		addSelector(service, *component.ConsensusSpec.Learner, accessMode)
	}

	for _, member := range component.ConsensusSpec.Followers {
		addSelector(service, member, accessMode)
	}
}

func createOrReplaceResources(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	objs []client.Object) error {
	ctx := reqCtx.Ctx
	logger := reqCtx.Log
	scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()
	var newCreateStsList []*appsv1.StatefulSet
	var existStsList []*appsv1.StatefulSet
	for _, obj := range objs {
		logger.Info("create or update", "objs", obj)
		if err := controllerutil.SetOwnerReference(cluster, obj, scheme); err != nil {
			return err
		}
		// appendToStsList is used to handleReplicationSet(...) when componentType is replication
		appendToStsList := func(stsList []*appsv1.StatefulSet) []*appsv1.StatefulSet {
			stsObj, ok := obj.(*appsv1.StatefulSet)
			if ok {
				stsList = append(stsList, stsObj)
			}
			return stsList
		}
		if err := cli.Create(ctx, obj); err == nil {
			newCreateStsList = appendToStsList(newCreateStsList)
			continue
		} else if apierrors.IsAlreadyExists(err) {
			existStsList = appendToStsList(existStsList)
		} else {
			return err
		}
		if !controllerutil.ContainsFinalizer(obj, dbClusterFinalizerName) {
			controllerutil.AddFinalizer(obj, dbClusterFinalizerName)
		}

		// Secret kind objects should only be applied once
		if _, ok := obj.(*corev1.Secret); ok {
			continue
		}

		// ConfigMap kind objects should only be applied once
		//
		// The Config is not allowed to be modified.
		// Once ISV adjusts the ConfigTemplateRef field of CusterDefinition, or ISV modifies the wrong config file, it may cause the application cluster may fail.
		//
		// TODO(zhixu.zt): Check whether the configmap object is a config file of component
		// Label check: ConfigMap.Labels["app.kubernetes.io/ins-configure"]
		if _, ok := obj.(*corev1.ConfigMap); ok {
			continue
		}

		key := client.ObjectKey{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		}
		stsProto, ok := obj.(*appsv1.StatefulSet)
		if ok {
			stsObj := &appsv1.StatefulSet{}
			if err := cli.Get(ctx, key, stsObj); err != nil {
				return err
			}
			stsObj.Spec.Template = stsProto.Spec.Template
			stsObj.Spec.Replicas = stsProto.Spec.Replicas
			stsObj.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
			if err := cli.Update(ctx, stsObj); err != nil {
				return err
			}
			// handle ConsensusSet Update
			if stsObj.Status.CurrentRevision != stsObj.Status.UpdateRevision {
				_, err := handleConsensusSetUpdate(ctx, cli, cluster, stsObj)
				if err != nil {
					return err
				}
			}
			// check stsObj.Spec.VolumeClaimTemplates storage
			// request size and find attached PVC and patch request
			// storage size
			for _, vct := range stsObj.Spec.VolumeClaimTemplates {
				var vctProto *corev1.PersistentVolumeClaim
				for _, i := range stsProto.Spec.VolumeClaimTemplates {
					if i.Name == vct.Name {
						vctProto = &i
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
					if err := cli.Get(ctx, pvcKey, pvc); err != nil {
						return err
					}
					if pvc.Spec.Resources.Requests[corev1.ResourceStorage] == vctProto.Spec.Resources.Requests[corev1.ResourceStorage] {
						continue
					}
					patch := client.MergeFrom(pvc.DeepCopy())
					pvc.Spec.Resources.Requests[corev1.ResourceStorage] = vctProto.Spec.Resources.Requests[corev1.ResourceStorage]
					if err := cli.Patch(ctx, pvc, patch); err != nil {
						return err
					}
				}
			}
			continue
		}
		deployProto, ok := obj.(*appsv1.Deployment)
		if ok {
			deployObj := &appsv1.Deployment{}
			if err := cli.Get(ctx, key, deployObj); err != nil {
				return err
			}
			deployObj.Spec = deployProto.Spec
			if err := cli.Update(ctx, deployObj); err != nil {
				return err
			}
			continue
		}
		svcProto, ok := obj.(*corev1.Service)
		if ok {
			svcObj := &corev1.Service{}
			if err := cli.Get(ctx, key, svcObj); err != nil {
				return err
			}
			svcObj.Spec = svcProto.Spec
			if err := cli.Update(ctx, svcObj); err != nil {
				return err
			}
			continue
		}
	}

	err := handleReplicationSet(reqCtx, cli, cluster, newCreateStsList, existStsList)
	if err != nil {
		return err
	}
	return nil
}

func handleReplicationSet(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	newCreateStsList []*appsv1.StatefulSet,
	existStsList []*appsv1.StatefulSet) error {

	filter := func(stsObj *appsv1.StatefulSet) (dbaasv1alpha1.ClusterDefinitionComponent, bool, error) {
		typeName := getComponentTypeName(*cluster, stsObj.Labels[appComponentLabelKey])
		component, err := getComponent(reqCtx.Ctx, cli, cluster, typeName)
		if err != nil {
			return dbaasv1alpha1.ClusterDefinitionComponent{}, false, err
		}
		if component.ComponentType != dbaasv1alpha1.Replication {
			return component, true, nil
		}
		return component, false, nil
	}

	// handle new create StatefulSets including create a replication relationship and update Pod label, etc
	err := handleReplicationSetNewCreateSts(reqCtx, cli, cluster, newCreateStsList, filter)
	if err != nil {
		return err
	}

	// handle exist StatefulSets including delete sts when pod number larger than cluster.component[i].replicas
	// delete the StatefulSets with the largest sequence number which is not the primary role
	err = handleReplicationSetExistSts(reqCtx, cli, cluster, existStsList, filter)
	if err != nil {
		return err
	}

	return nil
}

func handleReplicationSetExistSts(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	existStsList []*appsv1.StatefulSet,
	filter func(stsObj *appsv1.StatefulSet) (dbaasv1alpha1.ClusterDefinitionComponent, bool, error)) error {

	clusterCompReplicasMap := make(map[string]int, len(cluster.Spec.Components))
	for _, clusterComp := range cluster.Spec.Components {
		clusterCompReplicasMap[clusterComp.Name] = clusterComp.Replicas
	}

	compOwnsStsMap := make(map[string]int)
	stsToDeleteMap := make(map[string]int)
	for _, stsObj := range existStsList {
		_, skip, err := filter(stsObj)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		if _, ok := compOwnsStsMap[stsObj.Labels[appComponentLabelKey]]; !ok {
			compOwnsStsMap[stsObj.Labels[appComponentLabelKey]] = 0
			stsToDeleteMap[stsObj.Labels[appComponentLabelKey]] = 0
		}
		compOwnsStsMap[stsObj.Labels[appComponentLabelKey]] += 1
		if compOwnsStsMap[stsObj.Labels[appComponentLabelKey]] > clusterCompReplicasMap[stsObj.Labels[appComponentLabelKey]] {
			stsToDeleteMap[stsObj.Labels[appComponentLabelKey]] += 1
		}
	}

	for compKey, stsToDelNum := range stsToDeleteMap {
		if stsToDelNum == 0 {
			break
		}
		// list all statefulSets by componentKey label
		allStsList := &appsv1.StatefulSetList{}
		selector, err := labels.Parse(appComponentLabelKey + "=" + compKey)
		if err != nil {
			return err
		}
		if err := cli.List(reqCtx.Ctx, allStsList,
			&client.ListOptions{Namespace: cluster.Namespace},
			client.MatchingLabelsSelector{Selector: selector}); err != nil {
			return err
		}
		if compOwnsStsMap[compKey] != len(allStsList.Items) {
			return fmt.Errorf("statefulset total number has changed")
		}
		dos := make([]*appsv1.StatefulSet, 0)
		partition := len(allStsList.Items) - stsToDelNum
		for _, sts := range allStsList.Items {
			// if current primary statefulSet ordinal is larger than target number replica, return err
			if getOrdinalSts(&sts) > partition && checkStsIsPrimary(&sts) {
				return fmt.Errorf("current primary statefulset ordinal is larger than target number replicas, can not be reduce, please switchover first")
			}
			dos = append(dos, sts.DeepCopy())
		}

		// sort the statefulSets by their ordinals
		sort.Sort(descendingOrdinalSts(dos))

		// delete statefulSets and svc, etc
		for i := 0; i < stsToDelNum; i++ {
			if err := cli.Delete(reqCtx.Ctx, dos[i]); err != nil {
				return err
			}
			svc := &corev1.Service{}
			svcKey := types.NamespacedName{
				Namespace: cluster.Namespace,
				Name:      fmt.Sprintf("%s-%d", dos[i].Name, 0),
			}
			if err := cli.Get(reqCtx.Ctx, svcKey, svc); err != nil {
				return err
			}
			if err := cli.Delete(reqCtx.Ctx, svc); err != nil {
				return err
			}
		}
	}
	return nil
}

func handleReplicationSetNewCreateSts(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	newCreateStsList []*appsv1.StatefulSet,
	filter func(stsObj *appsv1.StatefulSet) (dbaasv1alpha1.ClusterDefinitionComponent, bool, error)) error {

	podRoleMap := make(map[string]string, len(newCreateStsList))
	stsRoleMap := make(map[string]string, len(newCreateStsList))
	for _, stsObj := range newCreateStsList {
		var isPrimarySts = false
		component, skip, err := filter(stsObj)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		// get target engine pod info in stsObj
		allPodList := &corev1.PodList{}
		selector, err := labels.Parse(appInstanceLabelKey + "=" + cluster.Name)
		if err != nil {
			return err
		}
		if err := cli.List(reqCtx.Ctx, allPodList,
			&client.ListOptions{Namespace: stsObj.Namespace},
			client.MatchingLabelsSelector{Selector: selector}); err != nil {
			return err
		}
		var targetPodList []corev1.Pod
		for _, pod := range allPodList.Items {
			if isMemberOf(stsObj, &pod) {
				targetPodList = append(targetPodList, pod)
			}
		}
		if len(targetPodList) != 1 {
			return fmt.Errorf("pod number in statefulset %s is not equal one", stsObj.Name)
		}
		var dbEnginePod = &targetPodList[0]
		claimPrimaryStsName := fmt.Sprintf("%s-%s-%d", cluster.Name, stsObj.Labels[appComponentLabelKey], getClaimPrimaryStsIndex(cluster, component))
		if stsObj.Name == claimPrimaryStsName {
			isPrimarySts = true
		}
		podRoleMap[dbEnginePod.Name] = string(dbaasv1alpha1.Primary)
		stsRoleMap[stsObj.Name] = string(dbaasv1alpha1.Primary)
		if !isPrimarySts {
			podRoleMap[dbEnginePod.Name] = string(dbaasv1alpha1.Secondary)
			stsRoleMap[stsObj.Name] = string(dbaasv1alpha1.Secondary)
			// if not primary, create a replication relationship by running a Job with kube exec
			err := createReplRelationJobAndEnsure(reqCtx, cli, cluster, component, stsObj, dbEnginePod)
			if err != nil {
				return err
			}
		}
	}
	// update replicationSet StatefulSet Label
	for k, v := range stsRoleMap {
		stsName := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      k,
		}
		err := updateReplicationSetStsRoleLabel(cli, reqCtx.Ctx, stsName, v)
		if err != nil {
			return err
		}
	}
	// update replicationSet Pod Label
	for k, v := range podRoleMap {
		podName := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      k,
		}
		err := updateReplicationSetPodRoleLabel(cli, reqCtx.Ctx, podName, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func getClaimPrimaryStsIndex(cluster *dbaasv1alpha1.Cluster, clusterDefComp dbaasv1alpha1.ClusterDefinitionComponent) int {
	claimPrimaryStsIndex := clusterDefComp.PrimaryStsIndex
	for _, clusterComp := range cluster.Spec.Components {
		if clusterComp.Type == clusterDefComp.TypeName {
			if clusterComp.PrimaryStsIndex != nil {
				claimPrimaryStsIndex = clusterComp.PrimaryStsIndex
			}
		}
	}
	return *claimPrimaryStsIndex
}

func createReplRelationJobAndEnsure(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	component dbaasv1alpha1.ClusterDefinitionComponent,
	stsObj *appsv1.StatefulSet,
	enginePod *corev1.Pod) error {
	key := types.NamespacedName{Namespace: stsObj.Namespace, Name: stsObj.Name + "-repl"}
	job := batchv1.Job{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, cli, key, &job)
	if err != nil {
		return err
	}
	if !exists {
		// if not exist job, create a new job
		jobPodSpec, err := buildReplRelationJobPodSpec(component, stsObj, enginePod)
		if err != nil {
			return err
		}
		var ttlSecondsAfterJobFinished int32 = 30
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: key.Namespace,
				Name:      key.Name,
				Labels:    nil,
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: key.Namespace,
						Name:      key.Name},
					Spec: jobPodSpec,
				},
				TTLSecondsAfterFinished: &ttlSecondsAfterJobFinished,
			},
		}
		scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()
		if err := controllerutil.SetOwnerReference(cluster, job, scheme); err != nil {
			return err
		}
		reqCtx.Log.Info("create a built-in job from create replication relationship", "job", job)
		if err := cli.Create(reqCtx.Ctx, job); err != nil {
			return err
		}
	}

	// ensure job finished
	jobStatusConditions := job.Status.Conditions
	if len(jobStatusConditions) > 0 {
		if jobStatusConditions[0].Type != batchv1.JobComplete {
			return fmt.Errorf("job status: %s is not Complete, please wait or check", jobStatusConditions[0].Type)
		}
	}
	return nil
}

func buildReplRelationJobPodSpec(
	component dbaasv1alpha1.ClusterDefinitionComponent,
	stsObj *appsv1.StatefulSet,
	dbEnginePod *corev1.Pod) (corev1.PodSpec, error) {
	podSpec := corev1.PodSpec{}
	container := corev1.Container{}
	container.Name = stsObj.Name

	var targetEngineContainer corev1.Container
	for _, c := range dbEnginePod.Spec.Containers {
		if c.Name == component.ReplicationSpec.CreateReplication.DbEngineContainer {
			targetEngineContainer = c
		}
	}
	container.Command = []string{"kubectl", "exec", "-i", dbEnginePod.Name, "-c", targetEngineContainer.Name, "--", "sh", "-c"}
	container.Args = component.ReplicationSpec.CreateReplication.Commands
	container.Image = component.ReplicationSpec.CreateReplication.Image
	container.VolumeMounts = targetEngineContainer.VolumeMounts
	container.Env = targetEngineContainer.Env
	podSpec.Containers = []corev1.Container{container}
	podSpec.Volumes = dbEnginePod.Spec.Volumes
	podSpec.RestartPolicy = corev1.RestartPolicyNever
	return podSpec, nil
}

func handleConsensusSetUpdate(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster, stsObj *appsv1.StatefulSet) (bool, error) {
	// get typeName from stsObj.name
	typeName := getComponentTypeName(*cluster, stsObj.Labels[appComponentLabelKey])

	// get component from ClusterDefinition by typeName
	component, err := getComponent(ctx, cli, cluster, typeName)
	if err != nil {
		return false, err
	}

	if component.ComponentType != dbaasv1alpha1.Consensus {
		return true, nil
	}

	// get podList owned by stsObj
	podList := &corev1.PodList{}
	selector, err := labels.Parse(appComponentLabelKey + "=" + stsObj.Labels[appComponentLabelKey])
	if err != nil {
		return false, err
	}
	if err := cli.List(ctx, podList,
		&client.ListOptions{Namespace: stsObj.Namespace},
		client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return false, err
	}
	pods := make([]corev1.Pod, 0)
	for _, pod := range podList.Items {
		if isMemberOf(stsObj, &pod) {
			pods = append(pods, pod)
		}
	}

	// get pod label and name, compute plan
	plan := generateConsensusUpdatePlan(ctx, cli, stsObj, pods, component)
	// execute plan
	return plan.walkOneStep()
}

// generateConsensusUpdatePlan generates Update plan based on UpdateStrategy
func generateConsensusUpdatePlan(ctx context.Context, cli client.Client, stsObj *appsv1.StatefulSet, pods []corev1.Pod, component dbaasv1alpha1.ClusterDefinitionComponent) *Plan {
	plan := &Plan{}
	plan.Start = &Step{}
	plan.WalkFunc = func(obj interface{}) (bool, error) {
		pod, ok := obj.(corev1.Pod)
		if !ok {
			return false, errors.New("wrong type: obj not Pod")
		}
		// if pod is the latest version, we do nothing
		if getPodRevision(&pod) == stsObj.Status.UpdateRevision {
			return false, nil
		}
		// if DeletionTimestamp is not nil, it is terminating.
		if pod.DeletionTimestamp != nil {
			return true, nil
		}
		// delete the pod to trigger associate StatefulSet to re-create it
		if err := cli.Delete(ctx, &pod); err != nil {
			return false, err
		}

		return true, nil
	}

	// list all roles
	if component.ConsensusSpec == nil {
		component.ConsensusSpec = &dbaasv1alpha1.ConsensusSetSpec{Leader: dbaasv1alpha1.DefaultLeader}
	}
	leader := component.ConsensusSpec.Leader.Name
	learner := ""
	if component.ConsensusSpec.Learner != nil {
		learner = component.ConsensusSpec.Learner.Name
	}
	// now all are followers
	noneFollowers := make(map[string]string)
	readonlyFollowers := make(map[string]string)
	readWriteFollowers := make(map[string]string)
	// a follower name set
	followers := make(map[string]string)
	exist := "EXIST"
	for _, follower := range component.ConsensusSpec.Followers {
		followers[follower.Name] = exist
		switch follower.AccessMode {
		case dbaasv1alpha1.None:
			noneFollowers[follower.Name] = exist
		case dbaasv1alpha1.Readonly:
			readonlyFollowers[follower.Name] = exist
		case dbaasv1alpha1.ReadWrite:
			readWriteFollowers[follower.Name] = exist
		}
	}

	// make a Serial pod list, e.g.: learner -> follower1 -> follower2 -> leader
	sort.SliceStable(pods, func(i, j int) bool {
		roleI := pods[i].Labels[consensusSetRoleLabelKey]
		roleJ := pods[j].Labels[consensusSetRoleLabelKey]
		if roleI == learner {
			return true
		}
		if roleJ == learner {
			return false
		}
		if roleI == leader {
			return false
		}
		if roleJ == leader {
			return true
		}
		if noneFollowers[roleI] == exist {
			return true
		}
		if noneFollowers[roleJ] == exist {
			return false
		}
		if readonlyFollowers[roleI] == exist {
			return true
		}
		if readonlyFollowers[roleJ] == exist {
			return false
		}
		if readWriteFollowers[roleI] == exist {
			return true
		}

		return false
	})

	// generate plan by UpdateStrategy
	switch component.ConsensusSpec.UpdateStrategy {
	case dbaasv1alpha1.Serial:
		// learner -> followers(none->readonly->readwrite) -> leader
		start := plan.Start
		for _, pod := range pods {
			nextStep := &Step{}
			nextStep.Obj = pod
			start.NextSteps = append(start.NextSteps, nextStep)
			start = nextStep
		}
	case dbaasv1alpha1.Parallel:
		// leader & followers & learner
		start := plan.Start
		for _, pod := range pods {
			nextStep := &Step{}
			nextStep.Obj = pod
			start.NextSteps = append(start.NextSteps, nextStep)
		}
	case dbaasv1alpha1.BestEffortParallel:
		// learner & 1/2 followers -> 1/2 followers -> leader
		start := plan.Start
		// append learner
		index := 0
		for _, pod := range pods {
			if pod.Labels[consensusSetRoleLabelKey] != learner {
				break
			}
			nextStep := &Step{}
			nextStep.Obj = pod
			start.NextSteps = append(start.NextSteps, nextStep)
			index++
		}
		if len(start.NextSteps) > 0 {
			start = start.NextSteps[0]
		}
		// append 1/2 followers
		podList := pods[index:]
		followerCount := 0
		for _, pod := range podList {
			if followers[pod.Labels[consensusSetRoleLabelKey]] == exist {
				followerCount++
			}
		}
		end := followerCount / 2
		for i := 0; i < end; i++ {
			nextStep := &Step{}
			nextStep.Obj = podList[i]
			start.NextSteps = append(start.NextSteps, nextStep)
		}

		if len(start.NextSteps) > 0 {
			start = start.NextSteps[0]
		}
		// append the other 1/2 followers
		podList = podList[end:]
		end = followerCount - end
		for i := 0; i < end; i++ {
			nextStep := &Step{}
			nextStep.Obj = podList[i]
			start.NextSteps = append(start.NextSteps, nextStep)
		}

		if len(start.NextSteps) > 0 {
			start = start.NextSteps[0]
		}
		// append leader
		podList = podList[end:]
		for _, pod := range podList {
			nextStep := &Step{}
			nextStep.Obj = pod
			start.NextSteps = append(start.NextSteps, nextStep)
		}
	}

	return plan
}

func getComponent(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster, typeName string) (dbaasv1alpha1.ClusterDefinitionComponent, error) {
	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	if err := cli.Get(ctx, client.ObjectKey{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
		return dbaasv1alpha1.ClusterDefinitionComponent{}, err
	}

	for _, component := range clusterDef.Spec.Components {
		if component.TypeName == typeName {
			return component, nil
		}
	}

	return dbaasv1alpha1.ClusterDefinitionComponent{}, errors.New("componentDef not found: " + typeName)
}

func getComponentTypeName(cluster dbaasv1alpha1.Cluster, componentName string) string {
	for _, component := range cluster.Spec.Components {
		if componentName == component.Name {
			return component.Type
		}
	}

	return componentName
}

func buildHeadlessSvcs(params createParams, sts *appsv1.StatefulSet) ([]client.Object, error) {
	stsPodLabels := sts.Spec.Template.Labels
	replicas := *sts.Spec.Replicas
	svcs := make([]client.Object, replicas)
	for i := 0; i < int(replicas); i++ {
		pod := &corev1.Pod{}
		pod.ObjectMeta.Name = fmt.Sprintf("%s-%d", sts.GetName(), i)
		pod.ObjectMeta.Namespace = sts.Namespace
		pod.ObjectMeta.Labels = map[string]string{
			statefulSetPodNameLabelKey: pod.ObjectMeta.Name,
			appNameLabelKey:            stsPodLabels[appNameLabelKey],
			appInstanceLabelKey:        stsPodLabels[appInstanceLabelKey],
			appComponentLabelKey:       stsPodLabels[appNameLabelKey],
		}
		pod.Spec.Containers = sts.Spec.Template.Spec.Containers

		svc, err := buildHeadlessService(params, pod)
		if err != nil {
			return nil, err
		}
		svcs[i] = svc
	}
	return svcs, nil
}

func buildSvc(params createParams) (*corev1.Service, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("service_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("service_template.cue"))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	clusterStrByte, err := params.getCacheBytesValue("cluster", func() ([]byte, error) {
		return json.Marshal(params.cluster)
	})
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("cluster", clusterStrByte); err != nil {
		return nil, err
	}

	componentStrByte, err := json.Marshal(params.component)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("component", componentStrByte); err != nil {
		return nil, err
	}

	svcStrByte, err := cueValue.Lookup("service")
	if err != nil {
		return nil, err
	}

	svc := corev1.Service{}
	if err = json.Unmarshal(svcStrByte, &svc); err != nil {
		return nil, err
	}

	return &svc, nil
}

func buildSecret(params createParams) (*corev1.Secret, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("secret_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("secret_template.cue"))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	clusterDefinitionStrByte, err := params.getCacheBytesValue("clusterDefinition", func() ([]byte, error) {
		return json.Marshal(params.clusterDefinition)
	})
	if err != nil {
		return nil, err
	}

	if err = cueValue.Fill("clusterdefinition", clusterDefinitionStrByte); err != nil {
		return nil, err
	}

	clusterStrByte, err := params.getCacheBytesValue("cluster", func() ([]byte, error) {
		return json.Marshal(params.cluster)
	})
	if err != nil {
		return nil, err
	}

	if err = cueValue.Fill("cluster", clusterStrByte); err != nil {
		return nil, err
	}

	secretStrByte, err := cueValue.Lookup("secret")
	if err != nil {
		return nil, err
	}

	secret := corev1.Secret{}
	if err = json.Unmarshal(secretStrByte, &secret); err != nil {
		return nil, err
	}

	return &secret, nil
}

// buildReplicationSet build on stateful set of replication
func buildReplicationSet(reqCtx intctrlutil.RequestCtx, cli client.Client, params createParams) ([]*appsv1.StatefulSet, error) {
	var stsList []*appsv1.StatefulSet

	// get math.Max(params.component.Replicas, current exist statefulSet)
	existStsList := &appsv1.StatefulSetList{}
	selector, err := labels.Parse(appComponentLabelKey + "=" + params.component.Name)
	if err != nil {
		return stsList, err
	}
	if err := cli.List(reqCtx.Ctx, existStsList,
		&client.ListOptions{Namespace: params.cluster.Namespace},
		client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return stsList, err
	}
	replicaNum := math.Max(float64(len(existStsList.Items)), float64(params.component.Replicas))

	for i := 0; i < int(replicaNum); i++ {
		sts, err := buildSts(reqCtx, params)
		if err != nil {
			return nil, err
		}
		// inject replicationSet Pod Env
		if sts, err = injectReplicationStsPodEnv(params, sts, i); err != nil {
			return nil, err
		}
		sts.ObjectMeta.Name = fmt.Sprintf("%s-%d", sts.ObjectMeta.Name, i)
		sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
		stsList = append(stsList, sts)
	}
	return stsList, nil
}

func injectReplicationStsPodEnv(params createParams, sts *appsv1.StatefulSet, index int) (*appsv1.StatefulSet, error) {
	for _, comp := range params.cluster.Spec.Components {
		if index != *comp.PrimaryStsIndex {
			for i := range sts.Spec.Template.Spec.Containers {
				c := &sts.Spec.Template.Spec.Containers[i]
				c.Env = append(c.Env, corev1.EnvVar{
					Name:      dbaasPrefix + "_PRIMARY_POD_NAME",
					Value:     sts.Name + "-" + strconv.Itoa(*comp.PrimaryStsIndex) + "-0",
					ValueFrom: nil,
				})
				for j, port := range c.Ports {
					c.Env = append(c.Env, corev1.EnvVar{
						Name:      dbaasPrefix + "_PRIMARY_POD_PORT_" + strconv.Itoa(j),
						Value:     strconv.Itoa(int(port.ContainerPort)),
						ValueFrom: nil,
					})
				}
			}
		}
	}
	return sts, nil
}

func buildSts(reqCtx intctrlutil.RequestCtx, params createParams) (*appsv1.StatefulSet, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("statefulset_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("statefulset_template.cue"))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	clusterStrByte, err := params.getCacheBytesValue("cluster", func() ([]byte, error) {
		return json.Marshal(params.cluster)
	})
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("cluster", clusterStrByte); err != nil {
		return nil, err
	}

	componentStrByte, err := json.Marshal(params.component)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("component", componentStrByte); err != nil {
		return nil, err
	}

	stsStrByte, err := cueValue.Lookup("statefulset")
	if err != nil {
		return nil, err
	}

	sts := appsv1.StatefulSet{}

	stsStrByte = injectEnv(stsStrByte, dbaasPrefix+"_SECRET_NAME", params.cluster.Name)

	if err = json.Unmarshal(stsStrByte, &sts); err != nil {
		return nil, err
	}

	probeContainers, err := buildProbeContainers(reqCtx, params)
	if err != nil {
		return nil, err
	}
	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, probeContainers...)
	prefix := dbaasPrefix + "_" + strings.ToUpper(params.component.Type) + "_"
	replicas := int(*sts.Spec.Replicas)
	for i := range sts.Spec.Template.Spec.Containers {
		// inject self scope env
		c := &sts.Spec.Template.Spec.Containers[i]
		c.Env = append(c.Env, corev1.EnvVar{
			Name: dbaasPrefix + "_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		})
		// inject component scope env
		c.Env = append(c.Env, corev1.EnvVar{
			Name:      prefix + "N",
			Value:     strconv.Itoa(replicas),
			ValueFrom: nil,
		})
		for j := 0; j < replicas; j++ {
			c.Env = append(c.Env, corev1.EnvVar{
				Name:      prefix + strconv.Itoa(j) + "_HOSTNAME",
				Value:     sts.Name + "-" + strconv.Itoa(j),
				ValueFrom: nil,
			})
		}
	}
	return &sts, nil
}

func buildProbeContainers(reqCtx intctrlutil.RequestCtx, params createParams) ([]corev1.Container, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("statefulset_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("statefulset_template.cue"))
	})
	if err != nil {
		return nil, err
	}
	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	probeContainerByte, err := cueValue.Lookup("probeContainer")
	if err != nil {
		return nil, err
	}
	probeContainers := []corev1.Container{}
	componentProbes := params.component.Probes
	reqCtx.Log.Info("probe", "settings", componentProbes)
	// if componentProbes.StatusProbe.Enable {
	//	container := corev1.Container{}
	//	if err = json.Unmarshal(probeContainerByte, &container); err != nil {
	//		return nil, err
	//	}

	//	container.Name = "kbprobe-statuscheck"
	//	probe := container.ReadinessProbe
	//	probe.Exec.Command = []string{"sh", "-c", "curl -X POST -H 'Content-Type: application/json' http://localhost:3501/v1.0/bindings/mtest  -d  '{\"operation\": \"statusCheck\", \"metadata\": {\"sql\" : \"\"}}'"}
	//	probe.PeriodSeconds = componentProbes.StatusProbe.PeriodSeconds
	//	probe.SuccessThreshold = componentProbes.StatusProbe.SuccessThreshold
	//	probe.FailureThreshold = componentProbes.StatusProbe.FailureThreshold
	//	probeContainers = append(probeContainers, container)
	// }

	// if componentProbes.RunningProbe.Enable {
	//	container := corev1.Container{}
	//	if err = json.Unmarshal(probeContainerByte, &container); err != nil {
	//		return nil, err
	//	}
	//	container.Name = "kbprobe-runningcheck"
	//	probe := container.ReadinessProbe
	//	probe.Exec.Command = []string{"sh", "-c", "curl -X POST -H 'Content-Type: application/json' http://localhost:3501/v1.0/bindings/mtest  -d  '{\"operation\": \"statusCheck\", \"metadata\": {\"sql\" : \"\"}}'"}
	//	//probe.HTTPGet.Path = "/"
	//	probe.PeriodSeconds = componentProbes.RunningProbe.PeriodSeconds
	//	probe.SuccessThreshold = componentProbes.RunningProbe.SuccessThreshold
	//	probe.FailureThreshold = componentProbes.RunningProbe.FailureThreshold
	//	probeContainers = append(probeContainers, container)
	// }

	if componentProbes.RoleChangedProbe.Enable {
		container := corev1.Container{}
		if err = json.Unmarshal(probeContainerByte, &container); err != nil {
			return nil, err
		}
		container.Name = "kbprobe-rolechangedcheck"
		probe := container.ReadinessProbe
		// probe.HTTPGet.Path = "/"
		// HACK: hardcoded - "http://localhost:3501/v1.0/bindings/mtest"
		// TODO: http port should be checked to avoid conflicts instead of hardcoded 3051
		probe.Exec.Command = []string{"curl", "-X", "POST", "-H", "Content-Type: application/json", "http://localhost:3501/v1.0/bindings/mtest", "-d", "{\"operation\": \"roleCheck\", \"metadata\": {\"sql\" : \"\"}}"}
		probe.PeriodSeconds = componentProbes.RoleChangedProbe.PeriodSeconds
		probe.SuccessThreshold = componentProbes.RoleChangedProbe.SuccessThreshold
		probe.FailureThreshold = componentProbes.RoleChangedProbe.FailureThreshold
		// probe.InitialDelaySeconds = 60
		probeContainers = append(probeContainers, container)
	}

	if len(probeContainers) >= 1 {
		container := &probeContainers[0]
		container.Image = viper.GetString("AGAMOTTO_IMAGE")
		container.ImagePullPolicy = corev1.PullPolicy(viper.GetString("AGAMOTTO_IMAGE_PULL_POLICY"))
		// HACK: hardcoded port values
		// TODO: ports should be checked to avoid conflicts instead of hardcoded values
		container.Command = []string{"probe", "--app-id", "batch-sdk",
			"--dapr-http-port", "3501",
			"--dapr-grpc-port", "54215",
			"--app-protocol", "http", "--components-path", "/config/components"}

		// set pod name and namespace, for role label updating inside pod
		podName := corev1.EnvVar{
			Name: "MY_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		}
		podNamespace := corev1.EnvVar{
			Name: "MY_POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		}
		container.Env = append(container.Env, podName, podNamespace)

		// HACK: hardcoded port values
		// TODO: ports should be checked to avoid conflicts instead of hardcoded values
		container.Ports = []corev1.ContainerPort{{
			ContainerPort: 3501,
			Name:          "probe-port",
			Protocol:      "TCP",
		}}
	}

	reqCtx.Log.Info("probe", "containers", probeContainers)
	return probeContainers, nil
}

// buildConsensusSet build on a stateful set
func buildConsensusSet(reqCtx intctrlutil.RequestCtx, params createParams) (*appsv1.StatefulSet, error) {
	sts, err := buildSts(reqCtx, params)
	if err != nil {
		return sts, err
	}

	sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
	return sts, err
}

func buildDeploy(params createParams) (*appsv1.Deployment, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("deployment_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("deployment_template.cue"))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	clusterStrByte, err := params.getCacheBytesValue("cluster", func() ([]byte, error) {
		return json.Marshal(params.cluster)
	})
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("cluster", clusterStrByte); err != nil {
		return nil, err
	}

	componentStrByte, err := json.Marshal(params.component)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("component", componentStrByte); err != nil {
		return nil, err
	}

	stsStrByte, err := cueValue.Lookup("deployment")
	if err != nil {
		return nil, err
	}

	deploy := appsv1.Deployment{}
	if err = json.Unmarshal(stsStrByte, &deploy); err != nil {
		return nil, err
	}

	stsStrByte = injectEnv(stsStrByte, dbaasPrefix+"_SECRET_NAME", params.cluster.Name)

	if err = json.Unmarshal(stsStrByte, &deploy); err != nil {
		return nil, err
	}

	// TODO: inject environment

	return &deploy, nil
}

func buildHeadlessService(params createParams, pod *corev1.Pod) (*corev1.Service, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("headless_service_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("headless_service_template.cue"))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)

	podStrByte, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}

	if err = cueValue.Fill("pod", podStrByte); err != nil {
		return nil, err
	}

	svcStrByte, err := cueValue.Lookup("service")
	if err != nil {
		return nil, err
	}
	svc := corev1.Service{}
	if err = json.Unmarshal(svcStrByte, &svc); err != nil {
		return nil, err
	}

	scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()
	if err = controllerutil.SetOwnerReference(params.cluster, &svc, scheme); err != nil {
		return nil, err
	}

	return &svc, nil
}

func buildPDB(params createParams) (*policyv1.PodDisruptionBudget, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("pdb_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("pdb_template.cue"))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)

	clusterStrByte, err := params.getCacheBytesValue("cluster", func() ([]byte, error) {
		return json.Marshal(params.cluster)
	})
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("cluster", clusterStrByte); err != nil {
		return nil, err
	}

	componentStrByte, err := json.Marshal(params.component)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("component", componentStrByte); err != nil {
		return nil, err
	}

	pdbStrByte, err := cueValue.Lookup("pdb")
	if err != nil {
		return nil, err
	}

	pdb := policyv1.PodDisruptionBudget{}
	if err = json.Unmarshal(pdbStrByte, &pdb); err != nil {
		return nil, err
	}

	return &pdb, nil
}

func injectEnv(strByte []byte, key string, value string) []byte {
	str := string(strByte)
	str = strings.ReplaceAll(str, "$("+key+")", value)
	return []byte(str)
}

// buildCfg generate volumes for PodTemplate, volumeMount for container, and configmap for config files
func buildCfg(params createParams, sts *appsv1.StatefulSet, ctx context.Context, cli client.Client) ([]client.Object, error) {
	// Need to merge configTemplateRef of AppVersion.Components[*].ConfigTemplateRefs and ClusterDefinition.Components[*].ConfigTemplateRefs
	tpls, err := params.getConfigTemplates()
	if err != nil {
		return nil, err
	}
	if len(tpls) == 0 {
		return nil, nil
	}

	clusterName := params.cluster.Name
	namespaceName := params.cluster.Namespace

	// New ConfigTemplateBuilder
	cfgTemplateBuilder := NewCfgTemplateBuilder(clusterName, namespaceName, params.cluster, params.appVersion)
	// Prepare built-in objects and built-in functions
	if err := cfgTemplateBuilder.InjectBuiltInObjectsAndFunctions(&sts.Spec.Template, tpls, params.component); err != nil {
		return nil, err
	}

	configs := make([]client.Object, 0, len(tpls))
	volumes := make(map[string]dbaasv1alpha1.ConfigTemplate, len(tpls))
	// TODO Support Update AppVersionRef of Cluster
	scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()
	for _, tpl := range tpls {
		// Check config cm already exists
		cmName := getInstanceCmName(sts, &tpl)
		volumes[cmName] = tpl
		isExist, err := isAlreadyExists(cmName, params.cluster.Namespace, ctx, cli)
		if err != nil {
			return nil, err
		}
		if isExist {
			continue
		}

		// Generate ConfigMap objects for config files
		configmap, err := generateConfigMapFromTpl(cfgTemplateBuilder, cmName, tpl, params, ctx, cli)
		if err != nil {
			return nil, err
		}

		// The owner of the configmap object is a cluster of users,
		// in order to manage the life cycle of configmap
		if err := controllerutil.SetOwnerReference(params.cluster, configmap, scheme); err != nil {
			return nil, err
		}
		configs = append(configs, configmap)
	}

	// Generate Pod Volumes for ConfigMap objects
	return configs, checkAndUpdatePodVolumes(sts, volumes)
}

func checkAndUpdatePodVolumes(sts *appsv1.StatefulSet, volumes map[string]dbaasv1alpha1.ConfigTemplate) error {
	podVolumes := make([]corev1.Volume, 0, len(volumes))
	for cmName, tpl := range volumes {
		// not cm volume
		volumeMounted := intctrlutil.GetVolumeMountName(podVolumes, cmName)
		// Update ConfigMap Volume
		if volumeMounted != nil {
			configMapVolume := volumeMounted.ConfigMap
			if configMapVolume == nil {
				return fmt.Errorf("mount volume[%s] type require ConfigMap: [%+v]", volumeMounted.Name, volumeMounted)
			}
			configMapVolume.Name = cmName
			continue
		}
		// Add New ConfigMap Volume
		podVolumes = append(podVolumes, corev1.Volume{
			Name: tpl.VolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			},
		})
	}
	// Update PodTemplate Volumes
	sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, podVolumes...)
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

// {{statefull.Name}}-{{appVersion.Name}}-{{tpl.Name}}-"config"
func getInstanceCmName(sts *appsv1.StatefulSet, tpl *dbaasv1alpha1.ConfigTemplate) string {
	return fmt.Sprintf("%s-%s-config", sts.GetName(), tpl.VolumeName)
}

// generateConfigMapFromTpl render config file by config template provided ISV
func generateConfigMapFromTpl(tplBuilder *ConfigTemplateBuilder, cmName string, tplCfg dbaasv1alpha1.ConfigTemplate, params createParams, ctx context.Context, cli client.Client) (*corev1.ConfigMap, error) {
	// Render config template by TplEngine
	// The template namespace must be the same as the ClusterDefinition namespace
	configs, err := processConfigMapTemplate(ctx, cli, tplBuilder, client.ObjectKey{
		Namespace: viper.GetString(cmNamespaceKey),
		Name:      tplCfg.Name,
	})
	if err != nil {
		return nil, err
	}

	// Using ConfigMap cue template render to configmap of config
	return generateConfigMapWithTemplate(configs, params, cmName, tplCfg.Name)
}

func generateConfigMapWithTemplate(configs map[string]string, params createParams, cmName, templateName string) (*corev1.ConfigMap, error) {

	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("config_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("config_template.cue"))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	// prepare cue data
	configMeta := map[string]map[string]string{
		"clusterDefinition": {
			"name": params.clusterDefinition.GetName(),
			"type": params.clusterDefinition.Spec.Type,
		},
		"cluster": {
			"name":      params.cluster.GetName(),
			"namespace": params.cluster.GetNamespace(),
		},
		"component": {
			"name":         params.component.Name,
			"type":         params.component.Type,
			"configName":   cmName,
			"templateName": templateName,
		},
	}
	configBytes, err := json.Marshal(configMeta)
	if err != nil {
		return nil, err
	}

	// Generate config files context by render cue template
	if err = cueValue.Fill("meta", configBytes); err != nil {
		return nil, err
	}

	configStrByte, err := cueValue.Lookup("config")
	if err != nil {
		return nil, err
	}

	cm := corev1.ConfigMap{}
	if err = json.Unmarshal(configStrByte, &cm); err != nil {
		return nil, err
	}

	// Update rendered config
	cm.Data = configs
	return &cm, nil
}

// processConfigMapTemplate Render config file using template engine
func processConfigMapTemplate(ctx context.Context, cli client.Client, tplBuilder *ConfigTemplateBuilder, cmKey client.ObjectKey) (map[string]string, error) {
	cmObj := &corev1.ConfigMap{}
	//  Require template configmap exist
	if err := cli.Get(ctx, cmKey, cmObj); err != nil {
		return nil, err
	}

	// TODO process invalid data: e.g empty data
	return tplBuilder.Render(cmObj.Data)
}
