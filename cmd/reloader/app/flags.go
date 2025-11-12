/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package app

import (
	"os"

	"github.com/spf13/pflag"

	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	configManagerDefaultPort = 9901
	configPodIPEnvName       = "CONFIG_MANAGER_POD_IP"
	localhostAddress         = "127.0.0.1"
)

func init() {
	if err := viper.BindEnv(configPodIPEnvName); err != nil {
		os.Exit(-2)
	}
	// viper.AutomaticEnv()
	viper.SetDefault(configPodIPEnvName, localhostAddress)
}

type serviceOptions struct {
	GrpcPort int
	PodIP    string

	// EnableRemoteOnlineUpdate enables remote online update
	RemoteOnlineUpdateEnable bool

	DebugMode bool

	LogLevel   string
	CombConfig string
}

func newServiceOptions() *serviceOptions {
	return &serviceOptions{
		GrpcPort:                 configManagerDefaultPort,
		PodIP:                    viper.GetString(configPodIPEnvName),
		DebugMode:                false,
		RemoteOnlineUpdateEnable: true,
		LogLevel:                 "info",
	}
}

func installFlags(flags *pflag.FlagSet, opts *serviceOptions) {
	flags.StringVar(&opts.LogLevel,
		"log-level",
		opts.LogLevel,
		"the config sets log level. enum: [error, info, debug]")
	flags.StringVar(&opts.PodIP,
		"pod-ip",
		opts.PodIP,
		"the config sets pod ip address.")
	flags.IntVar(&opts.GrpcPort,
		"tcp",
		opts.GrpcPort,
		"the config sets service port.")
	flags.BoolVar(&opts.DebugMode,
		"debug",
		opts.DebugMode,
		"the config sets debug mode.")

	flags.BoolVar(&opts.RemoteOnlineUpdateEnable,
		"operator-update-enable",
		opts.RemoteOnlineUpdateEnable,
		"the config sets enable operator update parameter.")

	// for multi handler
	flags.StringVar(&opts.CombConfig,
		"config",
		"",
		"the reload config.")
}
