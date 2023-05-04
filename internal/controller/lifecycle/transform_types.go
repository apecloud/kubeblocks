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

// default reconcile requeue after duration
var requeueDuration = time.Millisecond * 100

type gvkName struct {
	gvk      schema.GroupVersionKind
	ns, name string
}

// lifecycleVertex describes expected object spec and how to reach it
// obj always represents the expected part: new object in Create/Update action and old object in Delete action
// oriObj is set in Update action
// all transformers doing their object manipulation works on obj.spec
// the root vertex(i.e. the cluster vertex) will be treated specially:
// as all its meta, spec and status can be updated in one reconciliation loop
// Update is ignored when immutable=true
// orphan object will be force deleted when action is DELETE
type lifecycleVertex struct {
	obj       client.Object
	oriObj    client.Object
	immutable bool
	isOrphan  bool
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

// IsRequeueError checks if the error is a RequeueError
func IsRequeueError(err error) bool {
	_, ok := err.(RequeueError)
	return ok
}

type delegateClient struct {
	client.Client
}

var _ client2.ReadonlyClient = delegateClient{}
var _ RequeueError = &realRequeueError{}
