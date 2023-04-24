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

package migration

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	cmdTest "k8s.io/kubectl/pkg/cmd/testing"

	app "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	v1alpha1 "github.com/apecloud/kubeblocks/internal/cli/types/migrationapi"
)

var (
	streams genericclioptions.IOStreams
	out     *bytes.Buffer
	tf      *cmdTest.TestFactory
)

const (
	namespace = "test"
)

var _ = Describe("create", func() {
	o := &CreateMigrationOptions{}

	BeforeEach(func() {
		streams, _, out, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)

		_ = app.AddToScheme(scheme.Scheme)

		tf.Client = tf.UnstructuredClient
	})

	Context("Input params validate", func() {
		var err error
		errMsgArr := make([]string, 0, 3)
		It("Endpoint with database", func() {
			o.Source = "user:123456@127.0.0.1:5432/database"
			err = o.SourceEndpointModel.BuildFromStr(&errMsgArr, o.Source)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.SourceEndpointModel.UserName).Should(Equal("user"))
			Expect(o.SourceEndpointModel.Password).Should(Equal("123456"))
			Expect(o.SourceEndpointModel.Address).Should(Equal("127.0.0.1:5432"))
			Expect(o.SourceEndpointModel.Database).Should(Equal("database"))
			Expect(len(errMsgArr)).Should(Equal(0))

			o.Sink = "user:123456127.0.0.1:5432/database"
			err = o.SinkEndpointModel.BuildFromStr(&errMsgArr, o.Sink)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(errMsgArr)).Should(Equal(1))
		})

		It("Endpoint with no database", func() {
			o.Source = "user:123456@127.0.0.1:3306"
			errMsgArr := make([]string, 0, 3)
			err = o.SourceEndpointModel.BuildFromStr(&errMsgArr, o.Source)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.SourceEndpointModel.UserName).Should(Equal("user"))
			Expect(o.SourceEndpointModel.Password).Should(Equal("123456"))
			Expect(o.SourceEndpointModel.Address).Should(Equal("127.0.0.1:3306"))
			Expect(o.SourceEndpointModel.Database).Should(BeEmpty())
			Expect(len(errMsgArr)).Should(Equal(0))

			o.Sink = "user:123456127.0.0.1:3306"
			err = o.SinkEndpointModel.BuildFromStr(&errMsgArr, o.Sink)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(errMsgArr)).Should(Equal(1))
		})

		It("MigrationObject", func() {
			o.MigrationObject = []string{"schema_public.table1", "schema2.table2_1", "schema2.table2_2", "schema3"}
			err = o.MigrationObjectModel.BuildFromStrs(&errMsgArr, o.MigrationObject)
			Expect(err).ShouldNot(HaveOccurred())
			for _, obj := range o.MigrationObjectModel.WhiteList {
				Expect(obj.SchemaName).Should(BeElementOf("schema_public", "schema2", "schema3"))
				switch obj.SchemaName {
				case "schema_public":
					Expect(len(obj.TableList)).Should(Equal(1))
					Expect(obj.TableList[0].TableName).Should(Equal("table1"))
					Expect(obj.TableList[0].IsAll).Should(BeTrue())
				case "schema2":
					Expect(len(obj.TableList)).Should(Equal(2))
					for _, tb := range obj.TableList {
						Expect(tb.TableName).Should(BeElementOf("table2_1", "table2_2"))
						Expect(tb.IsAll).Should(BeTrue())
					}
				case "schema3":
					Expect(obj.IsAll).Should(BeTrue())
				}
			}
		})

		It("Steps", func() {
			o.Steps = make([]string, 0)
			err = o.BuildWithSteps(&errMsgArr)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.TaskType).Should(Equal(InitializationAndCdc.String()))
			Expect(o.StepsModel).Should(ContainElements(v1alpha1.StepPreCheck.String(), v1alpha1.StepStructPreFullLoad.String(), v1alpha1.StepFullLoad.String()))
			o.Steps = []string{"precheck=true", "init-struct=false", "cdc=false"}
			err = o.BuildWithSteps(&errMsgArr)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.TaskType).Should(Equal(Initialization.String()))
			Expect(o.StepsModel).Should(ContainElements(v1alpha1.StepPreCheck.String(), v1alpha1.StepFullLoad.String()))
		})

		It("Tolerations", func() {
			o.Tolerations = []string{
				"step=global,key=engineType,value=pg,operator=Equal,effect=NoSchedule",
				"step=init-data,key=engineType,value=pg1,operator=Equal,effect=NoSchedule",
				"key=engineType,value=pg2,operator=Equal,effect=NoSchedule",
			}
			err = o.BuildWithTolerations()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.TolerationModel[v1alpha1.CliStepGlobal.String()]).ShouldNot(BeEmpty())
			Expect(o.TolerationModel[v1alpha1.CliStepInitData.String()]).ShouldNot(BeEmpty())
			Expect(len(o.TolerationModel[v1alpha1.CliStepInitData.String()])).Should(Equal(1))
			Expect(len(o.TolerationModel[v1alpha1.CliStepGlobal.String()])).Should(Equal(2))
			Expect(len(o.TolerationModel[v1alpha1.CliStepPreCheck.String()])).Should(Equal(0))
		})

		It("Resources", func() {
			o.Resources = []string{
				"step=global,cpu=1000m,memory=1Gi",
				"step=init-data,cpu=2000m,memory=2Gi",
				"cpu=3000m,memory=3Gi",
			}
			err = o.BuildWithResources()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(o.ResourceModel[v1alpha1.CliStepGlobal.String()]).ShouldNot(BeEmpty())
			Expect(o.ResourceModel[v1alpha1.CliStepInitData.String()]).ShouldNot(BeEmpty())
			Expect(o.ResourceModel[v1alpha1.CliStepPreCheck.String()]).Should(BeEmpty())
		})

		It("RuntimeParams", func() {
			type void struct{}
			var setValue void
			serverIDSet := make(map[uint32]void)

			loopCount := 0
			for loopCount < 1000 {
				newServerID := o.generateRandomMySQLServerID()
				Expect(newServerID >= 10001 && newServerID <= 1<<32-10001).Should(BeTrue())
				serverIDSet[newServerID] = setValue

				loopCount += 1
			}
			Expect(len(serverIDSet) > 500).Should(BeTrue())
		})
	})

	Context("Mock run", func() {
		It("test", func() {
			cmd := NewMigrationCreateCmd(tf, streams)
			Expect(cmd).ShouldNot(BeNil())
		})
	})
})
