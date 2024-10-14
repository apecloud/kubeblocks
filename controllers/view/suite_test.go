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

package view

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	// +kubebuilder:scaffold:imports

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	scheme = runtime.NewScheme()

	namespace       = "foo"
	name            = "bar"
	uid             = uuid.NewUUID()
	resourceVersion = "612345"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	err := viewv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = kbappsv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	go func() {
		defer GinkgoRecover()
	}()
})

var _ = AfterSuite(func() {
})
