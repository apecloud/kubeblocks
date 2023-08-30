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

// Package v1alpha1 contains API Schema definitions for the apps v1alpha1 API group
package v1alpha1

type BuiltInVars string

// BuiltInString defines a kind of string that may contain BuiltInVars within it.
// For this kind of string, we will replace all built-in vars with actual value before using it.
type BuiltInString string

const (
	// KB_RANDOM_PASSWD - random 8 characters
	KB_RANDOM_PASSWD BuiltInVars = "$(KB_RANDOM_PASSWD)"

	// KB_UUID - random UUID v4 string
	KB_UUID BuiltInVars = "$(UUID)"
	// KB_UUID_B64 - random UUID v4 BASE64 encoded string
	KB_UUID_B64 BuiltInVars = "$(UUID_B64)"
	// KB_UUID_STR_B64 - random UUID v4 string then BASE64 encoded
	KB_UUID_STR_B64 BuiltInVars = "$(UUID_STR_B64)"
	// KB_UUID_HEX - random UUID v4 HEX representation.
	KB_UUID_HEX BuiltInVars = "$(UUID_HEX)"

	// KB_SVC_FQDN - service FQDN  placeholder, value pattern - $(CLUSTER_NAME)-$(1ST_COMP_NAME).$(NAMESPACE).svc,
	// where 1ST_COMP_NAME is the 1st component that provide `ClusterDefinition.spec.componentDefs[].service` attribute
	KB_SVC_FQDN BuiltInVars = "$(SVC_FQDN)"

	// KB_HEADLESS_SVC_FQDN - headless service FQDN placeholder, value pattern - $(CLUSTER_NAME)-$(1ST_COMP_NAME)-headless.$(NAMESPACE).svc,
	// where 1ST_COMP_NAME is the 1st component that provide `ClusterDefinition.spec.componentDefs[].service` attribute
	KB_HEADLESS_SVC_FQDN BuiltInVars = "$(HEADLESS_SVC_FQDN)"

	// KB_SVC_PORT - a ServicePort's port value with specified port name, i.e, a servicePort JSON struct:
	// `{"name": "mysql", "targetPort": "mysqlContainerPort", "port": 3306}`, and "$(SVC_PORT_mysql)" will be rendered with value 3306.
	KB_SVC_PORT BuiltInVars = "$(SVC_PORT_{PORT-NAME})"

	KB_NAMESPACE          BuiltInVars = "$(KB_NAMESPACE)"
	KB_CLUSTER_NAME       BuiltInVars = "$(KB_CLUSTER_NAME)"
	KB_COMPONENT_NAME     BuiltInVars = "$(KB_COMPONENT_NAME)"
	KB_COMPONENT_REPLICAS BuiltInVars = "$(KB_COMPONENT_REPLICAS)"

	KB_HOST_NAME   BuiltInVars = "$(KB_NODE_NAME)"
	KB_HOST_IP     BuiltInVars = "$(KB_HOST_IP)"
	KB_HOST_FQDN   BuiltInVars = "$(KB_HOST_FQDN)"
	KB_POD_NAME    BuiltInVars = "$(KB_POD_NAME)"
	KB_POD_IP      BuiltInVars = "$(KB_POD_IP)"
	KB_POD_FQDN    BuiltInVars = "$(KB_POD_FQDN)"
	KB_POD_ORDINAL BuiltInVars = "$(KB_POD_ORDINAL)"

	KB_SERVICE_ENDPOINT BuiltInVars = "$(KB_SERVICE_ENDPOINT)"
	KB_SERVICE_PORT     BuiltInVars = "$(KB_SERVICE_PORT)"
	KB_SERVICE_USER     BuiltInVars = "$(KB_SERVICE_USER)"
	KB_SERVICE_PASSWORD BuiltInVars = "$(KB_SERVICE_PASSWORD)"

	KB_REPLICA_ROLE BuiltInVars = "$(KB_REPLICA_ROLE)"

	// TODO: built-in operators, i.e, length, symbol/digit, lower/upper - for password
)
