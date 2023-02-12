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

package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestGetAvailableContainerPorts(t *testing.T) {
	var containers []corev1.Container

	tests := []struct {
		inputPort  int32
		outputPort int32
	}{{
		inputPort:  80, // 80 is a privileged port
		outputPort: minAvailPort,
	}, {
		inputPort:  65536, // 65536 is an invalid port
		outputPort: minAvailPort,
	}, {
		inputPort:  3306, // 3306 is a qualified port
		outputPort: 3306,
	}}

	for _, test := range tests {
		containerPorts := []int32{test.inputPort}
		foundPorts, err := getAvailableContainerPorts(containers, containerPorts)
		if err != nil {
			t.Error("expect getAvailableContainerPorts success")
		}
		if len(foundPorts) != 1 || foundPorts[0] != test.outputPort {
			t.Error("expect getAvailableContainerPorts returns", test.outputPort)
		}
	}
}

func TestGetAvailableContainerPortsPartlyOccupied(t *testing.T) {
	var containers []corev1.Container

	destPort := 3306
	for p := minAvailPort; p < destPort; p++ {
		containers = append(containers, corev1.Container{Ports: []corev1.ContainerPort{{ContainerPort: int32(p)}}})
	}

	containerPorts := []int32{minAvailPort + 1}
	foundPorts, err := getAvailableContainerPorts(containers, containerPorts)
	if err != nil {
		t.Error("expect getAvailableContainerPorts success")
	}
	if len(foundPorts) != 1 || foundPorts[0] != int32(destPort) {
		t.Error("expect getAvailableContainerPorts returns 3306")
	}
}

func TestGetAvailableContainerPortsFullyOccupied(t *testing.T) {
	var containers []corev1.Container

	for p := minAvailPort; p <= maxAvailPort; p++ {
		containers = append(containers, corev1.Container{Ports: []corev1.ContainerPort{{ContainerPort: int32(p)}}})
	}

	containerPorts := []int32{3306}
	_, err := getAvailableContainerPorts(containers, containerPorts)
	if err == nil {
		t.Error("expect getAvailableContainerPorts return error")
	}
}
