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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	dataprotection "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

var _ = Describe("backup builder", func() {
	It("should work well", func() {
		const (
			name = "foo"
			ns   = "default"
		)
		policyName := "policyName"
		backupType := dataprotection.BackupTypeSnapshot
		parent := "parent"
		backup := NewBackupBuilder(ns, name).
			SetBackupPolicyName(policyName).
			SetBackType(backupType).
			SetParentBackupName(parent).
			GetObject()

		Expect(backup.Name).Should(Equal(name))
		Expect(backup.Namespace).Should(Equal(ns))
		Expect(backup.Spec.BackupPolicyName).Should(Equal(policyName))
		Expect(backup.Spec.BackupType).Should(Equal(backupType))
		Expect(backup.Spec.ParentBackupName).Should(Equal(parent))
	})
})
