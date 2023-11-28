package oracle

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

var _ = Describe("Oracle Engine", func() {
	It("connection command", func() {
		oracle := NewCommands()

		Expect(oracle.ConnectCommand(nil)).ShouldNot(BeNil())
		authInfo := &engines.AuthInfo{
			UserName:   "user-test",
			UserPasswd: "pwd-test",
		}
		Expect(oracle.ConnectCommand(authInfo)).ShouldNot(BeNil())
	})

	It("connection example", func() {
		oracle := NewCommands().(*Commands)

		info := &engines.ConnectionInfo{
			User:     "user",
			Host:     "host",
			Password: "*****",
			Port:     "1234",
		}
		for k := range oracle.examples {
			fmt.Printf("%s Connection Example\n", k.String())
			Expect(oracle.ConnectExample(info, k.String())).ShouldNot(BeZero())
		}

		Expect(oracle.ConnectExample(info, "")).ShouldNot(BeZero())
	})

})
