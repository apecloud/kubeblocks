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
	"fmt"
	"math"

	corev1 "k8s.io/api/core/v1"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// calReverseRebaseBuffer Cal reserved memory for system
func calReverseRebaseBuffer(memSizeMB int64, cpuNum int) int64 {
	const (
		RebaseMemorySize        = int64(2048)
		ReverseRebaseBufferSize = 285
	)

	// MIN(RDS ins class for mem / 2, 2048)
	r1 := int64(math.Min(float64(memSizeMB>>1), float64(RebaseMemorySize)))
	// MAX(RDS ins class for CPU * 64, RDS ins class for mem / 64)
	r2 := int64(math.Max(float64(cpuNum<<6), float64(memSizeMB>>6)))

	return r1 + r2 + memSizeMB>>6 + ReverseRebaseBufferSize
}

// https://help.aliyun.com/document_detail/162326.html?utm_content=g_1000230851&spm=5176.20966629.toubu.3.f2991ddcpxxvD1#title-rey-j7j-4dt
// build-in function
// calMysqlPoolSizeByResource Cal mysql buffer size
func calMysqlPoolSizeByResource(resource *ResourceDefinition, isShared bool) string {
	const (
		DefaultPoolSize      = "128M"
		MinBufferSizeMB      = 128
		SmallClassMemorySize = int64(1024 * 1024 * 1024)
	)

	if resource == nil || resource.CoreNum == 0 || resource.MemorySize == 0 {
		return DefaultPoolSize
	}

	// small instance class
	// mem_size <= 1G or
	// core <= 2
	if resource.MemorySize <= SmallClassMemorySize {
		return DefaultPoolSize
	}

	memSizeMB := resource.MemorySize / 1024 / 1024
	maxBufferSize := int32(memSizeMB * 80 / 100)
	totalMemorySize := memSizeMB

	if !isShared {
		reverseBuffer := calReverseRebaseBuffer(memSizeMB, resource.CoreNum)
		totalMemorySize = memSizeMB - reverseBuffer

		// for small instance class
		if resource.CoreNum <= 2 {
			totalMemorySize -= 128
		}
	}

	if totalMemorySize <= MinBufferSizeMB {
		return DefaultPoolSize
	}

	// (total_memory - reverseBuffer) * 75
	bufferSize := int32(totalMemorySize * 75 / 100)
	if bufferSize > maxBufferSize {
		bufferSize = maxBufferSize
	}

	// https://dev.mysql.com/doc/refman/8.0/en/innodb-parameters.html#sysvar_innodb_buffer_pool_size
	// Buffer size require aligned 128MB or 1G
	var alignedSize int32 = 128
	if bufferSize > 1024 {
		alignedSize = 1024
	}

	bufferSize /= alignedSize
	bufferSize *= alignedSize
	return fmt.Sprintf("%dM", bufferSize)
}

// calDbPoolSize for specific engine: mysql
func calDbPoolSize(container corev1.Container) string {
	if len(container.Resources.Limits) == 0 {
		return ""
	}
	resource := ResourceDefinition{
		MemorySize: intctrlutil.GetMemorySize(container),
		CoreNum:    intctrlutil.GetCoreNum(container),
	}
	return calMysqlPoolSizeByResource(&resource, false)

}

// getPodContainerByName for general built-in
// User overwrite podSpec of Cluster CR, the correctness of access via index cannot be guaranteed
// if User modify name of container, pray users don't
func getPodContainerByName(containers []corev1.Container, containerName string) *corev1.Container {
	for _, v := range containers {
		if v.Name == containerName {
			return &v
		}
	}
	return nil
}

// getVolumeMountPathByName for general built-in
func getVolumeMountPathByName(container *corev1.Container, volumeName string) string {
	for _, v := range container.VolumeMounts {
		if v.Name == volumeName {
			return v.MountPath
		}
	}
	return ""
}

// getPvcByName for general built-in
func getPvcByName(volumes []corev1.Volume, volumeName string) *corev1.VolumeSource {
	for _, v := range volumes {
		if v.Name == volumeName {
			return &v.VolumeSource
		}
	}
	return nil
}

// getEnvByName for general built-in
func getEnvByName(container *corev1.Container, envName string) string {
	for _, v := range container.Env {
		if v.Name == envName {
			return v.Value
		}
	}
	return ""
}

// getArgByName for general built-in
func getArgByName(container *corev1.Container, argName string) string {
	// TODO Support parse command args
	return ""
}

// getPortByName for general built-in
func getPortByName(container *corev1.Container, portName string) *corev1.ContainerPort {
	for _, v := range container.Ports {
		if v.Name == portName {
			return &v
		}
	}

	return nil
}
