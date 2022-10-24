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
	"bytes"
	"context"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/dapr/kit/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

		// or emit event
		event, err := createRoleChangedEvent(name, role)
		if err != nil {
			log.Error(err)
			return
		}
		_, err = clientset.CoreV1().Events(namespace).Create(ctx, event, metav1.CreateOptions{})
		if err != nil {
			log.Error(err)
			return
		}

		lastRoleObserved = role
	}

	// TODO parameterize interval
	go wait.UntilWithContext(context.TODO(), roleObserve, time.Second*5)
}

func createRoleChangedEvent(podName, role string) (*corev1.Event, error) {
	eventTmpl := `
apiVersion: v1
kind: Event
metadata:
  name: {{ .PodName }}.{{ .EventSeq }}
  namespace: default
involvedObject:
  apiVersion: v1
  fieldPath: spec.containers{kbprobe-rolechangedcheck}
  kind: Pod
  name: {{ .PodName }}
  namespace: default
message: "{\"data\":{\"role\":\"{{ .Role }}\"}}"
reason: RoleChanged
type: Normal
`

	seq := randStringBytes(16)
	roleValue := roleEventValue{
		PodName:  podName,
		EventSeq: seq,
		Role:     role,
	}
	tmpl, err := template.New("event-tmpl").Parse(eventTmpl)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	tmpl.Execute(buf, roleValue)

	event, _, err := scheme.Codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
	if err != nil {
		return nil, err
	}

	return event.(*corev1.Event), nil
}

type roleEventValue struct {
	PodName  string
	EventSeq string
	Role     string
}

const letterBytes = "0123456789abcdefghijklmnopqrstuvwxyz"

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
