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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("Cluster", func() {
	const (
		testComponentPath                    = "../../testing/testdata/component.yaml"
		testComponentWithClassPath           = "../../testing/testdata/component_with_class_1c1g.yaml"
		testComponentWithInvalidClassPath    = "../../testing/testdata/component_with_invalid_class.yaml"
		testComponentWithResourcePath        = "../../testing/testdata/component_with_resource_1c1g.yaml"
		testComponentWithInvalidResourcePath = "../../testing/testdata/component_with_invalid_resource.yaml"
	)

	const (
		clusterName = "test"
		namespace   = "default"
	)
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(namespace)
		cd := testing.FakeClusterDef()
		fakeDefaultStorageClass := testing.FakeStorageClass(testing.StorageClassName, testing.IsDefautl)
		tf.FakeDynamicClient = testing.FakeDynamicClient(cd, fakeDefaultStorageClass, testing.FakeClusterVersion())
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	Context("create", func() {
		It("without name", func() {
			o := &CreateOptions{
				ClusterDefRef:     testing.ClusterDefName,
				ClusterVersionRef: testing.ClusterVersionName,
				SetFile:           testComponentPath,
				UpdatableFlags: UpdatableFlags{
					TerminationPolicy: "Delete",
				},
				CreateOptions: create.CreateOptions{
					Factory:   tf,
					Dynamic:   tf.FakeDynamicClient,
					IOStreams: streams,
				},
			}
			Expect(o.Validate()).To(Succeed())
			Expect(o.Name).ShouldNot(BeEmpty())
		})

		It("new command", func() {
			cmd := NewCreateCmd(tf, streams)
			Expect(cmd).ShouldNot(BeNil())
			Expect(cmd.Flags().Set("cluster-definition", testing.ClusterDefName)).Should(Succeed())
			Expect(cmd.Flags().Set("cluster-version", testing.ClusterVersionName)).Should(Succeed())
			Expect(cmd.Flags().Set("set-file", testComponentPath)).Should(Succeed())
			Expect(cmd.Flags().Set("termination-policy", "Delete")).Should(Succeed())

			// must succeed otherwise exit 1 and make test fails
			cmd.Run(nil, []string{"test1"})
		})

	})

	Context("run", func() {
		var o *CreateOptions

		BeforeEach(func() {
			clusterDef := testing.FakeClusterDef()
			tf.FakeDynamicClient = testing.FakeDynamicClient(
				clusterDef,
				testing.FakeStorageClass(testing.StorageClassName, testing.IsDefautl),
				testing.FakeClusterVersion(),
				testing.FakeComponentClassDef(fmt.Sprintf("custom-%s", testing.ComponentDefName), clusterDef.Name, testing.ComponentDefName),
				testing.FakeComponentClassDef("custom-mysql", clusterDef.Name, "mysql"),
			)
			o = &CreateOptions{
				CreateOptions: create.CreateOptions{
					IOStreams:       streams,
					Name:            clusterName,
					Dynamic:         tf.FakeDynamicClient,
					CueTemplateName: CueTemplateName,
					Factory:         tf,
					GVR:             types.ClusterGVR(),
				},
				SetFile:           "",
				ClusterDefRef:     testing.ClusterDefName,
				ClusterVersionRef: testing.ClusterVersionName,
				UpdatableFlags: UpdatableFlags{
					PodAntiAffinity: "Preferred",
					TopologyKeys:    []string{"kubernetes.io/hostname"},
					NodeLabels:      map[string]string{"testLabelKey": "testLabelValue"},
					TolerationsRaw:  []string{"key=engineType,value=mongo,operator=Equal,effect=NoSchedule"},
					Tenancy:         string(appsv1alpha1.SharedNode),
				},
			}
			o.TerminationPolicy = "WipeOut"
		})

		Run := func() {
			o.CreateOptions.Options = o
			o.Args = []string{clusterName}
			Expect(o.CreateOptions.Complete()).Should(Succeed())
			Expect(o.Namespace).To(Equal(namespace))
			Expect(o.Name).To(Equal(clusterName))
			Expect(o.Run()).Should(Succeed())
		}

		It("validate tolerations", func() {
			Expect(len(o.TolerationsRaw)).Should(Equal(1))
			Expect(o.Complete()).Should(Succeed())
			Expect(len(o.Tolerations)).Should(Equal(1))
		})

		It("validate termination policy should be set", func() {
			o.TerminationPolicy = ""
			Expect(o.Validate()).Should(HaveOccurred())
		})

		It("should succeed if component with valid class", func() {
			o.Values = []string{fmt.Sprintf("type=%s,class=%s", testing.ComponentDefName, testapps.Class1c1gName)}
			Expect(o.Complete()).Should(Succeed())
			Expect(o.Validate()).Should(Succeed())
			Run()
		})

		It("should fail if component with invalid class", func() {
			o.Values = []string{fmt.Sprintf("type=%s,class=class-not-exists", testing.ComponentDefName)}
			Expect(o.Complete()).Should(HaveOccurred())
		})

		It("should succeed if component with resource matching to one class", func() {
			o.Values = []string{fmt.Sprintf("type=%s,cpu=1,memory=1Gi", testing.ComponentDefName)}
			Expect(o.Complete()).Should(Succeed())
			Expect(o.Validate()).Should(Succeed())
			Run()
		})

		It("should succeed if component with resource equivalent to class", func() {
			o.Values = []string{fmt.Sprintf("type=%s,cpu=1000m,memory=1024Mi", testing.ComponentDefName)}
			Expect(o.Complete()).Should(Succeed())
			Expect(o.Validate()).Should(Succeed())
			Run()
		})

		It("should fail if component with resource not matching to any class", func() {
			o.Values = []string{fmt.Sprintf("type=%s,cpu=1,memory=2Gi", testing.ComponentDefName)}
			Expect(o.Complete()).Should(HaveOccurred())
		})

		It("should succeed if component with cpu matching one class", func() {
			o.Values = []string{fmt.Sprintf("type=%s,cpu=1", testing.ComponentDefName)}
			Expect(o.Complete()).Should(Succeed())
			Expect(o.Validate()).Should(Succeed())
			Run()
		})

		It("should fail if component with cpu not matching to any class", func() {
			o.Values = []string{fmt.Sprintf("type=%s,cpu=3", testing.ComponentDefName)}
			Expect(o.Complete()).Should(HaveOccurred())
		})

		It("should succeed if component with memory matching one class", func() {
			o.Values = []string{fmt.Sprintf("type=%s,memory=1Gi", testing.ComponentDefName)}
			Expect(o.Complete()).Should(Succeed())
			Expect(o.Validate()).Should(Succeed())
			Run()
		})

		It("should fail if component with memory not matching any class", func() {
			o.Values = []string{fmt.Sprintf("type=%s,memory=7Gi", testing.ComponentDefName)}
			Expect(o.Complete()).Should(HaveOccurred())
		})

		It("should succeed if component don't have class definition", func() {
			o.Values = []string{fmt.Sprintf("type=%s,cpu=3,memory=7Gi", testing.ExtraComponentDefName)}
			Expect(o.Complete()).Should(Succeed())
			Expect(o.Validate()).Should(Succeed())
			Run()
		})

		It("should fail if create cluster by file not existing", func() {
			o.SetFile = "test.yaml"
			Expect(o.Complete()).Should(HaveOccurred())
		})

		It("should succeed if create cluster by empty file", func() {
			o.SetFile = ""
			Expect(o.Complete()).Should(Succeed())
			Expect(o.Validate()).Should(Succeed())
			Run()
		})

		It("should succeed if create cluster by file without class and resource", func() {
			o.SetFile = testComponentPath
			Expect(o.Complete()).Should(Succeed())
			Expect(o.Validate()).Should(Succeed())
			Run()
		})

		It("should succeed if create cluster by file with class", func() {
			o.SetFile = testComponentWithClassPath
			Expect(o.Complete()).Should(Succeed())
			Expect(o.Validate()).Should(Succeed())
			Run()
		})

		It("should succeed if create cluster by file with resource", func() {
			o.SetFile = testComponentWithResourcePath
			Expect(o.Complete()).Should(Succeed())
			Expect(o.Validate()).Should(Succeed())
			Run()
		})

		It("should fail if create cluster by file with class not exists", func() {
			o.SetFile = testComponentWithInvalidClassPath
			Expect(o.Complete()).Should(HaveOccurred())
		})

		It("should fail if create cluster by file with resource not matching to any class", func() {
			o.SetFile = testComponentWithInvalidResourcePath
			Expect(o.Complete()).Should(HaveOccurred())
		})
	})

	Context("create validate", func() {
		var o *CreateOptions
		BeforeEach(func() {
			o = &CreateOptions{
				ClusterDefRef:     testing.ClusterDefName,
				ClusterVersionRef: testing.ClusterVersionName,
				SetFile:           testComponentPath,
				UpdatableFlags: UpdatableFlags{
					TerminationPolicy: "Delete",
				},
				CreateOptions: create.CreateOptions{
					Factory:   tf,
					Namespace: "default",
					Name:      "mycluster",
					Dynamic:   tf.FakeDynamicClient,
					IOStreams: streams,
				},
				ComponentSpecs: make([]map[string]interface{}, 1),
			}
			o.ComponentSpecs[0] = make(map[string]interface{})
			o.ComponentSpecs[0]["volumeClaimTemplates"] = make([]interface{}, 1)
			vct := o.ComponentSpecs[0]["volumeClaimTemplates"].([]interface{})
			vct[0] = make(map[string]interface{})
			vct[0].(map[string]interface{})["spec"] = make(map[string]interface{})
			spec := vct[0].(map[string]interface{})["spec"]
			spec.(map[string]interface{})["storageClassName"] = testing.StorageClassName
		})

		It("can validate whether the ClusterDefRef is null when create a new cluster ", func() {
			Expect(o.ClusterDefRef).ShouldNot(BeEmpty())
			Expect(o.Validate()).Should(Succeed())
			o.ClusterDefRef = ""
			Expect(o.Validate()).Should(HaveOccurred())
		})

		It("can validate whether the TerminationPolicy is null when create a new cluster ", func() {
			Expect(o.TerminationPolicy).ShouldNot(BeEmpty())
			Expect(o.Validate()).Should(Succeed())
			o.TerminationPolicy = ""
			Expect(o.Validate()).Should(HaveOccurred())
		})

		It("can validate whether the ClusterVersionRef is null and can't get latest version from client when create a new cluster ", func() {
			Expect(o.ClusterVersionRef).ShouldNot(BeEmpty())
			Expect(o.Validate()).Should(Succeed())
			o.ClusterVersionRef = ""
			Expect(o.Validate()).Should(Succeed())
		})

		It("can validate whether --set and --set-file both are specified when create a new cluster ", func() {
			Expect(o.SetFile).ShouldNot(BeEmpty())
			Expect(o.Values).Should(BeNil())
			Expect(o.Validate()).Should(Succeed())
			o.Values = []string{"notEmpty"}
			Expect(o.Validate()).Should(HaveOccurred())
		})

		It("can validate whether the name is not specified and fail to generate a random cluster name when create a new cluster ", func() {
			Expect(o.Name).ShouldNot(BeEmpty())
			Expect(o.Validate()).Should(Succeed())
			o.Name = ""
			// Expected to generate a random name
			Expect(o.Validate()).Should(Succeed())
		})

		It("can validate whether the name is not longer than 16 characters when create a new cluster", func() {
			Expect(len(o.Name)).Should(BeNumerically("<=", 16))
			Expect(o.Validate()).Should(Succeed())
			moreThan16 := 17
			bytes := make([]byte, 0)
			var clusterNameMoreThan16 string
			for i := 0; i < moreThan16; i++ {
				bytes = append(bytes, byte(i%26+'a'))
			}
			clusterNameMoreThan16 = string(bytes)
			Expect(len(clusterNameMoreThan16)).Should(BeNumerically(">", 16))
			o.Name = clusterNameMoreThan16
			Expect(o.Validate()).Should(HaveOccurred())
		})

		Context("validate storageClass", func() {
			It("can get all StorageClasses in K8S and check out if the cluster have a defalut StorageClasses by GetStorageClasses()", func() {
				storageClasses, existedDefault, err := getStorageClasses(o.Dynamic)
				Expect(err).Should(Succeed())
				Expect(storageClasses).Should(HaveKey(testing.StorageClassName))
				Expect(existedDefault).Should(BeTrue())
				fakeNotDefaultStorageClass := testing.FakeStorageClass(testing.StorageClassName, testing.IsNotDefault)
				cd := testing.FakeClusterDef()
				tf.FakeDynamicClient = testing.FakeDynamicClient(cd, fakeNotDefaultStorageClass, testing.FakeClusterVersion())
				storageClasses, existedDefault, err = getStorageClasses(tf.FakeDynamicClient)
				Expect(err).Should(Succeed())
				Expect(storageClasses).Should(HaveKey(testing.StorageClassName))
				Expect(existedDefault).ShouldNot(BeTrue())
			})

			It("can specify the StorageClass and the StorageClass must exist", func() {
				Expect(validateStorageClass(o.Dynamic, o.ComponentSpecs)).Should(Succeed())
				fakeNotDefaultStorageClass := testing.FakeStorageClass(testing.StorageClassName+"-other", testing.IsNotDefault)
				cd := testing.FakeClusterDef()
				FakeDynamicClientWithNotDefaultSC := testing.FakeDynamicClient(cd, fakeNotDefaultStorageClass, testing.FakeClusterVersion())
				Expect(validateStorageClass(FakeDynamicClientWithNotDefaultSC, o.ComponentSpecs)).Should(HaveOccurred())
			})

			It("can get valiate the default StorageClasses", func() {
				vct := o.ComponentSpecs[0]["volumeClaimTemplates"].([]interface{})
				spec := vct[0].(map[string]interface{})["spec"]
				delete(spec.(map[string]interface{}), "storageClassName")
				Expect(validateStorageClass(o.Dynamic, o.ComponentSpecs)).Should(Succeed())
				fakeNotDefaultStorageClass := testing.FakeStorageClass(testing.StorageClassName+"-other", testing.IsNotDefault)
				cd := testing.FakeClusterDef()
				FakeDynamicClientWithNotDefaultSC := testing.FakeDynamicClient(cd, fakeNotDefaultStorageClass, testing.FakeClusterVersion())
				Expect(validateStorageClass(FakeDynamicClientWithNotDefaultSC, o.ComponentSpecs)).Should(HaveOccurred())
			})
		})

	})

	It("delete", func() {
		cmd := NewDeleteCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("cluster", func() {
		cmd := NewClusterCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.HasSubCommands()).To(BeTrue())
	})

	It("connect", func() {
		cmd := NewConnectCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("list-logs-type", func() {
		cmd := NewListLogsCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("logs", func() {
		cmd := NewLogsCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})
})
