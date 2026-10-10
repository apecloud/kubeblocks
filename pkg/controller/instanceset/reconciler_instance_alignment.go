/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"slices"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	workloadlifecycle "github.com/apecloud/kubeblocks/pkg/controller/workloads/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// instanceAlignmentReconciler is responsible for aligning the actual instances(pods) with the desired replicas specified in the spec,
// including horizontal scaling and recovering from unintended pod deletions etc.
// only handle instance count, don't care instance revision.
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
	return kubebuilderx.ConditionSatisfied
}

func (r *instanceAlignmentReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	itsExt, err := instancetemplate.BuildInstanceSetExt(its, tree)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// 1. build desired name to template map
	nameBuilder, err := instancetemplate.NewPodNameBuilder(
		itsExt, &instancetemplate.PodNameBuilderOpts{EventLogger: tree.EventRecorder},
	)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	nameToTemplateMap, err := nameBuilder.BuildInstanceName2TemplateMap()
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// 2. find the create and delete set
	newNameSet := sets.New[string]()
	if !isStopRequested(its) {
		for name := range nameToTemplateMap {
			newNameSet.Insert(name)
		}
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

	// 3. handle alignment (create new instances and delete useless instances)
	// create new instances
	newNameList := sets.List(newNameSet)
	baseSort(newNameList, func(i int) (string, int) {
		return parseParentNameAndOrdinal(newNameList[i])
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
			continue
		}
		if !isOrderedReady && concurrency <= 0 {
			break
		}
		predecessor := getPredecessor(i)
		if isOrderedReady && predecessor != nil && !intctrlutil.IsPodAvailable(predecessor, its.Spec.MinReadySeconds) {
			break
		}
		newPod, err := buildInstancePodByTemplate(name, nameToTemplateMap[name], its, "")
		if err != nil {
			return kubebuilderx.Continue, err
		}

		if err := lifecycleCreatePod(tree, its, newPod); err != nil {
			return lifecycleWait(tree, its, err)
		}
		if err := tree.Add(newPod); err != nil {
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
		pvcs, err := buildInstancePVCByTemplate(name, nameToTemplateMap[name], its)
		if err != nil {
			return kubebuilderx.Continue, err
		}
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

	if workloadlifecycle.Enabled(its) {
		// Guard-only recovered Pods wait until the parent observes a new PVC, then restart with a current task.
		for _, pod := range oldInstanceMap {
			status := workloadlifecycle.Status(its, pod.Name)
			if pod.Status.Phase != corev1.PodPending || !newNameSet.Has(pod.Name) || status == nil || status.DataLoaded == nil || *status.DataLoaded || pod.DeletionTimestamp != nil {
				continue
			}
			cleaned := pod.Spec.DeepCopy()
			changed, err := workloadlifecycle.CleanTask(cleaned)
			if err != nil {
				return kubebuilderx.Continue, err
			}
			if changed {
				// Refresh an init worker when its donor address changed; running databases keep their completed init spec.
				task, err := workloadlifecycle.GeneratedTask(&pod.Spec)
				if err != nil {
					return kubebuilderx.Continue, err
				}
				desiredPod := pod.DeepCopy()
				if err := lifecycleCreatePod(tree, its, desiredPod); err != nil {
					return lifecycleWait(tree, its, err)
				}
				current, err := workloadlifecycle.GeneratedTask(&desiredPod.Spec)
				if err != nil {
					return kubebuilderx.Continue, err
				}
				if task == current {
					continue
				}
			}
			if err := tree.Delete(pod); err != nil {
				return kubebuilderx.Continue, err
			}
			return kubebuilderx.RetryAfter(time.Second), nil
		}
		retention := its.Spec.PersistentVolumeClaimRetentionPolicy
		if !isStopRequested(its) && (retention == nil || retention.WhenScaled != kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType) {
			for _, obj := range oldPVCList {
				name := obj.GetLabels()[constant.KBAppPodNameLabelKey]
				if name == "" || newNameSet.Has(name) || oldNameSet.Has(name) || (!slices.Contains(its.Spec.OfflineInstances, name) && workloadlifecycle.NeedsLeave(its, workloadlifecycle.Status(its, name))) {
					continue
				}
				if err := tree.Delete(obj); err != nil {
					return kubebuilderx.Continue, err
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
		if workloadlifecycle.Enabled(its) && !isStopRequested(its) && !slices.Contains(its.Spec.OfflineInstances, pod.Name) && workloadlifecycle.NeedsLeave(its, workloadlifecycle.Status(its, pod.Name)) {
			return kubebuilderx.RetryAfter(time.Second), nil
		}
		if err := tree.Delete(pod); err != nil {
			return kubebuilderx.Continue, err
		}
		if workloadlifecycle.Enabled(its) {
			return kubebuilderx.RetryAfter(time.Second), nil
		}

		if !isStopRequested(its) {
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
		}

		if isOrderedReady {
			break
		}
		concurrency--
	}

	if workloadlifecycle.Enabled(its) {
		for _, status := range its.Status.InstanceStatus {
			if status.DataLoaded != nil && !*status.DataLoaded && newNameSet.Has(status.PodName) {
				return kubebuilderx.RetryAfter(2 * time.Second), nil
			}
		}
		return kubebuilderx.RetryAfter(5 * time.Second), nil
	}
	return kubebuilderx.Continue, nil
}

var _ kubebuilderx.Reconciler = &instanceAlignmentReconciler{}
