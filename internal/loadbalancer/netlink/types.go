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
