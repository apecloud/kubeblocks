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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("probe", func() {
	Context("probe", func() {
		var (
			actions = []proto.Action{
				{
					Name: "roleProbe",
					Exec: &proto.ExecAction{
						Commands: []string{"/bin/bash", "-c", "echo -n leader"},
					},
				},
			}
			probes = []proto.Probe{
				{
					Action:              "roleProbe",
					InitialDelaySeconds: 0,
					PeriodSeconds:       1,
					SuccessThreshold:    1,
					FailureThreshold:    1,
					ReportPeriodSeconds: nil,
				},
			}

			actionSvc *actionService
		)

		BeforeEach(func() {
			var err error
			actionSvc, err = newActionService(logr.New(nil), actions)
			Expect(err).Should(BeNil())
		})

		// func newProbeService(logger logr.Logger, actionService *actionService, probes []proto.Probe) (*probeService, error) {
		It("new", func() {
			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			Expect(service).ShouldNot(BeNil())
			Expect(service.Kind()).Should(Equal(proto.ServiceProbe.Kind))
		})

		It("start", func() {
			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			Expect(service).ShouldNot(BeNil())

			Expect(service.Start()).Should(Succeed())
			Expect(len(service.probes)).Should(Equal(len(service.runners)))
		})

		It("handle request", func() {
			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			Expect(service).ShouldNot(BeNil())

			_, _, err = service.HandleRequest(ctx, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrNotImplemented)).Should(BeTrue())
		})

		It("initial delay seconds", func() {
			probes[0].InitialDelaySeconds = 60
			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			Expect(service).ShouldNot(BeNil())

			Expect(service.Start()).Should(Succeed())

			time.Sleep(1 * time.Second)
			r := service.runners["roleProbe"]
			Expect(r).ShouldNot(BeNil())
			Expect(r.ticker).Should(BeNil())
		})

		// TODO: more test cases
	})
})
