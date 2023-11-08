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
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic/fake"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/pkg/cli/scheme"
	"github.com/apecloud/kubeblocks/pkg/cli/testing"
)

var _ = Describe("backuprepo update command", func() {
	var streams genericiooptions.IOStreams
	var tf *cmdtesting.TestFactory
	var cmd *cobra.Command
	var options *updateOptions

	BeforeEach(func() {
		streams, _, _, _ = genericiooptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}

		secretObj := testing.FakeSecret("credential", testing.Namespace)

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
					"/api/v1/namespaces/fake-namespace/secrets/credential": httpResp(secretObj),
				}
				return mapping[req.URL.Path], nil
			}),
		}
		tf.Client = tf.UnstructuredClient

		options = &updateOptions{}
		cmd = newUpdateCommand(options, tf, streams)
		err := options.init(tf)
		Expect(err).ShouldNot(HaveOccurred())

		providerObj := testing.FakeStorageProvider("fake-storage-provider", nil)
		repoObj1 := testing.FakeBackupRepo("backuprepo-non-existent-provider", false)
		repoObj1.Spec.StorageProviderRef = "non-existent"
		repoObj2 := testing.FakeBackupRepo("default-backuprepo", true)
		repoObj3 := testing.FakeBackupRepo("test-backuprepo", false)
		repoObj3.Spec.Credential = &corev1.SecretReference{
			Name:      "credential",
			Namespace: testing.Namespace,
		}
		tf.FakeDynamicClient = fake.NewSimpleDynamicClient(
			scheme.Scheme, providerObj, repoObj1, repoObj2, repoObj3)
		err = options.init(tf)
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	Describe("parseFlags", func() {
		It("should fail if backuprepo name is not specified", func() {
			err := options.parseFlags(cmd, []string{}, tf)
			Expect(err).Should(MatchError(ContainSubstring("please specify the name of the backup repository to update")))
		})

		It("should fail if the backuprepo is not existing", func() {
			err := options.parseFlags(cmd, []string{"non-existent"}, tf)
			Expect(err).Should(MatchError(ContainSubstring("backup repository \"non-existent\" is not found")))
		})

		It("should fail if the referenced storage provider is not existing", func() {
			err := options.parseFlags(cmd, []string{"backuprepo-non-existent-provider"}, tf)
			Expect(err).Should(MatchError(ContainSubstring("storage provider \"non-existent\" is not found")))
		})

		It("should able to parse flags", func() {
			err := options.parseFlags(cmd, []string{"test-backuprepo"}, tf)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should fail to parse unknown flags", func() {
			err := options.parseFlags(cmd, []string{"test-backuprepo", "--foo", "abc"}, tf)
			Expect(err).Should(MatchError(ContainSubstring("unknown flag: --foo")))
		})

		It("should only parse flags about credential", func() {
			err := options.parseFlags(cmd, []string{"test-backuprepo", "--region", "us-west-2"}, tf)
			Expect(err).Should(MatchError(ContainSubstring("unknown flag: --region")))
		})

		It("should return ErrHelp if --help is specified", func() {
			err := options.parseFlags(cmd, []string{"test-backuprepo", "--help"}, tf)
			Expect(err).Should(MatchError(pflag.ErrHelp))
		})
	})

	Describe("complete", func() {
		It("should set fields in updateOptions", func() {
			err := options.parseFlags(cmd, []string{
				"test-backuprepo",
				"--access-key-id", "",
				"--secret-access-key", "def",
			}, tf)
			Expect(err).ShouldNot(HaveOccurred())
			err = options.complete(cmd)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(options.repoName).Should(Equal("test-backuprepo"))
			Expect(options.allValues).Should(Equal(map[string]string{
				"accessKeyId":     "",
				"secretAccessKey": "def",
			}))
			Expect(options.credential).Should(Equal(map[string]string{
				"accessKeyId":     "",
				"secretAccessKey": "def",
			}))
		})
	})

	Describe("validate", func() {
		BeforeEach(func() {
			err := options.parseFlags(cmd, []string{
				"test-backuprepo", "--access-key-id", "abc", "--secret-access-key", "def",
			}, tf)
			Expect(err).ShouldNot(HaveOccurred())
			err = options.complete(cmd)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should validate if there is a default backup repo", func() {
			options.isDefault = true
			options.hasDefaultFlag = true
			err := options.validate(cmd)
			Expect(err).Should(MatchError(ContainSubstring("there is already a default backup repo")))
		})
	})

	Describe("run", func() {
		It("should update the credential secret", func() {
			cmd.SetArgs([]string{"test-backuprepo", "--access-key-id", "foo", "--secret-access-key", "bar"})
			err := cmd.Execute()
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should unset the default backuprepo", func() {
			cmd.SetArgs([]string{"default-backuprepo", "--default=false"})
			err := cmd.Execute()
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should set the backuprepo as default", func() {
			providerObj := testing.FakeStorageProvider("fake-storage-provider", nil)
			repoObj := testing.FakeBackupRepo("test-backuprepo", false)
			tf.FakeDynamicClient = fake.NewSimpleDynamicClient(scheme.Scheme, providerObj, repoObj)
			err := options.init(tf)
			Expect(err).ShouldNot(HaveOccurred())

			cmd.SetArgs([]string{"test-backuprepo", "--default"})
			err = cmd.Execute()
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
