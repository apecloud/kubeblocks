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
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	fasthttprouter "github.com/fasthttp/router"
	"github.com/valyala/fasthttp"

	"github.com/apecloud/kubeblocks/pkg/kbagent/service"
)

const (
	jsonContentTypeHeader = "application/json"
)

type server struct {
	config  Config
	servers []*fasthttp.Server
	service service.Service
}

var _ Server = &server{}

// StartNonBlocking starts a new server in a goroutine.
func (s *server) StartNonBlocking() error {
	logger.Info("Starting HTTP Server")
	handler := s.router()

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

func (s *server) router() fasthttp.RequestHandler {
	router := fasthttprouter.New()

	path := fmt.Sprintf("/%s/%s", s.service.Version(), s.service.URI())
	router.Handle(fasthttp.MethodPost, path, s.dispatcher())
	logger.Info("service route", "method", fasthttp.MethodPost, "path", path)

	return router.Handler
}

type request struct {
	Action     string            `json:"action"`
	Data       interface{}       `json:"data,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

func (s *server) dispatcher() func(*fasthttp.RequestCtx) {
	return func(reqCtx *fasthttp.RequestCtx) {
		ctx := context.Background()
		body := reqCtx.PostBody()

		var req request
		if len(body) > 0 {
			err := json.Unmarshal(body, &req)
			if err != nil {
				msg := NewErrorResponse("ERR_MALFORMED_REQUEST", fmt.Sprintf("unmarshal HTTP body failed: %v", err))
				respond(reqCtx, withError(fasthttp.StatusBadRequest, msg))
				return
			}
		}

		_, err := json.Marshal(req.Data)
		if err != nil {
			msg := NewErrorResponse("ERR_MALFORMED_REQUEST_DATA", fmt.Sprintf("marshal request data field: %v", err))
			respond(reqCtx, withError(fasthttp.StatusInternalServerError, msg))
			logger.Info("marshal request data field", "error", err.Error())
			return
		}

		if req.Action == "" {
			msg := NewErrorResponse("ERR_MALFORMED_REQUEST_DATA", "no action in request")
			respond(reqCtx, withError(fasthttp.StatusBadRequest, msg))
			return
		}

		rsp, err := s.service.Call(ctx, req.Action, req.Parameters)
		statusCode := fasthttp.StatusOK
		if err != nil {
			// TODO: not-implemented error
			statusCode = fasthttp.StatusInternalServerError
			logger.Info("action exec failed", "action", req.Action, "error", err.Error())

			msg := NewErrorResponse("ERR_ACTION_FAILED", fmt.Sprintf("action exec failed: %s", err.Error()))
			respond(reqCtx, withError(statusCode, msg))
			return
		}

		if rsp == nil {
			respond(reqCtx, withEmpty())
		} else {
			respond(reqCtx, withJSON(statusCode, rsp))
		}
	}
}

type option = func(ctx *fasthttp.RequestCtx)

// withJSON overrides the content-type with application/json.
func withJSON(code int, obj []byte) option {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.SetStatusCode(code)
		ctx.Response.SetBody(obj)
		ctx.Response.Header.SetContentType(jsonContentTypeHeader)
	}
}

// withError sets error code and jsonify error message.
func withError(code int, resp ErrorResponse) option {
	b, _ := json.Marshal(&resp)
	return withJSON(code, b)
}

func withEmpty() option {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.SetBody(nil)
		ctx.Response.SetStatusCode(fasthttp.StatusNoContent)
	}
}

func respond(ctx *fasthttp.RequestCtx, options ...option) {
	for _, option := range options {
		option(ctx)
	}
}
