/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package instanceset

const (
	WorkloadsManagedByLabelKey = "workloads.kubeblocks.io/managed-by"
	WorkloadsInstanceLabelKey  = "workloads.kubeblocks.io/instance"

	RoleLabelKey       = "kubeblocks.io/role"
	AccessModeLabelKey = "workloads.kubeblocks.io/access-mode"

	LegacyRSMFinalizerName = "rsm.workloads.kubeblocks.io/finalizer"

	roleProbeContainerName       = "kb-role-probe"
	usernameCredentialVarName    = "KB_RSM_USERNAME"
	passwordCredentialVarName    = "KB_RSM_PASSWORD"
	servicePortVarName           = "KB_RSM_SERVICE_PORT"
	actionSvcListVarName         = "KB_RSM_ACTION_SVC_LIST"
	RoleUpdateMechanismVarName   = "KB_RSM_ROLE_UPDATE_MECHANISM"
	roleProbeTimeoutVarName      = "KB_RSM_ROLE_PROBE_TIMEOUT"
	readinessProbeEventFieldPath = "spec.containers{" + roleProbeContainerName + "}"
)

const (
	EventReasonInvalidSpec   = "InvalidSpec"
	EventReasonStrictInPlace = "StrictInPlace"
)

const (
	// MaxPlainRevisionCount specified max number of plain revision stored in status.updateRevisions.
	// All revisions will be compressed if exceeding this value.
	MaxPlainRevisionCount = "MAX_PLAIN_REVISION_COUNT"

	templateRefAnnotationKey = "kubeblocks.io/template-ref"
	templateRefDataKey       = "instances"
	revisionsZSTDKey         = "zstd"

	FeatureGateIgnorePodVerticalScaling = "IGNORE_POD_VERTICAL_SCALING"

	finalizer = "instanceset.workloads.kubeblocks.io/finalizer"
)

// AnnotationScope defines scope that annotations belong to.
//
// it is a common pattern to add annotations to extend the functionalities of K8s builtin resources.
//
// e.g.: Prometheus will start to scrape metrics if a service has annotation 'prometheus.io/scrape'.
//
// The InstanceSet has encapsulated K8s builtin resources like Service, Headless Service, Pod, ConfigMap etc.
// AnnotationScope specified a way to tell the InstanceSet controller which resource an annotation belongs to.
//
// e.g.:
// let's say we want to add an annotation 'prometheus.io/scrape' with value 'true' to the underlying headless service.
// here is what we should do:
// add annotation 'prometheus.io/scrape' with an HeadlessServiceScope suffix to the RSM object's annotations field.
//
//	kind: InstanceSet
//	metadata:
//	  annotations:
//	    prometheus.io/scrape.headless.its: true
//
// the InstanceSet controller will figure out which objects this annotation belongs to, cut the suffix and set it to the right place:
//
//	kind: Service
//	metadata:
//	  annotations:
//	    prometheus.io/scrape: true
type AnnotationScope string

const (
	// RootScope specifies the annotation belongs to the RSM object itself.
	// they will also be set on the encapsulated StatefulSet.
	RootScope AnnotationScope = ""

	// HeadlessServiceScope specifies the annotation belongs to the encapsulated headless Service.
	HeadlessServiceScope AnnotationScope = ".headless.its"

	// ServiceScope specifies the annotation belongs to the encapsulated Service.
	ServiceScope AnnotationScope = ".svc.its"

	// AlternativeServiceScope specifies the annotation belongs to the encapsulated alternative Services.
	AlternativeServiceScope AnnotationScope = ".alternative.its"

	// ConfigMapScope specifies the annotation belongs to the encapsulated ConfigMap.
	ConfigMapScope AnnotationScope = ".cm.its"
)

const scopeSuffix = ".its"
