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

package rsm

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

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

type podTemplateSpecExt struct {
	Replicas int32
	corev1.PodTemplateSpec
	VolumeClaimTemplates []corev1.PersistentVolumeClaim
}

var _ graph.Transformer = &ObjectGenerationTransformer{}

func (t *ObjectGenerationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*rsmTransformContext)
	rsm := transCtx.rsm
	rsmOrig := transCtx.rsmOrig
	cli, _ := transCtx.Client.(model.GraphClient)

	if model.IsObjectDeleting(rsmOrig) {
		return nil
	}

	// sum of spec.templates[*].replicas should not greater than spec.replicas
	replicasInTemplates := int32(0)
	for _, instance := range rsm.Spec.Instances {
		replicas := int32(1)
		if instance.Replicas != nil {
			replicas = *instance.Replicas
		}
		replicasInTemplates += replicas
	}
	if replicasInTemplates > *rsm.Spec.Replicas {
		msgFmt := "total replicas in instances(%d) should not greater than replicas in spec(%d)"
		transCtx.EventRecorder.Eventf(rsm, corev1.EventTypeWarning, reasonBuildPods, msgFmt, replicasInTemplates, *rsm.Spec.Replicas)
		return model.NewRequeueError(time.Second*10, fmt.Sprintf(msgFmt, replicasInTemplates, *rsm.Spec.Replicas))
	}
	// instance.replicas should be nil or 1 if instance.name set
	// TODO(free6om): do validation

	// read cache snapshot
	ml := getLabels(rsm)
	oldSnapshot, err := model.ReadCacheSnapshot(ctx, rsm, ml, ownedKinds()...)
	if err != nil {
		return err
	}

	// generate objects by current spec
	svc := buildSvc(*rsm)
	altSvs := buildAlternativeSvs(*rsm)
	headLessSvc := buildHeadlessSvc(*rsm)
	envConfig := buildEnvConfigMap(*rsm)
	workloadList, err := buildWorkloads(*rsm, headLessSvc.Name, *envConfig, oldSnapshot)
	if err != nil {
		transCtx.EventRecorder.Eventf(rsm, corev1.EventTypeWarning, reasonBuildPods, err.Error())
		return model.NewRequeueError(time.Second*10, err.Error())
	}
	var objects []client.Object
	objects = append(objects, workloadList...)
	objects = append(objects, headLessSvc, envConfig)
	if svc != nil {
		objects = append(objects, svc)
	}
	for _, s := range altSvs {
		objects = append(objects, s)
	}

	for _, object := range objects {
		if err := setOwnership(rsm, object, model.GetScheme(), getFinalizer(object)); err != nil {
			return err
		}
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
		index := len(workloadList)
		for i := range workloadList {
			cli.DependOn(dag, workloadList[i], objects[index:]...)
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
			return c.Name == roleProbeContainerName || c.Name == constant.RoleProbeContainerName
		})
	}

	copyAndMergeSts := func(oldSts, newSts *apps.StatefulSet) client.Object {
		oldSts.Labels = mergeMetadataMap(oldSts.Labels, newSts.Labels)

		// for upgrade compatibility from 0.7 to 0.8
		oldRoleProbeContainerIndex := getRoleProbeContainerIndex(oldSts.Spec.Template.Spec.Containers)
		newRoleProbeContainerIndex := getRoleProbeContainerIndex(newSts.Spec.Template.Spec.Containers)
		if oldRoleProbeContainerIndex >= 0 && newRoleProbeContainerIndex >= 0 {
			newCopySts := newSts.DeepCopy()
			newCopySts.Spec.Template.Spec.Containers[newRoleProbeContainerIndex] = *oldSts.Spec.Template.Spec.Containers[oldRoleProbeContainerIndex].DeepCopy()
			for i := range newCopySts.Spec.Template.Spec.Containers {
				newContainer := &newCopySts.Spec.Template.Spec.Containers[i]
				for j := range oldSts.Spec.Template.Spec.Containers {
					oldContainer := oldSts.Spec.Template.Spec.Containers[j]
					if newContainer.Name == oldContainer.Name {
						controllerutil.ResolveContainerDefaultFields(oldContainer, newContainer)
						break
					}
				}
			}
			for i := range newCopySts.Spec.Template.Spec.InitContainers {
				newContainer := &newCopySts.Spec.Template.Spec.InitContainers[i]
				for j := range oldSts.Spec.Template.Spec.InitContainers {
					oldContainer := oldSts.Spec.Template.Spec.InitContainers[j]
					if newContainer.Name == oldContainer.Name {
						controllerutil.ResolveContainerDefaultFields(oldContainer, newContainer)
						break
					}
				}
			}

			if reflect.DeepEqual(newCopySts.Spec.Template.Spec.Containers, oldSts.Spec.Template.Spec.Containers) &&
				reflect.DeepEqual(newCopySts.Spec.Template.Spec.InitContainers, oldSts.Spec.Template.Spec.InitContainers) {
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
		// TODO(free6om):finish me
		panic("finish me")
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
		AddAnnotationsInMap(annotations).
		SetPublishNotReadyAddresses(true)

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

func buildWorkloads(rsm workloads.ReplicatedStateMachine, headlessSvcName string, envConfig corev1.ConfigMap, oldSnapshot model.ObjectSnapshot) ([]client.Object, error) {
	// return true if there is a StatefulSet object in old cache
	isManagingSts := func() bool {
		for _, object := range oldSnapshot {
			if _, ok := object.(*apps.StatefulSet); ok {
				return true
			}
		}
		return false
	}

	if isManagingSts() {
		return []client.Object{buildSts(rsm, headlessSvcName, envConfig)}, nil
	}

	return buildPods(rsm, envConfig)
}

func buildPods(rsm workloads.ReplicatedStateMachine, envConfig corev1.ConfigMap) ([]client.Object, error) {
	// 1. prepare all templates
	var podTemplates []*podTemplateSpecExt
	var replicasInTemplates int32
	defaultTemplate := buildPodTemplate(rsm, envConfig)
	buildPodTemplateExt := func(replicas int32) *podTemplateSpecExt {
		claims := make([]corev1.PersistentVolumeClaim, len(rsm.Spec.VolumeClaimTemplates))
		copy(claims, rsm.Spec.VolumeClaimTemplates)
		return &podTemplateSpecExt{
			Replicas:             replicas,
			PodTemplateSpec:      *defaultTemplate.DeepCopy(),
			VolumeClaimTemplates: claims,
		}
	}
	for _, instance := range rsm.Spec.Instances {
		replicas := int32(1)
		if instance.Replicas != nil {
			replicas = *instance.Replicas
		}
		template := buildPodTemplateExt(replicas)
		applyInstanceTemplate(instance, template)
		podTemplates = append(podTemplates, template)
		replicasInTemplates += template.Replicas
	}
	if replicasInTemplates < *rsm.Spec.Replicas {
		template := buildPodTemplateExt(*rsm.Spec.Replicas - replicasInTemplates)
		podTemplates = append(podTemplates, template)
	}
	// set the default name generator and namespace
	for _, template := range podTemplates {
		if template.GenerateName == "" {
			template.GenerateName = rsm.Name
		}
		template.Namespace = rsm.Namespace
	}

	// 2. build all pods from podTemplates
	// group the pod templates by template.Name if set or by template.GenerateName
	podTemplateGroups := make(map[string][]*podTemplateSpecExt)
	for _, template := range podTemplates {
		name := template.Name
		if template.Name == "" {
			name = template.GenerateName
		}
		templates := podTemplateGroups[name]
		templates = append(templates, template)
		podTemplateGroups[name] = templates
	}
	// build pods by groups
	var pods []client.Object
	var pvcs []client.Object
	for _, templateList := range podTemplateGroups {
		var (
			podList []*corev1.Pod
			pvcList []*corev1.PersistentVolumeClaim
			ordinal int
		)
		for _, template := range templateList {
			podList, pvcList, ordinal = buildPodByTemplate(template, ordinal)
			for _, pod := range podList {
				pods = append(pods, pod)
			}
			for _, pvc := range pvcList {
				pvcs = append(pvcs, pvc)
			}
		}
	}
	// validate duplicate pod names
	podNameCount := make(map[string]int)
	for _, pod := range pods {
		count, exist := podNameCount[pod.GetName()]
		if exist {
			count++
		} else {
			count = 1
		}
		podNameCount[pod.GetName()] = count
	}
	dupNames := ""
	for name, count := range podNameCount {
		if count > 1 {
			dupNames = fmt.Sprintf("%s%s,", dupNames, name)
		}
	}
	if len(dupNames) > 0 {
		return nil, fmt.Errorf("duplicate pod names: %s", dupNames)
	}
	return append(pods, pvcs...), nil
}

func buildPodByTemplate(template *podTemplateSpecExt, ordinal int) ([]*corev1.Pod, []*corev1.PersistentVolumeClaim, int) {
	var (
		podList []*corev1.Pod
		pvcList []*corev1.PersistentVolumeClaim
	)
	generatePodName := func(name, generateName string, ordinal int) (string, int) {
		if len(name) > 0 {
			return name, ordinal
		}
		n := fmt.Sprintf("%s-%d", generateName, ordinal)
		ordinal++
		return n, ordinal
	}
	for i := 0; i < int(template.Replicas); i++ {
		// 1. generate pod name
		namespace := template.Namespace
		var name string
		name, ordinal = generatePodName(template.Name, template.GenerateName, ordinal)

		// 2. build a pod from template
		pod := builder.NewPodBuilder(namespace, name).
			AddAnnotationsInMap(template.Annotations).
			AddLabelsInMap(template.Labels).
			SetPodSpec(*template.Spec.DeepCopy()).
			GetObject()
		podList = append(podList, pod)

		// 3. build pvcs from template
		pvcMap := make(map[string]*corev1.PersistentVolumeClaim)
		pvcNameMap := make(map[string]string)
		for _, claimTemplate := range template.VolumeClaimTemplates {
			pvcName := fmt.Sprintf("%s-%s", claimTemplate.Name, pod.GetName())
			pvc := builder.NewPVCBuilder(namespace, pvcName).SetSpec(*claimTemplate.Spec.DeepCopy()).GetObject()
			pvcMap[pvcName] = pvc
			pvcNameMap[pvcName] = claimTemplate.Name
		}

		// 4. update pod volumes
		var volumeList []corev1.Volume
		for pvcName, pvc := range pvcMap {
			pvcList = append(pvcList, pvc)
			volume := builder.NewVolumeBuilder(pvcNameMap[pvcName]).
				SetVolumeSource(corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
				}).GetObject()
			volumeList = append(volumeList, *volume)
		}
		mergeList(&volumeList, &pod.Spec.Volumes, func(item corev1.Volume) func(corev1.Volume) bool {
			return func(v corev1.Volume) bool {
				return v.Name == item.Name
			}
		})

		// in case illegal template.replicas set
		if len(template.Name) > 0 {
			break
		}
	}
	return podList, pvcList, ordinal
}

func applyInstanceTemplate(instance workloads.InstanceTemplate, template *podTemplateSpecExt) {
	replicas := int32(1)
	if instance.Replicas != nil {
		replicas = *instance.Replicas
	}
	template.Replicas = replicas
	if instance.Name != nil {
		template.Name = *instance.Name
	}
	if instance.NodeName != nil {
		template.Spec.NodeName = *instance.NodeName
	}
	mergeMap(&instance.Annotations, &template.Annotations)
	mergeMap(&instance.Labels, &template.Labels)
	mergeMap(&instance.NodeSelector, &template.Spec.NodeSelector)
	if instance.Resources != nil {
		if len(template.Spec.Containers) > 0 {
			template.Spec.Containers[0].Resources = *instance.Resources
		}
	}
	mergeList(&instance.Volumes, &template.Spec.Volumes,
		func(item corev1.Volume) func(corev1.Volume) bool {
			return func(v corev1.Volume) bool {
				return v.Name == item.Name
			}
		})
	mergeList(&instance.VolumeMounts, &template.Spec.Containers[0].VolumeMounts,
		func(item corev1.VolumeMount) func(corev1.VolumeMount) bool {
			return func(vm corev1.VolumeMount) bool {
				return vm.Name == item.Name
			}
		})
	mergeList(&instance.VolumeClaimTemplates, &template.VolumeClaimTemplates,
		func(item corev1.PersistentVolumeClaim) func(corev1.PersistentVolumeClaim) bool {
			return func(claim corev1.PersistentVolumeClaim) bool {
				return claim.Name == item.Name
			}
		})
}

func buildSts(rsm workloads.ReplicatedStateMachine, headlessSvcName string, envConfig corev1.ConfigMap) *apps.StatefulSet {
	template := buildPodTemplate(rsm, envConfig)
	annotations := ParseAnnotationsOfScope(RootScope, rsm.Annotations)
	labels := getLabels(&rsm)
	return builder.NewStatefulSetBuilder(rsm.Namespace, rsm.Name).
		AddLabelsInMap(labels).
		AddLabels(rsmGenerationLabelKey, strconv.FormatInt(rsm.Generation, 10)).
		AddAnnotationsInMap(annotations).
		SetSelector(rsm.Spec.Selector).
		SetServiceName(headlessSvcName).
		SetReplicas(*rsm.Spec.Replicas).
		SetMinReadySeconds(rsm.Spec.MinReadySeconds).
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

func buildPodTemplate(rsm workloads.ReplicatedStateMachine, envConfig corev1.ConfigMap) *corev1.PodTemplateSpec {
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
