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

package nebula

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

var _ = Describe("Nebula Engine", func() {
	It("connection command", func() {
		nebula := NewCommands()

		Expect(nebula.ConnectCommand(nil)).ShouldNot(BeNil())
		authInfo := &engines.AuthInfo{
			UserName:   "user-test",
			UserPasswd: "pwd-test",
		}
		Expect(nebula.ConnectCommand(authInfo)).ShouldNot(BeNil())
	})

	It("connection example", func() {
		nebula := NewCommands().(*Commands)

		info := &engines.ConnectionInfo{
			User:     "user",
			Host:     "host",
			Password: "*****",
			Port:     "1234",
		}
		for k := range nebula.examples {
			fmt.Printf("%s Connection Example\n", k.String())
			Expect(nebula.ConnectExample(info, k.String())).ShouldNot(BeZero())
		}

		Expect(nebula.ConnectExample(info, "")).ShouldNot(BeZero())
	})
})
