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

	// Register all components
	bindingsLoader "github.com/dapr/dapr/pkg/components/bindings"
	configurationLoader "github.com/dapr/dapr/pkg/components/configuration"
	lockLoader "github.com/dapr/dapr/pkg/components/lock"
	httpMiddlewareLoader "github.com/dapr/dapr/pkg/components/middleware/http"
	nrLoader "github.com/dapr/dapr/pkg/components/nameresolution"
	pubsubLoader "github.com/dapr/dapr/pkg/components/pubsub"
	secretstoresLoader "github.com/dapr/dapr/pkg/components/secretstores"
	stateLoader "github.com/dapr/dapr/pkg/components/state"
	httpMiddleware "github.com/dapr/dapr/pkg/middleware/http"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/dapr/dapr/pkg/runtime"
	"github.com/dapr/kit/logger"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/custom"
	dhttp "github.com/dapr/components-contrib/bindings/http"
	"github.com/dapr/components-contrib/bindings/localstorage"
	"github.com/dapr/components-contrib/middleware"
	"github.com/dapr/components-contrib/nameresolution/mdns"

	"go.uber.org/automaxprocs/maxprocs"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/etcd"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/mongodb"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/mysql"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/postgres"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/redis"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/middleware/http/probe"
)

var (
	log        = logger.NewLogger("dapr.runtime")
	logContrib = logger.NewLogger("dapr.contrib")
)

func init() {
	viper.AutomaticEnv()
	bindingsLoader.DefaultRegistry.RegisterOutputBinding(mysql.NewMysql, "mysql")
	bindingsLoader.DefaultRegistry.RegisterOutputBinding(etcd.NewEtcd, "etcd")
	bindingsLoader.DefaultRegistry.RegisterOutputBinding(mongodb.NewMongoDB, "mongodb")
	bindingsLoader.DefaultRegistry.RegisterOutputBinding(redis.NewRedis, "redis")
	bindingsLoader.DefaultRegistry.RegisterOutputBinding(postgres.NewPostgres, "postgres")
	bindingsLoader.DefaultRegistry.RegisterOutputBinding(custom.NewHTTPCustom, "custom")
	bindingsLoader.DefaultRegistry.RegisterOutputBinding(dhttp.NewHTTP, "http")
	bindingsLoader.DefaultRegistry.RegisterOutputBinding(localstorage.NewLocalStorage, "localstorage")
	nrLoader.DefaultRegistry.RegisterComponent(mdns.NewResolver, "mdns")
	httpMiddlewareLoader.DefaultRegistry.RegisterComponent(func(log logger.Logger) httpMiddlewareLoader.FactoryMethod {
		return func(metadata middleware.Metadata) (httpMiddleware.Middleware, error) {
			return probe.NewProbeMiddleware(log).GetHandler(metadata)
		}
	}, "probe")

}

func main() {
	// set GOMAXPROCS
	_, _ = maxprocs.Set()

	rt, err := runtime.FromFlags()
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

	secretstoresLoader.DefaultRegistry.Logger = logContrib
	stateLoader.DefaultRegistry.Logger = logContrib
	configurationLoader.DefaultRegistry.Logger = logContrib
	lockLoader.DefaultRegistry.Logger = logContrib
	pubsubLoader.DefaultRegistry.Logger = logContrib
	nrLoader.DefaultRegistry.Logger = logContrib
	bindingsLoader.DefaultRegistry.Logger = logContrib
	httpMiddlewareLoader.DefaultRegistry.Logger = logContrib

	err = rt.Run(
		runtime.WithSecretStores(secretstoresLoader.DefaultRegistry),
		runtime.WithStates(stateLoader.DefaultRegistry),
		runtime.WithConfigurations(configurationLoader.DefaultRegistry),
		runtime.WithLocks(lockLoader.DefaultRegistry),
		runtime.WithPubSubs(pubsubLoader.DefaultRegistry),
		runtime.WithNameResolutions(nrLoader.DefaultRegistry),
		runtime.WithBindings(bindingsLoader.DefaultRegistry),
		runtime.WithHTTPMiddlewares(httpMiddlewareLoader.DefaultRegistry),
	)
	if err != nil {
		log.Fatalf("fatal error from runtime: %s", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	<-stop
	rt.ShutdownWithWait()
}
