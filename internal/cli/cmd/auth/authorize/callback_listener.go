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
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
)

const (
	ListenerAddress = "127.0.0.1"
	DIR             = "./internal/cli/cmd/auth/authorize/callback_html/"
)

type HTTPServer interface {
	start(addr string)
	shutdown()
}

// CallbackService is used to handle the callback received in the PKCE flow
type CallbackService struct {
	addr       string
	httpServer HTTPServer
}

type callbackServer struct {
	server *http.Server
}

func NewCallbackService(port string) *CallbackService {
	return &CallbackService{
		strings.Join([]string{ListenerAddress, port}, ":"),
		&callbackServer{},
	}
}

func (s *callbackServer) start(addr string) {
	s.server = &http.Server{
		Addr: addr,
	}
	go func() {
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			panic(err)
		}
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	go func() {
		<-signalCh
		s.shutdown()
		os.Exit(0)
	}()
}

func (s *callbackServer) shutdown() {
	if err := s.server.Shutdown(context.Background()); err != nil {
		log.Printf("HTTP server Shutdown error: %v", err)
	}
}

func (c *CallbackService) GetCallbackURL() string {
	fmt.Println("callback url: ", fmt.Sprintf("http://%s/callback", c.addr))
	return fmt.Sprintf("http://%s/callback", c.addr)
}

func (c *CallbackService) Close() {
	c.httpServer.shutdown()
}

// AwaitResponse sets up the response channel to receive the code that comes in
// the from authorization code callback handler
func (c *CallbackService) AwaitResponse(callbackResponse chan CallbackResponse, state string) {
	c.httpServer.start(c.addr)

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		response := CallbackResponse{}
		if r.URL.Query().Get("state") != state {
			response.Error = errors.New("callback completed with incorrect state")
			writeHTML(w, "error.html")
		} else if callbackErr := r.URL.Query().Get("error"); callbackErr != "" {
			response.Error = fmt.Errorf("%s: %s", callbackErr, r.URL.Query().Get("error_description"))
			writeHTML(w, "error.html")
		} else if code := r.URL.Query().Get("code"); code != "" {
			response.Code = code
			writeHTML(w, "complete.html")
		} else {
			response.Error = errors.New("callback completed with no error or code")
			writeHTML(w, "error.html")
		}
		callbackResponse <- response
	})
}

func writeHTML(w http.ResponseWriter, file string) {
	htmlContent, err := os.ReadFile(DIR + file)
	if err != nil {
		http.Error(w, "Failed to read HTML file", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	write(w, string(htmlContent))
}

func write(w http.ResponseWriter, msg string) {
	_, err := w.Write([]byte(msg))
	if err != nil {
		fmt.Println("Error writing response:", err)
	}
}
