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

package rsmcommon

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("revision util test", func() {
	Context("BuildUpdateRevisions & GetUpdateRevisions", func() {
		It("should work well", func() {
			updateRevisions := map[string]string{
				"pod-0": "revision-0",
				"pod-1": "revision-1",
				"pod-2": "revision-2",
				"pod-3": "revision-3",
				"pod-4": "revision-4",
			}
			revisions, err := BuildUpdateRevisions(updateRevisions)
			Expect(err).Should(BeNil())
			decodeRevisions, err := GetUpdateRevisions(revisions)
			Expect(err).Should(BeNil())
			Expect(decodeRevisions).Should(Equal(updateRevisions))
		})
	})
})
