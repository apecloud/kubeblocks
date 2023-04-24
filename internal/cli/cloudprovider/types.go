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
