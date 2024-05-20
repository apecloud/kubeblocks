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
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
)

var (
	crdPath   string
	version   string
	namespace string
)

func setupFlags() {
	pflag.StringVar(&crdPath, "crd", "/kubeblocks/crd", "CRD directory for the kubeblocks")
	pflag.StringVar(&version, "version", "", "KubeBlocks version")
	pflag.StringVar(&namespace, "namespace", "default", "The namespace scope for this request")
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

	config, err := clientcmd.BuildConfigFromFlags("", "")
	hook.CheckErr(err)

	upgradeContext := hook.NewUpgradeContext(ctx, config, version, crdPath, namespace)
	hook.CheckErr(hook.NewUpgradeWorkflow().
		AddStage(&hook.Prepare{}).
		AddStage(&hook.StopOperator{}).
		AddStage(&hook.Addon{}).
		AddStage(&hook.UpdateCRD{}).
		Do(upgradeContext))
}
