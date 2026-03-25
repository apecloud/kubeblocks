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
	"errors"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
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

	if retryAfter, err := r.reconcileScaleOutLifecycle(tree, its); err != nil {
		return kubebuilderx.Continue, err
	} else if retryAfter {
		return kubebuilderx.RetryAfter(0), nil
	}

	// delete useless instances
	priorities := make(map[string]int)
	sortObjects(oldInstanceList, priorities, false)
	serialLifecycle := getMemberUpdateStrategy(its) == workloads.SerialUpdateStrategy
	scaleInBatchNames, err := r.selectScaleInBatchNames(its, oldInstanceList, deleteNameSet)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	if len(scaleInBatchNames) > 0 && concurrency < len(scaleInBatchNames) {
		concurrency = len(scaleInBatchNames)
	}
	for _, object := range oldInstanceList {
		pod, _ := object.(*corev1.Pod)
		if _, ok := deleteNameSet[pod.Name]; !ok {
			continue
		}
		if len(scaleInBatchNames) > 0 && !scaleInBatchNames.Has(pod.Name) {
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
		if retryAfter, err := r.reconcileScaleInLifecycle(tree, its, pod); err != nil {
			return kubebuilderx.Continue, err
		} else if retryAfter {
			return kubebuilderx.RetryAfter(0), nil
		}
		status := findInstanceStatus(its, pod.Name)
		joined := status != nil && status.MemberJoined != nil && *status.MemberJoined
		if err := tree.Delete(pod); err != nil {
			return kubebuilderx.Continue, err
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
		if serialLifecycle && joined {
			break
		}
		concurrency--
	}

	return kubebuilderx.Continue, nil
}

func (r *instanceAlignmentReconciler) reconcileScaleOutLifecycle(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet) (bool, error) {
	if its.Spec.LifecycleActions == nil {
		return false, nil
	}
	pods := sortedPods(tree.List(&corev1.Pod{}))
	serialLifecycle := getMemberUpdateStrategy(its) == workloads.SerialUpdateStrategy
	for _, pod := range pods {
		status := findInstanceStatus(its, pod.Name)
		if status == nil || !intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds) {
			continue
		}
		if status.DataLoaded != nil && !*status.DataLoaded {
			done, err := r.runDataLoad(tree, its, pod, status)
			if err != nil {
				return false, err
			}
			if serialLifecycle || !done {
				return true, nil
			}
		}
		if status.MemberJoined != nil && !*status.MemberJoined {
			done, err := r.runMemberJoin(tree, its, pod, status)
			if err != nil {
				return false, err
			}
			if serialLifecycle || !done {
				return true, nil
			}
		}
	}
	return false, nil
}

func (r *instanceAlignmentReconciler) runDataLoad(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod, status *workloads.InstanceStatus) (bool, error) {
	// InstanceSet only orchestrates target-side initialization here.
	// Source-side data export/streaming remains an implementation detail of the lifecycle action itself.
	lfa, err := newLifecycleAction(its, tree, pod)
	if err != nil {
		return false, err
	}
	if err = lfa.DataLoad(tree.Context, tree.Reader, nil); err != nil {
		if errors.Is(err, lifecycle.ErrActionNotDefined) {
			done := true
			status.DataLoaded = &done
			return true, nil
		}
		return false, err
	}
	done := true
	status.DataLoaded = &done
	return true, nil
}

func (r *instanceAlignmentReconciler) runMemberJoin(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod, status *workloads.InstanceStatus) (bool, error) {
	lfa, err := newLifecycleAction(its, tree, pod)
	if err != nil {
		return false, err
	}
	if err = lfa.MemberJoin(tree.Context, tree.Reader, nil); err != nil {
		if errors.Is(err, lifecycle.ErrActionNotDefined) {
			done := true
			status.MemberJoined = &done
			return true, nil
		}
		return false, err
	}
	done := true
	status.MemberJoined = &done
	return true, nil
}

func (r *instanceAlignmentReconciler) reconcileScaleInLifecycle(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod) (bool, error) {
	status := findInstanceStatus(its, pod.Name)
	if status == nil || status.MemberJoined == nil || !*status.MemberJoined {
		return false, nil
	}
	lfa, err := newLifecycleAction(its, tree, pod)
	if err != nil {
		return false, err
	}
	if err = lfa.MemberLeave(tree.Context, tree.Reader, nil); err != nil {
		if errors.Is(err, lifecycle.ErrActionNotDefined) {
			done := false
			status.MemberJoined = &done
			return false, nil
		}
		return false, err
	}
	done := false
	status.MemberJoined = &done
	return false, nil
}

func (r *instanceAlignmentReconciler) selectScaleInBatchNames(its *workloads.InstanceSet, oldInstanceList []client.Object, deleteNameSet sets.Set[string]) (sets.Set[string], error) {
	if its.Spec.LifecycleActions == nil || its.Spec.LifecycleActions.MemberLeave == nil || deleteNameSet.Len() == 0 {
		return nil, nil
	}
	pods := make([]corev1.Pod, 0, deleteNameSet.Len())
	for _, object := range oldInstanceList {
		pod := object.(*corev1.Pod)
		if deleteNameSet.Has(pod.Name) {
			pods = append(pods, *pod)
		}
	}
	if len(pods) == 0 {
		return nil, nil
	}
	plan := &realUpdatePlan{
		its:  *its,
		pods: pods,
		dag:  graph.NewDAG(),
		isPodUpdated: func(_ *workloads.InstanceSet, _ *corev1.Pod) (bool, error) {
			return false, nil
		},
	}
	selected, err := plan.Execute()
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, nil
	}
	names := sets.New[string]()
	for _, pod := range selected {
		names.Insert(pod.Name)
	}
	return names, nil
}

func findInstanceStatus(its *workloads.InstanceSet, podName string) *workloads.InstanceStatus {
	for i := range its.Status.InstanceStatus {
		if its.Status.InstanceStatus[i].PodName == podName {
			return &its.Status.InstanceStatus[i]
		}
	}
	return nil
}

func sortedPods(objects []client.Object) []*corev1.Pod {
	pods := make([]*corev1.Pod, 0, len(objects))
	for _, obj := range objects {
		pods = append(pods, obj.(*corev1.Pod))
	}
	slices.SortFunc(pods, func(a, b *corev1.Pod) int {
		aParent, aOrdinal := parseParentNameAndOrdinal(a.Name)
		bParent, bOrdinal := parseParentNameAndOrdinal(b.Name)
		if aParent != bParent {
			if aParent < bParent {
				return -1
			}
			return 1
		}
		switch {
		case aOrdinal < bOrdinal:
			return -1
		case aOrdinal > bOrdinal:
			return 1
		default:
			return 0
		}
	})
	return pods
}

var _ kubebuilderx.Reconciler = &instanceAlignmentReconciler{}
