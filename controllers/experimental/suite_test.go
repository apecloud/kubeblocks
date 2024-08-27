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

package experimental

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	//+kubebuilder:scaffold:imports
	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	experimentalv1alpha1 "github.com/apecloud/kubeblocks/apis/experimental/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	namespace = "foo"
	name      = "bar"
)

var (
	tree           *kubebuilderx.ObjectTree
	ncs            *experimentalv1alpha1.NodeCountScaler
	clusterName    = "foo"
	componentNames = []string{"bar-0", "bar-1"}
)

func mockTestTree() *kubebuilderx.ObjectTree {
	ncs = builder.NewNodeCountScalerBuilder(namespace, name).
		SetTargetClusterName(clusterName).
		SetTargetComponentNames(componentNames).
		GetObject()

	specs := []appsv1alpha1.ClusterComponentSpec{
		{
			Name: componentNames[0],
		},
		{
			Name: componentNames[1],
		},
	}
	cluster := builder.NewClusterBuilder(namespace, clusterName).SetComponentSpecs(specs).GetObject()
	its0 := builder.NewInstanceSetBuilder(namespace, constant.GenerateClusterComponentName(clusterName, componentNames[0])).GetObject()
	its1 := builder.NewInstanceSetBuilder(namespace, constant.GenerateClusterComponentName(clusterName, componentNames[1])).GetObject()
	node0 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "node-0",
		},
	}
	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "node-1",
		},
	}

	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(ncs)
	Expect(tree.Add(cluster, its0, its1, node0, node1))

	return tree
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	model.AddScheme(experimentalv1alpha1.AddToScheme)
	model.AddScheme(appsv1alpha1.AddToScheme)
	model.AddScheme(appsv1.AddToScheme)
	model.AddScheme(workloads.AddToScheme)

	//+kubebuilder:scaffold:scheme

})

var _ = AfterSuite(func() {
})
