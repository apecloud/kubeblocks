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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
			logger := log.FromContext(ctx).WithValues("ReplicatedStateMachine", "test")
			controller := NewController(context.Background(), nil, req, nil, logger)
			err := controller.Prepare(&dummyLoader{}).Do(&dummyReconciler{}).Commit()
			Expect(err).Should(BeNil())
			// TODO test control flow
		})
	})
})

type dummyLoader struct {}

func (d *dummyLoader) Read(ctx context.Context, reader client.Reader, request ctrl.Request, recorder record.EventRecorder, logger logr.Logger) (*ObjectTree, error) {
	//TODO implement me
	panic("implement me")
}

var _ TreeLoader = &dummyLoader{}

type dummyReconciler struct {}

func (d *dummyReconciler) PreCondition(tree *ObjectTree) *CheckResult {
	//TODO implement me
	panic("implement me")
}

func (d *dummyReconciler) Reconcile(tree *ObjectTree) (*ObjectTree, error) {
	//TODO implement me
	panic("implement me")
}

var _ Reconciler = &dummyReconciler{}