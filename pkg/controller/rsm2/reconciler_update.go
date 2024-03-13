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
	"fmt"
	"reflect"
	"time"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	rsm1 "github.com/apecloud/kubeblocks/pkg/controller/rsm"
)

type updateReconciler struct{}

type podTemplateSpecExt struct {
	Replicas int32
	corev1.PodTemplateSpec
	VolumeClaimTemplates []corev1.PersistentVolumeClaim
}

type replica struct {
	pod  *corev1.Pod
	pvcs []*corev1.PersistentVolumeClaim
}

var _ kubebuilderx.Reconciler = &updateReconciler{}

func NewUpdateReconciler() kubebuilderx.Reconciler {
	return &updateReconciler{}
}

func (r *updateReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ResultUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ResultUnsatisfied
	}
	rsm, _ := tree.GetRoot().(*workloads.ReplicatedStateMachine)
	if err := validateSpec(rsm); err != nil {
		return kubebuilderx.CheckResultWithError(err)
	}
	return kubebuilderx.ResultSatisfied
}

func (r *updateReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	// handle none workload objects
	if err := handleNoneWorkloadObjectUpdate(tree); err != nil {
		return nil, err
	}

	// handle workload objects(i.e. pod and pvc)
	return tree, handleWorkloadObjectUpdate(tree)
}

func handleNoneWorkloadObjectUpdate(tree *kubebuilderx.ObjectTree) error {
	rsm, _ := tree.GetRoot().(*workloads.ReplicatedStateMachine)

	// generate objects by current spec
	svc := rsm1.BuildSvc(*rsm)
	altSvs := rsm1.BuildAlternativeSvs(*rsm)
	headLessSvc := rsm1.BuildHeadlessSvc(*rsm)
	envConfig := rsm1.BuildEnvConfigMap(*rsm)
	var objects []client.Object
	if svc != nil {
		objects = append(objects, svc)
	}
	for _, s := range altSvs {
		objects = append(objects, s)
	}
	objects = append(objects, headLessSvc, envConfig)
	for _, object := range objects {
		if err := rsm1.SetOwnership(rsm, object, model.GetScheme(), rsm1.GetFinalizer(object)); err != nil {
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
	oldSnapshot := make(map[model.GVKNObjKey]client.Object)
	svcList := tree.List(&corev1.Service{})
	cmList := tree.List(&corev1.ConfigMap{})
	for _, objectList := range [][]client.Object{svcList, cmList} {
		for _, object := range objectList {
			name, err := model.GetGVKName(object)
			if err != nil {
				return err
			}
			oldSnapshot[*name] = object
		}
	}
	// now compute the diff between old and target snapshot and generate the plan
	oldNameSet := sets.KeySet(oldSnapshot)
	newNameSet := sets.KeySet(newSnapshot)

	createSet := newNameSet.Difference(oldNameSet)
	updateSet := newNameSet.Intersection(oldNameSet)
	deleteSet := oldNameSet.Difference(newNameSet)
	for name := range createSet {
		if err := tree.Add(newSnapshot[name]); err != nil {
			return err
		}
	}
	for name := range updateSet {
		oldObj := oldSnapshot[name]
		newObj := copyAndMerge(oldObj, newSnapshot[name])
		if err := tree.Update(newObj); err != nil {
			return err
		}
	}
	for name := range deleteSet {
		if err := tree.Delete(oldSnapshot[name]); err != nil {
			return err
		}
	}
	return nil
}

func handleWorkloadObjectUpdate(tree *kubebuilderx.ObjectTree) error {
	// 1. build desired replicas
	rsm, _ := tree.GetRoot().(*workloads.ReplicatedStateMachine)
	replicaList, err := buildReplicas(rsm)
	if err != nil {
		tree.EventRecorder.Eventf(rsm, corev1.EventTypeWarning, reasonBuildPods, err.Error())
		return model.NewRequeueError(time.Second*10, err.Error())
	}

	// 2. sort replicas by pod name and role priority
	priorities := rsm1.ComposeRolePriorityMap(rsm.Spec.Roles)
	SortReplicas(replicaList, priorities, false)
	newReplicaMap := make(map[string]int, len(replicaList))
	for i := range replicaList {
		newReplicaMap[replicaList[i].pod.Name] = i
	}

	oldReplicaList := tree.List(&corev1.Pod{})
	SortObjects(oldReplicaList, priorities, false)
	oldReplicaMap := make(map[string]int, len(oldReplicaList))
	for i := range oldReplicaList {
		oldReplicaMap[oldReplicaList[i].GetName()] = i
	}

	// now compute the diff between current and desired pods and generate the plan
	oldNameSet := sets.KeySet(oldReplicaMap)
	newNameSet := sets.KeySet(newReplicaMap)

	createSet := newNameSet.Difference(oldNameSet)
	updateSet := newNameSet.Intersection(oldNameSet)
	deleteSet := oldNameSet.Difference(newNameSet)

	// 3. handle scaling
	// TODO(free6om): refine the following block
	if rsm.Spec.PodManagementPolicy == apps.ParallelPodManagement {
		for name := range createSet {
			i := newReplicaMap[name]
			if err = tree.Add(replicaList[i].pod); err != nil {
				return err
			}
			for _, pvc := range replicaList[i].pvcs {
				if err = tree.Add(pvc); err != nil {
					return err
				}
			}
		}
		for name := range deleteSet {
			i := oldReplicaMap[name]
			if err = tree.Delete(oldReplicaList[i]); err != nil {
				return err
			}
			// TODO(free6om): handle pvc management policy
		}
	} else {
		for i, r := range replicaList {
			if _, ok := createSet[r.pod.GetName()]; !ok {
				continue
			}
			if i == 0 || isRunningAndAvailable(replicaList[i-1].pod, rsm.Spec.MinReadySeconds) {
				if err = tree.Add(r.pod); err != nil {
					return err
				}
				for _, pvc := range r.pvcs {
					if err = tree.Add(pvc); err != nil {
						return err
					}
				}
			}
			break
		}
		for i := len(oldReplicaList) - 1; i >= 0; i-- {
			pod, _ := oldReplicaList[i].(*corev1.Pod)
			if _, ok := deleteSet[pod.Name]; !ok {
				continue
			}
			if isRunningAndAvailable(pod, rsm.Spec.MinReadySeconds) {
				if err = tree.Delete(pod); err != nil {
					return err
				}
				// TODO(free6om): handle pvc management policy
				// Retain by default.
			}
			break
		}
	}
	// TODO(free6om): handle BestEffortParallel: always keep the majority available.

	// 4. handle update
	// do nothing if UpdateStrategyType is 'OnDelete'
	if rsm.Spec.UpdateStrategy.Type == apps.OnDeleteStatefulSetStrategyType {
		return nil
	}
	// handle 'RollingUpdate'
	partition, maxUnavailable, err := parsePartitionNMaxUnavailable(rsm.Spec.UpdateStrategy.RollingUpdate, len(replicaList))
	if err != nil {
		return err
	}
	currentUnavailable := 0
	for _, object := range oldReplicaList {
		pod, _ := object.(*corev1.Pod)
		if !isHealthy(pod) {
			currentUnavailable++
		}
	}
	unavailable := maxUnavailable - currentUnavailable

	// TODO(free6om): calculate updateCount from PodManagementPolicy
	updateCount := 1
	deletedPods := 0
	updatedPods := 0
	for _, object := range oldReplicaList {
		pod, _ := object.(*corev1.Pod)
		if _, ok := updateSet[pod.Name]; !ok {
			continue
		}
		newPod := replicaList[newReplicaMap[pod.Name]].pod
		if getPodRevision(pod) != getPodRevision(newPod) && !isTerminating(pod) {
			if err = tree.Delete(pod); err != nil {
				return err
			}
			// TODO(free6om): handle pvc management policy
			// Retain by default.
			deletedPods++
		}
		updatedPods++

		if deletedPods > updateCount || deletedPods > unavailable {
			break
		}
		if updatedPods >= partition {
			break
		}
	}
	return nil
}

func parsePartitionNMaxUnavailable(rollingUpdate *apps.RollingUpdateStatefulSetStrategy, replicas int) (int, int, error) {
	partition := 0
	maxUnavailable := 1
	if rollingUpdate == nil {
		return partition, maxUnavailable, nil
	}
	if rollingUpdate.Partition != nil {
		partition = int(*rollingUpdate.Partition)
	}
	if rollingUpdate.MaxUnavailable != nil {
		maxUnavailableNum, err := intstr.GetScaledValueFromIntOrPercent(intstr.ValueOrDefault(rollingUpdate.MaxUnavailable, intstr.FromInt32(1)), replicas, false)
		if err != nil {
			return 0, 0, err
		}
		// maxUnavailable might be zero for small percentage with round down.
		// So we have to enforce it not to be less than 1.
		if maxUnavailableNum < 1 {
			maxUnavailableNum = 1
		}
		maxUnavailable = maxUnavailableNum
	}
	return partition, maxUnavailable, nil
}

func validateSpec(rsm *workloads.ReplicatedStateMachine) error {
	replicasInTemplates := int32(0)
	var names string
	for _, instance := range rsm.Spec.Instances {
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

func buildReplicas(rsm *workloads.ReplicatedStateMachine) ([]replica, error) {
	// 1. prepare all templates
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
	// build replica list by groups
	var replicaList []replica
	for _, templateList := range podTemplateGroups {
		var (
			replicas []replica
			ordinal  int
			err      error
		)
		for _, template := range templateList {
			replicas, ordinal, err = buildPodByTemplate(template, ordinal, rsm)
			if err != nil {
				return nil, err
			}
			replicaList = append(replicaList, replicas...)
		}
	}
	// validate duplicate pod names
	podNameCount := make(map[string]int)
	updatedRevisions := make(map[string]string, len(replicaList))
	for _, r := range replicaList {
		count, exist := podNameCount[r.pod.GetName()]
		if exist {
			count++
		} else {
			count = 1
		}
		podNameCount[r.pod.GetName()] = count
		updatedRevisions[r.pod.GetName()] = getPodRevision(r.pod)
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

	rsm.Status.UpdateRevisions = updatedRevisions
	rsm.Status.UpdateRevision = getPodRevision(replicaList[len(replicaList)-1].pod)

	// set ownership
	for _, r := range replicaList {
		if err := rsm1.SetOwnership(rsm, r.pod, model.GetScheme(), rsm1.GetFinalizer(r.pod)); err != nil {
			return nil, err
		}
		for _, pvc := range r.pvcs {
			if err := rsm1.SetOwnership(rsm, pvc, model.GetScheme(), rsm1.GetFinalizer(pvc)); err != nil {
				return nil, err
			}
		}
	}

	return replicaList, nil
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

	copyAndMergeSts := func(oldSts, newSts *apps.StatefulSet) client.Object {
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
			oldPVC.Spec.Resources.Requests.Storage().Cmp(*newPVC.Spec.Resources.Requests.Storage()) <= 0 {
			return nil
		}
		oldPVC.Spec.AccessModes = newPVC.Spec.AccessModes
		*oldPVC.Spec.Resources.Requests.Storage() = *newPVC.Spec.Resources.Requests.Storage()
		return oldPVC
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
	case *corev1.PersistentVolumeClaim:
		return copyAndMergePVC(targetObj.(*corev1.PersistentVolumeClaim), o)
	default:
		return newObj
	}
}

func buildPodByTemplate(template *podTemplateSpecExt, ordinal int, parent *workloads.ReplicatedStateMachine) ([]replica, int, error) {
	generatePodName := func(name, generateName string, ordinal int) (string, int) {
		if len(name) > 0 {
			return name, ordinal
		}
		n := fmt.Sprintf("%s-%d", generateName, ordinal)
		ordinal++
		return n, ordinal
	}
	revision, err := buildPodTemplateRevision(template, parent)
	if err != nil {
		return nil, ordinal, err
	}
	var replicaList []replica
	for i := 0; i < int(template.Replicas); i++ {
		// 1. generate pod name
		namespace := template.Namespace
		var name string
		name, ordinal = generatePodName(template.Name, template.GenerateName, ordinal)

		// 2. build a pod from template
		pod := builder.NewPodBuilder(namespace, name).
			AddAnnotationsInMap(template.Annotations).
			AddLabelsInMap(template.Labels).
			AddControllerRevisionHashLabel(revision).
			SetPodSpec(*template.Spec.DeepCopy()).
			GetObject()
		// Set these immutable fields only on initial Pod creation, not updates.
		pod.Spec.Hostname = pod.Name
		pod.Spec.Subdomain = parent.Spec.ServiceName

		// 3. build pvcs from template
		pvcMap := make(map[string]*corev1.PersistentVolumeClaim)
		pvcNameMap := make(map[string]string)
		for _, claimTemplate := range template.VolumeClaimTemplates {
			pvcName := fmt.Sprintf("%s-%s", claimTemplate.Name, pod.GetName())
			pvc := builder.NewPVCBuilder(namespace, pvcName).
				AddLabelsInMap(template.Labels).
				SetSpec(*claimTemplate.Spec.DeepCopy()).
				GetObject()
			pvcMap[pvcName] = pvc
			pvcNameMap[pvcName] = claimTemplate.Name
		}

		// 4. update pod volumes
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

		replica := replica{
			pod:  pod,
			pvcs: pvcs,
		}
		replicaList = append(replicaList, replica)
	}
	return replicaList, ordinal, nil
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
