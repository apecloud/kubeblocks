package instancetemplate

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestInstanceTemplate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceTemplate Suite")
}
