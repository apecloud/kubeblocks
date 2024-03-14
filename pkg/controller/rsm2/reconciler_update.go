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
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	rsm1 "github.com/apecloud/kubeblocks/pkg/controller/rsm"
)

// updateReconciler handles the updates of replicas based on the UpdateStrategy.
// Currently, two update strategies are supported: 'OnDelete' and 'RollingUpdate'.
type updateReconciler struct{}

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
	replicas := 1
	if rsm.Spec.Replicas != nil {
		replicas = int(*rsm.Spec.Replicas)
	}
	if len(tree.List(&corev1.Pod{})) != replicas {
		return kubebuilderx.ResultUnsatisfied
	}
	if err := validateSpec(rsm); err != nil {
		return kubebuilderx.CheckResultWithError(err)
	}
	return kubebuilderx.ResultSatisfied
}

func (r *updateReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	rsm, _ := tree.GetRoot().(*workloads.ReplicatedStateMachine)
	// 1. build desired name to template map
	nameToTemplateMap := buildReplicaName2TemplateMap(rsm)

	// 2. validate the update set
	newNameSet := sets.NewString()
	for name := range nameToTemplateMap {
		newNameSet.Insert(name)
	}
	oldNameSet := sets.NewString()
	oldReplicaMap := make(map[string]*corev1.Pod)
	oldReplicaList := tree.List(&corev1.Pod{})
	for _, object := range oldReplicaList {
		oldNameSet.Insert(object.GetName())
		pod, _ := object.(*corev1.Pod)
		oldReplicaMap[object.GetName()] = pod
	}
	updateNameSet := oldNameSet.Intersection(newNameSet)
	if len(updateNameSet) != len(oldNameSet) || len(updateNameSet) != len(newNameSet) {
		tree.Logger.Info(fmt.Sprintf("RSM %s/%s replicas are not aligned", rsm.Namespace, rsm.Name))
		return tree, nil
	}

	// 3. do update
	// do nothing if UpdateStrategyType is 'OnDelete'
	if rsm.Spec.UpdateStrategy.Type == apps.OnDeleteStatefulSetStrategyType {
		return tree, nil
	}

	// handle 'RollingUpdate'
	priorities := rsm1.ComposeRolePriorityMap(rsm.Spec.Roles)
	SortObjects(oldReplicaList, priorities, false)
	partition, maxUnavailable, err := parsePartitionNMaxUnavailable(rsm.Spec.UpdateStrategy.RollingUpdate, len(oldReplicaList))
	if err != nil {
		return nil, err
	}
	currentUnavailable := 0
	for _, object := range oldReplicaList {
		pod, _ := object.(*corev1.Pod)
		if !isHealthy(pod) {
			currentUnavailable++
		}
	}
	unavailable := maxUnavailable - currentUnavailable

	// TODO(free6om): compute updateCount from PodManagementPolicy(Serial/OrderedReady, Parallel, BestEffortParallel).
	updateCount := 1
	deletedPods := 0
	updatedPods := 0
	for _, object := range oldReplicaList {
		if deletedPods > updateCount || deletedPods > unavailable {
			break
		}
		if updatedPods >= partition {
			break
		}

		pod, _ := object.(*corev1.Pod)
		if !isHealthy(pod) {
			tree.Logger.Info(fmt.Sprintf("RSM %s/%s blocks on scale-in as the pod %s is not healthy", rsm.Namespace, rsm.Name, pod.Name))
			break
		}
		newPodRevision := rsm.Status.UpdateRevisions[pod.Name]
		if getPodRevision(pod) != newPodRevision && !isTerminating(pod) {
			if err = tree.Delete(pod); err != nil {
				return nil, err
			}
			// TODO(free6om): handle pvc management policy
			// Retain by default.
			deletedPods++
		}
		updatedPods++
	}
	return tree, nil
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
		if err := controllerutil.SetControllerReference(rsm, r.pod, model.GetScheme()); err != nil {
			return nil, err
		}
		for _, pvc := range r.pvcs {
			if err := controllerutil.SetControllerReference(rsm, pvc, model.GetScheme()); err != nil {
				return nil, err
			}
		}
	}

	return replicaList, nil
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
