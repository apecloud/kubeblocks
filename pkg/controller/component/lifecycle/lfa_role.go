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

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

const (
	switchoverCandidateName = "KB_SWITCHOVER_CANDIDATE_NAME"
	switchoverCandidateFQDN = "KB_SWITCHOVER_CANDIDATE_FQDN"
)

type switchover struct {
	namespace   string
	clusterName string
	compName    string
	roles       []appsv1alpha1.ReplicaRole
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
	m, err := hackParameters4Role(ctx, cli, a.namespace, a.clusterName, a.compName, a.roles)
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

////////// hack for legacy Addons //////////
// The container executing this action has access to following variables:
//
// - KB_LEADER_POD_IP: The IP address of the current leader's pod prior to the switchover.
// - KB_LEADER_POD_NAME: The name of the current leader's pod prior to the switchover.
// - KB_LEADER_POD_FQDN: The FQDN of the current leader's pod prior to the switchover.

func hackParameters4Role(ctx context.Context, cli client.Reader, namespace, clusterName, compName string, roles []appsv1alpha1.ReplicaRole) (map[string]string, error) {
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

func leaderRole(roles []appsv1alpha1.ReplicaRole) (string, error) {
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
