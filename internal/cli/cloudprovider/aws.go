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
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-exec/tfexec"
)

const (
	AWSDefaultRegion = "cn-north-1"
)

type awsCloudProvider struct {
	region       string
	accessKey    string
	accessSecret string

	awsPath     string
	clusterName string
}

func NewAWSCloudProvider(accessKey, accessSecret, region string) (Interface, error) {
	if accessKey == "" {
		return nil, fmt.Errorf("access key should be specified")
	}
	if accessSecret == "" {
		return nil, fmt.Errorf("access secret should be specified")
	}
	if region == "" {
		region = AWSDefaultRegion
	}
	provider := &awsCloudProvider{
		region:       region,
		accessKey:    accessKey,
		accessSecret: accessSecret,
	}
	return provider, nil
}

func (p *awsCloudProvider) Name() string {
	return AWS
}

// CreateK8sCluster create a EKS cluster
func (p *awsCloudProvider) CreateK8sCluster(name string) error {
	cpPath, err := GitRepoLocalPath()
	if err != nil {
		return err
	}
	p.awsPath = filepath.Join(cpPath, "aws")
	p.clusterName = name

	subPaths, err := getSubPaths(p.awsPath, []string{"eks", "lb"})
	if err != nil {
		return err
	}

	// init terraform
	if err = initTerraform(); err != nil {
		return err
	}

	// create EKS cluster
	if err = tfInitAndApply(subPaths[0], tfexec.Var("cluster_name="+p.clusterName), tfexec.Var("region="+p.region)); err != nil {
		return err
	}

	// install load balancer
	return tfInitAndApply(subPaths[1])
}

func (p *awsCloudProvider) DeleteK8sCluster(name string) error {
	subPaths, err := getSubPaths(p.awsPath, []string{"eks", "lb"})
	if err != nil {
		return err
	}

	// init terraform
	if err = initTerraform(); err != nil {
		return err
	}

	// destroy load balancer
	if err = tfInitAndApply(subPaths[1]); err != nil {
		return err
	}

	// destroy EKS cluster
	return tfDestroy(subPaths[0])
}

func getSubPaths(parent string, names []string) ([]string, error) {
	subPaths := make([]string, len(names))
	for i, name := range names {
		subPath := filepath.Join(parent, name)
		if _, err := os.Stat(subPath); err != nil {
			return nil, err
		}
		subPaths[i] = subPath
	}
	return subPaths, nil
}
