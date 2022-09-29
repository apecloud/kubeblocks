package agent

import "github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"

type ENIManager interface {
	ChooseBusiestENI() (*cloud.ENIMetadata, error)

	GetManagedENIs() ([]*cloud.ENIMetadata, error)
}
