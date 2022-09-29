package agent

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/apecloud/kubeblocks/internal/dbctl/util"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/network"
)

const (
	EnvMaxENI     = "MAX_ENI"
	DefaultMaxENI = -1

	EnvMinPrivateIP     = "MIN_PRIVATE_IP"
	DefaultMinPrivateIP = 1
)

type eniManager struct {
	sync.RWMutex
	logger logr.Logger

	cp           cloud.Provider
	nc           network.Client
	maxIPsPerENI int
	maxENI       int
	minPrivateIP int
}

func NewENIManager(logger logr.Logger, cp cloud.Provider, nc network.Client) (*eniManager, error) {
	c := &eniManager{
		cp:     cp,
		nc:     nc,
		logger: logger,
	}

	ipStr, found := os.LookupEnv(EnvMinPrivateIP)
	envMin := DefaultMinPrivateIP
	if found {
		if input, err := strconv.Atoi(ipStr); err == nil && input >= 1 {
			c.logger.Info("Using MIN_PRIVATE_IP", "count", input)
			envMin = input
		}
	}
	c.minPrivateIP = envMin

	if err := c.initENIAndIPLimits(); err != nil {
		return nil, errors.Wrap(err, "Failed to get eni and private ip limits")
	}

	return c, c.init()
}

func (c *eniManager) Start() error {
	enis, err := c.cp.DescribeAllENIs()
	if err != nil {
		return errors.Wrap(err, "Failed to get all enis, retry later")
	}

	var primaryENI *cloud.ENIMetadata
	for _, eni := range enis {
		if eni.DeviceNumber == 0 {
			primaryENI = eni
			break
		}
	}
	if primaryENI == nil {
		return errors.Wrap(err, "Failed to find primary eni")
	}
	// TODO enable check when we can ensure traffic comes in and out from same interface
	if err := c.cp.ModifySourceDestCheck(primaryENI.ENIId, true); err != nil {
		return errors.Wrap(err, "Failed to disable primary eni source/destination check")
	}
	c.logger.Info("Successfully disable primary eni source/destination check")

	worker := func() {
		if err := c.ensureENI(); err != nil {
			c.logger.Error(err, "Failed to ensure eni")
		}
	}
	go wait.Forever(worker, 15*time.Second)

	return nil
}

func (c *eniManager) init() error {
	managedENIs, err := c.GetManagedENIs()
	if err != nil {
		return errors.Wrap(err, "ipamd init: failed to retrieve attached ENIs info")
	}

	for i := range managedENIs {
		eni := managedENIs[i]
		c.logger.Info("Discovered managed ENI, trying to set it up", "eni id", eni.ENIId)

		options := &util.RetryOptions{MaxRetry: 3, Delay: 10 * time.Second}
		if err := util.DoWithRetry(context.Background(), c.logger, func() error {
			return c.nc.SetupNetworkForENI(eni)
		}, options); err != nil {
			c.logger.Error(err, "Failed to setup ENI", "eni id", eni.ENIId)
		} else {
			c.logger.Info("ENI set up completed", "eni id", eni.ENIId)
		}
	}
	c.logger.Info("Successfully init node")

	return nil
}

func (c *eniManager) GetManagedENIs() ([]*cloud.ENIMetadata, error) {
	enis, err := c.cp.DescribeAllENIs()
	if err != nil {
		return nil, errors.Wrap(err, "ipamd init: failed to retrieve attached ENIs info")
	}

	return c.filterManagedENIs(enis), nil
}

func (c *eniManager) filterManagedENIs(enis map[string]*cloud.ENIMetadata) []*cloud.ENIMetadata {
	var (
		ids            []string
		managedENIList []*cloud.ENIMetadata
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

func (c *eniManager) ChooseBusiestENI() (*cloud.ENIMetadata, error) {
	enis, err := c.cp.DescribeAllENIs()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get attached ENIs info")
	}

	managedENIs := c.filterManagedENIs(enis)
	if len(managedENIs) == 0 {
		return nil, errors.New("No managed eni found")
	}
	candidate := managedENIs[0]
	for _, eni := range managedENIs {
		if len(eni.IPv4Addresses) > len(candidate.IPv4Addresses) && len(eni.IPv4Addresses) < c.maxIPsPerENI {
			candidate = eni
		}
	}
	c.logger.Info("Found busiest eni", "eni id", candidate.ENIId)
	return candidate, nil
}

func (c *eniManager) ensureENI() error {
	enis, err := c.cp.DescribeAllENIs()
	if err != nil {
		return errors.Wrap(err, "Failed to get all enis, retry later")
	}

	var (
		managedENIs = c.filterManagedENIs(enis)
		status      = make(map[string]interface{}, len(managedENIs))
		totalSpare  int
		min         = c.minPrivateIP
		max         = c.minPrivateIP + c.maxIPsPerENI
	)
	for _, eni := range managedENIs {
		spare := c.maxIPsPerENI - len(eni.IPv4Addresses)
		status[eni.ENIId] = map[string]int{
			"total": c.maxIPsPerENI,
			"used":  len(eni.IPv4Addresses),
			"spare": spare,
		}
		totalSpare += spare
	}

	c.logger.Info("Local private ip buffer status",
		"spare private ip", totalSpare, "min spare private ip", min, "max spare private ip", max)

	b, _ := json.Marshal(status)
	c.logger.Info("Local private ip buffer status", "info", string(b))

	if totalSpare < min {
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

	enis, err := c.GetManagedENIs()
	if err != nil {
		return errors.Wrap(err, "Failed to get managed enis")
	}
	if len(enis) >= c.maxENI {
		c.logger.Info("Limit exceed, can not alloc new eni", "current", enis, "max", c.maxENI)
		return nil
	}

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
	if err := c.nc.SetupNetworkForENI(&eni); err != nil {
		return errors.Wrapf(err, "Failed to set up network for eni %s", eni.ENIId)
	}
	c.logger.Info("Successfully initialized new eni", "eni id", eniId)
	return nil
}

func (c *eniManager) tryDetachAndDeleteENI(enis []*cloud.ENIMetadata) error {
	c.logger.Info("Try to detach and delete idle eni")

	for _, eni := range enis {
		if len(eni.IPv4Addresses) > 1 {
			continue
		}
		if !aws.BoolValue(eni.IPv4Addresses[0].Primary) {
			continue
		}
		if err := c.nc.CleanNetworkForENI(eni); err != nil {
			return errors.Wrapf(err, "Failed to clean network for eni %s", eni.ENIId)
		}
		if err := c.cp.FreeENI(eni.ENIId); err != nil {
			return errors.Wrapf(err, "Failed to free eni %s", eni.ENIId)
		}
		c.logger.Info("Successfully detach and delete idle eni", "eni id", eni.ENIId)
		return nil
	}
	return errors.New("Failed to find a idle eni")
}

func (c *eniManager) initENIAndIPLimits() (err error) {
	nodeMaxENI, err := c.getMaxENI()
	if err != nil {
		c.logger.Error(err, "Failed to get ENI limit")
		return err
	}
	c.maxENI = nodeMaxENI

	c.maxIPsPerENI = c.cp.GetENIIPv4Limit()
	if err != nil {
		return err
	}
	c.logger.Info("Query resource quota", "max eni", c.maxENI, "max private ip per eni", c.maxIPsPerENI)

	return nil
}

// getMaxENI returns the maximum number of ENIs to attach to this instance. This is calculated as the lesser of
// the limit for the instance type and the value configured via the MAX_ENI environment variable. If the value of
// the environment variable is 0 or less, it will be ignored and the maximum for the instance is returned.
func (c *eniManager) getMaxENI() (int, error) {
	instanceMaxENI := c.cp.GetENILimit()

	inputStr, found := os.LookupEnv(EnvMaxENI)
	envMax := DefaultMaxENI
	if found {
		if input, err := strconv.Atoi(inputStr); err == nil && input >= 1 {
			c.logger.Info("Using MAX_ENI", "count", input)
			envMax = input
		}
	}

	if envMax >= 1 && envMax < instanceMaxENI {
		return envMax, nil
	}
	return instanceMaxENI, nil
}
