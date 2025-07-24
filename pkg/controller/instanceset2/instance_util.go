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

package instanceset2

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// parseParentNameAndOrdinal parses parent (instance template) Name and ordinal from the give instance name.
// -1 will be returned if no numeric suffix contained.
func parseParentNameAndOrdinal(s string) (string, int) {
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
		name, ordinal := parseParentNameAndOrdinal(objects[i].GetName())
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

func parseNodeSelectorOnceAnnotation(its *workloads.InstanceSet) (map[string]string, error) {
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

func buildInstanceByTemplate(tree *kubebuilderx.ObjectTree,
	instName string, template *instancetemplate.InstanceTemplateExt, its *workloads.InstanceSet) (*workloads.Instance, error) {
	labels := getMatchLabels(its.Name)
	b := builder.NewInstanceBuilder(its.Namespace, instName).
		AddAnnotationsInMap(template.Annotations).
		AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(its.Generation, 10)).
		AddLabelsInMap(template.Labels).
		AddLabelsInMap(labels).
		AddLabels(constant.KBAppInstanceTemplateLabelKey, template.Name).
		SetPodTemplate(*template.DeepCopy()).
		SetSelector(its.Spec.Selector).
		SetMinReadySeconds(its.Spec.MinReadySeconds).
		SetInstanceSetName(its.Name).
		SetInstanceTemplateName(template.Name).
		SetInstanceUpdateStrategyType(its.Spec.InstanceUpdateStrategy).
		SetPodUpdatePolicy(its.Spec.PodUpdatePolicy).
		SetRoles(its.Spec.Roles).
		SetMembershipReconfiguration(its.Spec.MembershipReconfiguration).
		SetTemplateVars(its.Spec.TemplateVars)

	// set these immutable fields only on initial Pod creation, not updates.
	b.SetHostname(instName).
		SetSubdomain(getHeadlessSvcName(its.Name))
	podToNodeMapping, err := parseNodeSelectorOnceAnnotation(its)
	if err != nil {
		return nil, err
	}
	if nodeName, ok := podToNodeMapping[instName]; ok {
		// don't specify nodeName directly here, because it may affect WaitForFirstConsumer StorageClass
		b.SetNodeSelector(map[string]string{corev1.LabelHostname: nodeName})
	}

	for i := range template.VolumeClaimTemplates {
		b.AddVolumeClaimTemplate(template.VolumeClaimTemplates[i])
	}
	b.SetPVCRetentionPolicy(its.Spec.PersistentVolumeClaimRetentionPolicy)

	if its.Spec.CloneAssistantObjects && len(its.Spec.AssistantObjects) > 0 {
		objs, err := cloneAssistantObjects(tree, its)
		if err != nil {
			return nil, err
		}
		b.SetAssistantObjects(objs)
	}

	inst := b.GetObject()
	if !its.Spec.CloneAssistantObjects {
		if err := controllerutil.SetControllerReference(its, inst, model.GetScheme()); err != nil {
			return nil, err
		}
	}
	return inst, nil
}

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

	targetObj := oldObj.DeepCopyObject()
	switch o := newObj.(type) {
	case *corev1.Service:
		return copyAndMergeSvc(targetObj.(*corev1.Service), o)
	default:
		return newObj
	}
}

func copyAndMergeInstance(oldInst, newInst *workloads.Instance) *workloads.Instance {
	targetInst := oldInst.DeepCopyObject().(*workloads.Instance)

	// merge pod
	mergeInPlaceFields(&newInst.Spec.Template, &targetInst.Spec.Template)
	targetInst.Spec.Selector = newInst.Spec.Selector
	targetInst.Spec.MinReadySeconds = newInst.Spec.MinReadySeconds

	// merge pvcs
	for i := range newInst.Spec.VolumeClaimTemplates {
		newPVC := &newInst.Spec.VolumeClaimTemplates[i]
		oldPVC := &targetInst.Spec.VolumeClaimTemplates[i]
		mergeMap(&newPVC.Labels, &oldPVC.Labels)
		mergeMap(&newPVC.Annotations, &oldPVC.Annotations)
		// resources.request.storage and accessModes support in-place update.
		// resources.request.storage only supports volume expansion.
		if reflect.DeepEqual(oldPVC.Spec.AccessModes, newPVC.Spec.AccessModes) &&
			oldPVC.Spec.Resources.Requests.Storage().Cmp(*newPVC.Spec.Resources.Requests.Storage()) >= 0 {
			continue
		}
		oldPVC.Spec.AccessModes = newPVC.Spec.AccessModes
		if newPVC.Spec.Resources.Requests == nil {
			continue
		}
		if _, ok := newPVC.Spec.Resources.Requests[corev1.ResourceStorage]; !ok {
			continue
		}
		requests := oldPVC.Spec.Resources.Requests
		if requests == nil {
			requests = make(corev1.ResourceList)
		}
		requests[corev1.ResourceStorage] = *newPVC.Spec.Resources.Requests.Storage()
		oldPVC.Spec.Resources.Requests = requests
	}
	targetInst.Spec.PersistentVolumeClaimRetentionPolicy = newInst.Spec.PersistentVolumeClaimRetentionPolicy

	copyAndMergeCM := func(old, new *corev1.ConfigMap) client.Object {
		mergeMap(&new.Labels, &old.Labels)
		mergeMap(&new.Annotations, &old.Annotations)
		old.Data = new.Data
		old.BinaryData = new.BinaryData
		return old
	}

	copyAndMergeSecret := func(old, new *corev1.Secret) client.Object {
		mergeMap(&new.Labels, &old.Labels)
		mergeMap(&new.Annotations, &old.Annotations)
		old.Data = new.Data
		old.StringData = new.StringData
		return old
	}

	copyAndMergeSA := func(old, new *corev1.ServiceAccount) client.Object {
		mergeMap(&new.Labels, &old.Labels)
		mergeMap(&new.Annotations, &old.Annotations)
		old.Secrets = new.Secrets
		return old
	}

	copyAndMergeRole := func(old, new *rbacv1.Role) client.Object {
		mergeMap(&new.Labels, &old.Labels)
		mergeMap(&new.Annotations, &old.Annotations)
		old.Rules = new.Rules
		return old
	}

	copyAndMergeRoleBinding := func(old, new *rbacv1.RoleBinding) client.Object {
		mergeMap(&new.Labels, &old.Labels)
		mergeMap(&new.Annotations, &old.Annotations)
		old.Subjects = new.Subjects
		old.RoleRef = new.RoleRef
		return old
	}

	copyNMergeAssistantObjects := func() {
		for i := range newInst.Spec.AssistantObjects {
			oldObj := &targetInst.Spec.AssistantObjects[i]
			newObj := &newInst.Spec.AssistantObjects[i]
			if newObj.ConfigMap != nil {
				copyAndMergeCM(oldObj.ConfigMap, newObj.ConfigMap)
			}
			if newObj.Secret != nil {
				copyAndMergeSecret(oldObj.Secret, newObj.Secret)
			}
			if newObj.ServiceAccount != nil {
				copyAndMergeSA(oldObj.ServiceAccount, newObj.ServiceAccount)
			}
			if newObj.Role != nil {
				copyAndMergeRole(oldObj.Role, newObj.Role)
			}
			if newObj.RoleBinding != nil {
				copyAndMergeRoleBinding(oldObj.RoleBinding, newObj.RoleBinding)
			}
		}
	}

	// merge assistant objects
	if len(targetInst.Spec.AssistantObjects) == 0 {
		targetInst.Spec.AssistantObjects = newInst.Spec.AssistantObjects
	} else {
		copyNMergeAssistantObjects()
	}

	// other fields
	targetInst.Spec.InstanceSetName = newInst.Spec.InstanceSetName
	targetInst.Spec.InstanceTemplateName = newInst.Spec.InstanceTemplateName
	targetInst.Spec.InstanceUpdateStrategyType = newInst.Spec.InstanceUpdateStrategyType
	targetInst.Spec.PodUpdatePolicy = newInst.Spec.PodUpdatePolicy
	targetInst.Spec.Roles = newInst.Spec.Roles
	// targetInst.Spec.MembershipReconfiguration = newInst.Spec.MembershipReconfiguration
	targetInst.Spec.TemplateVars = newInst.Spec.TemplateVars

	// object meta
	mergeMap(&newInst.Labels, &targetInst.Labels)
	mergeMap(&newInst.Annotations, &targetInst.Annotations)

	if equality.Semantic.DeepEqual(&oldInst.Spec, &targetInst.Spec) ||
		!equality.Semantic.DeepEqual(oldInst.Labels, targetInst.Labels) ||
		!equality.Semantic.DeepEqual(oldInst.Annotations, targetInst.Annotations) {
		return targetInst
	}
	return nil
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

func getHeadlessSvcName(itsName string) string {
	return strings.Join([]string{itsName, "headless"}, "-")
}

func mergeInPlaceFields(src, dst *corev1.PodTemplateSpec) {
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

func isInstanceUpdated(its *workloads.InstanceSet, inst *workloads.Instance) bool {
	generation, ok := inst.Annotations[constant.KubeBlocksGenerationKey]
	if !ok {
		return false
	}
	return strconv.FormatInt(its.Generation, 10) == generation
}
