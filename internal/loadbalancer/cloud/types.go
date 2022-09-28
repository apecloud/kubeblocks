package cloud

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

const (
	ProviderAWS             = "aws"
	TagENICreatedAt         = "kubeblocks.apecloud.com/created-at"
	TagENINode              = "kubeblocks.apecloud.com/instance-id"
	TagENIKubeBlocksManaged = "kubeblocks.apecloud.com/managed"
)

type newFunc func(...interface{}) (Provider, error)

var (
	lock      sync.RWMutex
	providers = make(map[string]newFunc)
)

type Provider interface {
	GetENILimit() int

	GetENIIPv4Limit() int

	AllocENI() (string, error)

	FreeENI(id string) error

	DescribeAllENIs() (map[string]*ENIMetadata, error)

	AllocIPAddresses(id string) (*ec2.AssignPrivateIpAddressesOutput, error)

	DeallocIPAddresses(id string, ips []string) error

	AssignPrivateIpAddresses(id string, ip string) error

	WaitForENIAttached(id string) (ENIMetadata, error)

	ModifySourceDestCheck(id string, enabled bool) error
}

func NewProvider(name string, logger logr.Logger) (Provider, error) {
	lock.RLock()
	defer lock.RUnlock()
	f, ok := providers[name]
	if !ok {
		return nil, errors.New("Unknown cloud provider")
	}
	return f(logger)
}

func RegisterProvider(name string, f newFunc) {
	lock.Lock()
	defer lock.Unlock()
	if _, ok := providers[name]; ok {
		panic(fmt.Sprintf("Cloud provider %s exists", name))
	}
	providers[name] = f
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
