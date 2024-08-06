/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package configuration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("resource Fetcher", func() {
	const (
		compDefName     = "test-compdef"
		clusterName     = "test-cluster"
		mysqlCompName   = "mysql"
		mysqlConfigName = "mysql-config-template"
	)

	var (
		k8sMockClient *testutil.K8sClientMockHelper
		cluster       *appsv1alpha1.Cluster
	)

	BeforeEach(func() {
		k8sMockClient = testutil.NewK8sMockClient()
		testapps.NewComponentDefinitionFactory(compDefName).
			SetDefaultSpec().
			GetObject()
		pvcSpec := testapps.NewPVCSpec("1Gi")
		cluster = testapps.NewClusterFactory("default", clusterName, "").
			AddComponentV2(mysqlCompName, compDefName).
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
					cluster,
					testapps.NewConfigMap("default", cfgcore.GetComponentCfgName(clusterName, mysqlCompName, mysqlConfigName)),
					&appsv1beta1.ConfigConstraint{
						ObjectMeta: metav1.ObjectMeta{
							Name: mysqlConfigName,
						},
					},
				},
			), testutil.WithAnyTimes()))
			err := NewTest(k8sMockClient.Client(), ctx).
				Cluster().
				ComponentSpec().
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
