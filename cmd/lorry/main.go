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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"go.uber.org/automaxprocs/maxprocs"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/internal/constant"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
	"github.com/apecloud/kubeblocks/lorry/component"
	"github.com/apecloud/kubeblocks/lorry/highavailability"
	"github.com/apecloud/kubeblocks/lorry/middleware/http/probe"
	"github.com/apecloud/kubeblocks/lorry/util"
)

var port int
var configDir string

const (
	DefaultPort       = 3501
	DefaultConfigPath = "config/probe"
)

func init() {
	viper.AutomaticEnv()
	flag.IntVar(&port, "port", DefaultPort, "probe default port")
	flag.StringVar(&configDir, "config-path", DefaultConfigPath, "probe default config directory for builtin type")
}

func main() {
	// set GOMAXPROCS
	_, _ = maxprocs.Set()

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		panic(fmt.Errorf("fatal error viper bindPFlags: %v", err))
	}

	err = component.GetAllComponent(configDir) // find all builtin config file and read
	if err != nil {                            // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %v", err))
	}

	err = probe.RegisterBuiltin() // register all builtin component
	if err != nil {
		panic(fmt.Errorf("fatal error register builtin: %v", err))
	}

	http.HandleFunc("/", probe.SetMiddleware(probe.GetRouter()))
	go func() {
		addr := fmt.Sprintf(":%d", port)
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			panic(fmt.Errorf("fatal error listen on port %d", port))
		}
	}()

	// ha dependent on dbmanager which is initialized by rt.Run
	characterType := viper.GetString(constant.KBEnvCharacterType)
	workloadType := viper.GetString(constant.KBEnvWorkloadType)
	logHa := ctrl.Log.WithName("HA")
	ha := highavailability.NewHa(logHa)
	if util.IsHAAvailable(characterType, workloadType) {
		if ha != nil {
			defer ha.ShutdownWithWait()
			go ha.Start()
		}
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	<-stop
}
