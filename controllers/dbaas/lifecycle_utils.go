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
	"fmt"
	"strconv"
	"strings"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/leaanthony/debme"
	"github.com/sethvargo/go-password/password"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	types2 "github.com/apecloud/kubeblocks/internal/dbctl/types"
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

func getClusterComponentsByType(components []dbaasv1alpha1.ClusterComponent, typeName string) []dbaasv1alpha1.ClusterComponent {
	comps := []dbaasv1alpha1.ClusterComponent{}
	for _, component := range components {
		if component.Type == typeName {
			comps = append(comps, component)
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
			intctrlutil.AppInstanceLabelKey:  clusterName,
			intctrlutil.AppComponentLabelKey: componentName,
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
		MinReplicas:     clusterDefComp.MinReplicas,
		MaxReplicas:     clusterDefComp.MaxReplicas,
		DefaultReplicas: clusterDefComp.DefaultReplicas,
		Replicas:        clusterDefComp.DefaultReplicas,
		AntiAffinity:    clusterDefComp.AntiAffinity,
		ComponentType:   clusterDefComp.ComponentType,
		ConsensusSpec:   clusterDefComp.ConsensusSpec,
		PodSpec:         clusterDefComp.PodSpec,
		Service:         clusterDefComp.Service,
		Probes:          clusterDefComp.Probes,
		LogConfigs:      clusterDefComp.LogConfigs,
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
		component.EnabledLogs = clusterComp.EnabledLogs

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
				buildTask(mergeComponents(cluster, clusterDefinition, &component, appVersionComponent, &clusterComp))
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

	if err := createOrReplaceResources(reqCtx, cli, params.cluster, params.clusterDefinition, *params.applyObjs); err != nil {
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
		return false
	}
	return existsPDBSpec(params.component.PodDisruptionBudgetSpec)
}

// TODO: @free6om handle config of all component types
func prepareComponentObjs(reqCtx intctrlutil.RequestCtx, cli client.Client, obj interface{}) error {
	params, ok := obj.(*createParams)
	if !ok {
		return fmt.Errorf("invalid arg")
	}

	switch params.component.ComponentType {
	case dbaasv1alpha1.Stateless:
		sts, err := buildDeploy(reqCtx, *params)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, sts)
	case dbaasv1alpha1.Stateful:
		envConfig, err := buildEnvConfig(*params)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, envConfig)

		sts, err := buildSts(reqCtx, *params, envConfig.Name)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, sts)

		svc, err := buildSvc(*params, true)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, svc)

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
		envConfig, err := buildEnvConfig(*params)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, envConfig)

		css, err := buildConsensusSet(reqCtx, *params, envConfig.Name)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, css)

		svc, err := buildSvc(*params, true)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, svc)

		// render config
		configs, err := buildCfg(*params, css, reqCtx.Ctx, cli)
		if err != nil {
			return err
		}
		if configs != nil {
			*params.applyObjs = append(*params.applyObjs, configs...)
		}
		// end render config
	}

	if needBuildPDB(params) {
		pdb, err := buildPDB(*params)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, pdb)
	}

	if params.component.Service.Ports != nil {
		svc, err := buildSvc(*params, false)
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
			service.Spec.Selector[intctrlutil.ConsensusSetRoleLabelKey] = member.Name
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
	clusterDef *dbaasv1alpha1.ClusterDefinition,
	objs []client.Object) error {
	ctx := reqCtx.Ctx
	logger := reqCtx.Log
	scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()
	for _, obj := range objs {
		logger.Info("create or update", "objs", obj)
		if err := controllerutil.SetOwnerReference(cluster, obj, scheme); err != nil {
			return err
		}
		if err := cli.Create(ctx, obj); err == nil {
			continue
		} else if !apierrors.IsAlreadyExists(err) {
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
		if cm, ok := obj.(*corev1.ConfigMap); ok {
			// if configmap is env config, should update
			if len(cm.Labels[intctrlutil.AppConfigTypeLabelKey]) > 0 {
				if err := cli.Update(ctx, cm); err != nil {
					return err
				}
			}
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
			// horizontal scaling
			if *stsObj.Spec.Replicas < *stsProto.Spec.Replicas {
				reqCtx.Recorder.Eventf(stsObj, corev1.EventTypeNormal, "HorizontalScale", "Start horizontal scale")
				var component dbaasv1alpha1.ClusterDefinitionComponent
				for _, comp := range clusterDef.Spec.Components {
					if comp.TypeName == stsObj.Labels[types2.ComponentLabelKey] {
						component = comp
					}
				}
				switch component.HorizontalScalePolicy {
				case dbaasv1alpha1.Backup:
					ml := client.MatchingLabels{
						clusterDefLabelKey: cluster.Spec.ClusterDefRef,
					}
					backupPolicyTemplateList := dataprotectionv1alpha1.BackupPolicyTemplateList{}
					if err := cli.List(ctx, &backupPolicyTemplateList, ml); err != nil {
						return err
					}
					if len(backupPolicyTemplateList.Items) > 0 {
						backupJobName := generateName(cluster.Name + "-scaling-")
						err := createBackup(ctx, cli, *stsObj, backupPolicyTemplateList.Items[0], backupJobName, cluster)
						if err != nil {
							return err
						}
						reqCtx.Recorder.Eventf(stsObj, corev1.EventTypeNormal, "BackupJobCreate", "Create backup job")
						for i := *stsObj.Spec.Replicas; i < *stsProto.Spec.Replicas; i++ {
							pvcKey := types.NamespacedName{
								Namespace: key.Namespace,
								Name:      fmt.Sprintf("%s-%s-%d", "data", stsObj.Name, i),
							}
							if err := createPVCFromSnapshot(ctx, cli, *stsObj, pvcKey, backupJobName); err != nil {
								return err
							}
						}
					} else {
						reqCtx.Recorder.Eventf(stsObj, corev1.EventTypeWarning, "HorizontalScaleFailed", "backup policy template not found for clusterdefinition %s", cluster.Spec.ClusterDefRef)
					}
				case dbaasv1alpha1.Snapshot:
					vsList := snapshotv1.VolumeSnapshotList{}
					// check volume snapshot available
					getVSErr := cli.List(ctx, &vsList)
					if getVSErr == nil && len(stsObj.Spec.VolumeClaimTemplates) > 0 {
						snapshotName := generateName(cluster.Name + "-scaling-")
						if len(stsObj.Spec.VolumeClaimTemplates) > 0 {
							reqCtx.Recorder.Eventf(stsObj, corev1.EventTypeNormal, "VolumeSnapshotCreate", "Create native volume snapshot")
							pvcName := strings.Join([]string{stsObj.Spec.VolumeClaimTemplates[0].Name, stsObj.Name, "0"}, "-")
							snapshot, err := buildVolumeSnapshot(snapshotName, pvcName, *stsObj)
							if err != nil {
								reqCtx.Recorder.Eventf(stsObj, corev1.EventTypeWarning, "HorizontalScaleFailed", err.Error())
								return err
							}
							if err := cli.Create(ctx, snapshot); err != nil {
								if !apierrors.IsAlreadyExists(err) {
									reqCtx.Recorder.Eventf(stsObj, corev1.EventTypeWarning, "HorizontalScaleFailed", err.Error())
									return err
								}
							}
							if err := controllerutil.SetOwnerReference(cluster, snapshot, scheme); err != nil {
								reqCtx.Recorder.Eventf(stsObj, corev1.EventTypeWarning, "HorizontalScaleFailed", err.Error())
								return err
							}
							for i := *stsObj.Spec.Replicas; i < *stsProto.Spec.Replicas; i++ {
								pvcKey := types.NamespacedName{
									Namespace: key.Namespace,
									Name:      fmt.Sprintf("%s-%s-%d", "data", stsObj.Name, i),
								}
								if err := createPVCFromSnapshot(ctx, cli, *stsObj, pvcKey, snapshotName); err != nil {
									reqCtx.Recorder.Eventf(stsObj, corev1.EventTypeWarning, "HorizontalScaleFailed", err.Error())
									return err
								}
							}
						}
					}
				case dbaasv1alpha1.ScaleNone:
					break
				}
			}
			tempAnnotations := stsObj.Spec.Template.Annotations
			stsObj.Spec.Template = stsProto.Spec.Template
			// keep the original template annotations.
			// if annotations exist and are replaced, the statefulSet will be updated
			stsObj.Spec.Template.Annotations = tempAnnotations
			stsObj.Spec.Replicas = stsProto.Spec.Replicas
			stsObj.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
			if err := cli.Update(ctx, stsObj); err != nil {
				return err
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
					var err error
					if err = cli.Get(ctx, pvcKey, pvc); err != nil {
						if apierrors.IsNotFound(err) {
							continue
						}
						if !apierrors.IsNotFound(err) {
							return err
						}
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
	return nil
}

func buildSvc(params createParams, headless bool) (*corev1.Service, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	svcTmpl := "service_template.cue"
	if headless {
		svcTmpl = "headless_service_template.cue"
	}

	cueTpl, err := params.getCacheCUETplValue(svcTmpl, func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(svcTmpl))
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

func randomString(length int) string {
	res, _ := password.Generate(length, 0, 0, false, false)
	return res
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

	if err = cueValue.FillRaw("secret.stringData.password", randomString(8)); err != nil {
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

func buildSts(reqCtx intctrlutil.RequestCtx, params createParams, envConfigName string) (*appsv1.StatefulSet, error) {
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

	if err = json.Unmarshal(stsStrByte, &sts); err != nil {
		return nil, err
	}

	// update sts.spec.volumeClaimTemplates[].metadata.labels
	if len(sts.Spec.VolumeClaimTemplates) > 0 && len(sts.GetLabels()) > 0 {
		for _, vct := range sts.Spec.VolumeClaimTemplates {
			if vct.Labels == nil {
				vct.Labels = make(map[string]string)
			}
			for k, v := range sts.Labels {
				if _, ok := vct.Labels[k]; !ok {
					vct.Labels[k] = v
				}
			}
		}
	}

	probeContainers, err := buildProbeContainers(reqCtx, params, sts.Spec.Template.Spec.Containers)
	if err != nil {
		return nil, err
	}
	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, probeContainers...)

	injectEnv := func(c *corev1.Container) {
		if c.Env == nil {
			c.Env = []corev1.EnvVar{}
		}
		c.Env = append(c.Env, corev1.EnvVar{
			Name: dbaasPrefix + "_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		})
		if c.EnvFrom == nil {
			c.EnvFrom = []corev1.EnvFromSource{}
		}
		c.EnvFrom = append(c.EnvFrom, corev1.EnvFromSource{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: envConfigName,
				},
			},
		})
		c.EnvFrom = append(c.EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: params.cluster.Name,
				},
			},
		})
	}

	for i := range sts.Spec.Template.Spec.Containers {
		injectEnv(&sts.Spec.Template.Spec.Containers[i])
	}
	for i := range sts.Spec.Template.Spec.InitContainers {
		injectEnv(&sts.Spec.Template.Spec.InitContainers[i])
	}

	return &sts, nil
}

// buildConsensusSet build on a stateful set
func buildConsensusSet(reqCtx intctrlutil.RequestCtx, params createParams, envConfigName string) (*appsv1.StatefulSet, error) {
	sts, err := buildSts(reqCtx, params, envConfigName)
	if err != nil {
		return sts, err
	}

	sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
	return sts, err
}

func buildDeploy(reqCtx intctrlutil.RequestCtx, params createParams) (*appsv1.Deployment, error) {
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

	if err = json.Unmarshal(stsStrByte, &deploy); err != nil {
		return nil, err
	}

	probeContainers, err := buildProbeContainers(reqCtx, params, deploy.Spec.Template.Spec.Containers)
	if err != nil {
		return nil, err
	}
	deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers, probeContainers...)

	// TODO: inject environment

	return &deploy, nil
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

func buildEnvConfig(params createParams) (*corev1.ConfigMap, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("env_config_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("env_config_template.cue"))
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

	prefix := dbaasPrefix + "_" + strings.ToUpper(params.component.Type) + "_"
	svcName := strings.Join([]string{params.cluster.Name, params.component.Name, "headless"}, "-")
	envData := map[string]string{}
	envData[prefix+"N"] = strconv.Itoa(int(params.component.Replicas))
	for j := 0; j < int(params.component.Replicas); j++ {
		envData[prefix+strconv.Itoa(j)+"_HOSTNAME"] = fmt.Sprintf("%s.%s", params.cluster.Name+"-"+params.component.Name+"-"+strconv.Itoa(j), svcName)
	}
	// build consensus env from cluster.status
	if params.cluster.Status.Components != nil && params.cluster.Status.Components[params.component.Type] != nil {
		consensusSetStatus := params.cluster.Status.Components[params.component.Type].ConsensusSetStatus
		if consensusSetStatus != nil {
			envData[prefix+"LEADER"] = consensusSetStatus.Leader.Pod
			followers := ""
			for _, follower := range consensusSetStatus.Followers {
				if len(followers) > 0 {
					followers += ","
				}
				followers += follower.Pod
			}
			envData[prefix+"FOLLOWERS"] = followers
		}
	}
	envDataStrByte, err := json.Marshal(envData)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("config.data", envDataStrByte); err != nil {
		return nil, err
	}

	configStrByte, err := cueValue.Lookup("config")
	if err != nil {
		return nil, err
	}

	config := corev1.ConfigMap{}
	if err = json.Unmarshal(configStrByte, &config); err != nil {
		return nil, err
	}

	return &config, nil
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

func createBackup(ctx context.Context, cli client.Client, sts appsv1.StatefulSet, backupPolicyTemplate dataprotectionv1alpha1.BackupPolicyTemplate, backupJobName string, cluster *dbaasv1alpha1.Cluster) error {
	backupPolicy, err := buildBackupPolicy(sts, backupPolicyTemplate)
	if err != nil {
		return err
	}
	if err := cli.Create(ctx, backupPolicy); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	backupJob, err := buildBackupJob(sts, backupJobName)
	if err != nil {
		return err
	}
	scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()
	if err := controllerutil.SetOwnerReference(cluster, backupJob, scheme); err != nil {
		return err
	}
	if err := cli.Create(ctx, backupJob); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func buildBackupPolicy(sts appsv1.StatefulSet, template dataprotectionv1alpha1.BackupPolicyTemplate) (*dataprotectionv1alpha1.BackupPolicy, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("backup_policy_template.cue"))
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	stsStrByte, err := json.Marshal(sts)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("sts", stsStrByte); err != nil {
		return nil, err
	}

	if err = cueValue.FillRaw("template", template.Name); err != nil {
		return nil, err
	}

	backupPolicyStrByte, err := cueValue.Lookup("backup_policy")
	if err != nil {
		return nil, err
	}

	backupPolicy := dataprotectionv1alpha1.BackupPolicy{}
	if err = json.Unmarshal(backupPolicyStrByte, &backupPolicy); err != nil {
		return nil, err
	}

	return &backupPolicy, nil
}

func buildBackupJob(sts appsv1.StatefulSet, backupJobName string) (*dataprotectionv1alpha1.BackupJob, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("backup_job_template.cue"))
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)

	stsStrByte, err := json.Marshal(sts)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("sts", stsStrByte); err != nil {
		return nil, err
	}

	if err = cueValue.FillRaw("backup_job_name", backupJobName); err != nil {
		return nil, err
	}

	backupJobStrByte, err := cueValue.Lookup("backup_job")
	if err != nil {
		return nil, err
	}

	backupJob := dataprotectionv1alpha1.BackupJob{}
	if err = json.Unmarshal(backupJobStrByte, &backupJob); err != nil {
		return nil, err
	}

	return &backupJob, nil
}

func createPVCFromSnapshot(ctx context.Context, cli client.Client, sts appsv1.StatefulSet, pvcKey types.NamespacedName, snapshotName string) error {
	pvc, err := buildPVCFromSnapshot(sts, pvcKey, snapshotName)
	if err != nil {
		return err
	}
	if err := cli.Create(ctx, pvc); err != nil {
		return err
	}
	return nil
}

func buildPVCFromSnapshot(sts appsv1.StatefulSet, pvcKey types.NamespacedName, snapshotName string) (*corev1.PersistentVolumeClaim, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("pvc_template.cue"))
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)

	stsStrByte, err := json.Marshal(sts)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("sts", stsStrByte); err != nil {
		return nil, err
	}

	pvcKeyStrByte, err := json.Marshal(pvcKey)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("pvc_key", pvcKeyStrByte); err != nil {
		return nil, err
	}

	if err := cueValue.FillRaw("snapshot_name", snapshotName); err != nil {
		return nil, err
	}

	pvcStrByte, err := cueValue.Lookup("pvc")
	if err != nil {
		return nil, err
	}

	pvc := corev1.PersistentVolumeClaim{}
	if err = json.Unmarshal(pvcStrByte, &pvc); err != nil {
		return nil, err
	}

	return &pvc, nil
}

const (
	maxNameLength          = 63
	randomLength           = 14
	MaxGeneratedNameLength = maxNameLength - randomLength
)

func generateName(base string) string {
	if len(base) > MaxGeneratedNameLength {
		base = base[:MaxGeneratedNameLength]
	}
	return fmt.Sprintf("%s%s", base, time.Now().Format("20060102150405"))
}

func prepareInjectEnvs(component *Component, cluster *dbaasv1alpha1.Cluster) []corev1.EnvVar {
	envs := []corev1.EnvVar{}
	if component == nil || cluster == nil {
		return envs
	}
	prefix := dbaasPrefix + "_" + strings.ToUpper(component.Type) + "_"
	svcName := strings.Join([]string{cluster.Name, component.Name, "headless"}, "-")
	envs = append(envs, corev1.EnvVar{
		Name: dbaasPrefix + "_POD_NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	})
	// inject component scope env
	envs = append(envs, corev1.EnvVar{
		Name:      prefix + "N",
		Value:     strconv.Itoa(int(component.Replicas)),
		ValueFrom: nil,
	})
	for j := 0; j < int(component.Replicas); j++ {
		envs = append(envs, corev1.EnvVar{
			Name:      prefix + strconv.Itoa(j) + "_HOSTNAME",
			Value:     fmt.Sprintf("%s.%s", cluster.Name+"-"+component.Name+"-"+strconv.Itoa(j), svcName),
			ValueFrom: nil,
		})
	}
	// inject consensusset role env
	if cluster.Status.Components != nil && cluster.Status.Components[component.Type] != nil {
		consensusSetStatus := cluster.Status.Components[component.Type].ConsensusSetStatus
		if consensusSetStatus != nil {
			envs = append(envs, corev1.EnvVar{
				Name:      prefix + "LEADER",
				Value:     consensusSetStatus.Leader.Pod,
				ValueFrom: nil,
			})
		}
	}
	return envs
}

func buildVolumeSnapshot(snapshotName string, pvcName string, sts appsv1.StatefulSet) (*snapshotv1.VolumeSnapshot, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("snapshot_template.cue"))
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)

	if err := cueValue.FillRaw("snapshot_name", snapshotName); err != nil {
		return nil, err
	}

	if err := cueValue.FillRaw("pvc_name", pvcName); err != nil {
		return nil, err
	}

	stsStrByte, err := json.Marshal(sts)
	if err != nil {
		return nil, err
	}
	if err := cueValue.Fill("sts", stsStrByte); err != nil {
		return nil, err
	}

	snapshotStrByte, err := cueValue.Lookup("snapshot")
	if err != nil {
		return nil, err
	}

	snapshot := snapshotv1.VolumeSnapshot{}
	if err = json.Unmarshal(snapshotStrByte, &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}
