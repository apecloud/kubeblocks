/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package consensusset

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

const (
	kindConsensusSet = "StatefulReplicaSet"

	defaultPodName = "Unknown"

	csSetFinalizerName = "cs.workloads.kubeblocks.io/finalizer"

	jobHandledLabel             = "cs.workloads.kubeblocks.io/job-handled"
	jobTypeLabel                = "cs.workloads.kubeblocks.io/job-type"
	jobScenarioLabel            = "cs.workloads.kubeblocks.io/job-scenario"
	jobHandledTrue              = "true"
	jobHandledFalse             = "false"
	jobTypeSwitchover           = "switchover"
	jobTypeMemberJoinNotifying  = "member-join"
	jobTypeMemberLeaveNotifying = "member-leave"
	jobTypeLogSync              = "log-sync"
	jobTypePromote              = "promote"
	jobScenarioMembership       = "membership-reconfiguration"
	jobScenarioUpdate           = "pod-update"

	roleObservationName              = "role-observe"
	roleAgentVolumeName              = "role-agent"
	roleAgentInstallerName           = "role-agent-installer"
	roleAgentVolumeMountPath         = "/role-observation"
	roleAgentName                    = "agent"
	shell2httpImage                  = "msoap/shell2http:1.16.0"
	shell2httpBinaryPath             = "/app/shell2http"
	shell2httpServePath              = "/role"
	defaultRoleObservationImage      = "apecloud/kubeblocks-role-observation:latest"
	defaultRoleObservationDaemonPort = 3501
	roleObservationURIFormat         = "http://localhost:%s/getRole"
	defaultActionImage               = "busybox:latest"
	usernameCredentialVarName        = "KB_CONSENSUS_SET_USERNAME"
	passwordCredentialVarName        = "KB_CONSENSUS_SET_PASSWORD"
	servicePortVarName               = "KB_CONSENSUS_SET_SERVICE_PORT"
	actionSvcListVarName             = "KB_CONSENSUS_SET_ACTION_SVC_LIST"
	leaderHostVarName                = "KB_CONSENSUS_SET_LEADER_HOST"
	targetHostVarName                = "KB_CONSENSUS_SET_TARGET_HOST"
	roleObservationEventFieldPath    = "spec.containers{" + roleObservationName + "}"
	actionSvcPortBase                = int32(36500)
)

type CSSetTransformContext struct {
	context.Context
	Client roclient.ReadonlyClient
	record.EventRecorder
	logr.Logger
	CSSet     *workloads.StatefulReplicaSet
	OrigCSSet *workloads.StatefulReplicaSet
}

func (c *CSSetTransformContext) GetContext() context.Context {
	return c.Context
}

func (c *CSSetTransformContext) GetClient() roclient.ReadonlyClient {
	return c.Client
}

func (c *CSSetTransformContext) GetRecorder() record.EventRecorder {
	return c.EventRecorder
}

func (c *CSSetTransformContext) GetLogger() logr.Logger {
	return c.Logger
}

var _ graph.TransformContext = &CSSetTransformContext{}
