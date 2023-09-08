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
	"os"
	"os/signal"
	"syscall"

	"github.com/dapr/kit/logger"
	"github.com/spf13/pflag"
	"go.uber.org/automaxprocs/maxprocs"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/apiserver"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/highavailability"
	"github.com/apecloud/kubeblocks/internal/constant"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

var (
	log   = logger.NewLogger("lorry.runtime")
	logHa = logger.NewLogger("lorry.ha")
)

func init() {
	viper.AutomaticEnv()

}

func main() {
	// set GOMAXPROCS
	_, _ = maxprocs.Set()

	// start apiserver for HTTP and GRPC
	rt, err := apiserver.StartDapr()
	if err != nil {
		log.Fatalf("Start ApiServer failed: %s", err)
	}

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	err = viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		panic(fmt.Errorf("fatal error viper bindPFlags: %v", err))
	}
	viper.SetConfigFile(viper.GetString("config")) // path to look for the config file in
	err = viper.ReadInConfig()                     // Find and read the config file
	if err != nil {                                // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %v", err))
	}

	// ha dependent on dbmanager which is initialized by rt.Run
	characterType := viper.GetString(constant.KBEnvCharacterType)
	workloadType := viper.GetString(constant.KBEnvWorkloadType)
	ha := highavailability.NewHa(logHa)
	if highavailability.IsHAAvailable(characterType, workloadType) {
		if ha != nil {
			defer ha.ShutdownWithWait()
			go ha.Start()
		}
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	<-stop
	rt.ShutdownWithWait()
}
