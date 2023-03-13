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
	"os"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	appscontrollers "github.com/apecloud/kubeblocks/controllers/apps"
	dataprotectioncontrollers "github.com/apecloud/kubeblocks/controllers/dataprotection"
	extensionscontrollers "github.com/apecloud/kubeblocks/controllers/extensions"

	// +kubebuilder:scaffold:imports

	discoverycli "k8s.io/client-go/discovery"

	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/controllers/apps/configuration"
	k8scorecontrollers "github.com/apecloud/kubeblocks/controllers/k8score"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/webhook"
)

// added lease.coordination.k8s.io for leader election
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch

const (
	appName = "kubeblocks"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(appsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(dataprotectionv1alpha1.AddToScheme(scheme))
	utilruntime.Must(snapshotv1.AddToScheme(scheme))
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	viper.SetConfigName("config")                          // name of config file (without extension)
	viper.SetConfigType("yaml")                            // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(fmt.Sprintf("/etc/%s/", appName))  // path to look for the config file in
	viper.AddConfigPath(fmt.Sprintf("$HOME/.%s", appName)) // call multiple times to add many search paths
	viper.AddConfigPath(".")                               // optionally look for config in the working directory
	viper.AutomaticEnv()

	viper.SetDefault("CERT_DIR", "/tmp/k8s-webhook-server/serving-certs")
	viper.SetDefault("VOLUMESNAPSHOT", true)
	viper.SetDefault("KUBEBLOCKS_IMAGE", "apecloud/kubeblocks:latest")
	viper.SetDefault("PROBE_SERVICE_HTTP_PORT", 3501)
	viper.SetDefault("PROBE_SERVICE_GRPC_PORT", 50001)
	viper.SetDefault("PROBE_SERVICE_LOG_LEVEL", "info")
	viper.SetDefault("KUBEBLOCKS_SERVICEACCOUNT_NAME", "kubeblocks")
	viper.SetDefault("CM_NAMESPACE", "default")
}

type flagName string

const (
	probeAddrFlagKey   flagName = "health-probe-bind-address"
	metricsAddrFlagKey flagName = "metrics-bind-address"
	leaderElectFlagKey flagName = "leader-elect"
)

func (r flagName) String() string {
	return string(r)
}

func (r flagName) viperName() string {
	return strings.ReplaceAll(r.String(), "-", "_")
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.String(metricsAddrFlagKey.String(), ":8080", "The address the metric endpoint binds to.")
	flag.String(probeAddrFlagKey.String(), ":8081", "The address the probe endpoint binds to.")
	flag.Bool(leaderElectFlagKey.String(), false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

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
		setupLog.Error(err, "unable able to bind flags")
		os.Exit(1)
	}

	// NOTES:
	// zap is "Blazing fast, structured, leveled logging in Go.", DON'T event try
	// to refactor this logging lib to anything else. Check FAQ - https://github.com/uber-go/zap/blob/master/FAQ.md
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		setupLog.Info("unable read in config, errors ignored")
	}
	setupLog.Info(fmt.Sprintf("config file: %s", viper.GetViper().ConfigFileUsed()))

	metricsAddr = viper.GetString(metricsAddrFlagKey.viperName())
	probeAddr = viper.GetString(probeAddrFlagKey.viperName())
	enableLeaderElection = viper.GetBool(leaderElectFlagKey.viperName())

	setupLog.Info(fmt.Sprintf("config settings: %v", viper.AllSettings()))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		// NOTES:
		// following LeaderElectionID is generated via hash/fnv (FNV-1 and FNV-1a), in
		// pattern of '{{ hashFNV .Repo }}.{{ .Domain }}', make sure regenerate this ID
		// if you have forked from this project template.
		LeaderElectionID: "001c317f.kubeblocks.io",

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

		CertDir:               viper.GetString("cert_dir"),
		ClientDisableCacheFor: intctrlutil.GetUncachedObjects(),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&appscontrollers.ClusterReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("cluster-controller"),
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

	if err = (&dataprotectioncontrollers.BackupToolReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("backup-tool-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BackupTool")
		os.Exit(1)
	}

	if err = (&dataprotectioncontrollers.BackupPolicyReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("backup-policy-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BackupPolicy")
		os.Exit(1)
	}

	if err = (&dataprotectioncontrollers.CronJobReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("cronjob-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CronJob")
		os.Exit(1)
	}

	if err = (&dataprotectioncontrollers.BackupReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("backup-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Backup")
		os.Exit(1)
	}

	if err = (&dataprotectioncontrollers.RestoreJobReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("restore-job-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RestoreJob")
		os.Exit(1)
	}

	if err = (&dataprotectioncontrollers.BackupPolicyTemplateReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BackupPolicyTemplate")
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
	if !viper.GetBool("DISABLE_ADDON_CTRLER") {
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
	// +kubebuilder:scaffold:builder

	if err = (&configuration.ReconfigureRequestReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("reconfigure-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ReconfigureRequest")
		os.Exit(1)
	}

	if err = (&appscontrollers.SystemAccountReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("system-account-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SystemAccount")
		os.Exit(1)
	}

	if err = (&k8scorecontrollers.EventReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("event-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Event")
		os.Exit(1)
	}

	if err = (&k8scorecontrollers.PersistentVolumeClaimReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("pvc-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PersistentVolumeClaim")
		os.Exit(1)
	}

	if err = (&components.StatefulSetReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("stateful-set-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "StatefulSet")
		os.Exit(1)
	}

	if err = (&components.DeploymentReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("deployment-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Deployment")
		os.Exit(1)
	}

	if viper.GetBool("enable_webhooks") {

		appsv1alpha1.RegisterWebhookManager(mgr)

		if err = (&appsv1alpha1.Cluster{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Cluster")
			os.Exit(1)
		}

		if err = (&appsv1alpha1.ClusterDefinition{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "ClusterDefinition")
			os.Exit(1)
		}

		if err = (&appsv1alpha1.ClusterVersion{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "ClusterVersion")
			os.Exit(1)
		}

		if err = (&appsv1alpha1.OpsRequest{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpsRequest")
			os.Exit(1)
		}

		if err = webhook.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to setup webhook")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
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
	viper.SetDefault("_KUBE_SERVER_INFO", *ver)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
