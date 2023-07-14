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

	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding"

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/mysql"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"go.uber.org/automaxprocs/maxprocs"
)

var mysqlOps *mysql.MysqlOperations

func init() {
	viper.AutomaticEnv()
	mysqlOps = mysql.NewMysql(logr.Logger{})
	err := mysqlOps.Init()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	// set GOMAXPROCS
	_, _ = maxprocs.Set()
	mainLogger := logr.Logger{}

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		panic(fmt.Errorf("fatal error viper bindPFlags: %v", err))
	}
	viper.SetConfigFile(viper.GetString("config")) // path to look for the config file in
	err = viper.ReadInConfig()                     // Find and read the config file
	if err != nil {                                // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %v", err))
	}

	if err != nil {
		mainLogger.Error(err, "fatal error from runtime")
		os.Exit(1)
	}

	go func() {
		http.HandleFunc("/probe", func(writer http.ResponseWriter, req *http.Request) {
			// Readiness 是 Get请求, CmdChannel 是 Post请求
			probeReq := &binding.ProbeRequest{Metadata: map[string]string{}}
			if req.Method == "GET" {
				probeReq.Operation = "getRole"
			} else {
				// 获取operation
				err := json.NewDecoder(req.Body).Decode(probeReq)
				if err != nil {
					mysqlOps.Logger.Error(err, "json Decode failed")
				}
			}
			dispatch, err := mysqlOps.Dispatch(req.Context(), probeReq)
			if err != nil {
				mysqlOps.Logger.Error(err, "dispatch failed")
			}
			if codeStr, ok := dispatch.Metadata[binding.StatusCode]; ok && (codeStr == binding.OperationNotFoundHTTPCode || codeStr == binding.OperationFailedHTTPCode) {
				code, _ := strconv.Atoi(codeStr)
				writer.WriteHeader(code)
			}
			// todo 修改状态码
			writer.Write(dispatch.Data)
		})
		http.ListenAndServe("localhost:7979", nil)
	}()

	// ha dependent on dbmanager which is initialized by rt.Run
	// ha := highavailability.NewHa(logHa)
	// if ha != nil {
	// 	defer ha.ShutdownWithWait()
	// 	go ha.Start()
	// }

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	<-stop
	// 收尾
}
