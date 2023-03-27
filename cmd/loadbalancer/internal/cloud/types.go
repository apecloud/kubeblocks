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

package cloud

const (
	ProviderAWS             = "aws"
	TagENICreatedAt         = "kubeblocks.io/created-at"
	TagENINode              = "kubeblocks.io/instance-id"
	TagENIKubeBlocksManaged = "kubeblocks.io/managed"
)

type Provider interface {
	GetENILimit() int

	GetENIIPv4Limit() int

	GetInstanceInfo() *InstanceInfo

	CreateENI(instanceID, subnetID string, securityGroupIDs []string) (string, error)

	AttachENI(instanceID string, eniID string) (string, error)

	DeleteENI(eniID string) error

	FreeENI(eniID string) error

	DescribeAllENIs() (map[string]*ENIMetadata, error)

	FindLeakedENIs(instanceID string) ([]*ENIMetadata, error)

	AllocIPAddresses(eniID string) (string, error)

	DeallocIPAddresses(eniID string, ips []string) error

	AssignPrivateIPAddresses(eniID string, ip string) error

	ModifySourceDestCheck(eniID string, enabled bool) error
}

type InstanceInfo struct {
	InstanceID string `json:"instance_id"`

	SubnetID string `json:"subnet_id"`

	SecurityGroupIDs []string `json:"security_group_ids"`
}

type IPv4Address struct {
	Primary bool

	Address string
}

type ENIMetadata struct {
	// ID is the id of network interface
	ID string

	// MAC is the mac address of network interface
	MAC string

	// DeviceNumber is the  device number of network interface
	// 0 means it is primary interface
	DeviceNumber int

	// SubnetID is the subnet id of network interface
	SubnetID string

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
