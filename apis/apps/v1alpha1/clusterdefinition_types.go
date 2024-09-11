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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterDefinitionSpec defines the desired state of ClusterDefinition.
type ClusterDefinitionSpec struct {
	// Specifies the well-known database type, such as mysql, redis, or mongodb.
	//
	// Deprecated since v0.9.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:validation:MaxLength=24
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Type string `json:"type,omitempty"`

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

	// Specifies the exact name, name prefix, or regular expression pattern for matching the name of the ComponentDefinition
	// custom resource (CR) that defines the Component's characteristics and behavior.
	//
	// The system selects the ComponentDefinition CR with the latest version that matches the pattern.
	// This approach allows:
	//
	// 1. Precise selection by providing the exact name of a ComponentDefinition CR.
	// 2. Flexible and automatic selection of the most up-to-date ComponentDefinition CR
	// 	  by specifying a name prefix or regular expression pattern.
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
