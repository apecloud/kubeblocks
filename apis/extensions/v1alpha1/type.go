/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
// +kubebuilder:validation:Enum={KubeGitVersion,KubeVersion}
type AddonSelectorKey string

const (
	KubeGitVersion AddonSelectorKey = "KubeGitVersion"
	KubeVersion    AddonSelectorKey = "KubeVersion"
)

const (
	// condition types
	ConditionTypeProgressing = "Progressing"
	ConditionTypeChecked     = "InstallableChecked"
	ConditionTypeSucceed     = "Succeed"
	ConditionTypeFailed      = "Failed"
)
