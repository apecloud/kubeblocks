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

// BuiltInEnvVar represents a predefined system or environment variable that can be used within BuiltInString to
// provide dynamic and context-specific values when processed.
type BuiltInEnvVar string

// BuiltInString defines a string type that may contain references to BuiltInEnvVar and/or BuiltInGeneratorVar placeholders.
// These placeholders are meant to be replaced with actual values at runtime to provide dynamic content.
type BuiltInString string

const (
	// KB_NAMESPACE references the namespace where the component is running.
	KB_NAMESPACE BuiltInEnvVar = "$(KB_NAMESPACE)"

	// KB_CLUSTER_NAME references the name of the cluster.
	KB_CLUSTER_NAME BuiltInEnvVar = "$(KB_CLUSTER_NAME)"

	// KB_COMPONENT_NAME references the name of the component.
	KB_COMPONENT_NAME BuiltInEnvVar = "$(KB_COMPONENT_NAME)"

	KB_HOST_NAME BuiltInEnvVar = "$(KB_HOST_NAME)"
	KB_HOST_IP   BuiltInEnvVar = "$(KB_HOST_IP)"
	KB_HOST_FQDN BuiltInEnvVar = "$(KB_HOST_FQDN)"
	KB_POD_NAME  BuiltInEnvVar = "$(KB_POD_NAME)"
	KB_POD_IP    BuiltInEnvVar = "$(KB_POD_IP)"
	KB_POD_FQDN  BuiltInEnvVar = "$(KB_POD_FQDN)"

	// KB_COMPONENT_REPLICAS references the number of replicas for the component.
	KB_COMPONENT_REPLICAS BuiltInEnvVar = "$(KB_COMPONENT_REPLICAS)"

	// KB_REPLICA_ROLE references the role of the replica (e.g., leader, follower).
	KB_REPLICA_ROLE BuiltInEnvVar = "$(KB_REPLICA_ROLE)"
)
