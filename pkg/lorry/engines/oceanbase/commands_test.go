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

package oceanbase

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

var _ = Describe("Oceanbase Engine", func() {
	It("connection command", func() {
		oceanbase := NewCommands()

		Expect(oceanbase.ConnectCommand(nil)).ShouldNot(BeNil())
		authInfo := &engines.AuthInfo{
			UserName:   "user-test",
			UserPasswd: "pwd-test",
		}
		Expect(oceanbase.ConnectCommand(authInfo)).ShouldNot(BeNil())
	})

	It("connection example", func() {
		oceanbase := NewCommands().(*Commands)

		info := &engines.ConnectionInfo{
			User:     "user",
			Host:     "host",
			Password: "*****",
			Port:     "1234",
		}
		for k := range oceanbase.examples {
			fmt.Printf("%s Connection Example\n", k.String())
			Expect(oceanbase.ConnectExample(info, k.String())).ShouldNot(BeZero())
		}

		Expect(oceanbase.ConnectExample(info, "")).ShouldNot(BeZero())
	})

	It("execute command", func() {
		oceanbase := NewCommands()

		cmd, _, err := oceanbase.ExecuteCommand(nil)
		Expect(err).Should(BeNil())
		Expect(cmd).ShouldNot(BeNil())
		engines.EnvVarMap[engines.PASSWORD] = ""
		cmd, _, err = oceanbase.ExecuteCommand(nil)
		Expect(err).Should(BeNil())
		Expect(cmd).ShouldNot(BeNil())
	})
})
