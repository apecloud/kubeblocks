package netlink

import "github.com/vishvananda/netlink"

type NetLink interface {
	NewRule() *netlink.Rule

	RuleAdd(rule *netlink.Rule) error

	RuleDel(rule *netlink.Rule) error

	RuleList(family int) ([]netlink.Rule, error)

	LinkSetMTU(link netlink.Link, mtu int) error

	AddrAdd(link netlink.Link, addr *netlink.Addr) error

	AddrDel(link netlink.Link, addr *netlink.Addr) error

	AddrList(link netlink.Link, family int) ([]netlink.Addr, error)

	LinkSetUp(link netlink.Link) error

	LinkList() ([]netlink.Link, error)

	RouteReplace(route *netlink.Route) error

	RouteDel(route *netlink.Route) error

	RouteList(link netlink.Link, family int) ([]netlink.Route, error)
}
