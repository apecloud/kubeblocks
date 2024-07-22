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

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type switchover struct{}

var _ lifecycleAction = &switchover{}

func (a *switchover) name() string {
	return "switchover"
}

func (a *switchover) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	// - KB_SWITCHOVER_CANDIDATE_NAME: The name of the pod for the new leader candidate, which may not be specified (empty).
	// - KB_SWITCHOVER_CANDIDATE_FQDN: The FQDN of the new leader candidate's pod, which may not be specified (empty).
	// - KB_LEADER_POD_IP: The IP address of the current leader's pod prior to the switchover.
	// - KB_LEADER_POD_NAME: The name of the current leader's pod prior to the switchover.
	// - KB_LEADER_POD_FQDN: The FQDN of the current leader's pod prior to the switchover.
	m := make(map[string]string)
	m["KB_SWITCHOVER_CANDIDATE_NAME"] = ""
	m["KB_SWITCHOVER_CANDIDATE_FQDN"] = ""
	m["KB_LEADER_POD_IP"] = ""
	m["KB_LEADER_POD_NAME"] = ""
	m["KB_LEADER_POD_FQDN"] = ""
	return m, nil
}
