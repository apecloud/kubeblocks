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

package dbaas

import (
	appsv1 "k8s.io/api/apps/v1"

	"github.com/apecloud/kubeblocks/internal/testutil"
)

// MockReplicationComponentStatefulSet mocks the component statefulSet, just using in envTest
func MockReplicationComponentStatefulSet(
	testCtx testutil.TestContext,
	clusterName,
	replicationCompName string,
	statefulSetName string,
	role string) *appsv1.StatefulSet {
	return CreateCustomizedObj(&testCtx, "replicationset/stateful_set.yaml", &appsv1.StatefulSet{},
		CustomizeObjYAML(replicationCompName, clusterName, role,
			statefulSetName, replicationCompName, clusterName, replicationCompName, clusterName))
}
