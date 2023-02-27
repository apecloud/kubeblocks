package addon

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAppp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Addon Cmd Test Suite")
}
