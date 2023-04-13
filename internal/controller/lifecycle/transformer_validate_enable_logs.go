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

package lifecycle

import (
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// ValidateEnableLogsTransformer validate config and send warning event log necessarily
type ValidateEnableLogsTransformer struct{}

func (e *ValidateEnableLogsTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	if isClusterDeleting(*cluster) {
		return nil
	}

	// validate config and send warning event log necessarily
	err := cluster.Spec.ValidateEnabledLogs(transCtx.ClusterDef)
	setProvisioningStartedCondition(&cluster.Status.Conditions, cluster.Name, cluster.Generation, err)
	if err != nil {
		return graph.ErrFastReturn
	}

	return nil
}

var _ graph.Transformer = &ValidateEnableLogsTransformer{}
