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

package engines

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Engine", func() {
	It("new engine", func() {
		for _, engineType := range []EngineType{MySQL, PostgreSQL, Redis, PostgreSQL, Nebula, FoxLake} {
			typeName := string(engineType)
			engine, _ := newClusterCommands(typeName)
			Expect(engine).ShouldNot(BeNil())

			url := engine.ConnectCommand(nil)
			Expect(len(url)).Should(Equal(3))
			// it is a tricky way to check the container name
			// for the moment, we only support mysql, postgresql and redis
			// and the container name is the same as the state name
			if typeName == string(MySQL) {
				// for wesql vtgate component we wuold use the first container, but its name is not mysql
				Expect(engine.Container()).Should(Equal(""))
			} else {
				Expect(engine.Container()).Should(ContainSubstring(typeName))
			}

		}
	})

	It("new unknown engine", func() {
		typeName := "unknown-type"
		engine, err := newClusterCommands(typeName)
		Expect(engine).Should(BeNil())
		Expect(err).Should(HaveOccurred())
	})

	It("new execute command ", func() {
		for _, engineType := range []EngineType{MySQL, PostgreSQL, Redis} {
			typeName := string(engineType)
			engine, _ := newClusterCommands(typeName)
			Expect(engine).ShouldNot(BeNil())

			_, _, err := engine.ExecuteCommand([]string{"some", "cmd"})
			Expect(err).Should(Succeed())
		}
		for _, engineType := range []EngineType{MongoDB, Nebula} {
			typeName := string(engineType)
			engine, _ := newClusterCommands(typeName)
			Expect(engine).ShouldNot(BeNil())

			_, _, err := engine.ExecuteCommand([]string{"some", "cmd"})
			Expect(err).Should(HaveOccurred())
		}
	})
})
