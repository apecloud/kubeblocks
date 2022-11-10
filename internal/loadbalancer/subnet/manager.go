package subnet

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
)

const (
	LoadBalancerConfigMapName = "loadbalancer-config"
)

type Manager interface {
	ChooseSubnet(az string) (string, error)
}

type manager struct {
	client.Client
	subnets map[string]map[string]cloud.Subnet
	cp      cloud.Provider
	logger  logr.Logger
}

func NewManager(logger logr.Logger, k8sClient client.Client, cp cloud.Provider) (Manager, error) {
	result := &manager{
		cp:      cp,
		logger:  logger,
		Client:  k8sClient,
		subnets: make(map[string]map[string]cloud.Subnet),
	}

	cm := &corev1.ConfigMap{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{}, cm); err != nil {
		return nil, errors.Wrap(err, "Failed to get subnet manager configmap")
	}

	for az, subnets := range cm.Data {
		for _, subnetID := range strings.Split(subnets, ",") {
			v, ok := result.subnets[az]
			if !ok {
				v = make(map[string]cloud.Subnet)
			}
			v[subnetID] = cloud.Subnet{ID: subnetID}
			result.subnets[az] = v
		}
	}

	return result, nil
}

func (m *manager) ChooseSubnet(az string) (string, error) {
	//TODO implement me
	panic("implement me")
}

var mgr Manager

func SetDefault(manager Manager) {
	mgr = manager
}

func ChooseSubnet(az string) (string, error) {
	return mgr.ChooseSubnet(az)
}
