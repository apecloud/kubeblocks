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

	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
		o = &CloudOrganization{Token: "test_token", APIURL: server.URL, APIPath: APIPath}
		os.Setenv("TEST_ENV", "true")
	})

	AfterEach(func() {
		defer os.Unsetenv("TEST_ENV")
	})

	Context("test cloud organization", func() {
		Expect(SetCurrentOrgAndContext(&CurrentOrgAndContext{
			CurrentOrganization: "test_org",
			CurrentContext:      "test_context",
		})).Should(BeNil())

		It("test getOrganization ", func() {
			ExpectWithOffset(1, func() error {
				_, err := o.getOrganization("test_org")
				return err
			}()).To(BeNil())
		})

		It("test GetOrganizations ", func() {
			ExpectWithOffset(1, func() error {
				_, err := o.GetOrganizations()
				return err
			}()).To(BeNil())
		})

		It("test GetCurrentOrgAndContext ", func() {
			ExpectWithOffset(1, func() error {
				_, err := GetCurrentOrgAndContext()
				return err
			}()).To(BeNil())
		})

		It("test switchOrganization ", func() {
			ExpectWithOffset(1, func() error {
				_, err := o.switchOrganization("test_org")
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
				err = o.addOrganization(body)
				return err
			}()).To(BeNil())
		})

		It("test deleteOrganization ", func() {
			ExpectWithOffset(1, func() error {
				err := o.deleteOrganization("test_org")
				return err
			}()).To(BeNil())
		})
	})
})
