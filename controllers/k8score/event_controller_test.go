/*
Copyright ApeCloud Inc.

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

package k8score

import (
	"bytes"
	"context"
	"text/template"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type roleEventValue struct {
	PodName  string
	EventSeq string
	Role     string
}

var _ = Describe("Event Controller", func() {
	var ctx = context.Background()

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &corev1.Event{},
			client.InNamespace(testCtx.DefaultNamespace),
			client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.DeleteAllOf(ctx, &corev1.Pod{},
			client.InNamespace(testCtx.DefaultNamespace),
			client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	Context("When receiving role changed event", func() {
		It("should handle it properly", func() {
			By("create involved pod")
			podName := "foo"
			pod := createInvolvedPod(podName)
			Expect(testCtx.CreateObj(ctx, &pod)).Should(Succeed())
			Eventually(func() error {
				p := &corev1.Pod{}
				return k8sClient.Get(ctx, types.NamespacedName{
					Namespace: pod.Namespace,
					Name:      pod.Name,
				}, p)
			}, time.Second*10, time.Second).Should(Succeed())

			By("send role changed event")
			sndEvent, err := createRoleChangedEvent(podName, "leader")
			Expect(err).Should(Succeed())
			Expect(testCtx.CreateObj(ctx, sndEvent)).Should(Succeed())
			Eventually(func() string {
				event := &corev1.Event{}
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: sndEvent.Namespace,
					Name:      sndEvent.Name,
				}, event); err != nil {
					return err.Error()
				}
				return event.InvolvedObject.Name
			}, time.Second*60, time.Second).Should(Equal(sndEvent.InvolvedObject.Name))
		})
	})
})

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

	seq, err := password.Generate(16, 16, 0, true, true)
	if err != nil {
		return nil, err
	}
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
	err = tmpl.Execute(buf, roleValue)
	if err != nil {
		return nil, err
	}

	event, _, err := scheme.Codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
	if err != nil {
		return nil, err
	}

	return event.(*corev1.Event), nil
}

func createInvolvedPod(name string) corev1.Pod {
	podYaml := `
apiVersion: v1
kind: Pod
metadata:
  name: my-name
  namespace: default
spec:
  containers:
  - image: docker.io/apecloud/wesql-server-8.0:0.1-SNAPSHOT
    name: mysql
`
	pod := corev1.Pod{}
	Expect(yaml.Unmarshal([]byte(podYaml), &pod)).Should(Succeed())
	pod.Name = name

	return pod
}
