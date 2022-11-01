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

package k8score

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Event Controller", func() {
	var ctx = context.Background()
	// eventChan := make(chan *corev1.Event)
	// rec := reconcile.Func(func(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
	//	event := &corev1.Event{}
	//	defer GinkgoRecover()
	//	Expect(k8sClient.Get(ctx, req.NamespacedName, event)).Should(Succeed())
	//	eventChan <- event
	//
	//	return reconcile.Result{}, nil
	// })

	Context("When receiving role changed event", func() {
		It("should handle it properly", func() {
			By("setup event listener")
			// err := ctrl.NewControllerManagedBy(k8sManager).
			//	For(&corev1.Event{}).
			//	Complete(rec)
			// Expect(err).NotTo(HaveOccurred())

			By("send role changed event")
			sndEvent, err := CreateRoleChangedEvent("hello", "leader")
			Expect(err).Should(Succeed())
			Expect(testCtx.CreateObj(ctx, sndEvent)).Should(Succeed())
			Eventually(func() string {
				event := &corev1.Event{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: sndEvent.Namespace,
					Name:      sndEvent.Name,
				}, event)).Should(Succeed())

				return event.InvolvedObject.Name
			}, time.Second*5, time.Second).Should(Equal(sndEvent.InvolvedObject.Name))

			// TODO: an interesting bug
			// event := <-eventChan
			// Expect(event.InvolvedObject.Name).Should(Equal(sndEvent.InvolvedObject.Name))
		})
	})

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &corev1.Event{},
			client.InNamespace(testCtx.DefaultNamespace),
			client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.DeleteAllOf(ctx, &corev1.Pod{},
			client.InNamespace(testCtx.DefaultNamespace),
			client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})
})
