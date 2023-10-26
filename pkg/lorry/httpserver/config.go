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
	"github.com/spf13/pflag"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Config struct {
	Port               int
	Address            string
	MaxRequestBodySize int
	UnixDomainSocket   string
	ReadBufferSize     int
	APILogging         bool
}

var config Config
var logger = ctrl.Log.WithName("HTTPServer")

func init() {
	pflag.IntVar(&config.Port, "port", 3501, "The HTTP Server listen port for Lorry service.")
	pflag.StringVar(&config.Address, "address", "0.0.0.0", "The HTTP Server listen address for Lorry service.")
	pflag.BoolVar(&config.APILogging, "api-logging", true, "Enable api logging for Lorry request.")
}
