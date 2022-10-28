/*
Copyright ApeCloud Inc.

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

package cloud

const (
	ProviderAWS             = "aws"
	TagENICreatedAt         = "kubeblocks.apecloud.com/created-at"
	TagENINode              = "kubeblocks.apecloud.com/instance-id"
	TagENIKubeBlocksManaged = "kubeblocks.apecloud.com/managed"
)

type Provider interface {
	GetENILimit() int

	GetENIIPv4Limit() int

	GetInstanceInfo() *InstanceInfo

	CreateENI(instanceId, subnetId string, securityGroupIds []string) (string, error)

	AttachENI(instanceId string, eniId string) (string, error)

	DeleteENI(eniId string) error

	FreeENI(eniId string) error

	DescribeAllENIs() (map[string]*ENIMetadata, error)

	FindLeakedENIs(instanceId string) ([]*ENIMetadata, error)

	AllocIPAddresses(eniId string) (string, error)

	DeallocIPAddresses(eniId string, ips []string) error

	AssignPrivateIpAddresses(eniId string, ip string) error

	WaitForENIAttached(eniId string) (ENIMetadata, error)

	ModifySourceDestCheck(eniId string, enabled bool) error
}

type InstanceInfo struct {
	InstanceId string `json:"instance_id"`

	SubnetId string `json:"subnet_id"`

	SecurityGroupIds []string `json:"security_group_ids"`
}

type IPv4Address struct {
	Primary bool

	Address string
}

type ENIMetadata struct {
	// ENIId is the id of network interface
	ENIId string

	// MAC is the mac address of network interface
	MAC string

	// DeviceNumber is the  device number of network interface
	// 0 means it is primary interface
	DeviceNumber int

	// SubnetId is the subnet id of network interface
	SubnetId string

	// SubnetIPv4CIDR is the IPv4 CIDR of network interface
	SubnetIPv4CIDR string

	// The ip addresses allocated for the network interface
	IPv4Addresses []*IPv4Address

	// Tags is the tag set of network interface
	Tags map[string]string
}

func (eni ENIMetadata) PrimaryIPv4Address() string {
	for _, addr := range eni.IPv4Addresses {
		if addr.Primary {
			return addr.Address
		}
	}
	return ""
}
