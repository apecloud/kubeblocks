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
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type RefResourcesValidator struct {
	context.Context
	roclient.ReadonlyClient
}

func (t *RefResourcesValidator) Validate(dag *graph.DAG) error {
	rootVertex, e := findRootVertex(dag)
	if e != nil {
		return e
	}
	cluster, _ := rootVertex.obj.(*appsv1alpha1.Cluster)
	if isClusterDeleting(*cluster) {
		return nil
	}

	var err error
	defer setProvisioningStartedCondition(&cluster.Status.Conditions, cluster.Name, cluster.Generation, err)

	validateExistence := func(key client.ObjectKey, object client.Object) error {
		err := t.ReadonlyClient.Get(t.Context, key, object)
		if err != nil {
			return newRequeueError(requeueDuration, err.Error())
		}
		return nil
	}

	// validate cd & cv existences
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

	// validate cd & cv availability
	if cd.Status.Phase != appsv1alpha1.AvailablePhase || (cv != nil && cv.Status.Phase != appsv1alpha1.AvailablePhase) {
		message := fmt.Sprintf("ref resource is unavailable, this problem needs to be solved first. cd: %s", cd.Name)
		if cv != nil {
			message = fmt.Sprintf("%s, cv: %s", message, cv.Name)
		}
		err = errors.New(message)
		return newRequeueError(requeueDuration, message)
	}

	return nil
}

var _ graph.Validator = &RefResourcesValidator{}