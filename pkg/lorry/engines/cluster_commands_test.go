/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package engines

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

var _ = Describe("Cluster commands", func() {
	It("new commands", func() {
		for _, engineType := range []models.EngineType{models.MySQL, models.PostgreSQL, models.Redis, models.PostgreSQL, models.Nebula, models.FoxLake} {
			typeName := string(engineType)
			engine, _ := newClusterCommands(typeName)
			Expect(engine).Should(BeNil())
		}
	})

	It("new unknown engine", func() {
		typeName := "unknown-type"
		engine, err := newClusterCommands(typeName)
		Expect(engine).Should(BeNil())
		Expect(err).Should(HaveOccurred())
	})
})
