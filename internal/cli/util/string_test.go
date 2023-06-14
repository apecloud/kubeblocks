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

package util

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("string utils", func() {
	It("convert to kebab case", func() {
		testCases := []struct {
			name     string
			expected string
		}{
			{
				name:     "hostNetworkAccessible",
				expected: "host-network-accessible",
			},
			{
				name:     "HostNetworkAccessible",
				expected: "host-network-accessible",
			},
		}
		for _, tc := range testCases {
			Expect(ToKebabCase(tc.name)).Should(Equal(tc.expected))
		}
	})

	It("convert to lower camel case", func() {
		testCases := []struct {
			name     string
			expected string
		}{
			{
				name:     "host-network-accessible",
				expected: "hostNetworkAccessible",
			},
			{
				name:     "Host-Network-Accessible",
				expected: "hostNetworkAccessible",
			},
		}
		for _, tc := range testCases {
			Expect(ToLowerCamelCase(tc.name)).Should(Equal(tc.expected))
		}
	})
})
