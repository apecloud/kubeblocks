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

package exec

import (
	"context"
	"fmt"
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
)

var _ = Describe("Exec", func() {
	It("new exec command", func() {
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

		testOptions := &testExecOptions{ExecOptions: NewExecOptions(tf, genericclioptions.NewTestIOStreamsDiscard())}
		execOptions := testOptions.ExecOptions

		By("complete")
		testOptions.PodName = "foo"
		Expect(execOptions.Complete()).Should(Succeed())
		Expect(execOptions.Config).ShouldNot(BeNil())
		Expect(execOptions.Namespace).Should(Equal("test"))

		By("validate")
		Expect(testOptions.complete([]string{"test"}))
		Expect(testOptions.validate()).Should(Succeed())
		Expect(testOptions.ContainerName).Should(Equal("test"))

		By("run")
		Expect(testOptions.Run()).Should(HaveOccurred())

		// Corner case test
		testOptions.ContainerName = ""
		Expect(testOptions.ExecOptions.validate()).Should(Succeed())
		Expect(testOptions.ContainerName).Should(Equal("bar"))
		testOptions.Pod = nil
		testOptions.PodName = ""
		Expect(testOptions.ExecOptions.validate()).Should(MatchError("failed to get the pod to execute"))
	})
})

type testExecOptions struct {
	name string
	*ExecOptions
}

func (o *testExecOptions) complete(args []string) error {
	var err error
	if len(args) == 0 {
		return fmt.Errorf("you must specified the cluster name")
	}
	o.name = args[0]
	if o.PodName == "" {
		return fmt.Errorf("you must specified the intance name")
	}
	o.Pod, err = o.Client.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	o.Command = []string{"test"}
	o.ContainerName = "test"
	return nil
}

func execPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Namespace:       "test",
			ResourceVersion: "10",
			Labels: map[string]string{
				"app.kubernetes.io/name": "state.mysql-apecloud-mysql",
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
