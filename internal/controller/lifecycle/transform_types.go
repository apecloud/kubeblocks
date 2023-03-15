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

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	client2 "github.com/apecloud/kubeblocks/internal/controller/client"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(appsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(dataprotectionv1alpha1.AddToScheme(scheme))
	utilruntime.Must(snapshotv1.AddToScheme(scheme))
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
}

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

type clusterRefResources struct {
	cd appsv1alpha1.ClusterDefinition
	cv appsv1alpha1.ClusterVersion
}

// lifecycleVertex describes expected object spec and how to reach it
// obj always represents the expected part: new object in Create/Update action and old object in Delete action
// oriObj is set in Update action
// all transformers doing their object manipulation works on obj.spec
// the root vertex(i.e. the cluster vertex) will be treated specially:
// as all its meta, spec and status can be updated in one reconciliation loop
// Update is ignored when immutable=true
type lifecycleVertex struct {
	obj       client.Object
	oriObj    client.Object
	immutable bool
	action    *Action
}

func (v lifecycleVertex) String() string {
	if v.action == nil {
		return fmt.Sprintf("{obj:%T, immutable: %v, action: nil}", v.obj, v.immutable)
	}
	return fmt.Sprintf("{obj:%T, immutable: %v, action: %v}", v.obj, v.immutable, *v.action)
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

var _ client2.ReadonlyClient = delegateClient{}
var _ RequeueError = &realRequeueError{}
