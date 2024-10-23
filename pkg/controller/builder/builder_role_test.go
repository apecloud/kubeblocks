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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	rbacv1 "k8s.io/api/rbac/v1"
)

var _ = Describe("role builder", func() {
	It("should build a role", func() {
		const (
			name = "foo"
			ns   = "default"
		)

		rules := []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pod"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pod/status"},
				Verbs:     []string{"get"},
			},
		}

		role := NewRoleBuilder(ns, name).
			AddPolicyRules(rules).
			GetObject()

		Expect(role.Name).Should(Equal(name))
		Expect(role.Namespace).Should(Equal(ns))
		Expect(role.Rules).Should(HaveLen(2))
		Expect(role.Rules[0]).Should(Equal(rules[0]))
	})
})
