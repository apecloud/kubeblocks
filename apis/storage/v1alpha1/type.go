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

// StorageProviderPhase defines phases of a `StorageProvider`.
//
// +enum
type StorageProviderPhase string

const (
	// StorageProviderNotReady indicates that the `StorageProvider` is not ready,
	// usually because the specified CSI driver is not yet installed.
	StorageProviderNotReady StorageProviderPhase = "NotReady"
	// StorageProviderReady indicates that the `StorageProvider` is ready for use.
	StorageProviderReady StorageProviderPhase = "Ready"
)

const (
	// ConditionTypeCSIDriverInstalled is the name of the condition that
	// indicates whether the CSI driver is installed.
	ConditionTypeCSIDriverInstalled = "CSIDriverInstalled"
)
