//go:build linux

package netlink

import "github.com/vishvananda/netlink"

type netLink struct {
}

func NewNetLink() NetLink {
	return &netLink{}
}

func (n *netLink) NewRule() *netlink.Rule {
	return netlink.NewRule()
}

func (n *netLink) RuleAdd(rule *netlink.Rule) error {
	return netlink.RuleAdd(rule)
}

func (n *netLink) RuleDel(rule *netlink.Rule) error {
	return netlink.RuleDel(rule)
}

func (n *netLink) RuleList(family int) ([]netlink.Rule, error) {
	return netlink.RuleList(family)
}

func (n *netLink) LinkSetMTU(link netlink.Link, mtu int) error {
	return netlink.LinkSetMTU(link, mtu)
}

func (n *netLink) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	return netlink.AddrAdd(link, addr)
}

func (n *netLink) AddrDel(link netlink.Link, addr *netlink.Addr) error {
	return netlink.AddrDel(link, addr)
}

func (n *netLink) AddrList(link netlink.Link, family int) ([]netlink.Addr, error) {
	return netlink.AddrList(link, family)
}

func (n *netLink) LinkSetUp(link netlink.Link) error {
	return netlink.LinkSetUp(link)
}

func (n *netLink) LinkList() ([]netlink.Link, error) {
	return netlink.LinkList()
}

func (n *netLink) RouteReplace(route *netlink.Route) error {
	return netlink.RouteReplace(route)
}

func (n *netLink) RouteDel(route *netlink.Route) error {
	return netlink.RouteDel(route)
}

func (n *netLink) RouteList(link netlink.Link, family int) ([]netlink.Route, error) {
	return netlink.RouteList(link, family)
}
