/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

// ClusterDefinitionSpec defines the desired state of ClusterDefinition.
type ClusterDefinitionSpec struct {
	// Provides the definitions for the cluster components.
	//
	// Deprecated since v0.9.
	// Components should now be individually defined using ComponentDefinition and
	// collectively referenced via `topology.components`.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ComponentDefs []ClusterComponentDefinition `json:"componentDefs" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Connection credential template used for creating a connection credential secret for cluster objects.
	//
	// Built-in objects are:
	//
	// - `$(RANDOM_PASSWD)` random 8 characters.
	// - `$(STRONG_RANDOM_PASSWD)` random 16 characters, with mixed cases, digits and symbols.
	// - `$(UUID)` generate a random UUID v4 string.
	// - `$(UUID_B64)` generate a random UUID v4 BASE64 encoded string.
	// - `$(UUID_STR_B64)` generate a random UUID v4 string then BASE64 encoded.
	// - `$(UUID_HEX)` generate a random UUID v4 HEX representation.
	// - `$(HEADLESS_SVC_FQDN)` headless service FQDN placeholder, value pattern is `$(CLUSTER_NAME)-$(1ST_COMP_NAME)-headless.$(NAMESPACE).svc`,
	//    where 1ST_COMP_NAME is the 1st component that provide `ClusterDefinition.spec.componentDefs[].service` attribute;
	// - `$(SVC_FQDN)` service FQDN placeholder, value pattern is `$(CLUSTER_NAME)-$(1ST_COMP_NAME).$(NAMESPACE).svc`,
	//    where 1ST_COMP_NAME is the 1st component that provide `ClusterDefinition.spec.componentDefs[].service` attribute;
	// - `$(SVC_PORT_{PORT-NAME})` is ServicePort's port value with specified port name, i.e, a servicePort JSON struct:
	//    `{"name": "mysql", "targetPort": "mysqlContainerPort", "port": 3306}`, and `$(SVC_PORT_mysql)` in the
	//    connection credential value is 3306.
	//
	// Deprecated since v0.9.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	ConnectionCredential map[string]string `json:"connectionCredential,omitempty"`

	// Topologies defines all possible topologies within the cluster.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +optional
	Topologies []ClusterTopology `json:"topologies,omitempty"`
}

// ClusterTopology represents the definition for a specific cluster topology.
type ClusterTopology struct {
	// Name is the unique identifier for the cluster topology.
	// Cannot be updated.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	Name string `json:"name"`

	// Components specifies the components in the topology.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	Components []ClusterTopologyComponent `json:"components"`

	// Specifies the sequence in which components within a cluster topology are
	// started, stopped, and upgraded.
	// This ordering is crucial for maintaining the correct dependencies and operational flow across components.
	//
	// +optional
	Orders *ClusterTopologyOrders `json:"orders,omitempty"`

	// Default indicates whether this topology serves as the default configuration.
	// When set to true, this topology is automatically used unless another is explicitly specified.
	//
	// +optional
	Default bool `json:"default,omitempty"`
}

// ClusterTopologyComponent defines a Component within a ClusterTopology.
type ClusterTopologyComponent struct {
	// Defines the unique identifier of the component within the cluster topology.
	// It follows IANA Service naming rules and is used as part of the Service's DNS name.
	// The name must start with a lowercase letter, can contain lowercase letters, numbers,
	// and hyphens, and must end with a lowercase letter or number.
	//
	// Cannot be updated once set.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=16
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Specifies the name or prefix of the ComponentDefinition custom resource(CR) that
	// defines the Component's characteristics and behavior.
	//
	// When a prefix is used, the system selects the ComponentDefinition CR with the latest version that matches the prefix.
	// This approach allows:
	//
	// 1. Precise selection by providing the exact name of a ComponentDefinition CR.
	// 2. Flexible and automatic selection of the most up-to-date ComponentDefinition CR by specifying a prefix.
	//
	// Once set, this field cannot be updated.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=64
	CompDef string `json:"compDef"`
}

// ClusterTopologyOrders manages the lifecycle of components within a cluster by defining their provisioning,
// terminating, and updating sequences.
// It organizes components into stages or groups, where each group indicates a set of components
// that can be managed concurrently.
// These groups are processed sequentially, allowing precise control based on component dependencies and requirements.
type ClusterTopologyOrders struct {
	// Specifies the order for creating and initializing components.
	// This is designed for components that depend on one another. Components without dependencies can be grouped together.
	//
	// Components that can be provisioned independently or have no dependencies can be listed together in the same stage,
	// separated by commas.
	//
	// +optional
	Provision []string `json:"provision,omitempty"`

	// Outlines the order for stopping and deleting components.
	// This sequence is designed for components that require a graceful shutdown or have interdependencies.
	//
	// Components that can be terminated independently or have no dependencies can be listed together in the same stage,
	// separated by commas.
	//
	// +optional
	Terminate []string `json:"terminate,omitempty"`

	// Update determines the order for updating components' specifications, such as image upgrades or resource scaling.
	// This sequence is designed for components that have dependencies or require specific update procedures.
	//
	// Components that can be updated independently or have no dependencies can be listed together in the same stage,
	// separated by commas.
	//
	// +optional
	Update []string `json:"update,omitempty"`
}

// CmdExecutorConfig specifies how to perform creation and deletion statements.
//
// Deprecated since v0.8.
type CmdExecutorConfig struct {
	CommandExecutorEnvItem `json:",inline"`
	CommandExecutorItem    `json:",inline"`
}

// PasswordConfig helps provide to customize complexity of password generation pattern.
type PasswordConfig struct {
	// The length of the password.
	//
	// +kubebuilder:validation:Maximum=32
	// +kubebuilder:validation:Minimum=8
	// +kubebuilder:default=16
	// +optional
	Length int32 `json:"length,omitempty"`

	// The number of digits in the password.
	//
	// +kubebuilder:validation:Maximum=8
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=4
	// +optional
	NumDigits int32 `json:"numDigits,omitempty"`

	// The number of symbols in the password.
	//
	// +kubebuilder:validation:Maximum=8
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	NumSymbols int32 `json:"numSymbols,omitempty"`

	// The case of the letters in the password.
	//
	// +kubebuilder:default=MixedCases
	// +optional
	LetterCase LetterCase `json:"letterCase,omitempty"`

	// Seed to generate the account's password.
	// Cannot be updated.
	//
	// +optional
	Seed string `json:"seed,omitempty"`
}

// ProvisionSecretRef represents the reference to a secret.
type ProvisionSecretRef struct {
	// The unique identifier of the secret.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// The namespace where the secret is located.
	//
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

// ClusterDefinitionStatus defines the observed state of ClusterDefinition
type ClusterDefinitionStatus struct {
	// Represents the most recent generation observed for this ClusterDefinition.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Specifies the current phase of the ClusterDefinition. Valid values are `empty`, `Available`, `Unavailable`.
	// When `Available`, the ClusterDefinition is ready and can be referenced by related objects.
	Phase Phase `json:"phase,omitempty"`

	// Provides additional information about the current phase.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// Topologies this ClusterDefinition supported.
	//
	// +optional
	Topologies string `json:"topologies,omitempty"`

	// The service references declared by this ClusterDefinition.
	//
	// +optional
	ServiceRefs string `json:"serviceRefs,omitempty"`
}

func (r ClusterDefinitionStatus) GetTerminalPhases() []Phase {
	return []Phase{AvailablePhase}
}

type LogConfig struct {
	// Specifies a descriptive label for the log type, such as 'slow' for a MySQL slow log file.
	// It provides a clear identification of the log's purpose and content.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name"`

	// Specifies the paths or patterns identifying where the log files are stored.
	// This field allows the system to locate and manage log files effectively.
	//
	// Examples:
	//
	// - /home/postgres/pgdata/pgroot/data/log/postgresql-*
	// - /data/mysql/log/mysqld-error.log
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=4096
	FilePathPattern string `json:"filePathPattern"`
}

// VolumeProtectionSpec is deprecated since v0.9, replaced with ComponentVolume.HighWatermark.
type VolumeProtectionSpec struct {
	// The high watermark threshold for volume space usage.
	// If there is any specified volumes who's space usage is over the threshold, the pre-defined "LOCK" action
	// will be triggered to degrade the service to protect volume from space exhaustion, such as to set the instance
	// as read-only. And after that, if all volumes' space usage drops under the threshold later, the pre-defined
	// "UNLOCK" action will be performed to recover the service normally.
	//
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=90
	// +optional
	HighWatermark int `json:"highWatermark,omitempty"`

	// The Volumes to be protected.
	//
	// +optional
	Volumes []ProtectedVolume `json:"volumes,omitempty"`
}

// ProtectedVolume is deprecated since v0.9, replaced with ComponentVolume.HighWatermark.
type ProtectedVolume struct {
	// The Name of the volume to protect.
	//
	// +optional
	Name string `json:"name,omitempty"`

	// Defines the high watermark threshold for the volume, it will override the component level threshold.
	// If the value is invalid, it will be ignored and the component level threshold will be used.
	//
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	// +optional
	HighWatermark *int `json:"highWatermark,omitempty"`
}

// ServiceRefDeclaration represents a reference to a service that can be either provided by a KubeBlocks Cluster
// or an external service.
// It acts as a placeholder for the actual service reference, which is determined later when a Cluster is created.
//
// The purpose of ServiceRefDeclaration is to declare a service dependency without specifying the concrete details
// of the service.
// It allows for flexibility and abstraction in defining service references within a Component.
// By using ServiceRefDeclaration, you can define service dependencies in a declarative manner, enabling loose coupling
// and easier management of service references across different components and clusters.
//
// Upon Cluster creation, the ServiceRefDeclaration is bound to an actual service through the ServiceRef field,
// effectively resolving and connecting to the specified service.
type ServiceRefDeclaration struct {
	// Specifies the name of the ServiceRefDeclaration.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Defines a list of constraints and requirements for services that can be bound to this ServiceRefDeclaration
	// upon Cluster creation.
	// Each ServiceRefDeclarationSpec defines a ServiceKind and ServiceVersion,
	// outlining the acceptable service types and versions that are compatible.
	//
	// This flexibility allows a ServiceRefDeclaration to be fulfilled by any one of the provided specs.
	// For example, if it requires an OLTP database, specs for both MySQL and PostgreSQL are listed,
	// either MySQL or PostgreSQL services can be used when binding.
	//
	// +kubebuilder:validation:Required
	ServiceRefDeclarationSpecs []ServiceRefDeclarationSpec `json:"serviceRefDeclarationSpecs"`

	// Specifies whether the service reference can be optional.
	//
	// For an optional service-ref, the component can still be created even if the service-ref is not provided.
	//
	// +optional
	Optional *bool `json:"optional,omitempty"`
}

type ServiceRefDeclarationSpec struct {
	// Specifies the type or nature of the service. This should be a well-known application cluster type, such as
	// {mysql, redis, mongodb}.
	// The field is case-insensitive and supports abbreviations for some well-known databases.
	// For instance, both `zk` and `zookeeper` are considered as a ZooKeeper cluster, while `pg`, `postgres`, `postgresql`
	// are all recognized as a PostgreSQL cluster.
	//
	// +kubebuilder:validation:Required
	ServiceKind string `json:"serviceKind"`

	// Defines the service version of the service reference. This is a regular expression that matches a version number pattern.
	// For instance, `^8.0.8$`, `8.0.\d{1,2}$`, `^[v\-]*?(\d{1,2}\.){0,3}\d{1,2}$` are all valid patterns.
	//
	// +kubebuilder:validation:Required
	ServiceVersion string `json:"serviceVersion"`
}

// ClusterComponentDefinition defines a Component within a ClusterDefinition but is deprecated and
// has been replaced by ComponentDefinition.
//
// Deprecated: Use ComponentDefinition instead. This type is deprecated as of version 0.8.
//
// +kubebuilder:validation:XValidation:rule="has(self.workloadType) && self.workloadType == 'Consensus' ? (has(self.consensusSpec) || has(self.rsmSpec)) : !has(self.consensusSpec)",message="componentDefs.consensusSpec(deprecated) or componentDefs.rsmSpec(recommended) is required when componentDefs.workloadType is Consensus, and forbidden otherwise"
type ClusterComponentDefinition struct {
	// This name could be used as default name of `cluster.spec.componentSpecs.name`, and needs to conform with same
	// validation rules as `cluster.spec.componentSpecs.name`, currently complying with IANA Service Naming rule.
	// This name will apply to cluster objects as the value of label "apps.kubeblocks.io/component-name".
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Defines the type of the workload.
	//
	// - `Stateless` describes stateless applications.
	// - `Stateful` describes common stateful applications.
	// - `Consensus` describes applications based on consensus protocols, such as raft and paxos.
	// - `Replication` describes applications based on the primary-secondary data replication protocol.
	//
	// +kubebuilder:validation:Required
	WorkloadType WorkloadType `json:"workloadType"`

	// Settings for health checks.
	//
	// +optional
	Probes *ClusterDefinitionProbes `json:"probes,omitempty"`

	// Defines the pod spec template of component.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	PodSpec *corev1.PodSpec `json:"podSpec,omitempty"`

	// Defines spec for `Stateless` workloads.
	//
	// +kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	// +optional
	StatelessSpec *StatelessSetSpec `json:"statelessSpec,omitempty"`

	// Defines spec for `Stateful` workloads.
	//
	// +kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	// +optional
	StatefulSpec *StatefulSetSpec `json:"statefulSpec,omitempty"`

	// Defines spec for `Consensus` workloads. It's required if the workload type is `Consensus`.
	//
	// +kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	// +optional
	ConsensusSpec *ConsensusSetSpec `json:"consensusSpec,omitempty"`

	// Defines spec for `Replication` workloads.
	//
	// +kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	// +optional
	ReplicationSpec *ReplicationSetSpec `json:"replicationSpec,omitempty"`

	// Defines workload spec of this component.
	// From KB 0.7.0, RSM(InstanceSetSpec) will be the underlying CR which powers all kinds of workload in KB.
	// RSM is an enhanced stateful workload extension dedicated for heavy-state workloads like databases.
	//
	// +optional
	RSMSpec *RSMSpec `json:"rsmSpec,omitempty"`

	// Defines the behavior of horizontal scale.
	//
	// +optional
	HorizontalScalePolicy *HorizontalScalePolicy `json:"horizontalScalePolicy,omitempty"`

	// Defines command to do switchover.
	// In particular, when workloadType=Replication, the command defined in switchoverSpec will only be executed under
	// the condition of cluster.componentSpecs[x].SwitchPolicy.type=Noop.
	//
	// +optional
	SwitchoverSpec *SwitchoverSpec `json:"switchoverSpec,omitempty"`
}

func (r *ClusterComponentDefinition) GetStatefulSetWorkload() StatefulSetWorkload {
	switch r.WorkloadType {
	case Stateless:
		return nil
	case Stateful:
		return r.StatefulSpec
	case Consensus:
		return r.ConsensusSpec
	case Replication:
		return r.ReplicationSpec
	}
	panic("unreachable")
}

func (r *ClusterComponentDefinition) IsStatelessWorkload() bool {
	return r.WorkloadType == Stateless
}

func (r *ClusterComponentDefinition) GetCommonStatefulSpec() (*StatefulSetSpec, error) {
	if r.IsStatelessWorkload() {
		return nil, ErrWorkloadTypeIsStateless
	}
	switch r.WorkloadType {
	case Stateful:
		return r.StatefulSpec, nil
	case Consensus:
		if r.ConsensusSpec != nil {
			return &r.ConsensusSpec.StatefulSetSpec, nil
		}
	case Replication:
		if r.ReplicationSpec != nil {
			return &r.ReplicationSpec.StatefulSetSpec, nil
		}
	default:
		panic("unreachable")
		// return nil, ErrWorkloadTypeIsUnknown
	}
	return nil, nil
}

// HorizontalScalePolicy is deprecated since v0.8.
type HorizontalScalePolicy struct {
	// Determines the data synchronization method when a component scales out.
	// The policy can be one of the following: {None, CloneVolume}. The default policy is `None`.
	//
	// - `None`: This is the default policy. It creates an empty volume without data cloning.
	// - `CloneVolume`: This policy clones data to newly scaled pods. It first tries to use a volume snapshot.
	//   If volume snapshot is not enabled, it will attempt to use a backup tool. If neither method works, it will report an error.
	// - `Snapshot`: This policy is deprecated and is an alias for CloneVolume.
	//
	// +kubebuilder:default=None
	// +optional
	Type HScaleDataClonePolicyType `json:"type,omitempty"`

	// Refers to the backup policy template.
	//
	// +optional
	BackupPolicyTemplateName string `json:"backupPolicyTemplateName,omitempty"`

	// Specifies the volumeMount of the container to backup.
	// This only works if Type is not None. If not specified, the first volumeMount will be selected.
	//
	// +optional
	VolumeMountsName string `json:"volumeMountsName,omitempty"`
}

// ClusterDefinitionProbeCMDs is deprecated since v0.8.
type ClusterDefinitionProbeCMDs struct {
	// Defines write checks that are executed on the probe sidecar.
	//
	// +optional
	Writes []string `json:"writes,omitempty"`

	// Defines read checks that are executed on the probe sidecar.
	//
	// +optional
	Queries []string `json:"queries,omitempty"`
}

// ClusterDefinitionProbe is deprecated since v0.8.
type ClusterDefinitionProbe struct {
	// How often (in seconds) to perform the probe.
	//
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`

	// Number of seconds after which the probe times out. Defaults to 1 second.
	//
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	//
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=2
	FailureThreshold int32 `json:"failureThreshold,omitempty"`

	// Commands used to execute for probe.
	//
	// +optional
	Commands *ClusterDefinitionProbeCMDs `json:"commands,omitempty"`
}

// ClusterDefinitionProbes is deprecated since v0.8.
type ClusterDefinitionProbes struct {
	// Specifies the probe used for checking the running status of the component.
	//
	// +optional
	RunningProbe *ClusterDefinitionProbe `json:"runningProbe,omitempty"`

	// Specifies the probe used for checking the status of the component.
	//
	// +optional
	StatusProbe *ClusterDefinitionProbe `json:"statusProbe,omitempty"`

	// Specifies the probe used for checking the role of the component.
	//
	// +kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	// +optional
	RoleProbe *ClusterDefinitionProbe `json:"roleProbe,omitempty"`

	// Defines the timeout (in seconds) for the role probe after all pods of the component are ready.
	// The system will check if the application is available in the pod.
	// If pods exceed the InitializationTimeoutSeconds time without a role label, this component will enter the
	// Failed/Abnormal phase.
	//
	// Note that this configuration will only take effect if the component supports RoleProbe
	// and will not affect the life cycle of the pod. default values are 60 seconds.
	//
	// +kubebuilder:validation:Minimum=30
	// +optional
	RoleProbeTimeoutAfterPodsReady int32 `json:"roleProbeTimeoutAfterPodsReady,omitempty"`
}

// StatelessSetSpec is deprecated since v0.7.
type StatelessSetSpec struct {
	// Specifies the deployment strategy that will be used to replace existing pods with new ones.
	//
	// +patchStrategy=retainKeys
	// +optional
	UpdateStrategy appsv1.DeploymentStrategy `json:"updateStrategy,omitempty"`
}

// StatefulSetSpec is deprecated since v0.7.
type StatefulSetSpec struct {
	// Specifies the strategy for updating Pods.
	// For workloadType=`Consensus`, the update strategy can be one of the following:
	//
	// - `Serial`: Updates Members sequentially to minimize component downtime.
	// - `BestEffortParallel`: Updates Members in parallel to minimize component write downtime. Majority remains online
	// at all times.
	// - `Parallel`: Forces parallel updates.
	//
	// +kubebuilder:default=Serial
	// +optional
	UpdateStrategy UpdateStrategy `json:"updateStrategy,omitempty"`

	// Controls the creation of pods during initial scale up, replacement of pods on nodes, and scaling down.
	//
	// - `OrderedReady`: Creates pods in increasing order (pod-0, then pod-1, etc). The controller waits until each pod
	// is ready before continuing. Pods are removed in reverse order when scaling down.
	// - `Parallel`: Creates pods in parallel to match the desired scale without waiting. All pods are deleted at once
	// when scaling down.
	//
	// +optional
	LLPodManagementPolicy appsv1.PodManagementPolicyType `json:"llPodManagementPolicy,omitempty"`

	// Specifies the low-level StatefulSetUpdateStrategy to be used when updating Pods in the StatefulSet upon a
	// revision to the Template.
	// `UpdateStrategy` will be ignored if this is provided.
	//
	// +optional
	LLUpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"llUpdateStrategy,omitempty"`
}

var _ StatefulSetWorkload = &StatefulSetSpec{}

func (r *StatefulSetSpec) GetUpdateStrategy() UpdateStrategy {
	if r == nil {
		return SerialStrategy
	}
	return r.UpdateStrategy
}

func (r *StatefulSetSpec) FinalStsUpdateStrategy() (appsv1.PodManagementPolicyType, appsv1.StatefulSetUpdateStrategy) {
	if r == nil {
		r = &StatefulSetSpec{
			UpdateStrategy: SerialStrategy,
		}
	}
	return r.finalStsUpdateStrategy()
}

func (r *StatefulSetSpec) finalStsUpdateStrategy() (appsv1.PodManagementPolicyType, appsv1.StatefulSetUpdateStrategy) {
	if r.LLUpdateStrategy != nil {
		return r.LLPodManagementPolicy, *r.LLUpdateStrategy
	}

	zeroPartition := int32(0)
	switch r.UpdateStrategy {
	case BestEffortParallelStrategy:
		m := intstr.FromString("49%")
		return appsv1.ParallelPodManagement, appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.RollingUpdateStatefulSetStrategyType,
			RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
				// explicitly set the partition as 0 to avoid update workload unexpectedly.
				Partition: &zeroPartition,
				// alpha feature since v1.24
				// ref: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#maximum-unavailable-pods
				MaxUnavailable: &m,
			},
		}
	case ParallelStrategy:
		return appsv1.ParallelPodManagement, appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.RollingUpdateStatefulSetStrategyType,
		}
	case SerialStrategy:
		fallthrough
	default:
		m := intstr.FromInt(1)
		return appsv1.OrderedReadyPodManagement, appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.RollingUpdateStatefulSetStrategyType,
			RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
				// explicitly set the partition as 0 to avoid update workload unexpectedly.
				Partition: &zeroPartition,
				// alpha feature since v1.24
				// ref: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#maximum-unavailable-pods
				MaxUnavailable: &m,
			},
		}
	}
}

// ConsensusSetSpec is deprecated since v0.7.
type ConsensusSetSpec struct {
	StatefulSetSpec `json:",inline"`

	// Represents a single leader in the consensus set.
	//
	// +kubebuilder:validation:Required
	Leader ConsensusMember `json:"leader"`

	// Members of the consensus set that have voting rights but are not the leader.
	//
	// +optional
	Followers []ConsensusMember `json:"followers,omitempty"`

	// Represents a member of the consensus set that does not have voting rights.
	//
	// +optional
	Learner *ConsensusMember `json:"learner,omitempty"`
}

var _ StatefulSetWorkload = &ConsensusSetSpec{}

func (r *ConsensusSetSpec) GetUpdateStrategy() UpdateStrategy {
	if r == nil {
		return SerialStrategy
	}
	return r.UpdateStrategy
}

func (r *ConsensusSetSpec) FinalStsUpdateStrategy() (appsv1.PodManagementPolicyType, appsv1.StatefulSetUpdateStrategy) {
	if r == nil {
		r = NewConsensusSetSpec()
	}
	if r.LLUpdateStrategy != nil {
		return r.LLPodManagementPolicy, *r.LLUpdateStrategy
	}
	_, s := r.StatefulSetSpec.finalStsUpdateStrategy()
	// switch r.UpdateStrategy {
	// case SerialStrategy, BestEffortParallelStrategy:
	s.Type = appsv1.OnDeleteStatefulSetStrategyType
	s.RollingUpdate = nil
	// }
	return appsv1.ParallelPodManagement, s
}

func NewConsensusSetSpec() *ConsensusSetSpec {
	return &ConsensusSetSpec{
		Leader: DefaultLeader,
		StatefulSetSpec: StatefulSetSpec{
			UpdateStrategy: SerialStrategy,
		},
	}
}

// ConsensusMember is deprecated since v0.7.
type ConsensusMember struct {
	// Specifies the name of the consensus member.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=leader
	Name string `json:"name"`

	// Specifies the services that this member is capable of providing.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=ReadWrite
	AccessMode AccessMode `json:"accessMode"`

	// Indicates the number of Pods that perform this role.
	// The default is 1 for `Leader`, 0 for `Learner`, others for `Followers`.
	//
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

// RSMSpec is deprecated since v0.8.
type RSMSpec struct {
	// Specifies a list of roles defined within the system.
	//
	// +optional
	Roles []workloads.ReplicaRole `json:"roles,omitempty"`

	// Defines the method used to probe a role.
	//
	// +optional
	RoleProbe *workloads.RoleProbe `json:"roleProbe,omitempty"`

	// Indicates the actions required for dynamic membership reconfiguration.
	//
	// +optional
	MembershipReconfiguration *workloads.MembershipReconfiguration `json:"membershipReconfiguration,omitempty"`

	// Describes the strategy for updating Members (Pods).
	//
	// - `Serial`: Updates Members sequentially to ensure minimum component downtime.
	// - `BestEffortParallel`: Updates Members in parallel to ensure minimum component write downtime.
	// - `Parallel`: Forces parallel updates.
	//
	// +kubebuilder:validation:Enum={Serial,BestEffortParallel,Parallel}
	// +optional
	MemberUpdateStrategy *workloads.MemberUpdateStrategy `json:"memberUpdateStrategy,omitempty"`
}

// ReplicationSetSpec is deprecated since v0.7.
type ReplicationSetSpec struct {
	StatefulSetSpec `json:",inline"`
}

var _ StatefulSetWorkload = &ReplicationSetSpec{}

func (r *ReplicationSetSpec) GetUpdateStrategy() UpdateStrategy {
	if r == nil {
		return SerialStrategy
	}
	return r.UpdateStrategy
}

func (r *ReplicationSetSpec) FinalStsUpdateStrategy() (appsv1.PodManagementPolicyType, appsv1.StatefulSetUpdateStrategy) {
	if r == nil {
		r = &ReplicationSetSpec{}
	}
	if r.LLUpdateStrategy != nil {
		return r.LLPodManagementPolicy, *r.LLUpdateStrategy
	}
	_, s := r.StatefulSetSpec.finalStsUpdateStrategy()
	s.Type = appsv1.OnDeleteStatefulSetStrategyType
	s.RollingUpdate = nil
	return appsv1.ParallelPodManagement, s
}

// SwitchoverSpec is deprecated since v0.8.
type SwitchoverSpec struct {
	// Represents the action of switching over to a specified candidate primary or leader instance.
	//
	// +optional
	WithCandidate *SwitchoverAction `json:"withCandidate,omitempty"`

	// Represents the action of switching over without specifying a candidate primary or leader instance.
	//
	// +optional
	WithoutCandidate *SwitchoverAction `json:"withoutCandidate,omitempty"`
}

// SwitchoverAction is deprecated since v0.8.
type SwitchoverAction struct {
	// Specifies the switchover command.
	//
	// +kubebuilder:validation:Required
	CmdExecutorConfig *CmdExecutorConfig `json:"cmdExecutorConfig"`

	// Used to select the script that need to be referenced.
	// When defined, the scripts defined in scriptSpecs can be referenced within the SwitchoverAction.CmdExecutorConfig.
	//
	// +kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.9.0"
	// +optional
	ScriptSpecSelectors []ScriptSpecSelector `json:"scriptSpecSelectors,omitempty"`
}

type ScriptSpecSelector struct {
	// Represents the name of the ScriptSpec referent.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`
}

// CommandExecutorEnvItem is deprecated since v0.8.
type CommandExecutorEnvItem struct {
	// Specifies the image used to execute the command.
	//
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// A list of environment variables that will be injected into the command execution context.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

// CommandExecutorItem is deprecated since v0.8.
type CommandExecutorItem struct {
	// The command to be executed.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Command []string `json:"command"`

	// Additional parameters used in the execution of the command.
	//
	// +optional
	Args []string `json:"args,omitempty"`
}

// TODO(API):
//  1. how to display the aggregated topologies and its service references line by line?
//  2. the services and versions supported

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=cd
// +kubebuilder:printcolumn:name="Topologies",type="string",JSONPath=".status.topologies",description="topologies"
// +kubebuilder:printcolumn:name="ServiceRefs",type="string",JSONPath=".status.serviceRefs",description="service references"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterDefinition defines the topology for databases or storage systems,
// offering a variety of topological configurations to meet diverse deployment needs and scenarios.
//
// It includes a list of Components, each linked to a ComponentDefinition, which enhances reusability and reduce redundancy.
// For example, widely used components such as etcd and Zookeeper can be defined once and reused across multiple ClusterDefinitions,
// simplifying the setup of new systems.
//
// Additionally, ClusterDefinition also specifies the sequence of startup, upgrade, and shutdown for Components,
// ensuring a controlled and predictable management of component lifecycles.
type ClusterDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterDefinitionSpec   `json:"spec,omitempty"`
	Status ClusterDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterDefinitionList contains a list of ClusterDefinition
type ClusterDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterDefinition{}, &ClusterDefinitionList{})
}

// GetComponentDefByName gets component definition from ClusterDefinition with compDefName
func (r *ClusterDefinition) GetComponentDefByName(compDefName string) *ClusterComponentDefinition {
	for _, component := range r.Spec.ComponentDefs {
		if component.Name == compDefName {
			return &component
		}
	}
	return nil
}

// FailurePolicyType specifies the type of failure policy.
//
// +enum
// +kubebuilder:validation:Enum={Ignore,Fail}
type FailurePolicyType string

const (
	// FailurePolicyIgnore means that an error will be ignored but logged.
	FailurePolicyIgnore FailurePolicyType = "Ignore"
	// FailurePolicyFail means that an error will be reported.
	FailurePolicyFail FailurePolicyType = "Fail"
)
