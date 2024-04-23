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

	RoleLabelKey          = "kubeblocks.io/role"
	rsmAccessModeLabelKey = "rsm.workloads.kubeblocks.io/access-mode"

	defaultPodName = "Unknown"

	LegacyRSMFinalizerName = "rsm.workloads.kubeblocks.io/finalizer"

	roleProbeContainerName       = "kb-role-probe"
	roleProbeBinaryName          = "lorry"
	roleAgentVolumeName          = "role-agent"
	roleAgentInstallerName       = "role-agent-installer"
	roleAgentVolumeMountPath     = "/role-probe"
	roleAgentName                = "agent"
	shell2httpImage              = "msoap/shell2http:1.16.0"
	shell2httpBinaryPath         = "/app/shell2http"
	shell2httpServePath          = "/role"
	defaultRoleProbeDaemonPort   = 7373
	defaultRoleProbeGRPCPort     = 50101
	roleProbeGRPCPortName        = "probe-grpc-port"
	httpRoleProbePath            = "/v1.0/checkrole"
	grpcHealthProbeBinaryPath    = "/bin/grpc_health_probe"
	grpcHealthProbeArgsFormat    = "-addr=:%d"
	defaultActionImage           = "busybox:1.35"
	usernameCredentialVarName    = "KB_RSM_USERNAME"
	passwordCredentialVarName    = "KB_RSM_PASSWORD"
	servicePortVarName           = "KB_RSM_SERVICE_PORT"
	actionSvcListVarName         = "KB_RSM_ACTION_SVC_LIST"
	leaderHostVarName            = "KB_RSM_LEADER_HOST"
	targetHostVarName            = "KB_RSM_TARGET_HOST"
	RoleUpdateMechanismVarName   = "KB_RSM_ROLE_UPDATE_MECHANISM"
	roleProbeTimeoutVarName      = "KB_RSM_ROLE_PROBE_TIMEOUT"
	readinessProbeEventFieldPath = "spec.containers{" + roleProbeContainerName + "}"
	legacyEventFieldPath         = "spec.containers{kb-checkrole}"
	lorryEventFieldPath          = "spec.containers{lorry}"
	checkRoleEventReason         = "checkRole"

	actionSvcPortBase = int32(
		36500,
	)
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
// RSM has encapsulated K8s builtin resources like Service, Headless Service, StatefulSet, ConfigMap etc.
// AnnotationScope specified a way to tell RSM controller which resource an annotation belongs to.
//
// e.g.:
// let's say we want to add an annotation 'prometheus.io/scrape' with value 'true' to the underlying headless service.
// here is what we should do:
// add annotation 'prometheus.io/scrape' with an HeadlessServiceScope suffix to the RSM object's annotations field.
//
//	kind: InstanceSet
//	metadata:
//	  annotations:
//	    prometheus.io/scrape.headless.rsm: true
//
// the RSM controller will figure out which objects this annotation belongs to, cut the suffix and set it to the right place:
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
	HeadlessServiceScope AnnotationScope = ".headless.rsm"

	// ServiceScope specifies the annotation belongs to the encapsulated Service.
	ServiceScope AnnotationScope = ".svc.rsm"

	// AlternativeServiceScope specifies the annotation belongs to the encapsulated alternative Services.
	AlternativeServiceScope AnnotationScope = ".alternative.rsm"

	// ConfigMapScope specifies the annotation belongs to the encapsulated ConfigMap.
	ConfigMapScope AnnotationScope = ".cm.rsm"
)

const scopeSuffix = ".rsm"
