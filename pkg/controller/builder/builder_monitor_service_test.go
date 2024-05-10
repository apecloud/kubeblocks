/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
)

var _ = Describe("monitor_service builder", func() {
	It("should work well", func() {
		const (
			name = "monitor_test"
			ns   = "default"
		)

		exporter := appsv1alpha1.Exporter{
			ScrapePath:   "metrics",
			ScrapePort:   "http-metrics",
			ScrapeScheme: appsv1alpha1.HTTPSProtocol,
		}

		ncs := NewMonitorServiceBuilder(ns, name).
			SetMonitorServiceSpec(monitoringv1.ServiceMonitorSpec{}).
			SetDefaultEndpoint(&common.Exporter{
				Exporter: exporter,
			}).
			GetObject()

		Expect(ncs.Name).Should(Equal(name))
		Expect(ncs.Namespace).Should(Equal(ns))
		Expect(len(ncs.Spec.Endpoints)).Should(Equal(1))
		Expect(ncs.Spec.Endpoints[0].Port).Should(Equal("http-metrics"))
		Expect(ncs.Spec.Endpoints[0].Scheme).Should(Equal("https"))
		Expect(ncs.Spec.Endpoints[0].Path).Should(Equal("metrics"))
	})
})
