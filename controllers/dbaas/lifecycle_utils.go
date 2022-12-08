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

	"github.com/leaanthony/debme"
	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// mergeConfigTemplates merge AppVersion.Components[*].ConfigTemplateRefs and ClusterDefinition.Components[*].ConfigTemplateRefs
func mergeConfigTemplates(appVersionTpl []dbaasv1alpha1.ConfigTemplate, cdTpl []dbaasv1alpha1.ConfigTemplate) []dbaasv1alpha1.ConfigTemplate {
	if len(appVersionTpl) == 0 {
		return cdTpl
	}

	if len(cdTpl) == 0 {
		return appVersionTpl
	}

	mergedCfgTpl := make([]dbaasv1alpha1.ConfigTemplate, 0, len(appVersionTpl)+len(cdTpl))
	mergedTplMap := make(map[string]struct{}, cap(mergedCfgTpl))

	for i := range appVersionTpl {
		if _, ok := (mergedTplMap)[appVersionTpl[i].VolumeName]; ok {
			// TODO: following error should be checked in validation webhook and record Warning event
			// return nil, fmt.Errorf("ConfigTemplate require not same volumeName [%s]", appVersionTpl[i].Name)
			continue
		}
		mergedCfgTpl = append(mergedCfgTpl, appVersionTpl[i])
		mergedTplMap[appVersionTpl[i].VolumeName] = struct{}{}
	}

	for i := range cdTpl {
		// AppVersion replace clusterDefinition
		if _, ok := (mergedTplMap)[cdTpl[i].VolumeName]; ok {
			continue
		}
		mergedCfgTpl = append(mergedCfgTpl, cdTpl[i])
		mergedTplMap[cdTpl[i].VolumeName] = struct{}{}
	}

	return mergedCfgTpl
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
	component.Monitor = &MonitorConfig{
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
		component.Monitor = &MonitorConfig{
			Enable:     true,
			ScrapePath: monitorConfig.Exporter.ScrapePath,
			ScrapePort: monitorConfig.Exporter.ScrapePort,
		}
		return
	}

	characterType := clusterDefComp.CharacterType
	if !isWellKnownCharacterType(characterType) {
		disableMonitor(component)
		return
	}

	switch characterType {
	case kMysql:
		err := wellKnownCharacterTypeFunc[kMysql](cluster, component)
		if err != nil {
			disableMonitor(component)
		}
	default:
		disableMonitor(component)
	}
}

func mergeComponents(
	reqCtx intctrlutil.RequestCtx,
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition,
	clusterDefComp *dbaasv1alpha1.ClusterDefinitionComponent,
	appVerComp *dbaasv1alpha1.AppVersionComponent,
	clusterComp *dbaasv1alpha1.ClusterComponent) *Component {
	if clusterDefComp == nil {
		return nil
	}

	clusterDefCompObj := clusterDefComp.DeepCopy()
	component := &Component{
		ClusterDefName:  clusterDef.Name,
		ClusterType:     clusterDef.Spec.Type,
		Name:            clusterDefCompObj.TypeName, // initial name for the component will be same as TypeName
		Type:            clusterDefCompObj.TypeName,
		CharacterType:   clusterDefCompObj.CharacterType,
		MinReplicas:     clusterDefCompObj.MinReplicas,
		MaxReplicas:     clusterDefCompObj.MaxReplicas,
		DefaultReplicas: clusterDefCompObj.DefaultReplicas,
		Replicas:        clusterDefCompObj.DefaultReplicas,
		AntiAffinity:    clusterDefCompObj.AntiAffinity,
		ComponentType:   clusterDefCompObj.ComponentType,
		ConsensusSpec:   clusterDefCompObj.ConsensusSpec,
		PodSpec:         clusterDefCompObj.PodSpec,
		Service:         clusterDefCompObj.Service,
		Probes:          clusterDefCompObj.Probes,
		LogConfigs:      clusterDefCompObj.LogConfigs,
		ConfigTemplates: clusterDefCompObj.ConfigTemplateRefs,
	}

	doContainerAttrOverride := func(container corev1.Container) {
		i, c := getContainerByName(component.PodSpec.Containers, container.Name)
		if c == nil {
			component.PodSpec.Containers = append(component.PodSpec.Containers, container)
			return
		}
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
	}

	if appVerComp != nil {
		component.ConfigTemplates = mergeConfigTemplates(appVerComp.ConfigTemplateRefs, component.ConfigTemplates)
		if appVerComp.PodSpec != nil {
			for _, c := range appVerComp.PodSpec.Containers {
				doContainerAttrOverride(c)
			}
		}
	}
	affinity := cluster.Spec.Affinity
	tolerations := cluster.Spec.Tolerations
	if clusterComp != nil {
		component.Name = clusterComp.Name // component name gets overrided
		component.EnabledLogs = clusterComp.EnabledLogs

		// user can scale down replicas to 0
		if clusterComp.Replicas != nil {
			component.Replicas = *clusterComp.Replicas
		}

		if clusterComp.VolumeClaimTemplates != nil {
			component.VolumeClaimTemplates = toK8sVolumeClaimTemplates(clusterComp.VolumeClaimTemplates)
		}

		if clusterComp.Resources.Requests != nil || clusterComp.Resources.Limits != nil {
			component.PodSpec.Containers[0].Resources = clusterComp.Resources
		}

		if clusterComp.ServiceType != "" {
			if component.Service == nil {
				component.Service = &corev1.ServiceSpec{}
			}
			component.Service.Type = clusterComp.ServiceType
		}

		if clusterComp.Affinity != nil {
			affinity = clusterComp.Affinity
		}
		if len(clusterComp.Tolerations) != 0 {
			tolerations = clusterComp.Tolerations
		}
	}
	if affinity != nil {
		component.PodSpec.Affinity = buildPodAffinity(cluster, affinity, component)
		component.PodSpec.TopologySpreadConstraints = buildPodTopologySpreadConstraints(cluster, affinity, component)
	}
	if tolerations != nil {
		component.PodSpec.Tolerations = tolerations
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
	err := buildProbeContainers(reqCtx, component)
	if err != nil {
		reqCtx.Log.Error(err, "build probe container failed.")
	}
	replaceValues(cluster, component)

	return component
}

func replaceValues(cluster *dbaasv1alpha1.Cluster, component *Component) {
	namedValues := map[string]string{
		"$(CONN_CREDENTIAL_SECRET_NAME)": fmt.Sprintf("%s-conn-credential", cluster.GetName()),
	}

	// replace env[].valueFrom.secretKeyRef.name variables
	for _, cc := range [][]corev1.Container{component.PodSpec.InitContainers, component.PodSpec.Containers} {
		for _, c := range cc {
			for _, e := range c.Env {
				if e.ValueFrom == nil {
					continue
				}
				if e.ValueFrom.SecretKeyRef == nil {
					continue
				}
				secretRef := e.ValueFrom.SecretKeyRef
				for k, v := range namedValues {
					r := strings.Replace(secretRef.Name, k, v, 1)
					if r == secretRef.Name {
						continue
					}
					secretRef.Name = r
					break
				}
			}
		}
	}
}

func buildClusterCreationTasks(
	reqCtx intctrlutil.RequestCtx,
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

	clusterDefComp := clusterDefinition.Spec.Components
	clusterCompTypes := cluster.GetTypeMappingComponents()

	// add default component if unspecified in Cluster.spec.components
	for _, c := range clusterDefComp {
		if c.DefaultReplicas <= 0 {
			continue
		}
		if _, ok := clusterCompTypes[c.TypeName]; ok {
			continue
		}
		r := c.DefaultReplicas
		cluster.Spec.Components = append(cluster.Spec.Components, dbaasv1alpha1.ClusterComponent{
			Name:     c.TypeName,
			Type:     c.TypeName,
			Replicas: &r,
		})
	}

	appCompTypes := appVersion.GetTypeMappingComponents()
	clusterCompTypes = cluster.GetTypeMappingComponents()
	for _, c := range clusterDefComp {
		typeName := c.TypeName
		appVersionComponent := appCompTypes[typeName]
		clusterComps := clusterCompTypes[typeName]
		for _, clusterComp := range clusterComps {
			buildTask(mergeComponents(reqCtx, cluster, clusterDefinition, &c, appVersionComponent, &clusterComp))
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

	secret, err := buildConnCredential(*params)
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

	workloadProcessor := func(customSetup func(*corev1.ConfigMap) (client.Object, error)) error {
		envConfig, err := buildEnvConfig(*params)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, envConfig)

		workload, err := customSetup(envConfig)
		if err != nil {
			return err
		}

		defer func() {
			// workload object should be append last
			*params.applyObjs = append(*params.applyObjs, workload)
		}()

		svc, err := buildSvc(*params, true)
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
				return buildDeploy(reqCtx, *params, envConfig.Name)
			}); err != nil {
			return err
		}
	case dbaasv1alpha1.Stateful:
		if err := workloadProcessor(
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return buildSts(reqCtx, *params, envConfig.Name)
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
	}

	if needBuildPDB(params) {
		pdb, err := buildPDB(*params)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, pdb)
	}

	if params.component.Service != nil && len(params.component.Service.Ports) > 0 {
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
		// Once ClusterDefinition provider adjusts the ConfigTemplateRef field of CusterDefinition,
		// or provider modifies the wrong config file, it may cause the application cluster may fail.
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
	return nil
}

func buildSvc(params createParams, headless bool) (*corev1.Service, error) {
	tplFile := "service_template.cue"
	if headless {
		tplFile = "headless_service_template.cue"
	}
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := params.getCacheCUETplValue(tplFile, func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplFile))
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

func buildConnCredential(params createParams) (*corev1.Secret, error) {
	const tplFile = "conn_credential_template.cue"
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := params.getCacheCUETplValue(tplFile, func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplFile))
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

	connCredential := corev1.Secret{}
	if err = json.Unmarshal(secretStrByte, &connCredential); err != nil {
		return nil, err
	}

	if len(connCredential.StringData) == 0 {
		return &connCredential, nil
	}

	// REVIEW: perhaps handles value replacement at `func mergeComponents`
	replaceData := func(placeHolderMap map[string]string) {
		copyStringData := connCredential.DeepCopy().StringData
		for k, v := range copyStringData {
			for i, vv := range []string{k, v} {
				if !strings.HasPrefix(vv, "$(") {
					continue
				}
				for j, r := range placeHolderMap {
					replaced := strings.Replace(vv, j, r, 1)
					if replaced == vv {
						continue
					}
					// replace key
					if i == 0 {
						delete(connCredential.StringData, vv)
						k = replaced
					} else {
						v = replaced
					}
					break
				}
			}
			connCredential.StringData[k] = v
		}
	}

	// 1st pass replace primary placeholder
	m := map[string]string{
		"$(RANDOM_PASSWD)": randomString(8),
	}
	replaceData(m)

	// 2nd pass replace $(CONN_CREDENTIAL) holding values
	m = map[string]string{}

	for k, v := range connCredential.StringData {
		m[fmt.Sprintf("$(CONN_CREDENTIAL).%s", k)] = v
	}

	replaceData(m)
	return &connCredential, nil
}

func buildSts(reqCtx intctrlutil.RequestCtx, params createParams, envConfigName string) (*appsv1.StatefulSet, error) {
	const tplFile = "statefulset_template.cue"
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := params.getCacheCUETplValue(tplFile, func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplFile))
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
		for index, vct := range sts.Spec.VolumeClaimTemplates {
			if vct.Labels == nil {
				vct.Labels = make(map[string]string)
			}
			vct.Labels[intctrlutil.VolumeClaimTemplateNameLabelKey] = vct.Name
			for k, v := range sts.Labels {
				if _, ok := vct.Labels[k]; !ok {
					vct.Labels[k] = v
				}
			}
			sts.Spec.VolumeClaimTemplates[index] = vct
		}
	}

	if err = processContainersInjection(reqCtx, params, envConfigName, &sts.Spec.Template.Spec); err != nil {
		return nil, err
	}
	return &sts, nil
}

func processContainersInjection(reqCtx intctrlutil.RequestCtx, params createParams, envConfigName string, podSpec *corev1.PodSpec) error {
	for _, cc := range []*[]corev1.Container{
		&podSpec.Containers,
		&podSpec.InitContainers,
	} {
		for i := range *cc {
			injectEnvs(params, envConfigName, &(*cc)[i])
		}
	}
	return nil
}

func injectEnvs(params createParams, envConfigName string, c *corev1.Container) {
	// can not use map, it is unordered
	envFieldPathSlice := []envVar{
		{name: "_POD_NAME", fieldPath: "metadata.name"},
		{name: "_NAMESPACE", fieldPath: "metadata.namespace"},
		{name: "_SA_NAME", fieldPath: "spec.serviceAccountName"},
		{name: "_NODENAME", fieldPath: "spec.nodeName"},
		{name: "_HOSTIP", fieldPath: "status.hostIP"},
		{name: "_PODIP", fieldPath: "status.podIP"},
		{name: "_PODIPS", fieldPath: "status.podIPs"},
	}

	clusterEnv := []envVar{
		{name: "_CLUSTER_NAME", value: params.cluster.Name},
		{name: "_COMP_NAME", value: params.component.Name},
		{name: "_CLUSTER_COMP_NAME", value: params.cluster.Name + "-" + params.component.Name},
	}
	toInjectEnv := make([]corev1.EnvVar, 0, len(envFieldPathSlice)+len(c.Env))
	for _, v := range envFieldPathSlice {
		toInjectEnv = append(toInjectEnv, corev1.EnvVar{
			Name: dbaasPrefix + v.name,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: v.fieldPath,
				},
			},
		})
	}

	for _, v := range clusterEnv {
		toInjectEnv = append(toInjectEnv, corev1.EnvVar{
			Name:  dbaasPrefix + v.name,
			Value: v.value,
		})
	}

	// have injected variables placed at the front of the slice
	if c.Env == nil {
		c.Env = toInjectEnv
	} else {
		c.Env = append(toInjectEnv, c.Env...)
	}

	if envConfigName == "" {
		return
	}
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

func buildDeploy(reqCtx intctrlutil.RequestCtx, params createParams, envConfigName string) (*appsv1.Deployment, error) {
	const tplFile = "deployment_template.cue"
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := params.getCacheCUETplValue(tplFile, func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplFile))
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

	deployStrByte, err := cueValue.Lookup("deployment")
	if err != nil {
		return nil, err
	}

	deploy := appsv1.Deployment{}
	if err = json.Unmarshal(deployStrByte, &deploy); err != nil {
		return nil, err
	}

	if err = json.Unmarshal(deployStrByte, &deploy); err != nil {
		return nil, err
	}

	if err = processContainersInjection(reqCtx, params, "", &deploy.Spec.Template.Spec); err != nil {
		return nil, err
	}
	return &deploy, nil
}

func buildPDB(params createParams) (*policyv1.PodDisruptionBudget, error) {
	const tplFile = "pdb_template.cue"
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := params.getCacheCUETplValue(tplFile, func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplFile))
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
func buildCfg(params createParams,
	obj client.Object,
	podSpec *corev1.PodSpec,
	ctx context.Context,
	cli client.Client) ([]client.Object, error) {
	// Need to merge configTemplateRef of AppVersion.Components[*].ConfigTemplateRefs and
	// ClusterDefinition.Components[*].ConfigTemplateRefs
	tpls := params.component.ConfigTemplates
	if len(tpls) == 0 {
		return nil, nil
	}

	clusterName := params.cluster.Name
	namespaceName := params.cluster.Namespace

	// New ConfigTemplateBuilder
	cfgTemplateBuilder := newCfgTemplateBuilder(clusterName, namespaceName, params.cluster, params.appVersion)
	// Prepare built-in objects and built-in functions
	if err := cfgTemplateBuilder.injectBuiltInObjectsAndFunctions(podSpec, tpls, params.component); err != nil {
		return nil, err
	}

	configs := make([]client.Object, 0, len(tpls))
	volumes := make(map[string]dbaasv1alpha1.ConfigTemplate, len(tpls))
	// TODO Support Update AppVersionRef of Cluster
	scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()
	for _, tpl := range tpls {
		// Check config cm already exists
		cmName := getInstanceCMName(obj, &tpl)
		volumes[cmName] = tpl
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

	// Generate Pod Volumes for ConfigMap objects
	return configs, checkAndUpdatePodVolumes(podSpec, volumes)
}

func buildEnvConfig(params createParams) (*corev1.ConfigMap, error) {
	const tplFile = "env_config_template.cue"
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := params.getCacheCUETplValue(tplFile, func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplFile))
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
	if params.cluster.Status.Components != nil {
		if v, ok := params.cluster.Status.Components[params.component.Type]; ok {
			consensusSetStatus := v.ConsensusSetStatus
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

func checkAndUpdatePodVolumes(podSpec *corev1.PodSpec, volumes map[string]dbaasv1alpha1.ConfigTemplate) error {
	var (
		err        error
		podVolumes = podSpec.Volumes
	)

	// Update PodTemplate Volumes
	for cmName, tpl := range volumes {
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

// {{statefull.Name}}-{{appVersion.Name}}-{{tpl.Name}}-"config"
func getInstanceCMName(obj client.Object, tpl *dbaasv1alpha1.ConfigTemplate) string {
	return fmt.Sprintf("%s-%s", obj.GetName(), tpl.VolumeName)
}

// generateConfigMapFromTpl render config file by config template provided by provider.
func generateConfigMapFromTpl(tplBuilder *configTemplateBuilder, cmName string, tplCfg dbaasv1alpha1.ConfigTemplate, params createParams, ctx context.Context, cli client.Client) (*corev1.ConfigMap, error) {
	// Render config template by TplEngine
	// The template namespace must be the same as the ClusterDefinition namespace
	configs, err := processConfigMapTemplate(ctx, cli, tplBuilder, client.ObjectKey{
		Namespace: tplCfg.Namespace,
		Name:      tplCfg.Name,
	})
	if err != nil {
		return nil, err
	}

	// Using ConfigMap cue template render to configmap of config
	return generateConfigMapWithTemplate(configs, params, cmName, tplCfg.Name)
}

func generateConfigMapWithTemplate(configs map[string]string, params createParams, cmName, templateName string) (*corev1.ConfigMap, error) {
	const tplFile = "config_template.cue"
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := params.getCacheCUETplValue(tplFile, func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplFile))
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
func processConfigMapTemplate(ctx context.Context, cli client.Client, tplBuilder *configTemplateBuilder, cmKey client.ObjectKey) (map[string]string, error) {
	cmObj := &corev1.ConfigMap{}
	//  Require template configmap exist
	if err := cli.Get(ctx, cmKey, cmObj); err != nil {
		return nil, err
	}

	if len(cmObj.Data) == 0 {
		return map[string]string{}, nil
	}

	tplBuilder.setTplName(cmKey.Name)
	return tplBuilder.render(cmObj.Data)
}
