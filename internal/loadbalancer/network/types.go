package network

import "github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"

type Client interface {
	SetupNetworkForService(privateIP string, eni *cloud.ENIMetadata) error

	CleanNetworkForService(privateIP string, eni *cloud.ENIMetadata) error

	SetupNetworkForENI(eni *cloud.ENIMetadata) error

	CleanNetworkForENI(eni *cloud.ENIMetadata) error
}
