//go:build linux

package netlink

import "github.com/vishvananda/netlink"

type netLink struct {
	*netlink.Handle
}

func NewNetLink() NetLink {
	nl, _ := netlink.NewHandle()
	return &netLink{nl}
}

func (n *netLink) NewRule() *netlink.Rule {
	return netlink.NewRule()
}
