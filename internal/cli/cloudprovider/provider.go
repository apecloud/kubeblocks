/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
	"os"
	"path/filepath"
)

type cloudProvider struct {
	name   string
	tfPath string

	stdout io.Writer
	stderr io.Writer
}

var _ Interface = &cloudProvider{}

func NewCloudProvider(provider, rootPath string, stdout, stderr io.Writer) (Interface, error) {
	k8sSvc := K8sService(provider)
	if k8sSvc == "" {
		return nil, fmt.Errorf("unknown cloud provider %s", provider)
	}

	tfPath := filepath.Join(rootPath, provider, k8sSvc)
	if _, err := os.Stat(tfPath); err != nil {
		return nil, err
	}

	return &cloudProvider{
		name:   provider,
		tfPath: tfPath,
		stdout: stdout,
		stderr: stderr,
	}, nil
}

func (p *cloudProvider) Name() string {
	return p.name
}

// CreateK8sCluster create a kubernetes cluster
func (p *cloudProvider) CreateK8sCluster(clusterInfo *K8sClusterInfo) error {
	// init terraform
	fmt.Fprintf(p.stdout, "Check and install terraform... \n")
	if err := initTerraform(); err != nil {
		return err
	}

	// create cluster
	fmt.Fprintf(p.stdout, "\nInit and apply %s in %s\n", K8sService(p.name), p.tfPath)
	return tfInitAndApply(p.tfPath, p.stdout, p.stderr, clusterInfo.buildApplyOpts()...)
}

func (p *cloudProvider) DeleteK8sCluster(clusterInfo *K8sClusterInfo) error {
	var err error
	if clusterInfo == nil {
		clusterInfo, err = p.GetClusterInfo()
		if err != nil {
			return err
		}
	}
	// init terraform
	fmt.Fprintf(p.stdout, "Check and install terraform... \n")
	if err = initTerraform(); err != nil {
		return err
	}

	// destroy cluster
	fmt.Fprintf(p.stdout, "\nDestroy %s cluster in %s\n", K8sService(p.name), p.tfPath)
	return tfInitAndDestroy(p.tfPath, p.stdout, p.stderr, clusterInfo.buildDestroyOpts()...)
}

func (p *cloudProvider) GetClusterInfo() (*K8sClusterInfo, error) {
	vals, err := getOutputValues(p.tfPath, clusterNameKey, regionKey, kubeConfigKey)
	if err != nil {
		return nil, err
	}
	return &K8sClusterInfo{
		CloudProvider: p.Name(),
		ClusterName:   vals[0],
		Region:        vals[1],
		KubeConfig:    vals[2],
	}, nil
}
