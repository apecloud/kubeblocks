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

package authenticator

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"net/http"
	"net/http/httptest"
)

var _ = Describe("CallbackService", func() {
	var (
		server  *httptest.Server
		service *CallbackService
	)

	BeforeEach(func() {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("callback response"))
		}))

		service = newCallbackService(server.URL)
	})

	AfterEach(func() {
		server.Close()
	})

	It("should await response and receive callback", func() {
		responseCh := make(chan CallbackResponse)
		state := "test_state"

		go service.awaitResponse(responseCh, state)

		resp, err := http.Get(server.URL + "/callback?state=" + state)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		response := <-responseCh

		Expect(response.Error).NotTo(HaveOccurred())
		Expect(response.Code).To(Equal("callback response"))
	})
})
