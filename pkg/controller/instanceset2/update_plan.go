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

package instanceset2

import (
	"errors"
	"math"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type updatePlan interface {
	// Execute executes the plan
	// return error when any error occurred
	// return instances to be updated,
	// nil slice means no instance need to be updated
	Execute() ([]*workloads.Instance, error)
}

type realUpdatePlan struct {
	its                  workloads.InstanceSet
	instances            []workloads.Instance
	dag                  *graph.DAG
	instancesToBeUpdated []*workloads.Instance
	isInstanceUpdated    func(*workloads.InstanceSet, *workloads.Instance) (bool, error)
}

var _ updatePlan = &realUpdatePlan{}

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
	inst, ok := v.Obj.(*workloads.Instance)
	if !ok {
		return ErrContinue
	}

	// if DeletionTimestamp is not nil, it is terminating.
	if !inst.DeletionTimestamp.IsZero() {
		return ErrWait
	}

	var (
		isInstanceUpdated bool
		err               error
	)
	if p.isInstanceUpdated == nil {
		isInstanceUpdated, err = p.defaultIsPodUpdatedFunc(&p.its, inst)
	} else {
		isInstanceUpdated, err = p.isInstanceUpdated(&p.its, inst)
	}
	if err != nil {
		return err
	}
	// if pod is the latest version, we do nothing
	if isInstanceUpdated {
		if !intctrlutil.IsInstanceReady(inst) {
			return ErrWait
		}
		isRoleful := func() bool { return len(p.its.Spec.Roles) > 0 }()
		if isRoleful && !intctrlutil.IsInstanceReadyWithRole(inst) {
			// If none of the replicas are ready, and the system is employing a serial update strategy, it may end up
			// waiting indefinitely. To prevent this, we choose to bypass the role label check if there are no replicas
			// that have had their role probed.
			//
			// This change may lead to false alarms, as when all replicas are temporarily unavailable for some reason,
			// the system will update them without waiting for their roles to be elected and probed. This cloud
			// potentially hide some uncertain risks.
			memberUpdateStrategy := getMemberUpdateStrategy(&p.its)
			serialUpdate := memberUpdateStrategy == workloads.SerialUpdateStrategy
			hasRoleProbed := len(p.its.Status.MembersStatus) > 0
			if !serialUpdate || hasRoleProbed {
				return ErrWait
			}
		}
		return ErrContinue
	}

	// delete the instance to trigger associate workload to re-create it
	p.instancesToBeUpdated = append(p.instancesToBeUpdated, inst)
	return ErrStop
}

func (p *realUpdatePlan) defaultIsPodUpdatedFunc(its *workloads.InstanceSet, inst *workloads.Instance) (bool, error) {
	// TODO: ???
	// return intctrlutil.GetPodRevision(pod) == its.Status.UpdateRevision, nil
	return true, nil
}

// build builds the update plan based on memberUpdateStrategy
func (p *realUpdatePlan) build() {
	// make a root vertex with nil Obj
	root := &model.ObjectVertex{}
	p.dag.AddVertex(root)

	memberUpdateStrategy := getMemberUpdateStrategy(&p.its)

	rolePriorityMap := ComposeRolePriorityMap(p.its.Spec.Roles)
	sortInstances(p.instances, rolePriorityMap, false)

	// generate plan by memberUpdateStrategy
	switch memberUpdateStrategy {
	case workloads.SerialUpdateStrategy:
		p.buildSerialUpdatePlan()
	case workloads.ParallelUpdateStrategy:
		p.buildParallelUpdatePlan()
	case workloads.BestEffortParallelUpdateStrategy:
		p.buildBestEffortParallelUpdatePlan(rolePriorityMap)
	}
}

// unknown & empty & roles that do not participate in quorum & 1/2 followers -> 1/2 followers -> leader
func (p *realUpdatePlan) buildBestEffortParallelUpdatePlan(rolePriorityMap map[string]int) {
	currentVertex, _ := model.FindRootVertex(p.dag)
	preVertex := currentVertex

	quorumPriority := math.MaxInt32
	leaderPriority := 0
	for _, role := range p.its.Spec.Roles {
		if rolePriorityMap[role.Name] > leaderPriority {
			leaderPriority = rolePriorityMap[role.Name]
		}
		if role.ParticipatesInQuorum && quorumPriority > rolePriorityMap[role.Name] {
			quorumPriority = rolePriorityMap[role.Name]
		}
	}

	// append unknown, empty and roles that do not participate in quorum
	index := 0
	instanceList := p.instances
	for i, inst := range instanceList {
		roleName := getInstanceRoleName(&inst)
		if rolePriorityMap[roleName] < quorumPriority {
			vertex := &model.ObjectVertex{Obj: &instanceList[i]}
			p.dag.AddConnect(preVertex, vertex)
			currentVertex = vertex
			index++
		}
	}
	preVertex = currentVertex

	// append 1/2 followers
	instanceList = instanceList[index:]
	followerCount := 0
	for _, inst := range instanceList {
		roleName := getInstanceRoleName(&inst)
		if rolePriorityMap[roleName] < leaderPriority {
			followerCount++
		}
	}
	end := followerCount / 2
	for i := 0; i < end; i++ {
		vertex := &model.ObjectVertex{Obj: &instanceList[i]}
		p.dag.AddConnect(preVertex, vertex)
		currentVertex = vertex
	}
	preVertex = currentVertex

	// append the other 1/2 followers
	instanceList = instanceList[end:]
	end = followerCount - end
	for i := 0; i < end; i++ {
		vertex := &model.ObjectVertex{Obj: &instanceList[i]}
		p.dag.AddConnect(preVertex, vertex)
		currentVertex = vertex
	}
	preVertex = currentVertex

	// append leader
	instanceList = instanceList[end:]
	end = len(instanceList)
	for i := 0; i < end; i++ {
		vertex := &model.ObjectVertex{Obj: &instanceList[i]}
		p.dag.AddConnect(preVertex, vertex)
	}
}

// unknown & empty & all roles
func (p *realUpdatePlan) buildParallelUpdatePlan() {
	root, _ := model.FindRootVertex(p.dag)
	for i := range p.instances {
		vertex := &model.ObjectVertex{Obj: &p.instances[i]}
		p.dag.AddConnect(root, vertex)
	}
}

// update according to role update priority
func (p *realUpdatePlan) buildSerialUpdatePlan() {
	preVertex, _ := model.FindRootVertex(p.dag)
	for i := range p.instances {
		vertex := &model.ObjectVertex{Obj: &p.instances[i]}
		p.dag.AddConnect(preVertex, vertex)
		preVertex = vertex
	}
}

func (p *realUpdatePlan) Execute() ([]*workloads.Instance, error) {
	p.build()
	if err := p.dag.WalkBFS(p.planWalkFunc); err != ErrContinue && err != ErrWait && err != ErrStop {
		return nil, err
	}
	return p.instancesToBeUpdated, nil
}

func NewUpdatePlan(its workloads.InstanceSet, instances []*workloads.Instance,
	isInstanceUpdated func(*workloads.InstanceSet, *workloads.Instance) (bool, error)) updatePlan {
	var instanceList []workloads.Instance
	for _, inst := range instances {
		instanceList = append(instanceList, *inst)
	}
	return &realUpdatePlan{
		its:               its,
		instances:         instanceList,
		dag:               graph.NewDAG(),
		isInstanceUpdated: isInstanceUpdated,
	}
}
