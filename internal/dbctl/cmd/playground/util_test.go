/*
Copyright © 2022 The dbctl Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package playground

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

var _ = Describe("util", func() {
	It("Print Guide info", func() {
		clusterInfo := types.PlaygroundInfo{
			DBCluster:     "mycluster",
			DBPort:        "3306",
			DBNamespace:   "default",
			Namespace:     "dbctl-playground",
			GrafanaSvc:    "prometheus-grafana",
			GrafanaPort:   "9100",
			GrafanaUser:   "admin",
			GrafanaPasswd: "prom-operator",
		}
		Expect(printPlaygroundGuide(clusterInfo)).Should(Succeed())
	})
})
