/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package consensusset

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"

	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
	"github.com/apecloud/kubeblocks/internal/testutil/k8s/mocks"
)

var (
	controller *gomock.Controller
	k8sMock    *mocks.MockClient
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

func init() {
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "StatefulReplicaSet Suite")
}

var _ = BeforeSuite(func() {
	controller, k8sMock = testutil.SetupK8sMock()
	go func() {
		defer GinkgoRecover()
	}()
})

var _ = AfterSuite(func() {
	controller.Finish()
})
