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
	"log"
	"net"
	"os"

	"github.com/apecloud/kubeblocks/cmd/probe/role/internal"

	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	health "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/spf13/viper"
)

func main() {
	viper.AutomaticEnv()

	var err error

	var port string
	var url string
	flag.StringVar(&port, "port", internal.DefaultPort, "")
	flag.StringVar(&url, "url", internal.DefaultServiceName, "")
	flag.Parse()

	server := internal.NewGrpcServer(url)
	if err = server.Init(); err != nil {
		log.Fatalf("fatal error init grpcserver failed: %v", err)
	}

	port = ":" + port
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("fatal error net listen failed :%v", err)
	}

	s := grpc.NewServer()
	health.RegisterHealthServer(s, server)
	log.Println("start gRPC liston on port " + port)

	err = s.Serve(listener)
	if err != nil {
		log.Fatalf("fatal error grpcserver serve failed: %v", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	<-stop
	server.ShutDownClient()
}
