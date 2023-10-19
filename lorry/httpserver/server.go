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

package httpserver

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/fasthttp/router"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"

	"github.com/apecloud/kubeblocks/lorry/operations"
)

// Server is an interface for the Dapr HTTP server.
type Server interface {
	io.Closer
	Router() fasthttp.RequestHandler
	StartNonBlocking() error
}

type server struct {
	config  Config
	api     API
	servers []*fasthttp.Server
}

// NewServer returns a new HTTP server.
func NewServer(ops map[string]operations.Operation) Server {
	a := &api{}
	a.RegisterOperations(ops)
	return &server{
		api:    a,
		config: config,
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
		socket := fmt.Sprintf("%s/lorry.socket", s.config.UnixDomainSocket)
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
		return errors.Errorf("could not listen on any endpoint")
	}

	for _, listener := range listeners {
		// customServer is created in a loop because each instance
		// has a handle on the underlying listener.
		customServer := &fasthttp.Server{
			Handler:            handler,
			MaxRequestBodySize: s.config.MaxRequestBodySize * 1024 * 1024,
			ReadBufferSize:     s.config.ReadBufferSize * 1024,
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
		reqLogger.Info("HTTP API Called", "method", string(ctx.Method()), "path", string(ctx.Path()))
		next(ctx)
		elapsed := float64(time.Since(start) / time.Millisecond)
		reqLogger.Info("HTTP API Response", "status code", ctx.Response.StatusCode(), "cost", elapsed)
	}
}

func (s *server) Router() fasthttp.RequestHandler {
	endpoints := s.api.Endpoints()
	router := s.getRouter(endpoints)

	return router.Handler
}

func (s *server) getRouter(endpoints []Endpoint) *router.Router {
	router := router.New()
	for _, e := range endpoints {
		path := fmt.Sprintf("/%s/%s", e.Version, e.Route)
		router.Handle(e.Method, path, e.Handler)

		if e.Duplicate != "" {
			path := fmt.Sprintf("/%s/%s", e.Version, e.Duplicate)
			router.Handle(e.Method, path, e.Handler)
		}
	}

	return router
}

func (s *server) Close() error {
	var merr error

	for _, ln := range s.servers {
		// This calls `Close()` on the underlying listener.
		if err := ln.Shutdown(); err != nil {
			logger.Error(err, "server close failed")
		}
	}

	return merr
}
