package authorize

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"context"
	"fmt"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/auth/authorize/test"
)

var _ = Describe("PKCE_Authenticator", func() {
	var (
		clientID   = "test_clientID"
		a          *PKCEAuthenticator
		err        error
		mockServer *test.MockServer
	)

	BeforeEach(func() {
		mockServer = test.NewMockServer()
		fmt.Println(mockServer.Port)
		go mockServer.Start()

		authURL := fmt.Sprintf("http://localhost:%s", mockServer.Port)
		ExpectWithOffset(1, func() error {
			a, err = NewPKCEAuthenticator(nil, clientID, authURL)
			return err
		}()).To(BeNil())
	})

	AfterEach(func() {
	})

	Context("test Authorization", func() {
		It("test get token", func() {
			authorizationResponse := &AuthorizationResponse{
				CallbackURL: "http://localhost:5000?code=test_code&state=test_state",
				Code:        "test_code",
			}
			ExpectWithOffset(1, func() error {
				_, err := a.GetToken(context.TODO(), authorizationResponse)
				return err
			}()).To(BeNil())
		})

		It("test get userInfo", func() {
			ExpectWithOffset(1, func() error {
				_, err := a.GetUserInfo(context.TODO(), "test_token")
				return err
			}()).To(BeNil())
		})

		It("test get RefreshToken", func() {
			authorizationResponse := &AuthorizationResponse{
				CallbackURL: "http://localhost:5000?code=test_code&state=test_state",
				Code:        "test_code",
			}
			ExpectWithOffset(1, func() error {
				_, err := a.GetToken(context.TODO(), authorizationResponse)
				return err
			}()).To(BeNil())
		})

		It("test logout", func() {
			openFunc := func(URL string) {
				fmt.Println(URL)
			}
			ExpectWithOffset(1, func() error {
				err := a.Logout(context.TODO(), "test_token", openFunc)
				return err
			}()).To(BeNil())
		})
	})
})
