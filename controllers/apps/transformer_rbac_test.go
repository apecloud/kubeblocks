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

package apps

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/factory"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

var _ = Describe("object rbac transformer test.", func() {
	const (
		clusterName        = "test-cluster"
		clusterDefName     = "test-clusterdef"
		clusterVersionName = "test-clusterversion"
		compName           = "compName"
		compDefName        = "compDefName"
		serviceAccountName = "kb-" + clusterName
	)

	var transCtx graph.TransformContext
	var ctx context.Context
	var logger logr.Logger
	var dag *graph.DAG
	var transformer graph.Transformer
	var cluster *appsv1alpha1.Cluster
	var clusterDefObj *appsv1alpha1.ClusterDefinition
	var saKey types.NamespacedName
	var allSettings map[string]interface{}

	BeforeEach(func() {
		ctx = context.Background()
		logger = logf.FromContext(ctx).WithValues("transformer-rbac-test", "testnamespace")
		By("Creating a cluster")
		cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefName, clusterVersionName).WithRandomName().
			AddComponent(compName, compDefName).
			SetServiceAccountName(serviceAccountName).GetObject()
		r := int32(1)
		cluster.Spec.Replicas = &r
		clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(testapps.StatefulMySQLComponent, "sts").
			GetObject()
		clusterDefObj.Spec.ComponentDefs[0].Probes = &appsv1alpha1.ClusterDefinitionProbes{}
		saKey = types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      serviceAccountName,
		}

		transCtx = &ClusterTransformContext{
			Context:       ctx,
			Client:        k8sClient,
			EventRecorder: nil,
			Logger:        logger,
			Cluster:       cluster,
			ClusterDef:    clusterDefObj,
		}

		dag = mockDAG(cluster)
		transformer = &RBACTransformer{}
		allSettings = viper.AllSettings()
		viper.SetDefault(constant.EnableRBACManager, true)
	})

	AfterEach(func() {
		viper.SetDefault(constant.EnableRBACManager, false)
		if allSettings != nil {
			Expect(viper.MergeConfigMap(allSettings)).ShouldNot(HaveOccurred())
			allSettings = nil
		}
	})

	Context("transformer rbac manager", func() {
		It("create serviceaccount, rolebinding if not exist", func() {
			clusterDefObj.Spec.ComponentDefs[0].VolumeProtectionSpec = nil
			Eventually(testapps.CheckObjExists(&testCtx, saKey,
				&corev1.ServiceAccount{}, false)).Should(Succeed())
			Expect(transformer.Transform(transCtx, dag)).Should(BeNil())

			serviceAccount := factory.BuildServiceAccount(cluster)
			serviceAccount.Name = serviceAccountName

			roleBinding := factory.BuildRoleBinding(cluster)
			roleBinding.Subjects[0].Name = serviceAccountName

			dagExpected := mockDAG(cluster)
			ictrltypes.LifecycleObjectCreate(dagExpected, serviceAccount, nil)
			ictrltypes.LifecycleObjectCreate(dagExpected, roleBinding, nil)
			Expect(dag.Equals(dagExpected, model.DefaultLess)).Should(BeTrue())
		})

		It("create clusterrolebinding if volumeprotection enabled", func() {
			clusterDefObj.Spec.ComponentDefs[0].VolumeProtectionSpec = &appsv1alpha1.VolumeProtectionSpec{}
			Eventually(testapps.CheckObjExists(&testCtx, saKey,
				&corev1.ServiceAccount{}, false)).Should(Succeed())
			Expect(transformer.Transform(transCtx, dag)).Should(BeNil())

			serviceAccount := factory.BuildServiceAccount(cluster)
			serviceAccount.Name = serviceAccountName

			roleBinding := factory.BuildRoleBinding(cluster)
			roleBinding.Subjects[0].Name = serviceAccountName

			clusterRoleBinding := factory.BuildClusterRoleBinding(cluster)
			clusterRoleBinding.Subjects[0].Name = serviceAccountName

			dagExpected := mockDAG(cluster)
			ictrltypes.LifecycleObjectCreate(dagExpected, serviceAccount, nil)
			ictrltypes.LifecycleObjectCreate(dagExpected, roleBinding, nil)
			ictrltypes.LifecycleObjectCreate(dagExpected, clusterRoleBinding, nil)
			Expect(dag.Equals(dagExpected, model.DefaultLess)).Should(BeTrue())
		})
	})
})

func mockDAG(cluster *appsv1alpha1.Cluster) *graph.DAG {
	d := graph.NewDAG()
	ictrltypes.LifecycleObjectCreate(d, cluster, nil)
	root, err := ictrltypes.FindRootVertex(d)
	Expect(err).Should(BeNil())
	sts := &appsv1.StatefulSet{}
	ictrltypes.LifecycleObjectCreate(d, sts, root)
	deploy := &appsv1.Deployment{}
	ictrltypes.LifecycleObjectCreate(d, deploy, root)
	return d
}
