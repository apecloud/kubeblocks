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
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"go.uber.org/automaxprocs/maxprocs"
	"google.golang.org/grpc"
	health "google.golang.org/grpc/health/grpc_health_v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/apecloud/kubeblocks/lorry/component"
	"github.com/apecloud/kubeblocks/lorry/highavailability"
	customgrpc "github.com/apecloud/kubeblocks/lorry/middleware/grpc"
	probe2 "github.com/apecloud/kubeblocks/lorry/middleware/probe"
	"github.com/apecloud/kubeblocks/lorry/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var (
	port      int
	grpcPort  int
	configDir string
)

const (
	DefaultPort       = 3501
	DefaultGRPCPort   = 50001
	DefaultConfigPath = "/config/lorry/components"
)

func init() {
	viper.AutomaticEnv()
	viper.SetDefault(constant.KBEnvCharacterType, "custom")
	flag.IntVar(&port, "port", DefaultPort, "lorry http default port")
	flag.IntVar(&grpcPort, "grpcport", DefaultGRPCPort, "lorry grpc default port")
	flag.StringVar(&configDir, "config-path", DefaultConfigPath, "lorry default config directory for builtin type")
}

func main() {
	// set GOMAXPROCS
	_, _ = maxprocs.Set()

	// setup log
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)

	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		panic(fmt.Errorf("fatal error viper bindPFlags: %v", err))
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	characterType := viper.GetString(constant.KBEnvCharacterType)
	workloadType := viper.GetString(constant.KBEnvWorkloadType)
	err = component.GetAllComponent(configDir) // find all builtin config file and read
	if err != nil {                            // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %v", err))
	}

	err = probe2.RegisterBuiltin(characterType) // register builtin component
	if err != nil {
		panic(fmt.Errorf("fatal error register builtin: %v", err))
	}

	// start http server for lorry client
	http.HandleFunc("/", probe2.SetMiddleware(probe2.GetRouter()))
	go func() {
		addr := fmt.Sprintf(":%d", port)
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			panic(fmt.Errorf("fatal error listen on port %d", port))
		}
	}()

	// start grpc server for role probe
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		panic(fmt.Errorf("fatal error listen on port %d: %v", grpcPort, err))
	}
	healthServer := customgrpc.NewGRPCServer()
	server := grpc.NewServer()
	health.RegisterHealthServer(server, healthServer)
	go func() {
		err = server.Serve(listen)
		if err != nil {
			panic(fmt.Errorf("fatal error grpcserver serve failed: %v", err))
		}
	}()

	// ha dependent on dbmanager which is initialized by rt.Run
	logHa := ctrl.Log.WithName("HA")
	ha := highavailability.NewHa(logHa)
	if util.IsHAAvailable(characterType, workloadType) {
		if ha != nil {
			defer ha.ShutdownWithWait()
			go ha.Start()
		}
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	<-stop
}
