/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

type RoleBuilder struct {
	BaseBuilder[rbacv1.Role, *rbacv1.Role, RoleBuilder]
}

func NewRoleBuilder(namespace, name string) *RoleBuilder {
	builder := &RoleBuilder{}
	builder.init(namespace, name, &rbacv1.Role{}, builder)
	return builder
}

func (builder *RoleBuilder) AddPolicyRules(rules []rbacv1.PolicyRule) *RoleBuilder {
	builder.get().Rules = append(builder.get().Rules, rules...)
	return builder
}
