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

package factory

import (
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud"
	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud/aws"
)

type newFunc func(...interface{}) (cloud.Provider, error)

var (
	lock      sync.RWMutex
	providers = make(map[string]newFunc)
)

func init() {
	RegisterProvider(cloud.ProviderAWS, func(args ...interface{}) (cloud.Provider, error) {
		return aws.NewAwsService(args[0].(logr.Logger))
	})
}

func NewProvider(name string, logger logr.Logger) (cloud.Provider, error) {
	lock.RLock()
	defer lock.RUnlock()
	f, ok := providers[name]
	if !ok {
		return nil, errors.New("Unknown cloud provider")
	}
	return f(logger)
}

func RegisterProvider(name string, f newFunc) {
	lock.Lock()
	defer lock.Unlock()
	if _, ok := providers[name]; ok {
		panic(fmt.Sprintf("Cloud provider %s exists", name))
	}
	providers[name] = f
}
