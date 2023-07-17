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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"net/http"
)

var _ = Describe("callback listener", func() {
	var (
		callbackListener *CallbackService
		state            string
		codeReceiverCh   chan CallbackResponse
		port             = "8001"
	)

	BeforeEach(func() {
		callbackListener = newCallbackService(port)
		state = "test_state"
		codeReceiverCh = make(chan CallbackResponse)

	})

	AfterEach(func() {
	})

	Context("test callback listener", func() {
		It("test callback listener", func() {
			callbackListener.awaitResponse(codeReceiverCh, state)

			go func() {
				_, _ = http.Get("http://127.0.0.1:" + port + "/callback?code=test_code&state=test_state")
			}()

			ExpectWithOffset(1, func() error {
				callbackResult := <-codeReceiverCh
				Expect(callbackResult.Code).To(Equal("test_code"))
				return callbackResult.Error
			}()).To(BeNil())
		})
	})
})
