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
	ConsensusSetKind = "ConsensusSet"

	DefaultPodName = "Unknown"

	CSSetFinalizerName = "cs.workloads.kubeblocks.io/finalizer"
)

type CSSetTransformContext struct {
	context.Context
	Client roclient.ReadonlyClient
	record.EventRecorder
	logr.Logger
	CSSet     *workloads.ConsensusSet
	OrigCSSet *workloads.ConsensusSet
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
