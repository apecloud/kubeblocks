/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package service

import (
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("marshal task event with size limit", func() {
	It("keeps small task events unchanged", func() {
		event := proto.TaskEvent{
			Instance: "comp",
			Task:     "newReplica",
			Replica:  "pod-1",
			Message:  "ok",
		}
		msg, err := marshalEventWithSizeLimit(&event, &event.Message, &event.Output)
		Expect(err).Should(BeNil())
		plain, err := json.Marshal(&event)
		Expect(err).Should(BeNil())
		Expect(msg).Should(Equal(plain))
	})

	It("truncates a long task failure message but keeps its head and valid JSON", func() {
		event := proto.TaskEvent{
			Instance: "comp",
			Task:     "newReplica",
			Replica:  "pod-1",
			Code:     -1,
			Message:  "replication setup failed: connection refused\n" + strings.Repeat("verbose replication log line\n", 200),
		}
		msg, err := marshalEventWithSizeLimit(&event, &event.Message, &event.Output)
		Expect(err).Should(BeNil())
		Expect(len(msg)).Should(BeNumerically("<=", maxEventMessageLength))

		var decoded proto.TaskEvent
		Expect(json.Unmarshal(msg, &decoded)).Should(Succeed())
		Expect(decoded.Code).Should(Equal(int32(-1)))
		Expect(decoded.Message).Should(HavePrefix("replication setup failed"))
		Expect(decoded.Message).Should(HaveSuffix("...(truncated)"))
	})

	It("shrinks oversized task output while keeping valid JSON", func() {
		event := proto.TaskEvent{
			Instance: "comp",
			Task:     "newReplica",
			Output:   []byte(strings.Repeat("y", 8192)),
		}
		msg, err := marshalEventWithSizeLimit(&event, &event.Message, &event.Output)
		Expect(err).Should(BeNil())
		Expect(len(msg)).Should(BeNumerically("<=", maxEventMessageLength))

		var decoded proto.TaskEvent
		Expect(json.Unmarshal(msg, &decoded)).Should(Succeed())
		Expect(decoded.Task).Should(Equal("newReplica"))
	})
})
