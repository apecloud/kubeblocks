package cloud

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const (
	ProviderAWS             = "aws"
	TagENICreatedAt         = "kubeblocks.apecloud.com/created-at"
	TagENINode              = "kubeblocks.apecloud.com/instance-id"
	TagENIKubeBlocksManaged = "kubeblocks.apecloud.com/managed"
)

type Provider interface {
	GetENILimit() int

	GetENIIPv4Limit() int

	AllocENI() (string, error)

	DeleteENI(id string) error

	FreeENI(id string) error

	DescribeAllENIs() (map[string]*ENIMetadata, error)

	FindLeakedENIs() ([]*ENIMetadata, error)

	AllocIPAddresses(id string) (string, error)

	DeallocIPAddresses(id string, ips []string) error

	AssignPrivateIpAddresses(id string, ip string) error

	WaitForENIAttached(id string) (ENIMetadata, error)

	ModifySourceDestCheck(id string, enabled bool) error
}

type ENIMetadata struct {
	// ENIId is the id of network interface
	ENIId string

	// MAC is the mac address of network interface
	MAC string

	// DeviceNumber is the  device number of network interface
	DeviceNumber int // 0 means it is primary interface

	// SubnetIPv4CIDR is the IPv4 CIDR of network interface
	SubnetIPv4CIDR string

	// TODO refactor fields, make them cloud neutral
	// The ip addresses allocated for the network interface
	IPv4Addresses []*ec2.NetworkInterfacePrivateIpAddress

	// Tags is the tag set of network interface
	Tags map[string]string
}

func (eni ENIMetadata) PrimaryIPv4Address() string {
	for _, addr := range eni.IPv4Addresses {
		if aws.BoolValue(addr.Primary) {
			return aws.StringValue(addr.PrivateIpAddress)
		}
	}
	return ""
}
