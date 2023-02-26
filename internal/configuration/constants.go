/*
Copyright ApeCloud, Inc.

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

package configuration

const (
	ConfigurationTemplateFinalizerName = "configuration.kubeblocks.io/finalizer"

	// ConfigurationTplLabelPrefixKey clusterVersion or clusterdefinition using tpl
	ConfigurationTplLabelPrefixKey         = "configuration.kubeblocks.io/cfg-tpl"
	ConfigurationConstraintsLabelPrefixKey = "configuration.kubeblocks.io/cfg-constraints"

	LastAppliedOpsCRAnnotation                  = "configuration.kubeblocks.io/last-applied-ops-name"
	LastAppliedConfigAnnotation                 = "configuration.kubeblocks.io/last-applied-configuration"
	DisableUpgradeInsConfigurationAnnotationKey = "configuration.kubeblocks.io/disable-reconfigure"
	UpgradePolicyAnnotationKey                  = "configuration.kubeblocks.io/reconfigure-policy"
	UpgradeRestartAnnotationKey                 = "configuration.kubeblocks.io/restart"

	// CMConfigurationTypeLabelKey configmap is config template type, e.g: "tpl", "instance"
	CMConfigurationTypeLabelKey            = "configuration.kubeblocks.io/configuration-type"
	CMConfigurationTplNameLabelKey         = "configuration.kubeblocks.io/configuration-tpl-name"
	CMConfigurationConstraintsNameLabelKey = "configuration.kubeblocks.io/configuration-constraints-name"
	CMInsConfigurationHashLabelKey         = "configuration.kubeblocks.io/configuration-hash"
	CMConfigurationProviderTplLabelKey     = "configuration.kubeblocks.io/configtemplate-name"

	// CMInsConfigurationLabelKey configmap is configuration file for component
	// CMInsConfigurationLabelKey = "configuration.kubeblocks.io/ins-configure"

	CMInsLastReconfigureMethodLabelKey = "configuration.kubeblocks.io/last-applied-reconfigure-policy"

	// ConfigSidecarIMAGE for config manager sidecar
	ConfigSidecarIMAGE       = "KUBEBLOCKS_IMAGE"
	ConfigSidecarName        = "config-manager"
	CRIRuntimeEndpoint       = "CONTAINER_RUNTIME_ENDPOINT"
	ConfigCRIType            = "CRI_TYPE"
	ConfigManagerGPRCPortEnv = "CONFIG_MANAGER_GRPC_PORT"

	PodMinReadySecondsEnv = "POD_MIN_READY_SECONDS"

	ConfigTemplateType = "tpl"
	ConfigInstanceType = "instance"
)
