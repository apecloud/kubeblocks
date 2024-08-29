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

package v1

// Phase represents the status of a CR.
//
// +enum
// +kubebuilder:validation:Enum={Available,Unavailable}
type Phase string

const (
	// AvailablePhase indicates that a CR is in an available state.
	AvailablePhase Phase = "Available"

	// UnavailablePhase indicates that a CR is in an unavailable state.
	UnavailablePhase Phase = "Unavailable"
)
