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
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud"
	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/config"
	pb "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/protocol"
)

var (
	ErrNodeNotFound = errors.New("Node not found")

	newGRPCConn = func(addr string) (*grpc.ClientConn, error) {
		return grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	newNode = func(logger logr.Logger, ip string, nc pb.NodeClient, cp cloud.Provider) (Node, error) {
		return NewNode(logger, ip, nc, cp)
	}
)

type NodeManager interface {
	Start(ctx context.Context) error

	GetNode(ip string) (Node, error)

	GetNodes() ([]Node, error)

	ChooseSpareNode(subnet string) (Node, error)
}

type nodeManager struct {
	sync.RWMutex
	client.Client

	rpcPort int
	cp      cloud.Provider
	logger  logr.Logger
	nodes   map[string]Node
}

func NewNodeManager(logger logr.Logger, rpcPort int, cp cloud.Provider, c client.Client) (*nodeManager, error) {
	nm := &nodeManager{
		Client:  c,
		cp:      cp,
		logger:  logger.WithName("NodeManager"),
		rpcPort: rpcPort,
		nodes:   make(map[string]Node),
	}
	nm.logger.Info("Monitoring nodes", "labels", fmt.Sprintf("%v", config.TrafficNodeLabels))
	return nm, nil
}

func (nm *nodeManager) Start(ctx context.Context) error {
	if err := nm.refreshNodes(); err != nil {
		return errors.Wrapf(err, "Failed to refresh nodes")
	}

	f := func() {
		if err := nm.refreshNodes(); err != nil {
			nm.logger.Error(err, "Failed to refresh nodes")
		} else {
			nm.logger.Info("Successfully refresh nodes")
		}
	}
	go wait.Until(f, config.RefreshNodeInterval, ctx.Done())
	return nil
}

func (nm *nodeManager) refreshNodes() error {
	nodeList := &corev1.NodeList{}
	opts := []client.ListOption{client.MatchingLabels(config.TrafficNodeLabels)}
	if err := nm.Client.List(context.Background(), nodeList, opts...); err != nil {
		return errors.Wrap(err, "Failed to list cluster nodes")
	}
	nodesLatest := make(map[string]struct{})
	for _, item := range nodeList.Items {
		var nodeIP string
		for _, addr := range item.Status.Addresses {
			if addr.Type != corev1.NodeInternalIP {
				continue
			}
			nodeIP = addr.Address
		}
		if nodeIP == "" {
			nm.logger.Error(fmt.Errorf("invalid cluster node %v", item), "Skip init node")
			continue
		}
		nodesLatest[nodeIP] = struct{}{}

		cachedNode, err := nm.GetNode(nodeIP)
		if err == nil && cachedNode != nil {
			continue
		}
		if err != ErrNodeNotFound {
			nm.logger.Error(err, "Failed to find node", "ip", nodeIP)
			continue
		}
		cachedNode, err = nm.initNode(nodeIP)
		if err != nil {
			return errors.Wrapf(err, "Failed to init node %s", nodeIP)
		}
		nm.SetNode(nodeIP, cachedNode)
	}

	nodesCached, err := nm.GetNodes()
	if err != nil {
		return errors.Wrapf(err, "Failed to get cached nodes")
	}
	for index := range nodesCached {
		cachedNode := nodesCached[index]
		if _, ok := nodesLatest[cachedNode.GetIP()]; ok {
			continue
		}
		cachedNode.Stop()
		nm.RemoveNode(cachedNode.GetIP())
		nm.logger.Info("Successfully removed node", "ip", cachedNode.GetIP())
	}
	return nil
}

func (nm *nodeManager) initNode(ip string) (Node, error) {
	addr := fmt.Sprintf("%s:%d", ip, nm.rpcPort)
	conn, err := newGRPCConn(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to dial: %v", addr)
	}
	n, err := newNode(nm.logger, ip, pb.NewNodeClient(conn), nm.cp)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to init node")
	}
	if err = n.Start(); err != nil {
		return nil, errors.Wrapf(err, "Failed to start node")
	}
	return n, nil
}

func (nm *nodeManager) ChooseSpareNode(subnet string) (Node, error) {
	nm.RLock()
	defer nm.RUnlock()
	var spareNode Node
	for _, item := range nm.nodes {
		if subnet != "" {
			_, ok := item.GetResource().SubnetIds[subnet]
			if !ok {
				continue
			}
		}
		if spareNode == nil {
			spareNode = item
			continue
		}
		if item.GetResource().GetSparePrivateIPs() < spareNode.GetResource().GetSparePrivateIPs() {
			continue
		}
		spareNode = item
	}
	if spareNode == nil {
		return nil, errors.New("Failed to find spare node")
	}
	return spareNode, nil
}

func (nm *nodeManager) GetNodes() ([]Node, error) {
	nm.RLock()
	defer nm.RUnlock()
	var result []Node
	for _, item := range nm.nodes {
		result = append(result, item)
	}
	return result, nil
}

func (nm *nodeManager) GetNode(ip string) (Node, error) {
	nm.RLock()
	defer nm.RUnlock()
	result, ok := nm.nodes[ip]
	if !ok {
		return nil, ErrNodeNotFound
	}
	return result, nil
}

func (nm *nodeManager) RemoveNode(ip string) {
	nm.Lock()
	defer nm.Unlock()
	delete(nm.nodes, ip)
}

func (nm *nodeManager) SetNode(ip string, node Node) {
	nm.Lock()
	defer nm.Unlock()
	nm.nodes[ip] = node
}
