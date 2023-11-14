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

type SecretBuilder struct {
	BaseBuilder[corev1.Secret, *corev1.Secret, SecretBuilder]
}

func NewSecretBuilder(namespace, name string) *SecretBuilder {
	builder := &SecretBuilder{}
	builder.init(namespace, name, &corev1.Secret{}, builder)
	return builder
}

func (builder *SecretBuilder) SetImmutable(immutable bool) *SecretBuilder {
	builder.get().Immutable = &immutable
	return builder
}

func (builder *SecretBuilder) PutStringData(key, value string) *SecretBuilder {
	data := builder.get().StringData
	if data == nil {
		data = make(map[string]string, 1)
	}
	data[key] = value
	builder.get().StringData = data
	return builder
}

func (builder *SecretBuilder) SetStringData(data map[string]string) *SecretBuilder {
	builder.get().StringData = data
	return builder
}

func (builder *SecretBuilder) PutData(key string, value []byte) *SecretBuilder {
	data := builder.get().Data
	if data == nil {
		data = make(map[string][]byte, 1)
	}
	data[key] = value
	builder.get().Data = data
	return builder
}

func (builder *SecretBuilder) SetData(binaryData map[string][]byte) *SecretBuilder {
	builder.get().Data = binaryData
	return builder
}

func (builder *SecretBuilder) SetSecretType(secretType corev1.SecretType) *SecretBuilder {
	builder.get().Type = secretType
	return builder
}
