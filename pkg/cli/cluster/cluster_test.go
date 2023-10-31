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

package cluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	"github.com/apecloud/kubeblocks/pkg/cli/testing"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

var _ = Describe("cluster util", func() {
	baseObjs := []runtime.Object{
		testing.FakePods(3, testing.Namespace, testing.ClusterName),
		testing.FakeSecrets(testing.Namespace, testing.ClusterName),
		testing.FakeServices(),
		testing.FakePVCs(),
	}

	baseObjsWithBackupPods := func() []runtime.Object {
		podsWithBackup := testing.FakePods(4, testing.Namespace, testing.ClusterName)
		labels := podsWithBackup.Items[0].GetLabels()
		labels[dptypes.BackupNameLabelKey] = testing.BackupName
		podsWithBackup.Items[0].SetLabels(labels)
		return []runtime.Object{
			podsWithBackup,
			testing.FakeSecrets(testing.Namespace, testing.ClusterName),
			testing.FakeServices(),
			testing.FakePVCs(),
		}
	}
	cluster := testing.FakeCluster(testing.ClusterName, testing.Namespace)
	dynamic := testing.FakeDynamicClient(
		cluster,
		testing.FakeClusterDef(),
		testing.FakeBackupPolicy("backupPolicy-test", testing.ClusterName),
		testing.FakeBackupWithCluster(cluster, "backup-test"),
		testing.FakeClusterVersion())

	getOptions := GetOptions{
		WithClusterDef:     true,
		WithClusterVersion: true,
		WithConfigMap:      true,
		WithService:        true,
		WithSecret:         true,
		WithPVC:            true,
		WithPod:            true,
		WithDataProtection: true,
	}

	It("get cluster objects", func() {
		var (
			err  error
			objs *ClusterObjects
		)

		testFn := func(client kubernetes.Interface) {
			clusterName := testing.ClusterName
			getter := ObjectsGetter{
				Client:     client,
				Dynamic:    dynamic,
				Name:       clusterName,
				Namespace:  testing.Namespace,
				GetOptions: getOptions,
			}

			objs, err = getter.Get()
			Expect(err).Should(Succeed())
			Expect(objs).ShouldNot(BeNil())
			Expect(objs.Cluster.Name).Should(Equal(clusterName))
			Expect(objs.ClusterDef.Name).Should(Equal(testing.ClusterDefName))
			Expect(objs.ClusterVersion.Name).Should(Equal(testing.ClusterVersionName))
			Expect(len(objs.Pods.Items)).Should(Equal(3))
			Expect(len(objs.Secrets.Items)).Should(Equal(1))
			Expect(len(objs.Services.Items)).Should(Equal(4))
			Expect(len(objs.PVCs.Items)).Should(Equal(1))
			Expect(len(objs.GetComponentInfo())).Should(Equal(1))
		}

		By("when node is not found")
		testFn(testing.FakeClientSet(baseObjs...))
		Expect(len(objs.Nodes)).Should(Equal(0))

		By("when node is available")
		baseObjs = append(baseObjs, testing.FakeNode())
		testFn(testing.FakeClientSet(baseObjs...))
		Expect(len(objs.Nodes)).Should(Equal(1))

		By("when pod is back-up created")
		testFn(testing.FakeClientSet(baseObjsWithBackupPods()...))
	})
})
