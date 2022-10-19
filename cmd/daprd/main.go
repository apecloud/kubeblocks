/*
Copyright 2021 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/automaxprocs/maxprocs"

	// Register all components
	//_ "github.com/dapr/dapr/cmd/daprd/components"

	bindingsLoader "github.com/dapr/dapr/pkg/components/bindings"
	// configurationLoader "github.com/dapr/dapr/pkg/components/configuration"
	// lockLoader "github.com/dapr/dapr/pkg/components/lock"
	// httpMiddlewareLoader "github.com/dapr/dapr/pkg/components/middleware/http"
	// nrLoader "github.com/dapr/dapr/pkg/components/nameresolution"
	// pubsubLoader "github.com/dapr/dapr/pkg/components/pubsub"
	// secretstoresLoader "github.com/dapr/dapr/pkg/components/secretstores"
	// stateLoader "github.com/dapr/dapr/pkg/components/state"

	"github.com/dapr/dapr/pkg/runtime"
	"github.com/dapr/kit/logger"

	// "github.com/dapr/components-contrib/bindings"
	// "github.com/dapr/components-contrib/bindings/mysql"
	// "github.com/dapr/components-contrib/bindings/postgres"
	// "github.com/dapr/components-contrib/bindings/redis"
	"github.com/dapr/components-contrib/bindings/http"
	// "github.com/dapr/components-contrib/bindings/kafka"
	"github.com/dapr/components-contrib/bindings/localstorage"

	"github.com/apecloud/kubeblocks/pkg/binding/mysql"
)

var (
	log        = logger.NewLogger("dapr.runtime")
	logContrib = logger.NewLogger("dapr.contrib")
)

func init() {
	bindingsLoader.DefaultRegistry.RegisterOutputBinding(mysql.NewMysql, "mysql")
	// bindingsLoader.DefaultRegistry.RegisterOutputBinding(postgres.NewPostgres, "postgres")
	// bindingsLoader.DefaultRegistry.RegisterOutputBinding(redis.NewRedis, "redis")
	bindingsLoader.DefaultRegistry.RegisterOutputBinding(http.NewHTTP, "http")
	// bindingsLoader.DefaultRegistry.RegisterInputBinding(func(l logger.Logger) bindings.InputBinding {
	// 	return kafka.NewKafka(l)
	// }, "kafka")
	// bindingsLoader.DefaultRegistry.RegisterOutputBinding(func(l logger.Logger) bindings.OutputBinding {
	// 	return kafka.NewKafka(l)
	// }, "kafka")
	bindingsLoader.DefaultRegistry.RegisterOutputBinding(localstorage.NewLocalStorage, "localstorage")
}

func main() {
	// set GOMAXPROCS
	_, _ = maxprocs.Set()

	rt, err := runtime.FromFlags()
	if err != nil {
		log.Fatal(err)
	}

	// secretstoresLoader.DefaultRegistry.Logger = logContrib
	// stateLoader.DefaultRegistry.Logger = logContrib
	// configurationLoader.DefaultRegistry.Logger = logContrib
	// lockLoader.DefaultRegistry.Logger = logContrib
	// pubsubLoader.DefaultRegistry.Logger = logContrib
	// nrLoader.DefaultRegistry.Logger = logContrib
	bindingsLoader.DefaultRegistry.Logger = logContrib
	// httpMiddlewareLoader.DefaultRegistry.Logger = log

	err = rt.Run(
		// runtime.WithSecretStores(secretstoresLoader.DefaultRegistry),
		// runtime.WithStates(stateLoader.DefaultRegistry),
		// runtime.WithConfigurations(configurationLoader.DefaultRegistry),
		// runtime.WithLocks(lockLoader.DefaultRegistry),
		// runtime.WithPubSubs(pubsubLoader.DefaultRegistry),
		// runtime.WithNameResolutions(nrLoader.DefaultRegistry),
		runtime.WithBindings(bindingsLoader.DefaultRegistry),
		// runtime.WithHTTPMiddlewares(httpMiddlewareLoader.DefaultRegistry),
	)
	if err != nil {
		log.Fatalf("fatal error from runtime: %s", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	<-stop
	rt.ShutdownWithWait()
}
