/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
