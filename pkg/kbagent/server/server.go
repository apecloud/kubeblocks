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
	"io"

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/kbagent/service"
)

// Server is an interface for the kb-agent server.
type Server interface {
	io.Closer
	StartNonBlocking() error
}

type Config struct {
	Server           bool
	Address          string
	UnixDomainSocket string
	Port             int
	StreamingPort    int
	Concurrency      int
	Logging          bool
}

// NewHTTPServer returns a new HTTP server.
func NewHTTPServer(logger logr.Logger, config Config, services []service.Service) Server {
	return &httpServer{
		logger:   logger,
		config:   config,
		services: services,
	}
}

// NewStreamingServer returns a new Streaming server.
func NewStreamingServer(logger logr.Logger, config Config, service service.Service) Server {
	return &streamingServer{
		logger:  logger,
		config:  config,
		service: service,
	}
}
