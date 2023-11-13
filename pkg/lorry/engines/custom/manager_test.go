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

package custom

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("ETCD DBManager", func() {
	// Set up relevant viper config variables
	Context("new db manager", func() {
		It("with right configurations", func() {
			viper.Set("KB_RSM_ACTION_SVC_LIST", "[3502]")
			properties := engines.Properties{}
			dbManger, err := NewManager(properties)
			Expect(err).Should(Succeed())
			Expect(dbManger).ShouldNot(BeNil())
		})

		It("with wrong configurations", func() {
			viper.Set("KB_RSM_ACTION_SVC_LIST", "wrong-setting")
			properties := engines.Properties{}
			dbManger, err := NewManager(properties)
			Expect(err).Should(HaveOccurred())
			Expect(dbManger).Should(BeNil())
		})
	})

	Context("global role snapshot", func() {
		It("success", func() {
			_ = setUpHost()
			manager, err := NewManager(nil)
			Expect(err).Should(BeNil())
			globalRole, err := manager.GetReplicaRole(context.TODO(), nil)
			Expect(err).Should(BeNil())
			snapshot := &common.GlobalRoleSnapshot{}
			Expect(json.Unmarshal([]byte(globalRole), snapshot)).Should(Succeed())
			Expect(snapshot.PodRoleNamePairs).Should(HaveLen(3))
			Expect(snapshot.Version).Should(Equal("1"))
		})
	})
})

func setUpHost() *httptest.Server {
	var lines []string
	for i := 0; i < 3; i++ {
		podName := "pod-" + strconv.Itoa(i)
		var role string
		if i == 0 {
			role = "leader"
		} else {
			role = "follower"
		}
		lines = append(lines, fmt.Sprintf("%d,%s,%s", 1, podName, role))
	}
	respContent := strings.Join(lines, "\n")

	s := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			_, _ = w.Write([]byte(respContent))
		}),
	)
	addr := s.Listener.Addr().String()
	index := strings.LastIndex(addr, ":")
	portStr := addr[index+1:]
	viper.Set("KB_RSM_ACTION_SVC_LIST", "["+portStr+"]")
	return s
}
