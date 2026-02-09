/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package constant

const (
	ConfigurationConstraintsLabelPrefixKey = "config.kubeblocks.io/constraints"

	// ConfigurationRevision defines the current revision
	// TODO support multi version
	ConfigurationRevision          = "config.kubeblocks.io/configuration-revision"
	LastConfigurationRevisionPhase = "config.kubeblocks.io/revision-reconcile-phase"
)

const (
	CMConfigurationSpecProviderLabelKey    = "config.kubeblocks.io/config-spec" // CMConfigurationSpecProviderLabelKey is ComponentConfigSpec name
	CMConfigurationTemplateNameLabelKey    = "config.kubeblocks.io/config-template-name"
	CMTemplateNameLabelKey                 = "config.kubeblocks.io/template-name"
	CMConfigurationTypeLabelKey            = "config.kubeblocks.io/config-type"
	CMInsConfigurationHashLabelKey         = "config.kubeblocks.io/config-hash"
	CMConfigurationConstraintsNameLabelKey = "config.kubeblocks.io/config-constraints-name"

	ParametersInitLabelKey               = "config.kubeblocks.io/init-parameters"
	CustomParameterTemplateAnnotationKey = "config.kubeblocks.io/custom-template"
)

const (
	DisableUpgradeInsConfigurationAnnotationKey = "config.kubeblocks.io/disable-reconfigure"
	LastAppliedConfigAnnotationKey              = "config.kubeblocks.io/last-applied-configuration"
	UpgradePolicyAnnotationKey                  = "config.kubeblocks.io/reconfigure-policy"
	UpgradeRestartAnnotationKey                 = "config.kubeblocks.io/restart"
	ConfigAppliedVersionAnnotationKey           = "config.kubeblocks.io/config-applied-version"
)

const (
	ConfigManagerGPRCPortEnv = "CONFIG_MANAGER_GRPC_PORT"
	ConfigManagerLogLevel    = "CONFIG_MANAGER_LOG_LEVEL"

	ConfigInstanceType = "instance"
)

const (
	FeatureGateIgnoreConfigTemplateDefaultMode = "IGNORE_CONFIG_TEMPLATE_DEFAULT_MODE"
)
