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
	"sort"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/rsm"
)

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
		instanceNames := GenerateInstanceNamesFromTemplate(itsExt.its.Name, template.Name, template.Replicas, itsExt.its.Spec.OfflineInstances)
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

func GenerateInstanceNamesFromTemplate(parentName, templateName string, replicas int32, offlineInstances []string) []string {
	instanceNames, _ := generateInstanceNames(parentName, templateName, replicas, 0, offlineInstances)
	return instanceNames
}

// generateInstanceNames generates instance names based on certain rules:
// The naming convention for instances (pods) based on the Parent Name, InstanceTemplate Name, and ordinal.
// The constructed instance name follows the pattern: $(parent.name)-$(template.name)-$(ordinal).
func generateInstanceNames(parentName, templateName string,
	replicas int32, ordinal int32, offlineInstances []string) ([]string, int32) {
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
	return instanceNameList, ordinal
}

func buildInstanceByTemplate(name string, template *instanceTemplateExt, parent *workloads.InstanceSet, revision string) (*instance, error) {
	// 1. build a pod from template
	var err error
	if len(revision) == 0 {
		revision, err = buildInstanceTemplateRevision(template, parent)
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
	pod.Spec.Subdomain = parent.Spec.ServiceName

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

	if err := controllerutil.SetControllerReference(parent, pod, model.GetScheme()); err != nil {
		return nil, err
	}
	inst := &instance{
		pod:  pod,
		pvcs: pvcs,
	}
	return inst, nil
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
		mergeInPlaceFields(newPod, oldPod)
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
			return fmt.Errorf("duplicate instance template name: %s", template.Name)
		}
		templateNames.Insert(template.Name)
	}
	// sum of spec.templates[*].replicas should not greater than spec.replicas
	if replicasInTemplates > *its.Spec.Replicas {
		return fmt.Errorf("total replicas in instances(%d) should not greater than replicas in spec(%d)", replicasInTemplates, *its.Spec.Replicas)
	}

	return nil
}

func buildInstanceTemplateRevision(template *instanceTemplateExt, parent *workloads.InstanceSet) (string, error) {
	podTemplate := filterInPlaceFields(&template.PodTemplateSpec)
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
	envConfigName := rsm.GetEnvConfigMapName(itsExt.its.Name)
	defaultTemplate := rsm.BuildPodTemplate(itsExt.its, envConfigName)
	makeInstanceTemplateExt := func() *instanceTemplateExt {
		var claims []corev1.PersistentVolumeClaim
		for _, template := range itsExt.its.Spec.VolumeClaimTemplates {
			claims = append(claims, *template.DeepCopy())
		}
		return &instanceTemplateExt{
			PodTemplateSpec:      *defaultTemplate.DeepCopy(),
			VolumeClaimTemplates: claims,
		}
	}

	var instanceTemplateExtList []*instanceTemplateExt
	for _, template := range itsExt.instanceTemplates {
		templateExt := makeInstanceTemplateExt()
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
	if template.NodeName != nil {
		templateExt.Spec.NodeName = *template.NodeName
	}
	mergeMap(&template.Annotations, &templateExt.Annotations)
	mergeMap(&template.Labels, &templateExt.Labels)
	mergeMap(&template.NodeSelector, &templateExt.Spec.NodeSelector)
	if len(templateExt.Spec.Containers) > 0 {
		if template.Image != nil {
			templateExt.Spec.Containers[0].Image = *template.Image
		}
		if template.Resources != nil {
			templateExt.Spec.Containers[0].Resources = *template.Resources
		}
		if template.Env != nil {
			mergeList(&template.Env, &templateExt.Spec.Containers[0].Env,
				func(item corev1.EnvVar) func(corev1.EnvVar) bool {
					return func(env corev1.EnvVar) bool {
						return env.Name == item.Name
					}
				})
		}
	}
	mergeList(&template.Tolerations, &templateExt.Spec.Tolerations,
		func(item corev1.Toleration) func(corev1.Toleration) bool {
			return func(t corev1.Toleration) bool {
				return reflect.DeepEqual(item, t)
			}
		})
	mergeList(&template.Volumes, &templateExt.Spec.Volumes,
		func(item corev1.Volume) func(corev1.Volume) bool {
			return func(v corev1.Volume) bool {
				return v.Name == item.Name
			}
		})
	mergeList(&template.VolumeMounts, &templateExt.Spec.Containers[0].VolumeMounts,
		func(item corev1.VolumeMount) func(corev1.VolumeMount) bool {
			return func(vm corev1.VolumeMount) bool {
				return vm.Name == item.Name
			}
		})
	mergeList(&template.VolumeClaimTemplates, &templateExt.VolumeClaimTemplates,
		func(item corev1.PersistentVolumeClaim) func(corev1.PersistentVolumeClaim) bool {
			return func(claim corev1.PersistentVolumeClaim) bool {
				return claim.Name == item.Name
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
