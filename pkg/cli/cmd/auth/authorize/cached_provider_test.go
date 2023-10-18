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

package authorize

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/pkg/cli/cmd/auth/authorize/authenticator"
)

type MockKeyring struct {
	key   string
	value []byte
}

func (m *MockKeyring) set(value []byte) error {
	m.value = value
	return nil
}

func (m *MockKeyring) get() ([]byte, error) {
	return m.value, nil
}

func (m *MockKeyring) remove() error {
	m.key = ""
	m.value = nil
	return nil
}

func (m *MockKeyring) isValid() bool {
	return true
}

var _ = Describe("cache", func() {
	var (
		mockKeyring   KeyringProvider
		cached        *KeyringCachedTokenProvider
		tokenResponse authenticator.TokenResponse
	)

	BeforeEach(func() {
		tokenResponse = authenticator.TokenResponse{
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
				err := cached.cacheTokens(&tokenResponse)
				return err
			}()).To(BeNil())
		})

		It("test get token", func() {
			ExpectWithOffset(1, func() error {
				err := cached.cacheTokens(&tokenResponse)
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
				err := cached.cacheTokens(&tokenResponse)
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
				err := cached.cacheTokens(&tokenResponse)
				return err
			}()).To(BeNil())

			ExpectWithOffset(1, func() error {
				err := cached.deleteTokens()
				return err
			}()).To(BeNil())

			ExpectWithOffset(1, func() error {
				_, err := cached.GetTokens()
				return err
			}()).NotTo(BeNil())
		})
	})
})
