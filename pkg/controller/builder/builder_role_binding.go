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

import (
	rbacv1 "k8s.io/api/rbac/v1"
)

type RoleBindingBuilder struct {
	BaseBuilder[rbacv1.RoleBinding, *rbacv1.RoleBinding, RoleBindingBuilder]
}

func NewRoleBindingBuilder(namespace, name string) *RoleBindingBuilder {
	builder := &RoleBindingBuilder{}
	builder.init(namespace, name, &rbacv1.RoleBinding{}, builder)
	return builder
}

func (builder *RoleBindingBuilder) SetRoleRef(roleRef rbacv1.RoleRef) *RoleBindingBuilder {
	builder.get().RoleRef = roleRef
	return builder
}

func (builder *RoleBindingBuilder) AddSubjects(subjects ...rbacv1.Subject) *RoleBindingBuilder {
	builder.get().Subjects = append(builder.get().Subjects, subjects...)
	return builder
}
