/*
Copyright ApeCloud Inc.

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
	"context"
	"fmt"
	"github.com/apecloud/kubeblocks/test/testdata"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var (
	ReplicationComponentName = "redis-rsts"
)

func InitReplicationRedis(ctx context.Context,
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	replicationCompName string) (*dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, *dbaasv1alpha1.Cluster) {
	clusterDef := CreateReplicationRedisClusterDef(ctx, testCtx, clusterDefName)
	clusterVersion := CreateReplicationRedisClusterVersion(ctx, testCtx, clusterDefName, clusterVersionName)
	cluster := CreateReplicationCluster(ctx, testCtx, clusterDefName, clusterVersionName, clusterName, replicationCompName)
	return clusterDef, clusterVersion, cluster
}

func CreateReplicationCluster(
	ctx context.Context,
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	replicationCompName string) *dbaasv1alpha1.Cluster {
	clusterBytes, err := testdata.GetTestDataFileContent("replicationset/redis.yaml")
	if err != nil {
		return nil
	}
	clusterYaml := fmt.Sprintf(string(clusterBytes), clusterVersionName, clusterDefName, clusterName,
		clusterVersionName, clusterDefName, replicationCompName)
	cluster := &dbaasv1alpha1.Cluster{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(gomega.Succeed())
	return CreateK8sResource(ctx, testCtx, cluster).(*dbaasv1alpha1.Cluster)
}

func CreateReplicationRedisClusterDef(ctx context.Context, testCtx testutil.TestContext, clusterDefName string) *dbaasv1alpha1.ClusterDefinition {
	clusterDefBytes, err := testdata.GetTestDataFileContent("replicationset/redis_cd.yaml")
	if err != nil {
		return nil
	}
	clusterDefYaml := fmt.Sprintf(string(clusterDefBytes), clusterDefName)
	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)).Should(gomega.Succeed())
	return CreateK8sResource(ctx, testCtx, clusterDef).(*dbaasv1alpha1.ClusterDefinition)
}

func CreateReplicationRedisClusterVersion(ctx context.Context, testCtx testutil.TestContext, clusterDefName, clusterVersionName string) *dbaasv1alpha1.ClusterVersion {
	clusterVersionBytes, err := testdata.GetTestDataFileContent("replicationset/redis_cv.yaml")
	if err != nil {
		return nil
	}
	clusterVersionYAML := fmt.Sprintf(string(clusterVersionBytes), clusterVersionName, clusterDefName)
	clusterVersion := &dbaasv1alpha1.ClusterVersion{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterVersionYAML), clusterVersion)).Should(gomega.Succeed())
	return CreateK8sResource(ctx, testCtx, clusterVersion).(*dbaasv1alpha1.ClusterVersion)
}

// MockReplicationComponentStatefulSet mock the component statefulSet, just using in envTest
func MockReplicationComponentStatefulSet(ctx context.Context,
	testCtx testutil.TestContext,
	clusterName,
	replicationCompName string) *appsv1.StatefulSet {

	stsBytes, err := testdata.GetTestDataFileContent("replicationset/stateful_set.yaml")
	if err != nil {
		return nil
	}
	stsName := clusterName + "-" + replicationCompName
	statefulSetYaml := fmt.Sprintf(string(stsBytes), replicationCompName, clusterName,
		stsName, replicationCompName, clusterName, replicationCompName, clusterName, "%")
	sts := &appsv1.StatefulSet{}
	gomega.Expect(yaml.Unmarshal([]byte(statefulSetYaml), sts)).Should(gomega.Succeed())
	return CreateK8sResource(ctx, testCtx, sts).(*appsv1.StatefulSet)
}
