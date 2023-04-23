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

package builder

import corev1 "k8s.io/api/core/v1"

type ConfigMapBuilder struct {
	BaseBuilder[corev1.ConfigMap, *corev1.ConfigMap, ConfigMapBuilder]
}

func NewConfigMapBuilder(namespace, name string) *ConfigMapBuilder {
	builder := &ConfigMapBuilder{}
	builder.init(namespace, name, &corev1.ConfigMap{}, builder)
	return builder
}

func (builder *ConfigMapBuilder) SetImmutable(immutable bool) *ConfigMapBuilder {
	builder.get().Immutable = &immutable
	return builder
}

func (builder *ConfigMapBuilder) PutData(key, value string) *ConfigMapBuilder {
	data := builder.get().Data
	if data == nil {
		data = make(map[string]string, 1)
	}
	data[key] = value
	return builder
}

func (builder *ConfigMapBuilder) SetData(data map[string]string) *ConfigMapBuilder {
	builder.get().Data = data
	return builder
}

func (builder *ConfigMapBuilder) PutBinaryData(key string, value []byte) *ConfigMapBuilder {
	data := builder.get().BinaryData
	if data == nil {
		data = make(map[string][]byte, 1)
	}
	data[key] = value
	return builder
}

func (builder *ConfigMapBuilder) SetBinaryData(binaryData map[string][]byte) *ConfigMapBuilder {
	builder.get().BinaryData = binaryData
	return builder
}
