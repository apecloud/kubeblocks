//go:build linux

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
	"fmt"
	"net"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud"
	mocknetlink "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/netlink/mocks"
	mockprocfs "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/procfs/mocks"
)

func init() {
	viper.AutomaticEnv()
}

var (
	logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), func(o *zap.Options) {
		o.TimeEncoder = zapcore.ISO8601TimeEncoder
	})
)

var _ = Describe("Client", func() {

	const (
		loMac   = "00:00:00:00:00:01"
		eth1Mac = "00:00:00:00:00:02"
		svcVIP  = "172.31.1.10"
		eniID   = "eni-01"
		extraIP = "1.1.1.1"
		subnet  = "172.31.0.0/16"
	)

	setup := func() (*gomock.Controller, Client, *mocknetlink.MockNetLink, *memoryIptables, *mockprocfs.MockProcFS) {
		ctrl := gomock.NewController(GinkgoT())

		ipt := NewMemoryIptables()
		nl := mocknetlink.NewMockNetLink(ctrl)
		procfs := mockprocfs.NewMockProcFS(ctrl)
		client, err := NewClient(logger, nl, ipt, procfs)
		Expect(err == nil).Should(BeTrue())
		return ctrl, client, nl, ipt, procfs
	}

	assertIptablesNotExists := func(ipt *memoryIptables, deletedRules map[string]map[string][][]string) {
		for table, chains := range deletedRules {
			for chain, rules := range chains {
				for _, rule := range rules {
					Expect(ipt.Exists(table, chain, rule...)).Should(BeFalse())
				}
			}
		}
	}

	Context("Setup and clean service network", func() {
		It("Should success without error", func() {
			ctrl, networkClient, mockNetlink, mockIPtables, _ := setup()

			lo := mocknetlink.NewMockLink(ctrl)
			eth1 := mocknetlink.NewMockLink(ctrl)
			mockNetlink.EXPECT().LinkList().Return([]netlink.Link{lo, eth1}, nil).AnyTimes()

			loHwAddr, err := net.ParseMAC(loMac)
			Expect(err).Should(BeNil())
			loAttrs := &netlink.LinkAttrs{
				HardwareAddr: loHwAddr,
			}
			lo.EXPECT().Attrs().Return(loAttrs).AnyTimes()

			eth1HwAddr, err := net.ParseMAC(eth1Mac)
			Expect(err).Should(BeNil())
			eth1Attrs := &netlink.LinkAttrs{
				HardwareAddr: eth1HwAddr,
			}
			eth1.EXPECT().Attrs().Return(eth1Attrs).AnyTimes()

			privateIPNet := &net.IPNet{IP: net.ParseIP(svcVIP), Mask: net.IPv4Mask(255, 255, 255, 255)}
			mockNetlink.EXPECT().AddrAdd(eth1, &netlink.Addr{IPNet: privateIPNet}).Return(errors.New("file exists"))

			eni := &cloud.ENIMetadata{ID: eniID, MAC: eth1Mac, DeviceNumber: 1}
			Expect(networkClient.SetupNetworkForService(svcVIP, eni)).Should(Succeed())

			expectIptables := map[string]map[string][][]string{
				"mangle": {
					"PREROUTING": [][]string{
						{"-m", "conntrack", "--ctorigdst", svcVIP, "-m", "comment", "--comment", fmt.Sprintf("KubeBlocks, %s", eniID), "-j", "CONNMARK", "--restore-mark", "--mask", fmt.Sprintf("%#x", getENIConnMark(eni))},
					},
					"OUTPUT": [][]string{
						{"-m", "conntrack", "--ctorigdst", svcVIP, "-m", "comment", "--comment", fmt.Sprintf("KubeBlocks, %s", eniID), "-j", "CONNMARK", "--restore-mark", "--mask", fmt.Sprintf("%#x", getENIConnMark(eni))},
					},
				},
			}
			Expect(reflect.DeepEqual(expectIptables, mockIPtables.rules)).Should(BeTrue())

			mockNetlink.EXPECT().AddrDel(eth1, &netlink.Addr{IPNet: privateIPNet}).Return(nil)
			Expect(networkClient.CleanNetworkForService(svcVIP, eni)).Should(Succeed())

			assertIptablesNotExists(mockIPtables, expectIptables)
		})
	})

	Context("Setup and clean eni network", func() {
		It("Should success without error", func() {
			ctrl, networkClient, mockNetlink, mockIPtables, mockProcfs := setup()

			lo := mocknetlink.NewMockLink(ctrl)
			eth1 := mocknetlink.NewMockLink(ctrl)
			mockNetlink.EXPECT().LinkList().Return([]netlink.Link{lo, eth1}, nil).AnyTimes()
			mockNetlink.EXPECT().LinkSetUp(eth1).Return(nil).AnyTimes()

			loHwAddr, err := net.ParseMAC(loMac)
			Expect(err).Should(BeNil())
			loAttrs := &netlink.LinkAttrs{
				HardwareAddr: loHwAddr,
			}
			lo.EXPECT().Attrs().Return(loAttrs).AnyTimes()

			eth1HwAddr, err := net.ParseMAC(eth1Mac)
			Expect(err).Should(BeNil())
			eth1Attrs := &netlink.LinkAttrs{
				HardwareAddr: eth1HwAddr,
				Name:         "eth1",
			}
			eth1.EXPECT().Attrs().Return(eth1Attrs).AnyTimes()

			addrs := []netlink.Addr{
				{
					IPNet: &net.IPNet{
						IP: net.ParseIP(extraIP),
					},
				},
			}
			eni := &cloud.ENIMetadata{
				ID:             eniID,
				MAC:            eth1Mac,
				DeviceNumber:   1,
				SubnetIPv4CIDR: subnet,
			}
			procKey := "net/ipv4/conf/eth1/rp_filter"
			mockProcfs.EXPECT().Get(procKey).Return("1", nil)
			mockProcfs.EXPECT().Set(procKey, LooseReversePathFilterValue).Return(nil)
			mockNetlink.EXPECT().AddrList(eth1, gomock.Any()).Return(addrs, nil)
			mockNetlink.EXPECT().AddrDel(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockNetlink.EXPECT().RouteDel(gomock.Any()).Return(nil).AnyTimes()
			mockNetlink.EXPECT().RouteReplace(gomock.Any()).Return(nil).AnyTimes()
			mockNetlink.EXPECT().RuleAdd(gomock.Any()).Return(nil)
			mockNetlink.EXPECT().RuleList(gomock.Any()).Return([]netlink.Rule{}, nil).AnyTimes()
			mockNetlink.EXPECT().RouteList(gomock.Any(), gomock.Any()).Return([]netlink.Route{}, nil).AnyTimes()

			Expect(networkClient.SetupNetworkForENI(eni)).Should(Succeed())

			mark := getENIConnMark(eni)
			expectIptables := map[string]map[string][][]string{
				"mangle": {
					"PREROUTING": [][]string{
						{
							"-i", eth1.Attrs().Name, "-m", "comment", "--comment", fmt.Sprintf("KubeBlocks, %s", eni.ID),
							"-m", "addrtype", "--dst-type", "LOCAL", "--limit-iface-in", "-j", "CONNMARK", "--set-xmark", fmt.Sprintf("%#x/%#x", mark, mark),
						},
					},
				},
			}
			Expect(reflect.DeepEqual(expectIptables, mockIPtables.rules)).Should(BeTrue())

			mockNetlink.EXPECT().RuleDel(gomock.Any()).Return(nil).AnyTimes()
			Expect(networkClient.CleanNetworkForENI(eni)).Should(Succeed())
			assertIptablesNotExists(mockIPtables, expectIptables)
		})
	})
})

type memoryIptables struct {
	rules map[string]map[string][][]string
}

func (m *memoryIptables) Exists(table, chain string, ruleSpec ...string) (bool, error) {
	if _, ok := m.rules[table]; !ok {
		return false, nil
	}
	rules, ok := m.rules[table][chain]
	if !ok {
		return false, nil
	}
	for _, rule := range rules {
		if reflect.DeepEqual(ruleSpec, rule) {
			return true, nil
		}
	}
	return false, nil
}

func (m *memoryIptables) Insert(table, chain string, ruleSpec ...string) error {
	if _, ok := m.rules[table]; !ok {
		m.rules[table] = make(map[string][][]string)
	}
	m.rules[table][chain] = append([][]string{ruleSpec}, m.rules[table][chain]...)
	return nil
}

func (m *memoryIptables) Append(table, chain string, ruleSpec ...string) error {
	if _, ok := m.rules[table]; !ok {
		m.rules[table] = make(map[string][][]string)
	}
	m.rules[table][chain] = append(m.rules[table][chain], ruleSpec)
	return nil
}

func (m *memoryIptables) Delete(table, chain string, ruleSpec ...string) error {
	if _, ok := m.rules[table]; !ok {
		return errors.Errorf("Can not find table %s", table)
	}
	rules, ok := m.rules[table][chain]
	if !ok {
		return errors.Errorf("Can not find chain %s", chain)
	}
	idx := -1
	for i, rule := range rules {
		if reflect.DeepEqual(rule, ruleSpec) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return errors.Errorf("Can not find rule to delete")
	}
	m.rules[table][chain] = append(rules[:idx], rules[idx+1:]...)
	return nil
}

func NewMemoryIptables() *memoryIptables {
	return &memoryIptables{
		rules: make(map[string]map[string][][]string),
	}
}
