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
