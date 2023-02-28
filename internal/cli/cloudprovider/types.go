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

const (
	Local        = "local"
	AWS          = "aws"
	AlibabaCloud = "alibaba-cloud"
	Azure        = "azure"
	GCP          = "gcp"
)

var (
	cloudProviderK8sServiceMap = map[string]string{
		Local:        "k3s",
		AWS:          "eks",
		AlibabaCloud: "ack",
		Azure:        "aks",
		GCP:          "gke",
	}
)

const (
	GitRepoName = "cloud-provider"
	GitRepoURL  = "https://github.com/apecloud/cloud-provider"
)

func CloudProviders() []string {
	return []string{Local, AWS, Azure, GCP, AlibabaCloud}
}

func K8sService(provider string) string {
	return cloudProviderK8sServiceMap[provider]
}
