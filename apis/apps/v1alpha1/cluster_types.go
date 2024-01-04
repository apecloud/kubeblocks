/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// ClusterSpec defines the desired state of Cluster.
type ClusterSpec struct {
	// Cluster referencing ClusterDefinition name. This is an immutable attribute.
	// If ClusterDefRef is not specified, ComponentDef must be specified for each Component in ComponentSpecs.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="clusterDefinitionRef is immutable"
	// +optional
	ClusterDefRef string `json:"clusterDefinitionRef,omitempty"`

	// Cluster referencing ClusterVersion name.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	ClusterVersionRef string `json:"clusterVersionRef,omitempty"`

	// Cluster termination policy. Valid values are DoNotTerminate, Halt, Delete, WipeOut.
	// DoNotTerminate will block delete operation.
	// Halt will delete workload resources such as statefulset, deployment workloads but keep PVCs.
	// Delete is based on Halt and deletes PVCs.
	// WipeOut is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.
	// +kubebuilder:validation:Required
	TerminationPolicy TerminationPolicyType `json:"terminationPolicy"`

	// List of componentSpecs you want to replace in ClusterDefinition and ClusterVersion. It will replace the field in ClusterDefinition's and ClusterVersion's component if type is matching.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:XValidation:rule="self.all(x, size(self.filter(c, c.name == x.name)) == 1)",message="duplicated component"
	ComponentSpecs []ClusterComponentSpec `json:"componentSpecs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// services defines the services to access a cluster.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Services []ClusterService `json:"services,omitempty"`

	// affinity is a group of affinity scheduling rules.
	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`

	// tolerations are attached to tolerate any taint that matches the triple `key,value,effect` using the matching operator `operator`.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// tenancy describes how pods are distributed across node.
	// SharedNode means multiple pods may share the same node.
	// DedicatedNode means each pod runs on their own dedicated node.
	// +optional
	Tenancy TenancyType `json:"tenancy,omitempty"`

	// availabilityPolicy describes the availability policy, including zone, node, and none.
	// +optional
	AvailabilityPolicy AvailabilityPolicyType `json:"availabilityPolicy,omitempty"`

	// replicas specifies the replicas of the first componentSpec, if the replicas of the first componentSpec is specified, this value will be ignored.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// resources specifies the resources of the first componentSpec, if the resources of the first componentSpec is specified, this value will be ignored.
	// +optional
	Resources ClusterResources `json:"resources,omitempty"`

	// storage specifies the storage of the first componentSpec, if the storage of the first componentSpec is specified, this value will be ignored.
	// +optional
	Storage ClusterStorage `json:"storage,omitempty"`

	// monitor specifies the configuration of monitor
	// +optional
	Monitor ClusterMonitor `json:"monitor,omitempty"`

	// network specifies the configuration of network
	// +optional
	Network *ClusterNetwork `json:"network,omitempty"`

	// cluster backup configuration.
	// +optional
	Backup *ClusterBackup `json:"backup,omitempty"`
}

type ClusterBackup struct {
	// enabled defines whether to enable automated backup.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// retentionPeriod determines a duration up to which the backup should be kept.
	// controller will remove all backups that are older than the RetentionPeriod.
	// For example, RetentionPeriod of `30d` will keep only the backups of last 30 days.
	// Sample duration format:
	// - years: 	2y
	// - months: 	6mo
	// - days: 		30d
	// - hours: 	12h
	// - minutes: 	30m
	// You can also combine the above durations. For example: 30d12h30m
	// +kubebuilder:default="7d"
	// +optional
	RetentionPeriod dpv1alpha1.RetentionPeriod `json:"retentionPeriod,omitempty"`

	// backup method name to use, that is defined in backupPolicy.
	// +optional
	Method string `json:"method"`

	// the cron expression for schedule, the timezone is in UTC. see https://en.wikipedia.org/wiki/Cron.
	// +optional
	CronExpression string `json:"cronExpression,omitempty"`

	// startingDeadlineMinutes defines the deadline in minutes for starting the backup job
	// if it misses scheduled time for any reason.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1440
	// +optional
	StartingDeadlineMinutes *int64 `json:"startingDeadlineMinutes,omitempty"`

	// repoName is the name of the backupRepo, if not set, will use the default backupRepo.
	// +optional
	RepoName string `json:"repoName,omitempty"`

	// pitrEnabled defines whether to enable point-in-time recovery.
	// +kubebuilder:default=false
	// +optional
	PITREnabled *bool `json:"pitrEnabled,omitempty"`
}

type ClusterResources struct {

	// cpu resource needed, more info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	CPU resource.Quantity `json:"cpu,omitempty"`

	// memory resource needed, more info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Memory resource.Quantity `json:"memory,omitempty"`
}

type ClusterStorage struct {

	// storage size needed, more info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Size resource.Quantity `json:"size,omitempty"`
}

type ResourceMeta struct {
	// name is the name of the referenced the Configmap/Secret object.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// mountPath is the path at which to mount the volume.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern:=`^/[a-z]([a-z0-9\-]*[a-z0-9])?$`
	MountPoint string `json:"mountPoint"`

	// subPath is a relative file path within the volume to mount.
	// +optional
	SubPath string `json:"subPath,omitempty"`

	// asVolumeFrom defines the list of containers where volumeMounts will be injected into.
	// +listType=set
	// +optional
	AsVolumeFrom []string `json:"asVolumeFrom,omitempty"`
}

type SecretRef struct {
	ResourceMeta `json:",inline"`

	// secret defines the secret volume source.
	// +kubebuilder:validation:Required
	Secret corev1.SecretVolumeSource `json:"secret"`
}

type ConfigMapRef struct {
	ResourceMeta `json:",inline"`

	// configMap defines the configmap volume source.
	// +kubebuilder:validation:Required
	ConfigMap corev1.ConfigMapVolumeSource `json:"configMap"`
}

type UserResourceRefs struct {
	// secretRefs defines the user-defined secrets.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	SecretRefs []SecretRef `json:"secretRefs,omitempty"`

	// configMapRefs defines the user-defined configmaps.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ConfigMapRefs []ConfigMapRef `json:"configMapRefs,omitempty"`
}

// ClusterStatus defines the observed state of Cluster.
type ClusterStatus struct {
	// observedGeneration is the most recent generation observed for this
	// Cluster. It corresponds to the Cluster's generation, which is
	// updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// phase describes the phase of the Cluster, the detail information of the phases are as following:
	// Creating: all components are in `Creating` phase.
	// Running: all components are in `Running` phase, means the cluster is working well.
	// Updating: all components are in `Creating`, `Running` or `Updating` phase,
	// and at least one component is in `Creating` or `Updating` phase, means the cluster is doing an update.
	// Stopping: at least one component is in `Stopping` phase, means the cluster is in a stop progress.
	// Stopped: all components are in 'Stopped` phase, means the cluster has stopped and didn't provide any function anymore.
	// Failed: all components are in `Failed` phase, means the cluster is unavailable.
	// Abnormal: some components are in `Failed` or `Abnormal` phase, means the cluster in a fragile state. troubleshoot need to be done.
	// Deleting: the cluster is being deleted.
	// +optional
	Phase ClusterPhase `json:"phase,omitempty"`

	// message describes cluster details message in current phase.
	// +optional
	Message string `json:"message,omitempty"`

	// components record the current status information of all components of the cluster.
	// +optional
	Components map[string]ClusterComponentStatus `json:"components,omitempty"`

	// clusterDefGeneration represents the generation number of ClusterDefinition referenced.
	// +optional
	ClusterDefGeneration int64 `json:"clusterDefGeneration,omitempty"`

	// Describe current state of cluster API Resource, like warning.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ClusterComponentSpec defines the cluster component spec.
// +kubebuilder:validation:XValidation:rule="has(self.componentDefRef) || has(self.componentDef)",message="either componentDefRef or componentDef should be provided"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.componentDefRef) || has(self.componentDefRef)", message="componentDefRef is required once set"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.componentDef) || has(self.componentDef)", message="componentDef is required once set"
type ClusterComponentSpec struct {
	// name defines cluster's component name, this name is also part of Service DNS name, so this name will
	// comply with IANA Service Naming rule.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`

	// componentDefRef references componentDef defined in ClusterDefinition spec. Need to
	// comply with IANA Service Naming rule.
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="componentDefRef is immutable"
	// +optional
	ComponentDefRef string `json:"componentDefRef,omitempty"`

	// componentDef references the name of the ComponentDefinition.
	// If both componentDefRef and componentDef are provided, the componentDef will take precedence over componentDefRef.
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="componentDef is immutable"
	// +optional
	ComponentDef string `json:"componentDef,omitempty"`

	// classDefRef references the class defined in ComponentClassDefinition.
	// +optional
	ClassDefRef *ClassDefRef `json:"classDefRef,omitempty"`

	// serviceRefs define service references for the current component. Based on the referenced services, they can be categorized into two types:
	// Service provided by external sources: These services are provided by external sources and are not managed by KubeBlocks. They can be Kubernetes-based or non-Kubernetes services. For external services, you need to provide an additional ServiceDescriptor object to establish the service binding.
	// Service provided by other KubeBlocks clusters: These services are provided by other KubeBlocks clusters. You can bind to these services by specifying the name of the hosting cluster.
	// Each type of service reference requires specific configurations and bindings to establish the connection and interaction with the respective services.
	// It should be noted that the ServiceRef has cluster-level semantic consistency, meaning that within the same Cluster, service references with the same ServiceRef.Name are considered to be the same service. It is only allowed to bind to the same Cluster or ServiceDescriptor.
	// +optional
	ServiceRefs []ServiceRef `json:"serviceRefs,omitempty"`

	// monitor is a switch to enable monitoring and is set as false by default.
	// KubeBlocks provides an extension mechanism to support component level monitoring,
	// which will scrape metrics auto or manually from servers in component and export
	// metrics to Time Series Database.
	// +kubebuilder:default=false
	// +optional
	Monitor bool `json:"monitor,omitempty"`

	// enabledLogs indicates which log file takes effect in the database cluster.
	// element is the log type which is defined in cluster definition logConfig.name,
	// and will set relative variables about this log type in database kernel.
	// +listType=set
	// +optional
	EnabledLogs []string `json:"enabledLogs,omitempty"`

	// Component replicas.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// affinity describes affinities specified by users.
	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`

	// Component tolerations will override ClusterSpec.Tolerations if specified.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Resources requests and limits of workload.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// volumeClaimTemplates information for statefulset.spec.volumeClaimTemplates.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	VolumeClaimTemplates []ClusterComponentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Services expose endpoints that can be accessed by clients.
	// +optional
	Services []ClusterComponentService `json:"services,omitempty"`

	// switchPolicy defines the strategy for switchover and failover when workloadType is Replication.
	// +optional
	SwitchPolicy *ClusterSwitchPolicy `json:"switchPolicy,omitempty"`

	// Enables or disables TLS certs.
	// +optional
	TLS bool `json:"tls,omitempty"`

	// issuer defines provider context for TLS certs.
	// required when TLS enabled
	// +optional
	Issuer *Issuer `json:"issuer,omitempty"`

	// serviceAccountName is the name of the ServiceAccount that running component depends on.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// noCreatePDB defines the PodDisruptionBudget creation behavior and is set to true if creation of PodDisruptionBudget
	// for this component is not needed. It defaults to false.
	// +kubebuilder:default=false
	// +optional
	NoCreatePDB bool `json:"noCreatePDB,omitempty"`

	// updateStrategy defines the update strategy for the component.
	// +optional
	UpdateStrategy *UpdateStrategy `json:"updateStrategy,omitempty"`

	// userResourceRefs defines the user-defined volumes.
	// +optional
	UserResourceRefs *UserResourceRefs `json:"userResourceRefs,omitempty"`

	// RsmTransformPolicy defines the policy generate sts using rsm.
	// ToSts: rsm transforms to statefulSet
	// ToPod: rsm transforms to pods
	// +kubebuilder:validation:Required
	// +kubebuilder:default=ToSts
	// +optional
	RsmTransformPolicy workloads.RsmTransformPolicy `json:"rsmTransformPolicy,omitempty"`

	// Nodes defines the list of nodes that pods can schedule
	// If the RsmTransformPolicy is specified as ToPod,the list of nodes will be used. If the list of nodes is empty,
	// no specific node will be assigned. However, if the list of node is filled, all pods will be evenly scheduled
	// across the nodes in the list.
	// +optional
	Nodes []types.NodeName `json:"nodes,omitempty"`

	// Instances defines the list of instance to be deleted priorly
	// If the RsmTransformPolicy is specified as ToPod,the list of instances will be used.
	// +optional
	Instances []string `json:"instances,omitempty"`
}

// GetMinAvailable wraps the 'prefer' value return. As for component replicaCount <= 1, it will return 0,
// and as for replicaCount=2 it will return 1.
func (r *ClusterComponentSpec) GetMinAvailable(prefer *intstr.IntOrString) *intstr.IntOrString {
	if r == nil || r.NoCreatePDB || prefer == nil {
		return nil
	}
	if r.Replicas <= 1 {
		m := intstr.FromInt(0)
		return &m
	} else if r.Replicas == 2 {
		m := intstr.FromInt(1)
		return &m
	}
	return prefer
}

type ComponentMessageMap map[string]string

// ClusterComponentStatus records components status.
type ClusterComponentStatus struct {
	// phase describes the phase of the component and the detail information of the phases are as following:
	// Creating: `Creating` is a special `Updating` with previous phase `empty`(means "") or `Creating`.
	// Running: component replicas > 0 and all pod specs are latest with a Running state.
	// Updating: component replicas > 0 and has no failed pods. the component is being updated.
	// Abnormal: component replicas > 0 but having some failed pods. the component basically works but in a fragile state.
	// Failed: component replicas > 0 but having some failed pods. the component doesn't work anymore.
	// Stopping: component replicas = 0 and has terminating pods.
	// Stopped: component replicas = 0 and all pods have been deleted.
	// Deleting: the component is being deleted.
	Phase ClusterComponentPhase `json:"phase,omitempty"`

	// message records the component details message in current phase.
	// Keys are podName or deployName or statefulSetName. The format is `ObjectKind/Name`.
	// +optional
	Message ComponentMessageMap `json:"message,omitempty"`

	// podsReady checks if all pods of the component are ready.
	// +optional
	PodsReady *bool `json:"podsReady,omitempty"`

	// podsReadyTime what time point of all component pods are ready,
	// this time is the ready time of the last component pod.
	// +optional
	PodsReadyTime *metav1.Time `json:"podsReadyTime,omitempty"`

	// members' status.
	// +optional
	MembersStatus []workloads.MemberStatus `json:"membersStatus,omitempty"`
}

type ClusterSwitchPolicy struct {
	// TODO other attribute extensions

	// clusterSwitchPolicy defines type of the switchPolicy when workloadType is Replication.
	// MaximumAvailability: [WIP] when the primary is active, do switch if the synchronization delay = 0 in the user-defined lagProbe data delay detection logic, otherwise do not switch. The primary is down, switch immediately. It will be available in future versions.
	// MaximumDataProtection: [WIP] when the primary is active, do switch if synchronization delay = 0 in the user-defined lagProbe data lag detection logic, otherwise do not switch. If the primary is down, if it can be judged that the primary and secondary data are consistent, then do the switch, otherwise do not switch. It will be available in future versions.
	// Noop: KubeBlocks will not perform high-availability switching on components. Users need to implement HA by themselves or integrate open source HA solution.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Noop
	// +optional
	Type SwitchPolicyType `json:"type"`
}

type ClusterComponentVolumeClaimTemplate struct {
	// Reference `ClusterDefinition.spec.componentDefs.containers.volumeMounts.name`.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// spec defines the desired characteristics of a volume requested by a pod author.
	// +optional
	Spec PersistentVolumeClaimSpec `json:"spec,omitempty"`
}

func (r *ClusterComponentVolumeClaimTemplate) toVolumeClaimTemplate() corev1.PersistentVolumeClaimTemplate {
	return corev1.PersistentVolumeClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.Name,
		},
		Spec: r.Spec.ToV1PersistentVolumeClaimSpec(),
	}
}

type PersistentVolumeClaimSpec struct {
	// accessModes contains the desired access modes the volume should have.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty" protobuf:"bytes,1,rep,name=accessModes,casttype=PersistentVolumeAccessMode"`
	// resources represents the minimum resources the volume should have.
	// If RecoverVolumeExpansionFailure feature is enabled users are allowed to specify resource requirements
	// that are lower than previous value but must still be higher than capacity recorded in the
	// status field of the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
	// storageClassName is the name of the StorageClass required by the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty" protobuf:"bytes,5,opt,name=storageClassName"`
	// volumeMode defines what type of volume is required by the claim.
	// +optional
	VolumeMode *corev1.PersistentVolumeMode `json:"volumeMode,omitempty" protobuf:"bytes,6,opt,name=volumeMode,casttype=PersistentVolumeMode"`
}

// ToV1PersistentVolumeClaimSpec converts to corev1.PersistentVolumeClaimSpec.
func (r *PersistentVolumeClaimSpec) ToV1PersistentVolumeClaimSpec() corev1.PersistentVolumeClaimSpec {
	return corev1.PersistentVolumeClaimSpec{
		AccessModes:      r.AccessModes,
		Resources:        r.Resources,
		StorageClassName: r.getStorageClassName(viper.GetString(constant.CfgKeyDefaultStorageClass)),
		VolumeMode:       r.VolumeMode,
	}
}

// getStorageClassName returns PersistentVolumeClaimSpec.StorageClassName if a value is assigned; otherwise,
// it returns preferSC argument.
func (r *PersistentVolumeClaimSpec) getStorageClassName(preferSC string) *string {
	if r.StorageClassName != nil && *r.StorageClassName != "" {
		return r.StorageClassName
	}
	if preferSC != "" {
		return &preferSC
	}
	return nil
}

type Affinity struct {
	// podAntiAffinity describes the anti-affinity level of pods within a component.
	// Preferred means try spread pods by `TopologyKeys`.
	// Required means must spread pods by `TopologyKeys`.
	// +kubebuilder:default=Preferred
	// +optional
	PodAntiAffinity PodAntiAffinity `json:"podAntiAffinity,omitempty"`

	// topologyKey is the key of node labels.
	// Nodes that have a label with this key and identical values are considered to be in the same topology.
	// It's used as the topology domain for pod anti-affinity and pod spread constraint.
	// Some well-known label keys, such as "kubernetes.io/hostname" and "topology.kubernetes.io/zone"
	// are often used as TopologyKey, as well as any other custom label key.
	// +listType=set
	// +optional
	TopologyKeys []string `json:"topologyKeys,omitempty"`

	// nodeLabels describes that pods must be scheduled to the nodes with the specified node labels.
	// +optional
	NodeLabels map[string]string `json:"nodeLabels,omitempty"`

	// tenancy describes how pods are distributed across node.
	// SharedNode means multiple pods may share the same node.
	// DedicatedNode means each pod runs on their own dedicated node.
	// +kubebuilder:default=SharedNode
	// +optional
	Tenancy TenancyType `json:"tenancy,omitempty"`
}

type TLSConfig struct {
	// +kubebuilder:default=false
	// +optional
	Enable bool `json:"enable,omitempty"`

	// +optional
	Issuer *Issuer `json:"issuer,omitempty"`
}

// Issuer defines Tls certs issuer
type Issuer struct {
	// Name of issuer.
	// Options supported:
	// - KubeBlocks - Certificates signed by KubeBlocks Operator.
	// - UserProvided - User provided own CA-signed certificates.
	// +kubebuilder:validation:Enum={KubeBlocks, UserProvided}
	// +kubebuilder:default=KubeBlocks
	// +kubebuilder:validation:Required
	Name IssuerName `json:"name"`

	// secretRef. TLS certs Secret reference
	// required when from is UserProvided
	// +optional
	SecretRef *TLSSecretRef `json:"secretRef,omitempty"`
}

// TLSSecretRef defines Secret contains Tls certs
type TLSSecretRef struct {
	// Name of the Secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// CA cert key in Secret
	// +kubebuilder:validation:Required
	CA string `json:"ca"`

	// Cert key in Secret
	// +kubebuilder:validation:Required
	Cert string `json:"cert"`

	// Key of TLS private key in Secret
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

type ClusterComponentService struct {
	// Service name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=15
	Name string `json:"name"`

	// serviceType determines how the Service is exposed. Valid
	// options are ClusterIP, NodePort, and LoadBalancer.
	// "ClusterIP" allocates a cluster-internal IP address for load-balancing
	// to endpoints. Endpoints are determined by the selector or if that is not
	// specified, they are determined by manual construction of an Endpoints object or
	// EndpointSlice objects. If clusterIP is "None", no virtual IP is
	// allocated and the endpoints are published as a set of endpoints rather
	// than a virtual IP.
	// "NodePort" builds on ClusterIP and allocates a port on every node which
	// routes to the same endpoints as the clusterIP.
	// "LoadBalancer" builds on NodePort and creates an external load-balancer
	// (if supported in the current cloud) which routes to the same endpoints
	// as the clusterIP.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types.
	// +kubebuilder:default=ClusterIP
	// +kubebuilder:validation:Enum={ClusterIP,NodePort,LoadBalancer}
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// If ServiceType is LoadBalancer, cloud provider related parameters can be put here
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ClassDefRef struct {
	// Name refers to the name of the ComponentClassDefinition.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	Name string `json:"name,omitempty"`

	// Class refers to the name of the class that is defined in the ComponentClassDefinition.
	// +kubebuilder:validation:Required
	Class string `json:"class"`
}

type ClusterMonitor struct {

	// monitoringInterval specifies interval of monitoring, no monitor if set to 0
	// +kubebuilder:validation:XIntOrString
	// +optional
	MonitoringInterval *intstr.IntOrString `json:"monitoringInterval,omitempty"`
}

type ClusterNetwork struct {

	// hostNetworkAccessible specifies whether host network is accessible. It defaults to false
	// +kubebuilder:default=false
	// +optional
	HostNetworkAccessible bool `json:"hostNetworkAccessible,omitempty"`

	// publiclyAccessible specifies whether it is publicly accessible. It defaults to false
	// +kubebuilder:default=false
	// +optional
	PubliclyAccessible bool `json:"publiclyAccessible,omitempty"`
}

type ServiceRef struct {
	// name of the service reference declaration. references the serviceRefDeclaration name defined in clusterDefinition.componentDefs[*].serviceRefDeclarations[*].name
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// namespace defines the namespace of the referenced Cluster or the namespace of the referenced ServiceDescriptor object.
	// If not set, the referenced Cluster and ServiceDescriptor will be searched in the namespace of the current cluster by default.
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`

	// When referencing a service provided by other KubeBlocks cluster, you need to provide the name of the Cluster being referenced.
	// By default, when other KubeBlocks Cluster are referenced, the ClusterDefinition.spec.connectionCredential secret corresponding to the referenced Cluster will be used to bind to the current component.
	// Currently, if a KubeBlocks cluster is to be referenced, the connection credential secret should include and correspond to the following fields: endpoint, port, username, and password.
	// Under this referencing approach, the ServiceKind and ServiceVersion of service reference declaration defined in the ClusterDefinition will not be validated.
	// If both Cluster and ServiceDescriptor are specified, the Cluster takes precedence.
	// +optional
	Cluster string `json:"cluster,omitempty"`

	// serviceDescriptor defines the service descriptor of the service provided by external sources.
	// When referencing a service provided by external sources, you need to provide the ServiceDescriptor object name to establish the service binding.
	// And serviceDescriptor is the name of the ServiceDescriptor object, furthermore, the ServiceDescriptor.spec.serviceKind and ServiceDescriptor.spec.serviceVersion
	// should match clusterDefinition.componentDefs[*].serviceRefDeclarations[*].serviceRefDeclarationSpecs[*].serviceKind
	// and the regular expression defines in clusterDefinition.componentDefs[*].serviceRefDeclarations[*].serviceRefDeclarationSpecs[*].serviceVersion.
	// If both Cluster and ServiceDescriptor are specified, the Cluster takes precedence.
	// +optional
	ServiceDescriptor string `json:"serviceDescriptor,omitempty"`
}

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all}
// +kubebuilder:printcolumn:name="CLUSTER-DEFINITION",type="string",JSONPath=".spec.clusterDefinitionRef",description="ClusterDefinition referenced by cluster."
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.clusterVersionRef",description="Cluster Application Version."
// +kubebuilder:printcolumn:name="TERMINATION-POLICY",type="string",JSONPath=".spec.terminationPolicy",description="Cluster termination policy."
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="Cluster Status."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Cluster is the Schema for the clusters API.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster.
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}

func (r Cluster) IsDeleting() bool {
	if r.GetDeletionTimestamp().IsZero() {
		return false
	}
	return r.Spec.TerminationPolicy != DoNotTerminate
}

func (r Cluster) IsUpdating() bool {
	return r.Status.ObservedGeneration != r.Generation
}

func (r Cluster) IsStatusUpdating() bool {
	return !r.IsDeleting() && !r.IsUpdating()
}

// GetVolumeClaimNames gets all PVC names of component compName.
//
// r.Spec.GetComponentByName(compName).VolumeClaimTemplates[*].Name will be used if no claimNames provided
//
// nil return if:
// 1. component compName not found or
// 2. len(VolumeClaimTemplates)==0 or
// 3. any claimNames not found
func (r *Cluster) GetVolumeClaimNames(compName string, claimNames ...string) []string {
	if r == nil {
		return nil
	}
	comp := r.Spec.GetComponentByName(compName)
	if comp == nil {
		return nil
	}
	if len(comp.VolumeClaimTemplates) == 0 {
		return nil
	}
	if len(claimNames) == 0 {
		for _, template := range comp.VolumeClaimTemplates {
			claimNames = append(claimNames, template.Name)
		}
	}
	allExist := true
	for _, name := range claimNames {
		found := false
		for _, template := range comp.VolumeClaimTemplates {
			if template.Name == name {
				found = true
				break
			}
		}
		if !found {
			allExist = false
			break
		}
	}
	if !allExist {
		return nil
	}

	pvcNames := make([]string, 0)
	for _, claimName := range claimNames {
		for i := 0; i < int(comp.Replicas); i++ {
			pvcName := fmt.Sprintf("%s-%s-%s-%d", claimName, r.Name, compName, i)
			pvcNames = append(pvcNames, pvcName)
		}
	}
	return pvcNames
}

// GetComponentByName gets component by name.
func (r ClusterSpec) GetComponentByName(componentName string) *ClusterComponentSpec {
	for _, v := range r.ComponentSpecs {
		if v.Name == componentName {
			return &v
		}
	}
	return nil
}

// GetComponentDefRefName gets the name of referenced component definition.
func (r ClusterSpec) GetComponentDefRefName(componentName string) string {
	for _, component := range r.ComponentSpecs {
		if componentName == component.Name {
			return component.ComponentDefRef
		}
	}
	return ""
}

// ValidateEnabledLogs validates enabledLogs config in cluster.yaml, and returns metav1.Condition when detecting invalid values.
func (r ClusterSpec) ValidateEnabledLogs(cd *ClusterDefinition) error {
	message := make([]string, 0)
	for _, comp := range r.ComponentSpecs {
		invalidLogNames := cd.ValidateEnabledLogConfigs(comp.ComponentDefRef, comp.EnabledLogs)
		if len(invalidLogNames) == 0 {
			continue
		}
		message = append(message, fmt.Sprintf("EnabledLogs: %s are not defined in Component: %s of the clusterDefinition", invalidLogNames, comp.Name))
	}
	if len(message) > 0 {
		return errors.New(strings.Join(message, ";"))
	}
	return nil
}

// GetDefNameMappingComponents returns ComponentDefRef name mapping ClusterComponentSpec.
func (r ClusterSpec) GetDefNameMappingComponents() map[string][]ClusterComponentSpec {
	m := map[string][]ClusterComponentSpec{}
	for _, c := range r.ComponentSpecs {
		v := m[c.ComponentDefRef]
		v = append(v, c)
		m[c.ComponentDefRef] = v
	}
	return m
}

// GetMessage gets message map deep copy object.
func (r ClusterComponentStatus) GetMessage() ComponentMessageMap {
	messageMap := map[string]string{}
	for k, v := range r.Message {
		messageMap[k] = v
	}
	return messageMap
}

// SetMessage overrides message map object.
func (r *ClusterComponentStatus) SetMessage(messageMap ComponentMessageMap) {
	if r == nil {
		return
	}
	r.Message = messageMap
}

// SetObjectMessage sets K8s workload message to component status message map.
func (r *ClusterComponentStatus) SetObjectMessage(objectKind, objectName, message string) {
	if r == nil {
		return
	}
	if r.Message == nil {
		r.Message = map[string]string{}
	}
	messageKey := fmt.Sprintf("%s/%s", objectKind, objectName)
	r.Message[messageKey] = message
}

// GetObjectMessage gets the k8s workload message in component status message map
func (r ClusterComponentStatus) GetObjectMessage(objectKind, objectName string) string {
	messageKey := fmt.Sprintf("%s/%s", objectKind, objectName)
	return r.Message[messageKey]
}

// SetObjectMessage sets k8s workload message to component status message map
func (r ComponentMessageMap) SetObjectMessage(objectKind, objectName, message string) {
	if r == nil {
		return
	}
	messageKey := fmt.Sprintf("%s/%s", objectKind, objectName)
	r[messageKey] = message
}

// SetComponentStatus does safe operation on ClusterStatus.Components map object update.
func (r *ClusterStatus) SetComponentStatus(name string, status ClusterComponentStatus) {
	r.checkedInitComponentsMap()
	r.Components[name] = status
}

func (r *ClusterStatus) checkedInitComponentsMap() {
	if r.Components == nil {
		r.Components = map[string]ClusterComponentStatus{}
	}
}

// ToVolumeClaimTemplates convert r.VolumeClaimTemplates to []corev1.PersistentVolumeClaimTemplate.
func (r *ClusterComponentSpec) ToVolumeClaimTemplates() []corev1.PersistentVolumeClaimTemplate {
	if r == nil {
		return nil
	}
	var ts []corev1.PersistentVolumeClaimTemplate
	for _, t := range r.VolumeClaimTemplates {
		ts = append(ts, t.toVolumeClaimTemplate())
	}
	return ts
}

// GetClusterUpRunningPhases returns Cluster running or partially running phases.
func GetClusterUpRunningPhases() []ClusterPhase {
	return []ClusterPhase{
		RunningClusterPhase,
		AbnormalClusterPhase,
		FailedClusterPhase, // REVIEW/TODO: single component with single pod component are handled as FailedClusterPhase, ought to remove this.
	}
}

// GetReconfiguringRunningPhases return Cluster running or partially running phases.
func GetReconfiguringRunningPhases() []ClusterPhase {
	return []ClusterPhase{
		RunningClusterPhase,
		UpdatingClusterPhase, // enable partial running for reconfiguring
		AbnormalClusterPhase,
		FailedClusterPhase,
	}
}

// GetComponentTerminalPhases return Cluster's component terminal phases.
func GetComponentTerminalPhases() []ClusterComponentPhase {
	return []ClusterComponentPhase{
		RunningClusterCompPhase,
		StoppedClusterCompPhase,
		FailedClusterCompPhase,
		AbnormalClusterCompPhase,
	}
}

// GetComponentUpRunningPhase returns component running or partially running phases.
func GetComponentUpRunningPhase() []ClusterComponentPhase {
	return []ClusterComponentPhase{
		RunningClusterCompPhase,
		AbnormalClusterCompPhase,
		FailedClusterCompPhase,
	}
}

// ComponentPodsAreReady checks if the pods of component are ready.
func ComponentPodsAreReady(podsAreReady *bool) bool {
	return podsAreReady != nil && *podsAreReady
}
