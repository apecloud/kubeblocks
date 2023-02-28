/*
Copyright ApeCloud, Inc.

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
	"context"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var _ = Describe("Expose", func() {

	const (
		namespace   = "default"
		clusterName = "test-cluster"
	)

	var (
		tf      *cmdtesting.TestFactory
		streams genericclioptions.IOStreams
		o       *ExposeOptions
	)

	checkExposeAsExpected := func(provider util.K8sProvider, enabled bool, exposeType ExposeType) {
		svcList, err := o.client.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
		Expect(err).ShouldNot(HaveOccurred())
		for _, svc := range svcList.Items {
			if enabled {
				expected := ProviderExposeAnnotations[provider][exposeType]
				actual := svc.GetAnnotations()
				Expect(reflect.DeepEqual(actual, expected)).Should(BeTrue())
				Expect(svc.Spec.Type).Should(Equal(corev1.ServiceTypeLoadBalancer))
			} else {
				// if expose is disabled, the service should not have any expose related annotations
				Expect(svc.Spec.Type).Should(Equal(corev1.ServiceTypeClusterIP))
				Expect(svc.GetAnnotations()[ServiceAnnotationExposeType]).Should(BeEmpty())
				for k := range ProviderExposeAnnotations[provider][exposeType] {
					Expect(svc.GetAnnotations()[k]).Should(BeEmpty())
				}
			}
		}
	}

	genServices := func() *corev1.ServiceList {
		svc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-svc",
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/instance": clusterName,
				},
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "169.254.1.1",
			},
		}
		return &corev1.ServiceList{Items: []corev1.Service{svc}}
	}

	BeforeEach(func() {
		tf = cmdtesting.NewTestFactory().WithNamespace(namespace)
		streams, _, _, _ = genericclioptions.NewTestIOStreams()

		o = &ExposeOptions{IOStreams: streams, Name: clusterName, Namespace: namespace}
		o.client = testing.FakeClientSet(genServices())
	})

	AfterEach(func() {
		defer tf.Cleanup()
	})

	It("should succeed to new command", func() {
		cmd := NewExposeCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("should fail with invalid expose type", func() {
		err := o.Validate([]string{"--type", "fake-expose-type"})
		Expect(err).Should(HaveOccurred())
	})

	It("should fail with invalid enable value", func() {
		err := o.Validate([]string{"--enable", "fake-enable-value"})
		Expect(err).Should(HaveOccurred())
	})

	It("should succeed to expose to vpc/internet", func() {
		for _, exposeType := range []ExposeType{ExposeToVPC, ExposeToInternet} {
			o.exposeType = exposeType
			o.enabled = true

			By("enable expose")
			err := o.run(util.EKSProvider)
			Expect(err).ShouldNot(HaveOccurred())
			checkExposeAsExpected(util.EKSProvider, true, o.exposeType)

			By("modify expose type")
			switch exposeType {
			case ExposeToVPC:
				o.exposeType = ExposeToInternet
			case ExposeToInternet:
				o.exposeType = ExposeToVPC
			}
			err = o.run(util.EKSProvider)
			Expect(err).ShouldNot(HaveOccurred())
			checkExposeAsExpected(util.EKSProvider, true, o.exposeType)

			By("disable expose")
			o.enabled = false
			err = o.run(util.EKSProvider)
			Expect(err).ShouldNot(HaveOccurred())
			checkExposeAsExpected(util.EKSProvider, false, o.exposeType)
		}
	})
})
