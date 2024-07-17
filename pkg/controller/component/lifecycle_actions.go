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

package component

import (
	"context"
	"errors"
	"fmt"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	kbagentcli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var (
	ErrActionNotDefined     = errors.New("action is not defined")
	ErrActionNotImplemented = errors.New("action is not implemented")
	ErrActionInProgress     = errors.New("action is in progress")
	ErrActionBusy           = errors.New("action is busy")
	ErrActionTimeout        = errors.New("action timeout")
	ErrActionFailed         = errors.New("action failed")
	ErrActionCanceled       = errors.New("action canceled")
	ErrActionInternalError  = errors.New("action internal error")
)

type LifecycleActions struct {
	lifecycleActions *appsv1alpha1.ComponentLifecycleActions
	kbagent          kbagentcli.Client
	// jobExecutor      *jobExecutor
}

type LifecycleActionOptions struct {
	NonBlocking    *bool
	TimeoutSeconds *int32
	RetryPolicy    *appsv1alpha1.RetryPolicy
}

func (a *LifecycleActions) PostProvision(ctx context.Context, opts *LifecycleActionOptions) ([]byte, error) {
	action := &postProvision{}
	if a.lifecycleActions.PostProvision == nil || a.lifecycleActions.PostProvision.CustomHandler == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, a.lifecycleActions.PostProvision.CustomHandler, action, opts)
}

func (a *LifecycleActions) PreTerminate(ctx context.Context, opts *LifecycleActionOptions) ([]byte, error) {
	action := &preTerminate{}
	if a.lifecycleActions.PreTerminate == nil || a.lifecycleActions.PreTerminate.CustomHandler == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, a.lifecycleActions.PreTerminate.CustomHandler, action, opts)
}

// func (a *LifecycleActions) RoleProbe(ctx context.Context, opts *LifecycleActionOptions) ([]byte, error) {
//	return nil, nil
// }

func (a *LifecycleActions) Switchover(ctx context.Context, opts *LifecycleActionOptions) ([]byte, error) {
	action := &switchover{}
	if a.lifecycleActions.Switchover == nil || a.lifecycleActions.Switchover.WithoutCandidate == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, a.lifecycleActions.Switchover.WithoutCandidate, action, opts)
}

func (a *LifecycleActions) MemberJoin(ctx context.Context, opts *LifecycleActionOptions) ([]byte, error) {
	action := &memberJoin{}
	if a.lifecycleActions.MemberJoin == nil || a.lifecycleActions.MemberJoin.CustomHandler == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, a.lifecycleActions.MemberJoin.CustomHandler, action, opts)
}

func (a *LifecycleActions) MemberLeave(ctx context.Context, opts *LifecycleActionOptions) ([]byte, error) {
	action := &memberLeave{}
	if a.lifecycleActions.MemberLeave == nil || a.lifecycleActions.MemberLeave.CustomHandler == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, a.lifecycleActions.MemberLeave.CustomHandler, action, opts)
}

// func (a *LifecycleActions) Readonly(ctx context.Context, opts *LifecycleActionOptions) ([]byte, error) {
//	return nil, nil
// }
//
// func (a *LifecycleActions) Readwrite(ctx context.Context, opts *LifecycleActionOptions) ([]byte, error) {
//	return nil, nil
// }

func (a *LifecycleActions) DataDump(ctx context.Context, opts *LifecycleActionOptions) ([]byte, error) {
	action := &memberLeave{}
	if a.lifecycleActions.DataDump == nil || a.lifecycleActions.DataDump.CustomHandler == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, a.lifecycleActions.DataDump.CustomHandler, action, opts)
}

func (a *LifecycleActions) DataLoad(ctx context.Context, opts *LifecycleActionOptions) ([]byte, error) {
	action := &memberLeave{}
	if a.lifecycleActions.DataLoad == nil || a.lifecycleActions.DataLoad.CustomHandler == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, a.lifecycleActions.DataLoad.CustomHandler, action, opts)
}

// func (a *LifecycleActions) Reconfigure(ctx context.Context, opts *LifecycleActionOptions) ([]byte, error) {
//	return nil, nil
// }

func (a *LifecycleActions) AccountProvision(ctx context.Context, opts *LifecycleActionOptions) ([]byte, error) {
	action := &memberLeave{}
	if a.lifecycleActions.AccountProvision == nil || a.lifecycleActions.AccountProvision.CustomHandler == nil {
		return nil, fmt.Errorf("action %s is not defined", action.name())
	}
	return a.callAction(ctx, a.lifecycleActions.AccountProvision.CustomHandler, action, opts)
}

func (a *LifecycleActions) callAction(ctx context.Context,
	spec *appsv1alpha1.Action, action lifecycleAction, opts *LifecycleActionOptions) ([]byte, error) {
	if len(spec.Image) > 0 {
		return nil, fmt.Errorf("NotImplemented") // TODO: job executor
	}
	return a.callActionByKBAgent(ctx, spec, action, opts)
}

func (a *LifecycleActions) callActionByKBAgent(ctx context.Context,
	_ *appsv1alpha1.Action, action lifecycleAction, opts *LifecycleActionOptions) ([]byte, error) {
	req := a.buildActionRequest(action, opts)
	rsp, err := a.kbagent.CallAction(ctx, req)
	if err != nil {
		return nil, err
	}
	return rsp.Output, nil
}

func (a *LifecycleActions) buildActionRequest(action lifecycleAction, opts *LifecycleActionOptions) proto.ActionRequest {
	req := proto.ActionRequest{
		Action:     action.name(),
		Parameters: action.parameters(),
	}
	if opts != nil {
		if opts.NonBlocking != nil {
			req.NonBlocking = opts.NonBlocking
		}
		if opts.TimeoutSeconds != nil {
			req.TimeoutSeconds = opts.TimeoutSeconds
		}
		if opts.RetryPolicy != nil {
			req.RetryPolicy = &proto.RetryPolicy{
				MaxRetries:    opts.RetryPolicy.MaxRetries,
				RetryInterval: opts.RetryPolicy.RetryInterval,
			}
		}
	}
	return req
}

type lifecycleAction interface {
	name() string
	parameters() map[string]string
}

type postProvision struct{}

var _ lifecycleAction = &postProvision{}

func (a *postProvision) name() string {
	return "postProvision"
}

func (a *postProvision) parameters() map[string]string {
	// - KB_CLUSTER_POD_IP_LIST: Comma-separated list of the cluster's pod IP addresses (e.g., "podIp1,podIp2").
	// - KB_CLUSTER_POD_NAME_LIST: Comma-separated list of the cluster's pod names (e.g., "pod1,pod2").
	// - KB_CLUSTER_POD_HOST_NAME_LIST: Comma-separated list of host names, each corresponding to a pod in
	//   KB_CLUSTER_POD_NAME_LIST (e.g., "hostName1,hostName2").
	// - KB_CLUSTER_POD_HOST_IP_LIST: Comma-separated list of host IP addresses, each corresponding to a pod in
	//   KB_CLUSTER_POD_NAME_LIST (e.g., "hostIp1,hostIp2").
	//
	// - KB_CLUSTER_COMPONENT_POD_NAME_LIST: Comma-separated list of all pod names within the component
	//   (e.g., "pod1,pod2").
	// - KB_CLUSTER_COMPONENT_POD_IP_LIST: Comma-separated list of pod IP addresses,
	//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "podIp1,podIp2").
	// - KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST: Comma-separated list of host names for each pod,
	//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "hostName1,hostName2").
	// - KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST: Comma-separated list of host IP addresses for each pod,
	//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "hostIp1,hostIp2").
	//
	// - KB_CLUSTER_COMPONENT_LIST: Comma-separated list of all cluster components (e.g., "comp1,comp2").
	// - KB_CLUSTER_COMPONENT_DELETING_LIST: Comma-separated list of components that are currently being deleted
	//   (e.g., "comp1,comp2").
	// - KB_CLUSTER_COMPONENT_UNDELETED_LIST: Comma-separated list of components that are not being deleted
	//   (e.g., "comp1,comp2").
	m := make(map[string]string)
	m["KB_CLUSTER_POD_IP_LIST"] = ""
	m["KB_CLUSTER_POD_NAME_LIST"] = ""
	m["KB_CLUSTER_POD_HOST_NAME_LIST"] = ""
	m["KB_CLUSTER_POD_NAME_LIST"] = ""
	m["KB_CLUSTER_POD_HOST_IP_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_POD_NAME_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_POD_IP_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_DELETING_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_UNDELETED_LIST"] = ""
	return m
}

type preTerminate struct{}

var _ lifecycleAction = &preTerminate{}

func (a *preTerminate) name() string {
	return "preTerminate"
}

func (a *preTerminate) parameters() map[string]string {
	// - KB_CLUSTER_POD_IP_LIST: Comma-separated list of the cluster's pod IP addresses (e.g., "podIp1,podIp2").
	// - KB_CLUSTER_POD_NAME_LIST: Comma-separated list of the cluster's pod names (e.g., "pod1,pod2").
	// - KB_CLUSTER_POD_HOST_NAME_LIST: Comma-separated list of host names, each corresponding to a pod in
	//   KB_CLUSTER_POD_NAME_LIST (e.g., "hostName1,hostName2").
	// - KB_CLUSTER_POD_HOST_IP_LIST: Comma-separated list of host IP addresses, each corresponding to a pod in
	//   KB_CLUSTER_POD_NAME_LIST (e.g., "hostIp1,hostIp2").
	//
	// - KB_CLUSTER_COMPONENT_POD_NAME_LIST: Comma-separated list of all pod names within the component
	//   (e.g., "pod1,pod2").
	// - KB_CLUSTER_COMPONENT_POD_IP_LIST: Comma-separated list of pod IP addresses,
	//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "podIp1,podIp2").
	// - KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST: Comma-separated list of host names for each pod,
	//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "hostName1,hostName2").
	// - KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST: Comma-separated list of host IP addresses for each pod,
	//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "hostIp1,hostIp2").
	//
	// - KB_CLUSTER_COMPONENT_LIST: Comma-separated list of all cluster components (e.g., "comp1,comp2").
	// - KB_CLUSTER_COMPONENT_DELETING_LIST: Comma-separated list of components that are currently being deleted
	//   (e.g., "comp1,comp2").
	// - KB_CLUSTER_COMPONENT_UNDELETED_LIST: Comma-separated list of components that are not being deleted
	//   (e.g., "comp1,comp2").
	//
	// - KB_CLUSTER_COMPONENT_IS_SCALING_IN: Indicates whether the component is currently scaling in.
	//   If this variable is present and set to "true", it denotes that the component is undergoing a scale-in operation.
	//   During scale-in, data rebalancing is necessary to maintain cluster integrity.
	//   Contrast this with a cluster deletion scenario where data rebalancing is not required as the entire cluster
	//   is being cleaned up.
	m := make(map[string]string)
	m["KB_CLUSTER_POD_IP_LIST"] = ""
	m["KB_CLUSTER_POD_NAME_LIST"] = ""
	m["KB_CLUSTER_POD_HOST_NAME_LIST"] = ""
	m["KB_CLUSTER_POD_NAME_LIST"] = ""
	m["KB_CLUSTER_POD_HOST_IP_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_POD_NAME_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_POD_IP_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_DELETING_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_UNDELETED_LIST"] = ""
	m["KB_CLUSTER_COMPONENT_IS_SCALING_IN"] = ""
	return m
}

type switchover struct{}

var _ lifecycleAction = &switchover{}

func (a *switchover) name() string {
	return "switchover"
}

func (a *switchover) parameters() map[string]string {
	// - KB_SWITCHOVER_CANDIDATE_NAME: The name of the pod for the new leader candidate, which may not be specified (empty).
	// - KB_SWITCHOVER_CANDIDATE_FQDN: The FQDN of the new leader candidate's pod, which may not be specified (empty).
	// - KB_LEADER_POD_IP: The IP address of the current leader's pod prior to the switchover.
	// - KB_LEADER_POD_NAME: The name of the current leader's pod prior to the switchover.
	// - KB_LEADER_POD_FQDN: The FQDN of the current leader's pod prior to the switchover.
	m := make(map[string]string)
	m["KB_SWITCHOVER_CANDIDATE_NAME"] = ""
	m["KB_SWITCHOVER_CANDIDATE_FQDN"] = ""
	m["KB_LEADER_POD_IP"] = ""
	m["KB_LEADER_POD_NAME"] = ""
	m["KB_LEADER_POD_FQDN"] = ""
	return m
}

type memberJoin struct{}

var _ lifecycleAction = &memberJoin{}

func (a *memberJoin) name() string {
	return "memberJoin"
}

func (a *memberJoin) parameters() map[string]string {
	// The container executing this action has access to following environment variables:
	//
	// - KB_SERVICE_PORT: The port used by the database service.
	// - KB_SERVICE_USER: The username with the necessary permissions to interact with the database service.
	// - KB_SERVICE_PASSWORD: The corresponding password for KB_SERVICE_USER to authenticate with the database service.
	// - KB_PRIMARY_POD_FQDN: The FQDN of the primary Pod within the replication group.
	// - KB_MEMBER_ADDRESSES: A comma-separated list of Pod addresses for all replicas in the group.
	// - KB_NEW_MEMBER_POD_NAME: The pod name of the replica being added to the group.
	// - KB_NEW_MEMBER_POD_IP: The IP address of the replica being added to the group.
	//
	// Expected action output:
	// - On Failure: An error message detailing the reason for any failure encountered
	//   during the addition of the new member.
	m := make(map[string]string)
	m["KB_SERVICE_PORT"] = ""
	m["KB_SERVICE_USER"] = ""
	m["KB_SERVICE_PASSWORD"] = ""
	m["KB_PRIMARY_POD_FQDN"] = ""
	m["KB_MEMBER_ADDRESSES"] = ""
	m["KB_NEW_MEMBER_POD_NAME"] = ""
	m["KB_NEW_MEMBER_POD_IP"] = ""
	return m
}

type memberLeave struct{}

var _ lifecycleAction = &memberLeave{}

func (a *memberLeave) name() string {
	return "memberLeave"
}

func (a *memberLeave) parameters() map[string]string {
	// The container executing this action has access to following environment variables:
	//
	// - KB_SERVICE_PORT: The port used by the database service.
	// - KB_SERVICE_USER: The username with the necessary permissions to interact with the database service.
	// - KB_SERVICE_PASSWORD: The corresponding password for KB_SERVICE_USER to authenticate with the database service.
	// - KB_PRIMARY_POD_FQDN: The FQDN of the primary Pod within the replication group.
	// - KB_MEMBER_ADDRESSES: A comma-separated list of Pod addresses for all replicas in the group.
	// - KB_LEAVE_MEMBER_POD_NAME: The pod name of the replica being removed from the group.
	// - KB_LEAVE_MEMBER_POD_IP: The IP address of the replica being removed from the group.
	//
	// Expected action output:
	// - On Failure: An error message, if applicable, indicating why the action failed.
	m := make(map[string]string)
	m["KB_SERVICE_PORT"] = ""
	m["KB_SERVICE_USER"] = ""
	m["KB_SERVICE_PASSWORD"] = ""
	m["KB_PRIMARY_POD_FQDN"] = ""
	m["KB_MEMBER_ADDRESSES"] = ""
	m["KB_LEAVE_MEMBER_POD_NAME"] = ""
	m["KB_LEAVE_MEMBER_POD_IP"] = ""
	return m
}

type dataDump struct{}

var _ lifecycleAction = &dataDump{}

func (a *dataDump) name() string {
	return "dataDump"
}

func (a *dataDump) parameters() map[string]string {
	return nil
}

type dataLoad struct{}

var _ lifecycleAction = &dataLoad{}

func (a *dataLoad) name() string {
	return "dataLoad"
}

func (a *dataLoad) parameters() map[string]string {
	return nil
}

type accountProvision struct{}

var _ lifecycleAction = &accountProvision{}

func (a *accountProvision) name() string {
	return "accountProvision"
}

func (a *accountProvision) parameters() map[string]string {
	return nil
}
