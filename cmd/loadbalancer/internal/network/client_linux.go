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

package network

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud"
	iptableswrapper "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/iptables"
	netlinkwrapper "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/netlink"
	procfswrapper "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/procfs"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type iptablesRule struct {
	name  string
	table string
	chain string
	rule  []string
}

func NewClient(logger logr.Logger, nl netlinkwrapper.NetLink, ipt iptableswrapper.IPTables, procfs procfswrapper.ProcFS) (*networkClient, error) {
	client := &networkClient{
		nl:     nl,
		ipt:    ipt,
		logger: logger,
		procfs: procfs,
	}
	return client, nil
}

func (c *networkClient) SetupNetworkForService(privateIP string, eni *cloud.ENIMetadata) error {
	ctxLog := c.logger.WithValues("eni id", eni.ID, "private ip", privateIP)
	ctxLog.Info("Configuring policy routing rules and routes")

	link, err := c.getLinkByMac(c.logger, eni.MAC)
	if err != nil {
		return errors.Wrapf(err, "Failed to get link by mac %s for private ip %s", eni.MAC, privateIP)
	}

	privateIPNet := &net.IPNet{IP: net.ParseIP(privateIP), Mask: net.IPv4Mask(255, 255, 255, 255)}
	if err := c.nl.AddrAdd(link, &netlink.Addr{IPNet: privateIPNet}); err != nil {
		if !strings.Contains(err.Error(), "file exists") {
			return errors.Wrapf(err, fmt.Sprintf("Failed to add private ip %s for link %s", privateIP, link.Attrs().Name))
		}
	}
	ctxLog.Info("Successfully add address to link")

	// add iptables rules
	iptablesRules := c.buildServiceIptablesRules(privateIP, eni)
	if err := c.updateIptablesRules(iptablesRules, false); err != nil {
		return err
	}
	ctxLog.Info("Successfully setup iptables for service")

	/*
		// add policy routing rule
		rule := c.buildServicePolicyRoutingRules(privateIPNet, eni)
		if err := c.nl.RuleAdd(rule); err != nil && !isRuleExistsError(err) {
			return errors.Wrapf(err, "Failed to add service rule, privateIP=%s, rtTable=%v", privateIP, rule.Table)
		}
		ctxLog.Info("Successfully setup from private ip rule", "route table", rule.Table)
	*/

	return nil
}

func (c *networkClient) CleanNetworkForService(privateIP string, eni *cloud.ENIMetadata) error {
	ctxLog := c.logger.WithValues("private ip", privateIP, "eni id", eni.ID)
	ctxLog.Info("Remove policy route rules and routes")

	link, err := c.getLinkByMac(c.logger, eni.MAC)
	if err != nil {
		return errors.Wrapf(err, "Failed to get link by mac %s for private ip %s", eni.MAC, privateIP)
	}

	privateIPNet := &net.IPNet{IP: net.ParseIP(privateIP), Mask: net.IPv4Mask(255, 255, 255, 255)}
	if err := c.nl.AddrDel(link, &netlink.Addr{IPNet: privateIPNet}); err != nil {
		if !strings.Contains(err.Error(), ErrAddressNotExists) {
			return errors.Wrapf(err, "Failed to remove addr for service")
		}
		ctxLog.Info("Address not exists, skip delete", "address", privateIPNet.String())
	}

	// add iptables rules
	iptablesRules := c.buildServiceIptablesRules(privateIP, eni)
	if err := c.updateIptablesRules(iptablesRules, true); err != nil {
		return err
	}
	ctxLog.Info("Successfully clean iptables for service")

	/*
		routingRules := c.buildServicePolicyRoutingRules(privateIPNet, eni)
		if err := c.nl.RuleDel(routingRules); err != nil {
			if strings.Contains(err.Error(), "no such file or directory") {
				c.logger.Info("Policy rule not exists, skip delete", "rule", routingRules.String())
				return nil
			} else {
				return errors.Wrapf(err, "Failed to remove service rule, privateIP=%s, rtTable=%v", privateIP, routingRules.Table)
			}
		}
		ctxLog.Info("Successfully remove routes", "route table", routingRules.Table)
	*/
	return nil
}

func (c *networkClient) SetupNetworkForENI(eni *cloud.ENIMetadata) error {
	ctxLog := c.logger.WithValues("eni id", eni.ID)

	if eni.DeviceNumber == 0 {
		return fmt.Errorf("can not setup primary eni %s", eni.ID)
	}

	if err := c.looseReversePathFilter(eni); err != nil {
		return errors.Wrapf(err, "Failed to loose reverse path filter for interface %s", eni.ID)
	}

	link, err := c.getLinkByMac(c.logger, eni.MAC)
	if err != nil {
		return errors.Wrap(err, "Failed to get link by mac")
	}

	if err := c.nl.LinkSetUp(link); err != nil {
		return errors.Wrap(err, "Failed to set link up")
	}

	_, subnetCIDR, err := net.ParseCIDR(eni.SubnetIPv4CIDR)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse subnet cidr")
	}

	gwIP, err := c.getNetworkGateway(subnetCIDR.IP)
	if err != nil {
		return errors.Wrap(err, "Failed to calculate gateway ip")
	}

	expectedIPMap := make(map[string]struct{}, len(eni.IPv4Addresses))
	for _, ip := range eni.IPv4Addresses {
		expectedIPMap[ip.Address] = struct{}{}
	}

	addrs, err := c.nl.AddrList(link, unix.AF_INET)
	if err != nil {
		return errors.Wrap(err, "Failed to list ip address for ENI")
	}

	// 1. remove unknown private ip, may be added by user
	assignedAddrs := make(map[string]struct{})
	for _, addr := range addrs {
		if _, ok := expectedIPMap[addr.IP.String()]; ok {
			assignedAddrs[addr.IP.String()] = struct{}{}
			continue
		}
		c.logger.Info("Deleting unknown ip address", "ip", addr.String())
		if err = c.nl.AddrDel(link, &addr); err != nil {
			if !strings.Contains(err.Error(), ErrAddressNotExists) {
				return errors.Wrapf(err, "Failed to delete ip %s from ENI", addr.IP.String())
			}
			ctxLog.Info("Address not exists, skip delete", "address", addr.IP.String())
		}
	}

	/*
		// 2. assign missing private ip
		for _, item := range eni.IPv4Addresses {
			ip := aws.StringValue(item.PrivateIpAddress)
			if _, ok := assignedAddrs[ip]; ok {
				continue
			}
			ipNet := &net.IPNet{IP: net.ParseIP(ip), Mask: subnetCIDR.Mask}
			if err = c.nl.AddrAdd(link, &netlink.Addr{IPNet: ipNet}); err != nil {
				return errors.Wrapf(err, "Failed to add private ip %s to link", ipNet.String())
			}
		}
	*/

	// 2. remove the route that default out to ENI out of main route table
	defaultRoute := netlink.Route{
		Dst:   subnetCIDR,
		Src:   net.ParseIP(eni.PrimaryIPv4Address()),
		Table: unix.RT_TABLE_MAIN,
		Scope: netlink.SCOPE_LINK,
	}

	if err := c.nl.RouteDel(&defaultRoute); err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno != syscall.ESRCH {
			return errors.Wrapf(err, "Unable to delete default route %s for source IP %s", subnetCIDR.String(), eni.PrimaryIPv4Address())
		}
	}
	ctxLog.Info("Successfully deleted default route for eni primary ip", "route", defaultRoute.String())

	// 3. add route table for eni, and configure routes
	var (
		linkIndex        = link.Attrs().Index
		routeTableNumber = getENIRouteTable(eni)
	)
	ctxLog.Info("Setting up eni default gateway", "gateway", gwIP, "route table", routeTableNumber)
	routes := []netlink.Route{
		// Add a direct link route for the host's ENI IP only
		{
			LinkIndex: linkIndex,
			Dst:       &net.IPNet{IP: gwIP, Mask: net.CIDRMask(32, 32)},
			Scope:     netlink.SCOPE_LINK,
			Table:     routeTableNumber,
		},
		// Route all other traffic via the host's ENI IP
		{
			LinkIndex: linkIndex,
			Dst:       &net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)},
			Scope:     netlink.SCOPE_UNIVERSE,
			Gw:        gwIP,
			Table:     routeTableNumber,
		},
	}
	for _, r := range routes {
		// RouteReplace must do two times for new created enis
		for i := 0; i < 2; i++ {
			err := util.DoWithRetry(context.Background(), c.logger, func() error {
				_ = c.nl.RouteReplace(&r)
				return c.nl.RouteReplace(&r)
			}, &util.RetryOptions{MaxRetry: 10, Delay: 1 * time.Second})

			if err != nil {
				return errors.Wrapf(err, "Failed to replace route: %s", r.String())
			}
		}
		ctxLog.Info("Successfully add route", "route", r.String())
	}

	rule := buildENIPolicyRoutingRule(eni)
	if err := c.nl.RuleAdd(rule); err != nil && !isRuleExistsError(err) {
		return errors.Wrap(err, "Failed to add connmark policy routing rule")
	}
	ctxLog.Info("Successfully add eni policy rule")

	iptablesRules := c.buildENIIptablesRules(link.Attrs().Name, eni)
	if err := c.updateIptablesRules(iptablesRules, false); err != nil {
		return err
	}
	ctxLog.Info("Successfully update iptables connmark rule", "count", len(iptablesRules))

	routingRules, _ := c.nl.RuleList(netlink.FAMILY_V4)
	ctxLog.Info("Found policy routing rules", "count", len(routingRules))
	for _, rule := range routingRules {
		ctxLog.Info("Found policy routing rule", "info", rule.String())
	}

	// TODO show routes from eni route table
	routes, _ = c.nl.RouteList(nil, netlink.FAMILY_V4)
	ctxLog.Info("Found routes", "count", len(routes))
	for _, route := range routes {
		ctxLog.Info("Found route", "info", route.String())
	}
	return nil
}

func (c *networkClient) CleanNetworkForENI(eni *cloud.ENIMetadata) error {
	if err := c.CleanNetworkForService(eni.PrimaryIPv4Address(), eni); err != nil {
		return errors.Wrap(err, "Failed to clean eni primary ip")
	}

	rule := buildENIPolicyRoutingRule(eni)
	if err := c.nl.RuleDel(rule); err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			c.logger.Info("Policy rule not exists, skip delete", "rule", rule.String())
		} else {
			return errors.Wrapf(err, "Failed to remove eni %s policy routing rule", eni.ID)
		}
	}

	link, err := c.getLinkByMac(c.logger, eni.MAC)
	if err != nil {
		return errors.Wrap(err, "Failed to get link by mac")
	}
	iptablesRules := c.buildENIIptablesRules(link.Attrs().Name, eni)
	if err := c.updateIptablesRules(iptablesRules, true); err != nil {
		return err
	}

	c.logger.Info("Successfully clean eni network", "eni id", eni.ID)
	return nil
}

func (c *networkClient) looseReversePathFilter(eni *cloud.ENIMetadata) error {
	var ifaceName string
	links, err := c.nl.LinkList()
	if err != nil {
		return errors.Wrap(err, "Failed to list interfaces")
	}
	for _, link := range links {
		if link.Attrs().HardwareAddr.String() == eni.MAC {
			ifaceName = link.Attrs().Name
			break
		}
	}
	if ifaceName == "" {
		return errors.Errorf("Failed to find local network interface with mac %s", eni.MAC)
	}

	procKey := fmt.Sprintf("net/ipv4/conf/%s/rp_filter", ifaceName)
	src, err := c.procfs.Get(procKey)
	if err != nil {
		return errors.Wrapf(err, "Failed to read sysctl config %s", procKey)
	}

	if err := c.procfs.Set(procKey, LooseReversePathFilterValue); err != nil {
		return errors.Wrapf(err, "Failed to update sysctl config %s", procKey)
	}

	c.logger.Info("Successfully loose network interface reverse path filter",
		"from", src, "to", LooseReversePathFilterValue, "eni id", eni.ID)
	return nil
}

func (c *networkClient) buildServiceIptablesRules(privateIP string, eni *cloud.ENIMetadata) []iptablesRule {
	var (
		rules []iptablesRule
		mark  = getENIConnMark(eni)
	)

	// handle nat-ed traffic which reply packet comes from other host, restore connmark at PREROUTING chain
	rules = append(rules, iptablesRule{
		name:  "connmark to fwmark copy",
		table: "mangle",
		chain: "PREROUTING",
		rule: []string{
			"-m", "conntrack", "--ctorigdst", privateIP,
			"-m", "comment", "--comment", fmt.Sprintf("KubeBlocks, %s", eni.ID),
			"-j", "CONNMARK", "--restore-mark", "--mask", fmt.Sprintf("%#x", mark),
		},
	})

	// handle normal traffic which reply packet comes from local process, restore connmark at OUTPUT chain
	rules = append(rules, iptablesRule{
		name:  "connmark to fwmark copy",
		table: "mangle",
		chain: "OUTPUT",
		rule: []string{
			"-m", "conntrack", "--ctorigdst", privateIP,
			"-m", "comment", "--comment", fmt.Sprintf("KubeBlocks, %s", eni.ID),
			"-j", "CONNMARK", "--restore-mark", "--mask", fmt.Sprintf("%#x", mark),
		},
	})
	return rules
}

func (c *networkClient) updateIptablesRules(iptableRules []iptablesRule, delete bool) error {
	for _, rule := range iptableRules {
		c.logger.Info("Execute iptable rule", "rule", rule.name)

		exists, err := c.ipt.Exists(rule.table, rule.chain, rule.rule...)
		if err != nil {
			c.logger.Error(err, "Failed to check existence of iptables rule", "info", rule)
			return errors.Wrapf(err, "Failed to check existence of %v", rule)
		}

		if !exists && !delete {
			err = c.ipt.Append(rule.table, rule.chain, rule.rule...)
			if err != nil {
				c.logger.Error(err, "Failed to add iptables rule", "info", rule)
				return errors.Wrapf(err, "Failed to add %v", rule)
			}
		} else if exists && delete {
			err = c.ipt.Delete(rule.table, rule.chain, rule.rule...)
			if err != nil {
				c.logger.Error(err, "Failed to delete iptables rule", "info", rule)
				return errors.Wrapf(err, "Failed to delete %v", rule)
			}
		}
	}
	return nil
}

func (c *networkClient) buildENIIptablesRules(iface string, eni *cloud.ENIMetadata) []iptablesRule {
	var (
		mark = getENIConnMark(eni)
	)

	var rules []iptablesRule
	rules = append(rules, iptablesRule{
		name:  "connmark rule for non-VPC outbound traffic",
		table: "mangle",
		chain: "PREROUTING",
		rule: []string{
			"-i", iface, "-m", "comment", "--comment", fmt.Sprintf("KubeBlocks, %s", eni.ID),
			"-m", "addrtype", "--dst-type", "LOCAL", "--limit-iface-in", "-j", "CONNMARK", "--set-xmark", fmt.Sprintf("%#x/%#x", mark, mark),
		}})

	/*
		rules = append(rules, iptablesRule{
			name:  "connmark to fwmark copy",
			table: "mangle",
			chain: "PREROUTING",
			rule: []string{
				"-i", "eni+", "-m", "comment", "--comment", fmt.Sprintf("KubeBlocks, %s", eni.ID),
				"-j", "CONNMARK", "--restore-mark", "--mask", fmt.Sprintf("%#x", mark),
			},
		})
	*/

	return rules
}

// The first four IP addresses and the last IP address in each subnet CIDR block are not available for your use, and they cannot be assigned to a resource, such as an EC2 instance.
// reference: https://docs.aws.amazon.com/vpc/latest/userguide/configure-subnets.html#subnet-sizing
func (c *networkClient) getNetworkGateway(ip net.IP) (net.IP, error) {
	ip4 := ip.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("%q is not a valid IPv4 Address", ip)
	}
	intIP := binary.BigEndian.Uint32(ip4)
	if intIP == (1<<32 - 1) {
		return nil, fmt.Errorf("%q will be overflowed", ip)
	}
	intIP++
	nextIPv4 := make(net.IP, 4)
	binary.BigEndian.PutUint32(nextIPv4, intIP)
	return nextIPv4, nil
}

func (c *networkClient) getLinkByMac(logger logr.Logger, mac string) (netlink.Link, error) {
	var result netlink.Link
	f := func() error {
		links, err := c.nl.LinkList()
		if err != nil {
			return err
		}

		for _, link := range links {
			if mac == link.Attrs().HardwareAddr.String() {
				logger.Info("Found ethernet link", "mac", mac, "device index", link.Attrs().Index)
				result = link
				return nil
			}
		}
		return errors.Errorf("Failed to find network interface with mac address %s", mac)
	}

	// The adapter might not be immediately available, so we perform retries
	retryOpts := &util.RetryOptions{MaxRetry: 10, Delay: 3 * time.Second}
	if err := util.DoWithRetry(context.Background(), logger, f, retryOpts); err != nil {
		return nil, err
	}
	return result, nil
}
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

func getENIRouteTable(eni *cloud.ENIMetadata) int {
	return eni.DeviceNumber + 10000
}

func getENIConnMark(eni *cloud.ENIMetadata) int {
	return MainENIMark + eni.DeviceNumber
}
