/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package service

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("reconfigure", func() {
	Context("reconfigure", func() {
		It("not a reconfigure request", func() {
			services, err := New(logr.New(nil), nil, nil, nil)
			Expect(err).Should(BeNil())
			Expect(services).Should(HaveLen(3))
			Expect(services[0]).ShouldNot(BeNil())
			Expect(services[1]).ShouldNot(BeNil())
			Expect(services[2]).ShouldNot(BeNil())
		})

		It("bad request", func() {
			actions := []proto.Action{
				{
					Name: "action",
				},
			}
			services, err := New(logr.New(nil), actions, nil, nil)
			Expect(err).Should(BeNil())
			Expect(services).Should(HaveLen(3))
			Expect(services[0]).ShouldNot(BeNil())
			Expect(services[1]).ShouldNot(BeNil())
			Expect(services[2]).ShouldNot(BeNil())
		})

		It("precondition failed", func() {
			actions := []proto.Action{
				{
					Name: "action",
				},
			}
			services, err := New(logr.New(nil), actions, nil, nil)
			Expect(err).Should(BeNil())
			Expect(services).Should(HaveLen(3))
			Expect(services[0]).ShouldNot(BeNil())
			Expect(services[1]).ShouldNot(BeNil())
			Expect(services[2]).ShouldNot(BeNil())
		})

		It("ok", func() {
			actions := []proto.Action{
				{
					Name: "action",
				},
			}
			services, err := New(logr.New(nil), actions, nil, nil)
			Expect(err).Should(BeNil())
			Expect(services).Should(HaveLen(3))
			Expect(services[0]).ShouldNot(BeNil())
			Expect(services[1]).ShouldNot(BeNil())
			Expect(services[2]).ShouldNot(BeNil())
		})
	})
})
