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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("role binding builder", func() {
	It("should work well", func() {
		const (
			name = "foo"
			ns   = "default"
		)
		roleRef := rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     constant.RBACRoleName,
		}
		subject := rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Namespace: ns,
			Name:      fmt.Sprintf("kb-%s", name),
		}
		roleBinding := NewRoleBindingBuilder(ns, name).
			SetRoleRef(roleRef).
			AddSubjects(subject).
			GetObject()

		Expect(roleBinding.Name).Should(Equal(name))
		Expect(roleBinding.Namespace).Should(Equal(ns))
		Expect(roleBinding.RoleRef).Should(Equal(roleRef))
		Expect(roleBinding.Subjects).Should(HaveLen(1))
		Expect(roleBinding.Subjects[0]).Should(Equal(subject))
	})
})
