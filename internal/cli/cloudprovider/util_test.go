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

package cloudprovider

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var _ = Describe("cloud provider util", func() {
	It("clone git repo", func() {
		path, _ := GitRepoLocalPath()
		Expect(util.CloneGitRepo("https://github.com/apecloud/cloud-provider", path)).Should(Succeed())
	})

	It("test", func() {
		fmt.Println(rand.String(10))
	})
})
