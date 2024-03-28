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

package rsm2

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	rsm1 "github.com/apecloud/kubeblocks/pkg/controller/rsm"
)

type podTemplateSpecExt struct {
	Replicas     int32
	OrdinalStart int32
	corev1.PodTemplateSpec
	VolumeClaimTemplates []corev1.PersistentVolumeClaim
}

type replica struct {
	pod  *corev1.Pod
	pvcs []*corev1.PersistentVolumeClaim
}

var replicaNameRegex = regexp.MustCompile("(.*)-([0-9]+)$")

var (
	reader *zstd.Decoder
	writer *zstd.Encoder
)

func init() {
	var err error
	reader, err = zstd.NewReader(nil)
	runtime.Must(err)
	writer, err = zstd.NewWriter(nil)
	runtime.Must(err)
}

// ParseParentNameAndOrdinal parses parent (instance template) GenerateName and ordinal from the give instance name.
// -1 will be returned if no numeric suffix contained.
func ParseParentNameAndOrdinal(s string) (string, int) {
	parent := s
	ordinal := -1
	subMatches := replicaNameRegex.FindStringSubmatch(s)
	if len(subMatches) < 3 {
		return parent, ordinal
	}
	parent = subMatches[1]
	if i, err := strconv.ParseInt(subMatches[2], 10, 32); err == nil {
		ordinal = int(i)
	}
	return parent, ordinal
}

// sortObjects sorts objects by their role priority and name
// e.g.: unknown -> empty -> learner -> follower1 -> follower2 -> leader, with follower1.Name < follower2.Name
// reverse it if reverse==true
func sortObjects[T client.Object](objects []T, rolePriorityMap map[string]int, reverse bool) {
	getRolePriorityFunc := func(i int) int {
		role := strings.ToLower(objects[i].GetLabels()[constant.RoleLabelKey])
		return rolePriorityMap[role]
	}
	getNameNOrdinalFunc := func(i int) (string, int) {
		return ParseParentNameAndOrdinal(objects[i].GetName())
	}
	BaseSort(objects, getNameNOrdinalFunc, getRolePriorityFunc, reverse)
}

func BaseSort(x any, getNameNOrdinalFunc func(i int) (string, int), getRolePriorityFunc func(i int) int, reverse bool) {
	if getRolePriorityFunc == nil {
		getRolePriorityFunc = func(_ int) int {
			return 0
		}
	}
	sort.SliceStable(x, func(i, j int) bool {
		if reverse {
			i, j = j, i
		}
		rolePriI := getRolePriorityFunc(i)
		rolePriJ := getRolePriorityFunc(j)
		if rolePriI != rolePriJ {
			return rolePriI < rolePriJ
		}
		name1, ordinal1 := getNameNOrdinalFunc(i)
		name2, ordinal2 := getNameNOrdinalFunc(j)
		if name1 != name2 {
			return name1 < name2
		}
		return ordinal1 < ordinal2
	})
}

// isRunningAndReady returns true if pod is in the PodRunning Phase, if it has a condition of PodReady.
func isRunningAndReady(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning && podutils.IsPodReady(pod)
}

func isRunningAndAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	return podutils.IsPodAvailable(pod, minReadySeconds, metav1.Now())
}

// isCreated returns true if pod has been created and is maintained by the API server
func isCreated(pod *corev1.Pod) bool {
	return pod.Status.Phase != ""
}

// isTerminating returns true if pod's DeletionTimestamp has been set
func isTerminating(pod *corev1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

// isHealthy returns true if pod is running and ready and has not been terminated
func isHealthy(pod *corev1.Pod) bool {
	return isRunningAndReady(pod) && !isTerminating(pod)
}

// getPodRevision gets the revision of Pod by inspecting the StatefulSetRevisionLabel. If pod has no revision the empty
// string is returned.
func getPodRevision(pod *corev1.Pod) string {
	if pod.Labels == nil {
		return ""
	}
	return pod.Labels[appsv1.ControllerRevisionHashLabelKey]
}

func validateDupReplicaNames[T any](replicas []T, getNameFunc func(item T) string) error {
	podNameCount := make(map[string]int)
	for _, r := range replicas {
		name := getNameFunc(r)
		count, exist := podNameCount[name]
		if exist {
			count++
		} else {
			count = 1
		}
		podNameCount[name] = count
	}
	dupNames := ""
	for name, count := range podNameCount {
		if count > 1 {
			dupNames = fmt.Sprintf("%s%s,", dupNames, name)
		}
	}
	if len(dupNames) > 0 {
		return fmt.Errorf("duplicate pod names: %s", dupNames)
	}
	return nil
}

func buildReplicaName2TemplateMap(rsm *workloads.ReplicatedStateMachine, tree *kubebuilderx.ObjectTree) (map[string]*podTemplateSpecExt, error) {
	replicaTemplateGroups, err := buildReplicaTemplateGroups(rsm, tree)
	if err != nil {
		return nil, err
	}
	nameTemplateMap := make(map[string]*podTemplateSpecExt)
	var (
		replicaNameList []string
		replicaNames    []string
		ordinal         int
	)
	for _, templateList := range replicaTemplateGroups {
		ordinal = 0
		for _, template := range templateList {
			replicaNames, ordinal, err = buildReplicaNames(template, ordinal)
			if err != nil {
				return nil, err
			}
			for _, name := range replicaNames {
				nameTemplateMap[name] = template
			}
			replicaNameList = append(replicaNameList, replicaNames...)
		}
	}
	// validate duplicate pod names
	getNameFunc := func(n string) string {
		return n
	}
	if err = validateDupReplicaNames(replicaNameList, getNameFunc); err != nil {
		return nil, err
	}

	return nameTemplateMap, nil
}

func buildReplicaNames(template *podTemplateSpecExt, ordinal int) ([]string, int, error) {
	var replicaNameList []string
	var name string
	for i := 0; i < int(template.Replicas); i++ {
		name, ordinal = GeneratePodName(template.Name, template.GenerateName, ordinal, int(template.OrdinalStart), i)
		replicaNameList = append(replicaNameList, name)
	}
	return replicaNameList, ordinal, nil
}

// GeneratePodName generates a pod name based on certain rules:
// 1. If a 'name' is provided in the InstanceTemplate, it is used directly.
// 2. If an 'ordinalStart' is provided in the InstanceTemplate, the pods are indexed using a separate ordinal range,
// i.e., [ordinalStart, ordinalStart+Replicas). In this function, the ordinal will be 'ordinalStart'+'index'.
// 3. All other InstanceTemplates with the same generateName will be indexed using a shared ordinal range, which we refer to as the default ordinal range.
//
// For example:
// Let's consider a template group with 4 InstanceTemplates:
//   - name: ""
//     generateName: "foo"
//     replicas: 2
//     ordinalStart: 0
//   - name: ""
//     generateName: "foo"
//     replicas: 2
//     ordinalStart: 100
//   - name: "foo"
//     generateName: ""
//     ordinalStart: 0
//   - name: ""
//     generateName: "foo"
//     replicas: 2
//     ordinalStart: 0
//
// According to rule #1, we generate a pod name 'foo' from template #3.
// According to rule #2, we generate 2 pod names 'foo-100' and 'foo-101' from template #2.
// According to rule #3, template #1 and #4 share the same ordinal range starting from 0. We generate 4 pod names 'foo-0', 'foo-1', 'foo-2', and 'foo-3'.
// Hence, the final 7 pod names are: 'foo', 'foo-0', 'foo-1', 'foo-2', 'foo-3', 'foo-100', and 'foo-101'.
func GeneratePodName(name, generateName string, defaultOrdinal, ordinalStart, index int) (podName string, updatedDefaultOrdinal int) {
	switch {
	case len(name) > 0:
		podName = name
		updatedDefaultOrdinal = defaultOrdinal
	case ordinalStart > 0:
		podName = fmt.Sprintf("%s-%d", generateName, ordinalStart+index)
		updatedDefaultOrdinal = defaultOrdinal
	default:
		podName = fmt.Sprintf("%s-%d", generateName, defaultOrdinal)
		updatedDefaultOrdinal = defaultOrdinal + 1
	}
	return
}

func buildReplicaByTemplate(name string, template *podTemplateSpecExt, parent *workloads.ReplicatedStateMachine) (*replica, error) {
	revision, err := buildPodTemplateRevision(template, parent)
	if err != nil {
		return nil, err
	}
	// 1. build a pod from template
	pod := builder.NewPodBuilder(parent.Namespace, name).
		AddAnnotationsInMap(template.Annotations).
		AddLabelsInMap(template.Labels).
		AddControllerRevisionHashLabel(revision).
		SetPodSpec(*template.Spec.DeepCopy()).
		GetObject()
	// Set these immutable fields only on initial Pod creation, not updates.
	pod.Spec.Hostname = pod.Name
	pod.Spec.Subdomain = parent.Spec.ServiceName

	// 2. build pvcs from template
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim)
	pvcNameMap := make(map[string]string)
	for _, claimTemplate := range template.VolumeClaimTemplates {
		pvcName := fmt.Sprintf("%s-%s", claimTemplate.Name, pod.GetName())
		pvc := builder.NewPVCBuilder(template.Namespace, pvcName).
			AddLabelsInMap(template.Labels).
			AddLabels(constant.VolumeClaimTemplateNameLabelKey, claimTemplate.Name).
			SetSpec(*claimTemplate.Spec.DeepCopy()).
			GetObject()
		pvcMap[pvcName] = pvc
		pvcNameMap[pvcName] = claimTemplate.Name
	}

	// 3. update pod volumes
	var pvcs []*corev1.PersistentVolumeClaim
	var volumeList []corev1.Volume
	for pvcName, pvc := range pvcMap {
		pvcs = append(pvcs, pvc)
		volume := builder.NewVolumeBuilder(pvcNameMap[pvcName]).
			SetVolumeSource(corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
			}).GetObject()
		volumeList = append(volumeList, *volume)
	}
	MergeList(&volumeList, &pod.Spec.Volumes, func(item corev1.Volume) func(corev1.Volume) bool {
		return func(v corev1.Volume) bool {
			return v.Name == item.Name
		}
	})

	if err = controllerutil.SetControllerReference(parent, pod, model.GetScheme()); err != nil {
		return nil, err
	}
	replica := &replica{
		pod:  pod,
		pvcs: pvcs,
	}
	return replica, nil
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

	copyAndMergeSts := func(oldSts, newSts *appsv1.StatefulSet) client.Object {
		oldSts.Labels = mergeMetadataMap(oldSts.Labels, newSts.Labels)
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
		// check pod update by revision
		if getPodRevision(newPod) == getPodRevision(oldPod) {
			return nil
		}

		// in-place update is not supported currently, means the pod update is done through delete+create.
		// so no need to merge the fields.
		return oldPod
	}

	copyAndMergePVC := func(oldPVC, newPVC *corev1.PersistentVolumeClaim) client.Object {
		// resources.request.storage and accessModes support in-place update.
		// resources.request.storage only supports volume expansion.
		if reflect.DeepEqual(oldPVC.Spec.AccessModes, newPVC.Spec.AccessModes) &&
			oldPVC.Spec.Resources.Requests.Storage().Cmp(*newPVC.Spec.Resources.Requests.Storage()) >= 0 {
			return nil
		}
		oldPVC.Spec.AccessModes = newPVC.Spec.AccessModes
		oldPVC.Spec.Resources.Requests[corev1.ResourceStorage] = *newPVC.Spec.Resources.Requests.Storage()
		return oldPVC
	}

	targetObj := oldObj.DeepCopyObject()
	switch o := newObj.(type) {
	case *appsv1.StatefulSet:
		return copyAndMergeSts(targetObj.(*appsv1.StatefulSet), o)
	case *corev1.Service:
		return copyAndMergeSvc(targetObj.(*corev1.Service), o)
	case *corev1.ConfigMap:
		return copyAndMergeCm(targetObj.(*corev1.ConfigMap), o)
	case *corev1.Pod:
		return copyAndMergePod(targetObj.(*corev1.Pod), o)
	case *corev1.PersistentVolumeClaim:
		return copyAndMergePVC(targetObj.(*corev1.PersistentVolumeClaim), o)
	default:
		return newObj
	}
}

func validateSpec(rsm *workloads.ReplicatedStateMachine, tree *kubebuilderx.ObjectTree) error {
	replicasInTemplates := int32(0)
	var names string
	template, err := findTemplate(rsm, tree)
	if err != nil {
		return err
	}
	instanceTemplates := getInstanceTemplates(rsm.Spec.Instances, template)
	for _, instance := range instanceTemplates {
		replicas := int32(1)
		if instance.Replicas != nil {
			replicas = *instance.Replicas
		}
		replicasInTemplates += replicas

		if instance.Name != nil {
			if instance.Replicas != nil && *instance.Replicas > 1 {
				names = fmt.Sprintf("%s%s,", names, *instance.Name)
			}
		}
	}
	// sum of spec.templates[*].replicas should not greater than spec.replicas
	if replicasInTemplates > *rsm.Spec.Replicas {
		return fmt.Errorf("total replicas in instances(%d) should not greater than replicas in spec(%d)", replicasInTemplates, *rsm.Spec.Replicas)
	}

	// instance.replicas should be nil or 1 if instance.name set
	if len(names) > 0 {
		return fmt.Errorf("replicas should be empty or no more than 1 if name set, instance names: %s", names)
	}

	return nil
}

func buildPodTemplateRevision(template *podTemplateSpecExt, parent *workloads.ReplicatedStateMachine) (string, error) {
	// try to generate the same revision as if generated by sts controller
	rsm := builder.NewReplicatedStateMachineBuilder(parent.Namespace, parent.Name).
		SetUID(parent.UID).
		AddAnnotationsInMap(parent.Annotations).
		AddMatchLabelsInMap(parent.Labels).
		SetTemplate(template.PodTemplateSpec).
		GetObject()

	cr, err := NewRevision(rsm)
	if err != nil {
		return "", err
	}
	return cr.Labels[ControllerRevisionHashLabel], nil
}

func buildReplicaTemplateGroups(rsm *workloads.ReplicatedStateMachine, tree *kubebuilderx.ObjectTree) (map[string][]*podTemplateSpecExt, error) {
	envConfigName := rsm1.GetEnvConfigMapName(rsm.Name)
	defaultTemplate := rsm1.BuildPodTemplate(rsm, envConfigName)
	buildPodTemplateExt := func(replicas int32) *podTemplateSpecExt {
		var claims []corev1.PersistentVolumeClaim
		for _, template := range rsm.Spec.VolumeClaimTemplates {
			claims = append(claims, *template.DeepCopy())
		}
		return &podTemplateSpecExt{
			Replicas:             replicas,
			PodTemplateSpec:      *defaultTemplate.DeepCopy(),
			VolumeClaimTemplates: claims,
		}
	}
	instancesCompressed, err := findTemplate(rsm, tree)
	if err != nil {
		return nil, err
	}

	instanceTemplateGroups := BuildInstanceTemplateGroups(rsm.Name, *rsm.Spec.Replicas, rsm.Spec.Instances, instancesCompressed)
	replicaTemplateGroups := make(map[string][]*podTemplateSpecExt)
	for name, instanceTemplates := range instanceTemplateGroups {
		var podTemplates []*podTemplateSpecExt
		for _, instance := range instanceTemplates {
			replicas := int32(1)
			if instance.Replicas != nil {
				replicas = *instance.Replicas
			}
			template := buildPodTemplateExt(replicas)
			applyInstanceTemplate(*instance, template)
			if template.GenerateName == "" {
				template.GenerateName = rsm.Name
			}
			podTemplates = append(podTemplates, template)
		}
		replicaTemplateGroups[name] = podTemplates
	}

	return replicaTemplateGroups, nil
}

func BuildInstanceTemplateGroups(parentName string, totalReplicas int32, instances []workloads.InstanceTemplate, instancesCompressed *corev1.ConfigMap) map[string][]*workloads.InstanceTemplate {
	var instanceTemplateList []*workloads.InstanceTemplate
	var replicasInTemplates int32
	instanceTemplates := getInstanceTemplates(instances, instancesCompressed)
	for i := range instanceTemplates {
		instance := &instanceTemplates[i]
		replicas := int32(1)
		if instance.Replicas != nil {
			replicas = *instance.Replicas
		}
		instanceTemplateList = append(instanceTemplateList, instance)
		replicasInTemplates += replicas
	}
	if replicasInTemplates < totalReplicas {
		replicas := totalReplicas - replicasInTemplates
		instance := &workloads.InstanceTemplate{Replicas: &replicas}
		instanceTemplateList = append(instanceTemplateList, instance)
	}

	// group the pod templates by template.Name if set or by template.GenerateName
	instanceTemplateGroups := make(map[string][]*workloads.InstanceTemplate)
	for _, template := range instanceTemplateList {
		name := GetGroupName(parentName, template.Name, template.GenerateName)
		templates := instanceTemplateGroups[name]
		templates = append(templates, template)
		instanceTemplateGroups[name] = templates
	}

	return instanceTemplateGroups
}

func GetGroupName(parentName string, instanceName, generateName *string) string {
	switch {
	case instanceName != nil:
		return *instanceName
	case generateName != nil:
		return *generateName
	default:
		return parentName
	}
}

func getInstanceTemplateMap(annotations map[string]string) (map[string]string, error) {
	if annotations == nil {
		return nil, nil
	}
	templateRef, ok := annotations[templateRefAnnotationKey]
	if !ok {
		return nil, nil
	}
	templateMap := make(map[string]string)
	if err := json.Unmarshal([]byte(templateRef), &templateMap); err != nil {
		return nil, err
	}
	return templateMap, nil
}

func getInstanceTemplates(instances []workloads.InstanceTemplate, template *corev1.ConfigMap) []workloads.InstanceTemplate {
	if template == nil {
		return instances
	}

	// if template is found with incorrect format, try it as the whole templates is corrupted.
	if template.BinaryData == nil {
		return nil
	}
	templateData, ok := template.BinaryData[templateRefDataKey]
	if !ok {
		return nil
	}
	templateByte, err := reader.DecodeAll(templateData, nil)
	if err != nil {
		return nil
	}
	extraTemplates := make([]workloads.InstanceTemplate, 0)
	err = json.Unmarshal(templateByte, &extraTemplates)
	if err != nil {
		return nil
	}

	return append(instances, extraTemplates...)
}

func findTemplate(rsm *workloads.ReplicatedStateMachine, tree *kubebuilderx.ObjectTree) (*corev1.ConfigMap, error) {
	templateMap, err := getInstanceTemplateMap(rsm.Annotations)
	// error has been checked in prepare stage, there should be no error occurs
	if err != nil {
		return nil, nil
	}
	for name, templateName := range templateMap {
		if name != rsm.Name {
			continue
		}
		// find the compressed instance templates, parse them
		template := builder.NewConfigMapBuilder(rsm.Namespace, templateName).GetObject()
		templateObj, err := tree.Get(template)
		if err != nil {
			return nil, err
		}
		template, _ = templateObj.(*corev1.ConfigMap)
		return template, nil
	}
	return nil, nil
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
	if instance.GenerateName != nil {
		template.GenerateName = *instance.GenerateName
	}
	ordinalStart := int32(0)
	if instance.OrdinalStart != nil {
		ordinalStart = *instance.OrdinalStart
	}
	template.OrdinalStart = ordinalStart
	if instance.NodeName != nil {
		template.Spec.NodeName = *instance.NodeName
	}
	MergeMap(&instance.Annotations, &template.Annotations)
	MergeMap(&instance.Labels, &template.Labels)
	MergeMap(&instance.NodeSelector, &template.Spec.NodeSelector)
	if len(template.Spec.Containers) > 0 {
		if instance.Image != nil {
			template.Spec.Containers[0].Image = *instance.Image
		}
		if instance.Resources != nil {
			template.Spec.Containers[0].Resources = *instance.Resources
		}
	}
	MergeList(&instance.Volumes, &template.Spec.Volumes,
		func(item corev1.Volume) func(corev1.Volume) bool {
			return func(v corev1.Volume) bool {
				return v.Name == item.Name
			}
		})
	MergeList(&instance.VolumeMounts, &template.Spec.Containers[0].VolumeMounts,
		func(item corev1.VolumeMount) func(corev1.VolumeMount) bool {
			return func(vm corev1.VolumeMount) bool {
				return vm.Name == item.Name
			}
		})
	MergeList(&instance.VolumeClaimTemplates, &template.VolumeClaimTemplates,
		func(item corev1.PersistentVolumeClaim) func(corev1.PersistentVolumeClaim) bool {
			return func(claim corev1.PersistentVolumeClaim) bool {
				return claim.Name == item.Name
			}
		})
}
