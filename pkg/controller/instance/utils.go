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
	"reflect"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset2"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func podName(inst *workloads.Instance) string {
	return inst.Name
}

func podObj(inst *workloads.Instance) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: inst.Namespace,
			Name:      podName(inst),
		},
	}
}

// getRoleName gets role name of pod
func getRoleName(pod *corev1.Pod) string {
	return strings.ToLower(pod.Labels[constant.RoleLabelKey])
}

func composeRoleMap(inst *workloads.Instance) map[string]workloads.ReplicaRole {
	roleMap := make(map[string]workloads.ReplicaRole)
	for _, role := range inst.Spec.Roles {
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
		constant.AppManagedByLabelKey:      constant.AppName,
		constant.KBAppInstanceNameLabelKey: name,
	}
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

func buildInstancePod(inst *workloads.Instance, revision string) (*corev1.Pod, error) {
	// 1. build a pod from pod template
	var err error
	if len(revision) == 0 {
		revision, err = buildInstancePodRevision(&inst.Spec.Template, inst)
		if err != nil {
			return nil, err
		}
	}
	labels := getMatchLabels(inst.Name)
	pod := builder.NewPodBuilder(inst.Namespace, inst.Name).
		AddAnnotationsInMap(inst.Spec.Template.Annotations).
		AddLabelsInMap(inst.Spec.Template.Labels).
		AddLabelsInMap(labels).
		AddLabels(constant.KBAppPodNameLabelKey, inst.Name). // used as a pod-service selector
		AddLabels(constant.KBAppInstanceTemplateLabelKey, inst.Spec.InstanceTemplateName).
		AddLabels(instanceset2.WorkloadsInstanceLabelKey, inst.Labels[instanceset2.WorkloadsInstanceLabelKey]). // to select pods by instanceset
		AddControllerRevisionHashLabel(revision).
		SetPodSpec(*inst.Spec.Template.Spec.DeepCopy()).
		GetObject()

	// 2. build pvcs from template
	pvcNameMap := make(map[string]string)
	for _, claimTemplate := range inst.Spec.VolumeClaimTemplates {
		pvcName := intctrlutil.ComposePVCName(corev1.PersistentVolumeClaim{ObjectMeta: claimTemplate.ObjectMeta}, inst.Spec.InstanceSetName, pod.GetName())
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

	if err := controllerutil.SetControllerReference(inst, pod, model.GetScheme()); err != nil {
		return nil, err
	}
	return pod, nil
}

func buildInstancePVCs(inst *workloads.Instance) ([]*corev1.PersistentVolumeClaim, error) {
	var pvcs []*corev1.PersistentVolumeClaim
	labels := getMatchLabels(inst.Name)
	for _, claimTemplate := range inst.Spec.VolumeClaimTemplates {
		pvcName := intctrlutil.ComposePVCName(corev1.PersistentVolumeClaim{ObjectMeta: claimTemplate.ObjectMeta}, inst.Spec.InstanceSetName, inst.Name)
		pvc := builder.NewPVCBuilder(inst.Namespace, pvcName).
			AddLabelsInMap(labels).
			AddLabelsInMap(claimTemplate.Labels).
			AddLabels(constant.KBAppPodNameLabelKey, inst.Name).
			AddLabels(constant.VolumeClaimTemplateNameLabelKey, claimTemplate.Name).
			AddAnnotationsInMap(claimTemplate.Annotations).
			SetSpec(*claimTemplate.Spec.DeepCopy()).
			GetObject()
		if inst.Spec.InstanceTemplateName != "" {
			pvc.Labels[constant.KBAppInstanceTemplateLabelKey] = inst.Spec.InstanceTemplateName
		}
		pvcs = append(pvcs, pvc)
	}
	for _, pvc := range pvcs {
		if err := controllerutil.SetControllerReference(inst, pvc, model.GetScheme()); err != nil {
			return nil, err
		}
	}
	return pvcs, nil
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

func newLifecycleAction(inst *workloads.Instance, objects []client.Object, pod *corev1.Pod) (lifecycle.Lifecycle, error) {
	var (
		clusterName      = inst.Labels[constant.AppInstanceLabelKey]
		compName         = inst.Labels[constant.KBAppComponentLabelKey]
		lifecycleActions = &kbappsv1.ComponentLifecycleActions{
			Switchover:  inst.Spec.LifecycleActions.Switchover,
			Reconfigure: inst.Spec.LifecycleActions.Reconfigure,
		}
		pods []*corev1.Pod
	)
	for i := range objects {
		pods = append(pods, objects[i].(*corev1.Pod))
	}
	return lifecycle.New(inst.Namespace, clusterName, compName,
		lifecycleActions, inst.Spec.LifecycleActions.TemplateVars, pod, pods...)
}
