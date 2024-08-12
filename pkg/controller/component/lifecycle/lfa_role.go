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
	"errors"
	"fmt"
	"strings"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/operations"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	switchoverCandidateName = "KB_SWITCHOVER_CANDIDATE_NAME"
	switchoverCandidateFQDN = "KB_SWITCHOVER_CANDIDATE_FQDN"
	leaderPodIP             = "KB_LEADER_POD_IP"
	leaderPodName           = "KB_LEADER_POD_NAME"
	leaderPodFQDN           = "KB_LEADER_POD_FQDN"
)

type switchover struct {
	synthesizedComp *component.SynthesizedComponent
	switchover      *appsv1alpha1.Switchover
}

var _ lifecycleAction = &switchover{}

func (a *switchover) name() string {
	return "switchover"
}

func (a *switchover) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	return parameters4SwitchOver(ctx, cli, a.synthesizedComp, a.switchover)
}

func parameters4SwitchOver(ctx context.Context, cli client.Reader, synthesizedComp *component.SynthesizedComponent, switchover *appsv1alpha1.Switchover) (map[string]string, error) {
	pod, err := getServiceableNWritablePod(ctx, cli, synthesizedComp)
	if err != nil {
		return nil, err
	}
	if pod == nil {
		return nil, errors.New("serviceable and writable pod not found")
	}
	svcName := strings.Join([]string{synthesizedComp.ClusterName, synthesizedComp.Name, "headless"}, "-")
	if switchover == nil {
		return nil, nil
	}
	if switchover.InstanceName == operations.KBSwitchoverCandidateInstanceForAnyPod {
		return nil, nil
	}
	// - KB_SWITCHOVER_CANDIDATE_NAME: The name of the pod for the new leader candidate, which may not be specified (empty).
	// - KB_SWITCHOVER_CANDIDATE_FQDN: The FQDN of the new leader candidate's pod, which may not be specified (empty).
	// - KB_LEADER_POD_IP: The IP address of the current leader's pod prior to the switchover.
	// - KB_LEADER_POD_NAME: The name of the current leader's pod prior to the switchover.
	// - KB_LEADER_POD_FQDN: The FQDN of the current leader's pod prior to the switchover.
	m := make(map[string]string)
	m[switchoverCandidateName] = switchover.InstanceName
	m[switchoverCandidateFQDN] = fmt.Sprintf("%s.%s", switchover.InstanceName, svcName)
	m[leaderPodIP] = pod.Status.PodIP
	m[leaderPodName] = pod.Name
	m[leaderPodFQDN] = fmt.Sprintf("%s.%s", pod.Name, svcName)
	return m, nil
}
