package bench

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBench(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bench Suite")
}
