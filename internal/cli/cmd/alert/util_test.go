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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("alter", func() {
	const (
		webhookURL = "https://oapi.dingtalk.com/robot/send?access_token=123456"
	)
	It("string to map", func() {
		key := "url"
		str := key + "=" + webhookURL
		res := strToMap(str)
		Expect(res).ShouldNot(BeNil())
		Expect(res["url"]).Should(Equal(webhookURL))
	})

	It("get url webhook type", func() {
		testCases := []struct {
			url      string
			expected webhookType
		}{
			{url: "", expected: unknownWebhookType},
			{url: "https://test.com", expected: unknownWebhookType},
			{url: webhookURL, expected: dingtalkWebhookType},
			{url: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=123456", expected: wechatWebhookType},
			{url: "https://open.feishu.cn/open-apis/bot/v2/hook/123456", expected: feishuWebhookType},
		}
		for _, tc := range testCases {
			webhookType := getWebhookType(tc.url)
			Expect(webhookType).Should(Equal(tc.expected))
		}
	})

	It("remove duplicate string from slice", func() {
		slice := []string{"a", "b", "a", "c"}
		res := removeDuplicateStr(slice)
		Expect(res).ShouldNot(BeNil())
		Expect(res).Should(Equal([]string{"a", "b", "c"}))
	})

	It("url validation", func() {
		testCases := []struct {
			url      string
			expected bool
		}{
			{url: "", expected: false},
			{url: "https://test.com", expected: true},
			{url: "/foo/bar", expected: true},
			{url: "\"https://test.com\"", expected: false},
		}
		for _, tc := range testCases {
			By(fmt.Sprintf("url: %s, expected: %t", tc.url, tc.expected))
			res, _ := urlIsValid(tc.url)
			Expect(res).Should(Equal(tc.expected))
		}
	})
})
