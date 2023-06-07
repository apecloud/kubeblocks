package dashboard

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("grafana open flag", func() {

	var testCharType string
	expectUrl := map[string]string{
		"apecloud-mysql": "http://127.0.0.1:8080/d/apecloud-mysql",
		"cadvisor":       "http://127.0.0.1:8080/d/cadvisor",
		"jmx":            "http://127.0.0.1:8080/d/jmx",
		"kafka":          "http://127.0.0.1:8080/d/kafka",
		"mongodb":        "http://127.0.0.1:8080/d/mongodb",
		"node":           "http://127.0.0.1:8080/d/node",
		"postgresql":     "http://127.0.0.1:8080/d/postgresql",
		"redis":          "http://127.0.0.1:8080/d/redis",
		"weaviate":       "http://127.0.0.1:8080/d/weaviate",
	}

	It("build grafana direct url", func() {
		testUrl := "http://127.0.0.1:8080"
		testCharType = "invalid"
		Expect(buildGrafanaDirectUrl(&testUrl, testCharType)).Should(HaveOccurred())
		for charType, targetUrl := range expectUrl {
			testUrl = "http://127.0.0.1:8080"
			Expect(buildGrafanaDirectUrl(&testUrl, charType)).Should(Succeed())
			Expect(testUrl).Should(Equal(targetUrl))
		}
	})
})
