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

package statefulreplicaset

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

const (
	kindStatefulReplicaSet = "StatefulReplicaSet"

	roleLabelKey          = "kubeblocks.io/role"
	srsAccessModeLabelKey = "srs.apps.kubeblocks.io/access-mode"

	defaultPodName = "Unknown"

	srsFinalizerName = "srs.workloads.kubeblocks.io/finalizer"

	jobHandledLabel             = "srs.workloads.kubeblocks.io/job-handled"
	jobTypeLabel                = "srs.workloads.kubeblocks.io/job-type"
	jobScenarioLabel            = "srs.workloads.kubeblocks.io/job-scenario"
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
	usernameCredentialVarName        = "KB_SRS_USERNAME"
	passwordCredentialVarName        = "KB_SRS_PASSWORD"
	servicePortVarName               = "KB_SRS_SERVICE_PORT"
	actionSvcListVarName             = "KB_SRS_ACTION_SVC_LIST"
	leaderHostVarName                = "KB_SRS_LEADER_HOST"
	targetHostVarName                = "KB_SRS_TARGET_HOST"
	roleObservationEventFieldPath    = "spec.containers{" + roleObservationName + "}"
	actionSvcPortBase                = int32(36500)
)

type SRSTransformContext struct {
	context.Context
	Client roclient.ReadonlyClient
	record.EventRecorder
	logr.Logger
	srs     *workloads.StatefulReplicaSet
	srsOrig *workloads.StatefulReplicaSet
}

func (c *SRSTransformContext) GetContext() context.Context {
	return c.Context
}

func (c *SRSTransformContext) GetClient() roclient.ReadonlyClient {
	return c.Client
}

func (c *SRSTransformContext) GetRecorder() record.EventRecorder {
	return c.EventRecorder
}

func (c *SRSTransformContext) GetLogger() logr.Logger {
	return c.Logger
}

var _ graph.TransformContext = &SRSTransformContext{}
