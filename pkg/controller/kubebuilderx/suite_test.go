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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	"github.com/apecloud/kubeblocks/pkg/testutil/k8s/mocks"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	mockController *gomock.Controller
	k8sMock        *mocks.MockClient
	ctx            context.Context
	logger         logr.Logger
)

func init() {
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "KubeBuilderX Suite")
}

var _ = BeforeSuite(func() {
	mockController, k8sMock = testutil.SetupK8sMock()
	ctx = context.Background()
	logger = logf.FromContext(ctx).WithValues("kubebuilderx-test", "foo")

	go func() {
		defer GinkgoRecover()
	}()
})

var _ = AfterSuite(func() {
	mockController.Finish()
})
