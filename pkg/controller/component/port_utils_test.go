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

package component

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
