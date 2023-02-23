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
	"os"
	"path"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/pkg/errors"
)

type Interface interface {
	// Name return the cloud provider name
	Name() string

	// CreateK8sCluster creates a kubernetes cluster
	CreateK8sCluster(name string) error

	// DeleteK8sCluster deletes a kubernetes cluster
	DeleteK8sCluster(name string) error
}

type Config struct {
	Name         string `json:"name"`
	AccessKey    string `json:"access_key"`
	AccessSecret string `json:"access_secret"`
	Region       string `json:"region"`
}

var (
	defaultProvider Interface
)

func init() {
	defaultProvider = &localCloudProvider{}
}

func initProvider() error {
	if err := os.MkdirAll(path.Dir(providerCfg), os.FileMode(0700)); err != nil {
		panic(errors.Wrap(err, "Failed to make provider config directory"))
	}
	if _, err := os.Stat(providerCfg); err != nil {
		if !os.IsNotExist(err) {
			panic(errors.Wrap(err, fmt.Sprintf("Failed to check if %s exists", providerCfg)))
		}

		defaultProvider, _ = New(Local, "", "", "")
		return nil
	}
	content, err := os.ReadFile(providerCfg)
	if err != nil {
		return errors.Wrap(err, "Failed to read provider configs")
	}
	cfg := Config{}
	if err := json.Unmarshal(content, &cfg); err != nil {
		return errors.Wrap(err, "Invalid cloud provider config, please destroy and try init playground again")
	}

	provider, err := New(cfg.Name, cfg.AccessKey, cfg.AccessSecret, cfg.Region)
	if err != nil {
		return errors.Wrap(err, "Failed to init cloud provider")
	}
	defaultProvider = provider
	return nil
}

func Get() (Interface, error) {
	err := initProvider()
	return defaultProvider, err
}

func InitProvider(provider, accessKey, accessSecret, region string) (Interface, error) {
	if err := initProvider(); err != nil {
		return nil, err
	}
	if defaultProvider.Name() != Local {
		fmt.Printf("Cloud Provider %s has already inited, skip", provider)
		return defaultProvider, nil
	}

	cfg := &Config{
		Name:         provider,
		AccessKey:    accessKey,
		AccessSecret: accessSecret,
		Region:       region,
	}

	result, err := json.Marshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to serialize cloud provider config")
	}
	if err := ioutils.AtomicWriteFile(providerCfg, result, os.FileMode(0600)); err != nil {
		return nil, errors.Wrap(err, "Failed to write cloud provider config")
	}
	return New(provider, accessKey, accessSecret, region)
}

func New(provider, accessKey, accessSecret, region string) (Interface, error) {
	switch provider {
	case AWS:
		return NewAWSCloudProvider(accessKey, accessSecret, region)
	case Local:
		return &localCloudProvider{}, nil
	default:
		return nil, errors.New(fmt.Sprintf("Unknown cloud provider %s", provider))
	}
}
