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

package app

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	cfgutil "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration/configmap"
)

// NewConfigReloadCommand This command is used to reload configuration
func NewConfigReloadCommand(ctx context.Context, name string) *cobra.Command {
	opt, err := NewVolumeWatcherOpts()
	if err != nil {
		logrus.Fatal("failed to new VolumeWatcherOpts.")
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: name + " Provides a mechanism to implement reload config files in a sidecar for kubeblocks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolumeWatchCommand(ctx, opt)
		},
	}

	cmd.SetContext(ctx)
	InstallFlags(cmd.Flags(), opt)
	return cmd
}

func runVolumeWatchCommand(ctx context.Context, opt *VolumeWatcherOpts) error {
	initLog(opt.LogLevel)

	if err := checkOptions(opt); err != nil {
		return err
	}

	// new volume watcher
	watcher := cfgcore.NewVolumeWatcher(opt.VolumeDirs, ctx)

	defer watcher.Close()
	err := watcher.AddHandler(createHandlerWithWatchType(opt)).Run()
	if err != nil {
		return err
	}

	logrus.Info("reload started.")
	<-ctx.Done()
	logrus.Info("reload started shutdown.")

	return nil
}

func checkOptions(opt *VolumeWatcherOpts) error {
	if len(opt.ProcessName) == 0 {
		return cfgutil.MakeError("require process name is null.")
	}

	if len(opt.VolumeDirs) == 0 {
		return cfgutil.MakeError("require volume directory is null.")
	}

	return nil
}

func initLog(level string) {
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.SetLevel(logLevel)
}
