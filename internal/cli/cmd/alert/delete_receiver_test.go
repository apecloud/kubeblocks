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

package alert

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	clientfake "k8s.io/client-go/rest/fake"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("alter", func() {
	var f *cmdtesting.TestFactory
	var s genericclioptions.IOStreams

	BeforeEach(func() {
		f = cmdtesting.NewTestFactory()
		f.Client = &clientfake.RESTClient{}
		s, _, _, _ = genericclioptions.NewTestIOStreams()
	})

	AfterEach(func() {
		f.Cleanup()
	})

	It("create new delete receiver cmd", func() {
		cmd := newDeleteReceiverCmd(f, s)
		Expect(cmd).NotTo(BeNil())
	})

	It("validate", func() {
		o := &deleteReceiverOptions{baseOptions: baseOptions{IOStreams: s}}
		Expect(o.validate([]string{})).Should(HaveOccurred())
		Expect(o.validate([]string{"test"})).Should(Succeed())
	})

	It("run", func() {
		o := &deleteReceiverOptions{baseOptions: mockBaseOptions(s)}
		o.client = testing.FakeClientSet(o.baseOptions.alterConfigMap, o.baseOptions.webhookConfigMap)
		o.names = []string{"receiver-7pb52"}
		Expect(o.run()).Should(Succeed())
	})
})

func mockBaseOptions(s genericclioptions.IOStreams) baseOptions {
	o := baseOptions{IOStreams: s}
	alertManagerConfig := `
    global: {}
    receivers:
    - name: default-receiver
    - name: receiver-7pb52
      webhook_configs:
      - max_alerts: 10
        url: http://kubeblocks-webhook-adaptor-config.default:5001/api/v1/notify/receiver-7pb52
    route:
      group_interval: 30s
      group_wait: 5s
      receiver: default-receiver
      repeat_interval: 10m
      routes:
      - continue: true
        matchers:
        - app_kubernetes_io_instance=~a|b|c
        - severity=~info|warning
        receiver: receiver-7pb52`
	webhookAdaptorConfig := `
    receivers:
    - name: receiver-7pb52
      params:
        url: https://oapi.dingtalk.com/robot/send?access_token=123456
      type: dingtalk-webhook`
	alertCM := mockConfigmap(alertConfigmapName, alertConfigFileName, alertManagerConfig)
	webhookAdaptorCM := mockConfigmap(webhookAdaptorName, webhookAdaptorFileName, webhookAdaptorConfig)
	o.alterConfigMap = alertCM
	o.webhookConfigMap = webhookAdaptorCM
	return o
}
