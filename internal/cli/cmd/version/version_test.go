/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package version

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

const kbVersion = "0.3.0"

var mockDeploy = func(version string) *appsv1.Deployment {
	deploy := &appsv1.Deployment{}
	deploy.SetLabels(map[string]string{
		"app.kubernetes.io/name": types.KubeBlocksChartName,
	})
	if len(version) > 0 {
		deploy.Labels["app.kubernetes.io/version"] = version
	}
	return deploy
}

var _ = Describe("version", func() {
	It("version", func() {
		tf := cmdtesting.NewTestFactory()
		tf.Client = &fake.RESTClient{}
		By("testing version command")
		cmd := NewVersionCmd(tf)
		Expect(cmd).ShouldNot(BeNil())

		By("complete")
		o := &versionOptions{}
		Expect(o.Complete(tf)).Should(Succeed())

		By("testing run")
		client := testing.FakeClientSet(mockDeploy(kbVersion))
		o = &versionOptions{
			client: client,
		}
		o.Run()
	})
})
