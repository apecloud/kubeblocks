/*
Copyright 2022 The KubeBlocks Authors

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

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BenchJob struct {
	// Image defines the fio docker image used for the benchmark
	Image string `json:"image"`
}

// BenchType The bechmark we supported now. Valid values are Fio, Sysbench.
// +enum
type BenchType string

// benchmark we suported now.
const (
	Fio      BenchType = "fio"
	Sysbench BenchType = "sysbench"
)

// VolumeSpec contains the Volume Definition used for the benchmarks.
// It can point to an HostPath, already existing PVC or auto created PVC
// described by AutoPersistentVolumeClaimSpec.
type VolumeSpec struct {
	// VolumeSource gives the source of the volume, e.g. HostPath, PersistentVolumeClaim, etc.
	// PersistentVolumeClaim.claimName can be an already set to an existing PVC or 'AUTO_CREATED'.
	// When set to 'AUTO_CREATED', The PVC will be created based on the AutoPersistentVolumeClaimSpec
	// provided.
	VolumeSource corev1.VolumeSource `json:"volumeSource"`

	// AutoPersistentVolumeClaimSpec describes the auto-created pv-claim spec when user want to let the
	// benchmark to created a pv-claim for him.
	// If specified, the VolumeSource.PersistentVolumeClaim's claimName must be set to 'AUTO_CREATED'
	// +optional
	AutoPersistentVolumeClaimSpec *corev1.PersistentVolumeClaimSpec `json:"autoPersistentVolumeClaimSpec,omitempty"`
}

// AutoCreatedPVC is the pre-defined name to be used as ClaimName
// when the PVC is created on the fly for the benchmark.
const AutoCreatedPVC = "AUTO_CREATED"

// Validate method validates that the provided VolumeSpec meets the
// requirements:
// If PersistentVolumeClaimSpec is provided, then the VolumeSource's
// PersistentVolumClaim's ClaimName should be set to AutoCreatedPVC
func (v *VolumeSpec) Validate() (ok bool, err error) {
	if v.AutoPersistentVolumeClaimSpec != nil {
		if v.VolumeSource.PersistentVolumeClaim != nil &&
			v.VolumeSource.PersistentVolumeClaim.ClaimName != AutoCreatedPVC {
			return false, errors.New("If AutoVolumeClaimSpec is given, " +
				"VolumeSource.PersistentVolumeClaim.ClaimName must be " + AutoCreatedPVC)
		}
	}
	return true, nil
}

type FioSpec struct {
	// CmdLineArgs contains the arguments to run fio.
	CmdLineArgs string `json:"cmdLineArgs"`

	// Volume contains the configuration for the volume that the fio job should
	// run on.
	Volume VolumeSpec `json:"volume"`
}

type SysbenchSpec struct {
	// CmdLineArgs contains the arguments to run fio.
	CmdLineArgs string `json:"cmdLineArgs"`
}

// BenchSpec defines the desired state of Bench
type BenchSpec struct {
	// Type of the benchmark to run.
	Type BenchType `json:"type"`

	// Foo is an example field of Bench. Edit bench_types.go to remove/update
	BenchJob BenchJob `json:"benchJob"`

	// Fio describes the fio job to run.
	// +optional
	Fio *FioSpec `json:"fio,omitempty"`

	// Sysbench describes the sysbench job to run.
	// +optional
	Sysbench *SysbenchSpec `json:"sysbench,omitempty"`
}

// TODO: add validation of BenchSpec

// BenchPhase The current phase. Valid values are New, InProgress, Completed, Failed.
// +enum
type BenchPhase string

// These are the valid statuses of Bench.
const (
	BenchNew        BenchPhase = "New"
	BenchInProgress BenchPhase = "Running"
	BenchCompleted  BenchPhase = "Completed"
	BenchFailed     BenchPhase = "Failed"
)

// BenchStatus defines the observed state of Bench
type BenchStatus struct {
	// benchmark running phase.
	// +optional
	Phase BenchPhase `json:"phase,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Bench is the Schema for the benches API
type Bench struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BenchSpec   `json:"spec,omitempty"`
	Status BenchStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BenchList contains a list of Bench
type BenchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bench `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Bench{}, &BenchList{})
}
