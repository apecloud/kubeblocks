/*
Copyright ApeCloud Inc.

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

package dbaas

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestGetAvailableContainerPortsNormal(t *testing.T) {
	var containers []corev1.Container

	destPort := 3306
	for p := 1024; p < destPort; p++ {
		containers = append(containers, corev1.Container{Ports: []corev1.ContainerPort{{ContainerPort: int32(p)}}})
	}

	containerPorts := []int32{1025}
	foundPorts, err := getAvailableContainerPorts(containers, containerPorts)
	if err != nil {
		t.Error("expect getAvailableContainerPorts success")
	}
	if len(foundPorts) != 1 || foundPorts[0] != int32(destPort) {
		t.Error("expect getAvailableContainerPorts returns 3306")
	}
}

func TestGetAvailableContainerPortsError(t *testing.T) {
	var containers []corev1.Container

	for p := 1024; p <= 100000; p++ {
		containers = append(containers, corev1.Container{Ports: []corev1.ContainerPort{{ContainerPort: int32(p)}}})
	}

	containerPorts := []int32{1025}
	_, err := getAvailableContainerPorts(containers, containerPorts)
	if err == nil {
		t.Error("expect getAvailableContainerPorts return error")
	}
}
