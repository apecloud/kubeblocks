package organization

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type MockOrganization struct {
	genericclioptions.IOStreams
}

func (m *MockOrganization) getOrganization(token string, name string) (*OrgItem, error) {
	return &OrgItem{
		ID:          "test",
		Name:        "test",
		Role:        "test",
		Description: "test",
		DisplayName: "test",
		CreatedAt:   "test",
	}, nil
}

func (m *MockOrganization) GetOrganizations(token string) (*Organizations, error) {
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

func (m *MockOrganization) switchOrganization(token string, name string) (string, error) {
	fmt.Printf("switch to %s\n", name)
	return "", nil
}

func (m *MockOrganization) addOrganization(token string, body []byte) error {
	fmt.Printf("add organization %s\n", string(body))
	return nil
}

func (m *MockOrganization) deleteOrganization(token string, name string) error {
	fmt.Printf("delete organization %s\n", name)
	return nil
}

func (m *MockOrganization) IsValidOrganization(token string, name string) (bool, error) {
	fmt.Printf("check organization %s\n", name)
	return true, nil
}

var _ = Describe("Test Organization", func() {
	var (
		streams genericclioptions.IOStreams
		o       *OrganizationOption
	)
	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		o = &OrganizationOption{IOStreams: streams, Organization: &MockOrganization{}}
	})

	AfterEach(func() {
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

		It("test organization add ", func() {
			cmd := newOrgListCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runAdd()).Should(Succeed())
		})

		It("test organization delete ", func() {
			cmd := newOrgListCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runDelete()).Should(Succeed())
		})
	})
})
