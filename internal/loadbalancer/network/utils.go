package network

import (
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
)

const (
	MainENIMark = 0x2000

	FromServiceRulePriority = 1000
	ConnMarkRulePriority    = 1000
)

func isRuleExistsError(err error) bool {
	if errno, ok := err.(syscall.Errno); ok {
		return errno == syscall.EEXIST
	}
	return false
}

func buildENIPolicyRoutingRule(eni *cloud.ENIMetadata) *netlink.Rule {
	var (
		mark = getENIConnMark(eni)
	)
	rule := netlink.NewRule()
	rule.Mark = mark
	rule.Mask = mark
	rule.Table = getENIRouteTable(eni)
	rule.Priority = ConnMarkRulePriority
	rule.Family = unix.AF_INET
	return rule
}

//lint:ignore U1000 we will use this function later
func buildServicePolicyRoutingRules(privateIP *net.IPNet, eni *cloud.ENIMetadata) *netlink.Rule {
	rule := netlink.NewRule()
	rule.Src = privateIP
	rule.Priority = FromServiceRulePriority
	rule.Table = getENIRouteTable(eni)
	return rule
}

func getENIRouteTable(eni *cloud.ENIMetadata) int {
	return eni.DeviceNumber + 10000
}

func getENIConnMark(eni *cloud.ENIMetadata) int {
	return MainENIMark + eni.DeviceNumber
}
