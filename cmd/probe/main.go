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
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/observation"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/automaxprocs/maxprocs"
)

var (
	logger = log.Logger{}
)

func main() {
	// set GOMAXPROCS
	_, _ = maxprocs.Set()

	var err error
	if err != nil {
		log.Fatal(err)
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

	custom := observation.NewController(logger)
	err = custom.Init()
	if err != nil {
		panic(fmt.Errorf("fatal error custom init: %v", err))
	}

	http.ListenAndServe("localhost:3501", nil)
	url := "/role"

	http.HandleFunc(url, func(writer http.ResponseWriter, request *http.Request) {
		opsRes, shouldNotify := custom.CheckRoleOps(request.Context())
		buf, err := json.Marshal(opsRes)
		if err != nil {
			panic(fmt.Errorf("fatal error json parse: %v", err))
		}

		if _, exist := opsRes["event"]; !exist || len(opsRes) == 0 {
			code, _ := strconv.Atoi(observation.RealReadinessFail)
			writer.WriteHeader(code)
			return
		}

		if shouldNotify {
			code, _ := strconv.Atoi(observation.OperationFailedHTTPCode)
			writer.WriteHeader(code)
			writer.Write(buf)
		} else {
			writer.WriteHeader(http.StatusNoContent)
		}
	})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	<-stop
	custom.ShutDownClient()
}
