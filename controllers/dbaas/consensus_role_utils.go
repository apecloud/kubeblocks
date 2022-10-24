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

package dbaas

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dapr/kit/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SetupConsensusRoleObservingLoop(log logger.Logger) {
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

	// lastRoleObserved, role cache
	lastRoleObserved := ""
	roleObserve := func(ctx context.Context) {
		// observe role through dapr
		url := "http://localhost:3501/v1.0/bindings/mtest"
		contentType := "application/json"
		reqBody := strings.NewReader("{\"operation\": \"roleCheck\", \"metadata\": {\"sql\" : \"\"}}")
		resp, err := http.Post(url, contentType, reqBody)
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
		role := strings.ToLower(string(body))
		log.Info("role observed: ", role)
		if role == lastRoleObserved {
			log.Info("no role change since last observing, ignore")
		}

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
			return
		}

		lastRoleObserved = role
	}

	// TODO parameterize interval
	go wait.UntilWithContext(context.TODO(), roleObserve, time.Second*5)
}
