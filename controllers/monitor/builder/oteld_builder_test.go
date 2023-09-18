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

package builder

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Monitor builder test", func() {

	var k8sMockClient *testutil.K8sClientMockHelper

	const clusterDefName = "test-clusterdef"
	const statefulCompDefName = "replicasets"
	const configSpecName = "mysql-config-tpl"
	const configVolumeName = "mysql-config"

	BeforeEach(func() {
		k8sMockClient = testutil.NewK8sMockClient()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		k8sMockClient.Finish()
	})

	Context("When updating configuration", func() {
		It("Should reconcile success", func() {

			clusterDefObj := testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, statefulCompDefName).
				GetObject()

			clusterDefObj.Spec.ComponentDefs[0].LogConfigs = []appsv1alpha1.LogConfig{{
				Name:            "error",
				FilePathPattern: "/data/mysql/log/mysqld-error.log",
			}, {
				Name:            "slow",
				FilePathPattern: "/data/mysql/log/mysqld-slowquery.log",
			}}
			k8sMockClient.MockGetMethod(
				testutil.WithGetReturned(testutil.WithConstructGetResult(clusterDefObj)))
			Expect(buildMysqlReceiverObject(context.TODO(), k8sMockClient.Client())).ShouldNot(Succeed())
		})
	})

})

func Test_buildMysqlReceiverObject(t *testing.T) {
}
