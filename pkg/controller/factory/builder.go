/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	VolumeName = "tls"
	CAName     = "ca.crt"
	CertName   = "tls.crt"
	KeyName    = "tls.key"
	MountPath  = "/etc/pki/tls"
)

func processContainersInjection(cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	podSpec *corev1.PodSpec) error {
	for _, cc := range []*[]corev1.Container{
		&podSpec.Containers,
		&podSpec.InitContainers,
	} {
		for i := range *cc {
			if err := injectEnvs(cluster, component, &(*cc)[i]); err != nil {
				return err
			}
			intctrlutil.InjectZeroResourcesLimitsIfEmpty(&(*cc)[i])
		}
	}
	return nil
}

func injectEnvs(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent, c *corev1.Container) error {
	// can not use map, it is unordered
	envFieldPathSlice := []struct {
		name      string
		fieldPath string
	}{
		{name: constant.KBEnvPodName, fieldPath: "metadata.name"},
		{name: constant.KBEnvPodUID, fieldPath: "metadata.uid"},
		{name: constant.KBEnvNamespace, fieldPath: "metadata.namespace"},
		{name: "KB_SA_NAME", fieldPath: "spec.serviceAccountName"},
		{name: constant.KBEnvNodeName, fieldPath: "spec.nodeName"},
		{name: constant.KBEnvHostIP, fieldPath: "status.hostIP"},
		{name: "KB_POD_IP", fieldPath: "status.podIP"},
		{name: "KB_POD_IPS", fieldPath: "status.podIPs"},
		// TODO: need to deprecate following
		{name: "KB_HOSTIP", fieldPath: "status.hostIP"},
		{name: "KB_PODIP", fieldPath: "status.podIP"},
		{name: "KB_PODIPS", fieldPath: "status.podIPs"},
	}

	toInjectEnvs := make([]corev1.EnvVar, 0, len(envFieldPathSlice)+len(c.Env))
	for _, v := range envFieldPathSlice {
		toInjectEnvs = append(toInjectEnvs, corev1.EnvVar{
			Name: v.name,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  v.fieldPath,
				},
			},
		})
	}

	var kbClusterPostfix8 string
	if len(cluster.UID) > 8 {
		kbClusterPostfix8 = string(cluster.UID)[len(cluster.UID)-8:]
	} else {
		kbClusterPostfix8 = string(cluster.UID)
	}
	toInjectEnvs = append(toInjectEnvs, []corev1.EnvVar{
		{Name: "KB_CLUSTER_NAME", Value: cluster.Name},
		{Name: "KB_COMP_NAME", Value: component.Name},
		{Name: "KB_CLUSTER_COMP_NAME", Value: cluster.Name + "-" + component.Name},
		{Name: "KB_CLUSTER_UID_POSTFIX_8", Value: kbClusterPostfix8},
		{Name: "KB_POD_FQDN", Value: fmt.Sprintf("%s.%s-headless.%s.svc", "$(KB_POD_NAME)",
			"$(KB_CLUSTER_COMP_NAME)", "$(KB_NAMESPACE)")},
	}...)

	if component.TLSConfig != nil && component.TLSConfig.Enable {
		toInjectEnvs = append(toInjectEnvs, []corev1.EnvVar{
			{Name: "KB_TLS_CERT_PATH", Value: MountPath},
			{Name: "KB_TLS_CA_FILE", Value: CAName},
			{Name: "KB_TLS_CERT_FILE", Value: CertName},
			{Name: "KB_TLS_KEY_FILE", Value: KeyName},
		}...)
	}

	if udeValue, ok := cluster.Annotations[constant.ExtraEnvAnnotationKey]; ok {
		udeMap := make(map[string]string)
		if err := json.Unmarshal([]byte(udeValue), &udeMap); err != nil {
			return err
		}
		keys := make([]string, 0)
		for k := range udeMap {
			if k == "" || udeMap[k] == "" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			toInjectEnvs = append(toInjectEnvs, corev1.EnvVar{
				Name:  k,
				Value: udeMap[k],
			})
		}
	}

	// build env from componentRefEnv
	if component.ComponentRefEnvs != nil {
		for _, env := range component.ComponentRefEnvs {
			toInjectEnvs = append(toInjectEnvs, *env)
		}
	}

	// have injected variables placed at the front of the slice
	if len(c.Env) == 0 {
		c.Env = toInjectEnvs
	} else {
		c.Env = append(toInjectEnvs, c.Env...)
	}
	return nil
}

// BuildPersistentVolumeClaimLabels builds a pvc name label, and synchronize the labels from sts to pvc.
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

	if component.VolumeTypes != nil {
		for _, t := range component.VolumeTypes {
			if t.Name == pvcTplName {
				pvc.Labels[constant.VolumeTypeLabelKey] = string(t.Type)
				break
			}
		}
	}
}

func BuildSts(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) (*appsv1.StatefulSet, error) {
	vctToPVC := func(vct corev1.PersistentVolumeClaimTemplate) corev1.PersistentVolumeClaim {
		return corev1.PersistentVolumeClaim{
			ObjectMeta: vct.ObjectMeta,
			Spec:       vct.Spec,
		}
	}
	commonLabels := constant.GetKBWellKnownLabels(component.ClusterDefName, cluster.Name, component.Name)
	podBuilder := builder.NewPodBuilder("", "").
		AddLabelsInMap(commonLabels).
		AddLabelsInMap(constant.GetClusterCompDefLabel(component.ClusterCompDefName)).
		AddLabelsInMap(constant.GetWorkloadTypeLabel(string(component.WorkloadType)))
	if len(cluster.Spec.ClusterVersionRef) > 0 {
		podBuilder.AddLabelsInMap(constant.GetClusterVersionLabel(cluster.Spec.ClusterVersionRef))
	}
	template := corev1.PodTemplateSpec{
		ObjectMeta: podBuilder.GetObject().ObjectMeta,
		Spec:       *component.PodSpec,
	}
	stsBuilder := builder.NewStatefulSetBuilder(cluster.Namespace, cluster.Name+"-"+component.Name).
		AddLabelsInMap(commonLabels).
		AddLabelsInMap(constant.GetClusterCompDefLabel(component.ClusterCompDefName)).
		AddMatchLabelsInMap(commonLabels).
		SetServiceName(cluster.Name + "-" + component.Name + "-headless").
		SetReplicas(component.Replicas).
		SetTemplate(template)

	var vcts []corev1.PersistentVolumeClaim
	for _, vct := range component.VolumeClaimTemplates {
		vcts = append(vcts, vctToPVC(vct))
	}
	stsBuilder.SetVolumeClaimTemplates(vcts...)

	if component.StatefulSetWorkload != nil {
		podManagementPolicy, updateStrategy := component.StatefulSetWorkload.FinalStsUpdateStrategy()
		stsBuilder.SetPodManagementPolicy(podManagementPolicy).SetUpdateStrategy(updateStrategy)
	}

	sts := stsBuilder.GetObject()

	// update sts.spec.volumeClaimTemplates[].metadata.labels
	if len(sts.Spec.VolumeClaimTemplates) > 0 && len(sts.GetLabels()) > 0 {
		for index, vct := range sts.Spec.VolumeClaimTemplates {
			BuildPersistentVolumeClaimLabels(component, &vct, vct.Name)
			sts.Spec.VolumeClaimTemplates[index] = vct
		}
	}

	if err := processContainersInjection(cluster, component, &sts.Spec.Template.Spec); err != nil {
		return nil, err
	}
	return sts, nil
}

func fixService(namespace, prefix string, component *component.SynthesizedComponent, alternativeServices ...corev1.Service) []corev1.Service {
	leaderName := getLeaderName(component)
	for i := range alternativeServices {
		if len(alternativeServices[i].Name) > 0 {
			alternativeServices[i].Name = prefix + "-" + alternativeServices[i].Name
		}
		if len(alternativeServices[i].Namespace) == 0 {
			alternativeServices[i].Namespace = namespace
		}
		if alternativeServices[i].Spec.Type == corev1.ServiceTypeLoadBalancer {
			alternativeServices[i].Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
		}
		if len(leaderName) > 0 {
			selector := alternativeServices[i].Spec.Selector
			if selector == nil {
				selector = make(map[string]string, 0)
			}
			selector[constant.RoleLabelKey] = leaderName
			alternativeServices[i].Spec.Selector = selector
		}
	}
	return alternativeServices
}

func getLeaderName(component *component.SynthesizedComponent) string {
	if component == nil {
		return ""
	}
	switch component.WorkloadType {
	case appsv1alpha1.Consensus:
		if component.ConsensusSpec != nil {
			return component.ConsensusSpec.Leader.Name
		}
	case appsv1alpha1.Replication:
		return constant.Primary
	}
	return ""
}

// separateServices separates 'services' to a main service from cd and alternative services from cluster
func separateServices(services []corev1.Service) (*corev1.Service, []corev1.Service) {
	if len(services) == 0 {
		return nil, nil
	}
	// from component.buildComponent (which contains component.Services' building process), the first item should be the main service
	// TODO(free6om): make two fields in component(i.e. Service and AlternativeServices) after RSM passes all testes.
	return &services[0], services[1:]
}

func buildRoleInfo(component *component.SynthesizedComponent) ([]workloads.ReplicaRole, *workloads.RoleProbe, *workloads.MembershipReconfiguration, *workloads.MemberUpdateStrategy) {
	if component.RSMSpec != nil {
		return buildRoleInfo2(component)
	}

	var (
		roles           []workloads.ReplicaRole
		probe           *workloads.RoleProbe
		reconfiguration *workloads.MembershipReconfiguration
		strategy        *workloads.MemberUpdateStrategy
	)

	handler := convertCharacterTypeToHandler(component.CharacterType, component.WorkloadType == appsv1alpha1.Consensus)

	if handler != nil && component.Probes != nil && component.Probes.RoleProbe != nil {
		probe = &workloads.RoleProbe{}
		probe.BuiltinHandler = (*string)(handler)
		roleProbe := component.Probes.RoleProbe
		probe.PeriodSeconds = roleProbe.PeriodSeconds
		probe.TimeoutSeconds = roleProbe.TimeoutSeconds
		probe.FailureThreshold = roleProbe.FailureThreshold
		// set to default value
		probe.SuccessThreshold = 1
		probe.RoleUpdateMechanism = workloads.DirectAPIServerEventUpdate
	}

	reconfiguration = nil

	switch component.WorkloadType {
	case appsv1alpha1.Consensus:
		roles, strategy = buildRoleInfoFromConsensus(component.ConsensusSpec)
	case appsv1alpha1.Replication:
		roles = buildRoleInfoFromReplication()
		reconfiguration = nil
		strgy := workloads.SerialUpdateStrategy
		strategy = &strgy
	}

	return roles, probe, reconfiguration, strategy
}

func buildRoleInfo2(component *component.SynthesizedComponent) ([]workloads.ReplicaRole, *workloads.RoleProbe, *workloads.MembershipReconfiguration, *workloads.MemberUpdateStrategy) {
	rsmSpec := component.RSMSpec
	return rsmSpec.Roles, rsmSpec.RoleProbe, rsmSpec.MembershipReconfiguration, rsmSpec.MemberUpdateStrategy
}

func buildRoleInfoFromReplication() []workloads.ReplicaRole {
	return []workloads.ReplicaRole{
		{
			Name:       constant.Primary,
			IsLeader:   true,
			CanVote:    true,
			AccessMode: workloads.ReadWriteMode,
		},
		{
			Name:       constant.Secondary,
			IsLeader:   false,
			CanVote:    true,
			AccessMode: workloads.ReadonlyMode,
		},
	}
}

func buildRoleInfoFromConsensus(consensusSpec *appsv1alpha1.ConsensusSetSpec) ([]workloads.ReplicaRole, *workloads.MemberUpdateStrategy) {
	if consensusSpec == nil {
		return nil, nil
	}

	var (
		roles    []workloads.ReplicaRole
		strategy *workloads.MemberUpdateStrategy
	)

	roles = append(roles, workloads.ReplicaRole{
		Name:       consensusSpec.Leader.Name,
		IsLeader:   true,
		CanVote:    true,
		AccessMode: workloads.AccessMode(consensusSpec.Leader.AccessMode),
	})
	for _, follower := range consensusSpec.Followers {
		roles = append(roles, workloads.ReplicaRole{
			Name:       follower.Name,
			IsLeader:   false,
			CanVote:    true,
			AccessMode: workloads.AccessMode(follower.AccessMode),
		})
	}
	if consensusSpec.Learner != nil {
		roles = append(roles, workloads.ReplicaRole{
			Name:       consensusSpec.Learner.Name,
			IsLeader:   false,
			CanVote:    false,
			AccessMode: workloads.AccessMode(consensusSpec.Learner.AccessMode),
		})
	}

	strgy := workloads.MemberUpdateStrategy(consensusSpec.UpdateStrategy)
	strategy = &strgy

	return roles, strategy
}

func convertCharacterTypeToHandler(characterType string, isConsensus bool) *common.BuiltinHandler {
	var handler common.BuiltinHandler
	kind := strings.ToLower(characterType)
	switch kind {
	case "mysql": //nolint:goconst
		if isConsensus {
			handler = common.WeSQLHandler
		} else {
			handler = common.MySQLHandler
		}
	case "postgres", "postgresql":
		handler = common.PostgresHandler
	case "mongodb":

		handler = common.MongoDBHandler
	case "etcd":
		handler = common.ETCDHandler
	case "redis":
		handler = common.RedisHandler
	case "kafka":
		handler = common.KafkaHandler
	}
	if handler != "" {
		return &handler
	}
	return nil
}

func randomString(length int) string {
	return rand.String(length)
}

func BuildConnCredential(clusterDefinition *appsv1alpha1.ClusterDefinition, cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) *corev1.Secret {
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

	// get restore password if exists during recovery.
	getRestorePassword := func() string {
		valueString := cluster.Annotations[constant.RestoreFromBackupAnnotationKey]
		if len(valueString) == 0 {
			return ""
		}
		backupMap := map[string]map[string]string{}
		err := json.Unmarshal([]byte(valueString), &backupMap)
		if err != nil {
			return ""
		}
		backupSource, ok := backupMap[component.Name]
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

	// TODO: do JIT value generation for lower CPU resources
	// 1st pass replace variables
	uuidVal := uuid.New()
	uuidBytes := uuidVal[:]
	uuidStr := uuidVal.String()
	uuidB64 := base64.RawStdEncoding.EncodeToString(uuidBytes)
	uuidStrB64 := base64.RawStdEncoding.EncodeToString([]byte(strings.ReplaceAll(uuidStr, "-", "")))
	uuidHex := hex.EncodeToString(uuidBytes)
	randomPassword := randomString(8)
	restorePassword := getRestorePassword()
	// check if a connection password is specified during recovery.
	// if exists, replace the random password
	if restorePassword != "" {
		randomPassword = restorePassword
	}
	m := map[string]string{
		"$(RANDOM_PASSWD)":        randomPassword,
		"$(UUID)":                 uuidStr,
		"$(UUID_B64)":             uuidB64,
		"$(UUID_STR_B64)":         uuidStrB64,
		"$(UUID_HEX)":             uuidHex,
		"$(SVC_FQDN)":             constant.GenerateDefaultComponentServiceName(cluster.Name, component.Name),
		"$(KB_CLUSTER_COMP_NAME)": constant.GenerateClusterComponentName(cluster.Name, component.Name),
		"$(HEADLESS_SVC_FQDN)":    constant.GenerateDefaultComponentHeadlessServiceName(cluster.Name, component.Name),
	}
	if len(component.Services) > 0 {
		for _, p := range component.Services[0].Spec.Ports {
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

func BuildConnCredential4Cluster(cluster *appsv1alpha1.Cluster, name string, data map[string][]byte) *corev1.Secret {
	secretName := constant.GenerateClusterConnCredential(cluster.Name, name)
	labels := constant.GetClusterWellKnownLabels(cluster.Name)
	return builder.NewSecretBuilder(cluster.Namespace, secretName).
		AddLabelsInMap(labels).
		SetData(data).
		SetImmutable(true).
		GetObject()
}

func BuildConnCredential4Component(comp *component.SynthesizedComponent, name string, data map[string][]byte) *corev1.Secret {
	secretName := constant.GenerateComponentConnCredential(comp.ClusterName, comp.Name, name)
	labels := constant.GetComponentWellKnownLabels(comp.ClusterName, comp.Name)
	return builder.NewSecretBuilder(comp.Namespace, secretName).
		AddLabelsInMap(labels).
		SetData(data).
		SetImmutable(true).
		GetObject()
}

func BuildPDB(synthesizedComp *component.SynthesizedComponent) *policyv1.PodDisruptionBudget {
	var (
		namespace   = synthesizedComp.Namespace
		clusterName = synthesizedComp.ClusterName
		compName    = synthesizedComp.Name
		labels      = constant.GetKBWellKnownLabels(synthesizedComp.ClusterDefName, clusterName, compName)
	)
	return builder.NewPDBBuilder(namespace, constant.GenerateClusterComponentName(clusterName, compName)).
		AddLabelsInMap(labels).
		AddLabelsInMap(constant.GetClusterCompDefLabel(synthesizedComp.ClusterCompDefName)).
		AddSelectorsInMap(labels).
		GetObject()
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

func BuildCfgManagerContainer(sidecarRenderedParam *cfgcm.CfgManagerBuildParams, component *component.SynthesizedComponent) (*corev1.Container, error) {
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
	if len(sidecarRenderedParam.CharacterType) > 0 {
		env = append(env, corev1.EnvVar{
			Name:  "DB_TYPE",
			Value: sidecarRenderedParam.CharacterType,
		})
	}
	if sidecarRenderedParam.CharacterType == "mysql" {
		env = append(env, corev1.EnvVar{
			Name: "MYSQL_USER",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key:                  "username",
					LocalObjectReference: corev1.LocalObjectReference{Name: sidecarRenderedParam.SecreteName},
				},
			},
		},
			corev1.EnvVar{
				Name: "MYSQL_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key:                  "password",
						LocalObjectReference: corev1.LocalObjectReference{Name: sidecarRenderedParam.SecreteName},
					},
				},
			},
			corev1.EnvVar{
				Name:  "DATA_SOURCE_NAME",
				Value: "$(MYSQL_USER):$(MYSQL_PASSWORD)@(localhost:3306)/",
			},
		)
	}
	containerBuilder := builder.NewContainerBuilder(sidecarRenderedParam.ManagerName).
		AddCommands("env").
		AddArgs("PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$(TOOLS_PATH)").
		AddArgs("/bin/reloader").
		AddArgs(sidecarRenderedParam.Args...).
		AddEnv(env...).
		SetImage(sidecarRenderedParam.Image).
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddVolumeMounts(sidecarRenderedParam.Volumes...)
	if sidecarRenderedParam.ShareProcessNamespace {
		user := int64(0)
		containerBuilder.SetSecurityContext(corev1.SecurityContext{
			RunAsUser: &user,
		})
	}
	container := containerBuilder.GetObject()
	if err := injectEnvs(sidecarRenderedParam.Cluster, component, container); err != nil {
		return nil, err
	}
	intctrlutil.InjectZeroResourcesLimitsIfEmpty(container)
	return container, nil
}

func BuildRestoreJob(cluster *appsv1alpha1.Cluster, synthesizedComponent *component.SynthesizedComponent, name, image string, command []string,
	volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, env []corev1.EnvVar, resources *corev1.ResourceRequirements) (*batchv1.Job, error) {
	containerBuilder := builder.NewContainerBuilder("restore").
		SetImage(image).
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddCommands(command...).
		AddVolumeMounts(volumeMounts...).
		AddEnv(env...)
	if resources != nil {
		containerBuilder.SetResources(*resources)
	}
	container := containerBuilder.GetObject()

	ctx := corev1.PodSecurityContext{}
	user := int64(0)
	ctx.RunAsUser = &user
	pod := builder.NewPodBuilder(cluster.Namespace, "").
		AddContainer(*container).
		AddVolumes(volumes...).
		SetRestartPolicy(corev1.RestartPolicyOnFailure).
		SetSecurityContext(ctx).
		GetObject()
	template := corev1.PodTemplateSpec{
		Spec: pod.Spec,
	}

	job := builder.NewJobBuilder(cluster.Namespace, name).
		AddLabels(constant.AppManagedByLabelKey, constant.AppName).
		SetPodTemplateSpec(template).
		GetObject()
	containers := job.Spec.Template.Spec.Containers
	if len(containers) > 0 {
		if err := injectEnvs(cluster, synthesizedComponent, &containers[0]); err != nil {
			return nil, err
		}
		intctrlutil.InjectZeroResourcesLimitsIfEmpty(&containers[0])
	}
	compSpec := cluster.Spec.GetComponentByName(synthesizedComponent.Name)
	if compSpec != nil {
		tolerations, err := component.BuildTolerations(cluster, compSpec)
		if err != nil {
			return nil, err
		}
		job.Spec.Template.Spec.Tolerations = tolerations
	}
	return job, nil
}

func BuildCfgManagerToolsContainer(sidecarRenderedParam *cfgcm.CfgManagerBuildParams, component *component.SynthesizedComponent, toolsMetas []appsv1alpha1.ToolConfig, toolsMap map[string]cfgcm.ConfigSpecMeta) ([]corev1.Container, error) {
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
		if err := injectEnvs(sidecarRenderedParam.Cluster, component, container); err != nil {
			return nil, err
		}
		intctrlutil.InjectZeroResourcesLimitsIfEmpty(container)
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
