package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/agent"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/network"
	pb "github.com/apecloud/kubeblocks/internal/loadbalancer/protocol"
)

type Proxy struct {
	grpc_health_v1.UnimplementedHealthServer
	pb.UnimplementedNodeServer

	nc network.Client
	em agent.ENIManager
}

func (p Proxy) ChooseBusiestENI(ctx context.Context, request *pb.ChooseBusiestENIRequest) (*pb.ChooseBusiestENIResponse, error) {
	eni, err := p.em.ChooseBusiestENI()
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("[%s], %s", request.GetRequestId(), err.Error()))
	}

	var addrs []*pb.IPv4Address
	for i := range eni.IPv4Addresses {
		addr := eni.IPv4Addresses[i]
		addrs = append(addrs, &pb.IPv4Address{
			Address: addr.Address,
			Primary: addr.Primary,
		})
	}
	resp := &pb.ChooseBusiestENIResponse{
		RequestId: request.GetRequestId(),
		Eni: &pb.ENIMetadata{
			EniId:          eni.ENIId,
			Mac:            eni.MAC,
			DeviceNumber:   int32(eni.DeviceNumber),
			SubnetIpv4Cidr: eni.SubnetIPv4CIDR,
			Tags:           eni.Tags,
			Ipv4Addresses:  addrs,
		},
	}
	return resp, nil
}

func (p Proxy) GetManagedENIs(ctx context.Context, request *pb.GetManagedENIsRequest) (*pb.GetManagedENIsResponse, error) {
	enis, err := p.em.GetManagedENIs()
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("[%s], %s", request.GetRequestId(), err.Error()))
	}
	resp := &pb.GetManagedENIsResponse{
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
		resp.Enis[enis[index].ENIId] = &pb.ENIMetadata{
			EniId:          eni.ENIId,
			Mac:            eni.MAC,
			DeviceNumber:   int32(eni.DeviceNumber),
			SubnetIpv4Cidr: eni.SubnetIPv4CIDR,
			Tags:           eni.Tags,
			Ipv4Addresses:  addrs,
		}
	}
	return resp, nil
}

func (p Proxy) SetupNetworkForService(ctx context.Context, request *pb.SetupNetworkForServiceRequest) (*pb.SetupNetworkForServiceResponse, error) {
	eni := cloud.ENIMetadata{
		ENIId:          request.GetEni().GetEniId(),
		MAC:            request.GetEni().GetMac(),
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
		ENIId:          request.GetEni().GetEniId(),
		MAC:            request.GetEni().GetMac(),
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

func (p Proxy) Check(context.Context, *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}
