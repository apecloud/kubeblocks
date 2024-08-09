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

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"go.uber.org/automaxprocs/maxprocs"
	"go.uber.org/zap"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	kzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	kbagent "github.com/apecloud/kubeblocks/pkg/kbagent"
	"github.com/apecloud/kubeblocks/pkg/kbagent/server"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	defaultPort           = 3501
	defaultMaxConcurrency = 8
)

var serverConfig server.Config

func init() {
	viper.AutomaticEnv()

	pflag.StringVar(&serverConfig.Address, "address", "0.0.0.0", "The HTTP Server listen address for kb-agent service.")
	pflag.StringVar(&serverConfig.UnixDomainSocket, "unix-socket", "", "The path of the Unix Domain Socket for kb-agent service.")
	pflag.IntVar(&serverConfig.Port, "port", defaultPort, "The HTTP Server listen port for kb-agent service.")
	pflag.IntVar(&serverConfig.Concurrency, "max-concurrency", defaultMaxConcurrency,
		fmt.Sprintf("The maximum number of concurrent connections the Server may serve, use the default value %d if <=0.", defaultMaxConcurrency))
	pflag.BoolVar(&serverConfig.Logging, "api-logging", true, "Enable api logging for kb-agent request.")
}

func main() {
	// Set GOMAXPROCS
	_, _ = maxprocs.Set()

	// Initialize flags
	opts := kzap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		panic(errors.Wrap(err, "fatal error viper bindPFlags"))
	}

	// Initialize logger
	kopts := []kzap.Opts{kzap.UseFlagOptions(&opts)}
	if strings.EqualFold("debug", viper.GetString("zap-log-level")) {
		kopts = append(kopts, kzap.RawZapOpts(zap.AddCaller()))
	}
	logger := kzap.New(kopts...)
	ctrl.SetLogger(logger)

	// initialize kb-agent
	services, err := kbagent.Initialize(logger, os.Environ())
	if err != nil {
		panic(errors.Wrap(err, "init action handlers failed"))
	}

	// start HTTP Server
	server := server.NewHTTPServer(logger, serverConfig, services)
	err = server.StartNonBlocking()
	if err != nil {
		panic(errors.Wrap(err, "failed to start HTTP server"))
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	<-stop
}
