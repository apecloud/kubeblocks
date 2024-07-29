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

package kubebuilderx

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

var _ = Describe("controller test", func() {
	const (
		namespace = "foo"
		name      = "bar"
	)

	Context("Dummy Controller", func() {
		It("should work well", func() {
			ctx := context.Background()
			cli := fake.NewFakeClient()
			req := ctrl.Request{}
			logger := log.FromContext(ctx).WithValues("InstanceSet", "test")
			var controller Controller
			tree := NewObjectTree()

			By("Load tree and reconcile it with no error")
			controller = NewController(ctx, cli, req, nil, logger)
			res, err := controller.Prepare(&dummyLoader{tree: tree}).Do(&dummyReconciler{res: Commit}).Commit()
			Expect(err).Should(BeNil())
			Expect(res.Requeue).Should(BeFalse())

			By("Load tree with error")
			controller = NewController(ctx, cli, req, nil, logger)
			loadErr := fmt.Errorf("load tree failed")
			res, err = controller.Prepare(&dummyLoader{err: loadErr}).Do(&dummyReconciler{res: Commit}).Commit()
			Expect(err).Should(Equal(loadErr))
			Expect(res.Requeue).Should(BeFalse())

			By("Reconcile with pre-condition error")
			controller = NewController(ctx, cli, req, nil, logger)
			reconcileCondErr := fmt.Errorf("reconcile pre-condition failed")
			res, err = controller.Prepare(&dummyLoader{tree: tree}).Do(&dummyReconciler{preErr: reconcileCondErr, res: Commit}).Commit()
			Expect(err).Should(Equal(reconcileCondErr))
			Expect(res.Requeue).Should(BeFalse())

			By("Reconcile with pre-condition unsatisfied")
			controller = NewController(ctx, cli, req, nil, logger)
			root := builder.NewPodBuilder(namespace, name).GetObject()
			tree.SetRoot(root)
			Expect(cli.Create(ctx, root)).Should(Succeed())
			newTree := NewObjectTree()
			newTree.SetRoot(root)
			res, err = controller.Prepare(&dummyLoader{tree: newTree}).Do(&dummyReconciler{unsatisfied: true, res: Commit}).Commit()
			Expect(err).Should(BeNil())
			Expect(res.Requeue).Should(BeFalse())
			Expect(newTree).Should(Equal(tree))

			By("Reconcile with error")
			controller = NewController(ctx, cli, req, nil, logger)
			reconcileErr := fmt.Errorf("reconcile with error")
			res, err = controller.Prepare(&dummyLoader{tree: tree}).Do(&dummyReconciler{err: reconcileErr, res: Commit}).Commit()
			Expect(err).Should(Equal(reconcileErr))
			Expect(res.Requeue).Should(BeFalse())

			By("Reconcile with Commit method")
			controller = NewController(ctx, cli, req, nil, logger)
			newTree = NewObjectTree()
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(root), root)).Should(Succeed())
			newTree.SetRoot(root)
			res, err = controller.Prepare(&dummyLoader{tree: newTree}).Do(&dummyReconciler{res: Commit}).Commit()
			Expect(err).Should(BeNil())
			Expect(res.Requeue).Should(BeFalse())
			Expect(newTree).Should(Equal(tree))

			By("Reconcile with Retry method")
			controller = NewController(ctx, cli, req, nil, logger)
			newTree = NewObjectTree()
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(root), root)).Should(Succeed())
			newTree.SetRoot(root)
			res, err = controller.Prepare(&dummyLoader{tree: newTree}).Do(&dummyReconciler{res: RetryAfter(time.Second)}).Commit()
			Expect(err).Should(BeNil())
			Expect(res.Requeue).Should(BeTrue())
			Expect(res.RequeueAfter).Should(Equal(time.Second))
			Expect(newTree).Should(Equal(tree))
		})
	})
})

type dummyLoader struct {
	err  error
	tree *ObjectTree
}

func (d *dummyLoader) Load(ctx context.Context, reader client.Reader, request ctrl.Request, recorder record.EventRecorder, logger logr.Logger) (*ObjectTree, error) {
	return d.tree, d.err
}

var _ TreeLoader = &dummyLoader{}

type dummyReconciler struct {
	preErr      error
	unsatisfied bool
	res         Result
	err         error
}

func (d *dummyReconciler) PreCondition(tree *ObjectTree) *CheckResult {
	if d.preErr != nil {
		return CheckResultWithError(d.preErr)
	}
	if d.unsatisfied {
		return ConditionUnsatisfied
	}
	return ConditionSatisfied
}

func (d *dummyReconciler) Reconcile(tree *ObjectTree) (Result, error) {
	if d.err != nil || d.res.Next != cntn {
		return d.res, d.err
	}
	if tree != nil {
		if err := tree.Add(builder.NewConfigMapBuilder("hello", "world").GetObject()); err != nil {
			return Result{}, err
		}
	}
	return d.res, d.err
}

var _ Reconciler = &dummyReconciler{}
