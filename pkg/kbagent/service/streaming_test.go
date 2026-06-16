/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("streaming", func() {
	Context("streaming", func() {
		It("exposes service metadata and request unsupported error", func() {
			actionSvc, err := newActionService(logr.New(nil), []proto.Action{{
				Name: "dump",
				Exec: &proto.ExecAction{Commands: []string{"/bin/bash", "-c", "cat"}},
			}})
			Expect(err).Should(BeNil())
			svc, err := newStreamingService(logr.New(nil), actionSvc, []string{"dump"})
			Expect(err).Should(BeNil())

			Expect(svc.Kind()).Should(Equal(proto.ServiceStreaming.Kind))
			Expect(svc.URI()).Should(Equal(proto.ServiceStreaming.URI))
			Expect(svc.Start()).Should(Succeed())
			out, err := svc.HandleRequest(ctx, []byte("{}"))
			Expect(out).Should(BeNil())
			Expect(errors.Is(err, proto.ErrNotImplemented)).Should(BeTrue())
		})

		It("rejects unsupported handshake actions", func() {
			actionSvc, err := newActionService(logr.New(nil), []proto.Action{{
				Name: "dump",
				Exec: &proto.ExecAction{Commands: []string{"/bin/bash", "-c", "cat"}},
			}})
			Expect(err).Should(BeNil())
			svc, err := newStreamingService(logr.New(nil), actionSvc, []string{"dump"})
			Expect(err).Should(BeNil())

			serverConn, clientConn := net.Pipe()
			defer clientConn.Close()
			go func() {
				defer GinkgoRecover()
				_, _ = clientConn.Write([]byte(`{"action":"missing"}`))
			}()

			err = svc.HandleConn(ctx, serverConn)
			Expect(err).Should(MatchError(ContainSubstring("missing is not supported")))
		})

		It("returns bad request for invalid handshake packets", func() {
			svc := &streamingService{}
			serverConn, clientConn := net.Pipe()
			defer clientConn.Close()
			go func() {
				defer GinkgoRecover()
				_, _ = clientConn.Write([]byte("{"))
				_ = clientConn.Close()
			}()

			req, err := svc.handshake(ctx, serverConn)
			Expect(req).Should(BeNil())
			Expect(errors.Is(err, proto.ErrBadRequest)).Should(BeTrue())
		})

		It("handles supported streaming actions", func() {
			actionSvc, err := newActionService(logr.New(nil), []proto.Action{{
				Name: "dump",
				Exec: &proto.ExecAction{Commands: []string{"/bin/bash", "-c", "true"}},
			}})
			Expect(err).Should(BeNil())
			svc, err := newStreamingService(logr.New(nil), actionSvc, []string{"dump"})
			Expect(err).Should(BeNil())

			serverConn, clientConn := net.Pipe()
			defer clientConn.Close()
			go func() {
				defer GinkgoRecover()
				_, _ = clientConn.Write([]byte(`{"action":"dump"}`))
			}()

			Expect(svc.HandleConn(ctx, serverConn)).Should(Succeed())
			Expect(serverConn.Close()).Should(Succeed())
		})
	})
})
