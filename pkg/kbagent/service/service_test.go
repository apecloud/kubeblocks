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

package service

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("service", func() {
	Context("new", func() {
		It("empty", func() {
			services, err := New(logr.New(nil), nil, nil, nil)
			Expect(err).Should(BeNil())
			Expect(services).Should(HaveLen(2))
			Expect(services[0]).ShouldNot(BeNil())
			Expect(services[1]).ShouldNot(BeNil())
		})

		It("action", func() {
			actions := []proto.Action{
				{
					Name: "action",
				},
			}
			services, err := New(logr.New(nil), actions, nil, nil)
			Expect(err).Should(BeNil())
			Expect(services).Should(HaveLen(2))
			Expect(services[0]).ShouldNot(BeNil())
			Expect(services[1]).ShouldNot(BeNil())
		})

		It("probe", func() {
			actions := []proto.Action{
				{
					Name: "action",
				},
			}
			probes := []proto.Probe{
				{
					Action: "action",
				},
			}
			services, err := New(logr.New(nil), actions, probes, nil)
			Expect(err).Should(BeNil())
			Expect(services).Should(HaveLen(2))
			Expect(services[0]).ShouldNot(BeNil())
			Expect(services[1]).ShouldNot(BeNil())
		})

		It("probe which has no action", func() {
			actions := []proto.Action{
				{
					Name: "action",
				},
			}
			probes := []proto.Probe{
				{
					Action: "action",
				},
				{
					Action: "not-defined",
				},
			}
			_, err := New(logr.New(nil), actions, probes, nil)
			Expect(err).ShouldNot(BeNil())
		})
	})
})
