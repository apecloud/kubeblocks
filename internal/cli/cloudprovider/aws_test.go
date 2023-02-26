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
)

var _ = Describe("aws cloud provider", func() {
	It("get cluster name from state file", func() {
		name, _ := getClusterNameFromStateFile("/Users/ldm/.kbcli/playground/cloud-provider/aws/")
		fmt.Println(name)
	})

	It("update kube config", func() {
		provider := awsCloudProvider{
			region: "cn-northwest-1",
		}
		_, _ = provider.UpdateKubeconfig("liutang-eks-nx")
	})
})
