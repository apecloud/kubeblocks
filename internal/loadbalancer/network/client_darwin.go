package network

import (
	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	iptableswrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/iptables"
	netlinkwrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/netlink"
	procfswrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/procfs"
)

func NewClient(logger logr.Logger, nl netlinkwrapper.NetLink, ipt iptableswrapper.IPTables, procfs procfswrapper.ProcFS) (*networkClient, error) {
	return &networkClient{ipt: ipt}, nil
}

func (n *networkClient) SetupNetworkForService(privateIP string, eni *cloud.ENIMetadata) error {
	return nil
}

func (n *networkClient) CleanNetworkForService(privateIP string, eni *cloud.ENIMetadata) error {
	return nil
}

func (n *networkClient) SetupNetworkForENI(eni *cloud.ENIMetadata) error {
	return nil
}

func (n *networkClient) CleanNetworkForENI(eni *cloud.ENIMetadata) error {
	return nil
}
