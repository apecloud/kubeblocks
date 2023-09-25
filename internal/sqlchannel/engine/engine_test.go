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

package engine

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Engine", func() {
	It("new engine", func() {
		for _, typeName := range []string{stateMysql, statePostgreSQL, stateRedis, statePostgreSQL, stateNebula} {
			engine, _ := New(typeName)
			Expect(engine).ShouldNot(BeNil())

			url := engine.ConnectCommand(nil)
			Expect(len(url)).Should(Equal(3))
			// it is a tricky way to check the container name
			// for the moment, we only support mysql, postgresql and redis
			// and the container name is the same as the state name
			if typeName == stateMysql {
				// for wesql vtgate component we wuold use the first container, but its name is not mysql
				Expect(engine.Container()).Should(Equal(""))
			} else {
				Expect(engine.Container()).Should(ContainSubstring(typeName))
			}

		}
	})

	It("new unknown engine", func() {
		typeName := "unknown-type"
		engine, err := New(typeName)
		Expect(engine).Should(BeNil())
		Expect(err).Should(HaveOccurred())
	})

	It("new execute command ", func() {
		for _, typeName := range []string{stateMysql, statePostgreSQL, stateRedis} {
			engine, _ := New(typeName)
			Expect(engine).ShouldNot(BeNil())

			_, _, err := engine.ExecuteCommand([]string{"some", "cmd"})
			Expect(err).Should(Succeed())
		}
		for _, typeName := range []string{stateMongoDB, stateNebula} {
			engine, _ := New(typeName)
			Expect(engine).ShouldNot(BeNil())

			_, _, err := engine.ExecuteCommand([]string{"some", "cmd"})
			Expect(err).Should(HaveOccurred())
		}
	})
})
