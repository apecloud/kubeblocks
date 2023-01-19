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
	"sort"
	"strconv"
	"strings"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/leaanthony/debme"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-password/password"
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

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/consensusset"
	cfgutil "github.com/apecloud/kubeblocks/controllers/dbaas/configuration"
	"github.com/apecloud/kubeblocks/controllers/dbaas/operations"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/configmap"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type createParams struct {
	clusterDefinition *dbaasv1alpha1.ClusterDefinition
	clusterVersion    *dbaasv1alpha1.ClusterVersion
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
	cacheCtx     = map[string]interface{}{}
)

func getCacheCUETplValue(key string, valueCreator func() (*intctrlutil.CUETpl, error)) (*intctrlutil.CUETpl, error) {
	vIf, ok := cacheCtx[key]
	if ok {
		return vIf.(*intctrlutil.CUETpl), nil
	}
	v, err := valueCreator()
	if err != nil {
		return nil, err
	}
	cacheCtx[key] = v
	return v, err
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

// mergeComponents generates a new Component object, which is a mixture of
// component-related configs from input Cluster, ClusterDef and ClusterVersion.
func mergeComponents(
	reqCtx intctrlutil.RequestCtx,
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition,
	clusterDefComp *dbaasv1alpha1.ClusterDefinitionComponent,
	clusterVersionComp *dbaasv1alpha1.ClusterVersionComponent,
	clusterComp *dbaasv1alpha1.ClusterComponent) *Component {
	if clusterDefComp == nil {
		return nil
	}

	clusterDefCompObj := clusterDefComp.DeepCopy()
	component := &Component{
		ClusterDefName:        clusterDef.Name,
		ClusterType:           clusterDef.Spec.Type,
		Name:                  clusterDefCompObj.TypeName, // initial name for the component will be same as TypeName
		Type:                  clusterDefCompObj.TypeName,
		CharacterType:         clusterDefCompObj.CharacterType,
		MinReplicas:           clusterDefCompObj.MinReplicas,
		MaxReplicas:           clusterDefCompObj.MaxReplicas,
		DefaultReplicas:       clusterDefCompObj.DefaultReplicas,
		Replicas:              clusterDefCompObj.DefaultReplicas,
		AntiAffinity:          clusterDefCompObj.AntiAffinity,
		ComponentType:         clusterDefCompObj.ComponentType,
		ConsensusSpec:         clusterDefCompObj.ConsensusSpec,
		PodSpec:               clusterDefCompObj.PodSpec,
		Service:               clusterDefCompObj.Service,
		Probes:                clusterDefCompObj.Probes,
		LogConfigs:            clusterDefCompObj.LogConfigs,
		HorizontalScalePolicy: clusterDefCompObj.HorizontalScalePolicy,
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

	if clusterDefCompObj.ConfigSpec != nil {
		component.ConfigTemplates = clusterDefCompObj.ConfigSpec.ConfigTemplateRefs
	}

	if clusterVersionComp != nil {
		component.ConfigTemplates = operations.MergeConfigTemplates(clusterVersionComp.ConfigTemplateRefs, component.ConfigTemplates)
		if clusterVersionComp.PodSpec != nil {
			for _, c := range clusterVersionComp.PodSpec.Containers {
				doContainerAttrOverride(c)
			}
		}
	}
	affinity := cluster.Spec.Affinity
	tolerations := cluster.Spec.Tolerations
	if clusterComp != nil {
		component.Name = clusterComp.Name // component name gets overrided
		component.EnabledLogs = clusterComp.EnabledLogs

		// user can scale in replicas to 0
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
	replacePlaceholderTokens(cluster, component)

	return component
}

func mergeComponentsList(reqCtx intctrlutil.RequestCtx,
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition,
	clusterDefCompList []dbaasv1alpha1.ClusterDefinitionComponent,
	clusterCompList []dbaasv1alpha1.ClusterComponent) []Component {
	var compList []Component
	for _, clusterDefComp := range clusterDefCompList {
		for _, clusterComp := range clusterCompList {
			if clusterComp.Type != clusterDefComp.TypeName {
				continue
			}
			comp := mergeComponents(reqCtx, cluster, clusterDef, &clusterDefComp, nil, &clusterComp)
			compList = append(compList, *comp)
		}
	}
	return compList
}

func getComponent(componentList []Component, name string) *Component {
	for _, comp := range componentList {
		if comp.Name == name {
			return &comp
		}
	}
	return nil
}

func replacePlaceholderTokens(cluster *dbaasv1alpha1.Cluster, component *Component) {
	namedValues := getEnvReplacementMapForConnCrential(cluster.GetName())

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

func createCluster(
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

	prepareComp := func(component *Component) error {
		iParams := params
		iParams.component = component
		return prepareComponentObjs(reqCtx, cli, &iParams)
	}

	for _, c := range clusterDefComps {
		typeName := c.TypeName
		clusterVersionComp := clusterVersionCompMap[typeName]
		clusterComps := clusterCompMap[typeName]
		for _, clusterComp := range clusterComps {
			if err := prepareComp(mergeComponents(reqCtx, cluster, clusterDefinition, &c, clusterVersionComp, &clusterComp)); err != nil {
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

	return createOrReplaceResources(reqCtx, cli, params.cluster, params.clusterDefinition, *params.applyObjs)
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
				return buildDeploy(reqCtx, *params)
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
			addLeaderSelectorLabels(svc, params.component)
		}
		*params.applyObjs = append(*params.applyObjs, svc)
	}

	return nil
}

// TODO multi roles with same accessMode support
func addLeaderSelectorLabels(service *corev1.Service, component *Component) {
	leader := component.ConsensusSpec.Leader
	if len(leader.Name) > 0 {
		service.Spec.Selector[intctrlutil.ConsensusSetRoleLabelKey] = leader.Name
	}
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
				isSnapshotAvailable(cli, ctx) {
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
		tempAnnotations := stsObj.Spec.Template.Annotations
		stsObj.Spec.Template = stsProto.Spec.Template
		// keep the original template annotations.
		// if annotations exist and are replaced, the statefulSet will be updated.
		if restartAnnotation, ok := tempAnnotations[intctrlutil.RestartAnnotationKey]; ok {
			if stsObj.Spec.Template.Annotations == nil {
				stsObj.Spec.Template.Annotations = map[string]string{}
			}
			stsObj.Spec.Template.Annotations[intctrlutil.RestartAnnotationKey] = restartAnnotation
		}
		stsObj.Spec.Replicas = stsProto.Spec.Replicas
		stsObj.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
		if err := cli.Update(ctx, stsObj); err != nil {
			return false, err
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
		deployObj.Spec = deployProto.Spec
		if err := cli.Update(ctx, deployObj); err != nil {
			return err
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

	for _, obj := range objs {
		logger.Info("create or update", "objs", obj)
		if err := controllerutil.SetOwnerReference(cluster, obj, scheme); err != nil {
			return false, err
		}
		if !controllerutil.ContainsFinalizer(obj, dbClusterFinalizerName) {
			controllerutil.AddFinalizer(obj, dbClusterFinalizerName)
		}
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

	return shouldRequeue, nil
}

func buildSvc(params createParams, headless bool) (*corev1.Service, error) {
	tplFile := "service_template.cue"
	if headless {
		tplFile = "headless_service_template.cue"
	}
	svc := corev1.Service{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":   params.cluster,
		"component": params.component,
	}, "service", &svc); err != nil {
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

	connCredential := corev1.Secret{}
	if err := buildFromCUE(tplFile, map[string]any{
		"clusterdefinition": params.clusterDefinition,
		"cluster":           params.cluster,
	}, "secret", &connCredential); err != nil {
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

	sts := appsv1.StatefulSet{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":   params.cluster,
		"component": params.component,
	}, "statefulset", &sts); err != nil {
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

	if err := processContainersInjection(reqCtx, params, envConfigName, &sts.Spec.Template.Spec); err != nil {
		return nil, err
	}
	return &sts, nil
}

func processContainersInjection(reqCtx intctrlutil.RequestCtx,
	params createParams,
	envConfigName string,
	podSpec *corev1.PodSpec) error {
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
func buildConsensusSet(reqCtx intctrlutil.RequestCtx,
	params createParams,
	envConfigName string) (*appsv1.StatefulSet, error) {
	sts, err := buildSts(reqCtx, params, envConfigName)
	if err != nil {
		return sts, err
	}

	sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
	return sts, err
}

func buildDeploy(reqCtx intctrlutil.RequestCtx, params createParams) (*appsv1.Deployment, error) {
	const tplFile = "deployment_template.cue"

	deploy := appsv1.Deployment{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":   params.cluster,
		"component": params.component,
	}, "deployment", &deploy); err != nil {
		return nil, err
	}

	if err := processContainersInjection(reqCtx, params, "", &deploy.Spec.Template.Spec); err != nil {
		return nil, err
	}
	return &deploy, nil
}

func buildPDB(params createParams) (*policyv1.PodDisruptionBudget, error) {
	const tplFile = "pdb_template.cue"
	pdb := policyv1.PodDisruptionBudget{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":   params.cluster,
		"component": params.component,
	}, "pdb", &pdb); err != nil {
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
	// Need to merge configTemplateRef of ClusterVersion.Components[*].ConfigTemplateRefs and
	// ClusterDefinition.Components[*].ConfigTemplateRefs
	tpls := params.component.ConfigTemplates
	if len(tpls) == 0 {
		return nil, nil
	}

	clusterName := params.cluster.Name
	namespaceName := params.cluster.Namespace

	// New ConfigTemplateBuilder
	cfgTemplateBuilder := newCfgTemplateBuilder(clusterName, namespaceName, params.cluster, params.clusterVersion)
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

	if container, err = buildCfgManagerContainer(params, managerSidecar); err != nil {
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

func buildEnvConfig(params createParams) (*corev1.ConfigMap, error) {
	const tplFile = "env_config_template.cue"

	prefix := dbaasPrefix + "_" + strings.ToUpper(params.component.Type) + "_"
	svcName := strings.Join([]string{params.cluster.Name, params.component.Name, "headless"}, "-")
	envData := map[string]string{}
	envData[prefix+"N"] = strconv.Itoa(int(params.component.Replicas))
	for j := 0; j < int(params.component.Replicas); j++ {
		envData[prefix+strconv.Itoa(j)+"_HOSTNAME"] = fmt.Sprintf("%s.%s", params.cluster.Name+"-"+params.component.Name+"-"+strconv.Itoa(j), svcName)
	}
	// TODO following code seems to be redundant with updateConsensusRoleInfo in consensus_set_utils.go
	// build consensus env from cluster.status
	if params.cluster.Status.Components != nil {
		if v, ok := params.cluster.Status.Components[params.component.Name]; ok {
			consensusSetStatus := v.ConsensusSetStatus
			if consensusSetStatus != nil {
				if consensusSetStatus.Leader.Pod != consensusset.DefaultPodName {
					envData[prefix+"LEADER"] = consensusSetStatus.Leader.Pod
				}

				followers := ""
				for _, follower := range consensusSetStatus.Followers {
					if follower.Pod == consensusset.DefaultPodName {
						continue
					}
					if len(followers) > 0 {
						followers += ","
					}
					followers += follower.Pod
				}
				envData[prefix+"FOLLOWERS"] = followers
			}
		}
	}

	config := corev1.ConfigMap{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":     params.cluster,
		"component":   params.component,
		"config.data": envData,
	}, "config", &config); err != nil {
		return nil, err
	}

	return &config, nil
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
	configs, err := processConfigMapTemplate(ctx, cli, tplBuilder, tplCfg)
	if err != nil {
		return nil, err
	}

	// Using ConfigMap cue template render to configmap of config
	return buildConfigMapWithTemplate(configs, params, cmName, tplCfg)
}

func buildConfigMapWithTemplate(
	configs map[string]string,
	params createParams,
	cmName string,
	tplCfg dbaasv1alpha1.ConfigTemplate) (*corev1.ConfigMap, error) {
	const tplFile = "config_template.cue"
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := getCacheCUETplValue(tplFile, func() (*intctrlutil.CUETpl, error) {
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
			"name":                  params.component.Name,
			"type":                  params.component.Type,
			"configName":            cmName,
			"templateName":          tplCfg.ConfigTplRef,
			"configConstraintsName": tplCfg.ConfigConstraintRef,
			"configTemplateName":    tplCfg.Name,
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

func buildCfgManagerContainer(params createParams, sidecarRenderedParam *cfgcm.ConfigManagerSidecar) (*corev1.Container, error) {
	const tplFile = "config_manager_sidecar.cue"
	cueFS, _ := debme.FS(CueTemplates, "cue")
	cueTpl, err := getCacheCUETplValue(tplFile, func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplFile))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	paramBytes, err := json.Marshal(sidecarRenderedParam)
	if err != nil {
		return nil, err
	}

	if err = cueValue.Fill("parameter", paramBytes); err != nil {
		return nil, err
	}

	containerStrByte, err := cueValue.Lookup("template")
	if err != nil {
		return nil, err
	}
	container := corev1.Container{}
	if err = json.Unmarshal(containerStrByte, &container); err != nil {
		return nil, err
	}
	return &container, nil
}

// processConfigMapTemplate Render config file using template engine
func processConfigMapTemplate(ctx context.Context, cli client.Client, tplBuilder *configTemplateBuilder, tplCfg dbaasv1alpha1.ConfigTemplate) (map[string]string, error) {
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
	configChecker := cfgcore.NewConfigValidator(&cfgTemplate.Spec)
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
	if err := configChecker.Validate(renderedCfg); err != nil {
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
		backupPolicy, err := buildBackupPolicy(sts, backupPolicyTemplate, backupKey)
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
		backup, err := buildBackup(sts, backupPolicyName, backupKey)
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

func buildBackupPolicy(sts *appsv1.StatefulSet,
	template *dataprotectionv1alpha1.BackupPolicyTemplate,
	backupKey types.NamespacedName) (*dataprotectionv1alpha1.BackupPolicy, error) {
	backupPolicy := dataprotectionv1alpha1.BackupPolicy{}
	if err := buildFromCUE("backup_policy_template.cue", map[string]any{
		"sts":        sts,
		"backup_key": backupKey,
		"template":   template.Name,
	}, "backup_policy", &backupPolicy); err != nil {
		return nil, err
	}

	return &backupPolicy, nil
}

func buildBackup(sts *appsv1.StatefulSet,
	backupPolicyName string,
	backupKey types.NamespacedName) (*dataprotectionv1alpha1.Backup, error) {
	backup := dataprotectionv1alpha1.Backup{}
	if err := buildFromCUE("backup_job_template.cue", map[string]any{
		"sts":                sts,
		"backup_policy_name": backupPolicyName,
		"backup_job_key":     backupKey,
	}, "backup_job", &backup); err != nil {
		return nil, err
	}

	return &backup, nil
}

func createPVCFromSnapshot(ctx context.Context,
	cli client.Client,
	sts *appsv1.StatefulSet,
	pvcKey types.NamespacedName,
	snapshotName string) error {
	pvc, err := buildPVCFromSnapshot(sts, pvcKey, snapshotName)
	if err != nil {
		return err
	}
	if err := cli.Create(ctx, pvc); err != nil {
		return intctrlutil.IgnoreIsAlreadyExists(err)
	}
	return nil
}

func buildPVCFromSnapshot(sts *appsv1.StatefulSet,
	pvcKey types.NamespacedName,
	snapshotName string) (*corev1.PersistentVolumeClaim, error) {

	pvc := corev1.PersistentVolumeClaim{}
	if err := buildFromCUE("pvc_template.cue", map[string]any{
		"sts":           sts,
		"pvc_key":       pvcKey,
		"snapshot_name": snapshotName,
	}, "pvc", &pvc); err != nil {
		return nil, err
	}

	return &pvc, nil
}

func buildVolumeSnapshot(snapshotKey types.NamespacedName,
	pvcName string,
	sts *appsv1.StatefulSet) (*snapshotv1.VolumeSnapshot, error) {
	snapshot := snapshotv1.VolumeSnapshot{}
	if err := buildFromCUE("snapshot_template.cue", map[string]any{
		"snapshot_key": snapshotKey,
		"pvc_name":     pvcName,
		"sts":          sts,
	}, "snapshot", &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
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
	component *Component) (bool, error) {
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
	component *Component) (bool, error) {
	ml := getBackupMatchingLabels(cluster.Name, component.Name)
	vsList := snapshotv1.VolumeSnapshotList{}
	if err := cli.List(ctx, &vsList, ml); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	if len(vsList.Items) == 0 || vsList.Items[0].Status == nil {
		return false, nil
	}
	return *vsList.Items[0].Status.ReadyToUse, nil
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
		snapshot, err := buildVolumeSnapshot(snapshotKey, pvcName, stsObj)
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
	component *Component,
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
		return createPVCFromSnapshot(ctx, cli, stsObj, pvcKey, vsList.Items[0].Name)
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
	component *Component) error {
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

func buildCronJob(pvcKey types.NamespacedName,
	schedule string,
	sts *appsv1.StatefulSet) (*v1.CronJob, error) {

	serviceAccount := viper.GetString("KUBEBLOCKS_SERVICE_ACCOUNT")
	if len(serviceAccount) == 0 {
		serviceAccount = "kubeblocks"
	}

	cronJob := v1.CronJob{}
	if err := buildFromCUE("delete_pvc_cron_job_template.cue", map[string]any{
		"pvc":                   pvcKey,
		"cronjob.spec.schedule": schedule,
		"cronjob.spec.jobTemplate.spec.template.spec.serviceAccount": serviceAccount,
		"sts": sts,
	}, "cronjob", &cronJob); err != nil {
		return nil, err
	}

	return &cronJob, nil
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
	cronJob, err := buildCronJob(pvcKey, schedule, stsObj)
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
	component *Component,
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

func buildFromCUE(tplName string, fillMap map[string]any, lookupKey string, target any) error {
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := getCacheCUETplValue(tplName, func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplName))
	})
	if err != nil {
		return err
	}
	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)

	for k, v := range fillMap {
		if err := cueValue.FillObj(k, v); err != nil {
			return err
		}
	}

	b, err := cueValue.Lookup(lookupKey)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(b, target); err != nil {
		return err
	}

	return nil
}

func getBackupMatchingLabels(clusterName string, componentName string) client.MatchingLabels {
	return client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey:  clusterName,
		intctrlutil.AppComponentLabelKey: componentName,
		intctrlutil.AppCreatedByLabelKey: intctrlutil.AppName,
	}
}
