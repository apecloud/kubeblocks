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

type PVCBuilder struct {
	BaseBuilder[corev1.PersistentVolumeClaim, *corev1.PersistentVolumeClaim, PVCBuilder]
}

func NewPVCBuilder(namespace, name string) *PVCBuilder {
	builder := &PVCBuilder{}
	builder.init(namespace, name, &corev1.PersistentVolumeClaim{}, builder)
	return builder
}

func (builder *PVCBuilder) SetResources(resources corev1.ResourceRequirements) *PVCBuilder {
	builder.get().Spec.Resources = resources
	return builder
}

func (builder *PVCBuilder) SetAccessModes(accessModes []corev1.PersistentVolumeAccessMode) *PVCBuilder {
	builder.get().Spec.AccessModes = accessModes
	return builder
}

func (builder *PVCBuilder) SetStorageClass(sc string) *PVCBuilder {
	builder.get().Spec.StorageClassName = &sc
	return builder
}

func (builder *PVCBuilder) SetDataSource(dataSource corev1.TypedLocalObjectReference) *PVCBuilder {
	builder.get().Spec.DataSource = &dataSource
	return builder
}
