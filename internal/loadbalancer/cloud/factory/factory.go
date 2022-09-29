package factory

import (
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud/aws"
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
