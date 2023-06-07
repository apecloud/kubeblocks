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
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/cmd/probe/role/internal"
)

func main() {

	var err error

	var port int
	var url string
	flag.IntVar(&port, "port", internal.DefaultRoleObservationPort, "")
	flag.StringVar(&url, "url", internal.DefaultRoleObservationPath, "")

	viper.SetConfigFile(viper.GetString("config")) // path to look for the config file in
	_ = viper.ReadInConfig()                       // Find and read the config file
	// just ignore err, if we hit err, use default settings

	agent := internal.NewRoleAgent(os.Stdin, "ROLE_OBSERVATION")
	err = agent.Init()
	if err != nil {
		panic(fmt.Errorf("fatal error custom init: %v", err))
	}

	err = http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil)
	if err != nil {
		panic(fmt.Errorf("fatal error http listen: %v", err))
	}

	http.HandleFunc(url, func(writer http.ResponseWriter, request *http.Request) {
		opsRes, shouldNotify := agent.CheckRole(request.Context())
		buf, err := json.Marshal(opsRes)
		if err != nil {
			panic(fmt.Errorf("fatal error json parse: %v", err))
		}

		if _, exist := opsRes["event"]; !exist || len(opsRes) == 0 {
			code, _ := strconv.Atoi(internal.RealReadinessFail)
			writer.WriteHeader(code)
			return
		}

		if shouldNotify {
			code, _ := strconv.Atoi(internal.OperationFailedHTTPCode)
			writer.WriteHeader(code)
			_, err = writer.Write(buf)
			if err != nil {
				panic(fmt.Errorf("fatal error response write: %v", err))
			}
		} else {
			writer.WriteHeader(http.StatusNoContent)
		}
	})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	<-stop
	agent.ShutDownClient()
}
