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

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/grpcserver"
	"github.com/apecloud/kubeblocks/pkg/lorry/highavailability"
	"github.com/apecloud/kubeblocks/pkg/lorry/httpserver"
	opsregister "github.com/apecloud/kubeblocks/pkg/lorry/operations/register"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var configDir string
var disableDNSChecker bool

func init() {
	viper.AutomaticEnv()
	pflag.StringVar(&configDir, "config-path", "/config/lorry/components/", "Lorry default config directory for builtin type")
	pflag.BoolVar(&disableDNSChecker, "disable-dns-checker", false, "disable dns checker, for test&dev")
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
	ctrl.SetLogger(kzap.New(kopts...))

	// Initialize DB Manager
	err = register.InitDBManager(configDir)
	if err != nil {
		panic(errors.Wrap(err, "DB manager initialize failed"))
	}

	// Initialize DCS (Distributed Control System)
	err = dcs.InitStore()
	if err != nil {
		panic(errors.Wrap(err, "DCS initialize failed"))
	}

	// Start HA
	characterType := viper.GetString(constant.KBEnvCharacterType)
	if viper.IsSet(constant.KBEnvBuiltinHandler) {
		characterType = viper.GetString(constant.KBEnvBuiltinHandler)
	}
	workloadType := viper.GetString(constant.KBEnvWorkloadType)
	if highavailability.IsHAAvailable(characterType, workloadType) {
		ha := highavailability.NewHa(disableDNSChecker)
		if ha != nil {
			defer ha.ShutdownWithWait()
			go ha.Start()
		}
	}

	// start grpc server for role probe
	grpcServer, err := grpcserver.NewGRPCServer()
	if err != nil {
		panic(fmt.Errorf("fatal error grpcserver create failed: %v", err))
	}
	err = grpcServer.StartNonBlocking()
	if err != nil {
		panic(fmt.Errorf("fatal error grpcserver serve failed: %v", err))
	}

	// Start HTTP Server
	ops := opsregister.Operations()
	httpServer := httpserver.NewServer(ops)
	err = httpServer.StartNonBlocking()
	if err != nil {
		panic(errors.Wrap(err, "HTTP server initialize failed"))
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	<-stop
}
