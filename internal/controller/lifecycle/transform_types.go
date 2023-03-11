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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

const (
	// TODO: deduplicate
	dbClusterFinalizerName = "cluster.kubeblocks.io/finalizer"
)

type Action string

const (
	CREATE = Action("CREATE")
	UPDATE = Action("UPDATE")
	DELETE = Action("DELETE")
	STATUS = Action("STATUS")
)

type gvkName struct {
	gvk      schema.GroupVersionKind
	ns, name string
}

type compoundCluster struct {
	cluster *appsv1alpha1.Cluster
	cd      appsv1alpha1.ClusterDefinition
	cv      appsv1alpha1.ClusterVersion
}

type lifecycleVertex struct {
	obj       client.Object
	oriObj    client.Object
	immutable bool
	action    *Action
}

type clusterSnapshot map[gvkName]client.Object

type RequeueError interface {
	RequeueAfter() time.Duration
	Reason() string
}

type realRequeueError struct {
	reason       string
	requeueAfter time.Duration
}

func (r *realRequeueError) Error() string {
	return fmt.Sprintf("requeue after: %v as: %s", r.requeueAfter, r.reason)
}

func (r *realRequeueError) RequeueAfter() time.Duration {
	return r.requeueAfter
}

func (r *realRequeueError) Reason() string {
	return r.reason
}
