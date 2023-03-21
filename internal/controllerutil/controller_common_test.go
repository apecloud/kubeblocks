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

package controllerutil

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var tlog = ctrl.Log.WithName("controller_testing")

func TestRequeueWithError(t *testing.T) {
	_, err := CheckedRequeueWithError(errors.New("test error"), tlog, "test")
	if err == nil {
		t.Error("Expected error to fall through, got nil")
	}

	// test with empty msg
	_, err = CheckedRequeueWithError(errors.New("test error"), tlog, "")
	if err == nil {
		t.Error("Expected error to fall through, got nil")
	}
}

func TestRequeueWithNotFoundError(t *testing.T) {
	notFoundErr := apierrors.NewNotFound(schema.GroupResource{
		Resource: "Pod",
	}, "no-body")
	_, err := CheckedRequeueWithError(notFoundErr, tlog, "test")
	if err != nil {
		t.Error("Expected error to be nil, got:", err)
	}
}

func TestRequeueAfter(t *testing.T) {
	_, err := RequeueAfter(time.Millisecond, tlog, "test")
	if err != nil {
		t.Error("Expected error to be nil, got:", err)
	}

	// test with empty msg
	_, err = RequeueAfter(time.Millisecond, tlog, "")
	if err != nil {
		t.Error("Expected error to be nil, got:", err)
	}
}

func TestRequeue(t *testing.T) {
	res, err := Requeue(tlog, "test")
	if err != nil {
		t.Error("Expected error to be nil, got:", err)
	}
	if !res.Requeue {
		t.Error("Expected requeue to be true, got false")
	}

	// test with empty msg
	res, err = Requeue(tlog, "")
	if err != nil {
		t.Error("Expected error to be nil, got:", err)
	}
	if !res.Requeue {
		t.Error("Expected requeue to be true, got false")
	}
}

func TestReconciled(t *testing.T) {
	res, err := Reconciled()
	if err != nil {
		t.Error("Expected error to be nil, got:", err)
	}
	if res.Requeue {
		t.Error("Expected requeue to be false, got true")
	}
}

var _ = Describe("Cluster Controller", func() {

	const finalizer = "finalizer/protection"
	const referencedLabelKey = "referenced/name"

	getObj := func(key client.ObjectKey) *corev1.ConfigMap {
		cm := &corev1.ConfigMap{}
		Ω(k8sClient.Get(context.Background(), key, cm)).ShouldNot(HaveOccurred())
		return cm
	}

	getObjWithFinalizer := func(key client.ObjectKey) *corev1.ConfigMap {
		obj := getObj(key)
		res, err := HandleCRDeletion(reqCtx, k8sClient, obj, finalizer, nil)
		Ω(err).ShouldNot(HaveOccurred())
		Expect(res).Should(BeNil())
		Expect(controllerutil.ContainsFinalizer(obj, finalizer)).Should(BeTrue())
		return getObj(key)
	}

	getDeletingObj := func(key client.ObjectKey) *corev1.ConfigMap {
		obj := getObj(key)
		Expect(controllerutil.ContainsFinalizer(obj, finalizer)).Should(BeTrue())
		Expect(obj.DeletionTimestamp.IsZero()).ShouldNot(BeTrue())
		return obj
	}

	createObj := func() (*corev1.ConfigMap, client.ObjectKey) {
		By("By create obj")
		randomStr, _ := password.Generate(6, 0, 0, true, false)
		key := client.ObjectKey{
			Name:      "cm-" + randomStr,
			Namespace: "default",
		}
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
		}
		Ω(k8sClient.Create(context.Background(), obj)).ShouldNot(HaveOccurred())
		return obj, key
	}

	deleteObj := func(key client.ObjectKey) {
		Eventually(func() error {
			cm := &corev1.ConfigMap{}
			if err := k8sClient.Get(context.Background(), key, cm); err == nil {
				controllerutil.RemoveFinalizer(cm, finalizer)
				return k8sClient.Delete(context.Background(), cm)
			} else {
				return client.IgnoreNotFound(err)
			}
		}).Should(Succeed())
	}

	createReferencedConfigMap := func(label string) *corev1.ConfigMap {
		By("By create obj")
		randomStr, _ := password.Generate(6, 0, 0, true, false)
		key := client.ObjectKey{
			Name:      "cm-" + randomStr,
			Namespace: "default",
		}
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
				Labels: map[string]string{
					referencedLabelKey: label,
				},
			},
		}
		Ω(k8sClient.Create(context.Background(), obj)).ShouldNot(HaveOccurred())
		return obj
	}

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	Context("When calling HandleCRDeletion for non-deleting obj", func() {
		It("Should have finalizer added and not reconciled", func() {
			_, key := createObj()
			By("By calling HandleCRDeletion")
			_ = getObjWithFinalizer(key)
			deleteObj(key)
		})
	})

	Context("When calling HandleCRDeletion for deleting obj", func() {
		It("Do deletion with no finalizer should simply reconciled", func() {
			obj, key := createObj()
			By("By fake delete obj")
			obj.DeletionTimestamp = &metav1.Time{
				Time: time.Now(),
			}
			By("By calling HandleCRDeletion")
			res, err := HandleCRDeletion(reqCtx, k8sClient, obj, finalizer, func() (*ctrl.Result, error) {
				return nil, nil
			})
			Ω(err).ShouldNot(HaveOccurred())
			Expect(res).ShouldNot(BeNil())
			Expect(*res == reconcile.Result{}).Should(BeTrue())

			deleteObj(key)
		})

		It("Do normal deletion with finalizer should have finalizer removed and reconciled", func() {
			_, key := createObj()
			By("By delete obj")
			obj := getObjWithFinalizer(key)
			Ω(k8sClient.Delete(ctx, obj)).ShouldNot(HaveOccurred())

			By("By calling HandleCRDeletion")
			obj = getObj(key)
			res, err := HandleCRDeletion(reqCtx, k8sClient, obj, finalizer, func() (*ctrl.Result, error) {
				return nil, nil
			})
			Ω(err).ShouldNot(HaveOccurred())
			Expect(controllerutil.ContainsFinalizer(obj, finalizer)).ShouldNot(BeTrue())
			Expect(res).ShouldNot(BeNil())
			Expect(*res == reconcile.Result{}).Should(BeTrue())
		})

		It("Do normal deletion with empty handler should have finalizer removed and reconciled", func() {
			_, key := createObj()
			By("By delete obj")
			obj := getObjWithFinalizer(key)
			Ω(k8sClient.Delete(ctx, obj)).ShouldNot(HaveOccurred())

			By("By calling HandleCRDeletion")
			obj = getDeletingObj(key)
			res, err := HandleCRDeletion(reqCtx, k8sClient, obj, finalizer, nil)
			Ω(err).ShouldNot(HaveOccurred())
			Expect(controllerutil.ContainsFinalizer(obj, finalizer)).ShouldNot(BeTrue())
			Expect(res).ShouldNot(BeNil())
			Expect(*res == reconcile.Result{}).Should(BeTrue())
		})

		It("Do normal deletion with error", func() {
			_, key := createObj()
			By("By delete obj")
			obj := getObjWithFinalizer(key)
			Ω(k8sClient.Delete(ctx, obj)).ShouldNot(HaveOccurred())

			By("By calling HandleCRDeletion")
			obj = getDeletingObj(key)
			res, err := HandleCRDeletion(reqCtx, k8sClient, obj, finalizer, func() (*ctrl.Result, error) {
				return nil, fmt.Errorf("err")
			})
			Ω(err).Should(HaveOccurred())
			Expect(controllerutil.ContainsFinalizer(obj, finalizer)).Should(BeTrue())
			Expect(res).ShouldNot(BeNil())
			Expect(*res == reconcile.Result{}).Should(BeTrue())
		})

		It("Do normal deletion with custom result", func() {
			_, key := createObj()
			By("By delete obj")
			obj := getObjWithFinalizer(key)
			Ω(k8sClient.Delete(ctx, obj)).ShouldNot(HaveOccurred())

			By("By calling HandleCRDeletion")
			obj = getDeletingObj(key)
			cRes := ctrl.Result{
				Requeue: true,
			}
			res, err := HandleCRDeletion(reqCtx, k8sClient, obj, finalizer, func() (*ctrl.Result, error) {
				return &cRes, nil
			})
			Ω(err).ShouldNot(HaveOccurred())
			Expect(controllerutil.ContainsFinalizer(obj, finalizer)).Should(BeTrue())
			Expect(res).ShouldNot(BeNil())
			Expect(*res == cRes).Should(BeTrue())
		})

		It("Do normal deletion with custom result and error", func() {
			_, key := createObj()
			By("By delete obj")
			obj := getObjWithFinalizer(key)
			Ω(k8sClient.Delete(ctx, obj)).ShouldNot(HaveOccurred())

			By("By calling HandleCRDeletion")
			obj = getDeletingObj(key)
			cRes := ctrl.Result{
				Requeue: true,
			}
			res, err := HandleCRDeletion(reqCtx, k8sClient, obj, finalizer, func() (*ctrl.Result, error) {
				return &cRes, fmt.Errorf("err")
			})
			Ω(err).Should(HaveOccurred())
			Expect(controllerutil.ContainsFinalizer(obj, finalizer)).Should(BeTrue())
			Expect(res).ShouldNot(BeNil())
			Expect(*res == cRes).Should(BeTrue())
		})

		It("Do delete CR with existing referencing CR", func() {
			obj, key := createObj()
			_ = createReferencedConfigMap(key.Name)
			recordEvent := func() {
				// mock record
				fmt.Println("mock send event ")
			}
			_, err := ValidateReferenceCR(reqCtx, k8sClient, obj, referencedLabelKey, recordEvent, &corev1.ConfigMapList{})
			Expect(err == nil)
		})
	})
})
