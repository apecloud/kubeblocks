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

	"context"
	"fmt"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/auth/authorize/authenticator"
)

var authorizationRes = authenticator.AuthorizationResponse{
	Code:        "test_code",
	CallbackURL: "test_callback_url",
}

type MockAuthenticator struct {
	userInfoResponse      *authenticator.UserInfoResponse
	tokenResponse         *authenticator.TokenResponse
	authorizationResponse *authenticator.AuthorizationResponse
}

func NewMockAuthenticator() *MockAuthenticator {
	return &MockAuthenticator{
		userInfoResponse:      &userInfoResponse,
		tokenResponse:         &tokenResponse,
		authorizationResponse: &authorizationRes,
	}
}

func (m *MockAuthenticator) GetAuthorization(ctx context.Context, openURLFunc func(URL string), states ...string) (interface{}, error) {
	return m.authorizationResponse, nil
}

func (m *MockAuthenticator) GetToken(ctx context.Context, authorization interface{}) (*authenticator.TokenResponse, error) {
	if response, ok := authorization.(*authenticator.AuthorizationResponse); ok {
		if response.Code == m.authorizationResponse.Code {
			return m.tokenResponse, nil
		}
	}
	return nil, fmt.Errorf("authorization not match")
}

func (m *MockAuthenticator) GetUserInfo(ctx context.Context, token string) (*authenticator.UserInfoResponse, error) {
	if token == m.tokenResponse.AccessToken {
		return m.userInfoResponse, nil
	}
	return nil, fmt.Errorf("token not match")
}

func (m *MockAuthenticator) Logout(ctx context.Context, token string, openURLFunc func(URL string)) error {
	if token == m.tokenResponse.AccessToken {
		return nil
	}
	return fmt.Errorf("token not match")
}

func (m *MockAuthenticator) RefreshToken(ctx context.Context, refreshToken string) (*authenticator.TokenResponse, error) {
	if refreshToken == m.tokenResponse.RefreshToken {
		m.tokenResponse.AccessToken = "newAccessToken"
		return m.tokenResponse, nil
	}
	return nil, fmt.Errorf("refresh token not match")
}

var _ = Describe("issued provider", func() {
	var (
		mockAuthenticator   *MockAuthenticator
		issuedTokenProvider *CloudIssuedTokenProvider
		tokenRes            *authenticator.TokenResponse
		o                   Options
		streams             genericiooptions.IOStreams
	)

	BeforeEach(func() {
		mockAuthenticator = NewMockAuthenticator()
		streams, _, _, _ = genericiooptions.NewTestIOStreams()
		o = Options{
			ClientID:  "test_client_id",
			AuthURL:   "test_auth_url",
			NoBrowser: true,
			IOStreams: streams,
		}
	})

	AfterEach(func() {
	})

	Context("test issued provider", func() {
		It("test authenticate", func() {
			ExpectWithOffset(1, func() error {
				var err error
				issuedTokenProvider, err = newIssuedTokenProvider(o, mockAuthenticator)
				return err
			}()).To(BeNil())

			ExpectWithOffset(1, func() error {
				_, err := issuedTokenProvider.authenticate(context.Background())
				return err
			}()).To(BeNil())
		})

		It("test getUserInfo", func() {
			ExpectWithOffset(1, func() error {
				var err error
				issuedTokenProvider, err = newIssuedTokenProvider(o, mockAuthenticator)
				return err
			}()).To(BeNil())

			ExpectWithOffset(1, func() error {
				var err error
				tokenRes, err = issuedTokenProvider.authenticate(context.Background())
				return err
			}()).To(BeNil())

			ExpectWithOffset(1, func() error {
				_, err := issuedTokenProvider.getUserInfo(tokenRes.AccessToken)
				return err
			}()).To(BeNil())
		})

		It("test refreshToken", func() {
			ExpectWithOffset(1, func() error {
				var err error
				issuedTokenProvider, err = newIssuedTokenProvider(o, mockAuthenticator)
				return err
			}()).To(BeNil())

			ExpectWithOffset(1, func() error {
				var err error
				tokenRes, err = issuedTokenProvider.authenticate(context.Background())
				return err
			}()).To(BeNil())

			ExpectWithOffset(1, func() error {
				_, err := issuedTokenProvider.refreshToken(tokenRes.RefreshToken)
				return err
			}()).To(BeNil())
		})

		It("test logout", func() {
			ExpectWithOffset(1, func() error {
				var err error
				issuedTokenProvider, err = newIssuedTokenProvider(o, mockAuthenticator)
				return err
			}()).To(BeNil())

			ExpectWithOffset(1, func() error {
				var err error
				tokenRes, err = issuedTokenProvider.authenticate(context.Background())
				return err
			}()).To(BeNil())

			ExpectWithOffset(1, func() error {
				err := issuedTokenProvider.logout(context.Background(), tokenRes.AccessToken)
				return err
			}()).To(BeNil())
		})
	})
})
