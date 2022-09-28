package list

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestList(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "List Suite")
}
