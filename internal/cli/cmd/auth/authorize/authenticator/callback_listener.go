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
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/leaanthony/debme"
)

var (
	//go:embed callback_html/*
	callbackHTML embed.FS
)

const (
	ListenerAddress = "127.0.0.1"
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

func newCallbackService(port string) *CallbackService {
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

func (c *CallbackService) getCallbackURL() string {
	return fmt.Sprintf("http://%s/callback", c.addr)
}

func (c *CallbackService) close() {
	c.httpServer.shutdown()
}

// AwaitResponse sets up the response channel to receive the code that comes in
// the from authorization code callback handler
func (c *CallbackService) awaitResponse(callbackResponse chan CallbackResponse, state string) {
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

func writeHTML(w http.ResponseWriter, fileName string) {
	tmplFs, _ := debme.FS(callbackHTML, "callback_html")
	tmlBytes, err := tmplFs.ReadFile(fileName)
	if err != nil {
		http.Error(w, "Failed to read HTML file", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	write(w, tmlBytes)
}

func write(w http.ResponseWriter, msg []byte) {
	_, err := w.Write(msg)
	if err != nil {
		fmt.Println("Error writing response:", err)
	}
}
