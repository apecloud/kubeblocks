package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/apecloud/kubeblocks/internal/dbctl/util"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/config"
	pb "github.com/apecloud/kubeblocks/internal/loadbalancer/protocol"
)

type NodeResource struct {
	TotalPrivateIPs int
	UsedPrivateIPs  int
	SubnetIds       map[string]map[string]*pb.ENIMetadata
	ENIResources    map[string]*ENIResource
}

func (h *NodeResource) GetSparePrivateIPs() int {
	return h.TotalPrivateIPs - h.UsedPrivateIPs
}

type ENIResource struct {
	ENIId           string
	SubnetId        string
	TotalPrivateIPs int
	UsedPrivateIPs  int
}

type eniManager struct {
	pb.NodeClient

	logger       logr.Logger
	maxIPsPerENI int
	maxENI       int
	minPrivateIP int
	resource     *NodeResource
	cp           cloud.Provider
}

func newENIManager(logger logr.Logger, nc pb.NodeClient, cp cloud.Provider) (*eniManager, error) {
	_ = viper.ReadInConfig()

	c := &eniManager{
		NodeClient: nc,
		cp:         cp,
		logger:     logger,
	}

	c.minPrivateIP = config.MinPrivateIP
	c.maxIPsPerENI = cp.GetENIIPv4Limit()

	c.maxENI = cp.GetENILimit()
	if config.MaxENI > 0 && config.MaxENI < c.maxENI {
		c.maxENI = config.MaxENI
	}

	return c, c.init()
}

func (c *eniManager) init() error {
	managedENIs, err := c.GetManagedENIs()
	if err != nil {
		return errors.Wrap(err, "ipamd init: failed to retrieve attached ENIs info")
	}
	hostResource := c.buildHostResource(managedENIs)
	c.updateHostResource(hostResource)

	for i := range managedENIs {
		eni := managedENIs[i]
		c.logger.Info("Discovered managed ENI, trying to set it up", "eni id", eni.EniId)

		options := &util.RetryOptions{MaxRetry: 10, Delay: 1 * time.Second}
		if err = util.DoWithRetry(context.Background(), c.logger, func() error {
			setupENIRequest := &pb.SetupNetworkForENIRequest{
				RequestId: util.GenRequestId(),
				Eni:       eni,
			}
			_, err = c.SetupNetworkForENI(context.Background(), setupENIRequest)
			return err
		}, options); err != nil {
			c.logger.Error(err, "Failed to setup ENI", "eni id", eni.EniId)
		} else {
			c.logger.Info("ENI set up completed", "eni id", eni.EniId)
		}
	}
	c.logger.Info("Successfully init node")

	return nil
}

func (c *eniManager) updateHostResource(resource *NodeResource) {
	c.resource = resource
}

func (c *eniManager) buildHostResource(enis []*pb.ENIMetadata) *NodeResource {
	result := &NodeResource{
		SubnetIds:    make(map[string]map[string]*pb.ENIMetadata),
		ENIResources: make(map[string]*ENIResource),
	}
	for index := range enis {
		eni := enis[index]
		result.TotalPrivateIPs += c.maxIPsPerENI
		result.UsedPrivateIPs += len(eni.Ipv4Addresses)
		result.ENIResources[eni.EniId] = &ENIResource{
			ENIId:           eni.EniId,
			SubnetId:        eni.SubnetId,
			TotalPrivateIPs: c.maxIPsPerENI,
			UsedPrivateIPs:  len(eni.Ipv4Addresses),
		}
		subnetEnis, ok := result.SubnetIds[eni.SubnetId]
		if !ok {
			subnetEnis = make(map[string]*pb.ENIMetadata)
		}
		subnetEnis[eni.EniId] = eni
		result.SubnetIds[eni.SubnetId] = subnetEnis
	}
	return result
}

func (c *eniManager) start(stop chan struct{}, reconcileInterval time.Duration, cleanLeakedInterval time.Duration) error {
	if err := c.modifyPrimaryENISourceDestCheck(true); err != nil {
		return errors.Wrap(err, "Failed to modify primary eni source/dest check")
	}

	f1 := func() {
		if err := c.ensureENI(); err != nil {
			c.logger.Error(err, "Failed to ensure eni")
		}
	}
	go wait.Until(f1, reconcileInterval, stop)

	f2 := func() {
		if err := c.cleanLeakedENIs(); err != nil {
			c.logger.Error(err, "Failed to clean leaked enis")
		}
	}
	go wait.Until(f2, cleanLeakedInterval, stop)

	return nil
}

func (c *eniManager) modifyPrimaryENISourceDestCheck(enabled bool) error {
	describeENIRequest := &pb.DescribeAllENIsRequest{RequestId: util.GenRequestId()}
	describeENIResponse, err := c.DescribeAllENIs(context.Background(), describeENIRequest)
	if err != nil {
		return errors.Wrap(err, "Failed to get all enis, retry later")
	}

	var primaryENI *pb.ENIMetadata
	for _, eni := range describeENIResponse.Enis {
		if eni.DeviceNumber == 0 {
			primaryENI = eni
			break
		}
	}
	if primaryENI == nil {
		return errors.Wrap(err, "Failed to find primary eni")
	}
	// TODO enable check when we can ensure traffic comes in and out from same interface
	if err := c.cp.ModifySourceDestCheck(primaryENI.GetEniId(), enabled); err != nil {
		return errors.Wrap(err, "Failed to disable primary eni source/destination check")
	}
	c.logger.Info("Successfully disable primary eni source/destination check")
	return nil
}

func (c *eniManager) GetManagedENIs() ([]*pb.ENIMetadata, error) {
	describeENIRequest := &pb.DescribeAllENIsRequest{RequestId: util.GenRequestId()}
	describeENIResponse, err := c.DescribeAllENIs(context.Background(), describeENIRequest)
	if err != nil {
		return nil, errors.Wrap(err, "ipamd init: failed to retrieve attached ENIs info")
	}

	return c.filterManagedENIs(describeENIResponse.GetEnis()), nil
}

func (c *eniManager) filterManagedENIs(enis map[string]*pb.ENIMetadata) []*pb.ENIMetadata {
	var (
		ids            []string
		managedENIList []*pb.ENIMetadata
	)
	for eniId, eni := range enis {
		if _, found := eni.Tags[cloud.TagENIKubeBlocksManaged]; !found {
			continue
		}

		ids = append(ids, eniId)
		managedENIList = append(managedENIList, enis[eniId])
	}
	c.logger.Info("Managed eni", "count", len(managedENIList), "ids", strings.Join(ids, ","))
	return managedENIList
}

func (c *eniManager) ensureENI() error {
	describeENIRequest := &pb.DescribeAllENIsRequest{RequestId: util.GenRequestId()}
	describeENIResponse, err := c.DescribeAllENIs(context.Background(), describeENIRequest)
	if err != nil {
		return errors.Wrap(err, "Failed to get all enis, retry later")
	}

	var (
		min          = c.minPrivateIP
		max          = c.minPrivateIP + c.maxIPsPerENI
		managedENIs  = c.filterManagedENIs(describeENIResponse.GetEnis())
		hostResource = c.buildHostResource(managedENIs)
		totalSpare   = hostResource.TotalPrivateIPs - hostResource.UsedPrivateIPs
	)

	c.updateHostResource(hostResource)

	c.logger.Info("Local private ip buffer status",
		"spare private ip", totalSpare, "min spare private ip", min, "max spare private ip", max)

	b, _ := json.Marshal(hostResource)
	c.logger.Info("Local private ip buffer status", "info", string(b))

	if totalSpare < min {
		if len(describeENIResponse.Enis) >= c.maxENI {
			c.logger.Info("Limit exceed, can not alloc new eni", "current", len(describeENIResponse.Enis), "max", c.maxENI)
			return nil
		}
		if err := c.tryAllocAndAttachENI(); err != nil {
			c.logger.Error(err, "Failed to alloc and attach new ENI")
		}
	} else if totalSpare > max {
		if err := c.tryDetachAndDeleteENI(managedENIs); err != nil {
			c.logger.Error(err, "Failed to detach and delete idle ENI")
		}
	}
	return nil
}

func (c *eniManager) tryAllocAndAttachENI() error {
	c.logger.Info("Try to alloc and attach new eni")

	// alloc ENI, use same sg and subnet as primary ENI
	eniId, err := c.cp.AllocENI()
	if err != nil {
		return errors.Wrap(err, "Failed to alloc ENI, retry later")
	}
	c.logger.Info("Successfully create new eni, waiting for attached", "eni id", eniId)

	// waiting for ENI attached
	eni, err := c.cp.WaitForENIAttached(eniId)
	if err != nil {
		return errors.Wrap(err, "Unable to discover attached ENI from metadata service")
	}
	c.logger.Info("New eni attached", "eni id", eniId)

	// setup ENI networking stack
	setupENIRequest := &pb.SetupNetworkForENIRequest{
		RequestId: util.GenRequestId(),
		Eni: &pb.ENIMetadata{
			EniId: eni.ENIId,
		},
	}
	if _, err = c.SetupNetworkForENI(context.Background(), setupENIRequest); err != nil {
		return errors.Wrapf(err, "Failed to set up network for eni %s", eni.ENIId)
	}
	c.logger.Info("Successfully initialized new eni", "eni id", eniId)
	return nil
}

func (c *eniManager) tryDetachAndDeleteENI(enis []*pb.ENIMetadata) error {
	c.logger.Info("Try to detach and delete idle eni")

	for _, eni := range enis {
		if len(eni.Ipv4Addresses) > 1 {
			continue
		}
		cleanENIRequest := &pb.CleanNetworkForENIRequest{
			RequestId: util.GenRequestId(),
			Eni:       eni,
		}
		if _, err := c.CleanNetworkForENI(context.Background(), cleanENIRequest); err != nil {
			return errors.Wrapf(err, "Failed to clean network for eni %s", eni.EniId)
		}
		if err := c.cp.FreeENI(eni.EniId); err != nil {
			return errors.Wrapf(err, "Failed to free eni %s", eni.EniId)
		}
		c.logger.Info("Successfully detach and delete idle eni", "eni id", eni.EniId)
		return nil
	}
	return errors.New("Failed to find a idle eni")
}

func (c *eniManager) cleanLeakedENIs() error {
	c.logger.Info("Start cleaning leaked enis")

	leakedENIs, err := c.cp.FindLeakedENIs()
	if err != nil {
		return errors.Wrap(err, "Failed to find leaked enis, skip")
	}
	if len(leakedENIs) == 0 {
		c.logger.Info("No leaked enis found, skip cleaning")
		return nil
	}

	var errs []string
	for _, eni := range leakedENIs {
		if err = c.cp.DeleteENI(eni.ENIId); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", eni.ENIId, err.Error()))
			continue
		}
		c.logger.Info("Successfully deleted leaked eni", "eni id", eni.ENIId)
	}
	if len(errs) != 0 {
		return errors.New(fmt.Sprintf("Failed to delete leaked enis, err: %s", strings.Join(errs, "|")))
	}
	return nil
}
