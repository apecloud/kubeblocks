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

func mockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
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
		a        *PKCEAuthenticator
		err      error
		server   *httptest.Server
	)

	BeforeEach(func() {
		server = mockServer()
		ExpectWithOffset(1, func() error {
			a, err = newPKCEAuthenticator(nil, clientID, server.URL)
			return err
		}()).To(BeNil())
	})

	AfterEach(func() {
		server.Close()
	})

	Context("test Authorization", func() {
		It("test get userInfo", func() {
			_, err := a.GetUserInfo(context.TODO(), "test_token")
			Expect(err).Should(HaveOccurred())
		})

		It("test get token", func() {
			authorizationResponse := &AuthorizationResponse{
				CallbackURL: server.URL + "?code=test_code&state=test_state",
				Code:        "test_code",
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
