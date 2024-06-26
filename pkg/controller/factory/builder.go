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
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
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
func BuildInstanceSet(synthesizedComp *component.SynthesizedComponent, componentDef *appsv1alpha1.ComponentDefinition) (*workloads.InstanceSet, error) {
	var (
		clusterDefName     = synthesizedComp.ClusterDefName
		clusterCompDefName = synthesizedComp.ClusterCompDefName
		compDefName        = synthesizedComp.CompDefName
		namespace          = synthesizedComp.Namespace
		clusterName        = synthesizedComp.ClusterName
		compName           = synthesizedComp.Name
	)
	// build labels
	labels := constant.GetKBWellKnownLabelsWithCompDef(compDefName, clusterName, compName)
	compDefLabel := constant.GetComponentDefLabel(compDefName)
	if len(compDefName) == 0 {
		// TODO(xingran): for backward compatibility in kubeBlocks version 0.8.0, it will be removed in the future.
		labels = constant.GetKBWellKnownLabels(clusterDefName, clusterName, compName)
		compDefLabel = constant.GetClusterCompDefLabel(clusterCompDefName)
	}
	mergeLabels := intctrlutil.MergeMetadataMaps(labels, compDefLabel, synthesizedComp.Labels)

	// build annotations
	mergeAnnotations := intctrlutil.MergeMetadataMaps(
		constant.GetKBGenerationAnnotation(synthesizedComp.ClusterGeneration),
		getMonitorAnnotations(synthesizedComp, componentDef),
		compDefLabel,
		constant.GetServiceVersionAnnotation(synthesizedComp.ServiceVersion),
		synthesizedComp.Annotations,
	)

	replicasStr := strconv.Itoa(int(synthesizedComp.Replicas))
	podBuilder := builder.NewPodBuilder("", "").
		AddLabelsInMap(synthesizedComp.Labels).
		AddLabelsInMap(labels).
		AddLabelsInMap(compDefLabel).
		AddLabelsInMap(constant.GetAppVersionLabel(compDefName)).
		AddLabelsInMap(synthesizedComp.UserDefinedLabels).
		AddAnnotations(constant.ComponentReplicasAnnotationKey, replicasStr).
		AddAnnotationsInMap(synthesizedComp.UserDefinedAnnotations)
	template := corev1.PodTemplateSpec{
		ObjectMeta: podBuilder.GetObject().ObjectMeta,
		Spec:       *synthesizedComp.PodSpec.DeepCopy(),
	}

	itsName := constant.GenerateWorkloadNamePattern(clusterName, compName)
	itsBuilder := builder.NewInstanceSetBuilder(namespace, itsName).
		AddLabelsInMap(mergeLabels).
		AddAnnotationsInMap(mergeAnnotations).
		AddMatchLabelsInMap(labels).
		SetReplicas(synthesizedComp.Replicas).
		SetMinReadySeconds(synthesizedComp.MinReadySeconds).
		SetTemplate(template)

	var vcts []corev1.PersistentVolumeClaim
	for _, vct := range synthesizedComp.VolumeClaimTemplates {
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
			BuildPersistentVolumeClaimLabels(synthesizedComp, &vct, vct.Name)
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
func getMonitorAnnotations(synthesizedComp *component.SynthesizedComponent, componentDef *appsv1alpha1.ComponentDefinition) map[string]string {
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
	pvcTplName string) {
	// strict args checking.
	if pvc == nil || component == nil {
		return
	}
	if pvc.Labels == nil {
		pvc.Labels = make(map[string]string)
	}
	pvc.Labels[constant.VolumeClaimTemplateNameLabelKey] = pvcTplName

	for _, t := range component.VolumeTypes {
		if t.Name == pvcTplName {
			pvc.Labels[constant.VolumeTypeLabelKey] = string(t.Type)
			break
		}
	}
}

func randomString(length int) string {
	return rand.String(length)
}

func strongRandomString(length int) string {
	str, _ := common.GeneratePassword(length, 3, 3, false, "")
	return str
}

func BuildConnCredential(clusterDefinition *appsv1alpha1.ClusterDefinition, cluster *appsv1alpha1.Cluster,
	synthesizedComp *component.SynthesizedComponent) *corev1.Secret {
	wellKnownLabels := constant.GetKBWellKnownLabels(clusterDefinition.Name, cluster.Name, "")
	delete(wellKnownLabels, constant.KBAppComponentLabelKey)
	credentialBuilder := builder.NewSecretBuilder(cluster.Namespace, constant.GenerateDefaultConnCredential(cluster.Name)).
		AddLabelsInMap(wellKnownLabels).
		SetStringData(clusterDefinition.Spec.ConnectionCredential)
	if len(clusterDefinition.Spec.Type) > 0 {
		credentialBuilder.AddLabelsInMap(constant.GetClusterDefTypeLabel(clusterDefinition.Spec.Type))
	}
	connCredential := credentialBuilder.GetObject()

	if len(connCredential.StringData) == 0 {
		return connCredential
	}

	replaceVarObjects := func(k, v *string, i int, origValue string, varObjectsMap map[string]string) {
		toReplace := origValue
		for j, r := range varObjectsMap {
			replaced := strings.ReplaceAll(toReplace, j, r)
			if replaced == toReplace {
				continue
			}
			toReplace = replaced
			// replace key
			if i == 0 {
				delete(connCredential.StringData, origValue)
				*k = replaced
			} else {
				*v = replaced
			}
		}
	}

	// REVIEW: perhaps handles value replacement at `func mergeComponents`
	replaceData := func(varObjectsMap map[string]string) {
		copyStringData := connCredential.DeepCopy().StringData
		for k, v := range copyStringData {
			for i, vv := range []string{k, v} {
				if !strings.Contains(vv, "$(") {
					continue
				}
				replaceVarObjects(&k, &v, i, vv, varObjectsMap)
			}
			connCredential.StringData[k] = v
		}
	}

	// TODO: do JIT value generation for lower CPU resources
	// 1st pass replace variables
	uuidVal := uuid.New()
	uuidBytes := uuidVal[:]
	uuidStr := uuidVal.String()
	uuidB64 := base64.RawStdEncoding.EncodeToString(uuidBytes)
	uuidStrB64 := base64.RawStdEncoding.EncodeToString([]byte(strings.ReplaceAll(uuidStr, "-", "")))
	uuidHex := hex.EncodeToString(uuidBytes)
	randomPassword := randomString(8)
	strongRandomPasswd := strongRandomString(16)
	restorePassword := GetRestorePassword(synthesizedComp)
	// check if a connection password is specified during recovery.
	// if exists, replace the random password
	if restorePassword != "" {
		randomPassword = restorePassword
		strongRandomPasswd = restorePassword
	}
	m := map[string]string{
		"$(RANDOM_PASSWD)":        randomPassword,
		"$(STRONG_RANDOM_PASSWD)": strongRandomPasswd,
		"$(UUID)":                 uuidStr,
		"$(UUID_B64)":             uuidB64,
		"$(UUID_STR_B64)":         uuidStrB64,
		"$(UUID_HEX)":             uuidHex,
		"$(SVC_FQDN)":             constant.GenerateDefaultComponentServiceName(cluster.Name, synthesizedComp.Name),
		constant.EnvPlaceHolder(constant.KBEnvClusterCompName): constant.GenerateClusterComponentName(cluster.Name, synthesizedComp.Name),
		"$(HEADLESS_SVC_FQDN)":                                 constant.GenerateDefaultComponentHeadlessServiceName(cluster.Name, synthesizedComp.Name),
	}
	if len(synthesizedComp.ComponentServices) > 0 {
		for _, p := range synthesizedComp.ComponentServices[0].Spec.Ports {
			m[fmt.Sprintf("$(SVC_PORT_%s)", p.Name)] = strconv.Itoa(int(p.Port))
		}
	}
	replaceData(m)

	// 2nd pass replace $(CONN_CREDENTIAL) variables
	m = map[string]string{}
	for k, v := range connCredential.StringData {
		m[fmt.Sprintf("$(CONN_CREDENTIAL).%s", k)] = v
	}
	replaceData(m)
	return connCredential
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

func BuildPVC(cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	vct *corev1.PersistentVolumeClaimTemplate,
	pvcKey types.NamespacedName,
	snapshotName string) *corev1.PersistentVolumeClaim {
	wellKnownLabels := constant.GetKBWellKnownLabels(component.ClusterDefName, cluster.Name, component.Name)
	pvcBuilder := builder.NewPVCBuilder(pvcKey.Namespace, pvcKey.Name).
		AddLabelsInMap(wellKnownLabels).
		AddLabels(constant.VolumeClaimTemplateNameLabelKey, vct.Name).
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
	BuildPersistentVolumeClaimLabels(component, pvc, vct.Name)
	return pvc
}

func BuildBackup(cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	backupPolicyName string,
	backupKey types.NamespacedName,
	backupMethod string) *dpv1alpha1.Backup {
	return builder.NewBackupBuilder(backupKey.Namespace, backupKey.Name).
		AddLabels(dptypes.BackupMethodLabelKey, backupMethod).
		AddLabels(dptypes.BackupPolicyLabelKey, backupPolicyName).
		AddLabels(constant.KBManagedByKey, "cluster").
		AddLabels(constant.AppNameLabelKey, component.ClusterDefName).
		AddLabels(constant.AppInstanceLabelKey, cluster.Name).
		AddLabels(constant.AppManagedByLabelKey, constant.AppName).
		AddLabels(constant.KBAppComponentLabelKey, component.Name).
		SetBackupPolicyName(backupPolicyName).
		SetBackupMethod(backupMethod).
		GetObject()
}

func BuildConfigMapWithTemplate(cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	configs map[string]string,
	cmName string,
	configTemplateSpec appsv1alpha1.ComponentTemplateSpec) *corev1.ConfigMap {
	wellKnownLabels := constant.GetKBWellKnownLabels(component.ClusterDefName, cluster.Name, component.Name)
	wellKnownLabels[constant.AppComponentLabelKey] = component.ClusterCompDefName
	return builder.NewConfigMapBuilder(cluster.Namespace, cmName).
		AddLabelsInMap(wellKnownLabels).
		AddLabels(constant.CMConfigurationTypeLabelKey, constant.ConfigInstanceType).
		AddLabels(constant.CMTemplateNameLabelKey, configTemplateSpec.TemplateRef).
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

func BuildVolumeSnapshotClass(name string, driver string) *snapshotv1.VolumeSnapshotClass {
	return builder.NewVolumeSnapshotClassBuilder("", name).
		AddLabels(constant.AppManagedByLabelKey, constant.AppName).
		SetDriver(driver).
		SetDeletionPolicy(snapshotv1.VolumeSnapshotContentDelete).
		GetObject()
}

func BuildServiceAccount(cluster *appsv1alpha1.Cluster, saName string) *corev1.ServiceAccount {
	// TODO(component): compName
	wellKnownLabels := constant.GetKBWellKnownLabels(cluster.Spec.ClusterDefRef, cluster.Name, "")
	return builder.NewServiceAccountBuilder(cluster.Namespace, saName).
		AddLabelsInMap(wellKnownLabels).
		GetObject()
}

func BuildRoleBinding(cluster *appsv1alpha1.Cluster, saName string) *rbacv1.RoleBinding {
	// TODO(component): compName
	wellKnownLabels := constant.GetKBWellKnownLabels(cluster.Spec.ClusterDefRef, cluster.Name, "")
	return builder.NewRoleBindingBuilder(cluster.Namespace, saName).
		AddLabelsInMap(wellKnownLabels).
		SetRoleRef(rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     constant.RBACRoleName,
		}).
		AddSubjects(rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Namespace: cluster.Namespace,
			Name:      saName,
		}).
		GetObject()
}

func BuildClusterRoleBinding(cluster *appsv1alpha1.Cluster, saName string) *rbacv1.ClusterRoleBinding {
	// TODO(component): compName
	wellKnownLabels := constant.GetKBWellKnownLabels(cluster.Spec.ClusterDefRef, cluster.Name, "")
	return builder.NewClusterRoleBindingBuilder(cluster.Namespace, saName).
		AddLabelsInMap(wellKnownLabels).
		SetRoleRef(rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     constant.RBACClusterRoleName,
		}).
		AddSubjects(rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Namespace: cluster.Namespace,
			Name:      saName,
		}).
		GetObject()
}
