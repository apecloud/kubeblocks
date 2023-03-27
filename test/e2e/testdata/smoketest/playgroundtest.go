/*
Copyright ApeCloud, Inc.

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

package smoketest

import (
	"log"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	e2eutil "github.com/apecloud/kubeblocks/test/e2e/util"
)

func PlaygroundTest() {
	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	Context("KubeBlocks playground test", func() {
		It("kbcli playground init", func() {
			cmd := "kbcli playground init"
			execResult := e2eutil.ExecCommand(cmd)
			log.Println(execResult)
			checkPlaygroundInit()
		})
		It("kbcli playground destroy", func() {
			cmd := "kbcli playground destroy"
			execResult := e2eutil.ExecCommand(cmd)
			log.Println(execResult)
		})
	})
}

func checkPlaygroundInit() {
	cmd := "kbcli cluster list | grep mycluster | awk '{print $6}'"
	clusterStatus := e2eutil.ExecCommand(cmd)
	Eventually(func(g Gomega) {
		g.Expect(e2eutil.StringStrip(clusterStatus)).Should(Equal("Running"))
	}, timeout, interval).Should(Succeed())

	commond := "kubectl get pod | grep mycluster | awk '{print $3}'"
	podStatus := e2eutil.ExecCommand(commond)
	Eventually(func(g Gomega) {
		g.Expect(e2eutil.StringStrip(podStatus)).Should(Equal("Running"))
	}, timeout, interval).Should(Succeed())
}
