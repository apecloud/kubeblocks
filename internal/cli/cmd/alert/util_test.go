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
