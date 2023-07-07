package authorize

import (
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
			AuthorizationEndpoint: "http://localhost:" + m.Port + "/authorize",
			TokenEndpoint:         "http://localhost:" + m.Port + "/oauth/token",
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

func (m *MockServer) Close() {
	err := m.server.Shutdown(context.Background())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Server is shutting down...")
	os.Exit(0)
}

func getAvailablePort() (string, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	return fmt.Sprintf("%d", l.Addr().(*net.TCPAddr).Port), nil
}
