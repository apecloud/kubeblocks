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

package organization

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"fmt"
	"os"

	"k8s.io/cli-runtime/pkg/genericiooptions"
)

type MockOrganization struct {
	genericiooptions.IOStreams
}

func (m *MockOrganization) getOrganization(name string) (*OrgItem, error) {
	return &OrgItem{
		ID:          "test",
		Name:        "test",
		Role:        "test",
		Description: "test",
		DisplayName: "test",
		CreatedAt:   "test",
	}, nil
}

func (m *MockOrganization) GetOrganizations() (*Organizations, error) {
	return &Organizations{
		Items: []OrgItem{
			{
				ID:          "test",
				Name:        "test",
				Role:        "test",
				Description: "test",
				DisplayName: "test",
				CreatedAt:   "test",
			},
		},
	}, nil
}

func (m *MockOrganization) switchOrganization(name string) (string, error) {
	fmt.Printf("switch to %s\n", name)
	return "", nil
}

func (m *MockOrganization) getCurrentOrganization() (string, error) {
	fmt.Printf("get current organization\n")
	return "", nil
}

func (m *MockOrganization) addOrganization(body []byte) error {
	fmt.Printf("add organization %s\n", string(body))
	return nil
}

func (m *MockOrganization) deleteOrganization(name string) error {
	fmt.Printf("delete organization %s\n", name)
	return nil
}

func (m *MockOrganization) IsValidOrganization(name string) (bool, error) {
	fmt.Printf("check organization %s\n", name)
	return true, nil
}

var _ = Describe("Test Organization", func() {
	var (
		streams genericiooptions.IOStreams
		o       *OrganizationOption
	)
	BeforeEach(func() {
		streams, _, _, _ = genericiooptions.NewTestIOStreams()
		o = &OrganizationOption{IOStreams: streams, Organization: &MockOrganization{}}
		os.Setenv("TEST_ENV", "true")
	})

	AfterEach(func() {
		defer os.Unsetenv("TEST_ENV")
	})

	Context("test organization", func() {
		args := []string{"test", "test", "test"}

		It("test organization list ", func() {
			cmd := newOrgListCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runList()).Should(Succeed())
		})

		It("test organization switch ", func() {
			cmd := newOrgListCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runSwitch()).Should(Succeed())
		})

		It("test organization current ", func() {
			cmd := newOrgListCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runCurrent()).Should(Succeed())
		})

		It("test organization describe ", func() {
			cmd := newOrgListCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runDescribe()).Should(Succeed())
		})
	})
})
