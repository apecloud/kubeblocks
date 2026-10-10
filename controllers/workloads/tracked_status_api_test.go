/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package workloads

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

var _ = Describe("InstanceSet tracked status API persistence", func() {
	It("round trips omitted, false, and true tracked facts through the status subresource", func() {
		its := &workloadsv1.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "tracked-facts-api", Namespace: testCtx.DefaultNamespace},
			Spec: workloadsv1.InstanceSetSpec{Replicas: ptr.To[int32](0),
				Selector:         &metav1.LabelSelector{MatchLabels: map[string]string{"app": "tracked-facts-api"}},
				Template:         corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "mysql"}}}},
				OfflineInstances: []string{"tracked-facts-api-0", "tracked-facts-api-1", "tracked-facts-api-2"}}}
		Expect(k8sClient.Create(ctx, its)).To(Succeed())
		key := client.ObjectKeyFromObject(its)
		DeferCleanup(func() {
			Eventually(func() error {
				current := &workloadsv1.InstanceSet{}
				if err := k8sClient.Get(ctx, key, current); err != nil {
					return client.IgnoreNotFound(err)
				}
				current.Finalizers = nil
				if err := k8sClient.Update(ctx, current); err != nil {
					return err
				}
				return client.IgnoreNotFound(k8sClient.Delete(ctx, current))
			}).Should(Succeed())
		})
		facts := []*bool{nil, ptr.To(false), ptr.To(true)}
		for _, values := range [][]*bool{facts, {ptr.To(true), ptr.To(false), nil}} {
			Eventually(func() error {
				if err := k8sClient.Get(ctx, key, its); err != nil {
					return err
				}
				its.Status.InstanceStatus = nil
				for i, name := range its.Spec.OfflineInstances {
					its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloadsv1.InstanceStatus{
						PodName: name, DesiredState: workloadsv1.InstanceDesiredStateOffline, CurrentState: workloadsv1.InstanceCurrentStateAbsent,
						Provisioned: i != 0, DataLoaded: values[i], MemberJoined: values[2-i],
					})
				}
				return k8sClient.Status().Update(ctx, its)
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				stored := &workloadsv1.InstanceSet{}
				g.Expect(k8sClient.Get(ctx, key, stored)).To(Succeed())
				for i, name := range its.Spec.OfflineInstances {
					status := stored.FindInstanceStatus(name)
					g.Expect(status).NotTo(BeNil())
					g.Expect(status.Provisioned).To(Equal(i != 0))
					g.Expect(reflect.DeepEqual(status.DataLoaded, values[i])).To(BeTrue())
					g.Expect(reflect.DeepEqual(status.MemberJoined, values[2-i])).To(BeTrue())
				}
			}).Should(Succeed())
		}
	})
})
