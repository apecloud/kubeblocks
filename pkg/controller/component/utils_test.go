/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package component

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("build service references", func() {
	Context("component utils", func() {
		It("comp def name matched test", func() {
			type compDefMatch struct {
				compDefPattern string
				compDef        string
			}
			tests := []struct {
				name   string
				fields compDefMatch
				want   bool
			}{{
				name: "version string test true",
				fields: compDefMatch{
					compDefPattern: "mysql-8.0.30-v1alpha1",
					compDef:        "mysql-8.0.30-v1alpha1",
				},
				want: true,
			}, {
				name: "version string test true",
				fields: compDefMatch{
					compDefPattern: "mysql-8.0.30",
					compDef:        "mysql-8.0.30-v1alpha1",
				},
				want: true,
			}, {
				name: "version string test false",
				fields: compDefMatch{
					compDefPattern: "mysql-8.0.30",
					compDef:        "mysql-8.0.29",
				},
				want: false,
			}, {
				name: "version string test false",
				fields: compDefMatch{
					compDefPattern: "^8.0.8$",
					compDef:        "v8.0.8",
				},
				want: false,
			}, {
				name: "version string test true",
				fields: compDefMatch{
					compDefPattern: "8.0.\\d{1,2}$",
					compDef:        "8.0.6",
				},
				want: true,
			}, {
				name: "version string test false",
				fields: compDefMatch{
					compDefPattern: "8.0.\\d{1,2}$",
					compDef:        "8.0.8.8.8",
				},
				want: false,
			}, {
				name: "version string test true",
				fields: compDefMatch{
					compDefPattern: "^[v\\-]*?(\\d{1,2}\\.){0,3}\\d{1,2}$",
					compDef:        "v-8.0.8.0",
				},
				want: true,
			}, {
				name: "version string test false",
				fields: compDefMatch{
					compDefPattern: "^[v\\-]*?(\\d{1,2}\\.){0,3}\\d{1,2}$",
					compDef:        "mysql-8.0.8",
				},
				want: false,
			}, {
				name: "version string test true",
				fields: compDefMatch{
					compDefPattern: "^mysql-8.0.\\d{1,2}$",
					compDef:        "mysql-8.0.8",
				},
				want: true,
			}, {
				name: "version string test false",
				fields: compDefMatch{
					compDefPattern: "mysql",
					compDef:        "abcmysql",
				},
				want: false,
			}, {
				name: "version string test false",
				fields: compDefMatch{
					compDefPattern: "mysql-",
					compDef:        "abc-mysql-",
				},
				want: false,
			}}
			for _, tt := range tests {
				match := CompDefMatched(tt.fields.compDef, tt.fields.compDefPattern)
				Expect(match).Should(Equal(tt.want))
			}
		})
	})
})
