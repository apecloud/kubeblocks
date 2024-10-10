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
	"runtime"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/fsnotify/fsnotify"
	snapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	discoverycli "k8s.io/client-go/discovery"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	// +kubebuilder:scaffold:imports

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dpcontrollers "github.com/apecloud/kubeblocks/controllers/dataprotection"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/metrics"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// added lease.coordination.k8s.io for leader election
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch

const (
	appName = "kubeblocks"
)

type flagName string

const (
	probeAddrFlagKey     flagName = "health-probe-bind-address"
	metricsAddrFlagKey   flagName = "metrics-bind-address"
	leaderElectFlagKey   flagName = "leader-elect"
	leaderElectIDFlagKey flagName = "leader-elect-id"

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
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(appsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(dpv1alpha1.AddToScheme(scheme))
	utilruntime.Must(snapshotv1.AddToScheme(scheme))
	utilruntime.Must(snapshotv1beta1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	viper.SetConfigName("config")                          // name of config file (without extension)
	viper.SetConfigType("yaml")                            // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(fmt.Sprintf("/etc/%s/", appName))  // path to look for the config file in
	viper.AddConfigPath(fmt.Sprintf("$HOME/.%s", appName)) // call multiple times to append search path
	viper.AddConfigPath(".")                               // optionally look for config in the working directory
	viper.AutomaticEnv()

	viper.SetDefault(constant.CfgKeyCtrlrReconcileRetryDurationMS, 1000)
	viper.SetDefault("CERT_DIR", "/tmp/k8s-webhook-server/serving-certs")
	viper.SetDefault("VOLUMESNAPSHOT_API_BETA", false)
	viper.SetDefault(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	viper.SetDefault("KUBEBLOCKS_SERVICEACCOUNT_NAME", "kubeblocks")
	viper.SetDefault(constant.CfgKeyCtrlrMgrNS, "default")
	viper.SetDefault(constant.KubernetesClusterDomainEnv, constant.DefaultDNSDomain)
	viper.SetDefault(dptypes.CfgKeyGCFrequencySeconds, dptypes.DefaultGCFrequencySeconds)
	viper.SetDefault(dptypes.CfgKeyWorkerServiceAccountName, "kubeblocks-dataprotection-worker")
	viper.SetDefault(dptypes.CfgKeyExecWorkerServiceAccountName, "kubeblocks-dataprotection-exec-worker")
	viper.SetDefault(dptypes.CfgKeyWorkerServiceAccountAnnotations, "{}")
	viper.SetDefault(dptypes.CfgKeyWorkerClusterRoleName, "kubeblocks-dataprotection-worker-role")
	viper.SetDefault(dptypes.CfgDataProtectionReconcileWorkers, runtime.NumCPU())
}

func main() {
	var (
		metricsAddr                  string
		enableLeaderElection         bool
		enableLeaderElectionID       string
		probeAddr                    string
		multiClusterKubeConfig       string
		multiClusterContexts         string
		multiClusterContextsDisabled string
		userAgent                    string
	)

	flag.String(metricsAddrFlagKey.String(), ":8080", "The address the metric endpoint binds to.")
	flag.String(probeAddrFlagKey.String(), ":8081", "The address the probe endpoint binds to.")
	flag.Bool(leaderElectFlagKey.String(), false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.String(leaderElectIDFlagKey.String(), "abd03fda",
		"The leader election ID prefix for controller manager. "+
			"This ID must be unique to controller manager.")

	flag.String(multiClusterKubeConfigFlagKey.String(), "", "Paths to the kubeconfig for multi-cluster accessing.")
	flag.String(multiClusterContextsFlagKey.String(), "", "Kube contexts the manager will talk to.")
	flag.String(multiClusterContextsDisabledFlagKey.String(), "", "Kube contexts that mark as disabled.")

	flag.String(userAgentFlagKey.String(), "", "User agent of the operator.")

	flag.String(constant.ManagedNamespacesFlag, "",
		"The namespaces that the operator will manage, multiple namespaces are separated by commas.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

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

	// NOTES:
	// zap is "Blazing fast, structured, leveled logging in Go.", DON'T event try
	// to refactor this logging lib to anything else. Check FAQ - https://github.com/uber-go/zap/blob/master/FAQ.md
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Find and read the config file
	if err := viper.ReadInConfig(); err != nil { // Handle errors reading the config file
		setupLog.Info("unable to read in config, errors ignored")
	}
	setupLog.Info(fmt.Sprintf("config file: %s", viper.GetViper().ConfigFileUsed()))
	viper.OnConfigChange(func(e fsnotify.Event) {
		setupLog.Info(fmt.Sprintf("config file changed: %s", e.Name))
	})
	viper.WatchConfig()

	metricsAddr = viper.GetString(metricsAddrFlagKey.viperName())
	probeAddr = viper.GetString(probeAddrFlagKey.viperName())
	enableLeaderElection = viper.GetBool(leaderElectFlagKey.viperName())
	enableLeaderElectionID = viper.GetString(leaderElectIDFlagKey.viperName())
	multiClusterKubeConfig = viper.GetString(multiClusterKubeConfigFlagKey.viperName())
	multiClusterContexts = viper.GetString(multiClusterContextsFlagKey.viperName())
	multiClusterContextsDisabled = viper.GetString(multiClusterContextsDisabledFlagKey.viperName())
	userAgent = viper.GetString(userAgentFlagKey.viperName())

	setupLog.Info(fmt.Sprintf("config settings: %v", viper.AllSettings()))
	if err := validateRequiredToParseConfigs(); err != nil {
		setupLog.Error(err, "config value error")
		os.Exit(1)
	}

	managedNamespaces := viper.GetString(strings.ReplaceAll(constant.ManagedNamespacesFlag, "-", "_"))
	if len(managedNamespaces) > 0 {
		setupLog.Info(fmt.Sprintf("managed namespaces: %s", managedNamespaces))
	}
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

	cli, err := discoverycli.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to create discovery client")
		os.Exit(1)
	}

	ver, err := cli.ServerVersion()
	if err != nil {
		setupLog.Error(err, "unable to discover version info")
		os.Exit(1)
	}
	viper.SetDefault(constant.CfgKeyServerInfo, *ver)

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

	if err = (&dpcontrollers.ActionSetReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("actionset-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ActionSet")
		os.Exit(1)
	}

	if err = (&dpcontrollers.BackupReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		Recorder:   mgr.GetEventRecorderFor("backup-controller"),
		RestConfig: mgr.GetConfig(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Backup")
		os.Exit(1)
	}

	if err = (&dpcontrollers.RestoreReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("restore-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Restore")
		os.Exit(1)
	}

	if err = (&dpcontrollers.VolumePopulatorReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("volume-populator-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VolumePopulator")
		os.Exit(1)
	}

	if err = (&dpcontrollers.BackupPolicyReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("backup-policy-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BackupPolicy")
		os.Exit(1)
	}

	if err = (&dpcontrollers.BackupScheduleReconciler{
		Client:   dputils.NewCompatClient(mgr.GetClient()),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("backup-schedule-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BackupSchedule")
		os.Exit(1)
	}

	if err = (&dpcontrollers.BackupPolicyTemplateReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("backup-policy-template-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BackupPolicyTemplate")
		os.Exit(1)
	}

	if err = (&dpcontrollers.BackupRepoReconciler{
		Client:          client,
		Scheme:          mgr.GetScheme(),
		Recorder:        mgr.GetEventRecorderFor("backup-repo-controller"),
		RestConfig:      mgr.GetConfig(),
		MultiClusterMgr: multiClusterMgr,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BackupRepo")
		os.Exit(1)
	}

	if err = (&dpcontrollers.StorageProviderReconciler{
		Client:          client,
		Scheme:          mgr.GetScheme(),
		Recorder:        mgr.GetEventRecorderFor("storage-provider-controller"),
		MultiClusterMgr: multiClusterMgr,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "StorageProvider")
		os.Exit(1)
	}

	if err = (&dpcontrollers.LogCollectionReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		Recorder:   mgr.GetEventRecorderFor("log-collection-controller"),
		RestConfig: mgr.GetConfig(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "logCollectionController")
		os.Exit(1)
	}

	if err = dpcontrollers.NewGCReconciler(mgr).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GarbageCollection")
		os.Exit(1)
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

func (r flagName) String() string {
	return string(r)
}

func (r flagName) viperName() string {
	return strings.ReplaceAll(r.String(), "-", "_")
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

	validateWorkerServiceAccountAnnotations := func(val string) error {
		if val == "" {
			return nil
		}
		annotations := map[string]string{}
		return json.Unmarshal([]byte(val), &annotations)
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
	if err := validateWorkerServiceAccountAnnotations(viper.GetString(dptypes.CfgKeyWorkerServiceAccountAnnotations)); err != nil {
		return err
	}

	if imagePullSecrets := viper.GetString(constant.KBImagePullSecrets); imagePullSecrets != "" {
		secrets := make([]corev1.LocalObjectReference, 0)
		if err := json.Unmarshal([]byte(imagePullSecrets), &secrets); err != nil {
			return err
		}
	}

	return nil
}
