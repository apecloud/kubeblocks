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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// instanceAlignmentReconciler is responsible for aligning the actual instances(pods) with the desired replicas specified in the spec,
// including horizontal scaling and recovering from unintended pod deletions etc.
// only handle instance count, don't care instance revision.
//
// TODO(free6om): support membership reconfiguration
type instanceAlignmentReconciler struct{}

func NewReplicasAlignmentReconciler() kubebuilderx.Reconciler {
	return &instanceAlignmentReconciler{}
}

func (r *instanceAlignmentReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	if err := validateSpec(its, tree); err != nil {
		return kubebuilderx.CheckResultWithError(err)
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *instanceAlignmentReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	itsExt, err := buildInstanceSetExt(its, tree)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// 1. build desired name to template map
	nameToTemplateMap, err := buildInstanceName2TemplateMap(itsExt)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// 2. find the create and delete set
	newNameSet := sets.New[string]()
	for name := range nameToTemplateMap {
		newNameSet.Insert(name)
	}
	oldNameSet := sets.New[string]()
	oldInstanceMap := make(map[string]*corev1.Pod)
	oldInstanceList := tree.List(&corev1.Pod{})
	oldPVCList := tree.List(&corev1.PersistentVolumeClaim{})
	for _, object := range oldInstanceList {
		oldNameSet.Insert(object.GetName())
		pod, _ := object.(*corev1.Pod)
		oldInstanceMap[object.GetName()] = pod
	}
	createNameSet := newNameSet.Difference(oldNameSet)
	deleteNameSet := oldNameSet.Difference(newNameSet)

	// instrumentation: emit a compact, deterministic snapshot of the old pods
	// observed by this reconcile pass. Logged at V(1) so it does not appear in
	// the default INFO stream. The `namespace` and `instanceSet` keys are
	// emitted as separate structured fields so log consumers can filter by
	// (namespace, instanceSet) without parsing a combined "ns/name" string.
	tree.Logger.V(1).Info(
		"alignment: oldInstance snapshot",
		"namespace", its.Namespace,
		"instanceSet", its.Name,
		"oldPods", formatOldInstanceMapSnapshot(oldInstanceMap, its.Spec.MinReadySeconds),
	)

	// default OrderedReady policy
	isOrderedReady := true
	concurrency := 0
	if its.Spec.PodManagementPolicy == appsv1.ParallelPodManagement {
		concurrency, err = CalculateConcurrencyReplicas(its.Spec.ParallelPodManagementConcurrency, int(*its.Spec.Replicas))
		if err != nil {
			return kubebuilderx.Continue, err
		}
		isOrderedReady = false
	}
	// TODO(free6om): handle BestEffortParallel: always keep the majority available.

	// instrumentation: emit the four name sets used by the alignment loop plus
	// the policy/concurrency context. Logged at V(1).
	{
		setSnapshot := formatNameSetSnapshot(oldNameSet, newNameSet, createNameSet, deleteNameSet)
		tree.Logger.V(1).Info(
			"alignment: nameSet snapshot",
			"namespace", its.Namespace,
			"instanceSet", its.Name,
			"podManagementPolicy", string(its.Spec.PodManagementPolicy),
			"concurrencyInit", concurrency,
			"oldNameSet", setSnapshot["oldNameSet"],
			"newNameSet", setSnapshot["newNameSet"],
			"createNameSet", setSnapshot["createNameSet"],
			"deleteNameSet", setSnapshot["deleteNameSet"],
		)
	}

	// 3. handle alignment (create new instances and delete useless instances)
	// create new instances
	newNameList := sets.List(newNameSet)
	baseSort(newNameList, func(i int) (string, int) {
		return ParseParentNameAndOrdinal(newNameList[i])
	}, nil, true)
	getPredecessor := func(i int) *corev1.Pod {
		if i <= 0 {
			return nil
		}
		return oldInstanceMap[newNameList[i-1]]
	}
	if !isOrderedReady {
		for _, name := range newNameList {
			if _, ok := createNameSet[name]; !ok {
				if !intctrlutil.IsPodAvailable(oldInstanceMap[name], its.Spec.MinReadySeconds) {
					concurrency--
				}
			}
		}
	}
	var currentAlignedNameList []string
	for i, name := range newNameList {
		if _, ok := createNameSet[name]; !ok {
			currentAlignedNameList = append(currentAlignedNameList, name)
			// instrumentation: this desired name already has a corresponding
			// pod in oldInstanceMap, so the alignment loop reuses the existing
			// instance instead of creating a new one. Logged at V(1) with a
			// nil-safe pod snapshot so the "expected old pod but got nil" case
			// is distinguishable from the normal reuse case via the
			// `oldPodFound=false` field rendered by formatPodSnapshot.
			// `namespace`, `instanceSet`, `podName`, and `podUID` are emitted
			// as separate structured fields so log consumers can filter by
			// (namespace, instanceSet, podName/uid) without parsing the
			// embedded snapshot string.
			existing := oldInstanceMap[name]
			var podUID string
			if existing != nil {
				podUID = string(existing.UID)
			}
			tree.Logger.V(1).Info(
				"alignment: reuse-existing-instance",
				"namespace", its.Namespace,
				"instanceSet", its.Name,
				"podName", name,
				"podUID", podUID,
				"podSnapshot", formatPodSnapshot(name, existing, its.Spec.MinReadySeconds),
			)
			continue
		}
		if !isOrderedReady && concurrency <= 0 {
			// instrumentation: parallel pod-management has exhausted its
			// per-reconcile concurrency budget, so no more new pods will be
			// created in this pass. Logged at V(1). `podName` carries the
			// desired name that did NOT get created in this pass so callers
			// can filter on the blocked pod even though no pod object yet
			// exists; `podUID` is empty because the pod has not been
			// constructed.
			tree.Logger.V(1).Info(
				"alignment: stop-create-loop",
				"namespace", its.Namespace,
				"instanceSet", its.Name,
				"reason", "concurrency-exhausted",
				"podName", name,
				"podUID", "",
				"podManagementPolicy", string(its.Spec.PodManagementPolicy),
				"concurrency", concurrency,
				"currentIndex", i,
				"totalNameCount", len(newNameList),
			)
			break
		}
		predecessor := getPredecessor(i)
		if isOrderedReady && predecessor != nil && !intctrlutil.IsPodAvailable(predecessor, its.Spec.MinReadySeconds) {
			// instrumentation: ordered pod-management is waiting for the
			// previous-ordinal pod to become Available before it will create
			// the next pod. Logged at V(1). The predecessor snapshot is
			// nil-safe; in this branch predecessor is guaranteed non-nil by
			// the guard above but formatPodSnapshot still handles nil for
			// defensive reasons. `predecessorPodUID` is emitted as a separate
			// structured field for filtering.
			tree.Logger.V(1).Info(
				"alignment: stop-create-loop",
				"namespace", its.Namespace,
				"instanceSet", its.Name,
				"reason", "predecessor-not-available",
				"podName", name,
				"podUID", "",
				"podManagementPolicy", "OrderedReady",
				"currentIndex", i,
				"predecessorPodName", predecessor.Name,
				"predecessorPodUID", string(predecessor.UID),
				"predecessorSnapshot", formatPodSnapshot(predecessor.Name, predecessor, its.Spec.MinReadySeconds),
			)
			break
		}
		inst, err := buildInstanceByTemplate(name, nameToTemplateMap[name], its, "")
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if err := tree.Add(inst.pod); err != nil {
			return kubebuilderx.Continue, err
		}
		currentAlignedNameList = append(currentAlignedNameList, name)

		if isOrderedReady {
			break
		}
		concurrency--
	}

	// create PVCs
	for _, name := range currentAlignedNameList {
		pvcs := buildInstancePVCByTemplate(name, nameToTemplateMap[name], its)
		for _, pvc := range pvcs {
			switch oldPvc, err := tree.Get(pvc); {
			case err != nil:
				return kubebuilderx.Continue, err
			case oldPvc == nil:
				if err = tree.Add(pvc); err != nil {
					return kubebuilderx.Continue, err
				}
			default:
				pvcObj := copyAndMerge(oldPvc, pvc)
				if pvcObj != nil {
					if err := tryTakeOverExternalPVC(its, pvcObj.(*corev1.PersistentVolumeClaim)); err != nil {
						return kubebuilderx.Continue, err
					}
					if err = tree.Update(pvcObj); err != nil {
						return kubebuilderx.Continue, err
					}
				}
			}
		}
	}

	// delete useless instances
	priorities := make(map[string]int)
	sortObjects(oldInstanceList, priorities, false)
	for _, object := range oldInstanceList {
		pod, _ := object.(*corev1.Pod)
		if _, ok := deleteNameSet[pod.Name]; !ok {
			continue
		}
		if !isOrderedReady && concurrency <= 0 {
			break
		}
		if isOrderedReady && !intctrlutil.IsPodReady(pod) {
			tree.EventRecorder.Eventf(its, corev1.EventTypeWarning, "InstanceSet %s/%s is waiting for Pod %s to be Ready",
				its.Namespace,
				its.Name,
				pod.Name)
		}
		if err := tree.Delete(pod); err != nil {
			return kubebuilderx.Continue, err
		}

		retentionPolicy := its.Spec.PersistentVolumeClaimRetentionPolicy
		// the default policy is `Delete`
		if retentionPolicy == nil || retentionPolicy.WhenScaled != kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
			for _, obj := range oldPVCList {
				pvc := obj.(*corev1.PersistentVolumeClaim)
				if pvc.Labels != nil && pvc.Labels[constant.KBAppPodNameLabelKey] == pod.Name {
					if err := tree.Delete(pvc); err != nil {
						return kubebuilderx.Continue, err
					}
				}
			}
		}

		if isOrderedReady {
			break
		}
		concurrency--
	}

	return kubebuilderx.Continue, nil
}

var _ kubebuilderx.Reconciler = &instanceAlignmentReconciler{}
