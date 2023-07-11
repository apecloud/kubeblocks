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
)

type MockIssued struct {
	tokenResponse    *TokenResponse
	userInfoResponse *UserInfoResponse
}

var tokenResponse = TokenResponse{
	AccessToken:  "test_access_token",
	RefreshToken: "test_refresh_token",
	IDToken:      "test_id_token",
	ExpiresIn:    3600000000000,
}

var userInfoResponse = UserInfoResponse{
	Name:    "test_name",
	Email:   "test_email",
	Locale:  "test_locale",
	Subject: "test_subject",
}

func (m *MockIssued) DeviceAuthenticate() (*TokenResponse, error) {
	return m.tokenResponse, nil
}

func (m *MockIssued) PKCEAuthenticate(ctx context.Context) (*TokenResponse, error) {
	return m.tokenResponse, nil
}

func (m *MockIssued) refreshTokenFromPKCE(refreshToken string) (*TokenResponse, error) {
	m.tokenResponse.AccessToken = "test_refreshed_access_token"
	return m.tokenResponse, nil
}

func (m *MockIssued) getUserInfoForDevice(token string) (*UserInfoResponse, error) {
	if token == m.tokenResponse.AccessToken {
		return m.userInfoResponse, nil
	}
	return nil, fmt.Errorf("token not match")
}

func (m *MockIssued) getUserInfoFromPKCE(token string) (*UserInfoResponse, error) {
	if token == m.tokenResponse.AccessToken {
		return m.userInfoResponse, nil
	}
	return nil, fmt.Errorf("token not match")
}

func (m *MockIssued) logoutForDevice(token string) error {
	if token == m.tokenResponse.IDToken {
		return nil
	}
	return fmt.Errorf("token not match")
}

func (m *MockIssued) logoutForPKCE(ctx context.Context, token string) error {
	return nil
}

type MockCached struct {
	tokenResponse    *TokenResponse
	userInfoResponse *UserInfoResponse
}

func (m *MockCached) cacheTokens(tokenResponse *TokenResponse) error {
	m.tokenResponse = tokenResponse
	return nil
}

func (m *MockCached) deleteTokens() error {
	m.tokenResponse = nil
	return nil
}

func (m *MockCached) cacheUserInfo(userInfoResponse *UserInfoResponse) error {
	m.userInfoResponse = userInfoResponse
	return nil
}

func (m *MockCached) GetTokens() (*TokenResponse, error) {
	return m.tokenResponse, nil
}

func (m *MockCached) getUserInfo() (*UserInfoResponse, error) {
	return m.userInfoResponse, nil
}

var _ = Describe("token provider", func() {
	var (
		mockIssued *MockIssued
		mockCached *MockCached
	)

	BeforeEach(func() {
		mockIssued = &MockIssued{
			tokenResponse:    &tokenResponse,
			userInfoResponse: &userInfoResponse,
		}

		mockCached = &MockCached{
			tokenResponse:    &tokenResponse,
			userInfoResponse: &userInfoResponse,
		}
	})

	AfterEach(func() {
	})

	Context("test token provider", func() {
		It("test login", func() {
			tokenProvider := newTokenProvider(mockCached, mockIssued)

			ExpectWithOffset(1, func() error {
				userInfo, err := tokenProvider.Login(context.Background())
				Expect(userInfo.Name).To(Equal("test_name"))
				return err
			}()).To(BeNil())
		})

		It("test logout", func() {
			tokenProvider := newTokenProvider(mockCached, mockIssued)

			ExpectWithOffset(1, func() error {
				err := tokenProvider.Logout(context.Background())
				return err
			}()).To(BeNil())
		})
	})
})
