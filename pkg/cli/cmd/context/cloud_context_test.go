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

package context

import (
	ginkgo_context "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/apecloud/kubeblocks/pkg/cli/cmd/organization"
)

func mockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/organizations/test_org/contexts/test_context":
			switch r.Method {
			case http.MethodGet:
				cloudContextResponse := CloudContextResponse{
					Name: "test_context",
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
			cloudContextsResponse := []CloudContextResponse{
				{
					Name: "test_context",
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
