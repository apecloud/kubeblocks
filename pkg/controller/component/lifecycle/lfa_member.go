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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

const (
	switchoverCandidateName = "KB_SWITCHOVER_CANDIDATE_NAME"
	switchoverCandidateFQDN = "KB_SWITCHOVER_CANDIDATE_FQDN"
	switchoverCurrentName   = "KB_SWITCHOVER_CURRENT_NAME"
	switchoverCurrentFQDN   = "KB_SWITCHOVER_CURRENT_FQDN"
	switchoverRole          = "KB_SWITCHOVER_ROLE"
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
	namespace    string
	clusterName  string
	compName     string
	role         string // the role that will be transferred to another replica.
	currentPod   string
	candidatePod string
}

var _ lifecycleAction = &switchover{}

func (a *switchover) name() string {
	return "switchover"
}

func (a *switchover) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	// refer to ComponentLifecycleActions.Switchover's documentation for explanation of each variable.
	m := make(map[string]string)
	if len(a.candidatePod) > 0 {
		compName := constant.GenerateClusterComponentName(a.clusterName, a.compName)
		m[switchoverCandidateName] = a.candidatePod
		m[switchoverCandidateFQDN] = component.PodFQDN(a.namespace, compName, a.candidatePod)
	}
	m[switchoverCurrentName] = a.currentPod
	m[switchoverCurrentFQDN] = component.PodFQDN(a.namespace, a.compName, a.currentPod)
	m[switchoverRole] = a.role
	return m, nil
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
