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

package apps

import (
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("cluster service transformer test", func() {
	const (
		clusterName    = "test-cluster"
		clusterDefName = "test-clusterdef"
	)

	var (
		reader   *mockReader
		dag      *graph.DAG
		transCtx *clusterTransformContext
	)

	newDAG := func(graphCli model.GraphClient, cluster *appsv1.Cluster) *graph.DAG {
		d := graph.NewDAG()
		graphCli.Root(d, cluster, cluster, model.ActionStatusPtr())
		return d
	}

	BeforeEach(func() {
		reader = &mockReader{}
		graphCli := model.NewGraphClient(reader)
		cluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName).
			SetReplicas(1).
			AddService(appsv1.ClusterService{
				Service: appsv1.Service{
					Name:        testapps.ServiceNodePortName,
					ServiceName: testapps.ServiceNodePortName,
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeNodePort,
					},
				},
			}).
			GetObject()

		dag = newDAG(graphCli, cluster)
		transCtx = &clusterTransformContext{
			Context:     ctx,
			Client:      graphCli,
			Logger:      logger,
			Cluster:     cluster,
			OrigCluster: cluster.DeepCopy(),
		}
	})

	clusterServiceName := func(clusterName, svcName string) string {
		return clusterName + "-" + svcName
	}

	clusterNodePortService := func() *corev1.Service {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      clusterServiceName(clusterName, testapps.ServiceNodePortName),
				Labels:    constant.GetClusterLabels(clusterName),
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeNodePort,
			},
		}
		err := controllerutil.SetOwnerReference(transCtx.Cluster, svc)
		Expect(err).Should(BeNil())
		return svc
	}

	Context("cluster service", func() {
		It("deletion", func() {
			reader.objs = append(reader.objs, clusterNodePortService())
			// remove cluster services
			transCtx.Cluster.Spec.Services = nil
			transformer := &clusterServiceTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check services to delete
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &corev1.Service{})
			Expect(len(objs)).Should(Equal(len(reader.objs)))
			slices.SortFunc(objs, func(a, b client.Object) int {
				return strings.Compare(a.GetName(), b.GetName())
			})
			for i := 0; i < len(reader.objs); i++ {
				svc := objs[i].(*corev1.Service)
				Expect(svc.Name).Should(Equal(clusterServiceName(clusterName, testapps.ServiceNodePortName)))
				Expect(graphCli.IsAction(dag, svc, model.ActionDeletePtr())).Should(BeTrue())
			}
		})
	})

	It("ports changed", func() {
		port := corev1.ServicePort{Name: "client", Port: 2379, TargetPort: intstr.FromInt32(2379), NodePort: 30001}
		newPorts := []corev1.ServicePort{
			{Name: "server", Port: 2380, NodePort: 30002},
			{Name: port.Name, Port: port.Port, TargetPort: intstr.FromInt32(0), NodePort: 0},
		}
		expectedPorts := []corev1.ServicePort{
			{Name: "client", Port: 2379, TargetPort: intstr.FromInt32(2379), NodePort: 30001},
			{Name: "server", Port: 2380, NodePort: 30002},
		}
		svc := clusterNodePortService()
		svc.Spec.Ports = []corev1.ServicePort{port}
		reader.objs = append(reader.objs, svc)
		transCtx.Cluster.Spec.Services[0].Spec.Ports = newPorts
		transformer := &clusterServiceTransformer{}
		err := transformer.Transform(transCtx, dag)
		Expect(err).Should(BeNil())

		// check services to make change
		graphCli := transCtx.Client.(model.GraphClient)
		objs := graphCli.FindAll(dag, &corev1.Service{})
		Expect(len(objs)).Should(Equal(len(transCtx.Cluster.Spec.Services)))

		for i := 0; i < len(transCtx.Cluster.Spec.Services); i++ {
			svc := objs[i].(*corev1.Service)
			slices.SortFunc(svc.Spec.Ports, func(a, b corev1.ServicePort) int { return strings.Compare(a.Name, b.Name) })
			slices.SortFunc(expectedPorts, func(a, b corev1.ServicePort) int { return strings.Compare(a.Name, b.Name) })
			Expect(svc.Spec.Ports).Should(Equal(expectedPorts))
			Expect(graphCli.IsAction(dag, svc, model.ActionUpdatePtr())).Should(BeTrue())
		}
	})
})
