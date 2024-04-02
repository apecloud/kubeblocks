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

// +--------------------------------------------------------------------------------------------------------------------------------------------------------------------+
// |                                                                                                                                                                    |
// |     ++---------------------------------------------------------------------------------------------------+                                                         |
// |     |+--------------------------------------------------------------------------------------------------+|                                                         |
// |     ||                                CInitPhase                                                        ||                                                         |
// |     ||                                                                                                  ||                                                         |
// |     ||   +-------------+          +---------------+            +-----------------+          +-----+     ||                                                         |
// |     ||   |             |          |               |            |                 |          |     |     ||      Creating                                           |
// |     ||   | PreparePhase|----------| RenderingPhase+----------->|GeneratingSideCar|--------->| END |     ||---------------- fireEvent                               |
// |     ||   |             |          |               |            |                 |          |     |     ||                    |                                    |
// |     ||   +-------------+          +---------------+            +-----------------+          +-----+     ||                    |                                    |
// |     ||                                                                                                  ||                    |                                    |
// |     ||                                                                                                  ||                    |                                    |
// |     ++--------------------------------------------------------------------------------------------------+|                    |    Reconfiguring                   |
// |     +----------------------------------------------------------------------------------------------------+                    |                                    |
// |           ^                    /                                           \                                                  |                                    |
// |           |                   /                                             \  Succeed                                        V                                    |
// |           | Creating         /                 +-----------------------------\----------------------------------------------------------------------------+        |
// |           |                 /                  |+-----------------------------V--------------------------------------------------------------------------+|        |
// |           |                /                   ||                                                                                                        ||        |
// |           |            Failed                  ||                                     RunningPhase                                                       ||        |
// |           |              /                     ||                                                              Reconfiguring                             ||        |
// |           |             /                      ||                                                   +---------------------------------+                  ||        |
// |           |            /                       ||                                                   v                                 |                  ||        |
// |           |           v                        ||      +---------------+                  +--------------------+             +--------+---------+        ||        |
// |      +----+----------------+                   ||      |               |  Reconfiguring   |                    |   Failed    |                  |        ||        |
// |      |                     |                   ||      |  CFinishPhase |----------------->|   MergingPhase     |------------>| MergeFailedPhase |        ||        |
// |      | CCreateFailedPhase  |                   ||      |               |               -> |                    |             |                  |        ||        |
// |      |                     |                   ||      +-------+-------+              /   +---------+----------+             +------------------+        ||        |
// |      +---------------------+                   ||              ^                     /              |                                                    ||        |
// |                                                ||              |                Reconfiguring       |                                                    ||        |
// |                                                ||              |                   /                |                                                    ||        |
// |                                                ||       Finish |                  /                 | Succeed                                            ||        |
// |                                                ||              |                 /                  |                                                    ||        |
// |                                                ||              |     +-----------------+            |                                                    ||        |
// |                                                ||              |     |                 |            |                                                    ||        |
// |                                                ||              +-----+ UpgradingPhase  |<-----------+                                                    ||        |
// |                                                ||                    |                 |                                                                 ||        |
// |                                                ||                    +------------+----+                                                                 ||        |
// |                                                ||                         ^       |                                                                      ||        |
// |                                                ||                         |       |                                                                      ||        |
// |                                                ||                         +-------+                                                                      ||        |
// |                                                ||                                                                                                        ||        |
// |                                                ||                                                                                                        ||        |
// |                                                |+--------------------------------------------------------------------------------------------------------+|        |
// |                                                +----------------------------------------------------------------------------------------------------------+        |
// |                                                                                                                                                                    |
// |                                                                                                                                                                    |
// +--------------------------------------------------------------------------------------------------------------------------------------------------------------------+
//

// ConfigurationPhase defines the Configuration FSM phase
// +enum
// +kubebuilder:validation:Enum={Creating,Init,Running,Pending,Merged,MergeFailed,FailedAndPause,Upgrading,Deleting,FailedAndRetry,Finished}
type ConfigurationPhase string

const (
	CCreatingPhase       ConfigurationPhase = "Creating"
	CInitPhase           ConfigurationPhase = "Init"
	CRunningPhase        ConfigurationPhase = "Running"
	CPendingPhase        ConfigurationPhase = "Pending"
	CFailedPhase         ConfigurationPhase = "FailedAndRetry"
	CFailedAndPausePhase ConfigurationPhase = "FailedAndPause"
	CMergedPhase         ConfigurationPhase = "Merged"
	CMergeFailedPhase    ConfigurationPhase = "MergeFailed"
	CDeletingPhase       ConfigurationPhase = "Deleting"
	CUpgradingPhase      ConfigurationPhase = "Upgrading"
	CFinishedPhase       ConfigurationPhase = "Finished"
)

type ConfigParams struct {
	// Holds the configuration keys and values. This field is a workaround for issues found in kubebuilder and code-generator.
	// Refer to https://github.com/kubernetes-sigs/kubebuilder/issues/528 and https://github.com/kubernetes/code-generator/issues/50 for more details.
	//
	// Represents the content of the configuration file.
	//
	// +optional
	Content *string `json:"content"`

	// Represents the updated parameters for a single configuration file.
	//
	// +optional
	Parameters map[string]*string `json:"parameters,omitempty"`
}
