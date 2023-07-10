package authorize

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
		callbackListener = NewCallbackService(port)
		state = "test_state"
		codeReceiverCh = make(chan CallbackResponse)
	})

	AfterEach(func() {
	})

	Context("test callback listener", func() {
		It("test callback listener", func() {
			callbackListener.AwaitResponse(codeReceiverCh, state)

			go func() {
				_, err := http.Get("http://127.0.0.1:" + port + "/callback?code=test_code&state=test_state")
				Expect(err).To(BeNil())
			}()

			ExpectWithOffset(1, func() error {
				callbackResult := <-codeReceiverCh
				Expect(callbackResult.Code).To(Equal("test_code"))
				return callbackResult.Error
			}()).To(BeNil())
		})
	})
})
