package agent

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/internal/dbctl/util"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/config"
	pb "github.com/apecloud/kubeblocks/internal/loadbalancer/protocol"
)

type Node interface {
	GetIP() string

	GetResource() *NodeResource

	ChooseENI() (*pb.ENIMetadata, error)

	GetManagedENIs() ([]*pb.ENIMetadata, error)

	SetupNetworkForService(floatingIP string, eni *pb.ENIMetadata) error

	CleanNetworkForService(floatingIP string, eni *pb.ENIMetadata) error
}

type node struct {
	*eniManager

	ip string
	nc pb.NodeClient
	cp cloud.Provider
}

func NewNode(logger logr.Logger, ip string, nc pb.NodeClient, cp cloud.Provider) (*node, error) {
	em, err := newENIManager(logger, nc, cp)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to init eni manager")
	}

	return &node{
		nc:         nc,
		ip:         ip,
		cp:         cp,
		eniManager: em,
	}, nil
}

func (n *node) Start(stop chan struct{}) error {
	return n.eniManager.start(stop, config.ENIReconcileInterval, config.CleanLeakedENIInterval)
}

func (n *node) GetIP() string {
	return n.ip
}

func (n *node) GetResource() *NodeResource {
	// TODO deepcopy
	return n.eniManager.resource
}

func (n *node) ChooseENI() (*pb.ENIMetadata, error) {
	managedENIs, err := n.GetManagedENIs()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get managed ENIs")
	}
	if len(managedENIs) == 0 {
		return nil, errors.New("No managed eni found")
	}
	candidate := managedENIs[0]
	for _, eni := range managedENIs {
		if len(eni.Ipv4Addresses) > len(candidate.Ipv4Addresses) && len(eni.Ipv4Addresses) < n.maxIPsPerENI {
			candidate = eni
		}
	}
	n.logger.Info("Found busiest eni", "eni id", candidate.EniId)
	return candidate, nil
}

func (n *node) SetupNetworkForService(floatingIP string, eni *pb.ENIMetadata) error {
	request := &pb.SetupNetworkForServiceRequest{
		RequestId: util.GenRequestId(),
		PrivateIp: floatingIP,
		Eni:       eni,
	}
	_, err := n.nc.SetupNetworkForService(context.Background(), request)
	return err
}
func (n *node) CleanNetworkForService(floatingIP string, eni *pb.ENIMetadata) error {
	request := &pb.CleanNetworkForServiceRequest{
		RequestId: util.GenRequestId(),
		PrivateIp: floatingIP,
		Eni:       eni,
	}
	_, err := n.nc.CleanNetworkForService(context.Background(), request)
	return err
}
