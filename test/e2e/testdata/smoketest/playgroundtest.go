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
			command := "kbcli cluster delete mycluster --auto-approve"
			log.Println(command)
			result := e2eutil.ExecuteCommand(command)
			Expect(result).Should(BeTrue())
		})
		It("check mycluster and pod", func() {
			command := "kbcli cluster list -A"
			Eventually(func(g Gomega) {
				cluster := e2eutil.ExecCommand(command)
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
	command := "kubectl get pod -n default -l 'app.kubernetes.io/instance in (mycluster)'| grep mycluster |" +
		" awk '{print $3}'"
	log.Println(command)
	Eventually(func(g Gomega) {
		podStatus := e2eutil.ExecCommand(command)
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
