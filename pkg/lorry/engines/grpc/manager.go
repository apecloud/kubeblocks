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
	"time"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/plugin"
	"github.com/apecloud/kubeblocks/pkg/viperx"
)

type Manager struct {
	engines.DBManagerBase

	dbClient plugin.ServicePluginClient
}

func NewManager(properties engines.Properties) (engines.DBManager, error) {
	logger := ctrl.Log.WithName("GRPC")
	managerBase, err := engines.NewDBManagerBase(logger)
	if err != nil {
		return nil, err
	}

	managerBase.DBStartupReady = false

	host := viperx.GetString(constant.KBEnvPodIP)
	if h, ok := properties["host"]; ok && h != "" {
		host = h
	}
	port, ok := properties["port"]
	if !ok || port == "" {
		return nil, errors.New("grpc port is not set")
	}
	dbClient, err := plugin.NewPluginClient(host, port)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create db plugin client")
	}

	mgr := &Manager{
		dbClient:      dbClient,
		DBManagerBase: *managerBase,
	}

	return mgr, nil
}

func (mgr *Manager) IsDBStartupReady() bool {
	if mgr.DBStartupReady {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := &plugin.IsServiceReadyRequest{}
	resp, err := mgr.dbClient.IsServiceReady(ctx, req)
	if err != nil {
		return false
	}
	if resp.Ready {
		mgr.DBStartupReady = true
	}
	return resp.Ready
}

// JoinCurrentMemberToCluster provides the following dedicated environment variables for the action:
//
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
// - KB_PRIMARY_POD_FQDN: The FQDN of the original primary Pod before switchover.
// - KB_MEMBER_ADDRESSES: The addresses of all members.
// - KB_NEW_MEMBER_POD_NAME: The name of the new member's Pod.
// - KB_NEW_MEMBER_POD_IP: The IP of the new member's Pod.
func (mgr *Manager) JoinCurrentMemberToCluster(ctx context.Context, cluster *dcs.Cluster) error {
	req := &plugin.JoinMemberRequest{
		ServiceInfo: &plugin.ServiceInfo{},
		NewMember:   mgr.CurrentMemberName,
		Members:     cluster.GetMemberAddrs(),
	}

	if cluster.Leader != nil && cluster.Leader.Name != "" {
		leaderMember := cluster.GetMemberWithName(cluster.Leader.Name)
		req.ServiceInfo.Fqdn = cluster.GetMemberAddr(*leaderMember)
	}

	member := cluster.GetMemberWithName(mgr.CurrentMemberName)
	if member != nil {
		req.NewMemberIp = member.PodIP
	}

	_, err := mgr.dbClient.JoinMember(ctx, req)
	return err
}

// LeaveMemberFromCluster provides the following dedicated environment variables for the action:
//
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
// - KB_PRIMARY_POD_FQDN: The FQDN of the original primary Pod before switchover.
// - KB_MEMBER_ADDRESSES: The addresses of all members.
// - KB_LEAVE_MEMBER_POD_NAME: The name of the leave member's Pod.
// - KB_LEAVE_MEMBER_POD_IP: The IP of the leave member's Pod.
func (mgr *Manager) LeaveMemberFromCluster(ctx context.Context, cluster *dcs.Cluster, memberName string) error {
	req := &plugin.LeaveMemberRequest{
		ServiceInfo: &plugin.ServiceInfo{},
		LeaveMember: memberName,
		Members:     cluster.GetMemberAddrs(),
	}

	if cluster.Leader != nil && cluster.Leader.Name != "" {
		leaderMember := cluster.GetMemberWithName(cluster.Leader.Name)
		req.ServiceInfo.Fqdn = cluster.GetMemberAddr(*leaderMember)
	}

	member := cluster.GetMemberWithName(memberName)
	if member != nil {
		req.LeaveMemberIp = member.PodIP
	}
	_, err := mgr.dbClient.LeaveMember(ctx, req)

	return err
}

// Lock provides the following dedicated environment variables for the action:
//
// - KB_POD_FQDN: The FQDN of the replica pod to check the role.
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
func (mgr *Manager) Lock(ctx context.Context, reason string) error {
	req := &plugin.ReadonlyRequest{
		ServiceInfo: &plugin.ServiceInfo{},
		Reason:      reason,
	}
	_, err := mgr.dbClient.Readonly(ctx, req)
	return err
}

// Unlock provides the following dedicated environment variables for the action:
//
// - KB_POD_FQDN: The FQDN of the replica pod to check the role.
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
func (mgr *Manager) Unlock(ctx context.Context) error {
	req := &plugin.ReadwriteRequest{
		ServiceInfo: &plugin.ServiceInfo{},
	}
	_, err := mgr.dbClient.Readwrite(ctx, req)
	return err
}
