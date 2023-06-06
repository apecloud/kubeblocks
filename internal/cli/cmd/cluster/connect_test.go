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

package cluster

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/exec"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("connection", func() {
	const (
		namespace   = "test"
		clusterName = "test"
	)

	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
	)

	BeforeEach(func() {
		tf = cmdtesting.NewTestFactory().WithNamespace("test")
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		cluster := testing.FakeCluster(clusterName, namespace)
		pods := testing.FakePods(3, namespace, clusterName)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}
		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				urlPrefix := "/api/v1/namespaces/" + namespace
				return map[string]*http.Response{
					urlPrefix + "/services":        httpResp(testing.FakeServices()),
					urlPrefix + "/secrets":         httpResp(testing.FakeSecrets(namespace, clusterName)),
					urlPrefix + "/pods":            httpResp(pods),
					urlPrefix + "/pods/test-pod-0": httpResp(findPod(pods, "test-pod-0")),
				}[req.URL.Path], nil
			}),
		}

		tf.Client = tf.UnstructuredClient
		tf.FakeDynamicClient = testing.FakeDynamicClient(cluster, testing.FakeClusterDef(), testing.FakeClusterVersion())
		streams = genericclioptions.NewTestIOStreamsDiscard()
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("new connection command", func() {
		cmd := NewConnectCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("validate", func() {
		o := &ConnectOptions{ExecOptions: exec.NewExecOptions(tf, streams)}

		By("specified more than one cluster")
		Expect(o.validate([]string{"c1", "c2"})).Should(HaveOccurred())

		By("without cluster name")
		Expect(o.validate(nil)).Should(HaveOccurred())

		Expect(o.validate([]string{clusterName})).Should(Succeed())

		// set instance name and cluster name, should fail
		o.PodName = "test-pod-0"
		Expect(o.validate([]string{clusterName})).Should(HaveOccurred())
		o.componentName = "test-component"
		Expect(o.validate([]string{})).Should(HaveOccurred())

		// unset pod name
		o.PodName = ""
		Expect(o.validate([]string{clusterName})).Should(Succeed())
		// unset component name
		o.componentName = ""
		Expect(o.validate([]string{clusterName})).Should(Succeed())
	})

	It("complete by cluster name", func() {
		o := &ConnectOptions{ExecOptions: exec.NewExecOptions(tf, streams)}
		Expect(o.validate([]string{clusterName})).Should(Succeed())
		Expect(o.complete()).Should(Succeed())
		Expect(o.Pod).ShouldNot(BeNil())
	})

	It("complete by pod name", func() {
		o := &ConnectOptions{ExecOptions: exec.NewExecOptions(tf, streams)}
		o.PodName = "test-pod-0"
		Expect(o.validate([]string{})).Should(Succeed())
		Expect(o.complete()).Should(Succeed())
		Expect(o.Pod).ShouldNot(BeNil())
	})

	It("show example", func() {
		o := &ConnectOptions{ExecOptions: exec.NewExecOptions(tf, streams)}
		Expect(o.validate([]string{clusterName})).Should(Succeed())
		Expect(o.complete()).Should(Succeed())

		By("specify one cluster")
		Expect(o.runShowExample()).Should(Succeed())
	})

	It("getUserAndPassword", func() {
		const (
			user     = "test-user"
			password = "test-password"
		)
		secret := corev1.Secret{}
		secret.Name = "test-conn-credential"
		secret.Data = map[string][]byte{
			"username": []byte(user),
			"password": []byte(password),
		}
		secretList := &corev1.SecretList{}
		secretList.Items = []corev1.Secret{secret}
		u, p, err := getUserAndPassword(testing.FakeClusterDef(), secretList)
		Expect(err).Should(Succeed())
		Expect(u).Should(Equal(user))
		Expect(p).Should(Equal(password))
	})
})

func mockPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Namespace:       "test",
			ResourceVersion: "10",
			Labels: map[string]string{
				"app.kubernetes.io/name": "mysql-apecloud-mysql",
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

func findPod(pods *corev1.PodList, name string) *corev1.Pod {
	for i, pod := range pods.Items {
		if pod.Name == name {
			return &pods.Items[i]
		}
	}
	return nil
}
