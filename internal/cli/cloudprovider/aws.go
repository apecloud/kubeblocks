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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/pkg/errors"
)

type awsCloudProvider struct {
	region string
	stdout io.Writer
	stderr io.Writer

	tfRootPath  string
	awsPath     string
	clusterName string
}

var _ Interface = &awsCloudProvider{}

func NewAWSCloudProvider(region, tfRootPath string, stdout, stderr io.Writer) (Interface, error) {
	// check aws path exists
	awsPath := filepath.Join(tfRootPath, "aws")
	if _, err := os.Stat(awsPath); err != nil {
		return nil, err
	}

	provider := &awsCloudProvider{
		region:     region,
		stdout:     stdout,
		stderr:     stderr,
		awsPath:    awsPath,
		tfRootPath: tfRootPath,
	}
	return provider, nil
}

func (p *awsCloudProvider) Name() string {
	return AWS
}

// CreateK8sCluster create a EKS cluster
func (p *awsCloudProvider) CreateK8sCluster(name string, init bool) error {
	p.clusterName = name

	subPaths, err := getSubPaths(p.awsPath, []string{"eks", "lb"})
	if err != nil {
		return err
	}

	// init terraform
	fmt.Fprintf(p.stdout, "Check and install terraform ... \n")
	if err = initTerraform(); err != nil {
		return err
	}

	// create EKS cluster
	fmt.Fprintf(p.stdout, "\nInit and apply eks in %s\n", subPaths[0])
	if err = tfInitAndApply(subPaths[0], init, p.stdout, p.stderr, p.buildApplyOpts()...); err != nil {
		return err
	}

	// install load balancer
	fmt.Fprintf(p.stdout, "\nInit and apply loadbalancer in %s\n", subPaths[1])
	return tfInitAndApply(subPaths[1], init, p.stdout, p.stderr, tfexec.Var("cluster_name="+p.clusterName))
}

func (p *awsCloudProvider) DeleteK8sCluster(name string) error {
	p.clusterName = name
	subPaths, err := getSubPaths(p.awsPath, []string{"eks", "lb"})
	if err != nil {
		return err
	}

	// init terraform
	fmt.Fprintf(p.stdout, "Check and install terraform ... \n")
	if err = initTerraform(); err != nil {
		return err
	}

	// destroy load balancer
	fmt.Fprintf(p.stdout, "\nDestroy loadbalancer in %s\n", subPaths[1])
	if err = tfDestroy(subPaths[1], p.stdout, p.stderr, tfexec.Var("cluster_name="+p.clusterName)); err != nil {
		fmt.Fprintln(p.stdout, err.Error())
	}

	// destroy EKS cluster
	fmt.Fprintf(p.stdout, "\nDestroy eks cluster in %s\n", subPaths[0])
	return tfDestroy(subPaths[0], p.stdout, p.stderr, p.buildDestroyOpts()...)
}

// GetExistedClusters get existed clusters
func (p *awsCloudProvider) GetExistedClusters() ([]string, error) {
	subPaths, err := getSubPaths(p.awsPath, []string{"eks", "lb"})
	if err != nil {
		return nil, err
	}

	clusterName, err := getClusterNameFromStateFile(subPaths[0])
	if err != nil {
		return nil, err
	}

	// previous cluster exists, try to destroy it
	if clusterName == "" {
		return nil, nil
	}
	return []string{clusterName}, nil
}

func (p *awsCloudProvider) buildApplyOpts() []tfexec.ApplyOption {
	return []tfexec.ApplyOption{tfexec.Var("cluster_name=" + p.clusterName), tfexec.Var("region=" + p.region)}
}

func (p *awsCloudProvider) buildDestroyOpts() []tfexec.DestroyOption {
	return []tfexec.DestroyOption{tfexec.Var("cluster_name=" + p.clusterName), tfexec.Var("region=" + p.region)}
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

func getClusterNameFromStateFile(tfPath string) (string, error) {
	return getOutputValue("cluster_name", tfPath)
}

func getOutputValue(key string, tfPath string) (string, error) {
	stateFile := filepath.Join(tfPath, tfStateFileName)
	content, err := os.ReadFile(stateFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	var state map[string]interface{}
	if err = json.Unmarshal(content, &state); err != nil {
		return "", err
	}
	outputs, ok := state["outputs"].(map[string]interface{})
	if !ok {
		return "", nil
	}

	value, ok := outputs[key].(map[string]interface{})
	if !ok {
		return "", nil
	}
	return value["value"].(string), nil
}
