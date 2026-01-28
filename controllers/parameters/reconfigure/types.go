/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package reconfigure

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

const (
	StatusNone           string = "None"           // finished and quit
	StatusRetry          string = "Retry"          // running
	StatusFailed         string = "Failed"         // failed and exited
	StatusFailedAndRetry string = "FailedAndRetry" // failed but can be retried
)

type Status struct {
	Status        string
	ExpectedCount int32
	SucceedCount  int32
}

func makeStatus(status string, ops ...func(status *Status)) Status {
	ret := Status{
		Status:        status,
		ExpectedCount: core.Unconfirmed,
		SucceedCount:  core.Unconfirmed,
	}
	for _, o := range ops {
		o(&ret)
	}
	return ret
}

func withSucceed(succeedCount int32) func(status *Status) {
	return func(status *Status) {
		status.SucceedCount = succeedCount
	}
}

func withExpected(expectedCount int32) func(status *Status) {
	return func(status *Status) {
		status.ExpectedCount = expectedCount
	}
}

type Context struct {
	intctrlutil.RequestCtx
	Client client.Client

	ConfigTemplate appsv1.ComponentFileTemplate
	ConfigHash     *string // the hash of the new configuration content

	Cluster          *appsv1.Cluster
	ClusterComponent *appsv1.ClusterComponentSpec
	ITS              *workloads.InstanceSet // TODO: use cluster or component API?

	ConfigDescription *parametersv1alpha1.ComponentConfigDescription
	ParametersDef     *parametersv1alpha1.ParametersDefinitionSpec
	Patch             *core.ConfigPatchInfo
}

func (c *Context) getTargetConfigHash() *string {
	return c.ConfigHash
}

func (c *Context) getTargetReplicas() int {
	return int(c.ClusterComponent.Replicas)
}

var (
	policyMap = map[parametersv1alpha1.ReloadPolicy]func(Context) (Status, error){}
)

func registerPolicy(policy parametersv1alpha1.ReloadPolicy, fn func(Context) (Status, error)) {
	policyMap[policy] = fn
}

type Task struct {
	Policy parametersv1alpha1.ReloadPolicy
	Ctx    Context
}

func (t Task) Reconfigure() (Status, error) {
	if policy, ok := policyMap[t.Policy]; ok {
		return policy(t.Ctx)
	}
	return Status{}, fmt.Errorf("not support reload action[%s]", t.Policy)
}
