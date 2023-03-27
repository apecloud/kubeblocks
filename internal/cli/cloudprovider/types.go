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
	"strings"

	"github.com/hashicorp/terraform-exec/tfexec"
)

type outputKey string

// terraform output keys
const (
	clusterNameKey outputKey = "cluster_name"
	regionKey      outputKey = "region"
	kubeConfigKey  outputKey = "kube_config"
)

const (
	GitRepoName = "cloud-provider"
	GitRepoURL  = "https://github.com/apecloud/cloud-provider"
)

const (
	Local        = "local"
	AWS          = "aws"
	AliCloud     = "alicloud"
	Azure        = "azure"
	GCP          = "gcp"
	TencentCloud = "tencentcloud"
)

var (
	cloudProviderK8sServiceMap = map[string]string{
		Local:        "k3s",
		AWS:          "eks",
		AliCloud:     "ack",
		Azure:        "aks",
		GCP:          "gke",
		TencentCloud: "tke",
	}
)

func CloudProviders() []string {
	return []string{Local, AWS, Azure, GCP, AliCloud, TencentCloud}
}

func K8sService(provider string) string {
	return cloudProviderK8sServiceMap[provider]
}

// K8sClusterInfo is the kubernetes cluster information for playground that will
// be serialized to a state file
type K8sClusterInfo struct {
	ClusterName   string `json:"cluster_name"`
	CloudProvider string `json:"cloud_provider"`
	Region        string `json:"region,omitempty"`
	KubeConfig    string `json:"kube_config,omitempty"`
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
		"kube_config: " + c.KubeConfig,
	}
	if c.CloudProvider != Local {
		fields = append(fields, "region: "+c.Region)
	}
	return strings.Join(fields, "\n  ")
}

func (c *K8sClusterInfo) buildApplyOpts() []tfexec.ApplyOption {
	return []tfexec.ApplyOption{tfexec.Var(fmt.Sprintf("%s=%s", clusterNameKey, c.ClusterName)),
		tfexec.Var(fmt.Sprintf("%s=%s", regionKey, c.Region))}
}

func (c *K8sClusterInfo) buildDestroyOpts() []tfexec.DestroyOption {
	return []tfexec.DestroyOption{tfexec.Var(fmt.Sprintf("%s=%s", clusterNameKey, c.ClusterName)),
		tfexec.Var(fmt.Sprintf("%s=%s", regionKey, c.Region))}
}
