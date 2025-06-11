/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package instanceset

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type InstanceTemplate interface {
	GetName() string
	GetReplicas() int32
	GetOrdinals() kbappsv1.Ordinals
}

type instanceTemplateExt struct {
	Name     string
	Replicas int32
	corev1.PodTemplateSpec
	VolumeClaimTemplates []corev1.PersistentVolumeClaim
}

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

// ParseParentNameAndOrdinal parses parent (instance template) Name and ordinal from the give instance name.
// -1 will be returned if no numeric suffix contained.
func ParseParentNameAndOrdinal(s string) (string, int) {
	parent := s
	ordinal := -1

	index := strings.LastIndex(s, "-")
	if index < 0 {
		return parent, ordinal
	}
	ordinalStr := s[index+1:]
	if i, err := strconv.ParseInt(ordinalStr, 10, 32); err == nil {
		ordinal = int(i)
		parent = s[:index]
	}
	return parent, ordinal
}

// sortObjects sorts objects by their role priority and name
// e.g.: unknown -> empty -> learner -> follower1 -> follower2 -> leader, with follower1.Name > follower2.Name
// reverse it if reverse==true
func sortObjects[T client.Object](objects []T, rolePriorityMap map[string]int, reverse bool) {
	getRolePriorityFunc := func(i int) int {
		role := strings.ToLower(objects[i].GetLabels()[constant.RoleLabelKey])
		return rolePriorityMap[role]
	}

	// cache the parent names and ordinals to accelerate the parsing process when there is a massive number of Pods.
	namesCache := make(map[string]string, len(objects))
	ordinalsCache := make(map[string]int, len(objects))
	getNameNOrdinalFunc := func(i int) (string, int) {
		if name, ok := namesCache[objects[i].GetName()]; ok {
			return name, ordinalsCache[objects[i].GetName()]
		}
		name, ordinal := ParseParentNameAndOrdinal(objects[i].GetName())
		namesCache[objects[i].GetName()] = name
		ordinalsCache[objects[i].GetName()] = ordinal
		return name, ordinal
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
			return name1 > name2
		}
		return ordinal1 > ordinal2
	})
}

// isRoleReady returns true if pod has role label
func isRoleReady(pod *corev1.Pod, roles []workloads.ReplicaRole) bool {
	if len(roles) == 0 {
		return true
	}
	_, ok := pod.Labels[constant.RoleLabelKey]
	return ok
}

// isCreated returns true if pod has been created and is maintained by the API server
func isCreated(pod *corev1.Pod) bool {
	return pod.Status.Phase != ""
}

// isTerminating returns true if pod's DeletionTimestamp has been set
func isTerminating(pod *corev1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

func isPodPending(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodPending
}

// isImageMatched returns true if all container statuses have same image as defined in pod spec
func isImageMatched(pod *corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		index := slices.IndexFunc(pod.Status.ContainerStatuses, func(status corev1.ContainerStatus) bool {
			return status.Name == container.Name
		})
		if index == -1 {
			continue
		}
		specImage := container.Image
		statusImage := pod.Status.ContainerStatuses[index].Image
		// Image in status may not match the image used in the PodSpec.
		// More info: https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#PodStatus
		specName, specTag, specDigest := imageSplit(specImage)
		statusName, statusTag, statusDigest := imageSplit(statusImage)
		// if digest presents in spec, it must be same in status
		if len(specDigest) != 0 && specDigest != statusDigest {
			return false
		}
		// if tag presents in spec, it must be same in status
		if len(specTag) != 0 && specTag != statusTag {
			return false
		}
		// otherwise, statusName should be same as or has suffix of specName
		if specName != statusName {
			specNames := strings.Split(specName, "/")
			statusNames := strings.Split(statusName, "/")
			if specNames[len(specNames)-1] != statusNames[len(statusNames)-1] {
				return false
			}
		}
	}
	return true
}

// imageSplit separates and returns the name and tag parts
// from the image string using either colon `:` or at `@` separators.
// image reference pattern: [[host[:port]/]component/]component[:tag][@digest]
func imageSplit(imageName string) (name string, tag string, digest string) {
	// check if image name contains a domain
	// if domain is present, ignore domain and check for `:`
	searchName := imageName
	slashIndex := strings.Index(imageName, "/")
	if slashIndex > 0 {
		searchName = imageName[slashIndex:]
	} else {
		slashIndex = 0
	}

	id := strings.Index(searchName, "@")
	ic := strings.Index(searchName, ":")

	// no tag or digest
	if ic < 0 && id < 0 {
		return imageName, "", ""
	}

	// digest only
	if id >= 0 && (id < ic || ic < 0) {
		id += slashIndex
		name = imageName[:id]
		digest = strings.TrimPrefix(imageName[id:], "@")
		return name, "", digest
	}

	// tag and digest
	if id >= 0 && ic >= 0 {
		id += slashIndex
		ic += slashIndex
		name = imageName[:ic]
		tag = strings.TrimPrefix(imageName[ic:id], ":")
		digest = strings.TrimPrefix(imageName[id:], "@")
		return name, tag, digest
	}

	// tag only
	ic += slashIndex
	name = imageName[:ic]
	tag = strings.TrimPrefix(imageName[ic:], ":")
	return name, tag, ""
}

// getPodRevision gets the revision of Pod by inspecting the StatefulSetRevisionLabel. If pod has no revision the empty
// string is returned.
func getPodRevision(pod *corev1.Pod) string {
	if pod.Labels == nil {
		return ""
	}
	return pod.Labels[appsv1.ControllerRevisionHashLabelKey]
}

func ValidateDupInstanceNames[T any](instances []T, getNameFunc func(item T) string) error {
	instanceNameCount := make(map[string]int)
	for _, r := range instances {
		name := getNameFunc(r)
		count, exist := instanceNameCount[name]
		if exist {
			count++
		} else {
			count = 1
		}
		instanceNameCount[name] = count
	}
	dupNames := ""
	for name, count := range instanceNameCount {
		if count > 1 {
			dupNames = fmt.Sprintf("%s%s,", dupNames, name)
		}
	}
	if len(dupNames) > 0 {
		return fmt.Errorf("duplicate pod names: %s", dupNames)
	}
	return nil
}

// Deprecated: should use instancetemplate.PodNameBuilder
func GenerateAllInstanceNames(parentName string, replicas int32, templates []InstanceTemplate, offlineInstances []string, defaultTemplateOrdinals kbappsv1.Ordinals) ([]string, error) {
	totalReplicas := int32(0)
	instanceNameList := make([]string, 0)
	for _, template := range templates {
		replicas := template.GetReplicas()
		ordinalList, err := convertOrdinalsToSortedList(template.GetOrdinals())
		if err != nil {
			return nil, err
		}
		names, err := GenerateInstanceNamesFromTemplate(parentName, template.GetName(), replicas, offlineInstances, ordinalList)
		if err != nil {
			return nil, err
		}
		instanceNameList = append(instanceNameList, names...)
		totalReplicas += replicas
	}
	if totalReplicas < replicas {
		ordinalList, err := convertOrdinalsToSortedList(defaultTemplateOrdinals)
		if err != nil {
			return nil, err
		}
		names, err := GenerateInstanceNamesFromTemplate(parentName, "", replicas-totalReplicas, offlineInstances, ordinalList)
		if err != nil {
			return nil, err
		}
		instanceNameList = append(instanceNameList, names...)
	}
	getNameNOrdinalFunc := func(i int) (string, int) {
		return ParseParentNameAndOrdinal(instanceNameList[i])
	}
	baseSort(instanceNameList, getNameNOrdinalFunc, nil, true)
	return instanceNameList, nil
}

func GenerateInstanceNamesFromTemplate(parentName, templateName string, replicas int32, offlineInstances []string, ordinalList []int32) ([]string, error) {
	instanceNames, err := generateInstanceNames(parentName, templateName, replicas, 0, offlineInstances, ordinalList)
	return instanceNames, err
}

// generateInstanceNames generates instance names based on certain rules:
// The naming convention for instances (pods) based on the Parent Name, InstanceTemplate Name, and ordinal.
// The constructed instance name follows the pattern: $(parent.name)-$(template.name)-$(ordinal).
func generateInstanceNames(parentName, templateName string,
	replicas int32, ordinal int32, offlineInstances []string, ordinalList []int32) ([]string, error) {
	if len(ordinalList) > 0 {
		return generateInstanceNamesWithOrdinalList(parentName, templateName, replicas, offlineInstances, ordinalList)
	}
	usedNames := sets.New(offlineInstances...)
	var instanceNameList []string
	for count := int32(0); count < replicas; count++ {
		var name string
		for {
			if len(templateName) == 0 {
				name = fmt.Sprintf("%s-%d", parentName, ordinal)
			} else {
				name = fmt.Sprintf("%s-%s-%d", parentName, templateName, ordinal)
			}
			ordinal++
			if !usedNames.Has(name) {
				instanceNameList = append(instanceNameList, name)
				break
			}
		}
	}
	return instanceNameList, nil
}

// generateInstanceNamesWithOrdinalList generates instance names based on ordinalList and offlineInstances.
func generateInstanceNamesWithOrdinalList(parentName, templateName string,
	replicas int32, offlineInstances []string, ordinalList []int32) ([]string, error) {
	var instanceNameList []string
	usedNames := sets.New(offlineInstances...)
	slices.Sort(ordinalList)
	for _, ordinal := range ordinalList {
		if len(instanceNameList) >= int(replicas) {
			break
		}
		var name string
		if len(templateName) == 0 {
			name = fmt.Sprintf("%s-%d", parentName, ordinal)
		} else {
			name = fmt.Sprintf("%s-%s-%d", parentName, templateName, ordinal)
		}
		if usedNames.Has(name) {
			continue
		}
		instanceNameList = append(instanceNameList, name)
	}
	if int32(len(instanceNameList)) != replicas {
		errorMessage := fmt.Sprintf("for template '%s', expected %d instance names but generated %d: [%s]",
			templateName, replicas, len(instanceNameList), strings.Join(instanceNameList, ", "))
		return instanceNameList, fmt.Errorf("%s", errorMessage)
	}
	return instanceNameList, nil
}

func getOrdinalsByTemplateName(its *workloads.InstanceSet, templateName string) (kbappsv1.Ordinals, error) {
	if templateName == "" {
		return its.Spec.DefaultTemplateOrdinals, nil
	}
	for _, template := range its.Spec.Instances {
		if template.Name == templateName {
			return template.Ordinals, nil
		}
	}
	return kbappsv1.Ordinals{}, fmt.Errorf("template %s not found", templateName)
}

func convertOrdinalsToSortedList(ordinals kbappsv1.Ordinals) ([]int32, error) {
	ordinalList := sets.New(ordinals.Discrete...)
	for _, item := range ordinals.Ranges {
		start := item.Start
		end := item.End

		if start > end {
			return nil, fmt.Errorf("range's end(%v) must >= start(%v)", end, start)
		}

		for ordinal := start; ordinal <= end; ordinal++ {
			if ordinalList.Has(ordinal) {
				klog.Warningf("Overlap detected: ordinal %v already exists in the ordinals", ordinal)
			}
			ordinalList.Insert(ordinal)
		}
	}
	sortedOrdinalList := ordinalList.UnsortedList()
	slices.Sort(sortedOrdinalList)
	return sortedOrdinalList, nil
}

// ParseNodeSelectorOnceAnnotation will return a non-nil map
func ParseNodeSelectorOnceAnnotation(its *workloads.InstanceSet) (map[string]string, error) {
	podToNodeMapping := make(map[string]string)
	data, ok := its.Annotations[constant.NodeSelectorOnceAnnotationKey]
	if !ok {
		return podToNodeMapping, nil
	}
	if err := json.Unmarshal([]byte(data), &podToNodeMapping); err != nil {
		return nil, fmt.Errorf("can't unmarshal scheduling information: %w", err)
	}
	return podToNodeMapping, nil
}

// sets annotation in place
func deleteNodeSelectorOnceAnnotation(its *workloads.InstanceSet, podName string) error {
	podToNodeMapping, err := ParseNodeSelectorOnceAnnotation(its)
	if err != nil {
		return err
	}
	delete(podToNodeMapping, podName)
	if len(podToNodeMapping) == 0 {
		delete(its.Annotations, constant.NodeSelectorOnceAnnotationKey)
	} else {
		data, err := json.Marshal(podToNodeMapping)
		if err != nil {
			return err
		}
		its.Annotations[constant.NodeSelectorOnceAnnotationKey] = string(data)
	}
	return nil
}

// MergeNodeSelectorOnceAnnotation merges its's nodeSelectorOnce annotation in place
func MergeNodeSelectorOnceAnnotation(its *workloads.InstanceSet, podToNodeMapping map[string]string) error {
	origPodToNodeMapping, err := ParseNodeSelectorOnceAnnotation(its)
	if err != nil {
		return err
	}
	for k, v := range podToNodeMapping {
		origPodToNodeMapping[k] = v
	}
	data, err := json.Marshal(origPodToNodeMapping)
	if err != nil {
		return err
	}
	if its.Annotations == nil {
		its.Annotations = make(map[string]string)
	}
	its.Annotations[constant.NodeSelectorOnceAnnotationKey] = string(data)
	return nil
}

func buildInstancePodByTemplate(name string, template *instancetemplate.InstanceTemplateExt, parent *workloads.InstanceSet, revision string) (*corev1.Pod, error) {
	// 1. build a pod from template
	var err error
	if len(revision) == 0 {
		revision, err = BuildInstanceTemplateRevision(&template.PodTemplateSpec, parent)
		if err != nil {
			return nil, err
		}
	}
	labels := getMatchLabels(parent.Name)
	pod := builder.NewPodBuilder(parent.Namespace, name).
		AddAnnotationsInMap(template.Annotations).
		AddLabelsInMap(template.Labels).
		AddLabelsInMap(labels).
		AddLabels(constant.KBAppPodNameLabelKey, name). // used as a pod-service selector
		AddLabels(instancetemplate.TemplateNameLabelKey, template.Name).
		AddControllerRevisionHashLabel(revision).
		SetPodSpec(*template.Spec.DeepCopy()).
		GetObject()
	// Set these immutable fields only on initial Pod creation, not updates.
	pod.Spec.Hostname = pod.Name
	pod.Spec.Subdomain = getHeadlessSvcName(parent.Name)

	podToNodeMapping, err := ParseNodeSelectorOnceAnnotation(parent)
	if err != nil {
		return nil, err
	}
	if nodeName, ok := podToNodeMapping[name]; ok {
		// don't specify nodeName directly here, because it may affect WaitForFirstConsumer StorageClass
		if pod.Spec.NodeSelector == nil {
			pod.Spec.NodeSelector = make(map[string]string)
		}
		pod.Spec.NodeSelector[corev1.LabelHostname] = nodeName
	}

	// 2. build pvcs from template
	pvcNameMap := make(map[string]string)
	for _, claimTemplate := range template.VolumeClaimTemplates {
		pvcName := intctrlutil.ComposePVCName(claimTemplate, parent.Name, pod.GetName())
		pvcNameMap[pvcName] = claimTemplate.Name
	}

	// 3. update pod volumes
	var volumeList []corev1.Volume
	for pvcName, claimTemplateName := range pvcNameMap {
		volume := builder.NewVolumeBuilder(claimTemplateName).
			SetVolumeSource(corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
			}).GetObject()
		volumeList = append(volumeList, *volume)
	}
	intctrlutil.MergeList(&volumeList, &pod.Spec.Volumes, func(item corev1.Volume) func(corev1.Volume) bool {
		return func(v corev1.Volume) bool {
			return v.Name == item.Name
		}
	})

	if err := controllerutil.SetControllerReference(parent, pod, model.GetScheme()); err != nil {
		return nil, err
	}
	return pod, nil
}

func buildInstancePVCByTemplate(name string, template *instancetemplate.InstanceTemplateExt, parent *workloads.InstanceSet) ([]*corev1.PersistentVolumeClaim, error) {
	var pvcs []*corev1.PersistentVolumeClaim
	labels := getMatchLabels(parent.Name)
	for _, claimTemplate := range template.VolumeClaimTemplates {
		pvcName := intctrlutil.ComposePVCName(claimTemplate, parent.Name, name)
		pvc := builder.NewPVCBuilder(parent.Namespace, pvcName).
			AddLabelsInMap(labels).
			AddLabelsInMap(template.Labels).
			AddLabelsInMap(claimTemplate.Labels).
			AddLabels(constant.VolumeClaimTemplateNameLabelKey, claimTemplate.Name).
			AddLabels(constant.KBAppPodNameLabelKey, name).
			AddAnnotationsInMap(claimTemplate.Annotations).
			SetSpec(*claimTemplate.Spec.DeepCopy()).
			GetObject()
		if template.Name != "" {
			pvc.Labels[constant.KBAppComponentInstanceTemplateLabelKey] = template.Name
		}
		pvcs = append(pvcs, pvc)
	}
	for _, pvc := range pvcs {
		if err := controllerutil.SetControllerReference(parent, pvc, model.GetScheme()); err != nil {
			return nil, err
		}
	}
	return pvcs, nil
}

// copyAndMerge merges two objects for updating:
// 1. new an object targetObj by copying from oldObj
// 2. merge all fields can be updated from newObj into targetObj
func copyAndMerge(oldObj, newObj client.Object) client.Object {
	if reflect.TypeOf(oldObj) != reflect.TypeOf(newObj) {
		return nil
	}

	copyAndMergeSvc := func(oldSvc *corev1.Service, newSvc *corev1.Service) client.Object {
		intctrlutil.MergeList(&newSvc.Finalizers, &oldSvc.Finalizers, func(finalizer string) func(string) bool {
			return func(item string) bool {
				return finalizer == item
			}
		})
		intctrlutil.MergeList(&newSvc.OwnerReferences, &oldSvc.OwnerReferences, func(reference metav1.OwnerReference) func(metav1.OwnerReference) bool {
			return func(item metav1.OwnerReference) bool {
				return reference.UID == item.UID
			}
		})
		mergeMap(&newSvc.Annotations, &oldSvc.Annotations)
		mergeMap(&newSvc.Labels, &oldSvc.Labels)
		oldSvc.Spec.Selector = newSvc.Spec.Selector
		oldSvc.Spec.Type = newSvc.Spec.Type
		oldSvc.Spec.PublishNotReadyAddresses = newSvc.Spec.PublishNotReadyAddresses
		// ignore NodePort&LB svc here, instanceSet only supports default headless svc
		oldSvc.Spec.Ports = newSvc.Spec.Ports
		return oldSvc
	}

	copyAndMergeCm := func(oldCm, newCm *corev1.ConfigMap) client.Object {
		intctrlutil.MergeList(&newCm.Finalizers, &oldCm.Finalizers, func(finalizer string) func(string) bool {
			return func(item string) bool {
				return finalizer == item
			}
		})
		intctrlutil.MergeList(&newCm.OwnerReferences, &oldCm.OwnerReferences, func(reference metav1.OwnerReference) func(metav1.OwnerReference) bool {
			return func(item metav1.OwnerReference) bool {
				return reference.UID == item.UID
			}
		})
		oldCm.Data = newCm.Data
		oldCm.BinaryData = newCm.BinaryData
		return oldCm
	}

	copyAndMergePod := func(oldPod, newPod *corev1.Pod) client.Object {
		mergeInPlaceFields(newPod, oldPod)
		return oldPod
	}

	copyAndMergePVC := func(oldPVC, newPVC *corev1.PersistentVolumeClaim) client.Object {
		mergeMap(&newPVC.Annotations, &oldPVC.Annotations)
		mergeMap(&newPVC.Labels, &oldPVC.Labels)
		// resources.request.storage and accessModes support in-place update.
		// resources.request.storage only supports volume expansion.
		if reflect.DeepEqual(oldPVC.Spec.AccessModes, newPVC.Spec.AccessModes) &&
			oldPVC.Spec.Resources.Requests.Storage().Cmp(*newPVC.Spec.Resources.Requests.Storage()) >= 0 {
			return oldPVC
		}
		oldPVC.Spec.AccessModes = newPVC.Spec.AccessModes
		if newPVC.Spec.Resources.Requests == nil {
			return oldPVC
		}
		if _, ok := newPVC.Spec.Resources.Requests[corev1.ResourceStorage]; !ok {
			return oldPVC
		}
		requests := oldPVC.Spec.Resources.Requests
		if requests == nil {
			requests = make(corev1.ResourceList)
		}
		requests[corev1.ResourceStorage] = *newPVC.Spec.Resources.Requests.Storage()
		oldPVC.Spec.Resources.Requests = requests
		return oldPVC
	}

	targetObj := oldObj.DeepCopyObject()
	switch o := newObj.(type) {
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

func BuildInstanceTemplateRevision(template *corev1.PodTemplateSpec, parent *workloads.InstanceSet) (string, error) {
	podTemplate := filterInPlaceFields(template)
	its := builder.NewInstanceSetBuilder(parent.Namespace, parent.Name).
		SetUID(parent.UID).
		AddAnnotationsInMap(parent.Annotations).
		SetSelectorMatchLabel(parent.Labels).
		SetTemplate(*podTemplate).
		GetObject()

	cr, err := NewRevision(its)
	if err != nil {
		return "", err
	}
	return cr.Labels[ControllerRevisionHashLabel], nil
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

func findTemplateObject(its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree) (*corev1.ConfigMap, error) {
	templateMap, err := getInstanceTemplateMap(its.Annotations)
	// error has been checked in prepare stage, there should be no error occurs
	if err != nil {
		return nil, nil
	}
	for name, templateName := range templateMap {
		if name != its.Name {
			continue
		}
		// find the compressed instance templates, parse them
		template := builder.NewConfigMapBuilder(its.Namespace, templateName).GetObject()
		templateObj, err := tree.Get(template)
		if err != nil {
			return nil, err
		}
		template, _ = templateObj.(*corev1.ConfigMap)
		return template, nil
	}
	return nil, nil
}
