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

	builder2 "github.com/apecloud/kubeblocks/pkg/controller/builder"
	graph2 "github.com/apecloud/kubeblocks/pkg/controller/graph"
	model2 "github.com/apecloud/kubeblocks/pkg/controller/model"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"

	"golang.org/x/exp/maps"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

type ObjectGenerationTransformer struct{}

var _ graph2.Transformer = &ObjectGenerationTransformer{}

func (t *ObjectGenerationTransformer) Transform(ctx graph2.TransformContext, dag *graph2.DAG) error {
	transCtx, _ := ctx.(*rsmTransformContext)
	rsm := transCtx.rsm
	rsmOrig := transCtx.rsmOrig
	cli, _ := transCtx.Client.(model2.GraphClient)

	if model2.IsObjectDeleting(rsmOrig) {
		return nil
	}

	// generate objects by current spec
	svc := buildSvc(*rsm)
	altSvs := buildAlternativeSvs(*rsm)
	headLessSvc := buildHeadlessSvc(*rsm)
	envConfig := buildEnvConfigMap(*rsm)
	sts := buildSts(*rsm, headLessSvc.Name, *envConfig)
	objects := []client.Object{headLessSvc, envConfig, sts}
	if svc != nil {
		objects = append(objects, svc)
	}
	for _, s := range altSvs {
		objects = append(objects, s)
	}

	for _, object := range objects {
		if err := setOwnership(rsm, object, model2.GetScheme(), getFinalizer(object)); err != nil {
			return err
		}
	}

	// read cache snapshot
	ml := getLabels(rsm)
	oldSnapshot, err := model2.ReadCacheSnapshot(ctx, rsm, ml, ownedKinds()...)
	if err != nil {
		return err
	}

	// compute create/update/delete set
	newSnapshot := make(map[model2.GVKNObjKey]client.Object)
	for _, object := range objects {
		name, err := model2.GetGVKName(object)
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
		cli.DependOn(dag, sts, svc, headLessSvc, envConfig)
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

	// mergeAnnotations keeps the original annotations.
	mergeAnnotations := func(originalAnnotations map[string]string, targetAnnotations *map[string]string) {
		if targetAnnotations == nil || originalAnnotations == nil {
			return
		}
		if *targetAnnotations == nil {
			*targetAnnotations = map[string]string{}
		}
		for k, v := range originalAnnotations {
			// if the annotation not exist in targetAnnotations, copy it from original.
			if _, ok := (*targetAnnotations)[k]; !ok {
				(*targetAnnotations)[k] = v
			}
		}
	}

	copyAndMergeSts := func(oldSts, newSts *apps.StatefulSet) client.Object {
		// if annotations exist and are replaced, the StatefulSet will be updated.
		mergeAnnotations(oldSts.Spec.Template.Annotations, &newSts.Spec.Template.Annotations)
		oldSts.Spec.Template = newSts.Spec.Template
		oldSts.Spec.Replicas = newSts.Spec.Replicas
		oldSts.Spec.UpdateStrategy = newSts.Spec.UpdateStrategy
		return oldSts
	}

	copyAndMergeSvc := func(oldSvc *corev1.Service, newSvc *corev1.Service) client.Object {
		// remove original monitor annotations
		if len(oldSvc.Annotations) > 0 {
			maps.DeleteFunc(oldSvc.Annotations, func(k, v string) bool {
				return strings.HasPrefix(k, "monitor.kubeblocks.io")
			})
		}
		mergeAnnotations(oldSvc.Annotations, &newSvc.Annotations)
		oldSvc.Annotations = newSvc.Annotations
		oldSvc.Spec = newSvc.Spec
		return oldSvc
	}

	copyAndMergeCm := func(oldCm, newCm *corev1.ConfigMap) client.Object {
		oldCm.Data = newCm.Data
		oldCm.BinaryData = newCm.BinaryData
		return oldCm
	}

	targetObj := oldObj.DeepCopyObject()
	switch o := newObj.(type) {
	case *apps.StatefulSet:
		return copyAndMergeSts(targetObj.(*apps.StatefulSet), o)
	case *corev1.Service:
		return copyAndMergeSvc(targetObj.(*corev1.Service), o)
	case *corev1.ConfigMap:
		return copyAndMergeCm(targetObj.(*corev1.ConfigMap), o)
	default:
		return newObj
	}
}

func buildSvc(rsm workloads.ReplicatedStateMachine) *corev1.Service {
	if rsm.Spec.Service == nil {
		return nil
	}
	labels := getLabels(&rsm)
	selectors := getSvcSelector(&rsm, false)
	return builder2.NewServiceBuilder(rsm.Namespace, rsm.Name).
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
		services = append(services, &service)
	}
	return services
}

func buildHeadlessSvc(rsm workloads.ReplicatedStateMachine) *corev1.Service {
	labels := getLabels(&rsm)
	selectors := getSvcSelector(&rsm, true)
	hdlBuilder := builder2.NewHeadlessServiceBuilder(rsm.Namespace, getHeadlessSvcName(rsm)).
		AddLabelsInMap(labels).
		AddSelectorsInMap(selectors)
	//	.AddAnnotations("prometheus.io/scrape", strconv.FormatBool(component.Monitor.Enable))
	// if component.Monitor.Enable {
	//	hdBuilder.AddAnnotations("prometheus.io/path", component.Monitor.ScrapePath).
	//		AddAnnotations("prometheus.io/port", strconv.Itoa(int(component.Monitor.ScrapePort))).
	//		AddAnnotations("prometheus.io/scheme", "http")
	// }
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
	labels := getLabels(&rsm)
	return builder2.NewStatefulSetBuilder(rsm.Namespace, rsm.Name).
		AddLabelsInMap(labels).
		AddAnnotationsInMap(rsm.Annotations).
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
	labels := getLabels(&rsm)
	if viper.GetBool(FeatureGateRSMCompatibilityMode) {
		labels[constant.AppConfigTypeLabelKey] = "kubeblocks-env"
	}
	return builder2.NewConfigMapBuilder(rsm.Namespace, rsm.Name+"-rsm-env").
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
	allUsedPorts := findAllUsedPorts(template)
	svcPort := actionSvcPortBase
	var actionSvcPorts []int32
	for range roleProbe.ProbeActions {
		svcPort = findNextAvailablePort(svcPort, allUsedPorts)
		actionSvcPorts = append(actionSvcPorts, svcPort)
	}
	injectProbeActionContainer(rsm, template, actionSvcPorts, credentialEnv)
	actionSvcList, _ := json.Marshal(actionSvcPorts)
	injectRoleProbeAgentContainer(rsm, template, string(actionSvcList), credentialEnv)
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

func injectRoleProbeAgentContainer(rsm workloads.ReplicatedStateMachine, template *corev1.PodTemplateSpec, actionSvcList string, credentialEnv []corev1.EnvVar) {
	// compute parameters for role probe agent container
	roleProbe := rsm.Spec.RoleProbe
	if roleProbe == nil {
		return
	}
	credential := rsm.Spec.Credential
	image := viper.GetString("ROLE_PROBE_AGENT_IMAGE")
	if len(image) == 0 {
		image = defaultRoleProbeAgentImage
	}
	probeDaemonPort := viper.GetInt("ROLE_PROBE_SERVICE_PORT")
	if probeDaemonPort == 0 {
		probeDaemonPort = defaultRoleProbeDaemonPort
	}
	roleProbeURI := fmt.Sprintf(roleProbeURIFormat, strconv.Itoa(probeDaemonPort))
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

	// build container
	container := corev1.Container{
		Name:            roleProbeName,
		Image:           image,
		ImagePullPolicy: "IfNotPresent",
		Command: []string{
			"role-agent",
			"--port", strconv.Itoa(probeDaemonPort),
		},
		Ports: []corev1.ContainerPort{{
			ContainerPort: int32(probeDaemonPort),
			Name:          roleProbeName,
			Protocol:      "TCP",
		}},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"/bin/grpc_health_probe",
						roleProbeURI,
					},
				},
			},
			InitialDelaySeconds: roleProbe.InitialDelaySeconds,
			TimeoutSeconds:      roleProbe.TimeoutSeconds,
			PeriodSeconds:       roleProbe.PeriodSeconds,
			SuccessThreshold:    roleProbe.SuccessThreshold,
			FailureThreshold:    roleProbe.FailureThreshold,
		},
		Env: env,
	}

	// inject role probe container
	template.Spec.Containers = append(template.Spec.Containers, container)
}

func injectProbeActionContainer(rsm workloads.ReplicatedStateMachine, template *corev1.PodTemplateSpec, actionSvcPorts []int32, credentialEnv []corev1.EnvVar) {
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
	for i, action := range rsm.Spec.RoleProbe.ProbeActions {
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

	return envData
}
