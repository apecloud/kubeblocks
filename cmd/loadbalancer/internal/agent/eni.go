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

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud"
	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/config"
	pb "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/protocol"
	"github.com/apecloud/kubeblocks/internal/cli/util"
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
	SubnetID        string
	TotalPrivateIPs int
	UsedPrivateIPs  int
}

type eniManager struct {
	logger           logr.Logger
	maxIPsPerENI     int
	maxENI           int
	minPrivateIP     int
	instanceID       string
	subnetID         string
	securityGroupIds []string
	resource         *NodeResource
	cp               cloud.Provider
	nc               pb.NodeClient
}

func newENIManager(logger logr.Logger, ip string, info *pb.InstanceInfo, nc pb.NodeClient, cp cloud.Provider) (*eniManager, error) {
	c := &eniManager{
		nc: nc,
		cp: cp,
	}

	c.instanceID = info.GetInstanceId()
	c.subnetID = info.GetSubnetId()
	c.securityGroupIds = info.GetSecurityGroupIds()
	c.logger = logger.WithValues("ip", ip, "instance id", c.instanceID)

	c.minPrivateIP = config.MinPrivateIP
	c.maxIPsPerENI = cp.GetENIIPv4Limit()

	c.maxENI = cp.GetENILimit()
	if config.MaxENI > 0 && config.MaxENI < c.maxENI {
		c.maxENI = config.MaxENI
	}

	return c, c.init()
}

func (c *eniManager) init() error {
	managedENIs, err := c.getManagedENIs()
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
				RequestId: util.GenRequestID(),
				Eni:       eni,
			}
			_, err = c.nc.SetupNetworkForENI(context.Background(), setupENIRequest)
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
			SubnetID:        eni.SubnetId,
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
	describeENIRequest := &pb.DescribeAllENIsRequest{RequestId: util.GenRequestID()}
	describeENIResponse, err := c.nc.DescribeAllENIs(context.Background(), describeENIRequest)
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

func (c *eniManager) getManagedENIs() ([]*pb.ENIMetadata, error) {
	describeENIRequest := &pb.DescribeAllENIsRequest{RequestId: util.GenRequestID()}
	describeENIResponse, err := c.nc.DescribeAllENIs(context.Background(), describeENIRequest)
	if err != nil {
		return nil, errors.Wrap(err, "ipamd init: failed to retrieve attached ENIs info")
	}

	return c.filterManagedENIs(describeENIResponse.GetEnis()), nil
}

func (c *eniManager) filterManagedENIs(enis map[string]*pb.ENIMetadata) []*pb.ENIMetadata {
	var (
		managedENIList []*pb.ENIMetadata
	)
	for eniID, eni := range enis {
		if _, found := eni.Tags[cloud.TagENIKubeBlocksManaged]; !found {
			continue
		}

		// ignore primary ENI even if tagged
		if eni.DeviceNumber == 0 {
			continue
		}

		managedENIList = append(managedENIList, enis[eniID])
	}
	return managedENIList
}

func (c *eniManager) ensureENI() error {
	describeENIRequest := &pb.DescribeAllENIsRequest{RequestId: util.GenRequestID()}
	describeENIResponse, err := c.nc.DescribeAllENIs(context.Background(), describeENIRequest)
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
			c.logger.Info("Limit exceed, can not create new eni", "current", len(describeENIResponse.Enis), "max", c.maxENI)
			return nil
		}
		if err := c.tryCreateAndAttachENI(); err != nil {
			c.logger.Error(err, "Failed to create and attach new ENI")
		}
	} else if totalSpare > max {
		if err := c.tryDetachAndDeleteENI(managedENIs); err != nil {
			c.logger.Error(err, "Failed to detach and delete idle ENI")
		}
	}
	return nil
}

func (c *eniManager) tryCreateAndAttachENI() error {
	c.logger.Info("Try to create and attach new eni")

	// create ENI, use same sg and subnet as primary ENI
	eniID, err := c.cp.CreateENI(c.instanceID, c.subnetID, c.securityGroupIds)
	if err != nil {
		return errors.Wrap(err, "Failed to create ENI, retry later")
	}
	c.logger.Info("Successfully create new eni", "eni id", eniID)

	if _, err = c.cp.AttachENI(c.instanceID, eniID); err != nil {
		if derr := c.cp.DeleteENI(eniID); derr != nil {
			c.logger.Error(derr, "Failed to delete newly created untagged ENI!")
		}
		return errors.Wrap(err, "Failed to attach ENI")
	}
	c.logger.Info("Successfully attach new eni, waiting for it to take effect", "eni id", eniID)

	// waiting for ENI attached
	if err := c.waitForENIAttached(eniID); err != nil {
		return errors.Wrap(err, "Unable to discover attached ENI from metadata service")
	}
	c.logger.Info("Successfully find eni attached", "eni id", eniID)

	// setup ENI networking stack
	setupENIRequest := &pb.SetupNetworkForENIRequest{
		RequestId: util.GenRequestID(),
		Eni: &pb.ENIMetadata{
			EniId: eniID,
		},
	}
	if _, err = c.nc.SetupNetworkForENI(context.Background(), setupENIRequest); err != nil {
		return errors.Wrapf(err, "Failed to set up network for eni %s", eniID)
	}
	c.logger.Info("Successfully initialized new eni", "eni id", eniID)
	return nil
}

func (c *eniManager) tryDetachAndDeleteENI(enis []*pb.ENIMetadata) error {
	c.logger.Info("Try to detach and delete idle eni")

	for _, eni := range enis {
		if len(eni.Ipv4Addresses) > 1 {
			continue
		}
		cleanENIRequest := &pb.CleanNetworkForENIRequest{
			RequestId: util.GenRequestID(),
			Eni:       eni,
		}
		if _, err := c.nc.CleanNetworkForENI(context.Background(), cleanENIRequest); err != nil {
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

	leakedENIs, err := c.cp.FindLeakedENIs(c.instanceID)
	if err != nil {
		return errors.Wrap(err, "Failed to find leaked enis, skip")
	}
	if len(leakedENIs) == 0 {
		c.logger.Info("No leaked enis found, skip cleaning")
		return nil
	}

	var errs []string
	for _, eni := range leakedENIs {
		if err = c.cp.DeleteENI(eni.ID); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", eni.ID, err.Error()))
			continue
		}
		c.logger.Info("Successfully deleted leaked eni", "eni id", eni.ID)
	}
	if len(errs) != 0 {
		return errors.New(fmt.Sprintf("Failed to delete leaked enis, err: %s", strings.Join(errs, "|")))
	}
	return nil
}

func (c *eniManager) waitForENIAttached(eniID string) error {
	request := &pb.WaitForENIAttachedRequest{
		RequestId: util.GenRequestID(),
		Eni:       &pb.ENIMetadata{EniId: eniID},
	}
	_, err := c.nc.WaitForENIAttached(context.Background(), request)
	return err
}
