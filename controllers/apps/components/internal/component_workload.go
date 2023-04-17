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

package internal

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO(refactor): define a custom workload to encapsulate all the resources.

type ComponentWorkloadBuilder interface {
	//	runtime, config, script, env, volume, service, monitor, probe
	BuildEnv() ComponentWorkloadBuilder
	BuildHeadlessService() ComponentWorkloadBuilder
	BuildService() ComponentWorkloadBuilder
	BuildTLSCert() ComponentWorkloadBuilder

	// workload related
	BuildWorkload(idx int32) ComponentWorkloadBuilder
	BuildConfig(idx int32) ComponentWorkloadBuilder
	BuildVolume(idx int32) ComponentWorkloadBuilder
	BuildVolumeMount(idx int32) ComponentWorkloadBuilder
	BuildTLSVolume(idx int32) ComponentWorkloadBuilder

	Complete() error

	MutableWorkload(idx int32) client.Object
}
