package describe

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDescribe(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Describe Suite")
}
