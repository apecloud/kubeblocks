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

package model

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO: copy from lifecycle.transform_types, should replace lifecycle's def

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

type Action string

const (
	CREATE = Action("CREATE")
	UPDATE = Action("UPDATE")
	DELETE = Action("DELETE")
	STATUS = Action("STATUS")
)

const (
	AppInstanceLabelKey            = "app.kubernetes.io/instance"
	KBManagedByKey                 = "apps.kubeblocks.io/managed-by"
	RoleLabelKey                   = "kubeblocks.io/role"
	ConsensusSetAccessModeLabelKey = "cs.apps.kubeblocks.io/access-mode"
)

// RequeueDuration default reconcile requeue after duration
var RequeueDuration = time.Millisecond * 100

type GVKName struct {
	gvk      schema.GroupVersionKind
	ns, name string
}

// ObjectVertex describes expected object spec and how to reach it
// obj always represents the expected part: new object in Create/Update action and old object in Delete action
// oriObj is set in Update action
// all transformers doing their object manipulation works on obj.spec
// the root vertex(i.e. the cluster vertex) will be treated specially:
// as all its meta, spec and status can be updated in one reconciliation loop
// Update is ignored when immutable=true
// orphan object will be force deleted when action is DELETE
type ObjectVertex struct {
	Obj       client.Object
	OriObj    client.Object
	Immutable bool
	IsOrphan  bool
	Action    *Action
}

func (v ObjectVertex) String() string {
	if v.Action == nil {
		return fmt.Sprintf("{obj:%T, name: %s, immutable: %v, orphan: %v, action: nil}",
			v.Obj, v.Obj.GetName(), v.Immutable, v.IsOrphan)
	}
	return fmt.Sprintf("{obj:%T, name: %s, immutable: %v, orphan: %v, action: %v}",
		v.Obj, v.Obj.GetName(), v.Immutable, v.IsOrphan, *v.Action)
}

type ObjectSnapshot map[GVKName]client.Object

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
