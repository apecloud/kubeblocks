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
	"github.com/valyala/fasthttp"
)

type Endpoint struct {
	Method string
	Route  string

	// Version represents the version of the API, which is currently v1.0.
	// the version is introduced to allow breaking changes.
	// If the API is upgraded to v2.0, the v1.0 API will be maintained
	// for compatibility until all legacy accesses are removed.
	Version string
	// LegacyRoute is used When the API is upgraded, some old APIs may
	// need to update their path routes. To ensure compatibility,
	// the old paths can be setted as legacyRoute.
	LegacyRoute string
	Handler     fasthttp.RequestHandler
}

type Request struct {
	Action     string         `json:"action"`
	Data       interface{}    `json:"data,omitempty"`
	Parameters map[string]any `json:"parameters,omitempty"`
}
