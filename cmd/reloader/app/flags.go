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

package app

import (
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/container"
)

type NotifyEventType int

const (
	UnixSignal NotifyEventType = iota // "signal"
	WebHook                           // "http"
	ShellTool                         // "exec"
	SQL                               // "sql"
	TPLScript                         // "tpl"
)

const (
	configManagerDefaultPort = 9901
	configPodIPEnvName       = "CONFIG_MANAGER_POD_IP"
	localhostAddress         = "127.0.0.1"
)

var allNotifyType = map[NotifyEventType]appsv1alpha1.CfgReloadType{
	UnixSignal: appsv1alpha1.UnixSignalType,
	ShellTool:  appsv1alpha1.ShellType,
	TPLScript:  appsv1alpha1.TPLScriptType,
}

func init() {
	if err := viper.BindEnv(configPodIPEnvName); err != nil {
		os.Exit(-2)
	}
	// viper.AutomaticEnv()
	viper.SetDefault(configPodIPEnvName, localhostAddress)
}

func (f *NotifyEventType) Type() string {
	return "notifyType"
}

func (f *NotifyEventType) Set(val string) error {
	for key, value := range allNotifyType {
		if val == string(value) {
			*f = key
			return nil
		}
	}
	return cfgcore.MakeError("not support type[%s], require list: [%v]", val, allNotifyType)
}

func (f *NotifyEventType) String() string {
	reloadType, ok := allNotifyType[*f]
	if !ok {
		return ""
	}
	return string(reloadType)
}

type ReconfigureServiceOptions struct {
	GrpcPort int
	PodIP    string

	// EnableRemoteOnlineUpdate enable remote online update
	RemoteOnlineUpdateEnable bool
	// EnableContainerRuntime enable container runtime
	ContainerRuntimeEnable bool

	DebugMode        bool
	ContainerRuntime cfgutil.CRIType
	RuntimeEndpoint  string
}

type VolumeWatcherOpts struct {
	VolumeDirs []string

	// fileRegex watch file regex
	FileRegex string

	// ProcessName: program name
	ProcessName string

	// Signal is valid for UnixSignal
	Signal appsv1alpha1.SignalType

	// Exec command for reload
	Command string

	// Exec command for reload
	TPLConfig       string
	BackupPath      string
	FormatterConfig *appsv1alpha1.FormatterConfig
	TPLScriptPath   string

	LogLevel       string
	NotifyHandType NotifyEventType

	ServiceOpt ReconfigureServiceOptions
}

func NewVolumeWatcherOpts() *VolumeWatcherOpts {
	return &VolumeWatcherOpts{
		// for reconfigure options
		ServiceOpt: ReconfigureServiceOptions{
			GrpcPort:                 configManagerDefaultPort,
			PodIP:                    viper.GetString(configPodIPEnvName),
			ContainerRuntime:         cfgutil.AutoType,
			DebugMode:                false,
			ContainerRuntimeEnable:   false,
			RemoteOnlineUpdateEnable: false,
		},
		// for configmap watch
		NotifyHandType: UnixSignal,
		Signal:         appsv1alpha1.SIGHUP,
		LogLevel:       "info",
	}
}

func InstallFlags(flags *pflag.FlagSet, opt *VolumeWatcherOpts) {
	flags.StringArrayVar(&opt.VolumeDirs,
		"volume-dir",
		opt.VolumeDirs,
		"the config map volume directory to watch for updates; may be used multiple times.")
	flags.Var(&opt.NotifyHandType,
		"notify-type",
		"the config describe how to process notification messages.",
	)

	// for signal handle
	flags.StringVar(&opt.ProcessName,
		"process",
		opt.ProcessName,
		"the config describe what is db program.")
	flags.StringVar((*string)(&opt.Signal),
		"signal",
		string(opt.Signal),
		"the config describe reload unix signal.")

	// for exec handle
	flags.StringVar(&opt.Command,
		"command",
		opt.Command,
		"the config describe reload command. ")

	// for exec tpl scripts
	flags.StringVar(&opt.TPLConfig,
		"tpl-config",
		opt.TPLConfig,
		"the config describe reload by tpl script.")
	flags.StringVar(&opt.BackupPath,
		"backup-path",
		opt.BackupPath,
		"the config describe.")

	flags.StringVar(&opt.LogLevel,
		"log-level",
		opt.LogLevel,
		"the config set log level. enum: [error, info, debug]")
	flags.StringVar(&opt.FileRegex,
		"regex",
		opt.FileRegex,
		"the config set filter config file.")

	flags.StringVar(&opt.ServiceOpt.PodIP,
		"pod-ip",
		opt.ServiceOpt.PodIP,
		"the config set pod ip address.")
	flags.IntVar(&opt.ServiceOpt.GrpcPort,
		"tcp",
		opt.ServiceOpt.GrpcPort,
		"the config set service port.")
	flags.BoolVar(&opt.ServiceOpt.DebugMode,
		"debug",
		opt.ServiceOpt.DebugMode,
		"the config set debug.")
	flags.StringVar((*string)(&opt.ServiceOpt.ContainerRuntime),
		"container-runtime",
		string(opt.ServiceOpt.ContainerRuntime),
		"the config set cri runtime type.")
	flags.StringVar(&opt.ServiceOpt.RuntimeEndpoint,
		"runtime-endpoint",
		opt.ServiceOpt.RuntimeEndpoint,
		"the config set cri runtime endpoint.")

	flags.BoolVar(&opt.ServiceOpt.ContainerRuntimeEnable,
		"cri-enable",
		opt.ServiceOpt.ContainerRuntimeEnable,
		"the config set enable cri.")

	flags.BoolVar(&opt.ServiceOpt.RemoteOnlineUpdateEnable,
		"operator-update-enable",
		opt.ServiceOpt.ContainerRuntimeEnable,
		"the config set enable operator update parameter.")
}
