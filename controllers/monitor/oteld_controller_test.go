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

package monitor

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var OTeldSignature = func(_ v1alpha1.OTeld, _ *v1alpha1.OTeld, _ v1alpha1.OTeldList, _ *v1alpha1.OTeldList) {}

const (
	logsSinkName    = "loki"
	metricsSinkName = "prometheus"
)

var _ = Describe("Oteld Monitor Controller", func() {
	Context("OTeld Controller", func() {
		It("reconcile", func() {
			otled := mockOTeldInstance()

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(otled),
				func(g Gomega, oteld *v1alpha1.OTeld) {
					g.Expect(oteld.Status.ObservedGeneration).Should(BeEquivalentTo(1))
				}), time.Second*30, time.Second*1).Should(Succeed())
		})
	})
})

func mockOTeldInstance() *v1alpha1.OTeld {
	oteld := testapps.CreateCustomizedObj(&testCtx, "monitor/oteld.yaml", &v1alpha1.OTeld{},
		testCtx.UseDefaultNamespace(),
		testapps.WithName("oteld"))

	testapps.CreateCustomizedObj(&testCtx, "monitor/metrics_exporter.yaml", &v1alpha1.MetricsExporterSink{},
		testCtx.UseDefaultNamespace(),
		testapps.WithName(metricsSinkName))

	testapps.CreateCustomizedObj(&testCtx, "monitor/logs_exporter.yaml", &v1alpha1.LogsExporterSink{},
		testCtx.UseDefaultNamespace(),
		testapps.WithName(logsSinkName))

	return oteld
}
