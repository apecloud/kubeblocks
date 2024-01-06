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

package rsm

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type ObjectGenerationTransformer struct{}

var _ graph.Transformer = &ObjectGenerationTransformer{}

func (t *ObjectGenerationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*rsmTransformContext)
	rsm := transCtx.rsm
	rsmOrig := transCtx.rsmOrig
	cli, _ := transCtx.Client.(model.GraphClient)

	if model.IsObjectDeleting(rsmOrig) {
		return nil
	}

	// generate objects by current spec
	var objects []client.Object
	if rsm.Spec.RsmTransformPolicy == workloads.ToPod {
		pods := buildPods(*rsm)
		for idx := range pods {
			pod := pods[idx]
			objects = append(objects, pod)
		}
	} else {
		svc := buildSvc(*rsm)
		altSvs := buildAlternativeSvs(*rsm)
		headLessSvc := buildHeadlessSvc(*rsm)
		envConfig := buildEnvConfigMap(*rsm)
		sts := buildSts(*rsm, headLessSvc.Name, *envConfig)
		objects = append(objects, sts)
		objects = append(objects, headLessSvc, envConfig)
		if svc != nil {
			objects = append(objects, svc)
		}
		for _, s := range altSvs {
			objects = append(objects, s)
		}
	}

	for _, object := range objects {
		if err := setOwnership(rsm, object, model.GetScheme(), getFinalizer(object)); err != nil {
			return err
		}
	}

	// read cache snapshot
	ml := getLabels(rsm)
	oldSnapshot, err := model.ReadCacheSnapshot(ctx, rsm, ml, ownedKinds(rsm.Spec.RsmTransformPolicy)...)
	if err != nil {
		return err
	}

	// compute create/update/delete set
	newSnapshot := make(map[model.GVKNObjKey]client.Object)
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
			cli.Create(dag, newSnapshot[name])
		}
	}
	updateObjects := func() {
		for name := range updateSet {
			oldObj := oldSnapshot[name]
			newObj := copyAndMerge(oldObj, newSnapshot[name])
			if reflect.DeepEqual(oldObj, newObj) {
				continue
			}
			cli.Update(dag, oldObj, newObj)
		}
	}
	deleteOrphanObjects := func() {
		for name := range deleteSet {
			if viper.GetBool(FeatureGateRSMCompatibilityMode) {
				// filter non-env configmaps
				if _, ok := oldSnapshot[name].(*corev1.ConfigMap); ok {
					continue
				}
			}
			cli.Delete(dag, oldSnapshot[name])
		}
	}
	handleDependencies := func() {
		// RsmTransformPolicy might be "", treat empty as ToSts for backward compatibility
		if rsm.Spec.RsmTransformPolicy != workloads.ToPod {
			// objects[0] is the sts object
			cli.DependOn(dag, objects[0], objects[1:]...)
		}
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

// copyAndMerge merges two objects for updating:
// 1. new an object targetObj by copying from oldObj
// 2. merge all fields can be updated from newObj into targetObj
func copyAndMerge(oldObj, newObj client.Object) client.Object {
	if reflect.TypeOf(oldObj) != reflect.TypeOf(newObj) {
		return nil
	}

	// mergeMetadataMap keeps the original elements.
	mergeMetadataMap := func(originalMap map[string]string, targetMap map[string]string) map[string]string {
		if targetMap == nil && originalMap == nil {
			return nil
		}
		if targetMap == nil {
			targetMap = map[string]string{}
		}
		for k, v := range originalMap {
			// if the element not exist in targetMap, copy it from original.
			if _, ok := (targetMap)[k]; !ok {
				(targetMap)[k] = v
			}
		}
		return targetMap
	}

	getRoleProbeContainerIndex := func(containers []corev1.Container) int {
		return slices.IndexFunc(containers, func(c corev1.Container) bool {
			return c.Name == roleProbeContainerName
		})
	}

	copyAndMergeSts := func(oldSts, newSts *apps.StatefulSet) client.Object {
		oldSts.Labels = mergeMetadataMap(oldSts.Labels, newSts.Labels)

		// for upgrade compatibility from 0.7 to 0.8
		oldRoleProbeContainerIndex := getRoleProbeContainerIndex(oldSts.Spec.Template.Spec.Containers)
		newRoleProbeContainerIndex := getRoleProbeContainerIndex(newSts.Spec.Template.Spec.Containers)
		if oldRoleProbeContainerIndex >= 0 && newRoleProbeContainerIndex >= 0 {
			newCopySts := newSts.DeepCopy()
			oldCopySts := newSts.DeepCopy()
			oldCopySts.Spec.Template = oldSts.Spec.Template
			oldCopySts.Spec.Replicas = oldSts.Spec.Replicas
			oldCopySts.Spec.UpdateStrategy = oldSts.Spec.UpdateStrategy
			newCopySts.Spec.Template.Spec.Containers[newRoleProbeContainerIndex] = *oldSts.Spec.Template.Spec.Containers[oldRoleProbeContainerIndex].DeepCopy()

			if reflect.DeepEqual(newCopySts.Spec, oldCopySts.Spec) {
				newSts = newCopySts
			}
		}
		// if annotations exist and are replaced, the StatefulSet will be updated.
		oldSts.Annotations = mergeMetadataMap(oldSts.Annotations, newSts.Annotations)
		oldSts.Spec.Template = newSts.Spec.Template
		oldSts.Spec.Replicas = newSts.Spec.Replicas
		oldSts.Spec.UpdateStrategy = newSts.Spec.UpdateStrategy
		return oldSts
	}

	copyAndMergeSvc := func(oldSvc *corev1.Service, newSvc *corev1.Service) client.Object {
		oldSvc.Annotations = mergeMetadataMap(oldSvc.Annotations, newSvc.Annotations)
		oldSvc.Spec = newSvc.Spec
		return oldSvc
	}

	copyAndMergeCm := func(oldCm, newCm *corev1.ConfigMap) client.Object {
		oldCm.Data = newCm.Data
		oldCm.BinaryData = newCm.BinaryData
		return oldCm
	}

	copyAndMergePod := func(oldPod, newPod *corev1.Pod) client.Object {
		oldPod.Spec.ActiveDeadlineSeconds = newPod.Spec.ActiveDeadlineSeconds
		for idx := range oldPod.Spec.Containers {
			oldPod.Spec.Containers[idx].Image = newPod.Spec.Containers[idx].Image
		}
		for idx := range oldPod.Spec.InitContainers {
			oldPod.Spec.InitContainers[idx].Image = newPod.Spec.InitContainers[idx].Image
		}
		// TODO `spec.tolerations` (only additions to existing tolerations),`spec.terminationGracePeriodSeconds` (allow it to be set to 1 if it was previously negative)
		return oldPod
	}

	targetObj := oldObj.DeepCopyObject()
	switch o := newObj.(type) {
	case *apps.StatefulSet:
		return copyAndMergeSts(targetObj.(*apps.StatefulSet), o)
	case *corev1.Service:
		return copyAndMergeSvc(targetObj.(*corev1.Service), o)
	case *corev1.ConfigMap:
		return copyAndMergeCm(targetObj.(*corev1.ConfigMap), o)
	case *corev1.Pod:
		return copyAndMergePod(targetObj.(*corev1.Pod), o)
	default:
		return newObj
	}
}

func buildSvc(rsm workloads.ReplicatedStateMachine) *corev1.Service {
	if rsm.Spec.Service == nil {
		return nil
	}
	annotations := ParseAnnotationsOfScope(ServiceScope, rsm.Annotations)
	labels := getLabels(&rsm)
	selectors := getSvcSelector(&rsm, false)
	return builder.NewServiceBuilder(rsm.Namespace, rsm.Name).
		AddAnnotationsInMap(annotations).
		AddLabelsInMap(rsm.Spec.Service.Labels).
		AddLabelsInMap(labels).
		AddSelectorsInMap(selectors).
		AddPorts(rsm.Spec.Service.Spec.Ports...).
		SetType(rsm.Spec.Service.Spec.Type).
		GetObject()
}

func buildAlternativeSvs(rsm workloads.ReplicatedStateMachine) []*corev1.Service {
	if rsm.Spec.Service == nil {
		return nil
	}
	annotations := ParseAnnotationsOfScope(AlternativeServiceScope, rsm.Annotations)
	svcLabels := getLabels(&rsm)
	var services []*corev1.Service
	for i := range rsm.Spec.AlternativeServices {
		service := rsm.Spec.AlternativeServices[i]
		if len(service.Namespace) == 0 {
			service.Namespace = rsm.Namespace
		}
		labels := service.Labels
		if labels == nil {
			labels = make(map[string]string, 0)
		}
		for k, v := range svcLabels {
			labels[k] = v
		}
		service.Labels = labels
		newAnnotations := make(map[string]string, 0)
		maps.Copy(newAnnotations, service.Annotations)
		maps.Copy(newAnnotations, annotations)
		if len(newAnnotations) > 0 {
			service.Annotations = newAnnotations
		}
		services = append(services, &service)
	}
	return services
}

func buildHeadlessSvc(rsm workloads.ReplicatedStateMachine) *corev1.Service {
	annotations := ParseAnnotationsOfScope(HeadlessServiceScope, rsm.Annotations)
	labels := getLabels(&rsm)
	selectors := getSvcSelector(&rsm, true)
	hdlBuilder := builder.NewHeadlessServiceBuilder(rsm.Namespace, getHeadlessSvcName(rsm)).
		AddLabelsInMap(labels).
		AddSelectorsInMap(selectors).
		AddAnnotationsInMap(annotations)

	for _, container := range rsm.Spec.Template.Spec.Containers {
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

func buildSts(rsm workloads.ReplicatedStateMachine, headlessSvcName string, envConfig corev1.ConfigMap) *apps.StatefulSet {
	template := buildStsPodTemplate(rsm, envConfig)
	annotations := ParseAnnotationsOfScope(RootScope, rsm.Annotations)
	labels := getLabels(&rsm)
	return builder.NewStatefulSetBuilder(rsm.Namespace, rsm.Name).
		AddLabelsInMap(labels).
		AddLabels(rsmGenerationLabelKey, strconv.FormatInt(rsm.Generation, 10)).
		AddAnnotationsInMap(annotations).
		SetSelector(rsm.Spec.Selector).
		SetServiceName(headlessSvcName).
		SetReplicas(*rsm.Spec.Replicas).
		SetPodManagementPolicy(rsm.Spec.PodManagementPolicy).
		SetVolumeClaimTemplates(rsm.Spec.VolumeClaimTemplates...).
		SetTemplate(*template).
		SetUpdateStrategy(rsm.Spec.UpdateStrategy).
		GetObject()
}

func buildEnvConfigMap(rsm workloads.ReplicatedStateMachine) *corev1.ConfigMap {
	envData := buildEnvConfigData(rsm)
	annotations := ParseAnnotationsOfScope(ConfigMapScope, rsm.Annotations)
	labels := getLabels(&rsm)
	return builder.NewConfigMapBuilder(rsm.Namespace, getEnvConfigMapName(rsm.Name)).
		AddAnnotationsInMap(annotations).
		AddLabelsInMap(labels).
		SetData(envData).GetObject()
}

func buildStsPodTemplate(rsm workloads.ReplicatedStateMachine, envConfig corev1.ConfigMap) *corev1.PodTemplateSpec {
	template := rsm.Spec.Template
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

	injectRoleProbeContainer(rsm, &template)

	return &template
}

func injectRoleProbeContainer(rsm workloads.ReplicatedStateMachine, template *corev1.PodTemplateSpec) {
	roleProbe := rsm.Spec.RoleProbe
	if roleProbe == nil {
		return
	}
	credential := rsm.Spec.Credential
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

	actionSvcPorts := buildActionSvcPorts(template, roleProbe.CustomHandler)

	actionSvcList, _ := json.Marshal(actionSvcPorts)
	injectRoleProbeBaseContainer(rsm, template, string(actionSvcList), credentialEnv)

	if roleProbe.CustomHandler != nil {
		injectCustomRoleProbeContainer(rsm, template, actionSvcPorts, credentialEnv)
	}
}

func buildActionSvcPorts(template *corev1.PodTemplateSpec, actions []workloads.Action) []int32 {
	findAllUsedPorts := func() []int32 {
		allUsedPorts := make([]int32, 0)
		for _, container := range template.Spec.Containers {
			for _, port := range container.Ports {
				allUsedPorts = append(allUsedPorts, port.ContainerPort)
				allUsedPorts = append(allUsedPorts, port.HostPort)
			}
		}
		return allUsedPorts
	}

	findNextAvailablePort := func(base int32, allUsedPorts []int32) int32 {
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

	allUsedPorts := findAllUsedPorts()
	svcPort := actionSvcPortBase
	var actionSvcPorts []int32
	for range actions {
		svcPort = findNextAvailablePort(svcPort, allUsedPorts)
		actionSvcPorts = append(actionSvcPorts, svcPort)
	}
	return actionSvcPorts
}

func injectRoleProbeBaseContainer(rsm workloads.ReplicatedStateMachine, template *corev1.PodTemplateSpec, actionSvcList string, credentialEnv []corev1.EnvVar) {
	// compute parameters for role probe base container
	roleProbe := rsm.Spec.RoleProbe
	if roleProbe == nil {
		return
	}
	credential := rsm.Spec.Credential
	image := viper.GetString(constant.KBToolsImage)
	probeHTTPPort := viper.GetInt("ROLE_SERVICE_HTTP_PORT")
	if probeHTTPPort == 0 {
		probeHTTPPort = defaultRoleProbeDaemonPort
	}
	probeGRPCPort := viper.GetInt("ROLE_PROBE_GRPC_PORT")
	if probeGRPCPort == 0 {
		probeGRPCPort = defaultRoleProbeGRPCPort
	}
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
				Name:      constant.KBEnvServiceUser,
				Value:     credential.Username.Value,
				ValueFrom: credential.Username.ValueFrom,
			},
			corev1.EnvVar{
				Name:      constant.KBEnvServicePassword,
				Value:     credential.Password.Value,
				ValueFrom: credential.Password.ValueFrom,
			})
	}
	// find service port of th db engine
	servicePort := findSvcPort(rsm)
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

	// inject role update mechanism env
	env = append(env,
		corev1.EnvVar{
			Name:  RoleUpdateMechanismVarName,
			Value: string(roleProbe.RoleUpdateMechanism),
		})

	// inject role probe timeout env
	env = append(env,
		corev1.EnvVar{
			Name:  roleProbeTimeoutVarName,
			Value: strconv.Itoa(int(roleProbe.TimeoutSeconds)),
		})

	// lorry related envs
	env = append(env,
		corev1.EnvVar{
			Name: constant.KBEnvPodName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		corev1.EnvVar{
			Name: constant.KBEnvNamespace,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		corev1.EnvVar{
			Name: constant.KBEnvPodUID,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.uid",
				},
			},
		},
		corev1.EnvVar{
			Name: constant.KBEnvNodeName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
		corev1.EnvVar{
			Name: constant.KBEnvClusterName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.labels['" + constant.AppInstanceLabelKey + "']",
				},
			},
		},
		corev1.EnvVar{
			Name: constant.KBEnvCompName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.labels['" + constant.KBAppComponentLabelKey + "']",
				},
			},
		},
	)

	characterType := "custom"
	if roleProbe.BuiltinHandler != nil {
		characterType = *roleProbe.BuiltinHandler
	}
	env = append(env, corev1.EnvVar{
		Name:  constant.KBEnvCharacterType,
		Value: characterType,
	})

	readinessProbe := &corev1.Probe{
		InitialDelaySeconds: roleProbe.InitialDelaySeconds,
		TimeoutSeconds:      roleProbe.TimeoutSeconds,
		PeriodSeconds:       roleProbe.PeriodSeconds,
		SuccessThreshold:    roleProbe.SuccessThreshold,
		FailureThreshold:    roleProbe.FailureThreshold,
	}

	readinessProbe.ProbeHandler = corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: []string{
				grpcHealthProbeBinaryPath,
				fmt.Sprintf(grpcHealthProbeArgsFormat, probeGRPCPort),
			},
		},
	}

	tryToGetLorryGrpcPort := func(container *corev1.Container) *corev1.ContainerPort {
		for i, port := range container.Ports {
			if port.Name == constant.LorryGRPCPortName {
				return &container.Ports[i]
			}
		}
		return nil
	}

	// if role probe container exists, update the readiness probe, env and serving container port
	if container := controllerutil.GetLorryContainer(template.Spec.Containers); container != nil {
		if roleProbe.RoleUpdateMechanism == workloads.ReadinessProbeEventUpdate ||
			// for compatibility when upgrading from 0.7 to 0.8
			(container.ReadinessProbe != nil && container.ReadinessProbe.HTTPGet != nil &&
				strings.HasPrefix(container.ReadinessProbe.HTTPGet.Path, "/v1.0/bindings")) {
			port := tryToGetLorryGrpcPort(container)
			if port != nil && port.ContainerPort != int32(probeGRPCPort) {
				readinessProbe.Exec.Command = []string{
					grpcHealthProbeBinaryPath,
					fmt.Sprintf(grpcHealthProbeArgsFormat, port.ContainerPort),
				}
			}
			container.ReadinessProbe = readinessProbe
		}

		for _, e := range env {
			if slices.IndexFunc(container.Env, func(v corev1.EnvVar) bool {
				return v.Name == e.Name || e.Name == constant.KBEnvServiceUser ||
					e.Name == constant.KBEnvServicePassword || e.Name == usernameCredentialVarName || e.Name == passwordCredentialVarName
			}) >= 0 {
				continue
			}
			container.Env = append(container.Env, e)
		}
		return
	}

	// if role probe container doesn't exist, create a new one
	// build container
	container := builder.NewContainerBuilder(roleProbeContainerName).
		SetImage(image).
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddCommands([]string{
			roleProbeBinaryName,
			"--port", strconv.Itoa(probeHTTPPort),
			"--grpcport", strconv.Itoa(probeGRPCPort),
		}...).
		AddEnv(env...).
		AddPorts(
			corev1.ContainerPort{
				ContainerPort: int32(probeHTTPPort),
				Name:          roleProbeContainerName,
				Protocol:      "TCP",
			},
			corev1.ContainerPort{
				ContainerPort: int32(probeGRPCPort),
				Name:          roleProbeGRPCPortName,
				Protocol:      "TCP",
			},
		).
		SetReadinessProbe(*readinessProbe).
		GetObject()

	// inject role probe container
	template.Spec.Containers = append(template.Spec.Containers, *container)
}

func injectCustomRoleProbeContainer(rsm workloads.ReplicatedStateMachine, template *corev1.PodTemplateSpec, actionSvcPorts []int32, credentialEnv []corev1.EnvVar) {
	if rsm.Spec.RoleProbe == nil {
		return
	}

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
	for i, action := range rsm.Spec.RoleProbe.CustomHandler {
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

func buildEnvConfigData(set workloads.ReplicatedStateMachine) map[string]string {
	envData := map[string]string{}
	svcName := getHeadlessSvcName(set)
	uid := string(set.UID)
	strReplicas := strconv.Itoa(int(*set.Spec.Replicas))
	generateReplicaEnv := func(prefix string) {
		for i := 0; i < int(*set.Spec.Replicas); i++ {
			hostNameTplKey := prefix + strconv.Itoa(i) + "_HOSTNAME"
			hostNameTplValue := set.Name + "-" + strconv.Itoa(i)
			envData[hostNameTplKey] = fmt.Sprintf("%s.%s", hostNameTplValue, svcName)
		}
	}
	// build member related envs from set.Status.MembersStatus
	generateMemberEnv := func(prefix string) {
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
	}

	prefix := constant.KBPrefix + "_RSM_"
	envData[prefix+"N"] = strReplicas
	generateReplicaEnv(prefix)
	generateMemberEnv(prefix)
	// set owner uid to let pod know if the owner is recreated
	envData[prefix+"OWNER_UID"] = uid
	envData[prefix+"OWNER_UID_SUFFIX8"] = uid[len(uid)-4:]

	// have backward compatible handling for env generated in version prior 0.6.0
	prefix = constant.KBPrefix + "_"
	envData[prefix+"REPLICA_COUNT"] = strReplicas
	generateReplicaEnv(prefix)
	generateMemberEnv(prefix)
	envData[prefix+"CLUSTER_UID"] = uid

	// have backward compatible handling for CM key with 'compDefName' being part of the key name, prior 0.5.0
	// and introduce env/cm key naming reference complexity
	componentDefName := set.Labels[constant.AppComponentLabelKey]
	prefixWithCompDefName := prefix + strings.ToUpper(componentDefName) + "_"
	envData[prefixWithCompDefName+"N"] = strReplicas
	generateReplicaEnv(prefixWithCompDefName)
	generateMemberEnv(prefixWithCompDefName)
	envData[prefixWithCompDefName+"CLUSTER_UID"] = uid

	lorryHTTPPort, err := controllerutil.GetLorryHTTPPortFromContainers(set.Spec.Template.Spec.Containers)
	if err == nil {
		envData[constant.KBEnvLorryHTTPPort] = strconv.Itoa(int(lorryHTTPPort))

	}

	return envData
}
