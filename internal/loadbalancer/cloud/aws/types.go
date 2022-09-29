package aws

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type EC2 interface {
	CreateNetworkInterfaceWithContext(ctx aws.Context, input *ec2.CreateNetworkInterfaceInput, opts ...request.Option) (*ec2.CreateNetworkInterfaceOutput, error)

	DescribeInstancesWithContext(ctx aws.Context, input *ec2.DescribeInstancesInput, opts ...request.Option) (*ec2.DescribeInstancesOutput, error)

	DescribeInstanceTypesWithContext(ctx aws.Context, input *ec2.DescribeInstanceTypesInput, opts ...request.Option) (*ec2.DescribeInstanceTypesOutput, error)

	AttachNetworkInterfaceWithContext(ctx aws.Context, input *ec2.AttachNetworkInterfaceInput, opts ...request.Option) (*ec2.AttachNetworkInterfaceOutput, error)

	DeleteNetworkInterfaceWithContext(ctx aws.Context, input *ec2.DeleteNetworkInterfaceInput, opts ...request.Option) (*ec2.DeleteNetworkInterfaceOutput, error)

	DetachNetworkInterfaceWithContext(ctx aws.Context, input *ec2.DetachNetworkInterfaceInput, opts ...request.Option) (*ec2.DetachNetworkInterfaceOutput, error)

	AssignPrivateIpAddressesWithContext(ctx aws.Context, input *ec2.AssignPrivateIpAddressesInput, opts ...request.Option) (*ec2.AssignPrivateIpAddressesOutput, error)

	UnassignPrivateIpAddressesWithContext(ctx aws.Context, input *ec2.UnassignPrivateIpAddressesInput, opts ...request.Option) (*ec2.UnassignPrivateIpAddressesOutput, error)

	DescribeNetworkInterfacesWithContext(ctx aws.Context, input *ec2.DescribeNetworkInterfacesInput, opts ...request.Option) (*ec2.DescribeNetworkInterfacesOutput, error)

	ModifyNetworkInterfaceAttributeWithContext(ctx aws.Context, input *ec2.ModifyNetworkInterfaceAttributeInput, opts ...request.Option) (*ec2.ModifyNetworkInterfaceAttributeOutput, error)

	CreateTagsWithContext(ctx aws.Context, input *ec2.CreateTagsInput, opts ...request.Option) (*ec2.CreateTagsOutput, error)

	DescribeNetworkInterfacesPagesWithContext(ctx aws.Context, input *ec2.DescribeNetworkInterfacesInput, fn func(*ec2.DescribeNetworkInterfacesOutput, bool) bool, opts ...request.Option) error
}

type IMDS interface {
	Region() (string, error)

	GetMetadataWithContext(ctx context.Context, p string) (string, error)
}
