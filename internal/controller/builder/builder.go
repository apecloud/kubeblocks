/*
Copyright ApeCloud, Inc.

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

package builder

import (
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/leaanthony/debme"
	"github.com/sethvargo/go-password/password"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/types"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	componentutil "github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/configmap"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type BuilderParams struct {
	ClusterDefinition *dbaasv1alpha1.ClusterDefinition
	ClusterVersion    *dbaasv1alpha1.ClusterVersion
	Cluster           *dbaasv1alpha1.Cluster
	Component         *component.Component
}

type envVar struct {
	name      string
	fieldPath string
	value     string
}

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
	params BuilderParams,
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

func injectEnvs(params BuilderParams, envConfigName string, c *corev1.Container) {
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
		{name: "_CLUSTER_NAME", value: params.Cluster.Name},
		{name: "_COMP_NAME", value: params.Component.Name},
		{name: "_CLUSTER_COMP_NAME", value: params.Cluster.Name + "-" + params.Component.Name},
	}
	toInjectEnv := make([]corev1.EnvVar, 0, len(envFieldPathSlice)+len(c.Env))
	for _, v := range envFieldPathSlice {
		toInjectEnv = append(toInjectEnv, corev1.EnvVar{
			Name: component.KBPrefix + v.name,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: v.fieldPath,
				},
			},
		})
	}

	for _, v := range clusterEnv {
		toInjectEnv = append(toInjectEnv, corev1.EnvVar{
			Name:  component.KBPrefix + v.name,
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

// buildPersistentVolumeClaimLabels builds a pvc name label, and synchronize the labels on the sts to the pvc labels.
func buildPersistentVolumeClaimLabels(sts *appsv1.StatefulSet, pvc *corev1.PersistentVolumeClaim) {
	if pvc.Labels == nil {
		pvc.Labels = make(map[string]string)
	}
	pvc.Labels[intctrlutil.VolumeClaimTemplateNameLabelKey] = pvc.Name
	for k, v := range sts.Labels {
		if _, ok := pvc.Labels[k]; !ok {
			pvc.Labels[k] = v
		}
	}
}

func BuildSvc(params BuilderParams, headless bool) (*corev1.Service, error) {
	tplFile := "service_template.cue"
	if headless {
		tplFile = "headless_service_template.cue"
	}
	svc := corev1.Service{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":   params.Cluster,
		"component": params.Component,
	}, "service", &svc); err != nil {
		return nil, err
	}

	return &svc, nil
}

func BuildSts(reqCtx intctrlutil.RequestCtx, params BuilderParams, envConfigName string) (*appsv1.StatefulSet, error) {
	const tplFile = "statefulset_template.cue"

	sts := appsv1.StatefulSet{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":   params.Cluster,
		"component": params.Component,
	}, "statefulset", &sts); err != nil {
		return nil, err
	}

	// update sts.spec.volumeClaimTemplates[].metadata.labels
	if len(sts.Spec.VolumeClaimTemplates) > 0 && len(sts.GetLabels()) > 0 {
		for index, vct := range sts.Spec.VolumeClaimTemplates {
			buildPersistentVolumeClaimLabels(&sts, &vct)
			sts.Spec.VolumeClaimTemplates[index] = vct
		}
	}

	if err := processContainersInjection(reqCtx, params, envConfigName, &sts.Spec.Template.Spec); err != nil {
		return nil, err
	}
	return &sts, nil
}

func randomString(length int) string {
	res, _ := password.Generate(length, 0, 0, false, false)
	return res
}

func BuildConnCredential(params BuilderParams) (*corev1.Secret, error) {
	const tplFile = "conn_credential_template.cue"

	connCredential := corev1.Secret{}
	if err := buildFromCUE(tplFile, map[string]any{
		"clusterdefinition": params.ClusterDefinition,
		"cluster":           params.Cluster,
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

func BuildPDB(params BuilderParams) (*policyv1.PodDisruptionBudget, error) {
	const tplFile = "pdb_template.cue"
	pdb := policyv1.PodDisruptionBudget{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":   params.Cluster,
		"component": params.Component,
	}, "pdb", &pdb); err != nil {
		return nil, err
	}

	return &pdb, nil
}

func BuildDeploy(reqCtx intctrlutil.RequestCtx, params BuilderParams) (*appsv1.Deployment, error) {
	const tplFile = "deployment_template.cue"

	deploy := appsv1.Deployment{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":   params.Cluster,
		"component": params.Component,
	}, "deployment", &deploy); err != nil {
		return nil, err
	}

	if err := processContainersInjection(reqCtx, params, "", &deploy.Spec.Template.Spec); err != nil {
		return nil, err
	}
	return &deploy, nil
}

func BuildPVCFromSnapshot(sts *appsv1.StatefulSet,
	vct corev1.PersistentVolumeClaim,
	pvcKey types.NamespacedName,
	snapshotName string) (*corev1.PersistentVolumeClaim, error) {

	pvc := corev1.PersistentVolumeClaim{}
	if err := buildFromCUE("pvc_template.cue", map[string]any{
		"sts":                 sts,
		"volumeClaimTemplate": vct,
		"pvc_key":             pvcKey,
		"snapshot_name":       snapshotName,
	}, "pvc", &pvc); err != nil {
		return nil, err
	}

	return &pvc, nil
}

func BuildEnvConfig(params BuilderParams) (*corev1.ConfigMap, error) {
	const tplFile = "env_config_template.cue"

	prefix := component.KBPrefix + "_" + strings.ToUpper(params.Component.Type) + "_"
	svcName := strings.Join([]string{params.Cluster.Name, params.Component.Name, "headless"}, "-")
	envData := map[string]string{}
	envData[prefix+"N"] = strconv.Itoa(int(params.Component.Replicas))
	for j := 0; j < int(params.Component.Replicas); j++ {
		envData[prefix+strconv.Itoa(j)+"_HOSTNAME"] = fmt.Sprintf("%s.%s", params.Cluster.Name+"-"+params.Component.Name+"-"+strconv.Itoa(j), svcName)
	}
	// TODO following code seems to be redundant with updateConsensusRoleInfo in consensus_set_utils.go
	// build consensus env from cluster.status
	if params.Cluster.Status.Components != nil {
		if v, ok := params.Cluster.Status.Components[params.Component.Name]; ok {
			consensusSetStatus := v.ConsensusSetStatus
			if consensusSetStatus != nil {
				if consensusSetStatus.Leader.Pod != componentutil.ComponentStatusDefaultPodName {
					envData[prefix+"LEADER"] = consensusSetStatus.Leader.Pod
				}

				followers := ""
				for _, follower := range consensusSetStatus.Followers {
					if follower.Pod == componentutil.ComponentStatusDefaultPodName {
						continue
					}
					if len(followers) > 0 {
						followers += ","
					}
					followers += follower.Pod
				}
				envData[prefix+"FOLLOWERS"] = followers
			}
			replicationSetStatus := v.ReplicationSetStatus
			if replicationSetStatus != nil {
				if replicationSetStatus.Primary.Pod != componentutil.ComponentStatusDefaultPodName {
					envData[prefix+"PRIMARY"] = replicationSetStatus.Primary.Pod
				}
				secondaries := ""
				for _, secondary := range replicationSetStatus.Secondaries {
					if secondary.Pod == componentutil.ComponentStatusDefaultPodName {
						continue
					}
					if len(secondaries) > 0 {
						secondaries += ","
					}
					secondaries += secondary.Pod
				}
				envData[prefix+"SECONDARIES"] = secondaries
			}
		}
	}

	config := corev1.ConfigMap{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":     params.Cluster,
		"component":   params.Component,
		"config.data": envData,
	}, "config", &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func BuildBackupPolicy(sts *appsv1.StatefulSet,
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

func BuildConfigMapWithTemplate(
	configs map[string]string,
	params BuilderParams,
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
			"name": params.ClusterDefinition.GetName(),
			"type": params.ClusterDefinition.Spec.Type,
		},
		"cluster": {
			"name":      params.Cluster.GetName(),
			"namespace": params.Cluster.GetNamespace(),
		},
		"component": {
			"name":                  params.Component.Name,
			"type":                  params.Component.Type,
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

func BuildCfgManagerContainer(sidecarRenderedParam *cfgcm.ConfigManagerSidecar) (*corev1.Container, error) {
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
