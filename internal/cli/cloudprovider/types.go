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
	"strings"
)

type outputKey string

// terraform output keys
const (
	clusterNameKey outputKey = "cluster_name"
	contextNameKey outputKey = "context_name"
	regionKey      outputKey = "region"
)

const (
	GitRepoName = "cloud-provider"
	GitRepoURL  = "https://github.com/apecloud/cloud-provider"
)

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

func CloudProviders() []string {
	return []string{Local, AWS, Azure, GCP, AlibabaCloud}
}

func K8sService(provider string) string {
	return cloudProviderK8sServiceMap[provider]
}

// K8sClusterInfo is the kubernetes cluster information for playground that will
// be serialized to a state file
type K8sClusterInfo struct {
	ClusterName   string `json:"cluster_name"`
	ContextName   string `json:"context_name"`
	KubeConfig    string `json:"kubeconfig"`
	CloudProvider string `json:"cloud_provider"`
	Region        string `json:"region,omitempty"`
}

// IsValid check if kubernetes cluster info is valid
func (c *K8sClusterInfo) IsValid() bool {
	if c.ClusterName == "" || c.CloudProvider == "" || (c.CloudProvider != Local && c.Region == "") {
		return false
	}
	return true
}

func (c *K8sClusterInfo) String() string {
	fields := []string{"  cloud provider: " + c.CloudProvider,
		"cluster name: " + c.ClusterName,
		"context name: " + c.ContextName,
		"kubeconfig: " + c.KubeConfig,
	}
	if c.CloudProvider != Local {
		fields = append(fields, "region: "+c.Region)
	}
	return strings.Join(fields, "\n  ")
}
