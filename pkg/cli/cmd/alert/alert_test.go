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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var _ = Describe("alter", func() {
	f := cmdtesting.NewTestFactory()
	s, _, _, _ := genericiooptions.NewTestIOStreams()
	defer f.Cleanup()

	It("create new alert cmd", func() {
		cmd := NewAlertCmd(f, s)
		Expect(cmd).NotTo(BeNil())
		Expect(cmd.HasSubCommands()).Should(BeTrue())
	})
})
