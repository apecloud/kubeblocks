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

package plan

import (
	"encoding/json"
	"fmt"
	"math"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/gotemplate"
)

func toJSONObject[T corev1.VolumeSource | corev1.Container | corev1.ContainerPort](obj T) (interface{}, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	var jsonObj any
	if err := json.Unmarshal(b, &jsonObj); err != nil {
		return nil, err
	}

	return jsonObj, nil
}

func fromJSONObject[T any](args interface{}) (*T, error) {
	b, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	var container T
	if err := json.Unmarshal(b, &container); err != nil {
		return nil, err
	}

	return &container, nil
}

func fromJSONArray[T corev1.Container | corev1.Volume](args interface{}) ([]T, error) {
	b, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	var list []T
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, err
	}

	return list, nil
}

const emptyString = ""

// calReverseRebaseBuffer Cal reserved memory for system
func calReverseRebaseBuffer(memSizeMB, cpuNum int64) int64 {
	const (
		rebaseMemorySize        = int64(2048)
		reverseRebaseBufferSize = 285
	)

	// MIN(RDS ins class for mem / 2, 2048)
	r1 := int64(math.Min(float64(memSizeMB>>1), float64(rebaseMemorySize)))
	// MAX(RDS ins class for CPU * 64, RDS ins class for mem / 64)
	r2 := int64(math.Max(float64(cpuNum<<6), float64(memSizeMB>>6)))
	return r1 + r2 + memSizeMB>>6 + reverseRebaseBufferSize
}

// template built-in functions
// calMysqlPoolSizeByResource Cal mysql buffer size
func calMysqlPoolSizeByResource(resource *ResourceDefinition, isShared bool) string {
	const (
		defaultPoolSize      = "128M"
		minBufferSizeMB      = 128
		smallClassMemorySize = int64(1024 * 1024 * 1024)
	)

	if resource == nil || resource.CoreNum == 0 || resource.MemorySize == 0 {
		return defaultPoolSize
	}

	// small instance class
	// mem_size <= 1G or
	// core <= 2
	if resource.MemorySize <= smallClassMemorySize {
		return defaultPoolSize
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

	if totalMemorySize <= minBufferSizeMB {
		return defaultPoolSize
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

// calDBPoolSize for specific engine: mysql
func calDBPoolSize(args interface{}) (string, error) {
	container, err := fromJSONObject[corev1.Container](args)
	if err != nil {
		return "", err
	}
	if len(container.Resources.Limits) == 0 {
		return "", nil
	}
	resource := ResourceDefinition{
		MemorySize: intctrlutil.GetMemorySize(*container),
		CoreNum:    intctrlutil.GetCoreNum(*container),
	}
	return calMysqlPoolSizeByResource(&resource, false), nil

}

// getPodContainerByName gets pod container by name
func getPodContainerByName(args []interface{}, containerName string) (interface{}, error) {
	containers, err := fromJSONArray[corev1.Container](args)
	if err != nil {
		return nil, err
	}
	for _, v := range containers {
		if v.Name == containerName {
			return toJSONObject(v)
		}
	}
	return nil, nil
}

// getVolumeMountPathByName gets volume mount path by name
func getVolumeMountPathByName(args interface{}, volumeName string) (string, error) {
	container, err := fromJSONObject[corev1.Container](args)
	if err != nil {
		return "", err
	}
	for _, v := range container.VolumeMounts {
		if v.Name == volumeName {
			return v.MountPath, nil
		}
	}
	return "", nil
}

// getPVCByName gets pvc by name
func getPVCByName(args []interface{}, volumeName string) (interface{}, error) {
	volumes, err := fromJSONArray[corev1.Volume](args)
	if err != nil {
		return nil, err
	}
	for _, v := range volumes {
		if v.Name == volumeName {
			return toJSONObject(v.VolumeSource)
		}
	}
	return nil, nil
}

// getContainerCPU gets container cpu limit
func getContainerCPU(args interface{}) (int64, error) {
	container, err := fromJSONObject[corev1.Container](args)
	if err != nil {
		return 0, err
	}
	return intctrlutil.GetCoreNum(*container), nil
}

// getContainerMemory gets container memory limit
func getContainerMemory(args interface{}) (int64, error) {
	container, err := fromJSONObject[corev1.Container](args)
	if err != nil {
		return 0, err
	}
	return intctrlutil.GetMemorySize(*container), nil
}

// getContainerRequestMemory gets container memory request
func getContainerRequestMemory(args interface{}) (int64, error) {
	container, err := fromJSONObject[corev1.Container](args)
	if err != nil {
		return 0, err
	}
	return intctrlutil.GetRequestMemorySize(*container), nil
}

// getArgByName get arg by name
func getArgByName(args interface{}, argName string) string {
	// TODO Support parse command args
	return emptyString
}

// getPortByName get port by name
func getPortByName(args interface{}, portName string) (interface{}, error) {
	container, err := fromJSONObject[corev1.Container](args)
	if err != nil {
		return nil, err
	}
	for _, v := range container.Ports {
		if v.Name == portName {
			return toJSONObject(v)
		}
	}

	return nil, nil
}

// getCAFile gets CA file
func getCAFile() string {
	return builder.MountPath + "/" + builder.CAName
}

// getCertFile gets cert file
func getCertFile() string {
	return builder.MountPath + "/" + builder.CertName
}

// getKeyFile gets key file
func getKeyFile() string {
	return builder.MountPath + "/" + builder.KeyName
}

// BuiltInCustomFunctions builds a map of customized functions for KubeBlocks
func BuiltInCustomFunctions(c *configTemplateBuilder, component *component.SynthesizedComponent, localObjs []client.Object) *gotemplate.BuiltInObjectsFunc {
	return &gotemplate.BuiltInObjectsFunc{
		builtInMysqlCalBufferFunctionName:            calDBPoolSize,
		builtInGetVolumeFunctionName:                 getVolumeMountPathByName,
		builtInGetPvcFunctionName:                    getPVCByName,
		builtInGetEnvFunctionName:                    wrapGetEnvByName(c, component, localObjs),
		builtInGetPortFunctionName:                   getPortByName,
		builtInGetArgFunctionName:                    getArgByName,
		builtInGetContainerFunctionName:              getPodContainerByName,
		builtInGetContainerCPUFunctionName:           getContainerCPU,
		builtInGetContainerMemoryFunctionName:        getContainerMemory,
		builtInGetContainerRequestMemoryFunctionName: getContainerRequestMemory,
		builtInGetCAFile:                             getCAFile,
		builtInGetCertFile:                           getCertFile,
		builtInGetKeyFile:                            getKeyFile,
	}

}
