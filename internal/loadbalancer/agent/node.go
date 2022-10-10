package agent

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/internal/dbctl/util"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	pb "github.com/apecloud/kubeblocks/internal/loadbalancer/protocol"
)

type Node struct {
	*eniManager
	pb.NodeClient

	ip string
	cp cloud.Provider
}

func NewNode(logger logr.Logger, ip string, nc pb.NodeClient, cp cloud.Provider) (*Node, error) {
	em, err := newENIManager(logger, nc, cp)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to init eni manager")
	}
	return &Node{
		ip:         ip,
		cp:         cp,
		eniManager: em,
		NodeClient: nc,
	}, nil
}

func (n *Node) GetIP() string {
	return n.ip
}

func (n *Node) CleanNetworkForService(floatingIP string, eni *pb.ENIMetadata) error {
	request := &pb.CleanNetworkForServiceRequest{
		RequestId: util.GenRequestId(),
		PrivateIp: floatingIP,
		Eni:       eni,
	}
	_, err := n.NodeClient.CleanNetworkForService(context.Background(), request)
	return err
}

func (n *Node) SetupNetworkForService(floatingIP string, eni *pb.ENIMetadata) error {
	request := &pb.SetupNetworkForServiceRequest{
		RequestId: util.GenRequestId(),
		PrivateIp: floatingIP,
		Eni:       eni,
	}
	_, err := n.NodeClient.SetupNetworkForService(context.Background(), request)
	return err
}
