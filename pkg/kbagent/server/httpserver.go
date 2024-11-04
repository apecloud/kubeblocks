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

package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	fasthttprouter "github.com/fasthttp/router"
	"github.com/go-logr/logr"
	"github.com/valyala/fasthttp"

	"github.com/apecloud/kubeblocks/pkg/kbagent/service"
)

const (
	defaultMaxConcurrency = 8
	jsonContentTypeHeader = "application/json"
)

type server struct {
	logger   logr.Logger
	config   Config
	services []service.Service
	servers  []*fasthttp.Server
}

var _ Server = &server{}

// StartNonBlocking starts a new server in a goroutine.
func (s *server) StartNonBlocking() error {
	s.logger.Info("starting HTTP server")

	// start all services first
	for i := range s.services {
		if err := s.services[i].Start(); err != nil {
			s.logger.Error(err, fmt.Sprintf("start service %s failed", s.services[i].Kind()))
			return err
		}
		s.logger.Info(fmt.Sprintf("service %s started...", s.services[i].Kind()))
	}

	handler := s.router()

	var listeners []net.Listener
	if s.config.UnixDomainSocket != "" {
		socket := fmt.Sprintf("%s/kbagent.socket", s.config.UnixDomainSocket)
		l, err := net.Listen("unix", socket)
		if err != nil {
			return err
		}
		listeners = append(listeners, l)
	} else {
		l, err := net.Listen("tcp", fmt.Sprintf("%s:%v", s.config.Address, s.config.Port))
		if err != nil {
			s.logger.Error(err, "listen address", s.config.Address, "port", s.config.Port)
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

		if s.config.Concurrency > 0 {
			customServer.Concurrency = s.config.Concurrency
		} else {
			customServer.Concurrency = defaultMaxConcurrency
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

func (s *server) Close() error {
	errs := make([]error, len(s.servers))

	for i, ln := range s.servers {
		// This calls `Close()` on the underlying listener.
		if err := ln.Shutdown(); err != nil {
			s.logger.Error(err, "server close failed")
			errs[i] = err
		}
	}

	return errors.Join()
}

func (s *server) router() fasthttp.RequestHandler {
	router := fasthttprouter.New()
	for i := range s.services {
		s.registerService(router, s.services[i])
	}
	return router.Handler
}

func (s *server) registerService(router *fasthttprouter.Router, svc service.Service) {
	router.Handle(fasthttp.MethodPost, svc.URI(), s.dispatcher(svc))
	s.logger.Info("register service to server", "service", svc.Kind(), "method", fasthttp.MethodPost, "uri", svc.URI())
}

func (s *server) dispatcher(svc service.Service) func(*fasthttp.RequestCtx) {
	return func(reqCtx *fasthttp.RequestCtx) {
		ctx := context.Background()
		body := reqCtx.PostBody()

		output, err := svc.HandleRequest(ctx, body)
		statusCode := fasthttp.StatusOK
		if err != nil {
			statusCode = fasthttp.StatusInternalServerError
		}
		respond(reqCtx, statusCode, output, err)
		if s.config.Logging {
			s.logger.Info("HTTP API Called",
				"user-agent", string(reqCtx.Request.Header.UserAgent()),
				"method", string(reqCtx.Method()),
				"path", string(reqCtx.Path()),
				"status code", statusCode,
				"cost", time.Since(reqCtx.Time()).Milliseconds(),
			)
		}
	}
}

func respond(ctx *fasthttp.RequestCtx, code int, body []byte, err error) {
	ctx.Response.Header.SetContentType(jsonContentTypeHeader)
	ctx.Response.SetStatusCode(code)
	switch {
	case err != nil:
		ctx.Response.SetBodyString(err.Error())
	case body != nil:
		ctx.Response.SetBody(body)
	}
}
