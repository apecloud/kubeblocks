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

package cluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
)

var _ = Describe("cluster create util", func() {
	const (
		namespace = "test-ns"
	)

	Context("add create flags", func() {
		var cmd *cobra.Command
		var tf *cmdtesting.TestFactory

		BeforeEach(func() {
			cmd = &cobra.Command{
				Use: "test-cmd",
			}
			tf = cmdtesting.NewTestFactory().WithNamespace(namespace)
		})

		It("add create flags for a legal engine type", func() {
			Expect(addCreateFlags(cmd, tf, cluster.MySQL)).Should(Succeed())
			Expect(cmd.Flag("version")).ShouldNot(BeNil())
		})

		It("add create flags for an illegal engine type", func() {
			Expect(addCreateFlags(cmd, tf, "test-engine-type")).Should(HaveOccurred())
		})
	})

	It("get values from command flags", func() {
		fs := &flag.FlagSet{}
		flags := []struct {
			tpe   string
			name  string
			value interface{}
		}{
			{
				tpe:   "int",
				name:  "int",
				value: 1,
			},
			{
				tpe:   "bool",
				name:  "bool",
				value: true,
			},
			{
				tpe:   "float64",
				name:  "float64",
				value: 1.1,
			},
			{
				tpe:   "string",
				name:  "hello",
				value: "Hello, KubeBlocks",
			},
		}

		for _, f := range flags {
			switch f.tpe {
			case "int":
				fs.Int(f.name, f.value.(int), f.name)
			case "bool":
				fs.Bool(f.name, f.value.(bool), f.name)
			case "float64":
				fs.Float64(f.name, f.value.(float64), f.name)
			default:
				fs.String(f.name, f.value.(string), f.name)
			}
		}

		values := getValuesFromFlags(fs)
		for _, f := range flags {
			Expect(values[f.name]).Should(Equal(f.value))
		}
	})

	It("get dry run strategy", func() {
		testCases := []struct {
			input    string
			expected create.DryRunStrategy
		}{
			{
				input:    "",
				expected: create.DryRunNone,
			},
			{
				input:    "client",
				expected: create.DryRunClient,
			},
			{
				input:    "server",
				expected: create.DryRunServer,
			},
			{
				input:    "unchanged",
				expected: create.DryRunClient,
			},
			{
				input:    "none",
				expected: create.DryRunNone,
			},
			{
				input:    "unknown",
				expected: create.DryRunNone,
			},
		}

		for _, t := range testCases {
			res, err := getDryRunStrategy(t.input)
			Expect(res).Should(Equal(t.expected))
			if t.input == "unknown" {
				Expect(err).Should(HaveOccurred())
			} else {
				Expect(err).Should(Succeed())
			}
		}
	})
})
