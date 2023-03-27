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

// CreateK8sCluster create a EKS cluster
func (p *cloudProvider) CreateK8sCluster(clusterInfo *K8sClusterInfo, init bool) error {
	// init terraform
	fmt.Fprintf(p.stdout, "Check and install terraform... \n")
	if err := initTerraform(); err != nil {
		return err
	}

	// create cluster
	fmt.Fprintf(p.stdout, "\nInit and apply %s in %s\n", K8sService(p.name), p.tfPath)
	return tfInitAndApply(p.tfPath, init, p.stdout, p.stderr, clusterInfo.buildApplyOpts()...)
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
	return tfDestroy(p.tfPath, p.stdout, p.stderr, clusterInfo.buildDestroyOpts()...)
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
