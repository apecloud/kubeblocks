/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
