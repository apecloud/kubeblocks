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

package cluster

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/exec"
)

var _ = Describe("connection", func() {
	It("new connection command", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("test")
		defer tf.Cleanup()

		cmd := NewConnectCmd(tf, genericclioptions.NewTestIOStreamsDiscard())
		Expect(cmd).ShouldNot(BeNil())
	})

	It("connection options", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("test")
		defer tf.Cleanup()
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		ns := scheme.Codecs.WithoutConversion()
		tf.Client = &fake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: "", Version: "v1"},
			NegotiatedSerializer: ns,
			Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				body := cmdtesting.ObjBody(codec, execPod())
				return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: body}, nil
			}),
		}
		tf.ClientConfigVal = &restclient.Config{APIPath: "/api", ContentConfig: restclient.ContentConfig{NegotiatedSerializer: scheme.Codecs, GroupVersion: &schema.GroupVersion{Version: "v1"}}}

		o := &ConnectOptions{ExecOptions: exec.NewExecOptions(tf, genericclioptions.NewTestIOStreamsDiscard())}
		input := &exec.ExecInput{
			Use:      "connect",
			Short:    "Connect to a database cluster",
			Validate: o.validate,
			Complete: o.complete,
			AddFlags: o.addFlags,
		}
		cmd := o.Build(input)
		Expect(cmd).ShouldNot(BeNil())
		Expect(o.complete([]string{})).Should(HaveOccurred())

		execOptions := o.ExecOptions
		o.PodName = "foo"
		Expect(execOptions.Complete([]string{"test"})).Should(Succeed())
		Expect(execOptions.Validate()).Should(Succeed())
		Expect(execOptions.Run()).Should(HaveOccurred())
	})
})

func execPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Namespace:       "test",
			ResourceVersion: "10",
			Labels: map[string]string{
				"app.kubernetes.io/name": "state.mysql-apecloud-wesql",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			DNSPolicy:     corev1.DNSClusterFirst,
			Containers: []corev1.Container{
				{
					Name: "bar",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}
