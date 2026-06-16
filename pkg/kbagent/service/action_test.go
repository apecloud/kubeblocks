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
	"bytes"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("action", func() {
	Context("action", func() {
		It("exposes service metadata and no-op methods", func() {
			svc, err := newActionService(logr.New(nil), nil)
			Expect(err).Should(BeNil())
			Expect(svc.Kind()).Should(Equal(proto.ServiceAction.Kind))
			Expect(svc.URI()).Should(Equal(proto.ServiceAction.URI))
			Expect(svc.Start()).Should(Succeed())
			Expect(svc.HandleConn(ctx, nil)).Should(Succeed())
		})

		It("decodes and encodes action responses", func() {
			svc, err := newActionService(logr.New(nil), nil)
			Expect(err).Should(BeNil())

			req, err := svc.decode([]byte(`{"action":"backup"}`))
			Expect(err).Should(BeNil())
			Expect(req.Action).Should(Equal("backup"))

			_, err = svc.decode([]byte("{"))
			Expect(errors.Is(err, proto.ErrBadRequest)).Should(BeTrue())

			data := svc.encode([]byte("ok"), nil)
			resp := &proto.ActionResponse{}
			Expect(json.Unmarshal(data, resp)).Should(Succeed())
			Expect(resp.Output).Should(Equal([]byte("ok")))

			data = svc.encode(nil, proto.ErrNotDefined)
			resp = &proto.ActionResponse{}
			Expect(json.Unmarshal(data, resp)).Should(Succeed())
			Expect(resp.Error).Should(Equal("notDefined"))
			Expect(resp.Message).Should(ContainSubstring(proto.ErrNotDefined.Error()))
		})

		It("handles request errors through encoded responses", func() {
			svc, err := newActionService(logr.New(nil), nil)
			Expect(err).Should(BeNil())

			data, err := svc.HandleRequest(ctx, []byte("{"))
			Expect(err).Should(BeNil())
			resp := &proto.ActionResponse{}
			Expect(json.Unmarshal(data, resp)).Should(Succeed())
			Expect(resp.Error).Should(Equal("badRequest"))

			data, err = svc.HandleRequest(ctx, []byte(`{"action":"missing"}`))
			Expect(err).Should(BeNil())
			resp = &proto.ActionResponse{}
			Expect(json.Unmarshal(data, resp)).Should(Succeed())
			Expect(resp.Error).Should(Equal("notDefined"))
		})

		It("handles non-blocking in-progress and completion states", func() {
			svc, err := newActionService(logr.New(nil), []proto.Action{{
				Name: "async",
				Exec: &proto.ExecAction{Commands: []string{"/bin/bash", "-c", "echo -n unused"}},
			}})
			Expect(err).Should(BeNil())

			resultChan := make(chan *asyncResult, 1)
			svc.runningActions["async"] = &runningAction{resultChan: resultChan}
			req := &proto.ActionRequest{Action: "async"}

			out, err := svc.handleRequestNonBlocking(ctx, req, svc.actions["async"], nil)
			Expect(out).Should(BeNil())
			Expect(errors.Is(err, proto.ErrInProgress)).Should(BeTrue())

			resultChan <- &asyncResult{stdout: bytes.NewBufferString("done"), stderr: bytes.NewBuffer(nil)}
			out, err = svc.handleRequestNonBlocking(ctx, req, svc.actions["async"], nil)
			Expect(err).Should(BeNil())
			Expect(string(out)).Should(Equal("done"))
			Expect(svc.runningActions).ShouldNot(HaveKey("async"))
		})

		It("rejects runtime arguments for non-exec actions in blocking and non-blocking calls", func() {
			action := &proto.Action{HTTP: &proto.HTTPAction{Port: "80"}}
			_, err := callActionWithRetry(ctx, action, nil, [][]string{{"arg"}}, nil, nil)
			Expect(errors.Is(err, proto.ErrBadRequest)).Should(BeTrue())

			_, err = nonBlockingCallActionWithRetry(ctx, action, nil, [][]string{{"arg"}}, nil, nil)
			Expect(errors.Is(err, proto.ErrBadRequest)).Should(BeTrue())
		})

		It("resolves timeout preference", func() {
			actionTimeout := int32(10)
			requestTimeout := int32(1)
			Expect(resolveTimeout(&actionTimeout, &requestTimeout)).Should(Equal(&requestTimeout))
			Expect(resolveTimeout(&actionTimeout, nil)).Should(Equal(&actionTimeout))
		})
	})
})
