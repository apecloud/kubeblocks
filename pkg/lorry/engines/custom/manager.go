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

package custom

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

type Manager struct {
	engines.DBManagerBase

	// For InstanceSet Actions
	actionSvcPorts *[]int
	client         *http.Client

	// For ComponentDefinition Actions
	actionCommands map[string][]string
}

func NewManager(properties engines.Properties) (engines.DBManager, error) {
	logger := ctrl.Log.WithName("custom")

	managerBase, err := engines.NewDBManagerBase(logger)
	if err != nil {
		return nil, err
	}

	managerBase.DBStartupReady = true
	mgr := &Manager{
		actionSvcPorts: &[]int{},
		DBManagerBase:  *managerBase,
	}

	err = mgr.InitInstanceSetActions()
	if err != nil {
		mgr.Logger.Info("init InstanceSet commands failed", "error", err.Error())
		return nil, err
	}
	err = mgr.InitComponentDefinitionActions()
	if err != nil {
		mgr.Logger.Info("init component definition commands failed", "error", err.Error())
		return nil, err
	}
	return mgr, nil
}

func (mgr *Manager) InitInstanceSetActions() error {
	actionSvcList := viper.GetString("KB_RSM_ACTION_SVC_LIST")
	if actionSvcList == "" || actionSvcList == "null" {
		return nil
	}
	err := json.Unmarshal([]byte(actionSvcList), mgr.actionSvcPorts)
	if err != nil {
		return err
	}

	// See guidance on proper HTTP client settings here:
	// https://medium.com/@nate510/don-t-use-go-s-default-http-client-4804cb19f779
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	netTransport := &http.Transport{
		Dial:                dialer.Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	mgr.client = &http.Client{
		Timeout:   time.Second * 30,
		Transport: netTransport,
	}

	return nil
}

func (mgr *Manager) InitComponentDefinitionActions() error {
	actionJSON := viper.GetString(constant.KBEnvActionHandlers)
	if actionJSON != "" {
		var actionHandlers = map[string]util.Handlers{}
		err := json.Unmarshal([]byte(actionJSON), &actionHandlers)
		if err != nil {
			return err
		}

		for action, handlers := range actionHandlers {
			if len(handlers.Command) > 0 {
				mgr.actionCommands[action] = handlers.Command
			}
		}
	}
	return nil
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
	memberJoinCmd, ok := mgr.actionCommands[constant.MemberJoinAction]
	if !ok || len(memberJoinCmd) == 0 {
		// return errors.New("member join command is empty!")
		return nil
	}
	envs, err := util.GetGlobalSharedEnvs()
	if err != nil {
		return err
	}

	if cluster.Leader != nil && cluster.Leader.Name != "" {
		leaderMember := cluster.GetMemberWithName(cluster.Leader.Name)
		fqdn := cluster.GetMemberAddr(*leaderMember)
		envs = append(envs, "KB_PRIMARY_POD_FQDN"+"="+fqdn)
	}

	addrs := cluster.GetMemberAddrs()
	envs = append(envs, "KB_MEMBER_ADDRESSES"+"="+strings.Join(addrs, ","))
	envs = append(envs, "KB_NEW_MEMBER_POD_NAME"+"="+mgr.CurrentMemberName)
	member := cluster.GetMemberWithName(mgr.CurrentMemberName)
	if member != nil {
		envs = append(envs, "KB_NEW_MEMBER_POD_IP"+"="+member.PodIP)
	}
	output, err := util.ExecCommand(ctx, memberJoinCmd, envs)

	if output != "" {
		mgr.Logger.Info("member join", "output", output)
	}
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
	memberLeaveCmd, ok := mgr.actionCommands[constant.MemberLeaveAction]
	if !ok || len(memberLeaveCmd) == 0 {
		// return errors.New("member leave command is empty!")
		return nil
	}
	envs := os.Environ()
	if cluster.Leader != nil && cluster.Leader.Name != "" {
		leaderMember := cluster.GetMemberWithName(cluster.Leader.Name)
		fqdn := cluster.GetMemberAddr(*leaderMember)
		envs = append(envs, "KB_PRIMARY_POD_FQDN"+"="+fqdn)
	}

	addrs := cluster.GetMemberAddrs()
	envs = append(envs, "KB_MEMBER_ADDRESSES"+"="+strings.Join(addrs, ","))
	envs = append(envs, "KB_LEAVE_MEMBER_POD_NAME"+"="+memberName)
	member := cluster.GetMemberWithName(memberName)
	if member != nil {
		envs = append(envs, "KB_LEAVE_MEMBER_POD_IP"+"="+member.PodIP)
	}
	output, err := util.ExecCommand(ctx, memberLeaveCmd, envs)

	if output != "" {
		mgr.Logger.Info("member leave", "output", output)
	}
	return err
}

// CurrentMemberHealthCheck provides the following dedicated environment variables for the action:
//
// - KB_POD_FQDN: The FQDN of the replica pod to check the role.
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
func (mgr *Manager) CurrentMemberHealthCheck(ctx context.Context, cluster *dcs.Cluster) error {
	healthyCheckCmd, ok := mgr.actionCommands[constant.CheckHealthyAction]
	if !ok || len(healthyCheckCmd) == 0 {
		return errors.New("member healthyCheck command is empty!")
	}
	envs, err := util.GetGlobalSharedEnvs()
	if err != nil {
		return err
	}
	output, err := util.ExecCommand(ctx, healthyCheckCmd, envs)

	if output != "" {
		mgr.Logger.Info("member healthy check", "output", output)
	}
	return err
}

// Lock provides the following dedicated environment variables for the action:
//
// - KB_POD_FQDN: The FQDN of the replica pod to check the role.
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
func (mgr *Manager) Lock(ctx context.Context, reason string) error {
	readonlyCmd, ok := mgr.actionCommands[constant.ReadonlyAction]
	if !ok || len(readonlyCmd) == 0 {
		// return errors.New("member lock command is empty!")
		return nil
	}
	envs, err := util.GetGlobalSharedEnvs()
	if err != nil {
		return err
	}
	output, err := util.ExecCommand(ctx, readonlyCmd, envs)

	if output != "" {
		mgr.Logger.Info("member lock", "output", output)
	}
	return err
}

// Unlock provides the following dedicated environment variables for the action:
//
// - KB_POD_FQDN: The FQDN of the replica pod to check the role.
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
func (mgr *Manager) Unlock(ctx context.Context) error {
	readWriteCmd, ok := mgr.actionCommands[constant.ReadWriteAction]
	if !ok || len(readWriteCmd) == 0 {
		// return errors.New("member unlock command is empty!")
		return nil
	}
	envs, err := util.GetGlobalSharedEnvs()
	if err != nil {
		return err
	}
	output, err := util.ExecCommand(ctx, readWriteCmd, envs)

	if output != "" {
		mgr.Logger.Info("member unlock", "output", output)
	}
	return err
}

// PostProvision provides the following dedicated environment variables for the action:
//
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
// - KB_CLUSTER_COMPONENT_LIST: Lists all components in the cluster, joined by ',' (e.g., "comp1,comp2").
// - KB_CLUSTER_COMPONENT_POD_NAME_LIST: Lists all pod names in this component, joined by ',' (e.g., "pod1,pod2").
// - KB_CLUSTER_COMPONENT_POD_IP_LIST: Lists the IP addresses of each pod in this component, corresponding one-to-one with each pod in the KB_CLUSTER_COMPONENT_POD_NAME_LIST. Joined by ',' (e.g., "podIp1,podIp2").
// - KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST: Lists the host names where each pod resides in this component, corresponding one-to-one with each pod in the KB_CLUSTER_COMPONENT_POD_NAME_LIST. Joined by ',' (e.g., "hostName1,hostName2").
// - KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST: Lists the host IP addresses where each pod resides in this component, corresponding one-to-one with each pod in the KB_CLUSTER_COMPONENT_POD_NAME_LIST. Joined by ',' (e.g., "hostIp1,hostIp2").
func (mgr *Manager) PostProvision(ctx context.Context, componentNames, podNames, podIPs, podHostNames, podHostIPs string) error {
	postProvisionCmd, ok := mgr.actionCommands[constant.PostProvisionAction]
	if !ok || len(postProvisionCmd) == 0 {
		// return errors.New("component postprovision command is empty!")
		return nil
	}
	envs, err := util.GetGlobalSharedEnvs()
	if err != nil {
		return err
	}

	envs = append(envs, "KB_CLUSTER_COMPONENT_LIST"+"="+componentNames)
	envs = append(envs, "KB_CLUSTER_COMPONENT_POD_NAME_LIST"+"="+podNames)
	envs = append(envs, "KB_CLUSTER_COMPONENT_POD_IP_LIST"+"="+podIPs)
	envs = append(envs, "KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST"+"="+podHostNames)
	envs = append(envs, "KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST"+"="+podHostIPs)
	output, err := util.ExecCommand(ctx, postProvisionCmd, envs)

	if output != "" {
		mgr.Logger.Info("component postprovision", "output", output)
	}
	return err
}

// PreTerminate provides the following dedicated environment variables for the action:
//
// - KB_POD_FQDN: The FQDN of the replica pod to check the role.
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
func (mgr *Manager) PreTerminate(ctx context.Context) error {
	preTerminateCmd, ok := mgr.actionCommands[constant.PreTerminateAction]
	if !ok || len(preTerminateCmd) == 0 {
		// return errors.New("component preterminate command is empty!")
		return nil
	}
	envs, err := util.GetGlobalSharedEnvs()
	if err != nil {
		return err
	}
	output, err := util.ExecCommand(ctx, preTerminateCmd, envs)

	if output != "" {
		mgr.Logger.Info("component preterminate", "output", output)
	}
	return err
}
