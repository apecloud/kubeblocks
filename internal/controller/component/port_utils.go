/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package component

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const minAvailPort = 1024
const maxAvailPort = 65535

func getAllContainerPorts(containers []corev1.Container) (map[int32]bool, error) {
	set := map[int32]bool{}
	for _, container := range containers {
		for _, v := range container.Ports {
			_, ok := set[v.ContainerPort]
			if ok {
				return nil, fmt.Errorf("containerPorts conflict: [%+v]", v.ContainerPort)
			}
			set[v.ContainerPort] = true
		}
	}
	return set, nil
}

// get available container ports, increased by one if conflict with exist ports
// util no conflicts.
func getAvailableContainerPorts(containers []corev1.Container, containerPorts []int32) ([]int32, error) {
	set, err := getAllContainerPorts(containers)
	if err != nil {
		return nil, err
	}

	iterAvailPort := func(p int32) (int32, error) {
		// The TCP/IP port numbers below 1024 are privileged ports, which are special
		// in that normal users are not allowed to run servers on them.
		if p < minAvailPort || p > maxAvailPort {
			p = minAvailPort
		}
		sentinel := p
		for {
			if _, ok := set[p]; !ok {
				set[p] = true
				return p, nil
			}
			p++
			if p == sentinel {
				return -1, errors.New("no available port for container")
			}
			if p > maxAvailPort {
				p = minAvailPort
			}
		}
	}

	for i, p := range containerPorts {
		if containerPorts[i], err = iterAvailPort(p); err != nil {
			return []int32{}, err
		}
	}
	return containerPorts, nil
}
