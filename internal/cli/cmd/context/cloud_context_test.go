package context

import (
	ginkgo_context "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/organization"
)

func mockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/organizations/test_org/contexts/test_context":
			switch r.Method {
			case http.MethodGet:
				cloudContextResponse := CloudContextResponse{
					APIVersion: "test_api_version",
					Kind:       "test_kind",
					Metadata: Metadata{
						Name: "test_context",
					},
				}
				jsonData, err := json.Marshal(cloudContextResponse)
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

		case "/api/v1/organizations/test_org/contexts":
			cloudContextsResponse := CloudContextsResponse{
				APIVersion: "test_api_version",
				Kind:       "test_kind",
				Items: []ClusterItem{
					{
						Metadata: Metadata{
							Name: "test_context",
						},
					},
				},
			}
			jsonData, err := json.Marshal(cloudContextsResponse)
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

var _ = ginkgo_context.Describe("Test Cloud Context", func() {
	var (
		o *CloudContext
	)
	ginkgo_context.BeforeEach(func() {
		server := mockServer()
		o = &CloudContext{
			ContextName: "test_context",
			OrgName:     "test_org",
			APIURL:      server.URL,
			APIPath:     organization.APIPath,
		}
	})

	ginkgo_context.AfterEach(func() {
	})

	ginkgo_context.Context("test cloud context", func() {
		Expect(organization.SetCurrentOrgAndContext(&organization.CurrentOrgAndContext{
			CurrentOrganization: "test_org",
			CurrentContext:      "test_context",
		})).Should(BeNil())

		ginkgo_context.It("test GetContext ", func() {
			ExpectWithOffset(1, func() error {
				_, err := o.GetContext()
				return err
			}()).To(BeNil())
		})

		ginkgo_context.It("test GetContexts ", func() {
			ExpectWithOffset(1, func() error {
				_, err := o.GetContexts()
				return err
			}()).To(BeNil())
		})

		ginkgo_context.It("test getCurrentContext ", func() {
			ExpectWithOffset(1, func() error {
				_, err := o.getCurrentContext()
				return err
			}()).To(BeNil())
		})

		ginkgo_context.It("test useContext ", func() {
			ExpectWithOffset(1, func() error {
				_, err := o.useContext("test_context")
				return err
			}()).To(BeNil())
		})

		ginkgo_context.It("test deleteOrganization ", func() {
			ExpectWithOffset(1, func() error {
				err := o.removeContext()
				return err
			}()).To(BeNil())
		})
	})
})
