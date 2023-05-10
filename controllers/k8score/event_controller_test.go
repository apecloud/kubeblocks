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

package k8score

import (
	"bytes"
	"context"
	"text/template"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	probeutil "github.com/apecloud/kubeblocks/cmd/probe/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

type roleEventValue struct {
	PodName  string
	EventSeq string
	Role     string
}

var _ = Describe("Event Controller", func() {
	var ctx = context.Background()

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.EventSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

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
			}).Should(Succeed())

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
			}).Should(Equal(sndEvent.InvolvedObject.Name))

			By("Test parse event message")
			reqCtx := intctrlutil.RequestCtx{
				Ctx: testCtx.Ctx,
				Log: log.FromContext(ctx).WithValues("event", testCtx.DefaultNamespace),
			}
			eventMessage := ParseProbeEventMessage(reqCtx, sndEvent)
			Expect(eventMessage).ShouldNot(BeNil())

			By("check whether the duration and number of events reach the threshold")
			IsOvertimeEvent(sndEvent, 5*time.Second)
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
message: "{\"event\":\"roleChanged\",\"originalRole\":\"secondary\",\"role\":\"{{ .Role }}\"}"
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

	event := &corev1.Event{}
	_, _, err := scheme.Codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, event)
	if err != nil {
		return nil, err
	}
	event.Reason = string(probeutil.CheckRoleOperation)

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
  - image: docker.io/apecloud/apecloud-mysql-server:latest
    name: mysql
`
	pod := corev1.Pod{}
	Expect(yaml.Unmarshal([]byte(podYaml), &pod)).Should(Succeed())
	pod.Name = name

	return pod
}
