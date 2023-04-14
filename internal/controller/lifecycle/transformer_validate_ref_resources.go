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
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// ValidateRefResourcesTransformer handles referenced resources'(cd & cv) validation
type ValidateRefResourcesTransformer struct{}

func (t *ValidateRefResourcesTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	if isClusterDeleting(*cluster) {
		return nil
	}

	var err error
	defer func() {
		setProvisioningStartedCondition(&cluster.Status.Conditions, cluster.Name, cluster.Generation, err)
	}()

	// validate cd & cv's existence
	// if we can't get the referenced cd & cv, set provisioning condition failed, and jump to plan.Execute()
	cd := &appsv1alpha1.ClusterDefinition{}
	if err = transCtx.Client.Get(transCtx.Context, types.NamespacedName{Name: cluster.Spec.ClusterDefRef}, cd); err != nil {
		return graph.ErrFastReturn
	}
	var cv *appsv1alpha1.ClusterVersion
	if len(cluster.Spec.ClusterVersionRef) > 0 {
		cv = &appsv1alpha1.ClusterVersion{}
		if err = transCtx.Client.Get(transCtx.Context, types.NamespacedName{Name: cluster.Spec.ClusterVersionRef}, cv); err != nil {
			return graph.ErrFastReturn
		}
	}

	// validate cd & cv's availability
	// if wrong phase, set provisioning condition failed, and jump to plan.Execute()
	if cd.Status.Phase != appsv1alpha1.AvailablePhase || (cv != nil && cv.Status.Phase != appsv1alpha1.AvailablePhase) {
		message := fmt.Sprintf("ref resource is unavailable, this problem needs to be solved first. cd: %s", cd.Name)
		if cv != nil {
			message = fmt.Sprintf("%s, cv: %s", message, cv.Name)
		}
		err = errors.New(message)
		transCtx.Logger.Info(fmt.Sprintf("validation error: %v", err))
		return graph.ErrFastReturn
	}

	return nil
}

var _ graph.Transformer = &ValidateRefResourcesTransformer{}
