/*
Copyright ApeCloud Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package component

import (
	"context"
	"sort"
	"strings"

	"github.com/google/go-cmp/cmp"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ConsensusRole string

type consensusMemberExt struct {
	name          string
	consensusRole ConsensusRole
	accessMode    dbaasv1alpha1.AccessMode
	podName       string
}

const (
	Leader   ConsensusRole = "Leader"
	Follower ConsensusRole = "Follower"
	Learner  ConsensusRole = "Learner"
)

const (
	consensusSetStatusDefaultPodName = "Unknown"
)

// GetPodListByStatefulSet get statefulSet pod list
func GetPodListByStatefulSet(ctx context.Context, cli client.Client, stsObj *appsv1.StatefulSet) ([]corev1.Pod, error) {
	// get podList owned by stsObj
	podList := &corev1.PodList{}
	if err := cli.List(ctx, podList,
		&client.ListOptions{Namespace: stsObj.Namespace},
		client.MatchingLabels{intctrlutil.AppComponentLabelKey: stsObj.Labels[intctrlutil.AppComponentLabelKey]}); err != nil {
		return nil, err
	}
	pods := make([]corev1.Pod, 0)
	for _, pod := range podList.Items {
		if IsMemberOf(stsObj, &pod) {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

// handleConsensusSetUpdate handle ConsensusSet component when it to do updating
func handleConsensusSetUpdate(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster, stsObj *appsv1.StatefulSet) (bool, error) {
	// get typeName from stsObj.name
	typeName := GetComponentTypeName(*cluster, stsObj.Labels[intctrlutil.AppComponentLabelKey])

	// get component from ClusterDefinition by typeName
	component, err := GetComponentFromClusterDefinition(ctx, cli, cluster, typeName)
	if err != nil {
		return false, err
	}

	if component.ComponentType != dbaasv1alpha1.Consensus {
		return true, nil
	}
	pods, err := GetPodListByStatefulSet(ctx, cli, stsObj)
	if err != nil {
		return false, err
	}

	// update cluster.status.component.consensusSetStatus based on all pods currently exist
	componentName := stsObj.Labels[intctrlutil.AppComponentLabelKey]

	// first, get the old status
	var oldConsensusSetStatus *dbaasv1alpha1.ConsensusSetStatus
	if cluster.Status.Components != nil && cluster.Status.Components[componentName] != nil {
		oldConsensusSetStatus = cluster.Status.Components[componentName].ConsensusSetStatus
	}
	newConsensusSetStatus := oldConsensusSetStatus.DeepCopy()
	// then, calculate the new status
	setConsensusSetStatusRoles(newConsensusSetStatus, *component, pods)
	// if status changed, do update
	if !cmp.Equal(newConsensusSetStatus, oldConsensusSetStatus) {
		patch := client.MergeFrom(cluster.DeepCopy())
		if oldConsensusSetStatus != nil {
			cluster.Status.Components[componentName].ConsensusSetStatus = nil
		}
		initClusterComponentStatusIfNeed(cluster, componentName)
		oldConsensusSetStatus = cluster.Status.Components[componentName].ConsensusSetStatus
		setConsensusSetStatusRoles(oldConsensusSetStatus, *component, pods)
		if err = cli.Status().Patch(ctx, cluster, patch); err != nil {
			return false, err
		}
	}

	// get pod label and name, compute plan
	plan := generateConsensusUpdatePlan(ctx, cli, stsObj, pods, *component)
	// execute plan
	return plan.walkOneStep()
}

// generateConsensusUpdatePlan generates Update plan based on UpdateStrategy
func generateConsensusUpdatePlan(ctx context.Context, cli client.Client, stsObj *appsv1.StatefulSet, pods []corev1.Pod,
	component dbaasv1alpha1.ClusterDefinitionComponent) *Plan {
	plan := &Plan{}
	plan.Start = &Step{}
	plan.WalkFunc = func(obj interface{}) (bool, error) {
		pod, ok := obj.(corev1.Pod)
		if !ok {
			return false, errors.New("wrong type: obj not Pod")
		}
		// if pod is the latest version, we do nothing
		if GetPodRevision(&pod) == stsObj.Status.UpdateRevision && stsObj.Generation == stsObj.Status.ObservedGeneration {
			return false, nil
		}
		// if DeletionTimestamp is not nil, it is terminating.
		if pod.DeletionTimestamp != nil {
			return true, nil
		}
		// delete the pod to trigger associate StatefulSet to re-create it
		if err := cli.Delete(ctx, &pod); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}

		return true, nil
	}

	// list all roles
	if component.ConsensusSpec == nil {
		component.ConsensusSpec = &dbaasv1alpha1.ConsensusSetSpec{Leader: dbaasv1alpha1.DefaultLeader}
	}
	leader := component.ConsensusSpec.Leader.Name
	learner := ""
	if component.ConsensusSpec.Learner != nil {
		learner = component.ConsensusSpec.Learner.Name
	}
	// now all are followers
	noneFollowers := make(map[string]string)
	readonlyFollowers := make(map[string]string)
	readWriteFollowers := make(map[string]string)
	// a follower name set
	followers := make(map[string]string)
	exist := "EXIST"
	for _, follower := range component.ConsensusSpec.Followers {
		followers[follower.Name] = exist
		switch follower.AccessMode {
		case dbaasv1alpha1.None:
			noneFollowers[follower.Name] = exist
		case dbaasv1alpha1.Readonly:
			readonlyFollowers[follower.Name] = exist
		case dbaasv1alpha1.ReadWrite:
			readWriteFollowers[follower.Name] = exist
		}
	}

	// make a Serial pod list, e.g.: learner -> follower1 -> follower2 -> leader
	sort.SliceStable(pods, func(i, j int) bool {
		roleI := pods[i].Labels[intctrlutil.ConsensusSetRoleLabelKey]
		roleJ := pods[j].Labels[intctrlutil.ConsensusSetRoleLabelKey]
		if roleI == learner {
			return true
		}
		if roleJ == learner {
			return false
		}
		if roleI == leader {
			return false
		}
		if roleJ == leader {
			return true
		}
		if noneFollowers[roleI] == exist {
			return true
		}
		if noneFollowers[roleJ] == exist {
			return false
		}
		if readonlyFollowers[roleI] == exist {
			return true
		}
		if readonlyFollowers[roleJ] == exist {
			return false
		}
		if readWriteFollowers[roleI] == exist {
			return true
		}

		return false
	})

	// generate plan by UpdateStrategy
	switch component.ConsensusSpec.UpdateStrategy {
	case dbaasv1alpha1.Serial:
		// learner -> followers(none->readonly->readwrite) -> leader
		start := plan.Start
		for _, pod := range pods {
			nextStep := &Step{}
			nextStep.Obj = pod
			start.NextSteps = append(start.NextSteps, nextStep)
			start = nextStep
		}
	case dbaasv1alpha1.Parallel:
		// leader & followers & learner
		start := plan.Start
		for _, pod := range pods {
			nextStep := &Step{}
			nextStep.Obj = pod
			start.NextSteps = append(start.NextSteps, nextStep)
		}
	case dbaasv1alpha1.BestEffortParallel:
		// learner & 1/2 followers -> 1/2 followers -> leader
		start := plan.Start
		// append learner
		index := 0
		for _, pod := range pods {
			if pod.Labels[intctrlutil.ConsensusSetRoleLabelKey] != learner {
				break
			}
			nextStep := &Step{}
			nextStep.Obj = pod
			start.NextSteps = append(start.NextSteps, nextStep)
			index++
		}
		if len(start.NextSteps) > 0 {
			start = start.NextSteps[0]
		}
		// append 1/2 followers
		podList := pods[index:]
		followerCount := 0
		for _, pod := range podList {
			if followers[pod.Labels[intctrlutil.ConsensusSetRoleLabelKey]] == exist {
				followerCount++
			}
		}
		end := followerCount / 2
		for i := 0; i < end; i++ {
			nextStep := &Step{}
			nextStep.Obj = podList[i]
			start.NextSteps = append(start.NextSteps, nextStep)
		}

		if len(start.NextSteps) > 0 {
			start = start.NextSteps[0]
		}
		// append the other 1/2 followers
		podList = podList[end:]
		end = followerCount - end
		for i := 0; i < end; i++ {
			nextStep := &Step{}
			nextStep.Obj = podList[i]
			start.NextSteps = append(start.NextSteps, nextStep)
		}

		if len(start.NextSteps) > 0 {
			start = start.NextSteps[0]
		}
		// append leader
		podList = podList[end:]
		for _, pod := range podList {
			nextStep := &Step{}
			nextStep.Obj = pod
			start.NextSteps = append(start.NextSteps, nextStep)
		}
	}

	return plan
}

func UpdateConsensusSetRoleLabel(cli client.Client, reqCtx intctrlutil.RequestCtx, podName types.NamespacedName, role string) error {
	ctx := reqCtx.Ctx

	// get pod
	pod := &corev1.Pod{}
	if err := cli.Get(ctx, podName, pod); err != nil {
		return err
	}

	// update pod role label
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Labels[intctrlutil.ConsensusSetRoleLabelKey] = role
	err := cli.Patch(ctx, pod, patch)
	if err != nil {
		return err
	}

	// update cluster status
	// get cluster obj
	cluster := &dbaasv1alpha1.Cluster{}
	err = cli.Get(ctx, types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Labels[intctrlutil.AppInstanceLabelKey],
	}, cluster)
	if err != nil {
		return err
	}

	// get componentDef this pod belongs to
	componentName := pod.Labels[intctrlutil.AppComponentLabelKey]
	typeName := GetComponentTypeName(*cluster, componentName)
	componentDef, err := GetComponentFromClusterDefinition(ctx, cli, cluster, typeName)
	if err != nil {
		return err
	}

	// prepare cluster status patch
	patch = client.MergeFrom(cluster.DeepCopy())
	initClusterComponentStatusIfNeed(cluster, componentName)
	consensusSetStatus := cluster.Status.Components[componentName].ConsensusSetStatus
	needUpdate := setConsensusSetStatusRole(consensusSetStatus, *componentDef, role, pod.Name)
	// update cluster status
	if !needUpdate {
		return nil
	}

	if err := cli.Status().Patch(ctx, cluster, patch); err != nil {
		return err
	}

	// update pod accessMode label
	if err := cli.Get(ctx, podName, pod); err != nil {
		return err
	}
	patchAccessMode := client.MergeFrom(pod.DeepCopy())
	roleMap := composeConsensusRoleMap(*componentDef)
	pod.Labels[intctrlutil.ConsensusSetAccessModeLabelKey] = string(roleMap[role].accessMode)

	return cli.Patch(ctx, pod, patchAccessMode)

}
func putConsensusMemberExt(roleMap map[string]consensusMemberExt, name string, role ConsensusRole, accessMode dbaasv1alpha1.AccessMode) {
	if roleMap == nil {
		return
	}

	if name == "" || role == "" || accessMode == "" {
		return
	}

	memberExt := consensusMemberExt{
		name:          name,
		consensusRole: role,
		accessMode:    accessMode,
	}

	roleMap[name] = memberExt
}

func composeConsensusRoleMap(componentDef dbaasv1alpha1.ClusterDefinitionComponent) map[string]consensusMemberExt {
	roleMap := make(map[string]consensusMemberExt, 0)

	putConsensusMemberExt(roleMap,
		componentDef.ConsensusSpec.Leader.Name,
		Leader,
		componentDef.ConsensusSpec.Leader.AccessMode)

	for _, follower := range componentDef.ConsensusSpec.Followers {
		putConsensusMemberExt(roleMap,
			follower.Name,
			Follower,
			follower.AccessMode)
	}

	if componentDef.ConsensusSpec.Learner != nil {
		putConsensusMemberExt(roleMap,
			componentDef.ConsensusSpec.Learner.Name,
			Learner,
			componentDef.ConsensusSpec.Learner.AccessMode)
	}

	return roleMap
}

func initClusterComponentStatusIfNeed(cluster *dbaasv1alpha1.Cluster, componentName string) {
	if cluster.Status.Components == nil {
		cluster.Status.Components = make(map[string]*dbaasv1alpha1.ClusterStatusComponent)
	}
	if cluster.Status.Components[componentName] == nil {
		typeName := GetComponentTypeName(*cluster, componentName)

		cluster.Status.Components[componentName] = &dbaasv1alpha1.ClusterStatusComponent{
			Type:  typeName,
			Phase: dbaasv1alpha1.RunningPhase,
		}
	}
	componentStatus := cluster.Status.Components[componentName]
	if componentStatus.ConsensusSetStatus == nil {
		componentStatus.ConsensusSetStatus = &dbaasv1alpha1.ConsensusSetStatus{
			Leader: dbaasv1alpha1.ConsensusMemberStatus{
				Pod:        consensusSetStatusDefaultPodName,
				AccessMode: dbaasv1alpha1.None,
				Name:       "",
			},
		}
	}
}

func setConsensusSetStatusLeader(consensusSetStatus *dbaasv1alpha1.ConsensusSetStatus, memberExt consensusMemberExt) bool {
	if consensusSetStatus.Leader.Pod == memberExt.podName {
		return false
	}

	consensusSetStatus.Leader.Pod = memberExt.podName
	consensusSetStatus.Leader.AccessMode = memberExt.accessMode
	consensusSetStatus.Leader.Name = memberExt.name

	return true
}

func setConsensusSetStatusFollower(consensusSetStatus *dbaasv1alpha1.ConsensusSetStatus, memberExt consensusMemberExt) bool {
	for _, member := range consensusSetStatus.Followers {
		if member.Pod == memberExt.podName {
			return false
		}
	}

	member := dbaasv1alpha1.ConsensusMemberStatus{
		Pod:        memberExt.podName,
		AccessMode: memberExt.accessMode,
		Name:       memberExt.name,
	}
	consensusSetStatus.Followers = append(consensusSetStatus.Followers, member)
	sort.SliceStable(consensusSetStatus.Followers, func(i, j int) bool {
		fi := consensusSetStatus.Followers[i]
		fj := consensusSetStatus.Followers[j]
		return strings.Compare(fi.Pod, fj.Pod) < 0
	})

	return true
}

func setConsensusSetStatusLearner(consensusSetStatus *dbaasv1alpha1.ConsensusSetStatus, memberExt consensusMemberExt) bool {
	if consensusSetStatus.Learner == nil {
		consensusSetStatus.Learner = &dbaasv1alpha1.ConsensusMemberStatus{}
	}

	if consensusSetStatus.Learner.Pod == memberExt.podName {
		return false
	}

	consensusSetStatus.Learner.Pod = memberExt.podName
	consensusSetStatus.Learner.AccessMode = memberExt.accessMode
	consensusSetStatus.Learner.Name = memberExt.name

	return true
}

func resetConsensusSetStatusRole(consensusSetStatus *dbaasv1alpha1.ConsensusSetStatus, podName string) {
	// reset header
	if consensusSetStatus.Leader.Pod == podName {
		consensusSetStatus.Leader.Pod = consensusSetStatusDefaultPodName
		consensusSetStatus.Leader.AccessMode = dbaasv1alpha1.None
		consensusSetStatus.Leader.Name = ""
	}

	// reset follower
	for index, member := range consensusSetStatus.Followers {
		if member.Pod == podName {
			consensusSetStatus.Followers = append(consensusSetStatus.Followers[:index], consensusSetStatus.Followers[index+1:]...)
		}
	}

	// reset learner
	if consensusSetStatus.Learner != nil && consensusSetStatus.Learner.Pod == podName {
		consensusSetStatus.Learner = nil
	}
}

func setConsensusSetStatusRoles(consensusSetStatus *dbaasv1alpha1.ConsensusSetStatus,
	componentDef dbaasv1alpha1.ClusterDefinitionComponent, pods []corev1.Pod) {
	if consensusSetStatus == nil {
		return
	}

	for _, pod := range pods {
		if role, ok := pod.Labels[intctrlutil.ConsensusSetRoleLabelKey]; ok {
			_ = setConsensusSetStatusRole(consensusSetStatus, componentDef, role, pod.Name)
		}
	}
}

func setConsensusSetStatusRole(consensusSetStatus *dbaasv1alpha1.ConsensusSetStatus,
	componentDef dbaasv1alpha1.ClusterDefinitionComponent,
	role, podName string) bool {
	// mapping role label to consensus member
	roleMap := composeConsensusRoleMap(componentDef)
	memberExt, ok := roleMap[role]
	if !ok {
		return false
	}
	memberExt.podName = podName

	resetConsensusSetStatusRole(consensusSetStatus, memberExt.podName)

	// update cluster.status
	needUpdate := false
	switch memberExt.consensusRole {
	case Leader:
		needUpdate = setConsensusSetStatusLeader(consensusSetStatus, memberExt)
	case Follower:
		needUpdate = setConsensusSetStatusFollower(consensusSetStatus, memberExt)
	case Learner:
		needUpdate = setConsensusSetStatusLearner(consensusSetStatus, memberExt)
	}

	return needUpdate
}
