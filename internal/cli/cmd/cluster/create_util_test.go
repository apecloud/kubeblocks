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
	"k8s.io/apimachinery/pkg/runtime/schema"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("cluster create util", func() {
	const (
		namespace   = "test-ns"
		clusterType = "mysql"
	)

	Context("add create flags", func() {
		var cmd *cobra.Command
		var tf *cmdtesting.TestFactory

		BeforeEach(func() {
			cmd = &cobra.Command{
				Use: "test-cmd",
			}
			tf = testing.NewTestFactory(namespace)
		})

		It("add create flags for a nil schema", func() {
			Expect(addCreateFlags(cmd, tf, nil)).Should(Succeed())
		})

		It("add create flags for a not-nil schema", func() {
			c, err := cluster.BuildChartInfo(clusterType)
			Expect(err).Should(Succeed())

			Expect(err).Should(Succeed())
			Expect(c.Schema).ShouldNot(BeNil())
			Expect(addCreateFlags(cmd, tf, c)).Should(Succeed())
			Expect(cmd.Flags().Lookup("version")).ShouldNot(BeNil())
		})

		It("get kubernetes object info from manifests", func() {
			testCases := []struct {
				gvr      schema.GroupVersionResource
				manifest string
			}{
				{
					gvr: types.ServiceAccountGVR(),
					manifest: `apiVersion: v1
kind: ServiceAccount
metadata:
  name: kb-sa-test-cluster
  namespace: default`},
				{
					gvr: types.RoleGVR(),
					manifest: `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kb-role-test-cluster
  namespace: default`},
				{
					gvr: types.ClusterGVR(),
					manifest: `apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
  namespace: default`},
			}

			for _, tc := range testCases {
				By(tc.gvr.String())
				infos, err := getObjectsInfo(tf, map[string]string{
					"manifest": tc.manifest,
				})
				Expect(err).Should(Succeed())
				Expect(infos).ShouldNot(BeNil())
				Expect(infos[0].gvr).Should(Equal(tc.gvr))
			}
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

	It("build helm values", func() {
		By("get cluster schema")
		c, err := cluster.BuildChartInfo(clusterType)
		Expect(err).Should(Succeed())
		Expect(c).ShouldNot(BeNil())

		By("build helm values")
		values := map[string]interface{}{
			"version":           "1.0.0",
			"cpu":               1,
			"memory":            1,
			"terminationPolicy": "Halt",
		}
		helmValues := buildHelmValues(c, values)
		Expect(helmValues).ShouldNot(BeNil())
		Expect(helmValues["version"]).Should(Equal("1.0.0"))
		Expect(helmValues[c.SubChartName]).ShouldNot(BeNil())
		Expect(helmValues[c.SubChartName].(map[string]interface{})["terminationPolicy"]).Should(Equal("Halt"))
	})
})
