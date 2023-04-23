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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// ValidateAndLoadRefResourcesTransformer handles referenced resources'(cd & cv) validation and load them into context
type ValidateAndLoadRefResourcesTransformer struct{}

func (t *ValidateAndLoadRefResourcesTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	if isClusterDeleting(*cluster) {
		return nil
	}

	var err error
	defer func() {
		setProvisioningStartedCondition(&cluster.Status.Conditions, cluster.Name, cluster.Generation, err)
	}()

	validateExistence := func(key client.ObjectKey, object client.Object) error {
		err = transCtx.Client.Get(transCtx.Context, key, object)
		if err != nil {
			return newRequeueError(requeueDuration, err.Error())
		}
		return nil
	}

	// validate cd & cv's existence
	// if we can't get the referenced cd & cv, set provisioning condition failed, and jump to plan.Execute()
	cd := &appsv1alpha1.ClusterDefinition{}
	if err = validateExistence(types.NamespacedName{Name: cluster.Spec.ClusterDefRef}, cd); err != nil {
		return err
	}
	var cv *appsv1alpha1.ClusterVersion
	if len(cluster.Spec.ClusterVersionRef) > 0 {
		cv = &appsv1alpha1.ClusterVersion{}
		if err = validateExistence(types.NamespacedName{Name: cluster.Spec.ClusterVersionRef}, cv); err != nil {
			return err
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
		return newRequeueError(requeueDuration, message)
	}

	// inject cd & cv into the shared ctx
	transCtx.ClusterDef = cd
	transCtx.ClusterVer = cv
	if cv == nil {
		transCtx.ClusterVer = &appsv1alpha1.ClusterVersion{}
	}

	return nil
}

var _ graph.Transformer = &ValidateAndLoadRefResourcesTransformer{}
