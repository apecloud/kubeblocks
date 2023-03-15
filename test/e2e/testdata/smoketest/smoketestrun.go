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
	"os"
	_ "path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	e2eutil "github.com/apecloud/kubeblocks/test/e2e/util"
)

func SmokeTest() {
	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	Context("KubeBlocks smoke test", func() {
		It("check addon auto-install", func() {
			checkAddons()
		})
		It("run test cases", func() {
			dir, err := os.Getwd()
			if err != nil {
				log.Println(err)
			}
			folders, _ := e2eutil.GetFolders(dir + "/testdata/smoketest")
			for _, folder := range folders {
				if folder == dir+"/testdata/smoketest" {
					continue
				}
				log.Println("folder: " + folder)
				files, _ := e2eutil.GetFiles(folder)
				clusterVersions := e2eutil.GetClusterVersion(folder)
				if len(clusterVersions) > 1 {
					for _, clusterVersion := range clusterVersions {
						if len(files) > 0 {
							file := e2eutil.GetClusterCreateYaml(files)
							e2eutil.ReplaceClusterVersionRef(file, clusterVersion)
							runTestCases(files)
						}
					}
				} else {
					runTestCases(files)
				}
			}
		})
	})
}

func runTestCases(files []string) {
	for _, file := range files {
		By("test " + file)
		b := e2eutil.OpsYaml(file, "apply")
		Expect(b).Should(BeTrue())
		e2eutil.WaitTime(400000000)
		podStatusResult := e2eutil.CheckPodStatus()
		log.Println(podStatusResult)
		for _, result := range podStatusResult {
			Expect(result).Should(BeTrue())
		}
		e2eutil.WaitTime(400000000)
		clusterStatusResult := e2eutil.CheckClusterStatus()
		Expect(clusterStatusResult).Should(BeTrue())
	}
	if len(files) > 0 {
		file := e2eutil.GetClusterCreateYaml(files)
		e2eutil.OpsYaml(file, "delete")
	}
}

func checkAddons() {
	e2eutil.WaitTime(500000000)
	kubernetes := e2eutil.KubernetesEnv()
	adaptorStatus := e2eutil.CheckAddonsInstall("alertmanager-webhook-adaptor")
	Expect(adaptorStatus).Should(Equal("Enabled"))
	mysqlStatus := e2eutil.CheckAddonsInstall("apecloud-mysql")
	Expect(mysqlStatus).Should(Equal("Enabled"))
	postgresqlStatus := e2eutil.CheckAddonsInstall("postgresql")
	Expect(postgresqlStatus).Should(Equal("Enabled"))
	grafanaStatus := e2eutil.CheckAddonsInstall("grafana")
	Expect(grafanaStatus).Should(Equal("Enabled"))
	prometheusStatus := e2eutil.CheckAddonsInstall("prometheus")
	Expect(prometheusStatus).Should(Equal("Enabled"))
	if strings.Contains(strings.ToLower(kubernetes), "k3s") {
		snapshotStatus := e2eutil.OpsAddon("enable", "snapshot-controller")
		Expect(snapshotStatus).Should(Equal("Enabled"))
		status := e2eutil.OpsAddon("disable", "snapshot-controller")
		Expect(status).Should(Equal("Disabled"))
	}
	if strings.Contains(strings.ToLower(kubernetes), "eks") {
		lbStatus := e2eutil.OpsAddon("enable", "loadbalancer")
		Expect(lbStatus).Should(Equal("Enabled"))
		status := e2eutil.OpsAddon("disable", "loadbalancer")
		Expect(status).Should(Equal("Disabled"))
	}
	if strings.Contains(strings.ToLower(kubernetes), "eks") ||
		strings.Contains(strings.ToLower(kubernetes), "gke") ||
		strings.Contains(strings.ToLower(kubernetes), "ack") ||
		strings.Contains(strings.ToLower(kubernetes), "aks") {
		csiStatus := e2eutil.OpsAddon("enable", "csi-s3")
		Expect(csiStatus).Should(Equal("Enabled"))
		status := e2eutil.OpsAddon("disable", "csi-s3")
		Expect(status).Should(Equal("Disabled"))
	}
}
