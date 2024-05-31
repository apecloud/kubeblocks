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

package models

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Models Suite")
}

var _ = Describe("IsLikelyPrimaryRole", func() {
	It("should return true for primary role", func() {
		Expect(IsLikelyPrimaryRole(PRIMARY)).To(BeTrue())
	})

	It("should return true for master role", func() {
		Expect(IsLikelyPrimaryRole(MASTER)).To(BeTrue())
	})

	It("should return true for leader role", func() {
		Expect(IsLikelyPrimaryRole(LEADER)).To(BeTrue())
	})

	It("should return false for secondary role", func() {
		Expect(IsLikelyPrimaryRole(SECONDARY)).To(BeFalse())
	})

	It("should return false for slave role", func() {
		Expect(IsLikelyPrimaryRole(SLAVE)).To(BeFalse())
	})

	It("should return false for follower role", func() {
		Expect(IsLikelyPrimaryRole(FOLLOWER)).To(BeFalse())
	})

	It("should return false for learner role", func() {
		Expect(IsLikelyPrimaryRole(LEARNER)).To(BeFalse())
	})

	It("should return false for candidate role", func() {
		Expect(IsLikelyPrimaryRole(CANDIDATE)).To(BeFalse())
	})
})
