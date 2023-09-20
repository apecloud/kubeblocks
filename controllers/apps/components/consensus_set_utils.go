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

package components

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch;delete

type consensusRole string

type consensusMemberExt struct {
	name          string
	consensusRole consensusRole
	accessMode    appsv1alpha1.AccessMode
	podName       string
}

const (
	roleLeader   consensusRole = "Leader"
	roleFollower consensusRole = "Follower"
	roleLearner  consensusRole = "Learner"
)

const (
	leaderPriority            = 1 << 5
	followerReadWritePriority = 1 << 4
	followerReadonlyPriority  = 1 << 3
	followerNonePriority      = 1 << 2
	learnerPriority           = 1 << 1
	emptyConsensusPriority    = 1 << 0
	// unknownPriority           = 0
)

// ComposeRolePriorityMap generates a priority map based on roles.
func ComposeRolePriorityMap(consensusSpec *appsv1alpha1.ConsensusSetSpec) map[string]int {
	if consensusSpec == nil {
		consensusSpec = appsv1alpha1.NewConsensusSetSpec()
	}
	rolePriorityMap := make(map[string]int, 0)
	rolePriorityMap[""] = emptyConsensusPriority
	rolePriorityMap[consensusSpec.Leader.Name] = leaderPriority
	if consensusSpec.Learner != nil {
		rolePriorityMap[consensusSpec.Learner.Name] = learnerPriority
	}
	for _, follower := range consensusSpec.Followers {
		switch follower.AccessMode {
		case appsv1alpha1.None:
			rolePriorityMap[follower.Name] = followerNonePriority
		case appsv1alpha1.Readonly:
			rolePriorityMap[follower.Name] = followerReadonlyPriority
		case appsv1alpha1.ReadWrite:
			rolePriorityMap[follower.Name] = followerReadWritePriority
		}
	}
	return rolePriorityMap
}

// UpdateConsensusSetRoleLabel updates pod role label when internal container role changed
func UpdateConsensusSetRoleLabel(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	event *corev1.Event,
	componentDef *appsv1alpha1.ClusterComponentDefinition,
	pod *corev1.Pod, role string) error {
	if componentDef == nil {
		return nil
	}
	return updateConsensusSetRoleLabel(cli, reqCtx, event, componentDef.ConsensusSpec, pod, role)
}

func updateConsensusSetRoleLabel(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	event *corev1.Event,
	consensusSpec *appsv1alpha1.ConsensusSetSpec,
	pod *corev1.Pod, role string) error {
	ctx := reqCtx.Ctx
	roleMap := composeConsensusRoleMap(consensusSpec)
	// role not defined in CR, ignore it
	if _, ok := roleMap[role]; !ok {
		return nil
	}

	// update pod role label
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Labels[constant.RoleLabelKey] = role
	pod.Labels[constant.ConsensusSetAccessModeLabelKey] = string(roleMap[role].accessMode)
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations[constant.LastRoleSnapshotVersionAnnotationKey] = event.EventTime.Time.Format(time.RFC3339Nano)
	return cli.Patch(ctx, pod, patch)
}

func putConsensusMemberExt(roleMap map[string]consensusMemberExt, name string, role consensusRole, accessMode appsv1alpha1.AccessMode) {
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

func composeConsensusRoleMap(consensusSpec *appsv1alpha1.ConsensusSetSpec) map[string]consensusMemberExt {
	roleMap := make(map[string]consensusMemberExt, 0)
	putConsensusMemberExt(roleMap,
		consensusSpec.Leader.Name,
		roleLeader,
		consensusSpec.Leader.AccessMode)

	for _, follower := range consensusSpec.Followers {
		putConsensusMemberExt(roleMap,
			follower.Name,
			roleFollower,
			follower.AccessMode)
	}

	if consensusSpec.Learner != nil {
		putConsensusMemberExt(roleMap,
			consensusSpec.Learner.Name,
			roleLearner,
			consensusSpec.Learner.AccessMode)
	}

	return roleMap
}
