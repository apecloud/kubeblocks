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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	e2eutil "github.com/apecloud/kubeblocks/test/e2e/util"
)

func PlaygroundInit() {
	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	Context("KubeBlocks playground init", func() {
		It("install kbcli", func() {
			err := e2eutil.CheckKbcliExists()
			if err != nil {
				log.Println(err)
				installCmd := "curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash "
				log.Println(installCmd)
				install := e2eutil.ExecuteCommand(installCmd)
				log.Println(install)
			}
		})
		It("kbcli playground init", func() {
			cmd := "kbcli playground init"
			log.Println(cmd)
			init := e2eutil.ExecuteCommand(cmd)
			log.Println(init)
		})
		It("check kbcli playground cluster and pod status", func() {
			checkPlaygroundCluster()
		})
	})
}

func UninstallKubeblocks() {
	BeforeEach(func() {
	})

	AfterEach(func() {
	})
	Context("KubeBlocks uninstall", func() {
		It("delete mycluster", func() {
			commond := "kbcli cluster delete mycluster --auto-approve"
			log.Println(commond)
			result := e2eutil.ExecuteCommand(commond)
			Expect(result).Should(BeTrue())
		})
		It("check mycluster and pod", func() {
			commond := "kbcli cluster list -A"
			Eventually(func(g Gomega) {
				cluster := e2eutil.ExecCommand(commond)
				g.Expect(e2eutil.StringStrip(cluster)).Should(Equal("Noclusterfound"))
			}, time.Second*10, time.Second*1).Should(Succeed())
			cmd := "kbcli cluster list-instances"
			Eventually(func(g Gomega) {
				instances := e2eutil.ExecCommand(cmd)
				g.Expect(e2eutil.StringStrip(instances)).Should(Equal("Noclusterfound"))
			}, time.Second*10, time.Second*1).Should(Succeed())
		})
		It("kbcli kubeblocks uninstall", func() {
			cmd := "kbcli kubeblocks uninstall --auto-approve --namespace=kb-system"
			log.Println(cmd)
			execResult := e2eutil.ExecuteCommand(cmd)
			log.Println(execResult)
		})
	})
}

func PlaygroundDestroy() {
	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	Context("KubeBlocks playground test", func() {
		It("kbcli playground destroy", func() {
			cmd := "kbcli playground destroy"
			execResult := e2eutil.ExecCommand(cmd)
			log.Println(execResult)
		})
	})
}

func checkPlaygroundCluster() {
	commond := "kubectl get pod -n default -l 'app.kubernetes.io/instance in (mycluster)'| grep mycluster |" +
		" awk '{print $3}'"
	log.Println(commond)
	Eventually(func(g Gomega) {
		podStatus := e2eutil.ExecCommand(commond)
		log.Println(e2eutil.StringStrip(podStatus))
		g.Expect(e2eutil.StringStrip(podStatus)).Should(Equal("Running"))
	}, time.Second*180, time.Second*1).Should(Succeed())
	cmd := "kbcli cluster list | grep mycluster | awk '{print $6}'"
	log.Println(cmd)
	Eventually(func(g Gomega) {
		clusterStatus := e2eutil.ExecCommand(cmd)
		log.Println(e2eutil.StringStrip(clusterStatus))
		g.Expect(e2eutil.StringStrip(clusterStatus)).Should(Equal("Running"))
	}, time.Second*360, time.Second*1).Should(Succeed())
}
