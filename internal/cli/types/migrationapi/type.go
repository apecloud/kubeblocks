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

import (
	"strings"

	appv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DBTypeEnum defines the MigrationTemplate CR .spec.Source.DbType or .spec.Sink.DbType
// +enum
// +kubebuilder:validation:Enum={MySQL, PostgreSQL}
type DBTypeEnum string

const (
	MigrationDBTypeMySQL      DBTypeEnum = "MySQL" // default value
	MigrationDBTypePostgreSQL DBTypeEnum = "PostgreSQL"
)

func (d DBTypeEnum) String() string {
	return string(d)
}

// TaskTypeEnum defines the MigrationTask CR .spec.taskType
// +enum
// +kubebuilder:validation:Enum={initialization,cdc,initialization-and-cdc,initialization-and-twoway-cdc}
type TaskTypeEnum string

const (
	Initialization       TaskTypeEnum = "initialization"
	CDC                  TaskTypeEnum = "cdc"
	InitializationAndCdc TaskTypeEnum = "initialization-and-cdc" // default value
)

// EndpointTypeEnum defines the MigrationTask CR .spec.source.endpointType and .spec.sink.endpointType
// +enum
// +kubebuilder:validation:Enum={address}
type EndpointTypeEnum string

const (
	AddressDirectConnect EndpointTypeEnum = "address" // default value
)

// non-use yet

type ConflictPolicyEnum string

const (
	Ignore   ConflictPolicyEnum = "ignore"   // default in FullLoad
	Override ConflictPolicyEnum = "override" // default in CDC
)

// DMLOpEnum defines the MigrationTask CR .spec.migrationObj
// +enum
// +kubebuilder:validation:Enum={all,none,insert,update,delete}
type DMLOpEnum string

const (
	AllDML  DMLOpEnum = "all"
	NoneDML DMLOpEnum = "none"
	Insert  DMLOpEnum = "insert"
	Update  DMLOpEnum = "update"
	Delete  DMLOpEnum = "delete"
)

// DDLOpEnum defines the MigrationTask CR .spec.migrationObj
// +enum
// +kubebuilder:validation:Enum={all,none}
type DDLOpEnum string

const (
	AllDDL  DDLOpEnum = "all"
	NoneDDL DDLOpEnum = "none"
)

// DCLOpEnum defines the MigrationTask CR .spec.migrationObj
// +enum
// +kubebuilder:validation:Enum={all,none}
type DCLOpEnum string

const (
	AllDCL  DDLOpEnum = "all"
	NoneDCL DDLOpEnum = "none"
)

// TaskStatus defines the MigrationTask CR .status.taskStatus
// +enum
// +kubebuilder:validation:Enum={Prepare,InitPrepared,Init,InitFinished,Running,Cached,Pause,Done}
type TaskStatus string

const (
	PrepareStatus TaskStatus = "Prepare"
	InitPrepared  TaskStatus = "InitPrepared"
	InitStatus    TaskStatus = "Init"
	InitFinished  TaskStatus = "InitFinished"
	RunningStatus TaskStatus = "Running"
	CachedStatus  TaskStatus = "Cached"
	PauseStatus   TaskStatus = "Pause"
	DoneStatus    TaskStatus = "Done"
)

// StepEnum defines the MigrationTask CR .spec.steps
// +enum
// +kubebuilder:validation:Enum={preCheck,initStruct,initData,initStructLater}
type StepEnum string

const (
	StepPreCheck            StepEnum = "preCheck"
	StepStructPreFullLoad   StepEnum = "initStruct"
	StepFullLoad            StepEnum = "initData"
	StepStructAfterFullLoad StepEnum = "initStructLater"
	StepInitialization      StepEnum = "initialization"
	StepPreDelete           StepEnum = "preDelete"
	StepCdc                 StepEnum = "cdc"
)

func (s StepEnum) String() string {
	return string(s)
}

func (s StepEnum) LowerCaseString() string {
	return strings.ToLower(s.String())
}

func (s StepEnum) CliString() string {
	switch s {
	case StepPreCheck:
		return CliStepPreCheck.String()
	case StepStructPreFullLoad:
		return CliStepInitStruct.String()
	case StepFullLoad:
		return CliStepInitData.String()
	case StepCdc:
		return CliStepCdc.String()
	default:
		return "unknown"
	}
}

type CliStepEnum string

const (
	CliStepGlobal     CliStepEnum = "global"
	CliStepPreCheck   CliStepEnum = "precheck"
	CliStepInitStruct CliStepEnum = "init-struct"
	CliStepInitData   CliStepEnum = "init-data"
	CliStepCdc        CliStepEnum = "cdc"
)

func (s CliStepEnum) String() string {
	return string(s)
}

// Phase defines the MigrationTemplate CR .status.phase
// +enum
// +kubebuilder:validation:Enum={Available,Unavailable}
type Phase string

const (
	AvailablePhase   Phase = "Available"
	UnavailablePhase Phase = "Unavailable"
)

type MigrationObjects struct {
	Task     *MigrationTask
	Template *MigrationTemplate

	Jobs         *batchv1.JobList
	Pods         *v1.PodList
	StatefulSets *appv1.StatefulSetList
}

// +k8s:deepcopy-gen=false

type IntOrStringMap map[string]interface{}

func (in *IntOrStringMap) DeepCopyInto(out *IntOrStringMap) {
	if in == nil {
		*out = nil
	} else {
		*out = runtime.DeepCopyJSON(*in)
	}
}

func (in *IntOrStringMap) DeepCopy() *IntOrStringMap {
	if in == nil {
		return nil
	}
	out := new(IntOrStringMap)
	in.DeepCopyInto(out)
	return out
}
