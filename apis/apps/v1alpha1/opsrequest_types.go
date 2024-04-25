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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// TODO: @wangyelei could refactor to ops group

// OpsRequestSpec defines the desired state of OpsRequest
// +kubebuilder:validation:XValidation:rule="has(self.cancel) && self.cancel ? (self.type in ['VerticalScaling', 'HorizontalScaling']) : true",message="forbidden to cancel the opsRequest which type not in ['VerticalScaling','HorizontalScaling']"
type OpsRequestSpec struct {
	// References the cluster object.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterRef"
	ClusterRef string `json:"clusterRef"`

	// Defines the action to cancel the `Pending/Creating/Running` opsRequest, supported types: `VerticalScaling/HorizontalScaling`.
	// Once set to true, this opsRequest will be canceled and modifying this property again will not take effect.
	// +optional
	Cancel bool `json:"cancel,omitempty"`

	// Indicates if pre-checks should be bypassed, allowing the opsRequest to execute immediately. If set to true, pre-checks are skipped except for 'Start' type.
	// Particularly useful when concurrent execution of VerticalScaling and HorizontalScaling opsRequests is required,
	// achievable through the use of the Force flag.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.force"
	// +optional
	Force bool `json:"force,omitempty"`

	// Defines the operation type.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.type"
	Type OpsType `json:"type"`

	// OpsRequest will be deleted after TTLSecondsAfterSucceed second when OpsRequest.status.phase is Succeed.
	// +optional
	TTLSecondsAfterSucceed int32 `json:"ttlSecondsAfterSucceed,omitempty"`

	// Specifies the cluster version by specifying clusterVersionRef.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.upgrade"
	Upgrade *Upgrade `json:"upgrade,omitempty"`

	// Defines what component need to horizontal scale the specified replicas.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.horizontalScaling"
	HorizontalScalingList []HorizontalScaling `json:"horizontalScaling,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Note: Quantity struct can not do immutable check by CEL.
	// Defines what component and volumeClaimTemplate need to expand the specified storage.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	VolumeExpansionList []VolumeExpansion `json:"volumeExpansion,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Restarts the specified components.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.restart"
	RestartList []ComponentOps `json:"restart,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Switches over the specified components.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.switchover"
	SwitchoverList []Switchover `json:"switchover,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Note: Quantity struct can not do immutable check by CEL.
	// Defines what component need to vertical scale the specified compute resources.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	VerticalScalingList []VerticalScaling `json:"verticalScaling,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Deprecated: replace by reconfigures.
	// Defines the variables that need to input when updating configuration.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.reconfigure"
	// +kubebuilder:validation:XValidation:rule="self.configurations.size() > 0", message="Value can not be empty"
	Reconfigure *Reconfigure `json:"reconfigure,omitempty"`

	// Defines the variables that need to input when updating configuration.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	Reconfigures []Reconfigure `json:"reconfigures,omitempty"`

	// Defines services the component needs to expose.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	ExposeList []Expose `json:"expose,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Cluster RestoreFrom backup or point in time.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.restoreFrom"
	RestoreFrom *RestoreFromSpec `json:"restoreFrom,omitempty"`

	// OpsRequest will wait at most TTLSecondsBeforeAbort seconds for start-conditions to be met.
	// If not specified, the default value is 0, which means that the start-conditions must be met immediately.
	// +kubebuilder:default=0
	// +optional
	TTLSecondsBeforeAbort *int32 `json:"ttlSecondsBeforeAbort,omitempty"`

	// Defines the script to be executed.
	// +optional
	ScriptSpec *ScriptSpec `json:"scriptSpec,omitempty"`

	// Defines how to backup the cluster.
	// +optional
	BackupSpec *BackupSpec `json:"backupSpec,omitempty"`

	// Defines how to restore the cluster.
	// Note that this restore operation will roll back cluster services.
	// +optional
	RestoreSpec *RestoreSpec `json:"restoreSpec,omitempty"`

	// Specifies the instances that require re-creation.
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.rebuildFrom"
	RebuildFrom []RebuildInstance `json:"rebuildFrom,omitempty"  patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Specifies a custom operation as defined by OpsDefinition.
	// +optional
	CustomSpec *CustomOpsSpec `json:"customSpec,omitempty"`
}

// ComponentOps represents the common variables required for operations within the scope of a component.
type ComponentOps struct {
	// Specifies the name of the cluster component.
	// +kubebuilder:validation:Required
	ComponentName string `json:"componentName"`
}

type RebuildInstance struct {
	ComponentOps `json:",inline"`

	// Defines the instances that need to be rebuilt.
	// +kubebuilder:validation:Required
	Instances []Instance `json:"instances"`

	// Indicates the name of the backup from which to recover. Currently, only a full physical backup is supported
	// unless your component only has one replica. Such as 'xtrabackup' is full physical backup for mysql and 'mysqldump' is not.
	// And if no specified backupName, the instance will be recreated with empty 'PersistentVolumes'.
	// +optional
	BackupName string `json:"backupName,omitempty"`

	// List of environment variables to set in the container for restore. These will be
	// merged with the env of Backup and ActionSet.
	//
	// The priority of merging is as follows: `Restore env > Backup env > ActionSet env`.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	EnvForRestore []corev1.EnvVar `json:"envForRestore,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

type Instance struct {
	// Pod name of the instance.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// The instance will rebuild on the specified node when the instance uses local PersistentVolume as the storage disk.
	// If not set, it will rebuild on a random node.
	// +optional
	TargetNodeName string `json:"targetNodeName,omitempty"`
}

type Switchover struct {
	ComponentOps `json:",inline"`

	// Utilized to designate the candidate primary or leader instance for the switchover process.
	// If assigned "*", it signifies that no specific primary or leader is designated for the switchover,
	// and the switchoverAction defined in `clusterDefinition.componentDefs[x].switchoverSpec.withoutCandidate` will be executed.
	//
	// It is mandatory that `clusterDefinition.componentDefs[x].switchoverSpec.withoutCandidate` is not left blank.
	//
	// If assigned a valid instance name other than "*", it signifies that a specific candidate primary or leader is designated for the switchover.
	// The value can be retrieved using `kbcli cluster list-instances`, any other value is considered invalid.
	//
	// In this scenario, the `switchoverAction` defined in clusterDefinition.componentDefs[x].switchoverSpec.withCandidate will be executed,
	// and it is mandatory that clusterDefinition.componentDefs[x].switchoverSpec.withCandidate is not left blank.
	//
	// +kubebuilder:validation:Required
	InstanceName string `json:"instanceName"`
}

// Upgrade represents the parameters required for an upgrade operation.
type Upgrade struct {
	// A reference to the name of the ClusterVersion.
	//
	// +kubebuilder:validation:Required
	ClusterVersionRef string `json:"clusterVersionRef"`
}

// VerticalScaling defines the parameters required for scaling compute resources.
type VerticalScaling struct {
	ComponentOps `json:",inline"`

	// Defines the computational resource size for vertical scaling.
	// +kubebuilder:pruning:PreserveUnknownFields
	corev1.ResourceRequirements `json:",inline"`
}

// VolumeExpansion encapsulates the parameters required for a volume expansion operation.
type VolumeExpansion struct {
	ComponentOps `json:",inline"`

	// volumeClaimTemplates specifies the storage size and volumeClaimTemplate name.
	// +kubebuilder:validation:Required
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	VolumeClaimTemplates []OpsRequestVolumeClaimTemplate `json:"volumeClaimTemplates" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
}

type OpsRequestVolumeClaimTemplate struct {
	// Specifies the requested storage size for the volume.
	// +kubebuilder:validation:Required
	Storage resource.Quantity `json:"storage"`

	// A reference to the volumeClaimTemplate name from the cluster components.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// HorizontalScaling defines the variables of horizontal scaling operation
type HorizontalScaling struct {
	ComponentOps `json:",inline"`

	//  Specifies the number of replicas for the workloads.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas"`

	// Specifies instances to be added and/or deleted for the workloads.
	// Name and Replicas should be provided. Other fields will simply be ignored.
	// The Replicas will be overridden if an existing InstanceTemplate is matched by Name.
	// Or the InstanceTemplate will be added as a new one.
	//
	// +optional
	Instances []InstanceTemplate `json:"instances,omitempty"`

	// Specifies instances to be scaled in with dedicated names in the list.
	//
	// +optional
	OfflineInstances []string `json:"offlineInstances,omitempty"`
}

// Reconfigure represents the variables required for updating a configuration.
type Reconfigure struct {
	ComponentOps `json:",inline"`

	// Specifies the components that will perform the operation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Configurations []ConfigurationItem `json:"configurations" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Indicates the duration for which the parameter changes are valid.
	// +optional
	// TTL *int64 `json:"ttl,omitempty"`

	// Specifies the time when the parameter changes should be applied.
	// +kubebuilder:validation:MaxLength=19
	// +kubebuilder:validation:MinLength=19
	// +kubebuilder:validation:Pattern:=`^([0-9]{2})/([0-9]{2})/([0-9]{4}) ([0-9]{2}):([0-9]{2}):([0-9]{2})$`
	// +optional
	// TriggeringTime *string `json:"triggeringTime,omitempty"`

	//  Identifies the component to be reconfigured.
	// +optional
	// Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

type ConfigurationItem struct {
	// Specifies the name of the configuration template.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Defines the upgrade policy for the configuration. This field is optional.
	// +optional
	Policy *UpgradePolicy `json:"policy,omitempty"`

	// Sets the parameters to be updated. It should contain at least one item. The keys are merged and retained during patch operations.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=key
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=key
	Keys []ParameterConfig `json:"keys" patchStrategy:"merge,retainKeys" patchMergeKey:"key"`
}

type CustomOpsSpec struct {

	// Is a reference to an OpsDefinition.
	// +kubebuilder:validation:Required
	OpsDefinitionRef string `json:"opsDefinitionRef"`

	ServiceAccountName *string `json:"serviceAccountName,omitempty"`

	// Defines the execution concurrency. By default, all incoming Components will be executed simultaneously.
	// The value can be an absolute number (e.g., 5) or a percentage of desired components (e.g., 10%).
	// The absolute number is calculated from the percentage by rounding up.
	// For instance, if the percentage value is 10% and the components length is 1,
	// the calculated number will be rounded up to 1.
	// +optional
	Parallelism intstr.IntOrString `json:"parallelism,omitempty"`

	// Defines which components need to perform the actions defined by this OpsDefinition.
	// At least one component is required. The components are identified by their name and can be merged or retained.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	CustomOpsComponents []CustomOpsComponent `json:"components" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
}

type CustomOpsComponent struct {
	// Specifies the unique identifier of the cluster component
	// +kubebuilder:validation:Required
	ComponentName string `json:"name"`

	// Represents the parameters for this operation as declared in the opsDefinition.spec.parametersSchema.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	Parameters []Parameter `json:"parameters,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
}

type Parameter struct {
	// Specifies the identifier of the parameter as defined in the OpsDefinition.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Holds the data associated with the parameter.
	// If the parameter type is an array, the format should be "v1,v2,v3".
	// +kubebuilder:validation:Required
	Value string `json:"value"`
}

type ParameterPair struct {
	// Represents the name of the parameter that is to be updated.
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Represents the parameter values that are to be updated.
	// If set to nil, the parameter defined by the Key field will be removed from the configuration file.
	// +optional
	Value *string `json:"value"`
}

type ParameterConfig struct {
	// Represents the unique identifier for the ConfigMap.
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Defines a list of key-value pairs for a single configuration file.
	// These parameters are used to update the specified configuration settings.
	// +optional
	Parameters []ParameterPair `json:"parameters,omitempty"`

	// Represents the content of the configuration file.
	// This field is used to update the entire content of the file.
	// +optional
	FileContent string `json:"fileContent,omitempty"`
}

// ExposeSwitch Specifies the switch for the expose operation. This switch can be used to enable or disable the expose operation.
// +enum
// +kubebuilder:validation:Enum={Enable, Disable}
type ExposeSwitch string

const (
	EnableExposeSwitch  ExposeSwitch = "Enable"
	DisableExposeSwitch ExposeSwitch = "Disable"
)

type Expose struct {
	ComponentOps `json:",inline"`

	// Controls the expose operation.
	// If set to Enable, the corresponding service will be exposed. Conversely, if set to Disable, the service will be removed.
	//
	// +kubebuilder:validation:Required
	Switch ExposeSwitch `json:"switch"`

	// A list of services that are to be exposed or removed.
	// If componentNamem is not specified, each `OpsService` in the list must specify ports and selectors.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minitems=0
	Services []OpsService `json:"services"`
}

type OpsService struct {
	// Specifies the name of the service. This name is used by others to refer to this service (e.g., connection credential).
	// Note: This field cannot be updated.
	// +required
	Name string `json:"name"`

	// Contains cloud provider related parameters if ServiceType is LoadBalancer.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Lists the ports that are exposed by this service.
	// If not provided, the default Services Ports defined in the ClusterDefinition or ComponentDefinition that are neither of NodePort nor LoadBalancer service type will be used.
	// If there is no corresponding Service defined in the ClusterDefinition or ComponentDefinition, the expose operation will fail.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies
	// +patchMergeKey=port
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=port
	// +listMapKey=protocol
	// +optional
	Ports []corev1.ServicePort `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"port" protobuf:"bytes,1,rep,name=ports"`

	// Allows you to specify a defined role as a selector for the service, extending the ServiceSpec.Selector.
	// +optional
	RoleSelector string `json:"roleSelector,omitempty"`

	// Routes service traffic to pods with label keys and values matching this selector.
	// If empty or not present, the service is assumed to have an external process managing its endpoints, which Kubernetes will not modify.
	// This only applies to types ClusterIP, NodePort, and LoadBalancer and is ignored if type is ExternalName.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/
	// +optional
	// +mapType=atomic
	Selector map[string]string `json:"selector,omitempty" protobuf:"bytes,2,rep,name=selector"`

	// Determines how the Service is exposed. Defaults to ClusterIP. Valid options are ExternalName, ClusterIP, NodePort, and LoadBalancer.
	// - `ClusterIP` allocates a cluster-internal IP address for load-balancing to endpoints.
	// - `NodePort` builds on ClusterIP and allocates a port on every node which routes to the same endpoints as the clusterIP.
	// - `LoadBalancer` builds on NodePort and creates an external load-balancer (if supported in the current cloud) which routes to the same endpoints as the clusterIP.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types.
	// +optional
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// IPFamilies is a list of IP families (e.g. IPv4, IPv6) assigned to this
	// service. This field is usually assigned automatically based on cluster
	// configuration and the ipFamilyPolicy field. If this field is specified
	// manually, the requested family is available in the cluster,
	// and ipFamilyPolicy allows it, it will be used; otherwise creation of
	// the service will fail. This field is conditionally mutable: it allows
	// for adding or removing a secondary IP family, but it does not allow
	// changing the primary IP family of the Service. Valid values are "IPv4"
	// and "IPv6".  This field only applies to Services of types ClusterIP,
	// NodePort, and LoadBalancer, and does apply to "headless" services.
	// This field will be wiped when updating a Service to type ExternalName.
	//
	// This field may hold a maximum of two entries (dual-stack families, in
	// either order).  These families must correspond to the values of the
	// clusterIPs field, if specified. Both clusterIPs and ipFamilies are
	// governed by the ipFamilyPolicy field.
	// +listType=atomic
	// +optional
	IPFamilies []corev1.IPFamily `json:"ipFamilies,omitempty" protobuf:"bytes,19,opt,name=ipFamilies,casttype=IPFamily"`

	// IPFamilyPolicy represents the dual-stack-ness requested or required by
	// this Service. If there is no value provided, then this field will be set
	// to SingleStack. Services can be "SingleStack" (a single IP family),
	// "PreferDualStack" (two IP families on dual-stack configured clusters or
	// a single IP family on single-stack clusters), or "RequireDualStack"
	// (two IP families on dual-stack configured clusters, otherwise fail). The
	// ipFamilies and clusterIPs fields depend on the value of this field. This
	// field will be wiped when updating a service to type ExternalName.
	// +optional
	IPFamilyPolicy *corev1.IPFamilyPolicy `json:"ipFamilyPolicy,omitempty" protobuf:"bytes,17,opt,name=ipFamilyPolicy,casttype=IPFamilyPolicy"`
}

type RestoreFromSpec struct {
	// Refers to the backup name and component name used for restoration. Supports recovery of multiple components.
	// +optional
	Backup []BackupRefSpec `json:"backup,omitempty"`

	// Refers to the specific point in time for recovery.
	// +optional
	PointInTime *PointInTimeRefSpec `json:"pointInTime,omitempty"`
}

type RefNamespaceName struct {
	// Refers to the specific name of the resource.
	// +optional
	Name string `json:"name,omitempty"`

	// Refers to the specific namespace of the resource.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

type BackupRefSpec struct {
	// Refers to a reference backup that needs to be restored.
	// +optional
	Ref RefNamespaceName `json:"ref,omitempty"`
}

type PointInTimeRefSpec struct {
	// Refers to the specific time point for restoration, with UTC as the time zone.
	// +optional
	Time *metav1.Time `json:"time,omitempty"`

	// Refers to a reference source cluster that needs to be restored.
	// +optional
	Ref RefNamespaceName `json:"ref,omitempty"`
}

// ScriptSpec is designed to execute specific operations such as creating a database or user.
// It is not a general-purpose script executor and is applicable for engines like MySQL, PostgreSQL, Redis, MongoDB, etc.
type ScriptSpec struct {
	ComponentOps `json:",inline"`
	// Specifies the image to be used for the exec command. By default, the image of kubeblocks-datascript is used.
	// +optional
	Image string `json:"image,omitempty"`

	// Defines the secret to be used to execute the script. If not specified, the default cluster root credential secret is used.
	// +optional
	Secret *ScriptSecret `json:"secret,omitempty"`

	// Defines the script to be executed.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.script"
	Script []string `json:"script,omitempty"`

	// Defines the script to be executed from a configMap or secret.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.scriptFrom"
	ScriptFrom *ScriptFrom `json:"scriptFrom,omitempty"`

	// By default, KubeBlocks will execute the script on the primary pod with role=leader.
	// Exceptions exist, such as Redis, which does not synchronize account information between primary and secondary.
	// In such cases, the script needs to be executed on all pods matching the selector.
	// Indicates the components on which the script is executed.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.script.selector"
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

type BackupSpec struct {
	// Specifies the name of the backup.
	// +optional
	BackupName string `json:"backupName,omitempty"`

	// Indicates the backupPolicy applied to perform this backup.
	// +optional
	BackupPolicyName string `json:"backupPolicyName,omitempty"`

	// Defines the backup method that is defined in backupPolicy.
	// +optional
	BackupMethod string `json:"backupMethod,omitempty"`

	// Determines whether the backup contents stored in backup repository
	// should be deleted when the backup custom resource is deleted.
	// Supported values are `Retain` and `Delete`.
	// - `Retain` means that the backup content and its physical snapshot on backup repository are kept.
	// - `Delete` means that the backup content and its physical snapshot on backup repository are deleted.
	// +kubebuilder:validation:Enum=Delete;Retain
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Delete
	// +optional
	DeletionPolicy string `json:"deletionPolicy,omitempty"`

	// Determines a duration up to which the backup should be kept.
	// Controller will remove all backups that are older than the RetentionPeriod.
	// For example, RetentionPeriod of `30d` will keep only the backups of last 30 days.
	// Sample duration format:
	//
	// - years: 2y
	// - months: 6mo
	// - days: 30d
	// - hours: 12h
	// - minutes: 30m
	//
	// You can also combine the above durations. For example: 30d12h30m.
	// If not set, the backup will be kept forever.
	// +optional
	RetentionPeriod string `json:"retentionPeriod,omitempty"`

	// If backupType is incremental, parentBackupName is required.
	// +optional
	ParentBackupName string `json:"parentBackupName,omitempty"`
}

type RestoreSpec struct {
	// Specifies the name of the backup.
	// +kubebuilder:validation:Required
	BackupName string `json:"backupName"`

	// Indicates if this backup will be restored for all components which refer to common ComponentDefinition.
	EffectiveCommonComponentDef bool `json:"effectiveCommonComponentDef,omitempty"`

	// Defines the point in time to restore. The restoreTimeStr parameter represents the time string to be formatted and validated. If the restoreTimeStr is empty, it is returned as is without any formatting or validation.
	//The function follows a specific time format/layout constraint for the restoreTimeStr API,
	//which is "Jan 02, 2006 15:04:05 UTC-0700". The restoreTimeStr is formatted using the RFC3339 format, after parsing this string into a time.Time object.
	//Example usage:
	// Define a restore time string
	// restoreTimeStr := "Jan 02,2006 15:04:05 UTC-0700"
	// Use the function
	// formattedTime, err := FormatRestoreTimeAndValidate(restoreTimeStr, backup)
	// if err != nil {
	//     fmt.Println("Error:", err)
	// }
	// fmt.Println("Formatted time:", formattedTime)
	// Parameters:
	//   - restoreTimeStr: A string representing a time in the given format.
	//   - backup: A pointer to the backup object that contains the time range
	//     for validation.
	// Returns:
	//   - string: The formatted restore time string.
	//   - error: An error if the restore time string cannot be parsed or is outside the time range.
	RestoreTimeStr string `json:"restoreTimeStr,omitempty"`

	// Specifies the volume claim restore policy, support values: [Serial, Parallel]
	// +kubebuilder:validation:Enum=Serial;Parallel
	// +kubebuilder:default=Parallel
	VolumeRestorePolicy string `json:"volumeRestorePolicy,omitempty"`
}

// ScriptSecret represents the secret that is used to execute the script.
type ScriptSecret struct {
	// Specifies the name of the secret.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`
	// Used to specify the username part of the secret.
	// +kubebuilder:default:="username"
	// +optional
	UsernameKey string `json:"usernameKey,omitempty"`
	// Used to specify the password part of the secret.
	// +kubebuilder:default:="password"
	// +optional
	PasswordKey string `json:"passwordKey,omitempty"`
}

// ScriptFrom represents the script that is to be executed from a configMap or a secret.
type ScriptFrom struct {
	// Specifies the configMap that is to be executed.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.scriptFrom.configMapRef"
	ConfigMapRef []corev1.ConfigMapKeySelector `json:"configMapRef,omitempty"`
	// Specifies the secret that is to be executed.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.scriptFrom.secretRef"
	SecretRef []corev1.SecretKeySelector `json:"secretRef,omitempty"`
}

// OpsRequestStatus represents the observed state of an OpsRequest.
type OpsRequestStatus struct {

	// Specifies the cluster generation after the OpsRequest action has been handled.
	// +optional
	ClusterGeneration int64 `json:"clusterGeneration,omitempty"`

	// Defines the phase of the OpsRequest.
	Phase OpsPhase `json:"phase,omitempty"`

	// Represents the progress of the OpsRequest.
	// +kubebuilder:validation:Pattern:=`^(\d+|\-)/(\d+|\-)$`
	// +kubebuilder:default=-/-
	Progress string `json:"progress"`

	// Records the last configuration before this operation took effect.
	// +optional
	LastConfiguration LastConfiguration `json:"lastConfiguration,omitempty"`

	// Records the status information of components changed due to the operation request.
	// +optional
	Components map[string]OpsRequestComponentStatus `json:"components,omitempty"`

	// A collection of additional key-value pairs that provide supplementary information for the opsRequest.
	Extras []map[string]string `json:"extras,omitempty"`

	// Indicates the time when the OpsRequest started processing.
	// +optional
	StartTimestamp metav1.Time `json:"startTimestamp,omitempty"`

	// Specifies the time when the OpsRequest was completed.
	// +optional
	CompletionTimestamp metav1.Time `json:"completionTimestamp,omitempty"`

	// Defines the time when the OpsRequest was cancelled.
	// +optional
	CancelTimestamp metav1.Time `json:"cancelTimestamp,omitempty"`

	// Deprecated: Replaced by ReconfiguringStatusAsComponent.
	// Defines the status information of reconfiguring.
	// +optional
	ReconfiguringStatus *ReconfiguringStatus `json:"reconfiguringStatus,omitempty"`

	// Represents the status information of reconfiguring.
	// +optional
	ReconfiguringStatusAsComponent map[string]*ReconfiguringStatus `json:"reconfiguringStatusAsComponent,omitempty"`

	// Describes the detailed status of the OpsRequest.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="has(self.objectKey) || has(self.actionName)", message="either objectKey and actionName."

type ProgressStatusDetail struct {
	// Specifies the group to which the current object belongs.
	// If the objects of a component belong to the same group, they can be ignored.
	// +optional
	Group string `json:"group,omitempty"`

	// Represents the unique key of the object.
	// either objectKey or actionName.
	// +optional
	ObjectKey string `json:"objectKey,omitempty"`

	// Refer to the action name of the OpsDefinition.spec.actions[*].name.
	// either objectKey or actionName.
	// +optional
	ActionName string `json:"actionName,omitempty"`

	// Records the tasks associated with an action. such as Jobs/Pods that executes action.
	// +optional
	ActionTasks []ActionTask `json:"actionTasks,omitempty"`

	// Indicates the state of processing the object.
	// +kubebuilder:validation:Required
	Status ProgressStatus `json:"status"`

	// Provides a human-readable message detailing the condition of the object.
	// +optional
	Message string `json:"message,omitempty"`

	// Represents the start time of object processing.
	// +optional
	StartTime metav1.Time `json:"startTime,omitempty"`

	// Represents the completion time of object processing.
	// +optional
	EndTime metav1.Time `json:"endTime,omitempty"`
}

type ActionTask struct {
	// Specifies the name of the task workload.
	// +kubebuilder:validation:Required
	ObjectKey string `json:"objectKey"`

	// Defines the namespace where the task workload is deployed.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// Indicates the current status of the task.
	// +kubebuilder:validation:Required
	Status ActionTaskStatus `json:"status"`

	// The name of the target pod for the task.
	// +optional
	TargetPodName string `json:"targetPodName,omitempty"`

	// The number of retry attempts for this task.
	// +optional
	Retries int32 `json:"retries,omitempty"`
}

type LastComponentConfiguration struct {
	// Represents the last replicas of the component.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Represents the last resources of the component.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	corev1.ResourceRequirements `json:",inline,omitempty"`

	// Records the last volumeClaimTemplates of the component.
	// +optional
	VolumeClaimTemplates []OpsRequestVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty"`

	// Records the last services of the component.
	// +optional
	Services []ClusterComponentService `json:"services,omitempty"`

	// Records the information about the target resources affected by the component.
	// The resource key is in the list of [pods].
	// +optional
	TargetResources map[ComponentResourceKey][]string `json:"targetResources,omitempty"`

	// Records the last instances of the component.
	// +optional
	Instances *[]InstanceTemplate `json:"instances,omitempty"`

	// Records the last offline instances of the component.
	// +optional
	OfflineInstances *[]string `json:"offlineInstances,omitempty"`
}

type LastConfiguration struct {
	// Specifies the reference to the ClusterVersion name.
	// +optional
	ClusterVersionRef string `json:"clusterVersionRef,omitempty"`

	// Records the last configuration of the component.
	// +optional
	Components map[string]LastComponentConfiguration `json:"components,omitempty"`
}

type OpsRequestComponentStatus struct {
	// Describes the component phase, referencing Cluster.status.component.phase.
	// +optional
	Phase ClusterComponentPhase `json:"phase,omitempty"`

	// Indicates the last time the component phase transitioned to Failed or Abnormal.
	// +optional
	LastFailedTime metav1.Time `json:"lastFailedTime,omitempty"`

	// Specifies the outcome of the preConditions check for the opsRequest. This result is crucial for determining the next steps in the operation.
	// +optional
	PreCheckResult *PreCheckResult `json:"preCheck,omitempty"`

	// Describes the progress details of the component for this operation.
	// +optional
	ProgressDetails []ProgressStatusDetail `json:"progressDetails,omitempty"`

	// References the workload type of component in ClusterDefinition.
	// +optional
	WorkloadType WorkloadType `json:"workloadType,omitempty"`

	// Describes the configuration covered by the latest OpsRequest of the same kind.
	// when reconciling, this information will be used as a benchmark rather than the 'spec', such as 'Spec.HorizontalScaling'.
	// +optional
	OverrideBy *OverrideBy `json:"overrideBy,omitempty"`

	// Describes the reason for the component phase.
	// +kubebuilder:validation:MaxLength=1024
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`

	// Provides a human-readable message indicating details about this operation.
	// +kubebuilder:validation:MaxLength=32768
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
}

type OverrideBy struct {
	// Indicates the opsRequest name.
	// +optional
	OpsName string `json:"opsName"`

	LastComponentConfiguration `json:",inline"`
}

type PreCheckResult struct {
	// Indicates whether the preCheck operation was successful or not.
	// +kubebuilder:validation:Required
	Pass bool `json:"pass"`

	// Provides additional details about the preCheck operation in a human-readable format.
	// +optional
	Message string `json:"message,omitempty"`
}

type ReconfiguringStatus struct {
	// Describes the reconfiguring detail status.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Describes the status of the component reconfiguring.
	// +kubebuilder:validation:Required
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ConfigurationStatus []ConfigurationItemStatus `json:"configurationStatus"`
}

type ConfigurationItemStatus struct {
	// Specifies the name of the configuration template.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Defines the policy for reconfiguration.
	// +optional
	UpdatePolicy UpgradePolicy `json:"updatePolicy,omitempty"`

	// Indicates the current state of the reconfiguration state machine.
	// +optional
	Status string `json:"status,omitempty"`

	// Provides details about the operation.
	// +optional
	Message string `json:"message,omitempty"`

	// Counts the number of successful reconfigurations.
	// +kubebuilder:default=0
	// +optional
	SucceedCount int32 `json:"succeedCount"`

	// Specifies the number of expected reconfigurations.
	// +kubebuilder:default=-1
	// +optional
	ExpectedCount int32 `json:"expectedCount"`

	// Records the last status of the reconfiguration controller.
	// +optional
	LastAppliedStatus string `json:"lastStatus,omitempty"`

	// Stores the last applied configuration.
	// +optional
	LastAppliedConfiguration map[string]string `json:"lastAppliedConfiguration,omitempty"`

	// Contains the updated parameters.
	// +optional
	UpdatedParameters UpdatedParameters `json:"updatedParameters"`
}

type UpdatedParameters struct {
	// Lists the keys that have been added.
	// +optional
	AddedKeys map[string]string `json:"addedKeys,omitempty"`

	// Lists the keys that have been deleted.
	// +optional
	DeletedKeys map[string]string `json:"deletedKeys,omitempty"`

	// Lists the keys that have been updated.
	// +optional
	UpdatedKeys map[string]string `json:"updatedKeys,omitempty"`
}

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all},shortName=ops
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.type",description="Operation request type."
// +kubebuilder:printcolumn:name="CLUSTER",type="string",JSONPath=".spec.clusterRef",description="Operand cluster."
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="Operation status phase."
// +kubebuilder:printcolumn:name="PROGRESS",type="string",JSONPath=".status.progress",description="Operation processing progress."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// OpsRequest is the Schema for the opsrequests API
type OpsRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpsRequestSpec   `json:"spec,omitempty"`
	Status OpsRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpsRequestList contains a list of OpsRequest
type OpsRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpsRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpsRequest{}, &OpsRequestList{})
}

// GetRestartComponentNameSet gets the component name map with restart operation.
func (r OpsRequestSpec) GetRestartComponentNameSet() ComponentNameSet {
	set := make(ComponentNameSet)
	for _, v := range r.RestartList {
		set[v.ComponentName] = struct{}{}
	}
	return set
}

// GetSwitchoverComponentNameSet gets the component name map with switchover operation.
func (r OpsRequestSpec) GetSwitchoverComponentNameSet() ComponentNameSet {
	set := make(ComponentNameSet)
	for _, v := range r.SwitchoverList {
		set[v.ComponentName] = struct{}{}
	}
	return set
}

// GetVerticalScalingComponentNameSet gets the component name map with vertical scaling operation.
func (r OpsRequestSpec) GetVerticalScalingComponentNameSet() ComponentNameSet {
	set := make(ComponentNameSet)
	for _, v := range r.VerticalScalingList {
		set[v.ComponentName] = struct{}{}
	}
	return set
}

// ToVerticalScalingListToMap converts OpsRequest.spec.verticalScaling list to map
func (r OpsRequestSpec) ToVerticalScalingListToMap() map[string]VerticalScaling {
	verticalScalingMap := make(map[string]VerticalScaling)
	for _, v := range r.VerticalScalingList {
		verticalScalingMap[v.ComponentName] = v
	}
	return verticalScalingMap
}

// GetHorizontalScalingComponentNameSet gets the component name map with horizontal scaling operation.
func (r OpsRequestSpec) GetHorizontalScalingComponentNameSet() ComponentNameSet {
	set := make(ComponentNameSet)
	for _, v := range r.HorizontalScalingList {
		set[v.ComponentName] = struct{}{}
	}
	return set
}

// ToHorizontalScalingListToMap converts OpsRequest.spec.horizontalScaling list to map
func (r OpsRequestSpec) ToHorizontalScalingListToMap() map[string]HorizontalScaling {
	verticalScalingMap := make(map[string]HorizontalScaling)
	for _, v := range r.HorizontalScalingList {
		verticalScalingMap[v.ComponentName] = v
	}
	return verticalScalingMap
}

// GetVolumeExpansionComponentNameSet gets the component name map with volume expansion operation.
func (r OpsRequestSpec) GetVolumeExpansionComponentNameSet() ComponentNameSet {
	set := make(ComponentNameSet)
	for _, v := range r.VolumeExpansionList {
		set[v.ComponentName] = struct{}{}
	}
	return set
}

// GetDataScriptComponentNameSet gets the component name map with switchover operation.
func (r OpsRequestSpec) GetDataScriptComponentNameSet() ComponentNameSet {
	set := make(ComponentNameSet)
	set[r.ScriptSpec.ComponentName] = struct{}{}
	return set
}

// ToVolumeExpansionListToMap converts volumeExpansionList to map
func (r OpsRequestSpec) ToVolumeExpansionListToMap() map[string]VolumeExpansion {
	volumeExpansionMap := make(map[string]VolumeExpansion)
	for _, v := range r.VolumeExpansionList {
		volumeExpansionMap[v.ComponentName] = v
	}
	return volumeExpansionMap
}

// ToExposeListToMap build expose map
func (r OpsRequestSpec) ToExposeListToMap() map[string]Expose {
	exposeMap := make(map[string]Expose)
	for _, v := range r.ExposeList {
		exposeMap[v.ComponentName] = v
	}
	return exposeMap
}

// GetReconfiguringComponentNameSet gets the component name map with reconfiguring operation.
func (r OpsRequestSpec) GetReconfiguringComponentNameSet() ComponentNameSet {
	if r.Reconfigure == nil {
		return nil
	}
	return ComponentNameSet{
		r.Reconfigure.ComponentName: {},
	}
}

func (r OpsRequestSpec) GetExposeComponentNameSet() ComponentNameSet {
	set := make(ComponentNameSet)
	for _, v := range r.ExposeList {
		set[v.ComponentName] = struct{}{}
	}
	return set
}

// GetUpgradeComponentNameSet gets the component name map with upgrade operation.
func (r *OpsRequest) GetUpgradeComponentNameSet() ComponentNameSet {
	if r == nil || r.Spec.Upgrade == nil {
		return nil
	}
	set := make(ComponentNameSet)
	for k := range r.Status.Components {
		set[k] = struct{}{}
	}
	return set
}

// GetComponentNameSet if the operations are within the scope of component, this function should be implemented
func (r *OpsRequest) GetComponentNameSet() ComponentNameSet {
	switch r.Spec.Type {
	case RestartType:
		return r.Spec.GetRestartComponentNameSet()
	case VerticalScalingType:
		return r.Spec.GetVerticalScalingComponentNameSet()
	case HorizontalScalingType:
		return r.Spec.GetHorizontalScalingComponentNameSet()
	case VolumeExpansionType:
		return r.Spec.GetVolumeExpansionComponentNameSet()
	case UpgradeType:
		return r.GetUpgradeComponentNameSet()
	case ReconfiguringType:
		return r.Spec.GetReconfiguringComponentNameSet()
	case ExposeType:
		return r.Spec.GetExposeComponentNameSet()
	case SwitchoverType:
		return r.Spec.GetSwitchoverComponentNameSet()
	case DataScriptType:
		return r.Spec.GetDataScriptComponentNameSet()
	default:
		return nil
	}
}

func (p *ProgressStatusDetail) SetStatusAndMessage(status ProgressStatus, message string) {
	p.Message = message
	p.Status = status
}
