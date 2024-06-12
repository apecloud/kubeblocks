// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v4.25.2
// source: engine_plugin.proto

package plugin

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// EnginePluginClient is the client API for EnginePlugin service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type EnginePluginClient interface {
	GetPluginInfo(ctx context.Context, in *GetPluginInfoRequest, opts ...grpc.CallOption) (*GetPluginInfoResponse, error)
	// IsEngineReady defines the mechanism to probe the readiness of the database.
	IsEngineReady(ctx context.Context, in *IsEngineReadyRequest, opts ...grpc.CallOption) (*IsEngineReadyResponse, error)
	// GetRole defines the mechanism to probe the role of replicas. The return role
	// must be one of the names defined in the componentdefinition roles.
	GetRole(ctx context.Context, in *GetRoleRequest, opts ...grpc.CallOption) (*GetRoleResponse, error)
	// MemberJoin defines how to add a new replica to the replication group.
	// This action is typically invoked when a new replica needs to be added,
	// such as during scale-out. It may involve updating configuration,
	// notifying other members, and ensuring data consistency.
	JoinMember(ctx context.Context, in *JoinMemberRequest, opts ...grpc.CallOption) (*JoinMemberResponse, error)
	// MemberLeave defines how to remove a replica from the replication group.
	// This action is typically invoked when a replica needs to be removed,
	// such as during scale-in. It may involve configuration updates and notifying
	// other members about the departure,
	LeaveMember(ctx context.Context, in *LeaveMemberRequest, opts ...grpc.CallOption) (*LeaveMemberResponse, error)
	// ReadOnly defines how to set a replica Engine as read-only.
	ReadOnly(ctx context.Context, in *ReadOnlyRequest, opts ...grpc.CallOption) (*ReadOnlyResponse, error)
	// ReadWrite defines how to set a replica Engine as read-write.
	ReadWrite(ctx context.Context, in *ReadWriteRequest, opts ...grpc.CallOption) (*ReadWriteResponse, error)
	// AccountProvision Defines the procedure to generate a new database account.
	AccountProvision(ctx context.Context, in *AccountProvisionRequest, opts ...grpc.CallOption) (*AccountProvisionResponse, error)
	// Switchover defines the procedure to switch replica roles in a primary-secondary
	// HA DBEngine by promoting the secondary to primary and demoting the current primary to secondary.
	Switchover(ctx context.Context, in *SwitchoverRequest, opts ...grpc.CallOption) (*SwitchoverResponse, error)
}

type enginePluginClient struct {
	cc grpc.ClientConnInterface
}

func NewEnginePluginClient(cc grpc.ClientConnInterface) EnginePluginClient {
	return &enginePluginClient{cc}
}

func (c *enginePluginClient) GetPluginInfo(ctx context.Context, in *GetPluginInfoRequest, opts ...grpc.CallOption) (*GetPluginInfoResponse, error) {
	out := new(GetPluginInfoResponse)
	err := c.cc.Invoke(ctx, "/plugin.v1.EnginePlugin/GetPluginInfo", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *enginePluginClient) IsEngineReady(ctx context.Context, in *IsEngineReadyRequest, opts ...grpc.CallOption) (*IsEngineReadyResponse, error) {
	out := new(IsEngineReadyResponse)
	err := c.cc.Invoke(ctx, "/plugin.v1.EnginePlugin/IsEngineReady", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *enginePluginClient) GetRole(ctx context.Context, in *GetRoleRequest, opts ...grpc.CallOption) (*GetRoleResponse, error) {
	out := new(GetRoleResponse)
	err := c.cc.Invoke(ctx, "/plugin.v1.EnginePlugin/GetRole", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *enginePluginClient) JoinMember(ctx context.Context, in *JoinMemberRequest, opts ...grpc.CallOption) (*JoinMemberResponse, error) {
	out := new(JoinMemberResponse)
	err := c.cc.Invoke(ctx, "/plugin.v1.EnginePlugin/JoinMember", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *enginePluginClient) LeaveMember(ctx context.Context, in *LeaveMemberRequest, opts ...grpc.CallOption) (*LeaveMemberResponse, error) {
	out := new(LeaveMemberResponse)
	err := c.cc.Invoke(ctx, "/plugin.v1.EnginePlugin/LeaveMember", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *enginePluginClient) ReadOnly(ctx context.Context, in *ReadOnlyRequest, opts ...grpc.CallOption) (*ReadOnlyResponse, error) {
	out := new(ReadOnlyResponse)
	err := c.cc.Invoke(ctx, "/plugin.v1.EnginePlugin/ReadOnly", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *enginePluginClient) ReadWrite(ctx context.Context, in *ReadWriteRequest, opts ...grpc.CallOption) (*ReadWriteResponse, error) {
	out := new(ReadWriteResponse)
	err := c.cc.Invoke(ctx, "/plugin.v1.EnginePlugin/ReadWrite", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *enginePluginClient) AccountProvision(ctx context.Context, in *AccountProvisionRequest, opts ...grpc.CallOption) (*AccountProvisionResponse, error) {
	out := new(AccountProvisionResponse)
	err := c.cc.Invoke(ctx, "/plugin.v1.EnginePlugin/AccountProvision", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *enginePluginClient) Switchover(ctx context.Context, in *SwitchoverRequest, opts ...grpc.CallOption) (*SwitchoverResponse, error) {
	out := new(SwitchoverResponse)
	err := c.cc.Invoke(ctx, "/plugin.v1.EnginePlugin/Switchover", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// EnginePluginServer is the server API for EnginePlugin service.
// All implementations must embed UnimplementedEnginePluginServer
// for forward compatibility
type EnginePluginServer interface {
	GetPluginInfo(context.Context, *GetPluginInfoRequest) (*GetPluginInfoResponse, error)
	// IsEngineReady defines the mechanism to probe the readiness of the database.
	IsEngineReady(context.Context, *IsEngineReadyRequest) (*IsEngineReadyResponse, error)
	// GetRole defines the mechanism to probe the role of replicas. The return role
	// must be one of the names defined in the componentdefinition roles.
	GetRole(context.Context, *GetRoleRequest) (*GetRoleResponse, error)
	// MemberJoin defines how to add a new replica to the replication group.
	// This action is typically invoked when a new replica needs to be added,
	// such as during scale-out. It may involve updating configuration,
	// notifying other members, and ensuring data consistency.
	JoinMember(context.Context, *JoinMemberRequest) (*JoinMemberResponse, error)
	// MemberLeave defines how to remove a replica from the replication group.
	// This action is typically invoked when a replica needs to be removed,
	// such as during scale-in. It may involve configuration updates and notifying
	// other members about the departure,
	LeaveMember(context.Context, *LeaveMemberRequest) (*LeaveMemberResponse, error)
	// ReadOnly defines how to set a replica Engine as read-only.
	ReadOnly(context.Context, *ReadOnlyRequest) (*ReadOnlyResponse, error)
	// ReadWrite defines how to set a replica Engine as read-write.
	ReadWrite(context.Context, *ReadWriteRequest) (*ReadWriteResponse, error)
	// AccountProvision Defines the procedure to generate a new database account.
	AccountProvision(context.Context, *AccountProvisionRequest) (*AccountProvisionResponse, error)
	// Switchover defines the procedure to switch replica roles in a primary-secondary
	// HA DBEngine by promoting the secondary to primary and demoting the current primary to secondary.
	Switchover(context.Context, *SwitchoverRequest) (*SwitchoverResponse, error)
	mustEmbedUnimplementedEnginePluginServer()
}

// UnimplementedEnginePluginServer must be embedded to have forward compatible implementations.
type UnimplementedEnginePluginServer struct {
}

func (UnimplementedEnginePluginServer) GetPluginInfo(context.Context, *GetPluginInfoRequest) (*GetPluginInfoResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetPluginInfo not implemented")
}
func (UnimplementedEnginePluginServer) IsEngineReady(context.Context, *IsEngineReadyRequest) (*IsEngineReadyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method IsEngineReady not implemented")
}
func (UnimplementedEnginePluginServer) GetRole(context.Context, *GetRoleRequest) (*GetRoleResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRole not implemented")
}
func (UnimplementedEnginePluginServer) JoinMember(context.Context, *JoinMemberRequest) (*JoinMemberResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method JoinMember not implemented")
}
func (UnimplementedEnginePluginServer) LeaveMember(context.Context, *LeaveMemberRequest) (*LeaveMemberResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method LeaveMember not implemented")
}
func (UnimplementedEnginePluginServer) ReadOnly(context.Context, *ReadOnlyRequest) (*ReadOnlyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadOnly not implemented")
}
func (UnimplementedEnginePluginServer) ReadWrite(context.Context, *ReadWriteRequest) (*ReadWriteResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadWrite not implemented")
}
func (UnimplementedEnginePluginServer) AccountProvision(context.Context, *AccountProvisionRequest) (*AccountProvisionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AccountProvision not implemented")
}
func (UnimplementedEnginePluginServer) Switchover(context.Context, *SwitchoverRequest) (*SwitchoverResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Switchover not implemented")
}
func (UnimplementedEnginePluginServer) mustEmbedUnimplementedEnginePluginServer() {}

// UnsafeEnginePluginServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to EnginePluginServer will
// result in compilation errors.
type UnsafeEnginePluginServer interface {
	mustEmbedUnimplementedEnginePluginServer()
}

func RegisterEnginePluginServer(s grpc.ServiceRegistrar, srv EnginePluginServer) {
	s.RegisterService(&EnginePlugin_ServiceDesc, srv)
}

func _EnginePlugin_GetPluginInfo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetPluginInfoRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EnginePluginServer).GetPluginInfo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/plugin.v1.EnginePlugin/GetPluginInfo",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EnginePluginServer).GetPluginInfo(ctx, req.(*GetPluginInfoRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EnginePlugin_IsEngineReady_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(IsEngineReadyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EnginePluginServer).IsEngineReady(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/plugin.v1.EnginePlugin/IsEngineReady",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EnginePluginServer).IsEngineReady(ctx, req.(*IsEngineReadyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EnginePlugin_GetRole_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetRoleRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EnginePluginServer).GetRole(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/plugin.v1.EnginePlugin/GetRole",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EnginePluginServer).GetRole(ctx, req.(*GetRoleRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EnginePlugin_JoinMember_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(JoinMemberRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EnginePluginServer).JoinMember(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/plugin.v1.EnginePlugin/JoinMember",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EnginePluginServer).JoinMember(ctx, req.(*JoinMemberRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EnginePlugin_LeaveMember_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LeaveMemberRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EnginePluginServer).LeaveMember(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/plugin.v1.EnginePlugin/LeaveMember",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EnginePluginServer).LeaveMember(ctx, req.(*LeaveMemberRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EnginePlugin_ReadOnly_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReadOnlyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EnginePluginServer).ReadOnly(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/plugin.v1.EnginePlugin/ReadOnly",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EnginePluginServer).ReadOnly(ctx, req.(*ReadOnlyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EnginePlugin_ReadWrite_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReadWriteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EnginePluginServer).ReadWrite(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/plugin.v1.EnginePlugin/ReadWrite",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EnginePluginServer).ReadWrite(ctx, req.(*ReadWriteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EnginePlugin_AccountProvision_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AccountProvisionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EnginePluginServer).AccountProvision(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/plugin.v1.EnginePlugin/AccountProvision",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EnginePluginServer).AccountProvision(ctx, req.(*AccountProvisionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EnginePlugin_Switchover_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SwitchoverRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EnginePluginServer).Switchover(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/plugin.v1.EnginePlugin/Switchover",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EnginePluginServer).Switchover(ctx, req.(*SwitchoverRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// EnginePlugin_ServiceDesc is the grpc.ServiceDesc for EnginePlugin service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var EnginePlugin_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "plugin.v1.EnginePlugin",
	HandlerType: (*EnginePluginServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetPluginInfo",
			Handler:    _EnginePlugin_GetPluginInfo_Handler,
		},
		{
			MethodName: "IsEngineReady",
			Handler:    _EnginePlugin_IsEngineReady_Handler,
		},
		{
			MethodName: "GetRole",
			Handler:    _EnginePlugin_GetRole_Handler,
		},
		{
			MethodName: "JoinMember",
			Handler:    _EnginePlugin_JoinMember_Handler,
		},
		{
			MethodName: "LeaveMember",
			Handler:    _EnginePlugin_LeaveMember_Handler,
		},
		{
			MethodName: "ReadOnly",
			Handler:    _EnginePlugin_ReadOnly_Handler,
		},
		{
			MethodName: "ReadWrite",
			Handler:    _EnginePlugin_ReadWrite_Handler,
		},
		{
			MethodName: "AccountProvision",
			Handler:    _EnginePlugin_AccountProvision_Handler,
		},
		{
			MethodName: "Switchover",
			Handler:    _EnginePlugin_Switchover_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "engine_plugin.proto",
}