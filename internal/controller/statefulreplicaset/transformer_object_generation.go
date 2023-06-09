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

package statefulreplicaset

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	"github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ObjectGenerationTransformer struct{}

func (t *ObjectGenerationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*SRSTransformContext)
	srs := transCtx.srs
	srsOrig := transCtx.srsOrig

	if model.IsObjectDeleting(srsOrig) {
		return nil
	}

	// generate objects by current spec
	svc := buildSvc(*srs)
	headLessSvc := buildHeadlessSvc(*srs)
	envConfig := buildEnvConfigMap(*srs)
	sts := buildSts(*srs, headLessSvc.Name, *envConfig)
	objects := []client.Object{svc, headLessSvc, envConfig, sts}

	for _, object := range objects {
		if err := controllerutil.SetOwnership(srs, object, model.GetScheme(), srsFinalizerName); err != nil {
			return err
		}
	}

	// read cache snapshot
	ml := client.MatchingLabels{model.AppInstanceLabelKey: srs.Name, model.KBManagedByKey: kindStatefulReplicaSet}
	oldSnapshot, err := model.ReadCacheSnapshot(ctx, srs, ml, ownedKinds()...)
	if err != nil {
		return err
	}

	// compute create/update/delete set
	newSnapshot := make(map[model.GVKName]client.Object)
	for _, object := range objects {
		name, err := model.GetGVKName(object)
		if err != nil {
			return err
		}
		newSnapshot[*name] = object
	}

	// now compute the diff between old and target snapshot and generate the plan
	oldNameSet := sets.KeySet(oldSnapshot)
	newNameSet := sets.KeySet(newSnapshot)

	createSet := newNameSet.Difference(oldNameSet)
	updateSet := newNameSet.Intersection(oldNameSet)
	deleteSet := oldNameSet.Difference(newNameSet)

	createNewObjects := func() {
		for name := range createSet {
			model.PrepareCreate(dag, newSnapshot[name])
		}
	}
	updateObjects := func() {
		for name := range updateSet {
			model.PrepareUpdate(dag, oldSnapshot[name], newSnapshot[name])
		}
	}
	deleteOrphanObjects := func() {
		for name := range deleteSet {
			model.PrepareDelete(dag, oldSnapshot[name])
		}
	}

	handleDependencies := func() {
		model.DependOn(dag, sts, svc, headLessSvc, envConfig)
	}

	// objects to be created
	createNewObjects()
	// objects to be updated
	updateObjects()
	// objects to be deleted
	deleteOrphanObjects()
	// handle object dependencies
	handleDependencies()

	return nil
}

func buildSvc(srs workloads.StatefulReplicaSet) *corev1.Service {
	svcBuilder := builder.NewServiceBuilder(srs.Namespace, srs.Name).
		AddLabels(model.AppInstanceLabelKey, srs.Name).
		AddLabels(model.KBManagedByKey, kindStatefulReplicaSet).
		// AddAnnotationsInMap(srs.Annotations).
		AddSelectors(model.AppInstanceLabelKey, srs.Name).
		AddSelectors(model.KBManagedByKey, kindStatefulReplicaSet).
		AddPorts(srs.Spec.Service.Ports...).
		SetType(srs.Spec.Service.Type)
	for _, role := range srs.Spec.Roles {
		if role.IsLeader && len(role.Name) > 0 {
			svcBuilder.AddSelectors(srsAccessModeLabelKey, string(role.AccessMode))
		}
	}
	return svcBuilder.GetObject()
}

func buildHeadlessSvc(srs workloads.StatefulReplicaSet) *corev1.Service {
	hdlBuilder := builder.NewHeadlessServiceBuilder(srs.Namespace, getHeadlessSvcName(srs)).
		AddLabels(model.AppInstanceLabelKey, srs.Name).
		AddLabels(model.KBManagedByKey, kindStatefulReplicaSet).
		AddSelectors(model.AppInstanceLabelKey, srs.Name).
		AddSelectors(model.KBManagedByKey, kindStatefulReplicaSet)
	//	.AddAnnotations("prometheus.io/scrape", strconv.FormatBool(component.Monitor.Enable))
	// if component.Monitor.Enable {
	//	hdBuilder.AddAnnotations("prometheus.io/path", component.Monitor.ScrapePath).
	//		AddAnnotations("prometheus.io/port", strconv.Itoa(int(component.Monitor.ScrapePort))).
	//		AddAnnotations("prometheus.io/scheme", "http")
	// }
	for _, container := range srs.Spec.Template.Spec.Containers {
		for _, port := range container.Ports {
			servicePort := corev1.ServicePort{
				Protocol: port.Protocol,
				Port:     port.ContainerPort,
			}
			switch {
			case len(port.Name) > 0:
				servicePort.Name = port.Name
				servicePort.TargetPort = intstr.FromString(port.Name)
			default:
				servicePort.Name = fmt.Sprintf("%s-%d", strings.ToLower(string(port.Protocol)), port.ContainerPort)
				servicePort.TargetPort = intstr.FromInt(int(port.ContainerPort))
			}
			hdlBuilder.AddPorts(servicePort)
		}
	}
	return hdlBuilder.GetObject()
}

func buildSts(srs workloads.StatefulReplicaSet, headlessSvcName string, envConfig corev1.ConfigMap) *apps.StatefulSet {
	stsBuilder := builder.NewStatefulSetBuilder(srs.Namespace, srs.Name)
	template := buildStsPodTemplate(srs, envConfig)
	stsBuilder.AddLabels(model.AppInstanceLabelKey, srs.Name).
		AddLabels(model.KBManagedByKey, kindStatefulReplicaSet).
		AddMatchLabel(model.AppInstanceLabelKey, srs.Name).
		AddMatchLabel(model.KBManagedByKey, kindStatefulReplicaSet).
		SetServiceName(headlessSvcName).
		SetReplicas(srs.Spec.Replicas).
		SetPodManagementPolicy(apps.OrderedReadyPodManagement).
		SetVolumeClaimTemplates(srs.Spec.VolumeClaimTemplates...).
		SetTemplate(*template).
		SetUpdateStrategyType(apps.OnDeleteStatefulSetStrategyType)
	return stsBuilder.GetObject()
}

func buildEnvConfigMap(srs workloads.StatefulReplicaSet) *corev1.ConfigMap {
	envData := buildEnvConfigData(srs)
	return builder.NewConfigMapBuilder(srs.Namespace, srs.Name+"-env").
		AddLabels(model.AppInstanceLabelKey, srs.Name).
		AddLabels(model.KBManagedByKey, kindStatefulReplicaSet).
		SetData(envData).GetObject()
}

func buildStsPodTemplate(srs workloads.StatefulReplicaSet, envConfig corev1.ConfigMap) *corev1.PodTemplateSpec {
	template := srs.Spec.Template
	labels := template.Labels
	if labels == nil {
		labels = make(map[string]string, 2)
	}
	labels[model.AppInstanceLabelKey] = srs.Name
	labels[model.KBManagedByKey] = kindStatefulReplicaSet
	template.Labels = labels

	// inject env ConfigMap into workload pods only
	for i := range template.Spec.Containers {
		template.Spec.Containers[i].EnvFrom = append(template.Spec.Containers[i].EnvFrom,
			corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: envConfig.Name,
					},
					Optional: func() *bool { optional := false; return &optional }(),
				}})
	}

	injectRoleObservationContainer(srs, &template)

	return &template
}

func injectRoleObservationContainer(srs workloads.StatefulReplicaSet, template *corev1.PodTemplateSpec) {
	roleObservation := srs.Spec.RoleObservation
	credential := srs.Spec.Credential
	credentialEnv := make([]corev1.EnvVar, 0)
	if credential != nil {
		credentialEnv = append(credentialEnv,
			corev1.EnvVar{
				Name:      usernameCredentialVarName,
				Value:     credential.Username.Value,
				ValueFrom: credential.Username.ValueFrom,
			},
			corev1.EnvVar{
				Name:      passwordCredentialVarName,
				Value:     credential.Password.Value,
				ValueFrom: credential.Password.ValueFrom,
			})
	}
	allUsedPorts := findAllUsedPorts(template)
	svcPort := actionSvcPortBase
	var actionSvcPorts []int32
	for range roleObservation.ObservationActions {
		svcPort = findNextAvailablePort(svcPort, allUsedPorts)
		actionSvcPorts = append(actionSvcPorts, svcPort)
	}
	injectObservationActionContainer(srs, template, actionSvcPorts, credentialEnv)
	actionSvcList, _ := json.Marshal(actionSvcPorts)
	injectRoleObserveContainer(srs, template, string(actionSvcList), credentialEnv)
}

func findNextAvailablePort(base int32, allUsedPorts []int32) int32 {
	for port := base + 1; port < 65535; port++ {
		available := true
		for _, usedPort := range allUsedPorts {
			if port == usedPort {
				available = false
				break
			}
		}
		if available {
			return port
		}
	}
	return 0
}

func findAllUsedPorts(template *corev1.PodTemplateSpec) []int32 {
	allUsedPorts := make([]int32, 0)
	for _, container := range template.Spec.Containers {
		for _, port := range container.Ports {
			allUsedPorts = append(allUsedPorts, port.ContainerPort)
			allUsedPorts = append(allUsedPorts, port.HostPort)
		}
	}
	return allUsedPorts
}

func injectRoleObserveContainer(srs workloads.StatefulReplicaSet, template *corev1.PodTemplateSpec, actionSvcList string, credentialEnv []corev1.EnvVar) {
	// compute parameters for role observation container
	roleObservation := srs.Spec.RoleObservation
	credential := srs.Spec.Credential
	image := viper.GetString("ROLE_OBSERVATION_IMAGE")
	if len(image) == 0 {
		image = defaultRoleObservationImage
	}
	observationDaemonPort := viper.GetInt("ROLE_OBSERVATION_SERVICE_PORT")
	if observationDaemonPort == 0 {
		observationDaemonPort = defaultRoleObservationDaemonPort
	}
	roleObserveURI := fmt.Sprintf(roleObservationURIFormat, strconv.Itoa(observationDaemonPort))
	env := credentialEnv
	env = append(env,
		corev1.EnvVar{
			Name:  actionSvcListVarName,
			Value: actionSvcList,
		})
	if credential != nil {
		// for compatibility with old probe env var names
		env = append(env,
			corev1.EnvVar{
				Name:      "KB_SERVICE_USER",
				Value:     credential.Username.Value,
				ValueFrom: credential.Username.ValueFrom,
			},
			corev1.EnvVar{
				Name:      "KB_SERVICE_PASSWORD",
				Value:     credential.Password.Value,
				ValueFrom: credential.Password.ValueFrom,
			})
	}
	// find service port of th db engine
	servicePort := findSvcPort(srs)
	if servicePort > 0 {
		env = append(env,
			corev1.EnvVar{
				Name:  servicePortVarName,
				Value: strconv.Itoa(servicePort),
			},
			// for compatibility with old probe env var names
			corev1.EnvVar{
				Name:  "KB_SERVICE_PORT",
				Value: strconv.Itoa(servicePort),
			})
	}

	// build container
	container := corev1.Container{
		Name:            roleObservationName,
		Image:           image,
		ImagePullPolicy: "IfNotPresent",
		Command: []string{
			"role-agent",
			"--port", strconv.Itoa(observationDaemonPort),
		},
		Ports: []corev1.ContainerPort{{
			ContainerPort: int32(observationDaemonPort),
			Name:          roleObservationName,
			Protocol:      "TCP",
		}},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"curl", "-X", "POST",
						"--max-time", "1",
						"--fail-with-body", "--silent",
						"-H", "Content-Type: application/json",
						roleObserveURI,
					},
				},
			},
			InitialDelaySeconds: roleObservation.InitialDelaySeconds,
			TimeoutSeconds:      roleObservation.TimeoutSeconds,
			PeriodSeconds:       roleObservation.PeriodSeconds,
			SuccessThreshold:    roleObservation.SuccessThreshold,
			FailureThreshold:    roleObservation.FailureThreshold,
		},
		Env: env,
	}

	// inject role observation container
	template.Spec.Containers = append(template.Spec.Containers, container)
}

func injectObservationActionContainer(srs workloads.StatefulReplicaSet, template *corev1.PodTemplateSpec, actionSvcPorts []int32, credentialEnv []corev1.EnvVar) {
	// inject shared volume
	agentVolume := corev1.Volume{
		Name: roleAgentVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	template.Spec.Volumes = append(template.Spec.Volumes, agentVolume)

	// inject init container
	agentVolumeMount := corev1.VolumeMount{
		Name:      roleAgentVolumeName,
		MountPath: roleAgentVolumeMountPath,
	}
	agentPath := strings.Join([]string{roleAgentVolumeMountPath, roleAgentName}, "/")
	initContainer := corev1.Container{
		Name:            roleAgentInstallerName,
		Image:           shell2httpImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		VolumeMounts:    []corev1.VolumeMount{agentVolumeMount},
		Command: []string{
			"cp",
			shell2httpBinaryPath,
			agentPath,
		},
	}
	template.Spec.InitContainers = append(template.Spec.InitContainers, initContainer)

	// inject action containers based on utility images
	for i, action := range srs.Spec.RoleObservation.ObservationActions {
		image := action.Image
		if len(image) == 0 {
			image = defaultActionImage
		}
		command := []string{
			agentPath,
			"-port", fmt.Sprintf("%d", actionSvcPorts[i]),
			"-export-all-vars",
			"-form",
			shell2httpServePath,
			strings.Join(action.Command, " "),
		}
		container := corev1.Container{
			Name:            fmt.Sprintf("action-%d", i),
			Image:           image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			VolumeMounts:    []corev1.VolumeMount{agentVolumeMount},
			Env:             credentialEnv,
			Command:         command,
		}
		template.Spec.Containers = append(template.Spec.Containers, container)
	}
}

func buildEnvConfigData(set workloads.StatefulReplicaSet) map[string]string {
	envData := map[string]string{}

	prefix := constant.KBPrefix + "_SRS_"
	svcName := getHeadlessSvcName(set)
	envData[prefix+"N"] = strconv.Itoa(int(set.Spec.Replicas))
	for i := 0; i < int(set.Spec.Replicas); i++ {
		hostNameTplKey := prefix + strconv.Itoa(i) + "_HOSTNAME"
		hostNameTplValue := set.Name + "-" + strconv.Itoa(i)
		envData[hostNameTplKey] = fmt.Sprintf("%s.%s", hostNameTplValue, svcName)
	}

	// build member related envs from set.Status.MembersStatus
	followers := ""
	for _, memberStatus := range set.Status.MembersStatus {
		if memberStatus.PodName == "" || memberStatus.PodName == defaultPodName {
			continue
		}
		switch {
		case memberStatus.IsLeader:
			envData[prefix+"LEADER"] = memberStatus.PodName
		case memberStatus.CanVote:
			if len(followers) > 0 {
				followers += ","
			}
			followers += memberStatus.PodName
		}
	}
	if followers != "" {
		envData[prefix+"FOLLOWERS"] = followers
	}

	// set owner uid to let pod know if the owner is recreated
	uid := string(set.UID)
	envData[prefix+"OWNER_UID"] = uid
	envData[prefix+"OWNER_UID_SUFFIX8"] = uid[len(uid)-4:]

	return envData
}

var _ graph.Transformer = &ObjectGenerationTransformer{}
