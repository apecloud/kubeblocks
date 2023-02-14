/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package engine

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Mysql Engine", func() {
	It("connection example", func() {
		mysql := newMySQL()

		info := &ConnectionInfo{
			User:     "user",
			Host:     "host",
			Password: "*****",
			Database: "test-db",
			Port:     "1234",
		}
		for k := range mysql.examples {
			fmt.Printf("%s Connection Example\n", k.String())
			fmt.Println(mysql.ConnectExample(info, k.String()))
		}

		fmt.Println(mysql.ConnectExample(info, ""))
	})
})
