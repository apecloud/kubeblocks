/*
Copyright 2022.

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
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"time"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud/factory"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/agent"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	iptableswrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/iptables"
	netlinkwrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/netlink"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/network"
	procfswrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/procfs"

	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	zaplogfmt "github.com/sykesm/zap-logfmt"
	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	lb "github.com/apecloud/kubeblocks/controllers/loadbalancer"
	//+kubebuilder:scaffold:imports
)

const (
	appName      = "loadbalancer"
	RFC3339Mills = "2006-01-02T15:04:05.000"
)

var (
	enableDebug string
	scheme      = runtime.NewScheme()
	setupLog    = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	viper.SetConfigName("config")                          // name of config file (without extension)
	viper.SetConfigType("yaml")                            // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(fmt.Sprintf("/etc/%s/", appName))  // path to look for the config file in
	viper.AddConfigPath(fmt.Sprintf("$HOME/.%s", appName)) // call multiple times to add many search paths
	viper.AddConfigPath(".")                               // optionally look for config in the working directory
	viper.AutomaticEnv()
	_ = viper.BindEnv(enableDebug, "ENABLE_DEBUG")

	viper.SetDefault("CERT_DIR", "/tmp/k8s-webhook-server/serving-certs")
}

func main() {
	var metricsAddr string
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	configLog := uzap.NewProductionEncoderConfig()
	configLog.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format(RFC3339Mills))
	}
	logFmtEncoder := zaplogfmt.NewEncoder(configLog)
	// NOTES:
	// zap is "Blazing fast, structured, leveled logging in Go.", DON'T event try
	// to refactor this logging lib to anything else. Check FAQ - https://github.com/uber-go/zap/blob/master/FAQ.md
	logger := zap.New(zap.UseDevMode(true), zap.WriteTo(os.Stdout), zap.Encoder(logFmtEncoder))
	ctrl.SetLogger(logger)

	err := viper.ReadInConfig() // Find and read the config file
	if err == nil {             // Handle errors reading the config file
		setupLog.Info(fmt.Sprintf("config file: %s", viper.GetViper().ConfigFileUsed()))
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: probeAddr,
		CertDir:                viper.GetString("cert_dir"),
	})
	if err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}

	if strings.ToLower(enableDebug) == "true" {
		go pprofListening(logger)
	}

	cp, err := factory.NewProvider(cloud.ProviderAWS, logger)
	if err != nil {
		setupLog.Error(err, "Failed to initialize cloud provider")
		os.Exit(1)
	}

	ipt, err := iptableswrapper.NewIPTables()
	if err != nil {
		setupLog.Error(err, "Failed to init iptables")
		os.Exit(1)
	}
	nc, err := network.NewClient(logger, netlinkwrapper.NewNetLink(), ipt, procfswrapper.NewProcFS())
	if err != nil {
		setupLog.Error(err, "Failed to init network client")
		os.Exit(1)
	}

	em, err := agent.NewENIManager(logger, cp, nc)
	if err != nil {
		setupLog.Error(err, "Failed to init eni manager")
		os.Exit(1)
	}
	if err := em.Start(); err != nil {
		setupLog.Error(err, "Failed to start eni controller")
	}

	serviceController, err := lb.NewServiceController(logger, mgr.GetClient(), mgr.GetScheme(), mgr.GetEventRecorderFor("LoadBalancer"), em, cp, nc)
	if err != nil {
		setupLog.Error(err, "Failed to init service controller")
		os.Exit(1)
	}
	if err := serviceController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "Service")
		os.Exit(1)
	}

	endpointController, err := lb.NewEndpointController(logger, mgr.GetClient(), mgr.GetScheme(), mgr.GetEventRecorderFor("LoadBalancer"))
	if err != nil {
		setupLog.Error(err, "Failed to init endpoints controller")
		os.Exit(1)
	}
	if err := endpointController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "Endpoints")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Problem running manager")
		os.Exit(1)
	}
}

func pprofListening(logger logr.Logger) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	logger.Info("")
	_ = http.Serve(l, nil)
}
