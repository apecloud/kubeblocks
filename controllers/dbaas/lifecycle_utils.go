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
	"github.com/spf13/viper"
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
		MinAvailable:    clusterDefComp.MinAvailable,
		MaxAvailable:    clusterDefComp.MaxAvailable,
		DefaultReplicas: clusterDefComp.DefaultReplicas,
		Replicas:        clusterDefComp.DefaultReplicas,
		AntiAffinity:    clusterDefComp.AntiAffinity,
		ComponentType:   clusterDefComp.ComponentType,
		ConsensusSpec:   clusterDefComp.ConsensusSpec,
		PodSpec:         clusterDefComp.PodSpec,
		Service:         clusterDefComp.Service,
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
		sts, err := buildDeploy(reqCtx, *params)
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
		// end render config
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

func buildHeadlessSvcs(params createParams, sts *appsv1.StatefulSet) ([]client.Object, error) {
	stsPodLabels := sts.Spec.Template.Labels
	replicas := *sts.Spec.Replicas
	svcs := make([]client.Object, replicas)
	for i := 0; i < int(replicas); i++ {
		pod := &corev1.Pod{}
		pod.ObjectMeta.Name = fmt.Sprintf("%s-%d", sts.GetName(), i)
		pod.ObjectMeta.Namespace = sts.Namespace
		pod.ObjectMeta.Labels = map[string]string{
			statefulSetPodNameLabelKey:       pod.ObjectMeta.Name,
			intctrlutil.AppNameLabelKey:      stsPodLabels[intctrlutil.AppNameLabelKey],
			intctrlutil.AppInstanceLabelKey:  stsPodLabels[intctrlutil.AppInstanceLabelKey],
			intctrlutil.AppComponentLabelKey: stsPodLabels[intctrlutil.AppNameLabelKey],
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

	// update sts.spec.volumeClaimTemplates[].metadata.labels
	if len(sts.Spec.VolumeClaimTemplates) > 0 && len(sts.GetLabels()) > 0 {
		for _, vct := range sts.Spec.VolumeClaimTemplates {
			if vct.Labels == nil {
				vct.Labels = make(map[string]string)
				continue
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

// buildConsensusSet build on a stateful set
func buildConsensusSet(reqCtx intctrlutil.RequestCtx, params createParams) (*appsv1.StatefulSet, error) {
	sts, err := buildSts(reqCtx, params)
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

	stsStrByte = injectEnv(stsStrByte, dbaasPrefix+"_SECRET_NAME", params.cluster.Name)

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
