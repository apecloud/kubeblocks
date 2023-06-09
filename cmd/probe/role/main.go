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

	"github.com/apecloud/kubeblocks/cmd/probe/role/internal"
)

func main() {

	var err error

	var port int
	var path string
	flag.IntVar(&port, "port", internal.DefaultRoleObservationPort, "")
	flag.StringVar(&path, "path", internal.DefaultRoleObservationPath, "")
	flag.Parse()

	agent := internal.NewRoleAgent(os.Stdin, "ROLE_OBSERVATION")
	err = agent.Init()
	if err != nil {
		log.Fatal(fmt.Errorf("fatal error custom init: %v", err))
	}

	err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Fatal(fmt.Errorf("fatal error http listen: %v", err))
	}

	http.HandleFunc(path, func(writer http.ResponseWriter, request *http.Request) {
		opsRes, shouldNotify := agent.CheckRole(request.Context())
		buf, err := json.Marshal(opsRes)
		if err != nil {
			shouldNotify = true
			buf = []byte(err.Error())
		}

		if _, exist := opsRes["event"]; !exist || len(opsRes) == 0 {
			shouldNotify = true
		}

		if shouldNotify {
			code, _ := strconv.Atoi(internal.OperationFailedHTTPCode)
			writer.WriteHeader(code)
			_, err = writer.Write(buf)
			if err != nil {
				log.Printf("fatal error ResponseWriter write: %v", err)
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
