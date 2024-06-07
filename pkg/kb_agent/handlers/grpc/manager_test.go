/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package grpc

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/plugin"
	"github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("GRPC Handler", func() {
	var (
		handler *Handler
		ctx     context.Context
	)

	BeforeEach(func() {
		handler = &Handler{}
		ctx = context.Background()
	})

	Describe("NewHandler", func() {
		Context("when podname is empty", func() {
			properties := map[string]string{
				"host": "localhost",
				"port": "50051",
			}

			handler, err := NewHandler(properties)

			It("should return a nil and error", func() {
				Expect(handler).To(BeNil())
				Expect(err).ToNot(Succeed())
			})
		})
		Context("when properties is empty", func() {
			viperx.Set(constant.KBEnvPodName, "test-pod-0")
			properties := map[string]string{
				"host": "localhost",
				"port": "50051",
			}

			handler, err := NewHandler(properties)

			It("should return a handler and no error", func() {
				Expect(handler).ToNot(BeNil())
				Expect(err).To(Succeed())
			})
		})
	})

	Describe("IsDBStartupReady", func() {
		Context("when DBStartupReady is true", func() {
			BeforeEach(func() {
				handler.DBStartupReady = true
			})

			It("should return true", func() {
				Expect(handler.IsDBStartupReady()).To(BeTrue())
			})
		})

		Context("when DBStartupReady is false", func() {
			BeforeEach(func() {
				handler.DBStartupReady = false
			})

			It("should return the response from dbClient", func() {
				mockIsEngineReady := &plugin.IsEngineReadyResponse{
					Ready: true,
				}
				mockIsEngineReadyFunc := func(ctx context.Context, req *plugin.IsEngineReadyRequest) (*plugin.IsEngineReadyResponse, error) {
					return mockIsEngineReady, nil
				}
				handler.dbClient = &mockEnginePluginClient{
					mockIsEngineReady: mockIsEngineReadyFunc,
				}

				Expect(handler.IsDBStartupReady()).To(BeTrue())
			})
		})
	})

	Describe("JoinMember", func() {
		It("should call dbClient.JoinMember with the correct request", func() {
			primaryFQDN := "test-pod-0"
			mockJoinMember := &plugin.JoinMemberRequest{
				EngineInfo: &plugin.EngineInfo{
					Fqdn: primaryFQDN,
				},
				NewMember: handler.CurrentMemberName,
			}
			mockJoinMemberFunc := func(ctx context.Context, req *plugin.JoinMemberRequest) (*plugin.JoinMemberResponse, error) {
				Expect(req).To(Equal(mockJoinMember))
				return nil, nil
			}
			handler.dbClient = &mockEnginePluginClient{
				mockJoinMember: mockJoinMemberFunc,
			}

			Expect(handler.JoinMember(ctx, primaryFQDN)).To(Succeed())
		})
	})

	Describe("LeaveMember", func() {
		It("should call dbClient.LeaveMember with the correct request", func() {
			primaryFQDN := "test-pod-0"
			mockLeaveMember := &plugin.LeaveMemberRequest{
				EngineInfo: &plugin.EngineInfo{
					Fqdn: primaryFQDN,
				},
			}
			mockLeaveMemberFunc := func(ctx context.Context, req *plugin.LeaveMemberRequest) (*plugin.LeaveMemberResponse, error) {
				Expect(req).To(Equal(mockLeaveMember))
				return nil, nil
			}
			handler.dbClient = &mockEnginePluginClient{
				mockLeaveMember: mockLeaveMemberFunc,
			}

			Expect(handler.LeaveMember(ctx, primaryFQDN)).To(Succeed())
		})
	})

	Describe("ReadOnly", func() {
		It("should call dbClient.Readonly with the correct request", func() {
			reason := "reason for test ReadOnly"
			mockReadonly := &plugin.ReadOnlyRequest{
				EngineInfo: &plugin.EngineInfo{},
				Reason:     reason,
			}
			mockReadonlyFunc := func(ctx context.Context, req *plugin.ReadOnlyRequest) (*plugin.ReadOnlyResponse, error) {
				Expect(req).To(Equal(mockReadonly))
				return nil, nil
			}
			handler.dbClient = &mockEnginePluginClient{
				mockReadonly: mockReadonlyFunc,
			}

			Expect(handler.ReadOnly(ctx, reason)).To(Succeed())
		})
	})

	Describe("ReadWrite", func() {
		It("should call dbClient.Readwrite with the correct request", func() {
			reason := "reason for test ReadWrite"
			mockReadwrite := &plugin.ReadWriteRequest{
				EngineInfo: &plugin.EngineInfo{},
				Reason:     reason,
			}
			mockReadwriteFunc := func(ctx context.Context, req *plugin.ReadWriteRequest) (*plugin.ReadWriteResponse, error) {
				Expect(req).To(Equal(mockReadwrite))
				return nil, nil
			}
			handler.dbClient = &mockEnginePluginClient{
				mockReadwrite: mockReadwriteFunc,
			}

			Expect(handler.ReadWrite(ctx, reason)).To(Succeed())
		})
	})
})

type mockEnginePluginClient struct {
	plugin.EnginePluginClient

	mockGetRole       func(ctx context.Context, req *plugin.GetRoleRequest) (*plugin.GetRoleResponse, error)
	mockIsEngineReady func(ctx context.Context, req *plugin.IsEngineReadyRequest) (*plugin.IsEngineReadyResponse, error)
	mockJoinMember    func(ctx context.Context, req *plugin.JoinMemberRequest) (*plugin.JoinMemberResponse, error)
	mockLeaveMember   func(ctx context.Context, req *plugin.LeaveMemberRequest) (*plugin.LeaveMemberResponse, error)
	mockReadonly      func(ctx context.Context, req *plugin.ReadOnlyRequest) (*plugin.ReadOnlyResponse, error)
	mockReadwrite     func(ctx context.Context, req *plugin.ReadWriteRequest) (*plugin.ReadWriteResponse, error)
}

func (m *mockEnginePluginClient) GetRole(ctx context.Context, req *plugin.GetRoleRequest, opts ...grpc.CallOption) (*plugin.GetRoleResponse, error) {
	if m.mockGetRole != nil {
		return m.mockGetRole(ctx, req)
	}
	return nil, nil
}

func (m *mockEnginePluginClient) IsEngineReady(ctx context.Context, req *plugin.IsEngineReadyRequest, opts ...grpc.CallOption) (*plugin.IsEngineReadyResponse, error) {
	if m.mockIsEngineReady != nil {
		return m.mockIsEngineReady(ctx, req)
	}
	return nil, nil
}

func (m *mockEnginePluginClient) JoinMember(ctx context.Context, req *plugin.JoinMemberRequest, opts ...grpc.CallOption) (*plugin.JoinMemberResponse, error) {
	if m.mockJoinMember != nil {
		return m.mockJoinMember(ctx, req)
	}
	return nil, nil
}

func (m *mockEnginePluginClient) LeaveMember(ctx context.Context, req *plugin.LeaveMemberRequest, opts ...grpc.CallOption) (*plugin.LeaveMemberResponse, error) {
	if m.mockLeaveMember != nil {
		return m.mockLeaveMember(ctx, req)
	}
	return nil, nil
}

func (m *mockEnginePluginClient) ReadOnly(ctx context.Context, req *plugin.ReadOnlyRequest, opts ...grpc.CallOption) (*plugin.ReadOnlyResponse, error) {
	if m.mockReadonly != nil {
		return m.mockReadonly(ctx, req)
	}
	return nil, nil
}

func (m *mockEnginePluginClient) ReadWrite(ctx context.Context, req *plugin.ReadWriteRequest, opts ...grpc.CallOption) (*plugin.ReadWriteResponse, error) {
	if m.mockReadwrite != nil {
		return m.mockReadwrite(ctx, req)
	}
	return nil, nil
}
