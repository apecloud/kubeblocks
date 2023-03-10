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

// TODO assign multiple private ip from different subnets
// TODO create subnets instead of use host subnet
// TODO if mask of src address in policy rule little than 32 bit, causing routing problem
// TODO monitor security group / subnet / vpc changes
// TODO reuse pb.ENIMetadata
// TODO enable grpc auth, transport credentials
// TODO replace with k8s built-in grpc liveness/readiness probe when we can ensure k8s version > 1.23.0
// TODO define FloatingIP CRD
// TODO implement device plugin to report floating ip resources
// TODO move DescribeAllENIs from agent to controller
// TODO delete enis when node is deleted

package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	zaplogfmt "github.com/sykesm/zap-logfmt"
	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	// +kubebuilder:scaffold:imports

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/agent"
	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud/factory"
	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/config"
	lb "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/controllers"
	iptableswrapper "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/iptables"
	netlinkwrapper "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/netlink"
	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/network"
	procfswrapper "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/procfs"
	pb "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/protocol"
)

// added lease.coordination.k8s.io for leader election
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch

const (
	appName      = "loadbalancer"
	RFC3339Mills = "2006-01-02T15:04:05.000"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)
var (
	metricsAddr          string
	probeAddr            string
	enableLeaderElection bool
	mode                 string
	logger               logr.Logger
)

type OnLeaderAction struct {
	sc *lb.ServiceController
	ec *lb.EndpointController
	nm agent.NodeManager
}

func (l OnLeaderAction) NeedLeaderElection() bool {
	return true
}

func (l OnLeaderAction) Start(ctx context.Context) error {
	if err := l.nm.Start(ctx); err != nil {
		return err
	}
	if err := l.sc.Start(ctx); err != nil {
		return err
	}
	if err := l.ec.Start(ctx); err != nil {
		return err
	}
	return nil
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	viper.SetConfigName("config")                          // name of config file (without extension)
	viper.SetConfigType("yaml")                            // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(fmt.Sprintf("/etc/%s/", appName))  // path to look for the config file in
	viper.AddConfigPath(fmt.Sprintf("$HOME/.%s", appName)) // call multiple times to add many search paths
	viper.AddConfigPath(".")                               // optionally look for config in the working directory
	viper.AutomaticEnv()

	viper.SetDefault("CERT_DIR", "/tmp/k8s-webhook-server/serving-certs")
	configLog := uzap.NewProductionEncoderConfig()
	configLog.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format(RFC3339Mills))
	}
	logFmtEncoder := zaplogfmt.NewEncoder(configLog)
	// NOTES:
	// zap is "Blazing fast, structured, leveled logging in Go.", DON'T event try
	// to refactor this logging lib to anything else. Check FAQ - https://github.com/uber-go/zap/blob/master/FAQ.md
	logger = zap.New(zap.UseDevMode(true), zap.WriteTo(os.Stdout), zap.Encoder(logFmtEncoder))

	flag.StringVar(&mode, "mode", "agent", "The mode this binary is running as, can be either 'agent' or 'controller'")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. "+
		"Enabling this will ensure there is only one active controller manager.")
}

func RunController(logger logr.Logger) {
	ctrl.SetLogger(logger)

	// init config
	config.ReadConfig(setupLog)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: probeAddr,
		CertDir:                viper.GetString("cert_dir"),
		LeaderElection:         enableLeaderElection,
		// NOTES:
		// following LeaderElectionID is generated via hash/fnv (FNV-1 and FNV-1a), in
		// pattern of '{{ hashFNV .Repo }}.{{ .Domain }}', make sure regenerate this ID
		// if you have forked from this project template.
		LeaderElectionID: "002c317f.kubeblocks.io",

		// NOTES:
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}

	if config.EnableDebug {
		go pprofListening(logger)
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		setupLog.Error(err, "Failed to get incluster config")
		os.Exit(1)
	}
	// https://github.com/kubernetes-sigs/controller-runtime/issues/343
	// The controller manager provided client is designed to do the right thing for controllers by default (which is to read from caches, meaning that it's not strongly consistent)
	// We must use raw client to talk with apiserver
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "Failed to init k8s client")
		os.Exit(1)
	}

	cp, err := factory.NewProvider(config.CloudProvider, logger)
	if err != nil {
		setupLog.Error(err, "Failed to initialize cloud provider")
		os.Exit(1)
	}
	nm, err := agent.NewNodeManager(logger, config.RPCPort, cp, c)
	if err != nil {
		setupLog.Error(err, "Failed to init node manager")
		os.Exit(1)
	}
	sc, err := lb.NewServiceController(logger, c, mgr.GetScheme(), mgr.GetEventRecorderFor("LoadBalancer"), cp, nm)
	if err != nil {
		setupLog.Error(err, "Failed to init service controller")
		os.Exit(1)
	}
	if err := sc.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "Service")
		os.Exit(1)
	}

	ec, err := lb.NewEndpointController(logger, c, mgr.GetScheme(), mgr.GetEventRecorderFor("LoadBalancer"))
	if err != nil {
		setupLog.Error(err, "Failed to init endpoints controller")
		os.Exit(1)
	}
	if err := ec.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "Endpoints")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		os.Exit(1)
	}

	if err := mgr.Add(&OnLeaderAction{sc: sc, ec: ec, nm: nm}); err != nil {
		setupLog.Error(err, "Failed to add on leader action")
		os.Exit(1)
	}

	setupLog.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Problem running manager")
		os.Exit(1)
	}
}

func RunAgent(logger logr.Logger) {
	flag.Parse()

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

	cp, err := factory.NewProvider(config.CloudProvider, logger)
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

func pprofListening(logger logr.Logger) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	logger.Info("Starting pprof", "addr", l.Addr().String())
	_ = http.Serve(l, nil)
}

func main() {
	flag.Parse()

	switch mode {
	case "agent":
		RunAgent(logger)
	case "controller":
		RunController(logger)
	default:
		logger.Error(fmt.Errorf("unknown mode %s", mode), "")
		os.Exit(1)
	}
}
