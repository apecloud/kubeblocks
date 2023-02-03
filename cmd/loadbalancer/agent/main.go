/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/go-logr/zapr"
	"github.com/spf13/viper"
	zaplogfmt "github.com/sykesm/zap-logfmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud/factory"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/config"
	iptableswrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/iptables"
	netlinkwrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/netlink"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/network"
	procfswrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/procfs"
	pb "github.com/apecloud/kubeblocks/internal/loadbalancer/protocol"
)

const (
	RFC3339Mills = "2006-01-02T15:04:05.000"
	appName      = "loadbalancer-agent"
)

func init() {
	viper.SetConfigName("config")                          // name of config file (without extension)
	viper.SetConfigType("yaml")                            // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(fmt.Sprintf("/etc/%s/", appName))  // path to look for the config file in
	viper.AddConfigPath(fmt.Sprintf("$HOME/.%s", appName)) // call multiple times to add many search paths
	viper.AddConfigPath(".")                               // optionally look for config in the working directory
	viper.AutomaticEnv()

	viper.SetDefault("CERT_DIR", "/tmp/k8s-webhook-server/serving-certs")
}

func main() {
	var metricsAddr string
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.Parse()

	configLog := zap.NewProductionEncoderConfig()
	configLog.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format(RFC3339Mills))
	}
	// NOTES:
	// zap is "Blazing fast, structured, leveled logging in Go.", DON'T event try
	// to refactor this logging lib to anything else. Check FAQ - https://github.com/uber-go/zap/blob/master/FAQ.md
	zaplog := zap.New(zapcore.NewCore(zaplogfmt.NewEncoder(configLog), os.Stdout, zapcore.DebugLevel))
	logger := zapr.NewLogger(zaplog)

	// init config
	config.ReadConfig(logger)

	ipt, err := iptableswrapper.NewIPTables()
	if err != nil {
		logger.Error(err, "Failed to init iptables")
		os.Exit(1)
	}
	nc, err := network.NewClient(logger, netlinkwrapper.NewNetLink(), ipt, procfswrapper.NewProcFS())
	if err != nil {
		logger.Error(err, "Failed to init network client")
		os.Exit(1)
	}

	cp, err := factory.NewProvider(cloud.ProviderAWS, logger)
	if err != nil {
		logger.Error(err, "Failed to initialize cloud provider")
		os.Exit(1)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", config.HostIP, config.RPCPort))
	if err != nil {
		logger.Error(err, "Failed to listen")
		os.Exit(1)
	}

	server := grpc.NewServer()
	proxy := &Proxy{nc: nc, cp: cp}
	pb.RegisterNodeServer(server, proxy)
	grpc_health_v1.RegisterHealthServer(server, proxy)
	logger.Info("Exit", "err", server.Serve(lis))
}
