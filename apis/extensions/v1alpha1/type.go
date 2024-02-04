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

// AddonType defines the addon types.
// +enum
// +kubebuilder:validation:Enum={Helm}
type AddonType string

const (
	HelmType AddonType = "Helm"
)

// LineSelectorOperator defines line selector operators.
// +enum
// +kubebuilder:validation:Enum={Contains,DoesNotContain,MatchRegex,DoesNotMatchRegex}
type LineSelectorOperator string

const (
	Contains          LineSelectorOperator = "Contains"
	DoesNotContain    LineSelectorOperator = "DoesNotContain"
	MatchRegex        LineSelectorOperator = "MatchRegex"
	DoesNotMatchRegex LineSelectorOperator = "DoesNotMatchRegex"
)

// AddonPhase defines addon phases.
// +enum
type AddonPhase string

const (
	AddonDisabled  AddonPhase = "Disabled"
	AddonEnabled   AddonPhase = "Enabled"
	AddonFailed    AddonPhase = "Failed"
	AddonEnabling  AddonPhase = "Enabling"
	AddonDisabling AddonPhase = "Disabling"
)

// AddonSelectorKey are selector requirement key types.
// +enum
// +kubebuilder:validation:Enum={KubeGitVersion,KubeVersion,KubeProvider}
type AddonSelectorKey string

const (
	KubeGitVersion AddonSelectorKey = "KubeGitVersion"
	KubeVersion    AddonSelectorKey = "KubeVersion"
	KubeProvider   AddonSelectorKey = "KubeProvider"
)

const (
	// condition types
	ConditionTypeProgressing = "Progressing"
	ConditionTypeChecked     = "InstallableChecked"
	ConditionTypeSucceed     = "Succeed"
	ConditionTypeFailed      = "Failed"
)
