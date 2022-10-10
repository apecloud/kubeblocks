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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	pb "github.com/apecloud/kubeblocks/internal/loadbalancer/protocol"
)

type NodeManager interface {
	GetNode(ip string) (Node, error)

	ChooseSpareNode(subnet string) (Node, error)
}

type nodeManager struct {
	sync.RWMutex
	client.Client

	cp      cloud.Provider
	rpcPort int
	logger  logr.Logger
	nodes   map[string]Node
}

func NewNodeManager(logger logr.Logger, rpcPort int, cp cloud.Provider, client client.Client) (*nodeManager, error) {
	n := &nodeManager{
		Client:  client,
		cp:      cp,
		logger:  logger,
		rpcPort: rpcPort,
		nodes:   make(map[string]Node),
	}
	nodeList := &corev1.NodeList{}
	if err := n.Client.List(context.Background(), nodeList); err != nil {
		return nil, errors.Wrap(err, "Failed to list cluster nodes")
	}
	return n, n.initNodes(nodeList)
}

func (n *nodeManager) initNodes(nodeList *corev1.NodeList) error {
	for _, item := range nodeList.Items {
		var nodeIP string
		for _, addr := range item.Status.Addresses {
			if addr.Type != corev1.NodeInternalIP {
				continue
			}
			nodeIP = addr.Address
		}
		if nodeIP == "" {
			n.logger.Error(fmt.Errorf("invalid cluster node %v", item), "Skip init node")
			continue
		}
		node, err := n.initNode(nodeIP)
		if err != nil {
			return errors.Wrapf(err, "Failed to init node %s", nodeIP)
		}
		n.SetNode(nodeIP, node)
	}
	return nil
}

func (n *nodeManager) initNode(ip string) (*node, error) {
	addr := fmt.Sprintf("%s:%d", ip, n.rpcPort)
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to dial: %v", addr)
	}
	node, err := NewNode(n.logger, ip, pb.NewNodeClient(conn), n.cp)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to init node")
	}
	if err = node.Start(make(chan struct{})); err != nil {
		return nil, errors.Wrapf(err, "Failed to start node")
	}
	return node, nil
}

func (n *nodeManager) ChooseSpareNode(subnet string) (Node, error) {
	n.RLock()
	defer n.RUnlock()
	var spareNode Node
	for _, node := range n.nodes {
		if subnet != "" {
			_, ok := node.GetResource().SubnetIds[subnet]
			if !ok {
				continue
			}
		}
		if spareNode == nil {
			spareNode = node
			continue
		}
		if node.GetResource().GetSparePrivateIPs() < spareNode.GetResource().GetSparePrivateIPs() {
			continue
		}
		spareNode = node
	}
	if spareNode == nil {
		return nil, errors.New("Failed to find spare node")
	}
	return spareNode, nil
}

func (n *nodeManager) GetNode(ip string) (Node, error) {
	n.RLock()
	defer n.RUnlock()
	node, ok := n.nodes[ip]
	if !ok {
		return nil, fmt.Errorf("can not find node %s", ip)
	}
	return node, nil
}

func (n *nodeManager) RemoveNode(ip string) {
	n.Lock()
	defer n.Unlock()
	delete(n.nodes, ip)
}

func (n *nodeManager) SetNode(ip string, node Node) {
	n.Lock()
	defer n.Unlock()
	n.nodes[ip] = node
}
