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

package builder

import (
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/leaanthony/debme"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type BuilderParams struct {
	ClusterDefinition *appsv1alpha1.ClusterDefinition
	ClusterVersion    *appsv1alpha1.ClusterVersion
	Cluster           *appsv1alpha1.Cluster
	Component         *component.SynthesizedComponent
}

type componentPathedName struct {
	Namespace   string `json:"namespace,omitempty"`
	ClusterName string `json:"clusterName,omitempty"`
	Name        string `json:"name,omitempty"`
}

const (
	VolumeName = "tls"
	CAName     = "ca.crt"
	CertName   = "tls.crt"
	KeyName    = "tls.key"
	MountPath  = "/etc/pki/tls"
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

func processContainersInjection(reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	envConfigName string,
	podSpec *corev1.PodSpec) error {
	for _, cc := range []*[]corev1.Container{
		&podSpec.Containers,
		&podSpec.InitContainers,
	} {
		for i := range *cc {
			injectEnvs(cluster, component, envConfigName, &(*cc)[i])
		}
	}
	return nil
}

func injectEnvs(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent, envConfigName string, c *corev1.Container) {
	// can not use map, it is unordered
	envFieldPathSlice := []struct {
		name      string
		fieldPath string
	}{
		{name: "KB_POD_NAME", fieldPath: "metadata.name"},
		{name: "KB_POD_UID", fieldPath: "metadata.uid"},
		{name: "KB_NAMESPACE", fieldPath: "metadata.namespace"},
		{name: "KB_SA_NAME", fieldPath: "spec.serviceAccountName"},
		{name: "KB_NODENAME", fieldPath: "spec.nodeName"},
		{name: "KB_HOST_IP", fieldPath: "status.hostIP"},
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
					FieldPath: v.fieldPath,
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

	if component.TLS {
		toInjectEnvs = append(toInjectEnvs, []corev1.EnvVar{
			{Name: "KB_TLS_CERT_PATH", Value: MountPath},
			{Name: "KB_TLS_CA_FILE", Value: CAName},
			{Name: "KB_TLS_CERT_FILE", Value: CertName},
			{Name: "KB_TLS_KEY_FILE", Value: KeyName},
		}...)
	}
	// have injected variables placed at the front of the slice
	if len(c.Env) == 0 {
		c.Env = toInjectEnvs
	} else {
		c.Env = append(toInjectEnvs, c.Env...)
	}
	if envConfigName == "" {
		return
	}
	c.EnvFrom = append(c.EnvFrom, corev1.EnvFromSource{
		ConfigMapRef: &corev1.ConfigMapEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: envConfigName,
			},
		},
	})
}

// BuildPersistentVolumeClaimLabels builds a pvc name label, and synchronize the labels on the sts to the pvc labels.
func BuildPersistentVolumeClaimLabels(sts *appsv1.StatefulSet, pvc *corev1.PersistentVolumeClaim,
	component *component.SynthesizedComponent, pvcTplName string) {
	// strict args checking.
	if sts == nil || pvc == nil || component == nil {
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

	for k, v := range sts.Labels {
		if _, ok := pvc.Labels[k]; !ok {
			pvc.Labels[k] = v
		}
	}
}

func BuildSvcListWithCustomAttributes(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent,
	customAttributeSetter func(*corev1.Service)) ([]*corev1.Service, error) {
	services, err := BuildSvcListLow(cluster, component)
	if err != nil {
		return nil, err
	}
	if customAttributeSetter != nil {
		for _, svc := range services {
			customAttributeSetter(svc)
		}
	}
	return services, nil
}

func BuildSvcListLow(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) ([]*corev1.Service, error) {
	const tplFile = "service_template.cue"
	var result = make([]*corev1.Service, 0)
	for _, item := range component.Services {
		if len(item.Spec.Ports) == 0 {
			continue
		}
		svc := corev1.Service{}
		if err := buildFromCUE(tplFile, map[string]any{
			"cluster":   cluster,
			"service":   item,
			"component": component,
		}, "svc", &svc); err != nil {
			return nil, err
		}
		result = append(result, &svc)
	}
	return result, nil
}

func BuildHeadlessSvc(params BuilderParams) (*corev1.Service, error) {
	return BuildHeadlessSvcLow(params.Cluster, params.Component)
}

func BuildHeadlessSvcLow(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) (*corev1.Service, error) {
	const tplFile = "headless_service_template.cue"
	service := corev1.Service{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":   cluster,
		"component": component,
	}, "service", &service); err != nil {
		return nil, err
	}
	return &service, nil
}

func BuildSts(reqCtx intctrlutil.RequestCtx, params BuilderParams, envConfigName string) (*appsv1.StatefulSet, error) {
	return BuildStsLow(reqCtx, params.Cluster, params.Component, envConfigName)
}

func BuildStsLow(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent,
	envConfigName string) (*appsv1.StatefulSet, error) {
	const tplFile = "statefulset_template.cue"

	sts := appsv1.StatefulSet{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":   cluster,
		"component": component,
	}, "statefulset", &sts); err != nil {
		return nil, err
	}

	if component.StatefulSetWorkload != nil {
		sts.Spec.PodManagementPolicy, sts.Spec.UpdateStrategy = component.StatefulSetWorkload.FinalStsUpdateStrategy()
	}

	// update sts.spec.volumeClaimTemplates[].metadata.labels
	if len(sts.Spec.VolumeClaimTemplates) > 0 && len(sts.GetLabels()) > 0 {
		for index, vct := range sts.Spec.VolumeClaimTemplates {
			BuildPersistentVolumeClaimLabels(&sts, &vct, component, vct.Name)
			sts.Spec.VolumeClaimTemplates[index] = vct
		}
	}

	if err := processContainersInjection(reqCtx, cluster, component, envConfigName, &sts.Spec.Template.Spec); err != nil {
		return nil, err
	}
	return &sts, nil
}

func randomString(length int) string {
	return rand.String(length)
}

func BuildConnCredential(params BuilderParams) (*corev1.Secret, error) {
	return BuildConnCredentialLow(params.ClusterDefinition, params.Cluster, params.Component)
}

func BuildConnCredentialLow(clusterDefiniiton *appsv1alpha1.ClusterDefinition, cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) (*corev1.Secret, error) {
	const tplFile = "conn_credential_template.cue"

	connCredential := corev1.Secret{}
	if err := buildFromCUE(tplFile, map[string]any{
		"clusterdefinition": clusterDefiniiton,
		"cluster":           cluster,
	}, "secret", &connCredential); err != nil {
		return nil, err
	}

	if len(connCredential.StringData) == 0 {
		return &connCredential, nil
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
	m := map[string]string{
		"$(RANDOM_PASSWD)":        randomString(8),
		"$(UUID)":                 uuidStr,
		"$(UUID_B64)":             uuidB64,
		"$(UUID_STR_B64)":         uuidStrB64,
		"$(UUID_HEX)":             uuidHex,
		"$(SVC_FQDN)":             fmt.Sprintf("%s-%s.%s.svc", cluster.Name, component.Name, cluster.Namespace),
		"$(KB_CLUSTER_COMP_NAME)": cluster.Name + "-" + component.Name,
		"$(HEADLESS_SVC_FQDN)":    fmt.Sprintf("%s-%s-headless.%s.svc", cluster.Name, component.Name, cluster.Namespace),
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
	return &connCredential, nil
}

func BuildPDB(params BuilderParams) (*policyv1.PodDisruptionBudget, error) {
	return BuildPDBLow(params.Cluster, params.Component)
}

func BuildPDBLow(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) (*policyv1.PodDisruptionBudget, error) {
	const tplFile = "pdb_template.cue"
	pdb := policyv1.PodDisruptionBudget{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":   cluster,
		"component": component,
	}, "pdb", &pdb); err != nil {
		return nil, err
	}
	return &pdb, nil
}

func BuildDeploy(reqCtx intctrlutil.RequestCtx, params BuilderParams) (*appsv1.Deployment, error) {
	return BuildDeployLow(reqCtx, params.Cluster, params.Component)
}

func BuildDeployLow(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) (*appsv1.Deployment, error) {
	const tplFile = "deployment_template.cue"
	deploy := appsv1.Deployment{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":   cluster,
		"component": component,
	}, "deployment", &deploy); err != nil {
		return nil, err
	}

	if component.StatelessSpec != nil {
		deploy.Spec.Strategy = component.StatelessSpec.UpdateStrategy
	}
	if err := processContainersInjection(reqCtx, cluster, component, "", &deploy.Spec.Template.Spec); err != nil {
		return nil, err
	}
	return &deploy, nil
}

func BuildPVCFromSnapshot(sts *appsv1.StatefulSet,
	vct corev1.PersistentVolumeClaimTemplate,
	pvcKey types.NamespacedName,
	snapshotName string,
	component *component.SynthesizedComponent) (*corev1.PersistentVolumeClaim, error) {
	pvc := corev1.PersistentVolumeClaim{}
	if err := buildFromCUE("pvc_template.cue", map[string]any{
		"sts":                 sts,
		"volumeClaimTemplate": vct,
		"pvc_key":             pvcKey,
		"snapshot_name":       snapshotName,
	}, "pvc", &pvc); err != nil {
		return nil, err
	}
	BuildPersistentVolumeClaimLabels(sts, &pvc, component, vct.Name)
	return &pvc, nil
}

// BuildEnvConfig build cluster component context ConfigMap object, which is to be used in workload container's
// envFrom.configMapRef with name of "$(cluster.metadata.name)-$(component.name)-env" pattern.
func BuildEnvConfig(params BuilderParams, reqCtx intctrlutil.RequestCtx, cli client.Client) (*corev1.ConfigMap, error) {
	return BuildEnvConfigLow(reqCtx, cli, params.Cluster, params.Component)
}

func BuildEnvConfigLow(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) (*corev1.ConfigMap, error) {

	const tplFile = "env_config_template.cue"

	prefix := constant.KBPrefix + "_" + strings.ToUpper(component.Type) + "_"
	svcName := strings.Join([]string{cluster.Name, component.Name, "headless"}, "-")
	envData := map[string]string{}
	envData[prefix+"N"] = strconv.Itoa(int(component.Replicas))
	for j := 0; j < int(component.Replicas); j++ {
		hostNameTplKey := prefix + strconv.Itoa(j) + "_HOSTNAME"
		hostNameTplValue := cluster.Name + "-" + component.Name + "-" + strconv.Itoa(j)
		envData[hostNameTplKey] = fmt.Sprintf("%s.%s", hostNameTplValue, svcName)

		// build env for replication workload
		if component.WorkloadType == appsv1alpha1.Replication {
			envData[constant.KBReplicationSetPrimaryPodName] =
				fmt.Sprintf("%s-%s-%d.%s", cluster.Name, component.Name, component.GetPrimaryIndex(), svcName)
		}
	}

	// TODO following code seems to be redundant with updateConsensusRoleInfo in consensus_set_utils.go
	// build consensus env from cluster.status
	if cluster.Status.Components != nil {
		if v, ok := cluster.Status.Components[component.Name]; ok {
			consensusSetStatus := v.ConsensusSetStatus
			if consensusSetStatus != nil {
				if consensusSetStatus.Leader.Pod != constant.ComponentStatusDefaultPodName {
					envData[prefix+"LEADER"] = consensusSetStatus.Leader.Pod
				}

				followers := ""
				for _, follower := range consensusSetStatus.Followers {
					if follower.Pod == constant.ComponentStatusDefaultPodName {
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

	// set cluster uid to let pod know if the cluster is recreated
	envData[prefix+"CLUSTER_UID"] = string(cluster.UID)

	config := corev1.ConfigMap{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":     cluster,
		"component":   component,
		"config.data": envData,
	}, "config", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func BuildBackup(sts *appsv1.StatefulSet,
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

func BuildVolumeSnapshot(snapshotKey types.NamespacedName,
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

func BuildCronJob(pvcKey types.NamespacedName,
	schedule string,
	sts *appsv1.StatefulSet) (*batchv1.CronJob, error) {
	serviceAccount := viper.GetString("KUBEBLOCKS_SERVICEACCOUNT_NAME")
	cronJob := batchv1.CronJob{}
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

func BuildConfigMapWithTemplate(configs map[string]string,
	params BuilderParams,
	cmName string,
	configConstraintName string,
	tplCfg appsv1alpha1.ComponentTemplateSpec) (*corev1.ConfigMap, error) {
	return BuildConfigMapWithTemplateLow(params.Cluster, params.Component, configs, cmName, configConstraintName, tplCfg)
}

func BuildConfigMapWithTemplateLow(cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	configs map[string]string,
	cmName string,
	configConstraintName string,
	tplCfg appsv1alpha1.ComponentTemplateSpec) (*corev1.ConfigMap, error) {
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
			"name": cluster.Spec.ClusterDefRef,
		},
		"cluster": {
			"name":      cluster.GetName(),
			"namespace": cluster.GetNamespace(),
		},
		"component": {
			"name":                  component.Name,
			"type":                  component.Type,
			"characterType":         component.CharacterType,
			"configName":            cmName,
			"templateName":          tplCfg.TemplateRef,
			"configConstraintsName": configConstraintName,
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

func BuildCfgManagerContainer(sidecarRenderedParam *cfgcm.CfgManagerBuildParams) (*corev1.Container, error) {
	const tplFile = "config_manager_sidecar.cue"
	cueFS, _ := debme.FS(cueTemplates, "cue")
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

func BuildTLSSecret(namespace, clusterName, componentName string) (*corev1.Secret, error) {
	const tplFile = "tls_certs_secret_template.cue"
	secret := &corev1.Secret{}
	pathedName := componentPathedName{
		Namespace:   namespace,
		ClusterName: clusterName,
		Name:        componentName,
	}
	if err := buildFromCUE(tplFile, map[string]any{"pathedName": pathedName}, "secret", secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func BuildBackupManifestsJob(key types.NamespacedName, backup *dataprotectionv1alpha1.Backup, podSpec *corev1.PodSpec) (*batchv1.Job, error) {
	const tplFile = "backup_manifests_template.cue"
	job := &batchv1.Job{}
	if err := buildFromCUE(tplFile,
		map[string]any{
			"job.metadata.name":      key.Name,
			"job.metadata.namespace": key.Namespace,
			"backup":                 backup,
			"podSpec":                podSpec,
		},
		"job", job); err != nil {
		return nil, err
	}
	return job, nil
}

func BuildPITRJob(name string, cluster *appsv1alpha1.Cluster, image string, command []string, args []string,
	volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, env []corev1.EnvVar) (*batchv1.Job, error) {
	const tplFile = "pitr_job_template.cue"
	job := &batchv1.Job{}
	if err := buildFromCUE(tplFile, map[string]any{
		"job.metadata.name":              name,
		"job.metadata.namespace":         cluster.Namespace,
		"job.spec.template.spec.volumes": volumes,
		"container.image":                image,
		"container.command":              command,
		"container.args":                 args,
		"container.volumeMounts":         volumeMounts,
		"container.env":                  env,
	}, "job", job); err != nil {
		return nil, err
	}
	return job, nil
}
