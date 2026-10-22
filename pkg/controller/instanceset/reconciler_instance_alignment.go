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
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	workloadlifecycle "github.com/apecloud/kubeblocks/pkg/controller/workloads/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type instanceAlignmentReconciler struct{ reader client.Reader }

func NewReplicasAlignmentReconciler(reader client.Reader) kubebuilderx.Reconciler {
	return &instanceAlignmentReconciler{reader: reader}
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
	newNameList := sets.List(newNameSet)
	baseSort(newNameList, func(i int) (string, int) {
		return parseParentNameAndOrdinal(newNameList[i])
	}, nil, true)
	workloadlifecycle.SortDataSourcesFirst(its, newNameList)
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
	retirementNames := deleteNameSet.Clone()
	for _, status := range its.Status.InstanceStatus {
		if !newNameSet.Has(status.PodName) && workloadlifecycle.NeedsMemberRetirement(its, &status) {
			retirementNames.Insert(status.PodName)
		}
	}
	for _, pvc := range tree.List(&corev1.PersistentVolumeClaim{}) {
		name := pvc.GetLabels()[constant.KBAppPodNameLabelKey]
		if name != "" && !newNameSet.Has(name) {
			retirementNames.Insert(name)
		}
	}
	retire := func(names sets.Set[string]) (kubebuilderx.Result, error) {
		waiting := false
		candidates := sets.List(names)
		baseSort(candidates, func(i int) (string, int) {
			return parseParentNameAndOrdinal(candidates[i])
		}, nil, false)
		retiredNames := sets.New[string]()
		for _, name := range candidates {
			pod := oldInstanceMap[name]
			absent := pod == nil
			if absent && !workloadlifecycle.NeedsMemberRetirement(its, workloadlifecycle.Status(its, name)) {
				continue
			}
			if !isOrderedReady && concurrency <= 0 {
				break
			}
			if isOrderedReady && !absent && !intctrlutil.IsPodReady(pod) {
				tree.EventRecorder.Eventf(its, corev1.EventTypeWarning, "InstanceSet %s/%s is waiting for Pod %s to be Ready",
					its.Namespace,
					its.Name,
					pod.Name)
			}
			if workloadlifecycle.Enabled(its) && !isStopRequested(its) && !slices.Contains(its.Spec.OfflineInstances, name) {
				if workloadlifecycle.RetirementBlocked(its, workloadlifecycle.Status(its, name)) {
					waiting = true
					continue
				}
				if workloadlifecycle.NeedsMemberRetirement(its, workloadlifecycle.Status(its, name)) {
					if absent {
						pod = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: its.Namespace}}
					}
					return reconcileMember(tree, its, pod, false, r.reader)
				}
			}
			if absent {
				continue
			}
			if err := tree.Delete(pod); err != nil {
				return kubebuilderx.Continue, err
			}
			if workloadlifecycle.Enabled(its) {
				return kubebuilderx.RetryAfter(time.Second), nil
			}

			retiredNames.Insert(pod.Name)

			if isOrderedReady {
				break
			}
			concurrency--
		}

		retentionPolicy := its.Spec.PersistentVolumeClaimRetentionPolicy
		retireClaims := !isStopRequested(its) && (retentionPolicy == nil || retentionPolicy.WhenScaled != kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType)
		if retireClaims {
			for _, obj := range tree.List(&corev1.PersistentVolumeClaim{}) {
				name := obj.GetLabels()[constant.KBAppPodNameLabelKey]
				if name == "" || !names.Has(name) {
					continue
				}
				if workloadlifecycle.Enabled(its) && !slices.Contains(its.Spec.OfflineInstances, name) && workloadlifecycle.RetirementBlocked(its, workloadlifecycle.Status(its, name)) {
					waiting = true
					continue
				}
				if workloadlifecycle.Enabled(its) && !slices.Contains(its.Spec.OfflineInstances, name) && workloadlifecycle.NeedsMemberRetirement(its, workloadlifecycle.Status(its, name)) {
					continue
				}
				if retiredNames.Has(name) || (!oldNameSet.Has(name) && metav1.IsControlledBy(obj, its)) {
					if err := tree.Delete(obj); err != nil {
						return kubebuilderx.Continue, err
					}
				}
			}
		}

		if waiting {
			return kubebuilderx.RetryAfter(time.Second), nil
		}
		return kubebuilderx.Continue, nil
	}
	if workloadlifecycle.Enabled(its) && !isStopRequested(its) {
		independentNames := retirementNames.Intersection(sets.New(its.Spec.OfflineInstances...))
		for name := range retirementNames {
			status := workloadlifecycle.Status(its, name)
			if status != nil && !workloadlifecycle.NeedsLeave(its, status) && ((status.MemberJoined != nil && !*status.MemberJoined) || (status.DataLoaded != nil && !*status.DataLoaded)) {
				independentNames.Insert(name)
			}
		}
		if result, err := retire(independentNames); err != nil || result != kubebuilderx.Continue {
			return result, err
		}
	}

	runtimeChanged := false
	var currentAlignedNameList []string
	for i, name := range newNameList {
		if _, ok := createNameSet[name]; !ok {
			pod := oldInstanceMap[name]
			if changed, err := recreatePendingDataPod(tree, its, pod); err != nil {
				return lifecycleWait(tree, its, err)
			} else if changed {
				return kubebuilderx.RetryAfter(time.Second), nil
			}
			if !runtimeChanged {
				if result, err := reconcileMember(tree, its, pod, true, r.reader); err != nil || result != kubebuilderx.Continue {
					return result, err
				}
			}
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
		runtimeChanged = true
		currentAlignedNameList = append(currentAlignedNameList, name)

		if isOrderedReady {
			break
		}
		concurrency--
	}

	// create PVCs
	for _, name := range currentAlignedNameList {
		if pod := oldInstanceMap[name]; pod != nil && model.IsObjectDeleting(pod) {
			continue
		}
		pvcs, err := buildInstancePVCByTemplate(name, nameToTemplateMap[name], its)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		for _, pvc := range pvcs {
			switch oldPvc, err := tree.Get(pvc); {
			case err != nil:
				return kubebuilderx.Continue, err
			case oldPvc == nil:
				runtimeChanged = true
				if err = tree.Add(pvc); err != nil {
					return kubebuilderx.Continue, err
				}
			default:
				pvcObj := copyAndMerge(oldPvc, pvc)
				if pvcObj != nil {
					if err := tryTakeOverExternalPVC(its, pvcObj.(*corev1.PersistentVolumeClaim)); err != nil {
						return kubebuilderx.Continue, err
					}
					runtimeChanged = runtimeChanged || !equality.Semantic.DeepEqual(oldPvc, pvcObj)
					if err = tree.Update(pvcObj); err != nil {
						return kubebuilderx.Continue, err
					}
				}
			}
		}
	}

	if workloadlifecycle.Enabled(its) && runtimeChanged {
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	if result, err := retire(retirementNames); err != nil || result != kubebuilderx.Continue {
		return result, err
	}

	if workloadlifecycle.Enabled(its) {
		meta.RemoveStatusCondition(&its.Status.Conditions, "InstanceLifecycle")
	}
	return kubebuilderx.Continue, nil
}

var _ kubebuilderx.Reconciler = &instanceAlignmentReconciler{}

func recreatePendingDataPod(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod) (bool, error) {
	status := workloadlifecycle.Status(its, pod.Name)
	if !workloadlifecycle.HasData(its) || status == nil || status.DataLoaded == nil || *status.DataLoaded || pod.Status.Phase != corev1.PodPending || model.IsObjectDeleting(pod) {
		return false, nil
	}

	actual, err := workloadlifecycle.DataTask(&pod.Spec, pod.Name)
	if err != nil {
		return false, err
	}
	desired := pod.DeepCopy()
	if err := lifecycleCreatePod(tree, its, desired); err != nil {
		return false, err
	}
	expected, err := workloadlifecycle.DataTask(&desired.Spec, pod.Name)
	if err != nil {
		return false, err
	}
	if equality.Semantic.DeepEqual(actual, expected) {
		return false, nil
	}
	if err := tree.Delete(pod); err != nil {
		return false, err
	}
	return true, nil
}
