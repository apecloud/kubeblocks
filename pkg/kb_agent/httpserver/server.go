/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package httpserver

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	fasthttprouter "github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
)

// Server is an interface for the kb-agent HTTP server.
type Server interface {
	io.Closer
	Router() fasthttp.RequestHandler
	StartNonBlocking() error
}

type server struct {
	config    Config
	endpoints []Endpoint
	servers   []*fasthttp.Server
}

// NewServer returns a new HTTP server.
func NewServer() Server {
	return &server{
		endpoints: Endpoints(),
		config:    config,
	}
}

// StartNonBlocking starts a new server in a goroutine.
func (s *server) StartNonBlocking() error {
	logger.Info("Starting HTTP Server")
	handler := s.Router()

	APILogging := s.config.APILogging
	if APILogging {
		handler = s.apiLogger(handler)
	}

	var listeners []net.Listener
	if s.config.UnixDomainSocket != "" {
		socket := fmt.Sprintf("%s/kb_agent.socket", s.config.UnixDomainSocket)
		l, err := net.Listen("unix", socket)
		if err != nil {
			return err
		}
		listeners = append(listeners, l)
	} else {
		apiListenAddress := s.config.Address
		l, err := net.Listen("tcp", fmt.Sprintf("%s:%v", apiListenAddress, s.config.Port))
		if err != nil {
			logger.Error(err, "listen address", apiListenAddress, "port", s.config.Port)
		} else {
			listeners = append(listeners, l)
		}
	}

	if len(listeners) == 0 {
		return errors.New("no endpoint to listen on")
	}

	for _, listener := range listeners {
		// customServer is created in a loop because each instance
		// has a handle on the underlying listener.
		customServer := &fasthttp.Server{
			Handler: handler,
		}

		if s.config.ConCurrency > 0 {
			customServer.Concurrency = s.config.ConCurrency
		} else {
			customServer.Concurrency = KBAgentDefaultConcurrency
		}

		s.servers = append(s.servers, customServer)
		go func(l net.Listener) {
			if err := customServer.Serve(l); err != nil {
				panic(err)
			}
		}(listener)
	}

	return nil
}

func (s *server) apiLogger(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		reqLogger := logger
		if userAgent := string(ctx.Request.Header.Peek("User-Agent")); userAgent != "" {
			reqLogger = logger.WithValues("useragent", userAgent)
		}
		start := time.Now()
		path := string(ctx.Path())
		reqLogger.Info("HTTP API Called", "method", string(ctx.Method()), "path", path)
		next(ctx)
		elapsed := float64(time.Since(start) / time.Millisecond)
		reqLogger.Info("HTTP API Called", "status code", ctx.Response.StatusCode(), "cost", elapsed)
	}
}

func (s *server) Router() fasthttp.RequestHandler {
	router := s.getRouter(s.endpoints)

	return router.Handler
}

func (s *server) getRouter(endpoints []Endpoint) *fasthttprouter.Router {
	router := fasthttprouter.New()
	for _, e := range endpoints {
		path := fmt.Sprintf("/%s/%s", e.Version, e.Route)
		router.Handle(e.Method, path, e.Handler)

		if e.LegacyRoute != "" {
			path := fmt.Sprintf("/%s/%s", e.Version, e.LegacyRoute)
			router.Handle(e.Method, path, e.Handler)
		}
	}
	for method, path := range router.List() {
		logger.Info("API route path", "method", method, "path", path)
	}

	return router
}

func (s *server) Close() error {
	errs := make([]error, len(s.servers))

	for i, ln := range s.servers {
		// This calls `Close()` on the underlying listener.
		if err := ln.Shutdown(); err != nil {
			logger.Error(err, "server close failed")
			errs[i] = err
		}
	}

	return errors.Join()
}
