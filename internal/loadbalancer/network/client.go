package network

import (
	"fmt"
	iptableswrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/iptables"
	netlinkwrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/netlink"
	procfswrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/procfs"
	"github.com/go-logr/logr"
)

const (
	LooseReversePathFilterValue = "2"

	ErrAddressNotExists = "cannot assign requested address"
)

type iptablesRule struct {
	name  string
	table string
	chain string
	rule  []string
}

func (r iptablesRule) String() string {
	return fmt.Sprintf("%s/%s rule %s rule %v", r.table, r.chain, r.name, r.rule)
}

type networkClient struct {
	logger logr.Logger
	nl     netlinkwrapper.NetLink
	ipt    iptableswrapper.IPTables
	procfs procfswrapper.ProcFS
}
