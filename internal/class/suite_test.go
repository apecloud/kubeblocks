package class

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestClass(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Class Test Suite")
}
