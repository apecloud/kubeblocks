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

package controllerutil

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

const (
	clusterDefName     = "test-clusterdef"
	clusterVersionName = "test-clusterversion"
	clusterName        = "test-cluster"

	mysqlCompDefName = "replicasets"
	mysqlCompName    = "mysql"
	mysqlConfigName  = "mysql-config-template"
	mysqlVolumeName  = "mysql-config"
)

var _ = Describe("resource Fetcher", func() {

	var (
		k8sMockClient  *testutil.K8sClientMockHelper
		clusterDef     *appsv1alpha1.ClusterDefinition
		clusterVersion *appsv1alpha1.ClusterVersion
		cluster        *appsv1alpha1.Cluster
	)

	BeforeEach(func() {
		k8sMockClient = testutil.NewK8sMockClient()
		clusterDef = testapps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
			AddConfigTemplate(mysqlConfigName, mysqlConfigName, mysqlConfigName, "default", mysqlVolumeName).
			GetObject()
		clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
			AddComponentVersion(mysqlCompDefName).
			AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			GetObject()
		pvcSpec := testapps.NewPVCSpec("1Gi")
		cluster = testapps.NewClusterFactory("default", clusterName,
			clusterDef.Name, clusterVersion.Name).
			AddComponent(mysqlCompName, mysqlCompDefName).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			GetObject()
	})

	AfterEach(func() {
		k8sMockClient.Finish()
	})

	Context("ResourceWrapper", func() {
		It("Should succeed with no error", func() {
			k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult(
				[]client.Object{
					clusterDef,
					clusterVersion,
					cluster,
					testapps.NewConfigMap("default", cfgcore.GetComponentCfgName(clusterName, mysqlCompName, mysqlConfigName)),
					&appsv1alpha1.ConfigConstraint{
						ObjectMeta: metav1.ObjectMeta{
							Name: mysqlConfigName,
						},
					},
				},
			), testutil.WithAnyTimes()))
			err := NewTest(k8sMockClient.Client(), ctx).
				Cluster().
				ClusterDef().
				ClusterVer().
				ClusterComponent().
				ClusterDefComponent().
				ConfigMap(mysqlConfigName).
				ConfigConstraints(mysqlConfigName).
				Configuration().
				Complete()
			Expect(err).Should(Succeed())
		})
	})

})

type test struct {
	ResourceFetcher[test]
}

func NewTest(cli client.Client, ctx context.Context) *test {
	tt := &test{}
	return tt.Init(&ResourceCtx{
		Client:  cli,
		Context: ctx,

		Namespace:     "default",
		ClusterName:   clusterName,
		ComponentName: mysqlCompName,
	}, tt)
}
