/*
Copyright ApeCloud Inc.

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

package controllerutil

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	AppName = "kubeblocks"
	// common label and annotation keys

	AppInstanceEnvLabelKey         = "app.kubernetes.io/ins-env"
	AppInstanceLabelKey            = "app.kubernetes.io/instance"
	AppComponentLabelKey           = "app.kubernetes.io/component-name"
	AppNameLabelKey                = "app.kubernetes.io/name"
	AppManagedByLabelKey           = "app.kubernetes.io/managed-by"
	AppConfigTypeLabelKey          = "app.kubernetes.io/config-type"
	ConsensusSetRoleLabelKey       = "cs.dbaas.kubeblocks.io/role"
	ConsensusSetAccessModeLabelKey = "cs.dbaas.kubeblocks.io/access-mode"

	// OpsRequestAnnotationKey OpsRequest annotation key in Cluster
	OpsRequestAnnotationKey = "kubeblocks.io/ops-request"
	// StorageClassAnnotationKey StorageClass annotation key in Cluster
	StorageClassAnnotationKey = "kubeblocks.io/storage-class"
)

const (
	// EventReasonNotFoundCR referenced custom resource not found
	EventReasonNotFoundCR = "NotFoundCR"
	// EventReasonRefCRUnavailable  referenced custom resource is unavailable
	EventReasonRefCRUnavailable = "ReferencedCRUnavailable"
	// EventReasonDeletedCR deleted custom resource
	EventReasonDeletedCR = "DeletedCR"
	// EventReasonDeletingCR deleting custom resource
	EventReasonDeletingCR = "DeletingCR"
	// EventReasonCreatedCR created custom resource
	EventReasonCreatedCR = "CreatedCR"
	// EventReasonRunTaskFailed run task failed
	EventReasonRunTaskFailed = "RunTaskFailed"
)

// RequestCtx wrapper for reconcile procedure context parameters
type RequestCtx struct {
	Ctx      context.Context
	Req      ctrl.Request
	Log      logr.Logger
	Recorder record.EventRecorder
}
