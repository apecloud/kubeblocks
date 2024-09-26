/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// BuildInstanceSet builds an InstanceSet object from SynthesizedComponent.
func BuildInstanceSet(synthesizedComp *component.SynthesizedComponent, componentDef *appsv1.ComponentDefinition) (*workloads.InstanceSet, error) {
	var (
		compDefName = synthesizedComp.CompDefName
		namespace   = synthesizedComp.Namespace
		clusterName = synthesizedComp.ClusterName
		compName    = synthesizedComp.Name
	)

	podBuilder := builder.NewPodBuilder("", "").
		AddLabelsInMap(constant.GetCompLabels(clusterName, compName, synthesizedComp.Labels)).
		AddLabelsInMap(synthesizedComp.DynamicLabels).
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddAnnotationsInMap(synthesizedComp.DynamicAnnotations).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations)
	template := corev1.PodTemplateSpec{
		ObjectMeta: podBuilder.GetObject().ObjectMeta,
		Spec:       *synthesizedComp.PodSpec.DeepCopy(),
	}

	itsName := constant.GenerateWorkloadNamePattern(clusterName, compName)
	itsBuilder := builder.NewInstanceSetBuilder(namespace, itsName).
		AddLabelsInMap(constant.GetCompLabels(clusterName, compName)).
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddAnnotations(constant.KubeBlocksGenerationKey, synthesizedComp.Generation).
		AddAnnotationsInMap(map[string]string{
			constant.AppComponentLabelKey:   compDefName,
			constant.KBAppServiceVersionKey: synthesizedComp.ServiceVersion,
		}).
		AddAnnotationsInMap(getMonitorAnnotations(synthesizedComp, componentDef)).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		SetTemplate(template).
		AddMatchLabelsInMap(constant.GetCompLabels(clusterName, compName)).
		SetReplicas(synthesizedComp.Replicas).
		SetMinReadySeconds(synthesizedComp.MinReadySeconds)

	var vcts []corev1.PersistentVolumeClaim
	for _, vct := range synthesizedComp.VolumeClaimTemplates {
		intctrlutil.MergeMetadataMapInplace(synthesizedComp.DynamicLabels, &vct.ObjectMeta.Labels)
		intctrlutil.MergeMetadataMapInplace(synthesizedComp.DynamicAnnotations, &vct.ObjectMeta.Annotations)
		vcts = append(vcts, vctToPVC(vct))
	}
	itsBuilder.SetVolumeClaimTemplates(vcts...)

	if common.IsCompactMode(synthesizedComp.Annotations) {
		itsBuilder.AddAnnotations(constant.FeatureReconciliationInCompactModeAnnotationKey,
			synthesizedComp.Annotations[constant.FeatureReconciliationInCompactModeAnnotationKey])
	}

	// convert componentDef attributes to workload attributes. including service, credential, roles, roleProbe, membershipReconfiguration, memberUpdateStrategy, etc.
	itsObj, err := component.BuildWorkloadFrom(synthesizedComp, itsBuilder.GetObject())
	if err != nil {
		return nil, err
	}

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

func vctToPVC(vct corev1.PersistentVolumeClaimTemplate) corev1.PersistentVolumeClaim {
	return corev1.PersistentVolumeClaim{
		ObjectMeta: vct.ObjectMeta,
		Spec:       vct.Spec,
	}
}

// getMonitorAnnotations returns the annotations for the monitor.
func getMonitorAnnotations(synthesizedComp *component.SynthesizedComponent, componentDef *appsv1.ComponentDefinition) map[string]string {
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

// GetRestoreSystemAccountPassword gets restore password if exists during recovery.
func GetRestoreSystemAccountPassword(synthesizedComp *component.SynthesizedComponent, account appsv1.SystemAccount) string {
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
	systemAccountsString, ok := backupSource[constant.EncryptedSystemAccounts]
	if !ok {
		return ""
	}
	systemAccountsMap := map[string]string{}
	err = json.Unmarshal([]byte(systemAccountsString), &systemAccountsMap)
	if err != nil {
		return ""
	}
	e := intctrlutil.NewEncryptor(viper.GetString(constant.CfgKeyDPEncryptionKey))
	encryptedPwd, ok := systemAccountsMap[account.Name]
	if !ok {
		return ""
	}
	password, _ := e.Decrypt([]byte(encryptedPwd))
	return password
}

func BuildPVC(cluster *appsv1.Cluster,
	synthesizedComp *component.SynthesizedComponent,
	vct *corev1.PersistentVolumeClaimTemplate,
	pvcKey types.NamespacedName,
	templateName,
	snapshotName string) *corev1.PersistentVolumeClaim {
	pvcBuilder := builder.NewPVCBuilder(pvcKey.Namespace, pvcKey.Name).
		AddLabelsInMap(constant.GetCompLabels(cluster.Name, synthesizedComp.Name)).
		AddLabelsInMap(synthesizedComp.DynamicLabels).
		AddLabels(constant.VolumeClaimTemplateNameLabelKey, vct.Name).
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddAnnotationsInMap(synthesizedComp.DynamicAnnotations).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		SetAccessModes(vct.Spec.AccessModes).
		SetResources(vct.Spec.Resources)
	if vct.Spec.StorageClassName != nil {
		pvcBuilder.SetStorageClass(*vct.Spec.StorageClassName)
	}
	if len(snapshotName) > 0 {
		apiGroup := "snapshot.storage.k8s.io"
		pvcBuilder.SetDataSource(corev1.TypedLocalObjectReference{
			APIGroup: &apiGroup,
			Kind:     "VolumeSnapshot",
			Name:     snapshotName,
		})
	}
	pvc := pvcBuilder.GetObject()
	BuildPersistentVolumeClaimLabels(synthesizedComp, pvc, vct.Name, templateName)
	return pvc
}

func BuildBackup(cluster *appsv1.Cluster,
	synthesizedComp *component.SynthesizedComponent,
	backupPolicyName string,
	backupKey types.NamespacedName,
	backupMethod string) *dpv1alpha1.Backup {
	return builder.NewBackupBuilder(backupKey.Namespace, backupKey.Name).
		AddLabels(dptypes.BackupMethodLabelKey, backupMethod).
		AddLabels(dptypes.BackupPolicyLabelKey, backupPolicyName).
		AddLabels(constant.KBManagedByKey, "cluster").
		AddLabels(constant.AppNameLabelKey, synthesizedComp.ClusterDefName).
		AddLabels(constant.AppInstanceLabelKey, cluster.Name).
		AddLabels(constant.AppManagedByLabelKey, constant.AppName).
		AddLabels(constant.KBAppComponentLabelKey, synthesizedComp.Name).
		SetBackupPolicyName(backupPolicyName).
		SetBackupMethod(backupMethod).
		GetObject()
}

func BuildConfigMapWithTemplate(cluster *appsv1.Cluster,
	synthesizedComp *component.SynthesizedComponent,
	configs map[string]string,
	cmName string,
	configTemplateSpec appsv1.ComponentTemplateSpec) *corev1.ConfigMap {
	return builder.NewConfigMapBuilder(cluster.Namespace, cmName).
		AddLabelsInMap(constant.GetCompLabels(cluster.Name, synthesizedComp.Name)).
		AddLabels(constant.CMConfigurationTypeLabelKey, constant.ConfigInstanceType).
		AddLabels(constant.CMTemplateNameLabelKey, configTemplateSpec.TemplateRef).
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddAnnotations(constant.DisableUpgradeInsConfigurationAnnotationKey, strconv.FormatBool(false)).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
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

func BuildCfgManagerToolsContainer(sidecarRenderedParam *cfgcm.CfgManagerBuildParams, toolsMetas []appsv1beta1.ToolConfig, toolsMap map[string]cfgcm.ConfigSpecMeta) ([]corev1.Container, error) {
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
		AddLabelsInMap(constant.GetCompLabels(synthesizedComp.ClusterName, synthesizedComp.Name)).
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		SetImagePullSecrets(intctrlutil.BuildImagePullSecrets()).
		GetObject()
}

func BuildRoleBinding(synthesizedComp *component.SynthesizedComponent, saName string) *rbacv1.RoleBinding {
	return builder.NewRoleBindingBuilder(synthesizedComp.Namespace, saName).
		AddLabelsInMap(constant.GetCompLabels(synthesizedComp.ClusterName, synthesizedComp.Name)).
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		SetRoleRef(rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     constant.RBACRoleName,
		}).
		AddSubjects(rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Namespace: synthesizedComp.Namespace,
			Name:      saName,
		}).
		GetObject()
}
