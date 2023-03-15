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

package aws

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

const (
	// ENINoManageTagKey is the tag that may be set on an ENI to indicate aws vpc cni should not manage it in any form.
	ENINoManageTagKey = "node.k8s.amazonaws.com/no_manage"

	ErrCodeENINotFound = "InvalidNetworkInterfaceID.NotFound"

	// MinENILifeTime is the minimum lifetime for ENI being garbage collected
	MinENILifeTime = 10 * time.Minute
)

var (
	// ErrENINotFound is an error when ENI is not found.
	ErrENINotFound = errors.New("ENI is not found")

	// ErrNoNetworkInterfaces occurs when DescribeNetworkInterfaces(eniID) returns no network interfaces
	ErrNoNetworkInterfaces = errors.New("No network interfaces found for ENI")
)

type awsService struct {
	ec2Svc           EC2
	imdsSvc          imdsService
	securityGroups   []string
	instanceID       string
	subnetID         string
	localIPv4        string
	instanceType     string
	primaryENI       string
	primaryENImac    string
	availabilityZone string
	eniLimit         int
	eniIPv4Limit     int
	logger           logr.Logger
}

func NewAwsService(logger logr.Logger) (*awsService, error) {
	svc := &awsService{
		logger: logger,
	}
	awsCfg := aws.Config{
		MaxRetries: aws.Int(2),
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		STSRegionalEndpoint: endpoints.RegionalSTSEndpoint,
	}
	sess := session.Must(session.NewSession(&awsCfg))
	svc.imdsSvc = imdsService{IMDS: ec2metadata.New(sess)}

	region, err := svc.imdsSvc.Region()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get region")
	}
	awsCfg.WithRegion(region).WithDisableSSL(true)
	svc.ec2Svc = ec2.New(sess.Copy(&awsCfg))

	if err := svc.initWithEC2Metadata(context.Background()); err != nil {
		return nil, errors.Wrap(err, "Failed to init ec2 metadata")
	}

	if err = svc.initInstanceTypeLimits(); err != nil {
		return nil, errors.Wrap(err, "Failed to init instance limits")
	}

	return svc, nil
}

func (c *awsService) initWithEC2Metadata(ctx context.Context) error {
	var kvs []interface{}
	defer func() {
		if len(kvs) > 0 {
			c.logger.Info("Init Instance metadata", kvs...)
		}
	}()

	var err error
	// retrieve availability-zone
	c.availabilityZone, err = c.imdsSvc.getAZ(ctx)
	if err != nil {
		return err
	}
	kvs = append(kvs, "az", c.availabilityZone)

	// retrieve eth0 local-ipv4
	c.localIPv4, err = c.imdsSvc.getLocalIPv4(ctx)
	if err != nil {
		return err
	}
	kvs = append(kvs, "local ip", c.localIPv4)

	// retrieve instance-id
	c.instanceID, err = c.imdsSvc.getInstanceID(ctx)
	if err != nil {
		return err
	}
	kvs = append(kvs, "instance id", c.instanceID)

	// retrieve instance-type
	c.instanceType, err = c.imdsSvc.getInstanceType(ctx)
	if err != nil {
		return err
	}
	kvs = append(kvs, "instance type", c.instanceType)

	// retrieve primary interface's mac
	c.primaryENImac, err = c.imdsSvc.getPrimaryMAC(ctx)
	if err != nil {
		return err
	}
	kvs = append(kvs, "primary eni mac", c.primaryENImac)

	c.primaryENI, err = c.imdsSvc.getInterfaceIDByMAC(ctx, c.primaryENImac)
	if err != nil {
		return err
	}
	kvs = append(kvs, "primary eni id", c.primaryENI)

	// retrieve sub-id
	c.subnetID, err = c.imdsSvc.getSubnetID(ctx, c.primaryENImac)
	if err != nil {
		return err
	}
	kvs = append(kvs, "subnet id", c.subnetID)

	c.securityGroups, err = c.imdsSvc.getSecurityGroupIds(ctx, c.primaryENImac)
	if err != nil {
		return err
	}
	kvs = append(kvs, "security groups", c.securityGroups)

	return nil
}

// GetInstanceInfo return EC2 instance info
func (c *awsService) GetInstanceInfo() *cloud.InstanceInfo {
	return &cloud.InstanceInfo{
		InstanceID:       c.instanceID,
		SubnetID:         c.subnetID,
		SecurityGroupIDs: c.securityGroups,
	}
}

// GetInstanceType return EC2 instance type
func (c *awsService) GetInstanceType() string {
	return c.instanceType
}

// GetENIIPv4Limit return IP address limit per ENI based on EC2 instance type
func (c *awsService) GetENIIPv4Limit() int {
	return c.eniIPv4Limit
}

// GetENILimit returns the number of ENIs can be attached to an instance
func (c *awsService) GetENILimit() int {
	return c.eniLimit
}

func (c *awsService) initInstanceTypeLimits() error {
	describeInstanceTypesInput := &ec2.DescribeInstanceTypesInput{InstanceTypes: []*string{aws.String(c.instanceType)}}
	output, err := c.ec2Svc.DescribeInstanceTypesWithContext(context.Background(), describeInstanceTypesInput)
	if err != nil || len(output.InstanceTypes) != 1 {
		return errors.New(fmt.Sprintf("Failed calling DescribeInstanceTypes for `%s`: %v", c.instanceType, err))
	}
	info := output.InstanceTypes[0]

	c.eniLimit = int(aws.Int64Value(info.NetworkInfo.MaximumNetworkInterfaces))
	c.eniIPv4Limit = int(aws.Int64Value(info.NetworkInfo.Ipv4AddressesPerInterface))

	// Not checking for empty hypervisorType since have seen certain instances not getting this filled.
	if aws.StringValue(info.InstanceType) != "" && c.eniLimit > 0 && c.eniIPv4Limit > 0 {
		return nil
	}
	return errors.New(fmt.Sprintf("Unknown instance type %s", c.instanceType))
}

func (c *awsService) DescribeAllENIs() (map[string]*cloud.ENIMetadata, error) {
	attachedENIList, err := c.GetAttachedENIs()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get local ENI metadata")
	}

	attachedENIMap := make(map[string]cloud.ENIMetadata, len(attachedENIList))
	for _, eni := range attachedENIList {
		attachedENIMap[eni.ID] = eni
	}

	input := &ec2.DescribeNetworkInterfacesInput{
		Filters: []*ec2.Filter{{
			Name: aws.String("attachment.instance-id"),
			Values: []*string{
				aws.String(c.instanceID),
			},
		}},
	}
	response, err := c.ec2Svc.DescribeNetworkInterfacesWithContext(context.Background(), input)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to query attached network interface")
	}

	var (
		result = make(map[string]*cloud.ENIMetadata, len(response.NetworkInterfaces))
	)
	for _, item := range response.NetworkInterfaces {

		c.logger.Info("Found eni", "card index", aws.Int64Value(item.Attachment.NetworkCardIndex),
			"device index", aws.Int64Value(item.Attachment.DeviceIndex),
			"eni id", aws.StringValue(item.NetworkInterfaceId))

		eniID := aws.StringValue(item.NetworkInterfaceId)

		eniMetadata := attachedENIMap[eniID]
		eniMetadata.SubnetID = aws.StringValue(item.SubnetId)
		eniMetadata.Tags = convertSDKTagsToTags(item.TagSet)

		result[eniID] = &eniMetadata

		// Check IPv4 addresses
		c.checkOutOfSyncState(eniID, eniMetadata.IPv4Addresses, item.PrivateIpAddresses)
	}
	return result, nil
}

func (c *awsService) FindLeakedENIs(instanceID string) ([]*cloud.ENIMetadata, error) {
	input := &ec2.DescribeNetworkInterfacesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag-key"),
				Values: []*string{
					aws.String(cloud.TagENINode),
				},
			},
			{
				Name: aws.String("status"),
				Values: []*string{
					aws.String(ec2.NetworkInterfaceStatusAvailable),
				},
			},
		},
		MaxResults: aws.Int64(1000),
	}

	needClean := func(eni *ec2.NetworkInterface) bool {
		ctxLog := c.logger.WithValues("eni id", eni.NetworkInterfaceId)

		var (
			tags  = convertSDKTagsToTags(eni.TagSet)
			eniID = aws.StringValue(eni.NetworkInterfaceId)
		)
		node, ok := tags[cloud.TagENINode]
		if !ok || node != instanceID {
			return true
		}

		if eni.Attachment != nil {
			ctxLog.Info("ENI is attached, skip it", "attachment id", eni.Attachment.AttachmentId)
			return false
		}

		retagMap := map[string]string{
			cloud.TagENICreatedAt: time.Now().Format(time.RFC3339),
		}
		createdAt, ok := tags[cloud.TagENICreatedAt]
		if !ok {
			ctxLog.Info("Timestamp tag not exists, tag it")
			if err := c.tagENI(eniID, retagMap); err != nil {
				ctxLog.Error(err, "Failed to add tag for eni", "eni id", eniID)
			}
			return false
		}

		t, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			ctxLog.Error(err, "Timestamp tag is wrong, retagging it with current timestamp", "time", createdAt)
			if err := c.tagENI(eniID, retagMap); err != nil {
				ctxLog.Error(err, "Failed to retagging for eni", "eni id", eniID)
			}
			return false
		}

		if time.Since(t) < MinENILifeTime {
			ctxLog.Info("Found an leaked eni created less than 10 minutes ago, skip it")
			return false
		}

		return true
	}

	var leakedENIs []*cloud.ENIMetadata
	pageFn := func(output *ec2.DescribeNetworkInterfacesOutput, lastPage bool) bool {
		enis := output.NetworkInterfaces
		for index := range enis {
			eni := enis[index]
			if needClean(eni) {
				leakedENIs = append(leakedENIs, &cloud.ENIMetadata{
					ID: aws.StringValue(eni.NetworkInterfaceId),
				})
			}
		}
		return true
	}
	if err := c.ec2Svc.DescribeNetworkInterfacesPagesWithContext(context.Background(), input, pageFn); err != nil {
		c.logger.Error(err, "")
		return nil, errors.Wrap(err, "Failed to describe leaked enis")
	}

	return leakedENIs, nil
}

func (c *awsService) tagENI(eniID string, tagMap map[string]string) error {
	input := &ec2.CreateTagsInput{
		Resources: []*string{
			aws.String(eniID),
		},
		Tags: convertTagsToSDKTags(tagMap),
	}
	_, err := c.ec2Svc.CreateTagsWithContext(context.Background(), input)
	return err
}

func (c *awsService) AssignPrivateIPAddresses(eniID string, privateIP string) error {
	input := &ec2.AssignPrivateIpAddressesInput{
		NetworkInterfaceId: aws.String(eniID),
		PrivateIpAddresses: []*string{&privateIP},
	}
	if _, err := c.ec2Svc.AssignPrivateIpAddressesWithContext(context.Background(), input); err != nil {
		return errors.Wrapf(err, "Failed to assign private ip address %s on eni %s", privateIP, eniID)
	}
	return nil
}

// Comparing the IMDS IPv4 addresses attached to the ENI with the DescribeNetworkInterfaces AWS API call, which
// technically should be the source of truth and contain the freshest information. Let's just do a quick scan here
// and output some diagnostic messages if we find stale info in the IMDS result.
func (c *awsService) checkOutOfSyncState(eniID string, imdsIPv4s []*cloud.IPv4Address, ec2IPv4s []*ec2.NetworkInterfacePrivateIpAddress) bool {
	ctxLog := c.logger.WithName("checkOutOfSyncState").WithValues("eni id", eniID)

	synced := true

	imdsIPv4Set := sets.String{}
	imdsPrimaryIP := ""
	for _, imdsIPv4 := range imdsIPv4s {
		imdsIPv4Set.Insert(imdsIPv4.Address)
		if imdsIPv4.Primary {
			imdsPrimaryIP = imdsIPv4.Address
		}
	}
	ec2IPv4Set := sets.String{}
	ec2IPv4PrimaryIP := ""
	for _, privateIPv4 := range ec2IPv4s {
		ec2IPv4Set.Insert(aws.StringValue(privateIPv4.PrivateIpAddress))
		if aws.BoolValue(privateIPv4.Primary) {
			ec2IPv4PrimaryIP = aws.StringValue(privateIPv4.PrivateIpAddress)
		}
	}
	missingIMDS := ec2IPv4Set.Difference(imdsIPv4Set).List()
	missingDNI := imdsIPv4Set.Difference(ec2IPv4Set).List()
	if len(missingIMDS) > 0 {
		synced = false
		strMissing := strings.Join(missingIMDS, ",")
		ctxLog.Info("DescribeNetworkInterfaces yielded private IPv4 addresses that were not yet found in IMDS.", "ip list", strMissing)
	}
	if len(missingDNI) > 0 {
		synced = false
		strMissing := strings.Join(missingDNI, ",")
		ctxLog.Info("IMDS query yielded stale IPv4 addresses that were not found in DescribeNetworkInterfaces.", "ip list", strMissing)
	}
	if imdsPrimaryIP != ec2IPv4PrimaryIP {
		synced = false
		ctxLog.Info("Primary IPs do not match", "imds", imdsPrimaryIP, "ec2", ec2IPv4PrimaryIP)
	}
	return synced
}

func (c *awsService) GetAttachedENIs() (eniList []cloud.ENIMetadata, err error) {
	ctx := context.TODO()

	macs, err := c.imdsSvc.getMACs(ctx)
	if err != nil {
		return nil, err
	}
	c.logger.Info("Total number of interfaces found", "count", len(macs))

	enis := make([]cloud.ENIMetadata, len(macs))
	for i, mac := range macs {
		enis[i], err = c.getENIMetadata(mac)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to retrieve metadata for ENI: %s", mac)
		}
	}
	return enis, nil
}

func (c *awsService) getENIMetadata(eniMAC string) (cloud.ENIMetadata, error) {
	var (
		err       error
		deviceNum int
		result    cloud.ENIMetadata
		ctx       = context.TODO()
	)

	eniID, err := c.imdsSvc.getInterfaceIDByMAC(ctx, eniMAC)
	if err != nil {
		return result, err
	}

	deviceNum, err = c.imdsSvc.getInterfaceDeviceNumber(ctx, eniMAC)
	if err != nil {
		return result, err
	}

	primaryMAC, err := c.imdsSvc.getPrimaryMAC(ctx)
	if err != nil {
		return result, err
	}
	if eniMAC == primaryMAC && deviceNum != 0 {
		// Can this even happen? To be backwards compatible, we will always use 0 here and log an error.
		c.logger.Error(errors.New(fmt.Sprintf("Device number of primary ENI is %d! Forcing it to be 0 as expected", deviceNum)), "")
		deviceNum = 0
	}

	cidr, err := c.imdsSvc.getSubnetIPv4CIDRBlock(ctx, eniMAC)
	if err != nil {
		return result, err
	}

	ips, err := c.imdsSvc.getInterfacePrivateAddresses(ctx, eniMAC)
	if err != nil {
		return result, err
	}

	ec2ip4s := make([]*cloud.IPv4Address, len(ips))
	for i, ip := range ips {
		ec2ip4s[i] = &cloud.IPv4Address{
			Primary: i == 0,
			Address: ip,
		}
	}

	return cloud.ENIMetadata{
		ID:             eniID,
		MAC:            eniMAC,
		DeviceNumber:   deviceNum,
		SubnetIPv4CIDR: cidr,
		IPv4Addresses:  ec2ip4s,
	}, nil
}

func (c *awsService) DeallocIPAddresses(eniID string, ips []string) error {
	ctxLog := c.logger.WithValues("eni id", eniID, "ip", ips)
	ctxLog.Info("Trying to unassign private ip from ENI")

	if len(ips) == 0 {
		return nil
	}
	ipsInput := aws.StringSlice(ips)

	input := &ec2.UnassignPrivateIpAddressesInput{
		NetworkInterfaceId: aws.String(eniID),
		PrivateIpAddresses: ipsInput,
	}

	_, err := c.ec2Svc.UnassignPrivateIpAddressesWithContext(context.Background(), input)
	if err != nil {
		if strings.Contains(err.Error(), "InvalidParameterValue: Some of the specified addresses are not assigned to interface") {
			ctxLog.Info("Private ip may has already been unassigned, skip")
			return nil
		}
		return errors.Wrapf(err, "Failed to unassign ip address %s", ips)
	}
	ctxLog.Info("Successfully unassigned IPs from ENI")
	return nil
}

func (c *awsService) AllocIPAddresses(eniID string) (string, error) {
	c.logger.Info("Trying to allocate IP addresses on ENI", "eni id", eniID)

	input := &ec2.AssignPrivateIpAddressesInput{
		NetworkInterfaceId:             aws.String(eniID),
		SecondaryPrivateIpAddressCount: aws.Int64(int64(1)),
	}

	output, err := c.ec2Svc.AssignPrivateIpAddressesWithContext(context.Background(), input)
	if err != nil {
		c.logger.Error(err, "Failed to allocate private ip on ENI", "eni id", eniID)
		return "", err
	}
	if output != nil {
		c.logger.Info("Successfully allocated private IP addresses", "ip list", output.AssignedPrivateIpAddresses)
	}
	return aws.StringValue(output.AssignedPrivateIpAddresses[0].PrivateIpAddress), nil
}

func (c *awsService) CreateENI(instanceID, subnetID string, securityGroupIds []string) (string, error) {
	ctxLog := c.logger.WithValues("instance id", instanceID)
	ctxLog.Info("Trying to create ENI")

	eniDescription := fmt.Sprintf("kubeblocks-lb-%s", instanceID)
	tags := map[string]string{
		cloud.TagENINode:              instanceID,
		cloud.TagENICreatedAt:         time.Now().Format(time.RFC3339),
		cloud.TagENIKubeBlocksManaged: "true",
		ENINoManageTagKey:             "true",
	}
	tagSpec := []*ec2.TagSpecification{
		{
			Tags:         convertTagsToSDKTags(tags),
			ResourceType: aws.String(ec2.ResourceTypeNetworkInterface),
		},
	}

	input := &ec2.CreateNetworkInterfaceInput{
		Description:       aws.String(eniDescription),
		Groups:            aws.StringSlice(securityGroupIds),
		SubnetId:          aws.String(subnetID),
		TagSpecifications: tagSpec,
	}

	result, err := c.ec2Svc.CreateNetworkInterfaceWithContext(context.Background(), input)
	if err != nil {
		return "", errors.Wrap(err, "failed to create network interface")
	}
	ctxLog.Info("Successfully created new ENI", "id", aws.StringValue(result.NetworkInterface.NetworkInterfaceId))

	return aws.StringValue(result.NetworkInterface.NetworkInterfaceId), nil
}

func (c *awsService) AttachENI(instanceID string, eniID string) (string, error) {
	ctxLog := c.logger.WithValues("instance id", instanceID, "eni id", eniID)
	ctxLog.Info("Trying to attach ENI")

	freeDevice, err := c.awsGetFreeDeviceNumber(instanceID)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get a free device number")
	}

	attachInput := &ec2.AttachNetworkInterfaceInput{
		DeviceIndex:        aws.Int64(int64(freeDevice)),
		InstanceId:         aws.String(instanceID),
		NetworkInterfaceId: aws.String(eniID),
	}
	attachOutput, err := c.ec2Svc.AttachNetworkInterfaceWithContext(context.Background(), attachInput)
	if err != nil {
		return "", errors.Wrap(err, "Failed to attach ENI")
	}

	attachmentID := aws.StringValue(attachOutput.AttachmentId)
	// Also change the ENI's attribute so that the ENI will be deleted when the instance is deleted.
	attributeInput := &ec2.ModifyNetworkInterfaceAttributeInput{
		Attachment: &ec2.NetworkInterfaceAttachmentChanges{
			AttachmentId:        aws.String(attachmentID),
			DeleteOnTermination: aws.Bool(true),
		},
		NetworkInterfaceId: aws.String(eniID),
	}

	if _, err := c.ec2Svc.ModifyNetworkInterfaceAttributeWithContext(context.Background(), attributeInput); err != nil {
		if err := c.FreeENI(eniID); err != nil {
			c.logger.Error(err, "Failed to delete newly created untagged ENI!")
		}
		return "", errors.Wrap(err, "Failed to change the ENI's attribute")
	}
	return attachmentID, err
}

func (c *awsService) awsGetFreeDeviceNumber(instanceID string) (int, error) {
	input := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(instanceID)},
	}

	result, err := c.ec2Svc.DescribeInstancesWithContext(context.Background(), input)
	if err != nil {
		return 0, errors.Wrap(err, "Failed to retrieve instance data from EC2 control plane")
	}

	if len(result.Reservations) != 1 {
		return 0, errors.New(fmt.Sprintf("invalid instance id %s", instanceID))
	}

	// TODO race condition with vpc cni
	inst := result.Reservations[0].Instances[0]
	device := make(map[int]bool, len(inst.NetworkInterfaces))
	for _, eni := range inst.NetworkInterfaces {
		device[int(aws.Int64Value(eni.Attachment.DeviceIndex))] = true
	}

	for freeDeviceIndex := 0; freeDeviceIndex < math.MaxInt; freeDeviceIndex++ {
		if !device[freeDeviceIndex] {
			c.logger.Info("Found a free device number", "index", freeDeviceIndex)
			return freeDeviceIndex, nil
		}
	}
	return 0, errors.New("no available device number")
}

func (c *awsService) DeleteENI(eniID string) error {
	ctxLog := c.logger.WithValues("eni id", eniID)
	ctxLog.Info("Trying to delete ENI")

	deleteInput := &ec2.DeleteNetworkInterfaceInput{
		NetworkInterfaceId: aws.String(eniID),
	}
	f := func() error {
		if _, err := c.ec2Svc.DeleteNetworkInterfaceWithContext(context.Background(), deleteInput); err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				if awsErr.Code() == ErrCodeENINotFound {
					ctxLog.Info("ENI has already been deleted")
					return nil
				}
			}
			return errors.Wrapf(err, "Failed to delete ENI")
		}
		ctxLog.Info("Successfully deleted ENI")
		return nil
	}
	return util.DoWithRetry(context.Background(), c.logger, f, &util.RetryOptions{MaxRetry: 10, Delay: 3 * time.Second})
}

func (c *awsService) FreeENI(eniID string) error {
	return c.freeENI(eniID, 2*time.Second)
}

func (c *awsService) freeENI(eniID string, sleepDelayAfterDetach time.Duration) error {
	ctxLog := c.logger.WithName("freeENI").WithValues("eni id", eniID)
	ctxLog.Info("Trying to free eni")

	// Find out attachment
	attachID, err := c.getENIAttachmentID(ctxLog, eniID)
	if err != nil {
		if err == ErrENINotFound {
			ctxLog.Info("ENI not found. It seems to be already freed")
			return nil
		}
		ctxLog.Error(err, "Failed to retrieve ENI")
		return errors.Wrap(err, "Failed to retrieve ENI's attachment id")
	}
	ctxLog.Info("Found ENI attachment id", "attachment id", aws.StringValue(attachID))

	detachInput := &ec2.DetachNetworkInterfaceInput{
		AttachmentId: attachID,
	}
	f := func() error {
		if _, err := c.ec2Svc.DetachNetworkInterfaceWithContext(context.Background(), detachInput); err != nil {
			return errors.Wrap(err, "Failed to detach ENI")
		}
		return nil
	}

	// Retry detaching the ENI from the instance
	err = util.DoWithRetry(context.Background(), c.logger, f, &util.RetryOptions{MaxRetry: 10, Delay: 3 * time.Second})
	if err != nil {
		return err
	}
	ctxLog.Info("Successfully detached ENI")

	// It does take awhile for EC2 to detach ENI from instance, so we wait 2s before trying the delete.
	time.Sleep(sleepDelayAfterDetach)
	err = c.DeleteENI(eniID)
	if err != nil {
		return errors.Wrapf(err, "FreeENI: failed to free ENI: %s", eniID)
	}

	ctxLog.Info("Successfully freed ENI")
	return nil
}

func (c *awsService) getENIAttachmentID(ctxLog logr.Logger, eniID string) (*string, error) {
	ctxLog = ctxLog.WithName("getENIAttachmentID").WithValues("eni id", eniID)
	ctxLog.Info("Trying to get ENI attachment id")

	eniIds := make([]*string, 0)
	eniIds = append(eniIds, aws.String(eniID))
	input := &ec2.DescribeNetworkInterfacesInput{
		NetworkInterfaceIds: eniIds,
	}

	result, err := c.ec2Svc.DescribeNetworkInterfacesWithContext(context.Background(), input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == ErrCodeENINotFound {
				return nil, ErrENINotFound
			}
		}
		ctxLog.Error(err, "Failed to get ENI information from EC2 control plane")
		return nil, errors.Wrap(err, "failed to describe network interface")
	}

	// Shouldn't happen, but let's be safe
	if len(result.NetworkInterfaces) == 0 {
		return nil, ErrNoNetworkInterfaces
	}
	firstNI := result.NetworkInterfaces[0]

	// We cannot assume that the NetworkInterface.Attachment field is a non-nil
	// pointer to a NetworkInterfaceAttachment struct.
	// Ref: https://github.com/aws/amazon-vpc-cni-k8s/issues/914
	var attachID *string
	if firstNI.Attachment != nil {
		attachID = firstNI.Attachment.AttachmentId
	}
	return attachID, nil
}

func (c *awsService) WaitForENIAttached(eniID string) (eniMetadata cloud.ENIMetadata, err error) {
	var result cloud.ENIMetadata
	f := func() error {
		enis, err := c.GetAttachedENIs()
		if err != nil {
			c.logger.Error(err, "Failed to discover attached ENIs")
			return ErrNoNetworkInterfaces
		}
		for _, eni := range enis {
			if eniID == eni.ID {
				result = eni
				return nil
			}
		}
		return ErrENINotFound
	}
	if err = util.DoWithRetry(context.Background(), c.logger, f, &util.RetryOptions{MaxRetry: 15, Delay: 3 * time.Second}); err != nil {
		return result, errors.New("Giving up trying to retrieve ENIs from metadata service")
	}
	return result, nil
}

func (c *awsService) ModifySourceDestCheck(eniID string, enabled bool) error {
	input := &ec2.ModifyNetworkInterfaceAttributeInput{
		NetworkInterfaceId: aws.String(eniID),
		SourceDestCheck:    &ec2.AttributeBooleanValue{Value: aws.Bool(enabled)},
	}
	_, err := c.ec2Svc.ModifyNetworkInterfaceAttributeWithContext(context.Background(), input)
	return err
}

func convertTagsToSDKTags(tagsMap map[string]string) []*ec2.Tag {
	if len(tagsMap) == 0 {
		return nil
	}

	sdkTags := make([]*ec2.Tag, 0, len(tagsMap))
	for _, key := range sets.StringKeySet(tagsMap).List() {
		sdkTags = append(sdkTags, &ec2.Tag{
			Key:   aws.String(key),
			Value: aws.String(tagsMap[key]),
		})
	}
	return sdkTags
}

func convertSDKTagsToTags(sdkTags []*ec2.Tag) map[string]string {
	if len(sdkTags) == 0 {
		return nil
	}

	tagsMap := make(map[string]string, len(sdkTags))
	for _, sdkTag := range sdkTags {
		tagsMap[aws.StringValue(sdkTag.Key)] = aws.StringValue(sdkTag.Value)
	}
	return tagsMap
}
