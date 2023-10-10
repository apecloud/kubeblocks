/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"go.uber.org/automaxprocs/maxprocs"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	kzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/apecloud/kubeblocks/internal/constant"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
	"github.com/apecloud/kubeblocks/lorry/engines/register"
	"github.com/apecloud/kubeblocks/lorry/highavailability"
	"github.com/apecloud/kubeblocks/lorry/httpserver"
	"github.com/apecloud/kubeblocks/lorry/operations"
	"github.com/apecloud/kubeblocks/lorry/util"
)

func main() {
	// Set GOMAXPROCS
	_, _ = maxprocs.Set()

	// Initialize flags
	opts := kzap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.AutomaticEnv()
	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		panic(errors.Wrap(err, "fatal error viper bindPFlags"))
	}

	// Initialize logger
	kopts := []kzap.Opts{kzap.UseFlagOptions(&opts)}
	if strings.EqualFold("debug", viper.GetString("zap-log-level")) {
		kopts = append(kopts, kzap.RawZapOpts(zap.AddCaller()))
	}
	ctrl.SetLogger(kzap.New(kopts...))

	// Initialize DB Manager
	_, err = register.GetOrCreateManager()
	if err != nil {
		panic(errors.Wrap(err, "DB manager initialize failed"))
	}

	// Start HA
	characterType := viper.GetString(constant.KBEnvCharacterType)
	workloadType := viper.GetString(constant.KBEnvWorkloadType)
	if util.IsHAAvailable(characterType, workloadType) {
		ha := highavailability.NewHa()
		if ha != nil {
			defer ha.ShutdownWithWait()
			go ha.Start()
		}
	}

	// Start HTTP Server
	ops := operations.Operations()
	hServer := httpserver.NewServer(ops)
	err = hServer.StartNonBlocking()
	if err != nil {
		panic(errors.Wrap(err, "HTTP server initialize failed"))
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	<-stop
}
