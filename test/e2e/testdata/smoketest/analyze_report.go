package smoketest

import (
	"fmt"
	"log"
	"os"

	. "github.com/onsi/ginkgo/v2"

	. "github.com/apecloud/kubeblocks/test/e2e"
	e2eutil "github.com/apecloud/kubeblocks/test/e2e/util"
)

func AnalyzeE2eReport() {

	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	Context("show e2e test report", func() {
		dir, err := os.Getwd()
		if err != nil {
			log.Println(err)
		}
		It("create e2e report file", func() {
			f, err := os.OpenFile(TestType+"-log.txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Println("Failed to open file:", err)
				return
			}
			log.SetOutput(f)
		})
		It("e2e report", func() {
			log.Println("\n====================" + TestType + " e2e test report====================")
			if len(TestResults) > 0 && len(TestType) > 0 {
				files, _ := e2eutil.GetFiles(dir + "/testdata/smoketest/" + TestType)
				if len(files) > len(TestResults) {
					failed := len(files) - len(TestResults)
					log.Println("Total " + fmt.Sprint(len(files)) + " | " + "Passed " +
						fmt.Sprint(len(TestResults)) + " | " + "Failed " + fmt.Sprint(failed))
				} else {
					log.Println("Total " + fmt.Sprint(len(TestResults)) + " | " + "Passed " + fmt.Sprint(len(TestResults)))
				}
				var CaseNames []string
				for _, v := range TestResults {
					CaseNames = append(CaseNames, v.CaseName)
					if v.ExecuteResult {
						log.Printf(" [PASS] [%s] %s %s ", v.CaseName, fmt.Sprint(v.ExecuteResult), v.TroubleShooting)
					} else {
						log.Printf(" [ERROR] [%s] %s %s ", v.CaseName, fmt.Sprint(v.ExecuteResult), v.TroubleShooting)
					}
				}
				if len(files) > len(TestResults) {
					var filesNames []string
					for _, file := range files {
						name := e2eutil.GetPrefix(file, "/")
						filesNames = append(filesNames, name)
					}
					diffs := e2eutil.RemoveElements(filesNames, CaseNames)
					for _, diff := range diffs {
						log.Printf(" [ERROR] [%s] %s %s ", diff, fmt.Sprint(false), "")
					}
				}
			}
			if len(TestResults) == 0 {
				log.Println("[ERROR] create cluster failed")
			}
		})
	})
}
