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
	"io"

	"github.com/pkg/errors"
)

type Interface interface {
	// Name return the cloud provider name
	Name() string

	// CreateK8sCluster creates a kubernetes cluster
	CreateK8sCluster(clusterInfo *K8sClusterInfo) error

	// DeleteK8sCluster deletes the created kubernetes cluster
	DeleteK8sCluster(clusterInfo *K8sClusterInfo) error

	// GetClusterInfo get cluster info
	GetClusterInfo() (*K8sClusterInfo, error)
}

func New(provider, tfRootPath string, stdout, stderr io.Writer) (Interface, error) {
	switch provider {
	case AWS, TencentCloud, AliCloud, GCP:
		return newCloudProvider(provider, tfRootPath, stdout, stderr)
	case Local:
		return newLocalCloudProvider(stdout, stderr), nil
	default:
		return nil, errors.New(fmt.Sprintf("Unknown cloud provider %s", provider))
	}
}
