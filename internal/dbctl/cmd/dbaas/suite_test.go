package dbaas

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDbaas(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dbaas Suite")
}
