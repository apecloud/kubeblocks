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

// EndpointTypeEnum defines the Sync2FoxLakeTask CR .spec.source.endpointType and .spec.sink.endpointType
// +enum
// +kubebuilder:validation:Enum={address,clustername}
type EndpointTypeEnum string

const (
	AddressDirectConnect EndpointTypeEnum = "address" // default value
	ClusterName          EndpointTypeEnum = "clustername"
)

type Status string

const (
	CreatingStatus    Status = "Creating"
	SyncedStatus      Status = "Synced"
	UpdatedStatus     Status = "Updated"
	RunningStatus     Status = "Running"
	PausedStatus      Status = "Paused"
	TerminatingStatus Status = "Terminating"
	FailedStatus      Status = "Failed"
)

type Endpoint struct {
	EndpointType EndpointTypeEnum `json:"endpointType"`
	Endpoint     string           `json:"endpoint"`
	UserName     string           `json:"userName"`
	Password     string           `json:"password"`
	// +optional
	Host string `json:"host"`
	// +optional
	Port string `json:"port"`
}
type SyncDatabaseSpec struct {
	DatabaseType     string   `json:"databaseType"`
	DatabaseSelected string   `json:"databaseSelected"`
	Engine           string   `json:"engine"`
	Lag              string   `json:"lag"`
	Quota            string   `json:"quota"`
	IsPaused         bool     `json:"isPaused"`
	TablesIncluded   []string `json:"tablesIncluded"`
	TablesExcluded   []string `json:"tablesExcluded"`
}
