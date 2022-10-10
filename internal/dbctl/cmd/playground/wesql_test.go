/*
Copyright 2022.

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

package playground

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("wesql", func() {
	wesql := &Wesql{
		serverVersion: wesqlVersion,
		replicas:      1,
	}

	It("Get repos", func() {
		repos := wesql.getRepos()
		Expect(repos != nil).To(BeTrue())
		Expect(len(repos)).To(Equal(0))
	})

	It("Get base charts", func() {
		charts := wesql.getBaseCharts("test")
		Expect(charts != nil).Should(BeTrue())
		Expect(len(charts)).To(Equal(1))
	})

	It("Get database charts", func() {
		charts := wesql.getDBCharts("test", "test")
		Expect(charts != nil).Should(BeTrue())
		Expect(len(charts)).To(Equal(1))
		Expect(charts[0].Chart).To(Equal(wesqlHelmChart))
	})
})
