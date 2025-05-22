/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package factory

import (
	"encoding/json"
	"path/filepath"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// BuildInstanceSet builds an InstanceSet object from SynthesizedComponent.
func BuildInstanceSet(synthesizedComp *component.SynthesizedComponent, componentDef *kbappsv1.ComponentDefinition) (*workloads.InstanceSet, error) {
	var (
		compDefName = synthesizedComp.CompDefName
		namespace   = synthesizedComp.Namespace
		clusterName = synthesizedComp.ClusterName
		compName    = synthesizedComp.Name
	)

	itsName := constant.GenerateWorkloadNamePattern(clusterName, compName)
	itsBuilder := builder.NewInstanceSetBuilder(namespace, itsName).
		// priority: static < dynamic < built-in
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddLabelsInMap(synthesizedComp.DynamicLabels).
		AddLabelsInMap(constant.GetCompLabels(clusterName, compName)).
		AddAnnotations(constant.KubeBlocksGenerationKey, synthesizedComp.Generation).
		AddAnnotations(constant.CRDAPIVersionAnnotationKey, workloads.GroupVersion.String()).
		AddAnnotationsInMap(map[string]string{
			constant.AppComponentLabelKey:   compDefName,
			constant.KBAppServiceVersionKey: synthesizedComp.ServiceVersion,
		}).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		AddAnnotationsInMap(getMonitorAnnotations(synthesizedComp, componentDef)).
		SetTemplate(getTemplate(synthesizedComp)).
		SetSelectorMatchLabel(constant.GetCompLabels(clusterName, compName)).
		SetReplicas(synthesizedComp.Replicas).
		SetVolumeClaimTemplates(getVolumeClaimTemplates(synthesizedComp)...).
		SetPVCRetentionPolicy(&synthesizedComp.PVCRetentionPolicy).
		SetMinReadySeconds(synthesizedComp.MinReadySeconds).
		SetInstances(getInstanceTemplates(synthesizedComp.Instances)).
		SetOfflineInstances(synthesizedComp.OfflineInstances).
		SetRoles(synthesizedComp.Roles).
		SetPodManagementPolicy(getPodManagementPolicy(synthesizedComp)).
		SetParallelPodManagementConcurrency(getParallelPodManagementConcurrency(synthesizedComp)).
		SetPodUpdatePolicy(getPodUpdatePolicy(synthesizedComp)).
		SetInstanceUpdateStrategy(getInstanceUpdateStrategy(synthesizedComp)).
		SetMemberUpdateStrategy(getMemberUpdateStrategy(synthesizedComp)).
		SetLifecycleActions(synthesizedComp.LifecycleActions).
		SetTemplateVars(synthesizedComp.TemplateVars)

	if common.IsCompactMode(synthesizedComp.Annotations) {
		itsBuilder.AddAnnotations(constant.FeatureReconciliationInCompactModeAnnotationKey,
			synthesizedComp.Annotations[constant.FeatureReconciliationInCompactModeAnnotationKey])
	}

	itsObj := itsBuilder.GetObject()

	// update its.spec.volumeClaimTemplates[].metadata.labels
	// TODO(xingran): synthesizedComp.VolumeTypes has been removed, and the following code needs to be refactored.
	if len(itsObj.Spec.VolumeClaimTemplates) > 0 && len(itsObj.GetLabels()) > 0 {
		for index, vct := range itsObj.Spec.VolumeClaimTemplates {
			BuildPersistentVolumeClaimLabels(synthesizedComp, &vct, vct.Name, "")
			itsObj.Spec.VolumeClaimTemplates[index] = vct
		}
	}

	setDefaultResourceLimits(itsObj)

	return itsObj, nil
}

func getTemplate(synthesizedComp *component.SynthesizedComponent) corev1.PodTemplateSpec {
	podBuilder := builder.NewPodBuilder("", "").
		// priority: static < dynamic < built-in
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddLabelsInMap(synthesizedComp.DynamicLabels).
		AddLabelsInMap(constant.GetCompLabels(synthesizedComp.ClusterName, synthesizedComp.Name, synthesizedComp.Labels)).
		AddLabels(constant.KBAppReleasePhaseKey, constant.ReleasePhaseStable).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		AddAnnotationsInMap(synthesizedComp.DynamicAnnotations)
	return corev1.PodTemplateSpec{
		ObjectMeta: podBuilder.GetObject().ObjectMeta,
		Spec:       *synthesizedComp.PodSpec.DeepCopy(),
	}
}

func getVolumeClaimTemplates(synthesizedComp *component.SynthesizedComponent) []corev1.PersistentVolumeClaim {
	pvc := func(vct corev1.PersistentVolumeClaimTemplate) corev1.PersistentVolumeClaim {
		return corev1.PersistentVolumeClaim{
			ObjectMeta: vct.ObjectMeta,
			Spec:       vct.Spec,
		}
	}

	var vcts []corev1.PersistentVolumeClaim
	for _, vct := range synthesizedComp.VolumeClaimTemplates {
		// priority: static < dynamic < built-in
		intctrlutil.MergeMetadataMapInplace(synthesizedComp.StaticLabels, &vct.ObjectMeta.Labels)
		intctrlutil.MergeMetadataMapInplace(synthesizedComp.StaticAnnotations, &vct.ObjectMeta.Annotations)
		intctrlutil.MergeMetadataMapInplace(synthesizedComp.DynamicLabels, &vct.ObjectMeta.Labels)
		intctrlutil.MergeMetadataMapInplace(synthesizedComp.DynamicAnnotations, &vct.ObjectMeta.Annotations)
		vcts = append(vcts, pvc(vct))
	}
	return vcts
}

func getInstanceTemplates(instances []kbappsv1.InstanceTemplate) []workloads.InstanceTemplate {
	if instances == nil {
		return nil
	}
	instanceTemplates := make([]workloads.InstanceTemplate, len(instances))
	for i := range instances {
		instanceTemplates[i] = workloads.InstanceTemplate{
			Name:             instances[i].Name,
			Replicas:         instances[i].Replicas,
			Ordinals:         instances[i].Ordinals,
			Annotations:      instances[i].Annotations,
			Labels:           instances[i].Labels,
			SchedulingPolicy: instances[i].SchedulingPolicy,
			Resources:        instances[i].Resources,
			Env:              instances[i].Env,
		}
	}
	return instanceTemplates
}

func getPodManagementPolicy(synthesizedComp *component.SynthesizedComponent) appsv1.PodManagementPolicyType {
	if synthesizedComp.PodManagementPolicy != nil {
		return *synthesizedComp.PodManagementPolicy
	}
	return appsv1.OrderedReadyPodManagement // default value
}

func getParallelPodManagementConcurrency(synthesizedComp *component.SynthesizedComponent) *intstr.IntOrString {
	if synthesizedComp.ParallelPodManagementConcurrency != nil {
		return synthesizedComp.ParallelPodManagementConcurrency
	}
	return &intstr.IntOrString{Type: intstr.String, StrVal: "100%"} // default value
}

func getPodUpdatePolicy(synthesizedComp *component.SynthesizedComponent) workloads.PodUpdatePolicyType {
	if synthesizedComp.PodUpdatePolicy != nil {
		return *synthesizedComp.PodUpdatePolicy
	}
	return kbappsv1.PreferInPlacePodUpdatePolicyType // default value
}

func getInstanceUpdateStrategy(synthesizedComp *component.SynthesizedComponent) *workloads.InstanceUpdateStrategy {
	// TODO: on-delete if the member update strategy is not null?
	return synthesizedComp.InstanceUpdateStrategy
}

func getMemberUpdateStrategy(synthesizedComp *component.SynthesizedComponent) *workloads.MemberUpdateStrategy {
	if synthesizedComp.UpdateStrategy != nil {
		return (*workloads.MemberUpdateStrategy)(synthesizedComp.UpdateStrategy)
	}
	return ptr.To(workloads.SerialUpdateStrategy)
}

// getMonitorAnnotations returns the annotations for the monitor.
func getMonitorAnnotations(synthesizedComp *component.SynthesizedComponent, componentDef *kbappsv1.ComponentDefinition) map[string]string {
	if synthesizedComp.DisableExporter == nil || *synthesizedComp.DisableExporter || componentDef == nil {
		return nil
	}

	exporter := component.GetExporter(componentDef.Spec)
	if exporter == nil {
		return nil
	}

	// Node: If it is an old addon, containerName may be empty.
	container := getBuiltinContainer(synthesizedComp, exporter.ContainerName)
	if container == nil && exporter.ScrapePort == "" && exporter.TargetPort == nil {
		klog.Warningf("invalid exporter port and ignore for component: %s, componentDef: %s", synthesizedComp.Name, componentDef.Name)
		return nil
	}
	return instanceset.AddAnnotationScope(instanceset.HeadlessServiceScope, common.GetScrapeAnnotations(*exporter, container))
}

func getBuiltinContainer(synthesizedComp *component.SynthesizedComponent, containerName string) *corev1.Container {
	containers := synthesizedComp.PodSpec.Containers
	for i := range containers {
		if containers[i].Name == containerName {
			return &containers[i]
		}
	}
	return nil
}

func setDefaultResourceLimits(its *workloads.InstanceSet) {
	for _, cc := range []*[]corev1.Container{&its.Spec.Template.Spec.Containers, &its.Spec.Template.Spec.InitContainers} {
		for i := range *cc {
			intctrlutil.InjectZeroResourcesLimitsIfEmpty(&(*cc)[i])
		}
	}
}

// BuildPersistentVolumeClaimLabels builds a pvc name label, and synchronize the labels from component to pvc.
func BuildPersistentVolumeClaimLabels(component *component.SynthesizedComponent, pvc *corev1.PersistentVolumeClaim,
	pvcTplName, templateName string) {
	// strict args checking.
	if pvc == nil || component == nil {
		return
	}
	if pvc.Labels == nil {
		pvc.Labels = make(map[string]string)
	}
	pvc.Labels[constant.VolumeClaimTemplateNameLabelKey] = pvcTplName
	if templateName != "" {
		pvc.Labels[constant.KBAppComponentInstanceTemplateLabelKey] = templateName
	}
}

// GetRestorePassword gets restore password if exists during recovery.
func GetRestorePassword(synthesizedComp *component.SynthesizedComponent) string {
	valueString := synthesizedComp.Annotations[constant.RestoreFromBackupAnnotationKey]
	if len(valueString) == 0 {
		return ""
	}
	backupMap := map[string]map[string]string{}
	err := json.Unmarshal([]byte(valueString), &backupMap)
	if err != nil {
		return ""
	}
	backupSource, ok := backupMap[synthesizedComp.Name]
	if !ok {
		return ""
	}
	password, ok := backupSource[constant.ConnectionPassword]
	if !ok {
		return ""
	}
	e := intctrlutil.NewEncryptor(viper.GetString(constant.CfgKeyDPEncryptionKey))
	password, _ = e.Decrypt([]byte(password))
	return password
}

// TODO: add dynamicLabels and dynamicAnnotations by @zhangtao

func BuildConfigMapWithTemplate(cluster *kbappsv1.Cluster,
	synthesizedComp *component.SynthesizedComponent,
	configs map[string]string,
	cmName string,
	configTemplateSpec kbappsv1.ComponentFileTemplate) *corev1.ConfigMap {
	return builder.NewConfigMapBuilder(cluster.Namespace, cmName).
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddLabelsInMap(constant.GetCompLabels(cluster.Name, synthesizedComp.Name)).
		AddLabels(constant.CMConfigurationTypeLabelKey, constant.ConfigInstanceType).
		AddLabels(constant.CMTemplateNameLabelKey, configTemplateSpec.Template).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		AddAnnotations(constant.DisableUpgradeInsConfigurationAnnotationKey, strconv.FormatBool(false)).
		SetData(configs).
		GetObject()
}

func BuildCfgManagerContainer(sidecarRenderedParam *cfgcm.CfgManagerBuildParams) (*corev1.Container, error) {
	var env []corev1.EnvVar
	env = append(env, corev1.EnvVar{
		Name: "CONFIG_MANAGER_POD_IP",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				APIVersion: "v1",
				FieldPath:  "status.podIP",
			},
		},
	})
	containerBuilder := builder.NewContainerBuilder(sidecarRenderedParam.ManagerName).
		AddCommands("env").
		AddArgs("PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$(TOOLS_PATH)").
		AddArgs(getSidecarBinaryPath(sidecarRenderedParam)).
		AddArgs(sidecarRenderedParam.Args...).
		AddEnv(env...).
		AddPorts(corev1.ContainerPort{
			Name:          constant.ConfigManagerPortName,
			ContainerPort: sidecarRenderedParam.ContainerPort,
			Protocol:      "TCP",
		}).
		SetImage(sidecarRenderedParam.Image).
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddVolumeMounts(sidecarRenderedParam.Volumes...)
	if sidecarRenderedParam.ShareProcessNamespace {
		user := int64(0)
		containerBuilder.SetSecurityContext(corev1.SecurityContext{
			RunAsUser: &user,
		})
	}
	return containerBuilder.GetObject(), nil
}

func getSidecarBinaryPath(buildParams *cfgcm.CfgManagerBuildParams) string {
	if buildParams.ConfigManagerReloadPath != "" {
		return buildParams.ConfigManagerReloadPath
	}
	return constant.ConfigManagerToolPath
}

func BuildCfgManagerToolsContainer(sidecarRenderedParam *cfgcm.CfgManagerBuildParams, toolsMetas []parametersv1alpha1.ToolConfig, toolsMap map[string]cfgcm.ConfigSpecMeta) ([]corev1.Container, error) {
	toolContainers := make([]corev1.Container, 0, len(toolsMetas))
	for _, toolConfig := range toolsMetas {
		toolContainerBuilder := builder.NewContainerBuilder(toolConfig.Name).
			AddCommands(toolConfig.Command...).
			SetImagePullPolicy(corev1.PullIfNotPresent).
			AddVolumeMounts(sidecarRenderedParam.Volumes...)
		if len(toolConfig.Image) > 0 {
			toolContainerBuilder.SetImage(toolConfig.Image)
		}
		toolContainers = append(toolContainers, *toolContainerBuilder.GetObject())
	}
	for i := range toolContainers {
		container := &toolContainers[i]
		if meta, ok := toolsMap[container.Name]; ok {
			setToolsScriptsPath(container, meta)
		}
	}
	return toolContainers, nil
}

func setToolsScriptsPath(container *corev1.Container, meta cfgcm.ConfigSpecMeta) {
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  cfgcm.KBTOOLSScriptsPathEnv,
		Value: filepath.Join(cfgcm.KBScriptVolumePath, meta.ConfigSpec.Name),
	})
}

func BuildServiceAccount(synthesizedComp *component.SynthesizedComponent, saName string) *corev1.ServiceAccount {
	return builder.NewServiceAccountBuilder(synthesizedComp.Namespace, saName).
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddLabelsInMap(constant.GetCompLabels(synthesizedComp.ClusterName, synthesizedComp.Name)).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		SetImagePullSecrets(intctrlutil.BuildImagePullSecrets()).
		GetObject()
}

func BuildRoleBinding(synthesizedComp *component.SynthesizedComponent, name string, roleRef *rbacv1.RoleRef, saName string) *rbacv1.RoleBinding {
	return builder.NewRoleBindingBuilder(synthesizedComp.Namespace, name).
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddLabelsInMap(constant.GetCompLabels(synthesizedComp.ClusterName, synthesizedComp.Name)).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		SetRoleRef(*roleRef).
		AddSubjects(rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Namespace: synthesizedComp.Namespace,
			Name:      saName,
		}).
		GetObject()
}

func BuildRole(synthesizedComp *component.SynthesizedComponent, cmpd *kbappsv1.ComponentDefinition) *rbacv1.Role {
	rules := cmpd.Spec.PolicyRules
	if len(rules) == 0 {
		return nil
	}
	return builder.NewRoleBuilder(synthesizedComp.Namespace, constant.GenerateDefaultRoleName(cmpd.Name)).
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddLabelsInMap(constant.GetCompLabels(synthesizedComp.ClusterName, synthesizedComp.Name)).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		AddPolicyRules(rules).
		GetObject()
}
