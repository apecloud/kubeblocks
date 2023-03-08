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

package agent

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud"
	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/config"
	pb "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/protocol"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type Node interface {
	Start() error

	Stop()

	GetIP() string

	GetResource() *NodeResource

	ChooseENI() (*pb.ENIMetadata, error)

	GetManagedENIs() ([]*pb.ENIMetadata, error)

	GetNodeInfo() *pb.InstanceInfo

	SetupNetworkForService(floatingIP string, eni *pb.ENIMetadata) error

	CleanNetworkForService(floatingIP string, eni *pb.ENIMetadata) error
}

type node struct {
	ip     string
	nc     pb.NodeClient
	cp     cloud.Provider
	em     *eniManager
	once   sync.Once
	info   *pb.InstanceInfo
	stop   chan struct{}
	logger logr.Logger
}

func NewNode(logger logr.Logger, ip string, nc pb.NodeClient, cp cloud.Provider) (*node, error) {
	result := &node{
		nc:     nc,
		ip:     ip,
		cp:     cp,
		once:   sync.Once{},
		stop:   make(chan struct{}),
		logger: logger.WithValues("ip", ip),
	}

	resp, err := nc.DescribeNodeInfo(context.Background(), &pb.DescribeNodeInfoRequest{RequestId: util.GenRequestID()})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to describe node info")
	}
	result.info = resp.GetInfo()

	em, err := newENIManager(logger, ip, result.info, nc, cp)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to init eni manager")
	}
	result.em = em

	return result, nil
}

func (n *node) Start() error {
	return n.em.start(n.stop, config.ENIReconcileInterval, config.CleanLeakedENIInterval)
}

func (n *node) Stop() {
	n.once.Do(func() {
		if n.stop != nil {
			close(n.stop)
		}
	})
}

func (n *node) GetIP() string {
	return n.ip
}

func (n *node) GetNodeInfo() *pb.InstanceInfo {
	return n.info
}

func (n *node) GetResource() *NodeResource {
	// TODO deepcopy
	return n.em.resource
}

func (n *node) GetManagedENIs() ([]*pb.ENIMetadata, error) {
	return n.em.getManagedENIs()
}

func (n *node) ChooseENI() (*pb.ENIMetadata, error) {
	managedENIs, err := n.em.getManagedENIs()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get managed ENIs")
	}
	if len(managedENIs) == 0 {
		return nil, errors.New("No managed eni found")
	}
	candidate := managedENIs[0]
	for _, eni := range managedENIs {
		if len(eni.Ipv4Addresses) > len(candidate.Ipv4Addresses) && len(eni.Ipv4Addresses) < n.em.maxIPsPerENI {
			candidate = eni
		}
	}
	n.logger.Info("Found busiest eni", "eni id", candidate.EniId)
	return candidate, nil
}

func (n *node) SetupNetworkForService(floatingIP string, eni *pb.ENIMetadata) error {
	request := &pb.SetupNetworkForServiceRequest{
		RequestId: util.GenRequestID(),
		PrivateIp: floatingIP,
		Eni:       eni,
	}
	_, err := n.nc.SetupNetworkForService(context.Background(), request)
	return err
}

func (n *node) CleanNetworkForService(floatingIP string, eni *pb.ENIMetadata) error {
	request := &pb.CleanNetworkForServiceRequest{
		RequestId: util.GenRequestID(),
		PrivateIp: floatingIP,
		Eni:       eni,
	}
	_, err := n.nc.CleanNetworkForService(context.Background(), request)
	return err
}
