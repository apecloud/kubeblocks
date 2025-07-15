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

package instance

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/integer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const defaultPriority = 0

// ComposeRolePriorityMap generates a priority map based on roles.
func ComposeRolePriorityMap(roles []workloads.ReplicaRole) map[string]int {
	rolePriorityMap := make(map[string]int)
	rolePriorityMap[""] = defaultPriority
	for _, role := range roles {
		roleName := strings.ToLower(role.Name)
		rolePriorityMap[roleName] = role.UpdatePriority
	}

	return rolePriorityMap
}

// SortPods sorts pods by their role priority
// e.g.: unknown -> empty -> learner -> follower1 -> follower2 -> leader, with follower1.Name > follower2.Name
// reverse it if reverse==true
func SortPods(pods []corev1.Pod, rolePriorityMap map[string]int, reverse bool) {
	getRolePriorityFunc := func(i int) int {
		role := getRoleName(&pods[i])
		return rolePriorityMap[role]
	}
	getNameNOrdinalFunc := func(i int) (string, int) {
		return parseParentNameAndOrdinal(pods[i].GetName())
	}
	baseSort(pods, getNameNOrdinalFunc, getRolePriorityFunc, reverse)
}

// getRoleName gets role name of pod 'pod'
func getRoleName(pod *corev1.Pod) string {
	return strings.ToLower(pod.Labels[constant.RoleLabelKey])
}

// AddAnnotationScope will add AnnotationScope defined by 'scope' to all keys in map 'annotations'.
func AddAnnotationScope(scope AnnotationScope, annotations map[string]string) map[string]string {
	if annotations == nil {
		return nil
	}
	scopedAnnotations := make(map[string]string, len(annotations))
	for k, v := range annotations {
		scopedAnnotations[fmt.Sprintf("%s%s", k, scope)] = v
	}
	return scopedAnnotations
}

// ParseAnnotationsOfScope parses all annotations with AnnotationScope defined by 'scope'.
// the AnnotationScope suffix of keys in result map will be trimmed.
func ParseAnnotationsOfScope(scope AnnotationScope, scopedAnnotations map[string]string) map[string]string {
	if scopedAnnotations == nil {
		return nil
	}

	annotations := make(map[string]string, 0)
	if scope == RootScope {
		for k, v := range scopedAnnotations {
			if strings.HasSuffix(k, scopeSuffix) {
				continue
			}
			annotations[k] = v
		}
		return annotations
	}

	for k, v := range scopedAnnotations {
		if strings.HasSuffix(k, string(scope)) {
			annotations[strings.TrimSuffix(k, string(scope))] = v
		}
	}
	return annotations
}

func composeRoleMap(its workloads.InstanceSet) map[string]workloads.ReplicaRole {
	roleMap := make(map[string]workloads.ReplicaRole)
	for _, role := range its.Spec.Roles {
		roleMap[strings.ToLower(role.Name)] = role
	}
	return roleMap
}

// mergeMap merge src to dst, dst is modified in place
// Items in src will overwrite items in dst, if possible.
func mergeMap[K comparable, V any](src, dst *map[K]V) {
	if len(*src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[K]V)
	}
	for k, v := range *src {
		(*dst)[k] = v
	}
}

func getMatchLabels(name string) map[string]string {
	return map[string]string{
		constant.AppManagedByLabelKey: constant.AppName,
		WorkloadsManagedByLabelKey:    workloads.InstanceSetKind,
		WorkloadsInstanceLabelKey:     name,
	}
}

// GetMatchLabels exposes getMatchLabels for external usages
// TODO: remove this method when no usage
func GetMatchLabels(name string) map[string]string {
	return getMatchLabels(name)
}

func getHeadlessSvcSelector(its *workloads.InstanceSet) map[string]string {
	selectors := make(map[string]string)
	for k, v := range its.Spec.Selector.MatchLabels {
		selectors[k] = v
	}
	selectors[constant.KBAppReleasePhaseKey] = constant.ReleasePhaseStable
	return selectors
}

// GetPodNameSetFromInstanceSetCondition get the pod name sets from the InstanceSet conditions
func GetPodNameSetFromInstanceSetCondition(its *workloads.InstanceSet, conditionType workloads.ConditionType) map[string]sets.Empty {
	podSet := map[string]sets.Empty{}
	condition := meta.FindStatusCondition(its.Status.Conditions, string(conditionType))
	if condition != nil &&
		condition.Status == metav1.ConditionFalse &&
		condition.Message != "" {
		var podNames []string
		_ = json.Unmarshal([]byte(condition.Message), &podNames)
		podSet = sets.New(podNames...)
	}
	return podSet
}

// CalculateConcurrencyReplicas returns absolute value of concurrency for workload. This func can solve some
// corner cases about percentage-type concurrency, such as:
// - if concurrency > "0%" and replicas > 0, it will ensure at least 1 pod is reserved.
// - if concurrency < "100%" and replicas > 1, it will ensure at least 1 pod is reserved.
//
// if concurrency is nil, concurrency will be treated as 100%.
func CalculateConcurrencyReplicas(concurrency *intstr.IntOrString, replicas int) (int, error) {
	if concurrency == nil {
		return integer.IntMax(replicas, 1), nil
	}

	// 'roundUp=true' will ensure at least 1 pod is reserved if concurrency > "0%" and replicas > 0.
	pValue, err := intstr.GetScaledValueFromIntOrPercent(concurrency, replicas, true)
	if err != nil {
		return pValue, err
	}

	// if concurrency < "100%" and replicas > 1, it will ensure at least 1 pod is reserved.
	if replicas > 1 && pValue == replicas && concurrency.Type == intstr.String && concurrency.StrVal != "100%" {
		pValue = replicas - 1
	}

	// if the calculated concurrency is 0, it will ensure the concurrency at least 1.
	pValue = integer.IntMax(integer.IntMin(pValue, replicas), 1)
	return pValue, nil
}

func getMemberUpdateStrategy(its *workloads.InstanceSet) workloads.MemberUpdateStrategy {
	updateStrategy := workloads.SerialUpdateStrategy
	if its.Spec.MemberUpdateStrategy != nil {
		updateStrategy = *its.Spec.MemberUpdateStrategy
	}
	return updateStrategy
}

func buildInstancePodByTemplate(name string, template *instancetemplate.InstanceTemplateExt, parent *workloads.InstanceSet, revision string) (*corev1.Pod, error) {
	// 1. build a pod from template
	var err error
	if len(revision) == 0 {
		revision, err = buildInstanceTemplateRevision(&template.PodTemplateSpec, parent)
		if err != nil {
			return nil, err
		}
	}
	labels := getMatchLabels(parent.Name)
	pod := builder.NewPodBuilder(parent.Namespace, name).
		AddAnnotationsInMap(template.Annotations).
		AddLabelsInMap(template.Labels).
		AddLabelsInMap(labels).
		AddLabels(constant.KBAppPodNameLabelKey, name).                  // used as a pod-service selector
		AddLabels(instancetemplate.TemplateNameLabelKey, template.Name). // TODO: remove this label later
		AddLabels(constant.KBAppInstanceTemplateLabelKey, template.Name).
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
			pvc.Labels[constant.KBAppInstanceTemplateLabelKey] = template.Name
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

func buildInstanceTemplateRevision(template *corev1.PodTemplateSpec, parent *workloads.InstanceSet) (string, error) {
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

func filterInPlaceFields(src *corev1.PodTemplateSpec) *corev1.PodTemplateSpec {
	template := src.DeepCopy()
	// filter annotations
	var annotations map[string]string
	if len(template.Annotations) > 0 {
		annotations = make(map[string]string)
		// keep Restart annotation
		if restart, ok := template.Annotations[constant.RestartAnnotationKey]; ok {
			annotations[constant.RestartAnnotationKey] = restart
		}
		// keep Reconfigure annotation
		for k, v := range template.Annotations {
			if strings.HasPrefix(k, constant.UpgradeRestartAnnotationKey) {
				annotations[k] = v
			}
		}
		if len(annotations) == 0 {
			annotations = nil
		}
	}
	template.Annotations = annotations
	// filter labels
	template.Labels = nil
	// filter spec.containers[*].images & spec.initContainers[*].images
	for i := range template.Spec.Containers {
		template.Spec.Containers[i].Image = ""
	}
	for i := range template.Spec.InitContainers {
		template.Spec.InitContainers[i].Image = ""
	}
	// filter spec.activeDeadlineSeconds
	template.Spec.ActiveDeadlineSeconds = nil
	// filter spec.tolerations
	template.Spec.Tolerations = nil
	// filter spec.containers[*].resources["cpu|memory"]
	for i := range template.Spec.Containers {
		delete(template.Spec.Containers[i].Resources.Requests, corev1.ResourceCPU)
		delete(template.Spec.Containers[i].Resources.Requests, corev1.ResourceMemory)
		delete(template.Spec.Containers[i].Resources.Limits, corev1.ResourceCPU)
		delete(template.Spec.Containers[i].Resources.Limits, corev1.ResourceMemory)
	}

	return template
}

func mergeInPlaceFields(src, dst *corev1.Pod) {
	mergeMap(&src.Annotations, &dst.Annotations)
	mergeMap(&src.Labels, &dst.Labels)
	dst.Spec.ActiveDeadlineSeconds = src.Spec.ActiveDeadlineSeconds
	// according to the Pod API spec, tolerations can only be appended.
	// means old tolerations must be in new toleration list.
	intctrlutil.MergeList(&src.Spec.Tolerations, &dst.Spec.Tolerations, func(item corev1.Toleration) func(corev1.Toleration) bool {
		return func(t corev1.Toleration) bool {
			return reflect.DeepEqual(item, t)
		}
	})
	for _, container := range src.Spec.InitContainers {
		for i, c := range dst.Spec.InitContainers {
			if container.Name == c.Name {
				dst.Spec.InitContainers[i].Image = container.Image
				break
			}
		}
	}
	mergeResources := func(src, dst *corev1.ResourceList) {
		if len(*src) == 0 {
			return
		}
		if *dst == nil {
			*dst = make(corev1.ResourceList)
		}
		for k, v := range *src {
			(*dst)[k] = v
		}
	}
	ignorePodVerticalScaling := viper.GetBool(FeatureGateIgnorePodVerticalScaling)
	for _, container := range src.Spec.Containers {
		for i, c := range dst.Spec.Containers {
			if container.Name == c.Name {
				dst.Spec.Containers[i].Image = container.Image
				if !ignorePodVerticalScaling {
					requests, limits := copyRequestsNLimitsFields(&container)
					mergeResources(&requests, &dst.Spec.Containers[i].Resources.Requests)
					mergeResources(&limits, &dst.Spec.Containers[i].Resources.Limits)
				}
				break
			}
		}
	}
}

func copyRequestsNLimitsFields(container *corev1.Container) (corev1.ResourceList, corev1.ResourceList) {
	requests := make(corev1.ResourceList)
	limits := make(corev1.ResourceList)
	if len(container.Resources.Requests) > 0 {
		if requestCPU, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
			requests[corev1.ResourceCPU] = requestCPU
		}
		if requestMemory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
			requests[corev1.ResourceMemory] = requestMemory
		}
	}
	if len(container.Resources.Limits) > 0 {
		if limitCPU, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
			limits[corev1.ResourceCPU] = limitCPU
		}
		if limitMemory, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
			limits[corev1.ResourceMemory] = limitMemory
		}
	}
	return requests, limits
}
