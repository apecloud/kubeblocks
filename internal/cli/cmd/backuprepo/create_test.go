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

package backuprepo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic/fake"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/scheme"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("backuprepo create command", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory
	var cmd *cobra.Command
	var options *createOptions

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}

		tf.UnstructuredClient = &clientfake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				urlPrefix := "/api/v1/namespaces/" + testing.Namespace
				if req.URL.Path == urlPrefix+"/secrets" && req.Method == http.MethodPost {
					dec := json.NewDecoder(req.Body)
					secret := &corev1.Secret{}
					_ = dec.Decode(secret)
					if secret.Name == "" && secret.GenerateName != "" {
						secret.Name = secret.GenerateName + "123456"
					}
					return httpResp(secret), nil
				}
				if strings.HasPrefix(req.URL.Path, urlPrefix+"/secrets") && req.Method == http.MethodPatch {
					return httpResp(&corev1.Secret{}), nil
				}
				mapping := map[string]*http.Response{
					"/api/v1/secrets": httpResp(&corev1.SecretList{}),
				}
				return mapping[req.URL.Path], nil
			}),
		}
		tf.Client = tf.UnstructuredClient

		options = &createOptions{}
		cmd = newCreateCommand(options, tf, streams)
		err := options.init(tf)
		Expect(err).ShouldNot(HaveOccurred())

		providerObj := testing.FakeStorageProvider("fake-s3")
		repoObj := testing.FakeBackupRepo("test-backuprepo", false)
		tf.FakeDynamicClient = fake.NewSimpleDynamicClient(
			scheme.Scheme, providerObj, repoObj)
		err = options.init(tf)
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	Describe("parseProviderFlags", func() {
		It("should fail if --provider is not specified", func() {
			err := options.parseProviderFlags(cmd, []string{}, tf)
			Expect(err).Should(MatchError(ContainSubstring("please specify the --provider flag")))
		})

		It("should fail if the specified provider is not existing", func() {
			err := options.parseProviderFlags(cmd, []string{"--provider", "non-existent"}, tf)
			Expect(err).Should(MatchError(ContainSubstring("storage provider \"non-existent\" is not found")))
		})

		It("should able to parse flags from the provider", func() {
			err := options.parseProviderFlags(cmd, []string{
				"--provider", "fake-s3",
				"--access-key-id", "abc",
				"--secret-access-key", "def",
			}, tf)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should fail to parse unknown flags", func() {
			err := options.parseProviderFlags(cmd, []string{"--provider", "fake-s3", "--foo", "abc"}, tf)
			Expect(err).Should(MatchError(ContainSubstring("unknown flag: --foo")))
		})

		It("should fail if required flags are not specified", func() {
			err := options.parseProviderFlags(cmd, []string{"--provider", "fake-s3"}, tf)
			Expect(err).Should(MatchError(ContainSubstring("required flag(s) \"access-key-id\", \"secret-access-key\" not set")))
		})

		It("should set isDefault field", func() {
			err := options.parseProviderFlags(cmd, []string{
				"--provider", "fake-s3", "--access-key-id", "abc", "--secret-access-key", "def", "--default",
			}, tf)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(options.isDefault).Should(BeTrue())
		})

		It("should return ErrHelp if --help is specified", func() {
			err := options.parseProviderFlags(cmd, []string{"--provider", "fake-s3", "--help"}, tf)
			Expect(err).Should(MatchError(pflag.ErrHelp))
		})
	})

	Describe("complete", func() {
		It("should set fields in createOptions", func() {
			err := options.parseProviderFlags(cmd, []string{
				"test-backuprepo",
				"--provider", "fake-s3",
				"--access-key-id", "abc",
				"--secret-access-key", "def",
				"--region", "us-west-2",
				"--bucket", "test-bucket",
			}, tf)
			Expect(err).ShouldNot(HaveOccurred())
			err = options.complete(cmd)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(options.repoName).Should(Equal("test-backuprepo"))
			Expect(options.allValues).Should(Equal(map[string]string{
				"accessKeyId":     "abc",
				"secretAccessKey": "def",
				"region":          "us-west-2",
				"bucket":          "test-bucket",
				"endpoint":        "",
				"mountOptions":    "",
			}))
			Expect(options.config).Should(Equal(map[string]string{
				"region":       "us-west-2",
				"bucket":       "test-bucket",
				"endpoint":     "",
				"mountOptions": "",
			}))
			Expect(options.credential).Should(Equal(map[string]string{
				"accessKeyId":     "abc",
				"secretAccessKey": "def",
			}))
		})
	})

	Describe("validate", func() {
		BeforeEach(func() {
			err := options.parseProviderFlags(cmd, []string{
				"--provider", "fake-s3", "--access-key-id", "abc", "--secret-access-key", "def",
			}, tf)
			Expect(err).ShouldNot(HaveOccurred())
			err = options.complete(cmd)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should validate parameters by the json schema", func() {
			options.allValues["region"] = "invalid-region"
			err := options.validate(cmd)
			Expect(err).Should(MatchError(fmt.Errorf("invalid flags")))
		})
		It("should validate pv reclaim policy", func() {
			options.pvReclaimPolicy = "whatever"
			err := options.validate(cmd)
			Expect(err).Should(MatchError(ContainSubstring("invalid --pv-reclaim-policy")))
		})
		It("should validate volume capacity", func() {
			options.volumeCapacity = "invalid"
			err := options.validate(cmd)
			Expect(err).Should(MatchError(ContainSubstring("invalid --volume-capacity")))
		})
		It("should validate the backup repo is not existing", func() {
			options.repoName = "test-backuprepo"
			err := options.validate(cmd)
			Expect(err).Should(MatchError(ContainSubstring("BackupRepo \"test-backuprepo\" is already exists")))
		})
		It("should validate if there is a default backup repo", func() {
			By("setting up a default backup repo")
			providerObj := testing.FakeStorageProvider("fake-s3")
			repoObj := testing.FakeBackupRepo("test-backuprepo", true)
			tf.FakeDynamicClient = fake.NewSimpleDynamicClient(
				scheme.Scheme, providerObj, repoObj)
			err := options.init(tf)
			Expect(err).ShouldNot(HaveOccurred())

			By("validating")
			options.isDefault = true
			err = options.validate(cmd)
			Expect(err).Should(MatchError(ContainSubstring("there is already a default backup repo")))
		})
	})

	Describe("run", func() {
		It("should success", func() {
			By("preparing the options")
			err := options.parseProviderFlags(cmd, []string{
				"--provider", "fake-s3", "--access-key-id", "abc", "--secret-access-key", "def",
				"--region", "us-west-1", "--bucket", "test-bucket", "--default",
			}, tf)
			Expect(err).ShouldNot(HaveOccurred())
			err = options.complete(cmd)
			Expect(err).ShouldNot(HaveOccurred())
			err = options.validate(cmd)
			Expect(err).ShouldNot(HaveOccurred())

			By("running the command")
			err = options.run()
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
