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
	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud"
	iptableswrapper "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/iptables"
	netlinkwrapper "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/netlink"
	procfswrapper "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/procfs"
)

func NewClient(logger logr.Logger, nl netlinkwrapper.NetLink, ipt iptableswrapper.IPTables, procfs procfswrapper.ProcFS) (*networkClient, error) {
	return &networkClient{logger: logger, ipt: ipt, procfs: procfs, nl: nl}, nil
}

func (c *networkClient) SetupNetworkForService(privateIP string, eni *cloud.ENIMetadata) error {
	return nil
}

func (c *networkClient) CleanNetworkForService(privateIP string, eni *cloud.ENIMetadata) error {
	return nil
}

func (c *networkClient) SetupNetworkForENI(eni *cloud.ENIMetadata) error {
	return nil
}

func (c *networkClient) CleanNetworkForENI(eni *cloud.ENIMetadata) error {
	return nil
}
