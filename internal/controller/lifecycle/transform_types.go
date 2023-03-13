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
	"github.com/apecloud/kubeblocks/internal/controller/types"
)

const (
	// TODO: deduplicate
	dbClusterFinalizerName = "cluster.kubeblocks.io/finalizer"
	clusterDefLabelKey     = "clusterdefinition.kubeblocks.io/name"
	clusterVersionLabelKey = "clusterversion.kubeblocks.io/name"
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

// lifecycleVertex describes expected object spec and how to reach it
// obj always represents the expected port: new object in Create/Update action and old object in Delete action
// oriObj is set in Update action
// all transformers doing their object manipulation works on obj.spec
// the root vertex(i.e. the cluster vertex) will be treated specially:
// as all its meta, spec and status can be updated in one reconciliation loop
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

// TODO: dedup
// postHandler defines the handler after patching cluster status.
type postHandler func(cluster *appsv1alpha1.Cluster) error

// TODO: dedup
// clusterStatusHandler a cluster status handler which changes of Cluster.status will be patched uniformly by doChainClusterStatusHandler.
type clusterStatusHandler func(cluster *appsv1alpha1.Cluster) (bool, postHandler)

type delegateClient struct {
	client.Client
}

var _ types.ReadonlyClient = delegateClient{}
var _ RequeueError = &realRequeueError{}
