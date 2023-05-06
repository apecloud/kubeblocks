/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package consensusset

import (
	"errors"

	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type updatePlan interface {
	// execute executes the plan
	// return error when any error occurred
	// return pods to be updated,
	// nil slice means no pods need to be updated
	execute() ([]*corev1.Pod, error)
}

type realUpdatePlan struct {
	csSet           workloads.ConsensusSet
	pods            []corev1.Pod
	dag             *graph.DAG
	podsToBeUpdated []*corev1.Pod
}

var (
	ErrContinue error
	ErrWait     = errors.New("wait")
	ErrStop     = errors.New("stop")
)

// planWalkFunc decides whether vertex should be updated
// nil error means vertex should be updated
func (p *realUpdatePlan) planWalkFunc(vertex graph.Vertex) error {
	v, _ := vertex.(*model.ObjectVertex)
	if v.Obj == nil {
		return ErrContinue
	}
	pod, ok := v.Obj.(*corev1.Pod)
	if !ok {
		return ErrContinue
	}

	// if DeletionTimestamp is not nil, it is terminating.
	if !pod.DeletionTimestamp.IsZero() {
		return ErrWait
	}

	// if pod is the latest version, we do nothing
	if intctrlutil.GetPodRevision(pod) == p.csSet.Status.UpdateRevision {
		if intctrlutil.PodIsReadyWithLabel(*pod) {
			return ErrContinue
		} else {
			return ErrWait
		}
	}

	// delete the pod to trigger associate StatefulSet to re-create it
	p.podsToBeUpdated = append(p.podsToBeUpdated, pod)
	return ErrStop
}

// build builds the update plan based on updateStrategy
func (p *realUpdatePlan) build() {
	// make a root vertex with nil Obj
	root := &model.ObjectVertex{}
	p.dag.AddVertex(root)

	rolePriorityMap := composeRolePriorityMap(p.csSet)
	sortPods(p.pods, rolePriorityMap, false)

	// generate plan by UpdateStrategy
	switch p.csSet.Spec.UpdateStrategy {
	case workloads.SerialUpdateStrategy:
		p.buildSerialUpdatePlan()
	case workloads.ParallelUpdateStrategy:
		p.buildParallelUpdatePlan()
	case workloads.BestEffortParallelUpdateStrategy:
		p.buildBestEffortParallelUpdatePlan(rolePriorityMap)
	}
}

// unknown & empty & learner & 1/2 followers -> 1/2 followers -> leader
func (p *realUpdatePlan) buildBestEffortParallelUpdatePlan(rolePriorityMap map[string]int) {
	currentVertex, _ := model.FindRootVertex(p.dag)
	preVertex := currentVertex

	// append unknown, empty and learner
	index := 0
	podList := p.pods
	for i, pod := range podList {
		role := pod.Labels[constant.RoleLabelKey]
		if rolePriorityMap[role] <= learnerPriority {
			vertex := &model.ObjectVertex{Obj: &podList[i]}
			p.dag.AddConnect(preVertex, vertex)
			currentVertex = vertex
			index++
		}
	}
	preVertex = currentVertex

	// append 1/2 followers
	podList = podList[index:]
	followerCount := 0
	for _, pod := range podList {
		if rolePriorityMap[pod.Labels[constant.RoleLabelKey]] < leaderPriority {
			followerCount++
		}
	}
	end := followerCount / 2
	for i := 0; i < end; i++ {
		vertex := &model.ObjectVertex{Obj: &podList[i]}
		p.dag.AddConnect(preVertex, vertex)
		currentVertex = vertex
	}
	preVertex = currentVertex

	// append the other 1/2 followers
	podList = podList[end:]
	end = followerCount - end
	for i := 0; i < end; i++ {
		vertex := &model.ObjectVertex{Obj: &podList[i]}
		p.dag.AddConnect(preVertex, vertex)
		currentVertex = vertex
	}
	preVertex = currentVertex

	// append leader
	podList = podList[end:]
	for _, pod := range podList {
		vertex := &model.ObjectVertex{Obj: &pod}
		p.dag.AddConnect(preVertex, vertex)
	}
}

// unknown & empty & leader & followers & learner
func (p *realUpdatePlan) buildParallelUpdatePlan() {
	root, _ := model.FindRootVertex(p.dag)
	for _, pod := range p.pods {
		vertex := &model.ObjectVertex{Obj: &pod}
		p.dag.AddConnect(root, vertex)
	}
}

// unknown -> empty -> learner -> followers(none->readonly->readwrite) -> leader
func (p *realUpdatePlan) buildSerialUpdatePlan() {
	preVertex, _ := model.FindRootVertex(p.dag)
	for _, pod := range p.pods {
		vertex := &model.ObjectVertex{Obj: &pod}
		p.dag.AddConnect(preVertex, vertex)
		preVertex = vertex
	}
}

func (p *realUpdatePlan) execute() ([]*corev1.Pod, error) {
	if err := p.dag.WalkBFS(p.planWalkFunc); err != nil && err != ErrWait && err != ErrStop {
		return nil, err
	}

	return p.podsToBeUpdated, nil
}

func newUpdatePlan(csSet workloads.ConsensusSet, pods []corev1.Pod) updatePlan {
	plan := &realUpdatePlan{
		csSet: csSet,
		pods:  pods,
		dag:   graph.NewDAG(),
	}
	plan.build()
	return plan
}

var _ updatePlan = &realUpdatePlan{}
