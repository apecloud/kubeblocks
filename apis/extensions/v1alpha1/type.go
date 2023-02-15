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

package v1alpha1

// AddonType defines the addon types.
// +enum
type AddonType string

const (
	HelmType AddonType = "Helm"
)

// LineSelectorOperator defines line selector operators.
// +enum
type LineSelectorOperator string

const (
	Contains           LineSelectorOperator = "Contains"
	DoesNotContain     LineSelectorOperator = "DoesNotContain"
	MatchRegex         LineSelectorOperator = "MatchRegex"
	DoesNoteMatchRegex LineSelectorOperator = "DoesNoteMatchRegex"
)

// AddonPhase defines addon phases.
// +enum
type AddonPhase string

const (
	Disabled  AddonPhase = "Disabled"
	Enabled   AddonPhase = "Enabled"
	Failed    AddonPhase = "Failed"
	Enabling  AddonPhase = "Enabling"
	Disabling AddonPhase = "Disabling"
)

// KeyHelmValueKey defines "key" Helm value's key types.
// +enum
type KeyHelmValueKey string

const (
	ReplicaCount KeyHelmValueKey = "ReplicaCount"
	PVEnabled    KeyHelmValueKey = "PVEnabled"
	StorageClass KeyHelmValueKey = "StorageClass"
	Tolerations  KeyHelmValueKey = "Tolerations"
)
