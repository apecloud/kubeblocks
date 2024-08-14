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

package instanceset

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
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
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type InstanceTemplate interface {
	GetName() string
	GetReplicas() int32
	GetOrdinals() workloads.Ordinals
}

type instanceTemplateExt struct {
	Name     string
	Replicas int32
	corev1.PodTemplateSpec
	VolumeClaimTemplates []corev1.PersistentVolumeClaim
}

type instanceSetExt struct {
	its               *workloads.InstanceSet
	instanceTemplates []*workloads.InstanceTemplate
}

var instanceNameRegex = regexp.MustCompile("(.*)-([0-9]+)$")

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

type instance struct {
	pod  *corev1.Pod
	pvcs []*corev1.PersistentVolumeClaim
}

// ParseParentNameAndOrdinal parses parent (instance template) Name and ordinal from the give instance name.
// -1 will be returned if no numeric suffix contained.
func ParseParentNameAndOrdinal(s string) (string, int) {
	parent := s
	ordinal := -1
	subMatches := instanceNameRegex.FindStringSubmatch(s)
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
// e.g.: unknown -> empty -> learner -> follower1 -> follower2 -> leader, with follower1.Name > follower2.Name
// reverse it if reverse==true
func sortObjects[T client.Object](objects []T, rolePriorityMap map[string]int, reverse bool) {
	getRolePriorityFunc := func(i int) int {
		role := strings.ToLower(objects[i].GetLabels()[constant.RoleLabelKey])
		return rolePriorityMap[role]
	}
	getNameNOrdinalFunc := func(i int) (string, int) {
		return ParseParentNameAndOrdinal(objects[i].GetName())
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

func buildInstanceName2TemplateMap(itsExt *instanceSetExt) (map[string]*instanceTemplateExt, error) {
	instanceTemplateList := buildInstanceTemplateExts(itsExt)
	allNameTemplateMap := make(map[string]*instanceTemplateExt)
	var instanceNameList []string
	for _, template := range instanceTemplateList {
		ordinalList, err := GetOrdinalListByTemplateName(itsExt.its, template.Name)
		if err != nil {
			return nil, err
		}
		instanceNames, err := GenerateInstanceNamesFromTemplate(itsExt.its.Name, template.Name, template.Replicas, itsExt.its.Spec.OfflineInstances, ordinalList)
		if err != nil {
			return nil, err
		}
		instanceNameList = append(instanceNameList, instanceNames...)
		for _, name := range instanceNames {
			allNameTemplateMap[name] = template
		}
	}
	// validate duplicate pod names
	getNameFunc := func(n string) string {
		return n
	}
	if err := ValidateDupInstanceNames(instanceNameList, getNameFunc); err != nil {
		return nil, err
	}

	return allNameTemplateMap, nil
}

func GenerateAllInstanceNames(parentName string, replicas int32, templates []InstanceTemplate, offlineInstances []string, defaultTemplateOrdinals workloads.Ordinals) ([]string, error) {
	totalReplicas := int32(0)
	instanceNameList := make([]string, 0)
	for _, template := range templates {
		replicas := template.GetReplicas()
		ordinalList, err := ConvertOrdinalsToSortedList(template.GetOrdinals())
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
		ordinalList, err := ConvertOrdinalsToSortedList(defaultTemplateOrdinals)
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
	instanceNames, err := GenerateInstanceNames(parentName, templateName, replicas, 0, offlineInstances, ordinalList)
	return instanceNames, err
}

// GenerateInstanceNames generates instance names based on certain rules:
// The naming convention for instances (pods) based on the Parent Name, InstanceTemplate Name, and ordinal.
// The constructed instance name follows the pattern: $(parent.name)-$(template.name)-$(ordinal).
func GenerateInstanceNames(parentName, templateName string,
	replicas int32, ordinal int32, offlineInstances []string, ordinalList []int32) ([]string, error) {
	if len(ordinalList) > 0 {
		return GenerateInstanceNamesWithOrdinalList(parentName, templateName, replicas, offlineInstances, ordinalList)
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

// GenerateInstanceNamesWithOrdinalList generates instance names based on ordinalList and offlineInstances.
func GenerateInstanceNamesWithOrdinalList(parentName, templateName string,
	replicas int32, offlineInstances []string, ordinalList []int32) ([]string, error) {
	var instanceNameList []string
	usedNames := sets.New(offlineInstances...)
	for _, ordinal := range ordinalList {
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

func GetOrdinalListByTemplateName(its *workloads.InstanceSet, templateName string) ([]int32, error) {
	ordinals, err := GetOrdinalsByTemplateName(its, templateName)
	if err != nil {
		return nil, err
	}
	return ConvertOrdinalsToSortedList(ordinals)
}

func GetOrdinalsByTemplateName(its *workloads.InstanceSet, templateName string) (workloads.Ordinals, error) {
	if templateName == "" {
		return its.Spec.DefaultTemplateOrdinals, nil
	}
	for _, template := range its.Spec.Instances {
		if template.Name == templateName {
			return template.Ordinals, nil
		}
	}
	return workloads.Ordinals{}, fmt.Errorf("template %s not found", templateName)
}

func ConvertOrdinalsToSortedList(ordinals workloads.Ordinals) ([]int32, error) {
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

func buildInstanceByTemplate(name string, template *instanceTemplateExt, parent *workloads.InstanceSet, revision string) (*instance, error) {
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
		AddControllerRevisionHashLabel(revision).
		AddLabelsInMap(map[string]string{constant.KBAppPodNameLabelKey: name}).
		SetPodSpec(*template.Spec.DeepCopy()).
		GetObject()
	// Set these immutable fields only on initial Pod creation, not updates.
	pod.Spec.Hostname = pod.Name
	pod.Spec.Subdomain = getHeadlessSvcName(parent.Name)

	// 2. build pvcs from template
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim)
	pvcNameMap := make(map[string]string)
	for _, claimTemplate := range template.VolumeClaimTemplates {
		pvcName := fmt.Sprintf("%s-%s", claimTemplate.Name, pod.GetName())
		pvc := builder.NewPVCBuilder(parent.Namespace, pvcName).
			AddLabelsInMap(template.Labels).
			AddLabelsInMap(labels).
			AddLabels(constant.VolumeClaimTemplateNameLabelKey, claimTemplate.Name).
			SetSpec(*claimTemplate.Spec.DeepCopy()).
			GetObject()
		if template.Name != "" {
			pvc.Labels[constant.KBAppComponentInstanceTemplateLabelKey] = template.Name
		}
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
	intctrlutil.MergeList(&volumeList, &pod.Spec.Volumes, func(item corev1.Volume) func(corev1.Volume) bool {
		return func(v corev1.Volume) bool {
			return v.Name == item.Name
		}
	})

	if err := controllerutil.SetControllerReference(parent, pod, model.GetScheme()); err != nil {
		return nil, err
	}
	inst := &instance{
		pod:  pod,
		pvcs: pvcs,
	}
	return inst, nil
}

func buildInstancePVCByTemplate(name string, template *instanceTemplateExt, parent *workloads.InstanceSet) []*corev1.PersistentVolumeClaim {
	// 2. build pvcs from template
	var pvcs []*corev1.PersistentVolumeClaim
	labels := getMatchLabels(parent.Name)
	for _, claimTemplate := range template.VolumeClaimTemplates {
		pvcName := fmt.Sprintf("%s-%s", claimTemplate.Name, name)
		pvc := builder.NewPVCBuilder(parent.Namespace, pvcName).
			AddLabelsInMap(template.Labels).
			AddLabelsInMap(labels).
			AddLabels(constant.VolumeClaimTemplateNameLabelKey, claimTemplate.Name).
			SetSpec(*claimTemplate.Spec.DeepCopy()).
			GetObject()
		if template.Name != "" {
			pvc.Labels[constant.KBAppComponentInstanceTemplateLabelKey] = template.Name
		}
		pvcs = append(pvcs, pvc)
	}

	return pvcs
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
		mergeMap(&newSvc.Spec.Selector, &oldSvc.Spec.Selector)
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

func validateSpec(its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree) error {
	replicasInTemplates := int32(0)
	itsExt, err := buildInstanceSetExt(its, tree)
	if err != nil {
		return err
	}
	templateNames := sets.New[string]()
	for _, template := range itsExt.instanceTemplates {
		replicas := int32(1)
		if template.Replicas != nil {
			replicas = *template.Replicas
		}
		replicasInTemplates += replicas
		if templateNames.Has(template.Name) {
			err = fmt.Errorf("duplicate instance template name: %s", template.Name)
			if tree != nil {
				tree.EventRecorder.Event(its, corev1.EventTypeWarning, EventReasonInvalidSpec, err.Error())
			}
			return err
		}
		templateNames.Insert(template.Name)
	}
	// sum of spec.templates[*].replicas should not greater than spec.replicas
	if replicasInTemplates > *its.Spec.Replicas {
		err = fmt.Errorf("total replicas in instances(%d) should not greater than replicas in spec(%d)", replicasInTemplates, *its.Spec.Replicas)
		if tree != nil {
			tree.EventRecorder.Event(its, corev1.EventTypeWarning, EventReasonInvalidSpec, err.Error())
		}
		return err
	}

	// try to generate all pod names
	var instances []InstanceTemplate
	for i := range its.Spec.Instances {
		instances = append(instances, &its.Spec.Instances[i])
	}
	_, err = GenerateAllInstanceNames(its.Name, *its.Spec.Replicas, instances, its.Spec.OfflineInstances, its.Spec.DefaultTemplateOrdinals)
	if err != nil {
		if tree != nil {
			tree.EventRecorder.Event(its, corev1.EventTypeWarning, EventReasonInvalidSpec, err.Error())
		}
		return err
	}

	return nil
}

func BuildInstanceTemplateRevision(template *corev1.PodTemplateSpec, parent *workloads.InstanceSet) (string, error) {
	podTemplate := filterInPlaceFields(template)
	its := builder.NewInstanceSetBuilder(parent.Namespace, parent.Name).
		SetUID(parent.UID).
		AddAnnotationsInMap(parent.Annotations).
		AddMatchLabelsInMap(parent.Labels).
		SetTemplate(*podTemplate).
		GetObject()

	cr, err := NewRevision(its)
	if err != nil {
		return "", err
	}
	return cr.Labels[ControllerRevisionHashLabel], nil
}

func buildInstanceTemplateExts(itsExt *instanceSetExt) []*instanceTemplateExt {
	envConfigName := GetEnvConfigMapName(itsExt.its.Name)
	defaultTemplate := BuildPodTemplate(itsExt.its, envConfigName)
	makeInstanceTemplateExt := func(templateName string) *instanceTemplateExt {
		var claims []corev1.PersistentVolumeClaim
		for _, template := range itsExt.its.Spec.VolumeClaimTemplates {
			claims = append(claims, *template.DeepCopy())
		}
		return &instanceTemplateExt{
			Name:                 templateName,
			PodTemplateSpec:      *defaultTemplate.DeepCopy(),
			VolumeClaimTemplates: claims,
		}
	}

	var instanceTemplateExtList []*instanceTemplateExt
	for _, template := range itsExt.instanceTemplates {
		templateExt := makeInstanceTemplateExt(template.Name)
		buildInstanceTemplateExt(*template, templateExt)
		instanceTemplateExtList = append(instanceTemplateExtList, templateExt)
	}
	return instanceTemplateExtList
}

func buildInstanceTemplates(totalReplicas int32, instances []workloads.InstanceTemplate, instancesCompressed *corev1.ConfigMap) []*workloads.InstanceTemplate {
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

	return instanceTemplateList
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

func buildInstanceTemplateExt(template workloads.InstanceTemplate, templateExt *instanceTemplateExt) {
	templateExt.Name = template.Name
	replicas := int32(1)
	if template.Replicas != nil {
		replicas = *template.Replicas
	}
	templateExt.Replicas = replicas
	if template.SchedulingPolicy != nil && template.SchedulingPolicy.NodeName != "" {
		templateExt.Spec.NodeName = template.SchedulingPolicy.NodeName
	}
	mergeMap(&template.Annotations, &templateExt.Annotations)
	mergeMap(&template.Labels, &templateExt.Labels)
	if template.SchedulingPolicy != nil {
		mergeMap(&template.SchedulingPolicy.NodeSelector, &templateExt.Spec.NodeSelector)
	}
	if len(templateExt.Spec.Containers) > 0 {
		if template.Image != nil {
			templateExt.Spec.Containers[0].Image = *template.Image
		}
		if template.Resources != nil {
			src := template.Resources
			dst := &templateExt.Spec.Containers[0].Resources
			mergeCPUNMemory(&src.Limits, &dst.Limits)
			mergeCPUNMemory(&src.Requests, &dst.Requests)
		}
		if template.Env != nil {
			intctrlutil.MergeList(&template.Env, &templateExt.Spec.Containers[0].Env,
				func(item corev1.EnvVar) func(corev1.EnvVar) bool {
					return func(env corev1.EnvVar) bool {
						return env.Name == item.Name
					}
				})
		}
	}

	if template.SchedulingPolicy != nil {
		intctrlutil.MergeList(&template.SchedulingPolicy.Tolerations, &templateExt.Spec.Tolerations,
			func(item corev1.Toleration) func(corev1.Toleration) bool {
				return func(t corev1.Toleration) bool {
					return reflect.DeepEqual(item, t)
				}
			})
		intctrlutil.MergeList(&template.SchedulingPolicy.TopologySpreadConstraints, &templateExt.Spec.TopologySpreadConstraints,
			func(item corev1.TopologySpreadConstraint) func(corev1.TopologySpreadConstraint) bool {
				return func(t corev1.TopologySpreadConstraint) bool {
					return reflect.DeepEqual(item, t)
				}
			})
		mergeAffinity(&template.SchedulingPolicy.Affinity, &templateExt.Spec.Affinity)
	}

	intctrlutil.MergeList(&template.Volumes, &templateExt.Spec.Volumes,
		func(item corev1.Volume) func(corev1.Volume) bool {
			return func(v corev1.Volume) bool {
				return v.Name == item.Name
			}
		})
	intctrlutil.MergeList(&template.VolumeMounts, &templateExt.Spec.Containers[0].VolumeMounts,
		func(item corev1.VolumeMount) func(corev1.VolumeMount) bool {
			return func(vm corev1.VolumeMount) bool {
				return vm.Name == item.Name
			}
		})
	intctrlutil.MergeList(&template.VolumeClaimTemplates, &templateExt.VolumeClaimTemplates,
		func(item corev1.PersistentVolumeClaim) func(corev1.PersistentVolumeClaim) bool {
			return func(claim corev1.PersistentVolumeClaim) bool {
				return claim.Name == item.Name
			}
		})
}

func mergeCPUNMemory(s, d *corev1.ResourceList) {
	if s == nil || *s == nil || d == nil {
		return
	}
	for _, k := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
		if v, ok := (*s)[k]; ok {
			if *d == nil {
				*d = make(corev1.ResourceList)
			}
			(*d)[k] = v
		}
	}
}

// TODO: merge with existing mergeAffinity function which locates at pkg/controller/scheduling/scheduling_utils.go
func mergeAffinity(affinity1Ptr, affinity2Ptr **corev1.Affinity) {
	if affinity1Ptr == nil || *affinity1Ptr == nil {
		return
	}
	if *affinity2Ptr == nil {
		*affinity2Ptr = &corev1.Affinity{}
	}
	affinity1 := *affinity1Ptr
	affinity2 := *affinity2Ptr

	// Merge PodAffinity
	mergePodAffinity(&affinity1.PodAffinity, &affinity2.PodAffinity)

	// Merge PodAntiAffinity
	mergePodAntiAffinity(&affinity1.PodAntiAffinity, &affinity2.PodAntiAffinity)

	// Merge NodeAffinity
	mergeNodeAffinity(&affinity1.NodeAffinity, &affinity2.NodeAffinity)
}

func mergePodAffinity(podAffinity1Ptr, podAffinity2Ptr **corev1.PodAffinity) {
	if podAffinity1Ptr == nil || *podAffinity1Ptr == nil {
		return
	}
	if *podAffinity2Ptr == nil {
		*podAffinity2Ptr = &corev1.PodAffinity{}
	}
	podAffinity1 := *podAffinity1Ptr
	podAffinity2 := *podAffinity2Ptr

	intctrlutil.MergeList(&podAffinity1.RequiredDuringSchedulingIgnoredDuringExecution, &podAffinity2.RequiredDuringSchedulingIgnoredDuringExecution,
		func(item corev1.PodAffinityTerm) func(corev1.PodAffinityTerm) bool {
			return func(t corev1.PodAffinityTerm) bool {
				return reflect.DeepEqual(item, t)
			}
		})
	intctrlutil.MergeList(&podAffinity1.PreferredDuringSchedulingIgnoredDuringExecution, &podAffinity2.PreferredDuringSchedulingIgnoredDuringExecution,
		func(item corev1.WeightedPodAffinityTerm) func(corev1.WeightedPodAffinityTerm) bool {
			return func(t corev1.WeightedPodAffinityTerm) bool {
				return reflect.DeepEqual(item, t)
			}
		})
}

func mergePodAntiAffinity(podAntiAffinity1Ptr, podAntiAffinity2Ptr **corev1.PodAntiAffinity) {
	if podAntiAffinity1Ptr == nil || *podAntiAffinity1Ptr == nil {
		return
	}
	if *podAntiAffinity2Ptr == nil {
		*podAntiAffinity2Ptr = &corev1.PodAntiAffinity{}
	}
	podAntiAffinity1 := *podAntiAffinity1Ptr
	podAntiAffinity2 := *podAntiAffinity2Ptr

	intctrlutil.MergeList(&podAntiAffinity1.RequiredDuringSchedulingIgnoredDuringExecution, &podAntiAffinity2.RequiredDuringSchedulingIgnoredDuringExecution,
		func(item corev1.PodAffinityTerm) func(corev1.PodAffinityTerm) bool {
			return func(t corev1.PodAffinityTerm) bool {
				return reflect.DeepEqual(item, t)
			}
		})
	intctrlutil.MergeList(&podAntiAffinity1.PreferredDuringSchedulingIgnoredDuringExecution, &podAntiAffinity2.PreferredDuringSchedulingIgnoredDuringExecution,
		func(item corev1.WeightedPodAffinityTerm) func(corev1.WeightedPodAffinityTerm) bool {
			return func(t corev1.WeightedPodAffinityTerm) bool {
				return reflect.DeepEqual(item, t)
			}
		})
}

func mergeNodeAffinity(nodeAffinity1Ptr, nodeAffinity2Ptr **corev1.NodeAffinity) {
	if nodeAffinity1Ptr == nil || *nodeAffinity1Ptr == nil {
		return
	}
	if *nodeAffinity2Ptr == nil {
		*nodeAffinity2Ptr = &corev1.NodeAffinity{}
	}
	nodeAffinity1 := *nodeAffinity1Ptr
	nodeAffinity2 := *nodeAffinity2Ptr

	if nodeAffinity1.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		if nodeAffinity2.RequiredDuringSchedulingIgnoredDuringExecution == nil {
			nodeAffinity2.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
		}
		intctrlutil.MergeList(&nodeAffinity1.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
			&nodeAffinity2.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
			func(item corev1.NodeSelectorTerm) func(corev1.NodeSelectorTerm) bool {
				return func(t corev1.NodeSelectorTerm) bool {
					return reflect.DeepEqual(item, t)
				}
			})
	}
	intctrlutil.MergeList(&nodeAffinity1.PreferredDuringSchedulingIgnoredDuringExecution,
		&nodeAffinity2.PreferredDuringSchedulingIgnoredDuringExecution,
		func(item corev1.PreferredSchedulingTerm) func(corev1.PreferredSchedulingTerm) bool {
			return func(t corev1.PreferredSchedulingTerm) bool {
				return reflect.DeepEqual(item, t)
			}
		})
}

func buildInstanceSetExt(its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree) (*instanceSetExt, error) {
	instancesCompressed, err := findTemplateObject(its, tree)
	if err != nil {
		return nil, err
	}

	instanceTemplateList := buildInstanceTemplates(*its.Spec.Replicas, its.Spec.Instances, instancesCompressed)

	return &instanceSetExt{
		its:               its,
		instanceTemplates: instanceTemplateList,
	}, nil
}
