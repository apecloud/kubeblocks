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
				for _, file := range files {
					By("test " + file)
					b := e2eutil.OpsYaml(file, "apply")
					Expect(b).Should(BeTrue())
					e2eutil.WaitTime()
					podStatusResult := e2eutil.CheckPodStatus()
					log.Println(podStatusResult)
					for _, result := range podStatusResult {
						Expect(result).Should(BeTrue())
					}
					e2eutil.WaitTime()
					clusterStatusResult := e2eutil.CheckClusterStatus()
					Expect(clusterStatusResult).Should(BeTrue())
				}
				if len(files) > 0 {
					file := e2eutil.GetClusterCreateYaml(files)
					e2eutil.OpsYaml(file, "delete")
				}
			}
		})
	})
}
