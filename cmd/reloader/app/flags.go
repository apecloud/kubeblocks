/*
Copyright ApeCloud Inc.

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
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

type NotifyEventType int

const (
	UnixSignal NotifyEventType = iota // "signal"
	WebHook                           // "http"
	ShellTool                         // "exec"
	Sql                               // "sql"
)

var allNotifyType = map[NotifyEventType]dbaasv1alpha1.CfgReloadType{
	UnixSignal: dbaasv1alpha1.UnixSignalType,
	WebHook:    dbaasv1alpha1.HttpType,
	ShellTool:  dbaasv1alpha1.ShellType,
	Sql:        dbaasv1alpha1.SqlType,
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

type VolumeWatcherOpts struct {
	VolumeDirs []string

	// fileRegex watch file regex
	FileRegex string

	// ProcessName: program name
	ProcessName string

	// Signal is valid for UnixSignal
	Signal string

	LogLevel       string
	NotifyHandType NotifyEventType
}

func NewVolumeWatcherOpts() (*VolumeWatcherOpts, error) {
	return &VolumeWatcherOpts{
		NotifyHandType: UnixSignal,
		Signal:         "SIGHUP",
		LogLevel:       logrus.InfoLevel.String(),
	}, nil
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
	flags.StringVar(&opt.ProcessName,
		"process",
		opt.ProcessName,
		"the config describe what is db program.")
	flags.StringVar(&opt.Signal,
		"signal",
		opt.Signal,
		"the config describe reload unix signal.")
	flags.StringVar(&opt.LogLevel,
		"log-level",
		opt.LogLevel,
		"the config set log level.")
	flags.StringVar(&opt.FileRegex,
		"regex",
		opt.FileRegex,
		"the config set filter config file.")
}
