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

	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

const (
	joinMemberPodFQDNVar  = "KB_JOIN_MEMBER_POD_FQDN"
	joinMemberPodNameVar  = "KB_JOIN_MEMBER_POD_NAME"
	leaveMemberPodFQDNVar = "KB_LEAVE_MEMBER_POD_FQDN"
	leaveMemberPodNameVar = "KB_LEAVE_MEMBER_POD_NAME"
)

type memberJoin struct {
	synthesizedComp *component.SynthesizedComponent
	pod             *corev1.Pod
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
	return map[string]string{
		joinMemberPodFQDNVar: component.PodFQDN(a.synthesizedComp.Namespace, a.synthesizedComp.FullCompName, a.pod.Name),
		joinMemberPodNameVar: a.pod.Name,
	}, nil
}

type memberLeave struct {
	synthesizedComp *component.SynthesizedComponent
	pod             *corev1.Pod
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
	return map[string]string{
		leaveMemberPodFQDNVar: component.PodFQDN(a.synthesizedComp.Namespace, a.synthesizedComp.FullCompName, a.pod.Name),
		leaveMemberPodNameVar: a.pod.Name,
	}, nil
}
