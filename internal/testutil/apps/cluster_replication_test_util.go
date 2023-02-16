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

package apps

import (
	appsv1 "k8s.io/api/apps/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

// InitReplicationRedis initializes a cluster environment which only contains a component of Replication type for testing,
// includes ClusterDefinition, ClusterVersion and Cluster resources.
func InitReplicationRedis(
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	replicationCompName string) (*appsv1alpha1.ClusterDefinition, *appsv1alpha1.ClusterVersion, *appsv1alpha1.Cluster) {
	clusterDef := CreateReplicationRedisClusterDef(testCtx, clusterDefName)
	clusterVersion := CreateReplicationRedisClusterVersion(testCtx, clusterDefName, clusterVersionName)
	cluster := CreateReplicationCluster(testCtx, clusterDefName, clusterVersionName, clusterName, replicationCompName)
	return clusterDef, clusterVersion, cluster
}

// CreateReplicationCluster creates a redis cluster with a component of Replication type.
func CreateReplicationCluster(
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	replicationCompName string) *appsv1alpha1.Cluster {
	return CreateCustomizedObj(&testCtx, "replicationset/redis.yaml", &appsv1alpha1.Cluster{},
		CustomizeObjYAML(clusterName, clusterDefName, clusterVersionName, replicationCompName))
}

// CreateReplicationRedisClusterDef creates a redis clusterDefinition with a component of Replication type.
func CreateReplicationRedisClusterDef(testCtx testutil.TestContext, clusterDefName string) *appsv1alpha1.ClusterDefinition {
	return CreateCustomizedObj(&testCtx, "replicationset/redis_cd.yaml", &appsv1alpha1.ClusterDefinition{},
		CustomizeObjYAML(clusterDefName))
}

// CreateReplicationRedisClusterVersion creates a redis clusterVersion with a component of Replication type.
func CreateReplicationRedisClusterVersion(testCtx testutil.TestContext, clusterDefName, clusterVersionName string) *appsv1alpha1.ClusterVersion {
	return CreateCustomizedObj(&testCtx, "replicationset/redis_cv.yaml", &appsv1alpha1.ClusterVersion{},
		CustomizeObjYAML(clusterVersionName, clusterDefName))
}

// MockReplicationComponentStatefulSet mocks the component statefulSet, just using in envTest
func MockReplicationComponentStatefulSet(
	testCtx testutil.TestContext,
	clusterName,
	replicationCompName string) *appsv1.StatefulSet {
	stsName := clusterName + "-" + replicationCompName
	return CreateCustomizedObj(&testCtx, "replicationset/stateful_set.yaml", &appsv1.StatefulSet{},
		CustomizeObjYAML(replicationCompName, clusterName,
			stsName, replicationCompName, clusterName, replicationCompName, clusterName))
}
