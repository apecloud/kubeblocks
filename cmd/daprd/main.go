/*
Copyright 2022 The KubeBlocks Authors

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
	"context"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/automaxprocs/maxprocs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	// Register all components
	//_ "github.com/dapr/dapr/cmd/daprd/components"

	bindingsLoader "github.com/dapr/dapr/pkg/components/bindings"
	configurationLoader "github.com/dapr/dapr/pkg/components/configuration"
	lockLoader "github.com/dapr/dapr/pkg/components/lock"
	httpMiddlewareLoader "github.com/dapr/dapr/pkg/components/middleware/http"
	nrLoader "github.com/dapr/dapr/pkg/components/nameresolution"
	pubsubLoader "github.com/dapr/dapr/pkg/components/pubsub"
	secretstoresLoader "github.com/dapr/dapr/pkg/components/secretstores"
	stateLoader "github.com/dapr/dapr/pkg/components/state"

	"github.com/dapr/dapr/pkg/runtime"
	"github.com/dapr/kit/logger"

	// "github.com/dapr/components-contrib/bindings"
	// "github.com/dapr/components-contrib/bindings/mysql"
	// "github.com/dapr/components-contrib/bindings/postgres"
	// "github.com/dapr/components-contrib/bindings/redis"
	dhttp "github.com/dapr/components-contrib/bindings/http"
	"github.com/dapr/components-contrib/bindings/localstorage"
	mdns "github.com/dapr/components-contrib/nameresolution/mdns"

	"github.com/apecloud/kubeblocks/pkg/binding/mysql"
)

var (
	log        = logger.NewLogger("dapr.runtime")
	logContrib = logger.NewLogger("dapr.contrib")
)

func init() {
	bindingsLoader.DefaultRegistry.RegisterOutputBinding(mysql.NewMysql, "mysql")
	bindingsLoader.DefaultRegistry.RegisterOutputBinding(dhttp.NewHTTP, "http")
	bindingsLoader.DefaultRegistry.RegisterOutputBinding(localstorage.NewLocalStorage, "localstorage")
	nrLoader.DefaultRegistry.RegisterComponent(mdns.NewResolver, "mdns")
}

func main() {
	// set GOMAXPROCS
	_, _ = maxprocs.Set()

	rt, err := runtime.FromFlags()
	if err != nil {
		log.Fatal(err)
	}

	secretstoresLoader.DefaultRegistry.Logger = logContrib
	stateLoader.DefaultRegistry.Logger = logContrib
	configurationLoader.DefaultRegistry.Logger = logContrib
	lockLoader.DefaultRegistry.Logger = logContrib
	pubsubLoader.DefaultRegistry.Logger = logContrib
	nrLoader.DefaultRegistry.Logger = logContrib
	bindingsLoader.DefaultRegistry.Logger = logContrib
	httpMiddlewareLoader.DefaultRegistry.Logger = log

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

	// start role label updating loop
	//dbaas.SetupConsensusRoleObservingLoop(logContrib)
	setupConsensusRoleObservingLoop(logContrib)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	<-stop
	rt.ShutdownWithWait()
}

const (
	consensusSetRoleLabelKey = "cs.dbaas.infracreate.com/role"
)

func setupConsensusRoleObservingLoop(log logger.Logger) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
		return
	}

	url := "http://localhost:3501/v1.0/bindings/mtest"
	contentType := "application/json"
	body := strings.NewReader("{\"operation\": \"roleCheck\", \"metadata\": {\"sql\" : \"\"}}")

	roleObserve := func(ctx context.Context) {
		// observe role through dapr
		resp, err := http.Post(url, contentType, body)
		if err != nil {
			log.Error(err)
			return
		}

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Error(err)
			return
		}

		// parse role
		role := strings.ToLower(string(body[:]))
		log.Info("role observed: ", role)

		// get pod object
		name := os.Getenv("MY_POD_NAME")
		namespace := os.Getenv("MY_POD_NAMESPACE")
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			log.Error(err)
			return
		}

		// update pod label
		patch := client.MergeFrom(pod.DeepCopy())
		pod.Labels[consensusSetRoleLabelKey] = role
		data, err := patch.Data(pod)
		if err != nil {
			log.Error(err)
			return
		}
		_, err = clientset.CoreV1().Pods(namespace).Patch(ctx, name, patch.Type(), data, metav1.PatchOptions{})
		if err != nil {
			log.Error(err)
		}
	}

	// TODO parameterize interval
	go wait.UntilWithContext(context.TODO(), roleObserve, time.Second*5)
}
