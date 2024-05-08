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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
			req := ctrl.Request{}
			logger := log.FromContext(ctx).WithValues("InstanceSet", "test")
			controller := NewController(context.Background(), nil, req, nil, logger)
			tree := NewObjectTree()

			By("Load tree and reconcile it with no error")
			err := controller.Prepare(&dummyLoader{tree: tree}).Do(&dummyReconciler{}).Commit()
			Expect(err).Should(BeNil())

			By("Load tree with error")
			loadErr := fmt.Errorf("load tree failed")
			err = controller.Prepare(&dummyLoader{err: loadErr}).Do(&dummyReconciler{}).Commit()
			Expect(err).Should(Equal(loadErr))

			By("Reconcile with pre-condition error")
			reconcileCondErr := fmt.Errorf("reconcile pre-condition failed")
			err = controller.Prepare(&dummyLoader{tree: tree}).Do(&dummyReconciler{preErr: reconcileCondErr}).Commit()
			Expect(err).Should(Equal(reconcileCondErr))

			By("Reconcile with pre-condition unsatisfied")
			Expect(tree.Add(builder.NewPodBuilder(namespace, name).GetObject())).Should(Succeed())
			newTree, err := tree.DeepCopy()
			Expect(err).Should(BeNil())
			err = controller.Prepare(&dummyLoader{tree: newTree}).Do(&dummyReconciler{unsatisfied: true}).Commit()
			Expect(err).Should(BeNil())
			Expect(newTree).Should(Equal(tree))

			By("Reconcile with error")
			reconcileErr := fmt.Errorf("reconcile with error")
			err = controller.Prepare(&dummyLoader{tree: tree}).Do(&dummyReconciler{err: reconcileErr}).Commit()
			Expect(err).Should(Equal(reconcileErr))
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
	err         error
}

func (d *dummyReconciler) PreCondition(tree *ObjectTree) *CheckResult {
	if d.preErr != nil {
		return CheckResultWithError(d.preErr)
	}
	if d.unsatisfied {
		return ResultUnsatisfied
	}
	return ResultSatisfied
}

func (d *dummyReconciler) Reconcile(tree *ObjectTree) (*ObjectTree, error) {
	if tree != nil {
		if err := tree.Add(builder.NewConfigMapBuilder("hello", "world").GetObject()); err != nil {
			return nil, err
		}
	}
	return tree, d.err
}

var _ Reconciler = &dummyReconciler{}
