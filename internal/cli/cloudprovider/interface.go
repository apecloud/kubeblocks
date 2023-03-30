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
	"io"

	"github.com/pkg/errors"
)

type Interface interface {
	// Name return the cloud provider name
	Name() string

	// CreateK8sCluster creates a kubernetes cluster
	CreateK8sCluster(clusterInfo *K8sClusterInfo, init bool) error

	// DeleteK8sCluster deletes the created kubernetes cluster
	DeleteK8sCluster(clusterInfo *K8sClusterInfo) error

	// GetClusterInfo get cluster info
	GetClusterInfo() (*K8sClusterInfo, error)
}

func New(provider, tfRootPath string, stdout, stderr io.Writer) (Interface, error) {
	switch provider {
	case AWS, TencentCloud, AliCloud, GCP:
		return NewCloudProvider(provider, tfRootPath, stdout, stderr)
	case Local:
		return NewLocalCloudProvider(stdout, stderr), nil
	default:
		return nil, errors.New(fmt.Sprintf("Unknown cloud provider %s", provider))
	}
}
