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
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/dcs"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/plugin"
)

var _ = Describe("GRPC Handler", func() {
	var (
		handler *Handler
		cluster *dcs.Cluster
		ctx     context.Context
	)

	BeforeEach(func() {
		handler = &Handler{}
		cluster = &dcs.Cluster{}
		ctx = context.Background()
	})

	Describe("GetReplicaRole", func() {
		It("get role successfully", func() {
			expectedRole := "primary"

			getRoleRequest := &plugin.GetRoleRequest{
				ServiceInfo: plugin.GetServiceInfo(),
			}
			getRoleResponse := &plugin.GetRoleResponse{
				Role: expectedRole,
			}
			mockGetRoleFunc := func(ctx context.Context, req *plugin.GetRoleRequest) (*plugin.GetRoleResponse, error) {
				Expect(req).To(Equal(getRoleRequest))
				return getRoleResponse, nil
			}
			handler.dbClient = &mockServicePluginClient{
				mockGetRole: mockGetRoleFunc,
			}

			Expect(handler.GetReplicaRole(ctx, cluster)).To(Equal(expectedRole))
		})

		It("get role error", func() {
			expectedError := errors.New("failed to get role")
			getRoleRequest := &plugin.GetRoleRequest{
				ServiceInfo: plugin.GetServiceInfo(),
			}
			mockGetRoleFunc := func(ctx context.Context, req *plugin.GetRoleRequest) (*plugin.GetRoleResponse, error) {
				Expect(req).To(Equal(getRoleRequest))
				return nil, expectedError
			}
			handler.dbClient = &mockServicePluginClient{
				mockGetRole: mockGetRoleFunc,
			}
			role, err := handler.GetReplicaRole(ctx, cluster)
			Expect(err).To(Equal(expectedError))
			Expect(role).To(BeZero())
		})
	})
})
