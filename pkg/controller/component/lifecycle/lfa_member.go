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

package lifecycle

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

const (
	switchoverCandidateName = "KB_SWITCHOVER_CANDIDATE_NAME"
	switchoverCandidateFQDN = "KB_SWITCHOVER_CANDIDATE_FQDN"
	joinMemberPodFQDNVar    = "KB_JOIN_MEMBER_POD_FQDN"
	joinMemberPodNameVar    = "KB_JOIN_MEMBER_POD_NAME"
	leaveMemberPodFQDNVar   = "KB_LEAVE_MEMBER_POD_FQDN"
	leaveMemberPodNameVar   = "KB_LEAVE_MEMBER_POD_NAME"
)

type roleProbe struct{}

var _ lifecycleAction = &roleProbe{}

func (a *roleProbe) name() string {
	return "roleProbe"
}

func (a *roleProbe) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	return nil, nil
}

type switchover struct {
	namespace   string
	clusterName string
	compName    string
	roles       []appsv1.ReplicaRole
	candidate   string
}

var _ lifecycleAction = &switchover{}

func (a *switchover) name() string {
	return "switchover"
}

func (a *switchover) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	// The container executing this action has access to following variables:
	//
	// - KB_SWITCHOVER_CANDIDATE_NAME: The name of the pod for the new leader candidate, which may not be specified (empty).
	// - KB_SWITCHOVER_CANDIDATE_FQDN: The FQDN of the new leader candidate's pod, which may not be specified (empty).
	m, err := hackParameters4Switchover(ctx, cli, a.namespace, a.clusterName, a.compName, a.roles)
	if err != nil {
		return nil, err
	}
	if len(a.candidate) > 0 {
		compName := constant.GenerateClusterComponentName(a.clusterName, a.compName)
		m[switchoverCandidateName] = a.candidate
		m[switchoverCandidateFQDN] = component.PodFQDN(a.namespace, compName, a.candidate)
	}
	return m, nil
}

type MemberJoinStatus string

const (
	MemberJoinProcessing MemberJoinStatus = "processing"
	MemberJoinCompleted  MemberJoinStatus = "completed"
)

func (s MemberJoinStatus) String() string {
	return string(s)
}

type memberJoin struct {
	namespace   string
	clusterName string
	compName    string
	pod         *corev1.Pod
}

var _ lifecycleAction = &memberJoin{}

func (a *memberJoin) name() string {
	return "memberJoin"
}

func (a *memberJoin) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	// The container executing this action has access to following variables:
	//
	// - KB_JOIN_MEMBER_POD_FQDN: The pod FQDN of the replica being added to the group.
	// - KB_JOIN_MEMBER_POD_NAME: The pod name of the replica being added to the group.
	compName := constant.GenerateClusterComponentName(a.clusterName, a.compName)
	return map[string]string{
		joinMemberPodFQDNVar: component.PodFQDN(a.namespace, compName, a.pod.Name),
		joinMemberPodNameVar: a.pod.Name,
	}, nil
}

type memberLeave struct {
	namespace   string
	clusterName string
	compName    string
	pod         *corev1.Pod
}

var _ lifecycleAction = &memberLeave{}

func (a *memberLeave) name() string {
	return "memberLeave"
}

func (a *memberLeave) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	// The container executing this action has access to following variables:
	//
	// - KB_LEAVE_MEMBER_POD_FQDN: The pod name of the replica being removed from the group.
	// - KB_LEAVE_MEMBER_POD_NAME: The pod name of the replica being removed from the group.
	compName := constant.GenerateClusterComponentName(a.clusterName, a.compName)
	return map[string]string{
		leaveMemberPodFQDNVar: component.PodFQDN(a.namespace, compName, a.pod.Name),
		leaveMemberPodNameVar: a.pod.Name,
	}, nil
}

////////// hack for legacy Addons //////////
// The container executing this action has access to following variables:
//
// - KB_LEADER_POD_IP: The IP address of the current leader's pod prior to the switchover.
// - KB_LEADER_POD_NAME: The name of the current leader's pod prior to the switchover.
// - KB_LEADER_POD_FQDN: The FQDN of the current leader's pod prior to the switchover.

func hackParameters4Switchover(ctx context.Context, cli client.Reader, namespace, clusterName, compName string, roles []appsv1.ReplicaRole) (map[string]string, error) {
	const (
		leaderPodName = "KB_LEADER_POD_NAME"
		leaderPodFQDN = "KB_LEADER_POD_FQDN"
		leaderPodIP   = "KB_LEADER_POD_IP"
	)

	role, err := leaderRole(roles)
	if err != nil {
		return nil, err
	}

	pods, err := component.ListOwnedPodsWithRole(ctx, cli, namespace, clusterName, compName, role)
	if err != nil {
		return nil, err
	}
	if len(pods) == 0 {
		return nil, fmt.Errorf("has no pod with the leader role %s", role)
	}
	if len(pods) > 1 {
		return nil, fmt.Errorf("more than one pod found as leader: %d, role: %s", len(pods), role)
	}

	pod := pods[0]
	return map[string]string{
		leaderPodName: pod.Name,
		leaderPodFQDN: component.PodFQDN(namespace, constant.GenerateClusterComponentName(clusterName, compName), pod.Name),
		leaderPodIP:   pod.Status.PodIP,
	}, nil
}

func leaderRole(roles []appsv1.ReplicaRole) (string, error) {
	targetRole := ""
	for _, role := range roles {
		if role.Serviceable && role.Writable {
			if targetRole != "" {
				return "", fmt.Errorf("more than one role defined as leader: %s,%s", targetRole, role.Name)
			}
			targetRole = role.Name
		}
	}
	if targetRole == "" {
		return "", fmt.Errorf("%s", "has no appropriate role defined as leader")
	}
	return targetRole, nil
}
