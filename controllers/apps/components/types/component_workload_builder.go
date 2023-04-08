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

package types

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO(refactor): define a custom workload to encapsulate all the resources.
//
//	runtime, config, script, env, volume, service, monitor, probe
type ComponentWorkloadBuilder interface {
	BuildEnv() ComponentWorkloadBuilder
	BuildHeadlessService() ComponentWorkloadBuilder
	BuildService() ComponentWorkloadBuilder
	BuildTLSCert() ComponentWorkloadBuilder

	// workload related
	BuildConfig(idx int32) ComponentWorkloadBuilder
	BuildWorkload(idx int32) ComponentWorkloadBuilder
	BuildVolume(idx int32) ComponentWorkloadBuilder
	BuildVolumeMount(idx int32) ComponentWorkloadBuilder
	BuildTLSVolume(idx int32) ComponentWorkloadBuilder

	Complete() error

	MutableWorkload(idx int32) client.Object
	MutableRuntime(idx int32) *corev1.PodSpec
}
