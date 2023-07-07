package authorize

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cache", func() {
	var (
		mockKeyring   KeyringProvider
		cached        *KeyringCachedTokenProvider
		tokenResponse TokenResponse
	)

	BeforeEach(func() {
		tokenResponse = TokenResponse{
			AccessToken:  "test",
			RefreshToken: "test",
		}
		mockKeyring = &MockKeyring{}
		cached = NewKeyringCachedTokenProvider(&mockKeyring)
	})

	AfterEach(func() {
	})

	Context("test cache", func() {
		It("test cached token", func() {
			ExpectWithOffset(1, func() error {
				err := cached.CacheTokens(&tokenResponse)
				return err
			}()).To(BeNil())
		})

		It("test get token", func() {
			ExpectWithOffset(1, func() error {
				err := cached.CacheTokens(&tokenResponse)
				return err
			}()).To(BeNil())
			ExpectWithOffset(1, func() error {
				tokenResponse, err := cached.GetTokens()
				Expect(tokenResponse.AccessToken).To(Equal("test"))
				return err
			}()).To(BeNil())
		})

		It("test fail to get token", func() {
			ExpectWithOffset(1, func() error {
				err := cached.CacheTokens(&tokenResponse)
				return err
			}()).To(BeNil())
			ExpectWithOffset(1, func() error {
				tokenResponse, err := cached.GetTokens()
				Expect(tokenResponse.AccessToken).NotTo(Equal("cloud"))
				return err
			}()).To(BeNil())
		})

		It("test delete token", func() {
			ExpectWithOffset(1, func() error {
				err := cached.CacheTokens(&tokenResponse)
				return err
			}()).To(BeNil())

			ExpectWithOffset(1, func() error {
				err := cached.DeleteTokens()
				return err
			}()).To(BeNil())

			ExpectWithOffset(1, func() error {
				_, err := cached.GetTokens()
				return err
			}()).NotTo(BeNil())
		})
	})
})
