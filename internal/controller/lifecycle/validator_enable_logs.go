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

package lifecycle

import (
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type enableLogsValidator struct {
	cluster    *appsv1alpha1.Cluster
	clusterDef *appsv1alpha1.ClusterDefinition
}

func (e *enableLogsValidator) Validate() error {
	// validate config and send warning event log necessarily
	return e.cluster.ValidateEnabledLogs(e.clusterDef)
}
