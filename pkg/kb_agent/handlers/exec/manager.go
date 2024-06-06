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

package exec

import (
	"context"
	"encoding/json"
	"os"

	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type Handler struct {
	handlers.HandlerBase

	Executor util.Executor
	// For ComponentDefinition Actions
	actionCommands map[string][]string
}

func NewHandler(properties map[string]string) (handlers.Handler, error) {
	logger := ctrl.Log.WithName("exec handler")

	handlerBase, err := handlers.NewHandlerBase(logger)
	if err != nil {
		return nil, err
	}

	handlerBase.DBStartupReady = true
	h := &Handler{
		HandlerBase:    *handlerBase,
		Executor:       &util.ExecutorImpl{},
		actionCommands: map[string][]string{},
	}

	err = h.InitComponentDefinitionActions()
	if err != nil {
		h.Logger.Info("init component definition commands failed", "error", err.Error())
		return nil, err
	}
	return h, nil
}

func (h *Handler) InitComponentDefinitionActions() error {
	actionJSON := viper.GetString(constant.KBEnvActionHandlers)
	if actionJSON != "" {
		var actionHandlers = map[string]util.Handlers{}
		err := json.Unmarshal([]byte(actionJSON), &actionHandlers)
		if err != nil {
			return err
		}

		for action, handlers := range actionHandlers {
			if len(handlers.Command) > 0 {
				h.actionCommands[action] = handlers.Command
			}
		}
	}
	return nil
}

// JoinMember provides the following dedicated environment variables for the action:
//
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
// - KB_PRIMARY_POD_FQDN: The FQDN of the original primary Pod before switchover.
// - KB_MEMBER_ADDRESSES: The addresses of all members.
// - KB_NEW_MEMBER_POD_NAME: The name of the new member's Pod.
// - KB_NEW_MEMBER_POD_IP: The name of the new member's Pod.
func (h *Handler) JoinMember(ctx context.Context, primary string) error {
	memberJoinCmd, ok := h.actionCommands[constant.MemberJoinAction]
	if !ok || len(memberJoinCmd) == 0 {
		h.Logger.Info("member join command is empty!")
		return nil
	}
	envs, err := util.GetGlobalSharedEnvs()
	if err != nil {
		return err
	}

	envs = append(envs, "KB_PRIMARY_POD_FQDN"+"="+primary)

	output, err := h.Executor.ExecCommand(ctx, memberJoinCmd, envs)

	if output != "" {
		h.Logger.Info("member join", "output", output)
	}
	return err
}

// LeaveMember provides the following dedicated environment variables for the action:
//
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
// - KB_PRIMARY_POD_FQDN: The FQDN of the original primary Pod before switchover.
// - KB_MEMBER_ADDRESSES: The addresses of all members.
// - KB_LEAVE_MEMBER_POD_NAME: The name of the leave member's Pod.
// - KB_LEAVE_MEMBER_POD_IP: The IP of the leave member's Pod.
func (h *Handler) LeaveMember(ctx context.Context, primary string) error {
	memberLeaveCmd, ok := h.actionCommands[constant.MemberLeaveAction]
	if !ok || len(memberLeaveCmd) == 0 {
		h.Logger.Info("member leave command is empty!")
		return nil
	}
	envs := os.Environ()
	envs = append(envs, "KB_PRIMARY_POD_FQDN"+"="+primary)

	output, err := h.Executor.ExecCommand(ctx, memberLeaveCmd, envs)

	if output != "" {
		h.Logger.Info("member leave", "output", output)
	}
	return err
}

// HealthyCheck provides the following dedicated environment variables for the action:
//
// - KB_POD_FQDN: The FQDN of the replica pod to check the role.
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
func (h *Handler) HealthyCheck(ctx context.Context) error {
	healthyCheckCmd, ok := h.actionCommands[constant.CheckHealthyAction]
	if !ok || len(healthyCheckCmd) == 0 {
		h.Logger.Info("member healthyCheck command is empty!")
		return nil
	}
	envs, err := util.GetGlobalSharedEnvs()
	if err != nil {
		return err
	}
	output, err := h.Executor.ExecCommand(ctx, healthyCheckCmd, envs)

	if output != "" {
		h.Logger.Info("member healthy check", "output", output)
	}
	return err
}

// ReadOnly provides the following dedicated environment variables for the action:
//
// - KB_POD_FQDN: The FQDN of the replica pod to check the role.
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
func (h *Handler) ReadOnly(ctx context.Context, reason string) error {
	readonlyCmd, ok := h.actionCommands[constant.ReadonlyAction]
	if !ok || len(readonlyCmd) == 0 {
		h.Logger.Info("member ReadOnly command is empty!")
		return nil
	}
	envs, err := util.GetGlobalSharedEnvs()
	if err != nil {
		return err
	}
	output, err := h.Executor.ExecCommand(ctx, readonlyCmd, envs)

	if output != "" {
		h.Logger.Info("member ReadOnly", "output", output)
	}
	return err
}

// ReadWrite provides the following dedicated environment variables for the action:
//
// - KB_POD_FQDN: The FQDN of the replica pod to check the role.
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
func (h *Handler) ReadWrite(ctx context.Context, reason string) error {
	readWriteCmd, ok := h.actionCommands[constant.ReadWriteAction]
	if !ok || len(readWriteCmd) == 0 {
		h.Logger.Info("member ReadWrite command is empty!")
		return nil
	}
	envs, err := util.GetGlobalSharedEnvs()
	if err != nil {
		return err
	}
	output, err := h.Executor.ExecCommand(ctx, readWriteCmd, envs)

	if output != "" {
		h.Logger.Info("member ReadWrite", "output", output)
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
func (h *Handler) PostProvision(ctx context.Context) error {
	postProvisionCmd, ok := h.actionCommands[constant.PostProvisionAction]
	if !ok || len(postProvisionCmd) == 0 {
		h.Logger.Info("component postprovision command is empty!")
		return nil
	}
	envs, err := util.GetGlobalSharedEnvs()
	if err != nil {
		return err
	}

	// envs = append(envs, "KB_CLUSTER_COMPONENT_LIST"+"="+componentNames)
	// envs = append(envs, "KB_CLUSTER_COMPONENT_POD_NAME_LIST"+"="+podNames)
	// envs = append(envs, "KB_CLUSTER_COMPONENT_POD_IP_LIST"+"="+podIPs)
	// envs = append(envs, "KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST"+"="+podHostNames)
	// envs = append(envs, "KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST"+"="+podHostIPs)
	output, err := h.Executor.ExecCommand(ctx, postProvisionCmd, envs)

	if output != "" {
		h.Logger.Info("component postprovision", "output", output)
	}
	return err
}

// PreTerminate provides the following dedicated environment variables for the action:
//
// - KB_POD_FQDN: The FQDN of the replica pod to check the role.
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
func (h *Handler) PreTerminate(ctx context.Context) error {
	preTerminateCmd, ok := h.actionCommands[constant.PreTerminateAction]
	if !ok || len(preTerminateCmd) == 0 {
		h.Logger.Info("component preterminate command is empty!")
		return nil
	}
	envs, err := util.GetGlobalSharedEnvs()
	if err != nil {
		return err
	}
	output, err := h.Executor.ExecCommand(ctx, preTerminateCmd, envs)

	if output != "" {
		h.Logger.Info("component preterminate", "output", output)
	}
	return err
}
