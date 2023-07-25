package organization

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"encoding/json"
	"net/http"
	"net/http/httptest"
)

func mockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/organizations/test_org":
			switch r.Method {
			case http.MethodGet:
				org := OrgItem{
					ID:          "test_id",
					Name:        "test_org",
					Role:        "test_role",
					Description: "test_description",
					DisplayName: "test_display_name",
					CreatedAt:   "test_created_at",
					UpdatedAt:   "test_updated_at",
				}

				jsonData, err := json.Marshal(org)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(jsonData)
			case http.MethodDelete:
				w.WriteHeader(http.StatusOK)
			case http.MethodPost:
				w.WriteHeader(http.StatusCreated)
			default:
				w.WriteHeader(http.StatusNotFound)
			}

		case "/api/v1/organizations":
			orgs := &Organizations{
				Items: []OrgItem{
					{
						ID:          "test_id",
						Name:        "test_org",
						Role:        "test_role",
						Description: "test_description",
						DisplayName: "test_display_name",
					},
				},
			}
			jsonData, err := json.Marshal(orgs)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(jsonData)
		}
	}))
}

var _ = Describe("Test Cloud Organization", func() {
	var (
		o *CloudOrganization
	)
	BeforeEach(func() {
		server := mockServer()
		o = &CloudOrganization{APIURL: server.URL, APIPath: APIPath}
	})

	AfterEach(func() {
	})

	Context("test cloud organization", func() {
		Expect(SetCurrentOrgAndContext(&CurrentOrgAndContext{
			CurrentOrganization: "test_org",
			CurrentContext:      "test_context",
		})).Should(BeNil())

		It("test getOrganization ", func() {
			ExpectWithOffset(1, func() error {
				_, err := o.getOrganization("test_token", "test_org")
				return err
			}()).To(BeNil())
		})

		It("test GetOrganizations ", func() {
			ExpectWithOffset(1, func() error {
				_, err := o.GetOrganizations("test_token")
				return err
			}()).To(BeNil())
		})

		It("test switchOrganization ", func() {
			ExpectWithOffset(1, func() error {
				_, err := o.switchOrganization("test_token", "test_org")
				return err
			}()).To(BeNil())
		})

		It("test addOrganization ", func() {
			ExpectWithOffset(1, func() error {
				org := &OrgItem{
					ID: "test_id",
				}
				body, err := json.Marshal(org)
				Expect(err).To(BeNil())
				err = o.addOrganization("test_token", body)
				return err
			}()).To(BeNil())
		})

		It("test deleteOrganization ", func() {
			ExpectWithOffset(1, func() error {
				err := o.deleteOrganization("test_token", "test_org")
				return err
			}()).To(BeNil())
		})
	})
})
