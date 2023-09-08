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

// BuiltInVar represents a predefined system or environment variable that can be used within BuiltInString to
// provide dynamic and context-specific values when processed.
type BuiltInVar string

// BuiltInString defines a string type that may contain references to BuiltInVar placeholders.
// These placeholders are meant to be replaced with actual values at runtime to provide dynamic content.
type BuiltInString string

const (
	// KB_RANDOM_PASSWD generates a random 8-character password.
	KB_RANDOM_PASSWD BuiltInVar = "$(KB_RANDOM_PASSWD)"

	// KB_UUID generates a random UUID v4 string.
	KB_UUID BuiltInVar = "$(UUID)"

	// KB_UUID_B64 generates a random UUID v4 and encode it in BASE64.
	KB_UUID_B64 BuiltInVar = "$(UUID_B64)"

	// KB_UUID_STR_B64 generates a random UUID v4 string, and encode it in BASE64.
	KB_UUID_STR_B64 BuiltInVar = "$(UUID_STR_B64)"

	// KB_UUID_HEX generates a random UUID v4 and represent it as a HEX string.
	KB_UUID_HEX BuiltInVar = "$(UUID_HEX)"

	// KB_SVC_FQDN placeholder for service FQDN value pattern - $(CLUSTER_NAME)-$(1ST_COMP_NAME).$(NAMESPACE).svc,
	// where 1ST_COMP_NAME is the 1st component that provides the `ClusterDefinition.spec.componentDefs[].service` attribute.
	KB_SVC_FQDN BuiltInVar = "$(SVC_FQDN)"

	// KB_HEADLESS_SVC_FQDN placeholder for headless service FQDN value pattern - $(CLUSTER_NAME)-$(1ST_COMP_NAME)-headless.$(NAMESPACE).svc,
	// where 1ST_COMP_NAME is the 1st component that provides the `ClusterDefinition.spec.componentDefs[].service` attribute.
	KB_HEADLESS_SVC_FQDN BuiltInVar = "$(HEADLESS_SVC_FQDN)"

	// KB_SVC_PORT references a ServicePort's port value with a specified port name.
	// Example: {"name": "mysql", "targetPort": "mysqlContainerPort", "port": 3306}.
	// Usage: $(SVC_PORT_mysql) will be rendered as 3306.
	KB_SVC_PORT BuiltInVar = "$(SVC_PORT_{PORT-NAME})"

	// KB_NAMESPACE references the namespace where the component is running.
	KB_NAMESPACE BuiltInVar = "$(KB_NAMESPACE)"

	// KB_CLUSTER_NAME references the name of the cluster.
	KB_CLUSTER_NAME BuiltInVar = "$(KB_CLUSTER_NAME)"

	// KB_COMPONENT_NAME references the name of the component.
	KB_COMPONENT_NAME BuiltInVar = "$(KB_COMPONENT_NAME)"

	// KB_COMPONENT_REPLICAS references the number of replicas for the component.
	KB_COMPONENT_REPLICAS BuiltInVar = "$(KB_COMPONENT_REPLICAS)"

	KB_HOST_NAME   BuiltInVar = "$(KB_HOST_NAME)"
	KB_HOST_IP     BuiltInVar = "$(KB_HOST_IP)"
	KB_HOST_FQDN   BuiltInVar = "$(KB_HOST_FQDN)"
	KB_POD_NAME    BuiltInVar = "$(KB_POD_NAME)"
	KB_POD_IP      BuiltInVar = "$(KB_POD_IP)"
	KB_POD_FQDN    BuiltInVar = "$(KB_POD_FQDN)"
	KB_POD_ORDINAL BuiltInVar = "$(KB_POD_ORDINAL)"

	KB_SERVICE_ENDPOINT BuiltInVar = "$(KB_SERVICE_ENDPOINT)"
	KB_SERVICE_PORT     BuiltInVar = "$(KB_SERVICE_PORT)"

	// KB_REPLICA_ROLE references the role of the replica (e.g., leader, follower).
	KB_REPLICA_ROLE BuiltInVar = "$(KB_REPLICA_ROLE)"

	// TODO: built-in operators, i.e, length, symbol/digit, lower/upper - for password
)
