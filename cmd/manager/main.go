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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"github.com/fsnotify/fsnotify"
	snapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	discoverycli "k8s.io/client-go/discovery"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	// +kubebuilder:scaffold:imports

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	experimentalv1alpha1 "github.com/apecloud/kubeblocks/apis/experimental/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	storagev1alpha1 "github.com/apecloud/kubeblocks/apis/storage/v1alpha1"
	"github.com/apecloud/kubeblocks/apis/workloads/legacy"
	workloadsv1alpha1 "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	appscontrollers "github.com/apecloud/kubeblocks/controllers/apps"
	"github.com/apecloud/kubeblocks/controllers/apps/configuration"
	experimentalcontrollers "github.com/apecloud/kubeblocks/controllers/experimental"
	extensionscontrollers "github.com/apecloud/kubeblocks/controllers/extensions"
	k8scorecontrollers "github.com/apecloud/kubeblocks/controllers/k8score"
	workloadscontrollers "github.com/apecloud/kubeblocks/controllers/workloads"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/metrics"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// added lease.coordination.k8s.io for leader election
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch

const (
	appName = "kubeblocks"

	probeAddrFlagKey     flagName = "health-probe-bind-address"
	metricsAddrFlagKey   flagName = "metrics-bind-address"
	leaderElectFlagKey   flagName = "leader-elect"
	leaderElectIDFlagKey flagName = "leader-elect-id"

	// switch flags key for API groups
	appsFlagKey         flagName = "apps"
	extensionsFlagKey   flagName = "extensions"
	workloadsFlagKey    flagName = "workloads"
	experimentalFlagKey flagName = "experimental"

	multiClusterKubeConfigFlagKey       flagName = "multi-cluster-kubeconfig"
	multiClusterContextsFlagKey         flagName = "multi-cluster-contexts"
	multiClusterContextsDisabledFlagKey flagName = "multi-cluster-contexts-disabled"

	userAgentFlagKey flagName = "user-agent"
)

var (
	scheme   = k8sruntime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(appsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(dpv1alpha1.AddToScheme(scheme))
	utilruntime.Must(snapshotv1.AddToScheme(scheme))
	utilruntime.Must(snapshotv1beta1.AddToScheme(scheme))
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(workloadsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(storagev1alpha1.AddToScheme(scheme))
	utilruntime.Must(appsv1beta1.AddToScheme(scheme))
	utilruntime.Must(legacy.AddToScheme(scheme))
	utilruntime.Must(apiextv1.AddToScheme(scheme))
	utilruntime.Must(experimentalv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	viper.SetConfigName("config")                          // name of config file (without extension)
	viper.SetConfigType("yaml")                            // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(fmt.Sprintf("/etc/%s/", appName))  // path to look for the config file in
	viper.AddConfigPath(fmt.Sprintf("$HOME/.%s", appName)) // call multiple times to append search path
	viper.AddConfigPath(".")                               // optionally look for config in the working directory
	viper.AutomaticEnv()

	viper.SetDefault(constant.CfgKeyCtrlrReconcileRetryDurationMS, 1000)
	viper.SetDefault("CERT_DIR", "/tmp/k8s-webhook-server/serving-certs")
	viper.SetDefault(constant.EnableRBACManager, true)
	viper.SetDefault("VOLUMESNAPSHOT_API_BETA", false)
	viper.SetDefault(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	viper.SetDefault(constant.KBEnvLorryHTTPPort, 3501)
	viper.SetDefault(constant.KBEnvLorryGRPCPort, 50001)
	viper.SetDefault(constant.KBEnvLorryLogLevel, "info")
	viper.SetDefault("KUBEBLOCKS_SERVICEACCOUNT_NAME", "kubeblocks")
	viper.SetDefault(constant.ConfigManagerGPRCPortEnv, 9901)
	viper.SetDefault("CONFIG_MANAGER_LOG_LEVEL", "info")
	viper.SetDefault(constant.CfgKeyCtrlrMgrNS, "default")
	viper.SetDefault(constant.CfgHostPortConfigMapName, "kubeblocks-host-ports")
	viper.SetDefault(constant.CfgHostPortIncludeRanges, "1025-65536")
	viper.SetDefault(constant.CfgHostPortExcludeRanges, "6443,10250,10257,10259,2379-2380,30000-32767")
	viper.SetDefault(constant.KBDataScriptClientsImage, "apecloud/kubeblocks-datascript:latest")
	viper.SetDefault(constant.KubernetesClusterDomainEnv, constant.DefaultDNSDomain)
	viper.SetDefault(instanceset.MaxPlainRevisionCount, 1024)
	viper.SetDefault(instanceset.FeatureGateIgnorePodVerticalScaling, false)
	viper.SetDefault(intctrlutil.FeatureGateEnableRuntimeMetrics, false)
	viper.SetDefault(constant.CfgKBReconcileWorkers, 8)
	viper.SetDefault(constant.FeatureGateIgnoreConfigTemplateDefaultMode, false)
}

type flagName string

func (r flagName) String() string {
	return string(r)
}

func (r flagName) viperName() string {
	return strings.ReplaceAll(r.String(), "-", "_")
}

func setupFlags() {
	flag.String(metricsAddrFlagKey.String(), ":8080", "The address the metric endpoint binds to.")
	flag.String(probeAddrFlagKey.String(), ":8081", "The address the probe endpoint binds to.")
	flag.Bool(leaderElectFlagKey.String(), false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.String(leaderElectIDFlagKey.String(), "001c317f",
		"The leader election ID prefix for controller manager. "+
			"This ID must be unique to controller manager.")

	flag.Bool(appsFlagKey.String(), true,
		"Enable the apps controller manager.")
	flag.Bool(extensionsFlagKey.String(), true,
		"Enable the extensions controller manager.")
	flag.Bool(workloadsFlagKey.String(), true,
		"Enable the workloads controller manager.")
	flag.Bool(experimentalFlagKey.String(), false,
		"Enable the experimental controller manager.")

	flag.String(multiClusterKubeConfigFlagKey.String(), "", "Paths to the kubeconfig for multi-cluster accessing.")
	flag.String(multiClusterContextsFlagKey.String(), "", "Kube contexts the manager will talk to.")
	flag.String(multiClusterContextsDisabledFlagKey.String(), "", "Kube contexts that mark as disabled.")

	flag.String(constant.ManagedNamespacesFlag, "",
		"The namespaces that the operator will manage, multiple namespaces are separated by commas.")

	flag.String(userAgentFlagKey.String(), "", "User agent of the operator.")

	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	// NOTES:
	// zap is "Blazing fast, structured, leveled logging in Go.", DON'T event try
	// to refactor this logging lib to anything else. Check FAQ - https://github.com/uber-go/zap/blob/master/FAQ.md
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// set normalizeFunc to replace flag name to viper name
	normalizeFunc := pflag.CommandLine.GetNormalizeFunc()
	pflag.CommandLine.SetNormalizeFunc(func(fs *pflag.FlagSet, name string) pflag.NormalizedName {
		result := normalizeFunc(fs, name)
		name = strings.ReplaceAll(string(result), "-", "_")
		return pflag.NormalizedName(name)
	})

	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		setupLog.Error(err, "unable to bind flags")
		os.Exit(1)
	}
}

func validateRequiredToParseConfigs() error {
	validateTolerations := func(val string) error {
		if val == "" {
			return nil
		}
		var tolerations []corev1.Toleration
		return json.Unmarshal([]byte(val), &tolerations)
	}

	validateAffinity := func(val string) error {
		if val == "" {
			return nil
		}
		affinity := corev1.Affinity{}
		return json.Unmarshal([]byte(val), &affinity)
	}

	if jobTTL := viper.GetString(constant.CfgKeyAddonJobTTL); jobTTL != "" {
		if _, err := time.ParseDuration(jobTTL); err != nil {
			return err
		}
	}
	if err := validateTolerations(viper.GetString(constant.CfgKeyCtrlrMgrTolerations)); err != nil {
		return err
	}
	if err := validateAffinity(viper.GetString(constant.CfgKeyCtrlrMgrAffinity)); err != nil {
		return err
	}
	if cmNodeSelector := viper.GetString(constant.CfgKeyCtrlrMgrNodeSelector); cmNodeSelector != "" {
		nodeSelector := map[string]string{}
		if err := json.Unmarshal([]byte(cmNodeSelector), &nodeSelector); err != nil {
			return err
		}
	}
	if err := validateTolerations(viper.GetString(constant.CfgKeyDataPlaneTolerations)); err != nil {
		return err
	}
	if err := validateAffinity(viper.GetString(constant.CfgKeyDataPlaneAffinity)); err != nil {
		return err
	}
	return nil
}

func main() {
	var (
		metricsAddr                  string
		probeAddr                    string
		enableLeaderElection         bool
		enableLeaderElectionID       string
		multiClusterKubeConfig       string
		multiClusterContexts         string
		multiClusterContextsDisabled string
		userAgent                    string
		err                          error
	)

	setupFlags()

	// Find and read the config file
	if err := viper.ReadInConfig(); err != nil { // Handle errors reading the config file
		setupLog.Info("unable to read in config, errors ignored")
	}
	setupLog.Info(fmt.Sprintf("config file: %s", viper.GetViper().ConfigFileUsed()))
	viper.OnConfigChange(func(e fsnotify.Event) {
		setupLog.Info(fmt.Sprintf("config file changed: %s", e.Name))
	})
	viper.WatchConfig()

	setupLog.Info(fmt.Sprintf("config settings: %v", viper.AllSettings()))
	if err := validateRequiredToParseConfigs(); err != nil {
		setupLog.Error(err, "config value error")
		os.Exit(1)
	}

	managedNamespaces := viper.GetString(strings.ReplaceAll(constant.ManagedNamespacesFlag, "-", "_"))
	if len(managedNamespaces) > 0 {
		setupLog.Info(fmt.Sprintf("managed namespaces: %s", managedNamespaces))
	}

	metricsAddr = viper.GetString(metricsAddrFlagKey.viperName())
	probeAddr = viper.GetString(probeAddrFlagKey.viperName())
	enableLeaderElection = viper.GetBool(leaderElectFlagKey.viperName())
	enableLeaderElectionID = viper.GetString(leaderElectIDFlagKey.viperName())
	multiClusterKubeConfig = viper.GetString(multiClusterKubeConfigFlagKey.viperName())
	multiClusterContexts = viper.GetString(multiClusterContextsFlagKey.viperName())
	multiClusterContextsDisabled = viper.GetString(multiClusterContextsDisabledFlagKey.viperName())

	userAgent = viper.GetString(userAgentFlagKey.viperName())

	setupLog.Info("golang runtime metrics.", "featureGate", intctrlutil.EnabledRuntimeMetrics())
	mgr, err := ctrl.NewManager(intctrlutil.GeKubeRestConfig(userAgent), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress:   metricsAddr,
			ExtraHandlers: metrics.RuntimeMetric(),
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		// NOTES:
		// following LeaderElectionID is generated via hash/fnv (FNV-1 and FNV-1a), in
		// pattern of '{{ hashFNV .Repo }}.{{ .Domain }}', make sure regenerate this ID
		// if you have forked from this project template.
		LeaderElectionID: enableLeaderElectionID + ".kubeblocks.io",

		// NOTES:
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader doesn't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or intending to do any operation such as performing cleanups
		// after the manager stops then its usage might be unsafe.
		LeaderElectionReleaseOnCancel: true,

		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    9443,
			CertDir: viper.GetString("cert_dir"),
		}),
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: intctrlutil.GetUncachedObjects(),
			},
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// multi-cluster manager for all data-plane k8s
	multiClusterMgr, err := multicluster.Setup(mgr.GetScheme(), mgr.GetConfig(), mgr.GetClient(),
		multiClusterKubeConfig, multiClusterContexts, multiClusterContextsDisabled)
	if err != nil {
		setupLog.Error(err, "unable to setup multi-cluster manager")
		os.Exit(1)
	}

	client := mgr.GetClient()
	if multiClusterMgr != nil {
		client = multiClusterMgr.GetClient()
	}

	if err := intctrlutil.InitHostPortManager(mgr.GetClient()); err != nil {
		setupLog.Error(err, "unable to init port manager")
		os.Exit(1)
	}

	if viper.GetBool(appsFlagKey.viperName()) {
		if err = (&appscontrollers.ClusterReconciler{
			Client:          client,
			Scheme:          mgr.GetScheme(),
			Recorder:        mgr.GetEventRecorderFor("cluster-controller"),
			MultiClusterMgr: multiClusterMgr,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Cluster")
			os.Exit(1)
		}

		if err = (&appscontrollers.ClusterDefinitionReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("cluster-definition-controller"),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ClusterDefinition")
			os.Exit(1)
		}

		if err = (&appscontrollers.ClusterVersionReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("cluster-version-controller"),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ClusterVersion")
			os.Exit(1)
		}

		if err = (&appscontrollers.ComponentReconciler{
			Client:   client,
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("component-controller"),
		}).SetupWithManager(mgr, multiClusterMgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Component")
			os.Exit(1)
		}

		if err = (&appscontrollers.ComponentDefinitionReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("component-definition-controller"),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ComponentDefinition")
			os.Exit(1)
		}

		if err = (&appscontrollers.ComponentVersionReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("component-version-controller"),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ComponentVersion")
			os.Exit(1)
		}

		if err = (&appscontrollers.OpsDefinitionReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("ops-definition-controller"),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "OpsDefinition")
			os.Exit(1)
		}

		if err = (&appscontrollers.OpsRequestReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("ops-request-controller"),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "OpsRequest")
			os.Exit(1)
		}

		if err = (&configuration.ConfigConstraintReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("config-constraint-controller"),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ConfigConstraint")
			os.Exit(1)
		}

		if err = (&configuration.ReconfigureReconciler{
			Client:   client,
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("reconfigure-controller"),
		}).SetupWithManager(mgr, multiClusterMgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ReconfigureRequest")
			os.Exit(1)
		}

		if err = (&configuration.ConfigurationReconciler{
			Client:   client,
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("configuration-controller"),
		}).SetupWithManager(mgr, multiClusterMgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Configuration")
			os.Exit(1)
		}

		if err = (&k8scorecontrollers.EventReconciler{
			Client:   client,
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("event-controller"),
		}).SetupWithManager(mgr, multiClusterMgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Event")
			os.Exit(1)
		}

		if err = (&appscontrollers.ServiceDescriptorReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("service-descriptor-controller"),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ServiceDescriptor")
			os.Exit(1)
		}

		if err = (&appscontrollers.BackupPolicyTemplateReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("backup-policy-template-controller"),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "BackupPolicyTemplate")
			os.Exit(1)
		}
	}

	if viper.GetBool(extensionsFlagKey.viperName()) {
		if err = (&extensionscontrollers.AddonReconciler{
			Client:     mgr.GetClient(),
			Scheme:     mgr.GetScheme(),
			Recorder:   mgr.GetEventRecorderFor("addon-controller"),
			RestConfig: mgr.GetConfig(),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Addon")
			os.Exit(1)
		}
	}

	if viper.GetBool(workloadsFlagKey.viperName()) {
		if err = (&workloadscontrollers.InstanceSetReconciler{
			Client:   client,
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("instance-set-controller"),
		}).SetupWithManager(mgr, multiClusterMgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "InstanceSet")
			os.Exit(1)
		}
	}

	if viper.GetBool(experimentalFlagKey.viperName()) {
		if err = (&experimentalcontrollers.NodeCountScalerReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("node-count-scaler-controller"),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "NodeCountScaler")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	discoveryClient, err := discoverycli.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to create discovery client")
		os.Exit(1)
	}

	ver, err := discoveryClient.ServerVersion()
	if err != nil {
		setupLog.Error(err, "unable to discover version info")
		os.Exit(1)
	}
	viper.SetDefault(constant.CfgKeyServerInfo, *ver)

	setupLog.Info("starting manager")
	if multiClusterMgr != nil {
		if err := multiClusterMgr.Bind(mgr); err != nil {
			setupLog.Error(err, "failed to bind multi-cluster manager to manager")
			os.Exit(1)
		}
	}
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
