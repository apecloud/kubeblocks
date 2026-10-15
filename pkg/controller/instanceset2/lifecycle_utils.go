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

package instanceset2

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	instctrl "github.com/apecloud/kubeblocks/pkg/controller/instance"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	workloadlifecycle "github.com/apecloud/kubeblocks/pkg/controller/workloads/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const lifecycleCondition = "InstanceLifecycle"

var newMemberLifecycle = workloadlifecycle.NewLifecycle

func observeLifecycleStatus(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its := tree.GetRoot().(*workloads.InstanceSet)
	assignments, _, err := instancetemplate.BuildAssignments(tree, its)
	if err != nil {
		if instancetemplate.IsAssignmentIncomplete(err) {
			return lifecycleWait(tree, its, err)
		}
		return kubebuilderx.Continue, err
	}
	runtimeNames, resourceNames := sets.New[string](), sets.New[string]()
	pods := lifecyclePods(tree)
	for _, pod := range pods {
		runtimeNames.Insert(pod.Name)
		resourceNames.Insert(pod.Name)
	}
	instances := []*workloads.Instance{}
	for _, obj := range tree.List(&workloads.Instance{}) {
		inst := obj.(*workloads.Instance)
		instances = append(instances, inst)
		resourceNames.Insert(inst.Name)
		if inst.Status.CurrentState == workloads.InstanceCurrentStatePresent || inst.Status.CurrentState == workloads.InstanceCurrentStateTerminating {
			runtimeNames.Insert(inst.Name)
		}
	}
	desiredInstances, _, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	for _, assignment := range assignments {
		inst := desiredInstances[assignment.InstanceName]
		pod, err := instctrl.BuildPod(inst)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if its.Spec.LifecycleActions.DataLoad != nil {
			pvc, err := dataPVC(tree, its, pod)
			if err != nil {
				return lifecycleWait(tree, its, err)
			}
			if pvc != nil {
				resourceNames.Insert(assignment.InstanceName)
			}
		}
	}
	registered := workloadlifecycle.Register(its, assignments, runtimeNames, resourceNames)
	previous := its.DeepCopy().Status.InstanceStatus
	for _, pod := range pods {
		if status := instanceLifecycleStatus(its, pod.Name); status != nil {
			status.Provisioned = true
		}
	}
	for _, inst := range instances {
		if inst.Status.CurrentState == workloads.InstanceCurrentStatePresent || inst.Status.CurrentState == workloads.InstanceCurrentStateTerminating {
			if status := instanceLifecycleStatus(its, inst.Name); status != nil {
				status.Provisioned = true
			}
		}
	}
	for i := range its.Status.InstanceStatus {
		status := &its.Status.InstanceStatus[i]
		if its.Spec.LifecycleActions.DataLoad == nil || desiredInstances[status.PodName] == nil {
			continue
		}
		inst := desiredInstances[status.PodName]
		pod, err := instctrl.BuildPod(inst)
		if err != nil {
			return lifecycleWait(tree, its, err)
		}
		pvc, err := dataPVC(tree, its, pod)
		if err != nil {
			return lifecycleWait(tree, its, err)
		}
		if pvc != nil {
			workloadlifecycle.ObserveData(status, pvc)
		}
	}
	if registered || workloadlifecycle.CriticalStatusChanged(previous, its.Status.InstanceStatus) {
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	return kubebuilderx.Continue, nil
}

func reconcileMember(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, inst *workloads.Instance, join bool, reader client.Reader) (kubebuilderx.Result, error) {
	status := instanceLifecycleStatus(its, inst.Name)
	if status == nil || !workloadlifecycle.Enabled(its) || isStopRequested(its) {
		return kubebuilderx.Continue, nil
	}
	if join && (its.Spec.LifecycleActions.MemberJoin == nil || status.MemberJoined == nil || *status.MemberJoined) {
		return kubebuilderx.Continue, nil
	}
	if !join && !workloadlifecycle.NeedsLeave(its, status) {
		return kubebuilderx.Continue, nil
	}
	pods := lifecyclePods(tree)
	leave := !join
	if join {
		current, err := tree.Get(&workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: status.PodName, Namespace: its.Namespace}})
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if current == nil || !current.GetDeletionTimestamp().IsZero() || ptr.Deref(current.(*workloads.Instance).Spec.Stop, false) {
			return kubebuilderx.Continue, nil
		}
	}
	if join && status.DataLoaded != nil && !*status.DataLoaded {
		return kubebuilderx.Continue, nil
	}
	var pod *corev1.Pod
	for _, p := range pods {
		if p.Name == status.PodName {
			pod = p
			break
		}
	}
	if pod == nil || (join && !pod.DeletionTimestamp.IsZero()) {
		return lifecycleWait(tree, its, fmt.Errorf("waiting for runtime of member %s", status.PodName))
	}
	if join && !intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds) {
		return kubebuilderx.Continue, nil
	}
	action := its.Spec.LifecycleActions.MemberJoin
	if leave {
		action = its.Spec.LifecycleActions.MemberLeave
	}
	opts, err := workloadlifecycle.ValidateExecution(action, pod, pods)
	if err != nil {
		return lifecycleWait(tree, its, err)
	}
	lfa, err := newMemberLifecycle(its, pod, pods)
	if err != nil {
		return lifecycleWait(tree, its, err)
	}
	if leave && its.Spec.LifecycleActions.Switchover != nil && exclusiveRole(its, pod) {
		opts, err := workloadlifecycle.ValidateExecution(its.Spec.LifecycleActions.Switchover, pod, pods)
		if err != nil {
			return lifecycleWait(tree, its, err)
		}
		if err := lfa.Switchover(tree.Context, reader, opts, ""); err != nil {
			return lifecycleWait(tree, its, err)
		}
		return lifecycleWait(tree, its, fmt.Errorf("waiting for %s to relinquish its exclusive role", pod.Name))
	}
	if leave {
		err = lfa.MemberLeave(tree.Context, reader, opts)
	} else {
		err = lfa.MemberJoin(tree.Context, reader, opts)
	}
	if err != nil {
		return lifecycleWait(tree, its, err)
	}
	status.MemberJoined = ptr.To(!leave)
	meta.RemoveStatusCondition(&its.Status.Conditions, lifecycleCondition)
	return kubebuilderx.RetryAfter(time.Second), nil
}

func lifecycleWait(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, err error) (kubebuilderx.Result, error) {
	previous := meta.FindStatusCondition(its.Status.Conditions, lifecycleCondition)
	if tree.EventRecorder != nil && (previous == nil || previous.Message != err.Error()) {
		tree.EventRecorder.Eventf(its, corev1.EventTypeWarning, "LifecycleWaiting", "%s", err.Error())
	}
	meta.SetStatusCondition(&its.Status.Conditions, metav1.Condition{Type: lifecycleCondition, Status: metav1.ConditionFalse, ObservedGeneration: its.Generation, Reason: "Waiting", Message: err.Error()})
	return kubebuilderx.RetryAfter(time.Second), nil
}
func lifecyclePods(tree *kubebuilderx.ObjectTree) []*corev1.Pod {
	pods := []*corev1.Pod{}
	for _, obj := range tree.List(&corev1.Pod{}) {
		pods = append(pods, obj.(*corev1.Pod))
	}
	return pods
}
func instanceLifecycleStatus(its *workloads.InstanceSet, name string) *workloads.InstanceStatus {
	for i := range its.Status.InstanceStatus {
		if its.Status.InstanceStatus[i].PodName == name {
			return &its.Status.InstanceStatus[i]
		}
	}
	return nil
}
func exclusiveRole(its *workloads.InstanceSet, pod *corev1.Pod) bool {
	for _, role := range its.Spec.Roles {
		if role.Name == pod.Labels[constant.RoleLabelKey] {
			return role.IsExclusive
		}
	}
	return false
}

type lifecycleReaderKey struct{}

func dataPVC(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod) (*corev1.PersistentVolumeClaim, error) {
	name, err := workloadlifecycle.DataPVCName(its, pod)
	if err != nil {
		return nil, err
	}
	if tree.Context != nil {
		if reader, ok := tree.Context.Value(lifecycleReaderKey{}).(client.Reader); ok {
			ctx := multicluster.IntoContext(tree.Context, its.Annotations[constant.KBAppMultiClusterPlacementKey])
			placement := pod.DeepCopy()
			if obj, err := tree.Get(&workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: pod.Name, Namespace: pod.Namespace}}); err != nil {
				return nil, err
			} else if obj != nil {
				placement.Annotations = obj.GetAnnotations()
			}
			_, ordinal := parseParentNameAndOrdinal(pod.Name)
			placement = multicluster.Assign(ctx, placement, func() int { return max(0, ordinal) }).(*corev1.Pod)
			pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: pod.Namespace, Annotations: placement.Annotations}}
			err := reader.Get(ctx, client.ObjectKeyFromObject(pvc), pvc, multicluster.InDataContext())
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return pvc, err
		}
	}
	obj, err := tree.Get(&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: pod.Namespace}})
	if err != nil || obj == nil {
		return nil, err
	}
	return obj.(*corev1.PersistentVolumeClaim), nil
}

// Refresh one-time inputs using the admitted Instance configuration. Persistent
// worker and action changes still pass through the normal update reconciler.
func refreshInstanceData(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, inst *workloads.Instance) error {
	applied := its.DeepCopy()
	applied.Spec.LifecycleActions = inst.Spec.LifecycleActions
	if !workloadlifecycle.Enabled(applied) || applied.Spec.LifecycleActions.DataLoad == nil {
		return nil
	}
	workerPresent := false
	for i := range inst.Spec.Template.Spec.InitContainers {
		if workloadlifecycle.IsDataWorker(&inst.Spec.Template.Spec.InitContainers[i]) {
			workerPresent = true
		}
	}
	if !workerPresent {
		return nil
	}
	generated := inst.DeepCopy()
	if err := configureInstance(tree, applied, generated, true); err != nil {
		return err
	}
	for i := range inst.Spec.Template.Spec.InitContainers {
		current := &inst.Spec.Template.Spec.InitContainers[i]
		if !workloadlifecycle.IsDataWorker(current) {
			continue
		}
		for _, worker := range generated.Spec.Template.Spec.InitContainers {
			if worker.Name != current.Name {
				continue
			}
			for _, name := range []string{"KB_AGENT_TASK", "KB_AGENT_DATA_LOAD_RESULT"} {
				var replacement *corev1.EnvVar
				for _, env := range worker.Env {
					if env.Name == name {
						value := env
						replacement = &value
						break
					}
				}
				found := false
				for j := 0; j < len(current.Env); j++ {
					if current.Env[j].Name != name {
						continue
					}
					found = true
					if replacement != nil {
						current.Env[j] = *replacement
					} else {
						current.Env = append(current.Env[:j], current.Env[j+1:]...)
					}
					break
				}
				if !found && replacement != nil {
					current.Env = append(current.Env, *replacement)
				}
			}

		}
	}
	inst.Spec.InstanceAssistantObjects = generated.Spec.InstanceAssistantObjects
	return nil
}

func configureInstance(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, inst *workloads.Instance, withTask bool) error {
	if !workloadlifecycle.Enabled(its) || ptr.Deref(its.Spec.Stop, false) {
		return nil
	}
	status := instanceLifecycleStatus(its, inst.Name)
	trackData := status != nil && status.DataLoaded != nil
	pod, err := instctrl.BuildPod(inst)
	if err != nil {
		return err
	}
	completed := true
	var source *corev1.Pod
	if trackData && withTask {
		pvc, err := dataPVC(tree, its, pod)
		if err != nil {
			return err
		}
		loaded := workloads.InstanceStatus{DataLoaded: status.DataLoaded}
		if pvc != nil {
			workloadlifecycle.ObserveData(&loaded, pvc)
		}
		completed = ptr.Deref(loaded.DataLoaded, false)
		if !completed {
			source, err = workloadlifecycle.SourcePod(its, lifecyclePods(tree), inst.Name)
			if err != nil {
				return err
			}
		}
	}
	if err := workloadlifecycle.ConfigurePod(its, pod, source, trackData, completed); err != nil {
		return err
	}
	// Configure runtime volume names without changing the Instance's PVC templates.
	inst.Spec.Template.Spec = pod.Spec
	if trackData && completed && withTask {
		if _, err := workloadlifecycle.CleanTask(&inst.Spec.Template.Spec); err != nil {
			return err
		}
		pvcName, err := workloadlifecycle.DataPVCName(its, pod)
		if err != nil {
			return err
		}
		result := &proto.DataLoadResult{Namespace: pod.Namespace, PVCName: pvcName}
		for i := range inst.Spec.InstanceAssistantObjects {
			if cm := inst.Spec.InstanceAssistantObjects[i].ConfigMap; cm != nil {
				if _, err := workloadlifecycle.CleanTaskConfigMap(cm, result); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
