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

package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud"
	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/network"
	pb "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/protocol"
)

type Proxy struct {
	grpc_health_v1.UnimplementedHealthServer
	pb.UnimplementedNodeServer

	cp cloud.Provider
	nc network.Client
}

func (p Proxy) DescribeAllENIs(ctx context.Context, request *pb.DescribeAllENIsRequest) (*pb.DescribeAllENIsResponse, error) {
	enis, err := p.cp.DescribeAllENIs()
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("[%s], %s", request.GetRequestId(), err.Error()))
	}
	resp := &pb.DescribeAllENIsResponse{
		Enis:      make(map[string]*pb.ENIMetadata),
		RequestId: request.GetRequestId(),
	}
	for index := range enis {
		eni := enis[index]
		var addrs []*pb.IPv4Address
		for i := range eni.IPv4Addresses {
			addr := eni.IPv4Addresses[i]
			addrs = append(addrs, &pb.IPv4Address{
				Address: addr.Address,
				Primary: addr.Primary,
			})
		}
		resp.Enis[enis[index].ID] = &pb.ENIMetadata{
			EniId:          eni.ID,
			Mac:            eni.MAC,
			SubnetId:       eni.SubnetID,
			DeviceNumber:   int32(eni.DeviceNumber),
			SubnetIpv4Cidr: eni.SubnetIPv4CIDR,
			Tags:           eni.Tags,
			Ipv4Addresses:  addrs,
		}
	}
	return resp, nil
}

func (p Proxy) DescribeNodeInfo(ctx context.Context, request *pb.DescribeNodeInfoRequest) (*pb.DescribeNodeInfoResponse, error) {
	info := p.cp.GetInstanceInfo()
	resp := &pb.DescribeNodeInfoResponse{
		RequestId: request.GetRequestId(),
		Info: &pb.InstanceInfo{
			InstanceId:       info.InstanceID,
			SubnetId:         info.SubnetID,
			SecurityGroupIds: info.SecurityGroupIDs,
		},
	}
	return resp, nil
}

func (p Proxy) SetupNetworkForService(ctx context.Context, request *pb.SetupNetworkForServiceRequest) (*pb.SetupNetworkForServiceResponse, error) {
	eni := cloud.ENIMetadata{
		ID:             request.GetEni().GetEniId(),
		MAC:            request.GetEni().GetMac(),
		SubnetID:       request.GetEni().GetSubnetId(),
		DeviceNumber:   int(request.GetEni().GetDeviceNumber()),
		SubnetIPv4CIDR: request.GetEni().GetSubnetIpv4Cidr(),
		Tags:           request.GetEni().GetTags(),
	}
	err := p.nc.SetupNetworkForService(request.PrivateIp, &eni)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("[%s], %s", request.GetRequestId(), err.Error()))
	}
	resp := &pb.SetupNetworkForServiceResponse{
		RequestId: request.GetRequestId(),
		PrivateIp: request.GetPrivateIp(),
		Eni:       request.GetEni(),
	}
	return resp, nil
}

func (p Proxy) CleanNetworkForService(ctx context.Context, request *pb.CleanNetworkForServiceRequest) (*pb.CleanNetworkForServiceResponse, error) {
	eni := cloud.ENIMetadata{
		ID:             request.GetEni().GetEniId(),
		MAC:            request.GetEni().GetMac(),
		SubnetID:       request.GetEni().GetSubnetId(),
		DeviceNumber:   int(request.GetEni().GetDeviceNumber()),
		SubnetIPv4CIDR: request.GetEni().GetSubnetIpv4Cidr(),
		Tags:           request.GetEni().GetTags(),
	}
	err := p.nc.CleanNetworkForService(request.PrivateIp, &eni)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("[%s], %s", request.GetRequestId(), err.Error()))
	}
	resp := &pb.CleanNetworkForServiceResponse{
		RequestId: request.GetRequestId(),
		PrivateIp: request.GetPrivateIp(),
		Eni:       request.GetEni(),
	}
	return resp, nil
}

func (p Proxy) SetupNetworkForENI(ctx context.Context, request *pb.SetupNetworkForENIRequest) (*pb.SetupNetworkForENIResponse, error) {
	eni := cloud.ENIMetadata{
		ID:             request.GetEni().GetEniId(),
		MAC:            request.GetEni().GetMac(),
		SubnetID:       request.GetEni().GetSubnetId(),
		DeviceNumber:   int(request.GetEni().GetDeviceNumber()),
		SubnetIPv4CIDR: request.GetEni().GetSubnetIpv4Cidr(),
		Tags:           request.GetEni().GetTags(),
	}
	err := p.nc.SetupNetworkForENI(&eni)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("[%s], %s", request.GetRequestId(), err.Error()))
	}
	resp := &pb.SetupNetworkForENIResponse{
		RequestId: request.GetRequestId(),
		Eni:       request.GetEni(),
	}
	return resp, nil

}

func (p Proxy) CleanNetworkForENI(ctx context.Context, request *pb.CleanNetworkForENIRequest) (*pb.CleanNetworkForENIResponse, error) {
	eni := cloud.ENIMetadata{
		ID:             request.GetEni().GetEniId(),
		MAC:            request.GetEni().GetMac(),
		DeviceNumber:   int(request.GetEni().GetDeviceNumber()),
		SubnetIPv4CIDR: request.GetEni().GetSubnetIpv4Cidr(),
		Tags:           request.GetEni().GetTags(),
	}
	err := p.nc.CleanNetworkForENI(&eni)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("[%s], %s", request.GetRequestId(), err.Error()))
	}
	resp := &pb.CleanNetworkForENIResponse{
		RequestId: request.GetRequestId(),
		Eni:       request.GetEni(),
	}
	return resp, nil

}

func (p Proxy) Check(context.Context, *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}
