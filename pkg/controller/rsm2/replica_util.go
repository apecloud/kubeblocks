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
	Replicas int32
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

func parseParentNameAndOrdinal(s string) (string, int) {
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
		return parseParentNameAndOrdinal(objects[i].GetName())
	}
	baseSort(objects, getNameNOrdinalFunc, getRolePriorityFunc, reverse)
}

func baseSort(x any, getNameNOrdinalFunc func(i int) (string, int), getRolePriorityFunc func(i int) int, reverse bool) {
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
	replicaTemplateGroups := buildReplicaTemplateGroups(rsm, tree)
	nameTemplateMap := make(map[string]*podTemplateSpecExt)
	var (
		replicaNameList []string
		replicaNames    []string
		ordinal         int
		err             error
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
	generatePodName := func(name, generateName string, ordinal int) (string, int) {
		if len(name) > 0 {
			return name, ordinal
		}
		n := fmt.Sprintf("%s-%d", generateName, ordinal)
		ordinal++
		return n, ordinal
	}
	var replicaNameList []string
	var name string
	for i := 0; i < int(template.Replicas); i++ {
		name, ordinal = generatePodName(template.Name, template.GenerateName, ordinal)
		replicaNameList = append(replicaNameList, name)
	}
	return replicaNameList, ordinal, nil
}

func buildReplicaByTemplate(name string, template *podTemplateSpecExt, parent *workloads.ReplicatedStateMachine) (*replica, error) {
	revision, err := buildPodTemplateRevision(template, parent)
	if err != nil {
		return nil, err
	}
	// 1. build a pod from template
	pod := builder.NewPodBuilder(template.Namespace, name).
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
	mergeList(&volumeList, &pod.Spec.Volumes, func(item corev1.Volume) func(corev1.Volume) bool {
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
	instanceTemplates := getInstanceTemplates(rsm, tree)
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

func buildReplicaTemplateGroups(rsm *workloads.ReplicatedStateMachine, tree *kubebuilderx.ObjectTree) map[string][]*podTemplateSpecExt {
	var podTemplates []*podTemplateSpecExt
	var replicasInTemplates int32
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
	instanceTemplates := getInstanceTemplates(rsm, tree)
	for _, instance := range instanceTemplates {
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

	// group the pod templates by template.Name if set or by template.GenerateName
	replicaTemplateGroups := make(map[string][]*podTemplateSpecExt)
	for _, template := range podTemplates {
		name := template.Name
		if template.Name == "" {
			name = template.GenerateName
		}
		templates := replicaTemplateGroups[name]
		templates = append(templates, template)
		replicaTemplateGroups[name] = templates
	}

	return replicaTemplateGroups
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

func getInstanceTemplates(rsm *workloads.ReplicatedStateMachine, tree *kubebuilderx.ObjectTree) []workloads.InstanceTemplate {
	if tree == nil {
		return rsm.Spec.Instances
	}
	templateMap, err := getInstanceTemplateMap(rsm.Annotations)
	// error has been checked in prepare stage, there should be no error occurs
	if err != nil {
		return nil
	}
	findTemplate := func() (*corev1.ConfigMap, error) {
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

	template, err := findTemplate()
	// error should not occur as has been checked in tree load stage.
	if err != nil {
		return nil
	}
	if template == nil {
		return rsm.Spec.Instances
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

	return append(rsm.Spec.Instances, extraTemplates...)
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
	if instance.NodeName != nil {
		template.Spec.NodeName = *instance.NodeName
	}
	mergeMap(&instance.Annotations, &template.Annotations)
	mergeMap(&instance.Labels, &template.Labels)
	mergeMap(&instance.NodeSelector, &template.Spec.NodeSelector)
	if len(template.Spec.Containers) > 0 {
		if instance.Image != nil {
			template.Spec.Containers[0].Image = *instance.Image
		}
		if instance.Resources != nil {
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
