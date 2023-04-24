/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package consensusset

import (
	"context"
	"github.com/apecloud/kubeblocks/internal/controller/graph"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
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
