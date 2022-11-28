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

package util

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/gosuri/uitable"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("util", func() {
	It("Get home dir", func() {
		home, err := GetCliHomeDir()
		Expect(len(home) > 0).Should(BeTrue())
		Expect(err == nil).Should(BeTrue())
	})

	It("Get kubeconfig dir", func() {
		dir := GetKubeconfigDir()
		Expect(len(dir) > 0).Should(BeTrue())
	})

	It("DoWithRetry", func() {
		op := func() error {
			return fmt.Errorf("test DowithRetry")
		}
		logger := logr.New(log.NullLogSink{})
		Expect(DoWithRetry(context.TODO(), logger, op, &RetryOptions{MaxRetry: 2})).Should(HaveOccurred())
	})

	It("Config path", func() {
		path := ConfigPath("")
		Expect(len(path) == 0).Should(BeTrue())
		path = ConfigPath("test")
		Expect(len(path) > 0).Should(BeTrue())
		Expect(RemoveConfig("")).Should(HaveOccurred())
	})

	It("Print yaml", func() {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "dataprotection.kubeblocks.io/v1alpha1",
				"kind":       "BackupJob",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "test",
				},
				"spec": map[string]interface{}{
					"backupPolicyName": "backup-policy-demo",
					"backupType":       "full",
					"ttl":              "168h0m0s",
				},
			},
		}
		Expect(PrintObjYAML(obj)).Should(Succeed())
	})

	It("Print go template", func() {
		Expect(PrintGoTemplate(os.Stdout, `key: {{.Value}}`, struct {
			Value string
		}{"test"})).Should(Succeed())
	})

	It("Test Spinner", func() {
		spinner := Spinner(os.Stdout, "dbctl spinner test ... ")
		spinner(true)

		spinner = Spinner(os.Stdout, "dbctl spinner test ... ")
		spinner(false)
	})

	It("Check errors", func() {
		CheckErr(nil)

		err := fmt.Errorf("test error")
		printErr(err)
	})

	It("PrintTable", func() {
		tbl := uitable.New()
		tbl.AddRow("TEST")
		tbl.AddRow("test")
		Expect(PrintTable(os.Stdout, tbl)).Should(Succeed())
	})

	It("GetNodeByName", func() {
		nodes := []*corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
		}
		Expect(GetNodeByName(nodes, "test")).ShouldNot(BeNil())
		Expect(GetNodeByName(nodes, "non-exists")).Should(BeNil())
	})

	It("GetPodStatus", func() {
		newPod := func(phase corev1.PodPhase) *corev1.Pod {
			return &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: phase,
				}}
		}

		var pods []*corev1.Pod
		for _, p := range []corev1.PodPhase{corev1.PodRunning, corev1.PodPending, corev1.PodSucceeded, corev1.PodFailed} {
			pods = append(pods, newPod(p))
		}

		r, w, s, f := GetPodStatus(pods)
		Expect(r).Should(Equal(1))
		Expect(w).Should(Equal(1))
		Expect(s).Should(Equal(1))
		Expect(f).Should(Equal(1))
	})

	It("Others", func() {
		if os.Getenv("TEST_GET_PUBLIC_IP") != "" {
			_, err := GetPublicIP()
			Expect(err).ShouldNot(HaveOccurred())
		}
		Expect(MakeSSHKeyPair("", "")).Should(HaveOccurred())
		Expect(SetKubeConfig("test")).Should(Succeed())
		Expect(NewFactory()).ShouldNot(BeNil())

		By("playground dir")
		dir, err := PlaygroundDir()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(dir).ShouldNot(Equal(""))

		By("resource is empty")
		res := resource.Quantity{}
		Expect(ResourceIsEmpty(&res)).Should(BeTrue())
		res.Set(20)
		Expect(ResourceIsEmpty(&res)).Should(BeFalse())
	})
})
