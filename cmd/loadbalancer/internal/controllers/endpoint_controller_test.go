/*
Copyright ApeCloud, Inc.

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

package loadbalancer

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var newEndpointsObj = func(svc *corev1.Service) (*corev1.Endpoints, types.NamespacedName) {
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.GetName(),
			Namespace: svc.GetNamespace(),
			Labels: map[string]string{
				"app": svc.GetName(),
			},
		},
	}
	return endpoints, types.NamespacedName{
		Name:      endpoints.GetName(),
		Namespace: endpoints.GetNamespace(),
	}
}

var _ = Describe("EndpointController", func() {
	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, intctrlutil.ServiceSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.EndpointsSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("", func() {
		It("", func() {
			svc, svcKey := newSvcObj(false, node1IP)
			ep, epKey := newEndpointsObj(svc)
			Expect(testCtx.CreateObj(context.Background(), svc)).Should(Succeed())
			Expect(testCtx.CreateObj(context.Background(), ep)).Should(Succeed())
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), svcKey, svc); err != nil {
					return false
				}
				if err := k8sClient.Get(context.Background(), epKey, ep); err != nil {
					return false
				}
				return svc.Annotations[AnnotationKeyEndpointsVersion] == ep.GetObjectMeta().GetResourceVersion()
			}).Should(BeTrue())
		})
	})
})
