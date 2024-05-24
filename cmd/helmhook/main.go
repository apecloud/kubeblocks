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
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
	_ "github.com/apecloud/kubeblocks/cmd/helmhook/hook/multiversion"
)

var (
	crdPath    string
	version    string
	namespace  string
	keepAddons bool
)

func setupFlags() {
	pflag.StringVar(&crdPath, "crd", "/kubeblocks/crd", "CRD directory for the kubeblocks")
	pflag.StringVar(&version, "version", "", "KubeBlocks version")
	pflag.StringVar(&namespace, "namespace", "default", "The namespace scope for this request")
	pflag.BoolVar(&keepAddons, "keep-addons", true, "Whether to allow addon updates. If set to true, the addons that KubeBlocks depends on will not be upgraded after KubeBlocks is upgrade")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
}

func main() {
	setupFlags()

	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	config, err := ctrl.GetConfig()
	hook.CheckErr(err)

	upgradeContext := hook.NewUpgradeContext(ctx, config, version, crdPath, namespace)
	hook.CheckErr(hook.NewUpgradeWorkflow().
		WrapStage(hook.PrepareFor).
		AddStage(&hook.StopOperator{}).
		AddStage(&hook.Addon{KeepAddons: keepAddons}).
		AddStage(&hook.Conversion{}).
		AddStage(&hook.UpdateCRD{}).
		AddStage(&hook.UpdateCR{}).
		Do(upgradeContext))
}
