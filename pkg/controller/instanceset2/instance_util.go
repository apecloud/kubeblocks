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

	"github.com/klauspost/compress/zstd"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

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

func getInstanceRevision(inst *workloads.Instance) string {
	return inst.Status.CurrentRevision
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

func buildInstanceByTemplate(tree *kubebuilderx.ObjectTree, name string, template *instancetemplate.InstanceTemplateExt, its *workloads.InstanceSet, revision string) (*workloads.Instance, error) {
	labels := getMatchLabels(its.Name)
	if len(revision) == 0 {
		var err error
		revision, err = buildInstanceTemplateRevision(&template.PodTemplateSpec, its)
		if err != nil {
			return nil, err
		}
	}

	b := builder.NewInstanceBuilder(its.Namespace, name).
		AddAnnotationsInMap(template.Annotations).
		AddLabelsInMap(template.Labels).
		AddLabelsInMap(labels).
		AddLabels(constant.KBAppInstanceTemplateLabelKey, template.Name).
		AddControllerRevisionHashLabel(revision).
		// TODO: labels & annotations for instance and pod
		SetPodSpec(*template.Spec.DeepCopy()).
		SetSelector(its.Spec.Selector).
		SetMinReadySeconds(its.Spec.MinReadySeconds).
		SetInstanceTemplateName(template.Name).
		SetInstanceUpdateStrategyType(its.Spec.InstanceUpdateStrategy).
		SetPodUpdatePolicy(its.Spec.PodUpdatePolicy).
		SetRoles(its.Spec.Roles).
		SetMembershipReconfiguration(its.Spec.MembershipReconfiguration).
		SetTemplateVars(its.Spec.TemplateVars)

	// set these immutable fields only on initial Pod creation, not updates.
	b.SetHostname(name).
		SetSubdomain(getHeadlessSvcName(its.Name))
	podToNodeMapping, err := ParseNodeSelectorOnceAnnotation(its)
	if err != nil {
		return nil, err
	}
	if nodeName, ok := podToNodeMapping[name]; ok {
		// don't specify nodeName directly here, because it may affect WaitForFirstConsumer StorageClass
		b.SetNodeSelector(map[string]string{corev1.LabelHostname: nodeName})
	}

	pvcNameMap := make(map[string]string)
	for _, claimTemplate := range template.VolumeClaimTemplates {
		pvcName := intctrlutil.ComposePVCName(claimTemplate, its.Name, name)
		pvc := builder.NewPVCBuilder(its.Namespace, pvcName).
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
		b.AddVolumeClaimTemplate(*pvc)
		pvcNameMap[pvcName] = claimTemplate.Name
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

	var volumeList []corev1.Volume
	for pvcName, claimTemplateName := range pvcNameMap {
		volume := builder.NewVolumeBuilder(claimTemplateName).
			SetVolumeSource(corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
			}).GetObject()
		volumeList = append(volumeList, *volume)
	}
	intctrlutil.MergeList(&volumeList, &inst.Spec.Template.Spec.Volumes, func(item corev1.Volume) func(corev1.Volume) bool {
		return func(v corev1.Volume) bool {
			return v.Name == item.Name
		}
	})

	if err := controllerutil.SetControllerReference(its, inst, model.GetScheme()); err != nil {
		return nil, err
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

	copyAndMergeInstance := func(oldInst, newInst *workloads.Instance) client.Object {
		// TODO: impl
		return newInst
	}

	targetObj := oldObj.DeepCopyObject()
	switch o := newObj.(type) {
	case *workloads.Instance:
		return copyAndMergeInstance(targetObj.(*workloads.Instance), o)
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
