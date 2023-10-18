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

package configmanager

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/DATA-DOG/go-sqlmock"

	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
)

var _ = Describe("Handler Util Test", func() {

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
	})

	AfterEach(func() {
	})

	Context("TestReconfiguringWrapper", func() {
		It("TestNewCommandChannel", func() {
			By("testMysql")
			_, err := NewCommandChannel(ctx, mysql, "")
			Expect(err).ShouldNot(Succeed())
			_, err = NewCommandChannel(ctx, mysql, "dsn")
			Expect(err).ShouldNot(Succeed())

			By("testPatroni")
			_, err = NewCommandChannel(ctx, patroni, "")
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("PATRONI_REST_API_URL"))

			_, err = NewCommandChannel(ctx, patroni, "localhost")
			Expect(err).Should(Succeed())

			By("testUnsupport")
			_, err = NewCommandChannel(ctx, "", "localhost")
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("not supported type"))
		})
	})

	Context("TestMysqlDB", func() {

		It("TestMysqlDB connect failed", func() {
			db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
			Expect(err).Should(Succeed())
			defer db.Close()

			mock.ExpectPing().WillReturnError(cfgcore.MakeError("ping error"))

			_, err = newDynamicParamUpdater(ctx, db)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("ping error"))
		})

		It("TestMysqlDB update parameter", func() {
			testSQLString := "SET GLOBAL max_connections = 1000"

			db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
			Expect(err).Should(Succeed())

			mock.ExpectPing().WillDelayFor(10 * time.Millisecond)
			mock.ExpectExec(testSQLString).WillReturnError(cfgcore.MakeError("failed to set parameter"))
			mock.ExpectExec(testSQLString).WillReturnResult(sqlmock.NewResult(0, 0))

			By("create DynamicParamUpdater")
			dUpdater, err := newDynamicParamUpdater(ctx, db)
			Expect(err).Should(Succeed())
			defer dUpdater.Close()

			By("test dynamic parameter")
			_, err = dUpdater.ExecCommand(ctx, testSQLString)
			Expect(err).ShouldNot(Succeed())
			r, err := dUpdater.ExecCommand(ctx, testSQLString)
			Expect(err).Should(Succeed())
			Expect("0").Should(BeEquivalentTo(r))
		})

	})
})
