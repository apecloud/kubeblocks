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

	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
)

func mockDeviceServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/device/code":
			deviceCodeResponse := DeviceCodeResponse{
				DeviceCode:              "test_device_code",
				UserCode:                "test_user_code",
				PollingInterval:         5,
				ExpiresIn:               1800,
				VerificationURI:         "https://example.com/device",
				VerificationCompleteURI: "https://example.com/device?user_code=test_user_code",
			}

			jsonData, err := json.Marshal(deviceCodeResponse)
			if err != nil {
				log.Fatalf("failed to marshal JSON: %v", err)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(jsonData)

		case "/oauth/token":
			tokenResp := TokenResponse{
				AccessToken:  "test_access_token",
				RefreshToken: "test_refresh_token",
				IDToken:      "test_id_token",
				ExpiresIn:    3600,
			}

			jsonData, err := json.Marshal(tokenResp)
			if err != nil {
				log.Fatalf("failed to marshal JSON: %v", err)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(jsonData)

		case "/userinfo":
			user := UserInfoResponse{
				Name:    "John Doe",
				Email:   "john.doe@example.com",
				Locale:  "en-US",
				Subject: "apecloud",
			}
			jsonData, err := json.Marshal(user)
			if err != nil {
				log.Fatalf("failed to marshal JSON: %v", err)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(jsonData)

		case "/oidc/logout":
			w.WriteHeader(http.StatusOK)
		}
	}))
}

var _ = Describe("PKCE_Authenticator", func() {
	var (
		clientID = "test_clientID"
		a        *DeviceAuthenticator
		err      error
		server   *httptest.Server
	)

	BeforeEach(func() {
		server = mockDeviceServer()
		ExpectWithOffset(1, func() error {
			a, err = newDeviceAuthenticator(nil, clientID, server.URL)
			return err
		}()).To(BeNil())
	})

	AfterEach(func() {
		server.Close()
	})

	Context("test Authorization", func() {
		It("test get Authorization", func() {
			ExpectWithOffset(1, func() error {
				openFunc := func(URL string) {
					fmt.Println(URL)
				}
				_, err := a.GetAuthorization(context.TODO(), openFunc)
				return err
			}()).To(BeNil())
		})

		It("test get userInfo", func() {
			ExpectWithOffset(1, func() error {
				_, err := a.GetUserInfo(context.TODO(), "test_token")
				return err
			}()).To(BeNil())
		})

		It("test get token", func() {
			authorizationResponse := &DeviceVerification{
				DeviceCode:              "test_device_code",
				UserCode:                "test_user_code",
				Interval:                5,
				VerificationURL:         "https://example.com/device",
				VerificationCompleteURL: "https://example.com/device?user_code=test_user_code",
			}
			ExpectWithOffset(1, func() error {
				_, err := a.GetToken(context.TODO(), authorizationResponse)
				return err
			}()).To(BeNil())
		})

		It("test get refreshToken", func() {
			ExpectWithOffset(1, func() error {
				_, err := a.RefreshToken(context.TODO(), "test_refresh_token")
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
