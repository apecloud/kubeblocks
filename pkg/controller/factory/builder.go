/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	cfgcm "github.com/apecloud/kubeblocks/pkg/parameters/configmanager"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

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
		pvc.Labels[constant.KBAppInstanceTemplateLabelKey] = templateName
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
