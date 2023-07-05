package smoketest

import (
	"os"

	. "github.com/onsi/ginkgo/v2"

	. "github.com/apecloud/kubeblocks/test/e2e"
	e2eutil "github.com/apecloud/kubeblocks/test/e2e/util"
)

func UploadReport() {

	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	Context("upload test report to s3", func() {
		It("upload report", func() {
			path, _ := os.Getwd()
			e2eutil.UploadToS3(path+"/report.json", "e2e/"+Version, "e2e-test")
		})

	})
}
