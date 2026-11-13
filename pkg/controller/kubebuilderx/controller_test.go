/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"testing"
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
		DescribeTable("retains delayed Continue wakeups through later steps", func(next Result, stepErr error, wantAfter time.Duration) {
			root := builder.NewPodBuilder(namespace, name).GetObject()
			cli := fake.NewClientBuilder().WithObjects(root).Build()
			tree := NewObjectTree()
			tree.SetRoot(root)
			tree.EventRecorder = record.NewFakeRecorder(100)
			ctrlr := NewController(ctx, cli, ctrl.Request{}, tree.EventRecorder, logger).
				Prepare(&dummyLoader{tree: tree}).
				Do(&dummyReconciler{res: ContinueWithRetryAfter(time.Second)}).
				Do(&dummyReconciler{res: next, err: stepErr})
			if next.Next != cntn {
				ctrlr = ctrlr.Do(&dummyReconciler{err: fmt.Errorf("step after Retry or Commit must not run")})
			}
			res, err := ctrlr.Commit()
			if stepErr != nil {
				Expect(err).To(Equal(stepErr))
				Expect(res).To(BeZero())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(wantAfter))
			Expect(res.Requeue).To(BeTrue())
			committed := builder.NewConfigMapBuilder("hello", "world").GetObject()
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(committed), committed)).To(Succeed())
		},
			Entry("plain Continue preserves the wakeup", Continue, nil, time.Second),
			Entry("an earlier Continue wakeup wins", ContinueWithRetryAfter(500*time.Millisecond), nil, 500*time.Millisecond),
			Entry("Retry stops the chain and preserves the earlier wakeup", RetryAfter(2*time.Second), nil, time.Second),
			Entry("immediate Retry takes precedence and stops the chain", RetryAfter(0), nil, time.Duration(0)),
			Entry("Commit stops the chain and preserves the wakeup", Commit, nil, time.Second),
			Entry("Continue still executes a failing later step", Continue, fmt.Errorf("later step failed"), time.Duration(0)),
		)

		It("should work well", func() {
			ctx := context.Background()
			cli := fake.NewFakeClient()
			req := ctrl.Request{}
			logger := log.FromContext(ctx).WithValues("InstanceSet", "test")
			var ctrlr Controller
			tree := NewObjectTree()

			By("Load tree and reconcile it with no error")
			ctrlr = NewController(ctx, cli, req, nil, logger)
			res, err := ctrlr.Prepare(&dummyLoader{tree: tree}).Do(&dummyReconciler{res: Commit}).Commit()
			Expect(err).Should(BeNil())
			Expect(res.Requeue).Should(BeFalse())

			By("Load tree with error")
			ctrlr = NewController(ctx, cli, req, nil, logger)
			loadErr := fmt.Errorf("load tree failed")
			res, err = ctrlr.Prepare(&dummyLoader{err: loadErr}).Do(&dummyReconciler{res: Commit}).Commit()
			Expect(err).Should(Equal(loadErr))
			Expect(res.Requeue).Should(BeFalse())

			By("Reconcile with pre-condition error")
			ctrlr = NewController(ctx, cli, req, nil, logger)
			reconcileCondErr := fmt.Errorf("reconcile pre-condition failed")
			res, err = ctrlr.Prepare(&dummyLoader{tree: tree}).Do(&dummyReconciler{preErr: reconcileCondErr, res: Commit}).Commit()
			Expect(err).Should(Equal(reconcileCondErr))
			Expect(res.Requeue).Should(BeFalse())

			By("Reconcile with pre-condition unsatisfied")
			ctrlr = NewController(ctx, cli, req, nil, logger)
			root := builder.NewPodBuilder(namespace, name).GetObject()
			tree.SetRoot(root)
			Expect(cli.Create(ctx, root)).Should(Succeed())
			newTree := NewObjectTree()
			newTree.SetRoot(root)
			res, err = ctrlr.Prepare(&dummyLoader{tree: newTree}).Do(&dummyReconciler{unsatisfied: true, res: Commit}).Commit()
			Expect(err).Should(BeNil())
			Expect(res.Requeue).Should(BeFalse())
			Expect(newTree).Should(Equal(tree))

			By("Skip current Do but continue later chained Do calls")
			ctrlr = NewController(ctx, cli, req, nil, logger)
			newTree = NewObjectTree()
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(root), root)).Should(Succeed())
			newTree.SetRoot(root)
			ctrlr = ctrlr.Prepare(&dummyLoader{tree: newTree}).
				Do(&dummyReconciler{unsatisfied: true, res: Commit}).
				Do(&dummyReconciler{res: Continue})
			ctl, ok := ctrlr.(*controller)
			Expect(ok).Should(BeTrue())
			Expect(ctl.err).Should(BeNil())
			Expect(ctl.tree.GetSecondaryObjects()).Should(HaveLen(1))

			By("Reconcile with error")
			ctrlr = NewController(ctx, cli, req, nil, logger)
			reconcileErr := fmt.Errorf("reconcile with error")
			res, err = ctrlr.Prepare(&dummyLoader{tree: tree}).Do(&dummyReconciler{err: reconcileErr, res: Commit}).Commit()
			Expect(err).Should(Equal(reconcileErr))
			Expect(res.Requeue).Should(BeFalse())

			By("Reconcile with Commit method")
			ctrlr = NewController(ctx, cli, req, nil, logger)
			newTree = NewObjectTree()
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(root), root)).Should(Succeed())
			newTree.SetRoot(root)
			res, err = ctrlr.Prepare(&dummyLoader{tree: newTree}).Do(&dummyReconciler{res: Commit}).Commit()
			Expect(err).Should(BeNil())
			Expect(res.Requeue).Should(BeFalse())
			Expect(newTree).Should(Equal(tree))

			By("Reconcile with Retry method")
			ctrlr = NewController(ctx, cli, req, nil, logger)
			newTree = NewObjectTree()
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(root), root)).Should(Succeed())
			newTree.SetRoot(root)
			res, err = ctrlr.Prepare(&dummyLoader{tree: newTree}).Do(&dummyReconciler{res: RetryAfter(time.Second)}).Commit()
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
		return ConditionUnsatisfiedWithError(d.preErr)
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

func TestOriginalRootPreservesLoadedValues(t *testing.T) {
	tree := NewObjectTree()
	if tree.GetOriginalRoot() != nil {
		t.Fatal("an unprepared tree has an original root")
	}
	root := builder.NewPodBuilder("default", "original").GetObject()
	root.Labels = map[string]string{"value": "loaded"}
	tree.SetRoot(root)
	cli := fake.NewClientBuilder().WithObjects(root).Build()
	c := NewController(context.Background(), cli, ctrl.Request{}, nil, logr.Discard()).
		Prepare(&dummyLoader{tree: tree}).(*controller)
	if c.err != nil {
		t.Fatal(c.err)
	}
	c.tree.GetRoot().SetLabels(map[string]string{"value": "changed"})
	original := c.tree.GetOriginalRoot()
	if original.GetLabels()["value"] != "loaded" {
		t.Fatal("the original root contains a reconciler's changes")
	}
	original.GetLabels()["value"] = "changed copy"
	if c.tree.GetOriginalRoot().GetLabels()["value"] != "loaded" || c.oldTree.GetRoot().GetLabels()["value"] != "loaded" {
		t.Fatal("changing the returned copy mutated the loaded root")
	}
	copy, err := c.tree.DeepCopy()
	if err != nil {
		t.Fatal(err)
	}
	if copy.GetOriginalRoot().GetLabels()["value"] != "loaded" || copy.GetRoot().GetLabels()["value"] != "changed" {
		t.Fatal("copying the working tree lost its original or current values")
	}
}
