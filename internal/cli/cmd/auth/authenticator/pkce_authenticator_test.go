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
	"net"
	"net/http"
	"os"
	"os/signal"
)

type MockServer struct {
	server *http.Server
	Port   string
}

func NewMockServer() *MockServer {
	port, err := getAvailablePort()
	if err != nil {
		log.Fatal(err)
	}
	return &MockServer{
		Port: port,
	}
}

func (m *MockServer) Start() {
	mux := http.NewServeMux()

	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		callbackURL := fmt.Sprintf("http://127.0.0.1:8000/callback?code=%s&state=%s", "test_code", "test_state")
		resp, err := http.Get(callbackURL)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		body := make([]byte, 1024)
		_, err = resp.Body.Read(body)
		if err != nil {
			log.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		tokenResp := TokenResponse{
			AccessToken:  "test_access_token",
			RefreshToken: "test_refresh_token",
			IDToken:      "test_id_token",
			ExpiresIn:    3600,
		}

		jsonData, err := json.Marshal(tokenResp)
		if err != nil {
			log.Fatal(err)
		}

		w.Header().Set("Content-Type", "application/json")

		_, err = w.Write(jsonData)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		user := UserInfoResponse{
			Name:    "John Doe",
			Email:   "john.doe@example.com",
			Locale:  "en-US",
			Subject: "apecloud",
		}

		jsonData, err := json.Marshal(user)
		if err != nil {
			log.Fatal(err)
		}

		w.Header().Set("Content-Type", "application/json")

		_, err = w.Write(jsonData)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/oidc/logout", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		oidcConfig := OIDCWellKnownEndpoints{
			AuthorizationEndpoint: "http://127.0.0.1:" + m.Port + "/authorize",
			TokenEndpoint:         "http://127.0.0.1:" + m.Port + "/oauth/token",
		}

		jsonData, err := json.Marshal(oidcConfig)
		if err != nil {
			log.Fatal(err)
		}

		w.Header().Set("Content-Type", "application/json")

		_, err = w.Write(jsonData)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	m.server = &http.Server{
		Addr:    ":" + m.Port,
		Handler: mux,
	}
	go func() {
		err := m.server.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	<-signalCh

	err := m.server.Shutdown(context.Background())
	if err != nil {
		return
	}
	log.Println("Server is shutting down...")
}

func getAvailablePort() (string, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	return fmt.Sprintf("%d", l.Addr().(*net.TCPAddr).Port), nil
}

var _ = Describe("PKCE_Authenticator", func() {
	var (
		clientID   = "test_clientID"
		a          *PKCEAuthenticator
		err        error
		mockServer *MockServer
	)

	BeforeEach(func() {
		mockServer = NewMockServer()
		go mockServer.Start()

		authURL := fmt.Sprintf("http://127.0.0.1:%s", mockServer.Port)
		ExpectWithOffset(1, func() error {
			a, err = newPKCEAuthenticator(nil, clientID, authURL)
			return err
		}()).To(BeNil())
	})

	AfterEach(func() {
	})

	Context("test Authorization", func() {
		It("test get token", func() {
			authorizationResponse := &AuthorizationResponse{
				CallbackURL: "http://127.0.0.1:5000?code=test_code&state=test_state",
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
				CallbackURL: "http://127.0.0.1:5000?code=test_code&state=test_state",
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
