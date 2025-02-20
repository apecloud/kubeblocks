/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("reconfigure", func() {
	Context("reconfigure", func() {
		var (
			localFile = "log.conf"
		)

		createFile := func() (string, string) {
			f, err1 := os.Create(localFile)
			Expect(err1).Should(BeNil())
			str := fmt.Sprintf("%s - d21afb29f88140d502f6957b4d5f8379", time.Now().String())
			cnt, err2 := f.WriteString(str)
			Expect(err2).Should(BeNil())
			Expect(cnt).Should(Equal(len(str)))
			Expect(f.Close()).Should(BeNil())

			return localFile, fmt.Sprintf("%x", sha256.Sum256([]byte(str)))
		}

		removeFile := func() {
			Expect(os.Remove(localFile)).Should(BeNil())
		}

		It("not a reconfigure request", func() {
			req := &proto.ActionRequest{
				Action: "switchover",
			}
			err := checkReconfigure(ctx, req)
			Expect(err).Should(BeNil())
		})

		It("empty parameter", func() {
			req := &proto.ActionRequest{
				Action:     "reconfigure",
				Parameters: map[string]string{},
			}
			err := checkReconfigure(ctx, req)
			Expect(err).Should(BeNil())
		})

		It("bad request", func() {
			req := &proto.ActionRequest{
				Action: "reconfigure",
				Parameters: map[string]string{
					configFilesUpdated: "log.conf",
				},
			}
			err := checkReconfigure(ctx, req)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrBadRequest)).Should(BeTrue())
		})

		It("check failed", func() {
			file, checksum := createFile()
			defer removeFile()

			req := &proto.ActionRequest{
				Action: "reconfigure",
				Parameters: map[string]string{
					configFilesUpdated: fmt.Sprintf("%s:%s++", file, checksum),
				},
			}
			err := checkReconfigure(ctx, req)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrPreconditionFailed)).Should(BeTrue())
		})

		It("ok", func() {
			file, checksum := createFile()
			defer removeFile()

			req := &proto.ActionRequest{
				Action: "reconfigure",
				Parameters: map[string]string{
					configFilesUpdated: fmt.Sprintf("%s:%s", file, checksum),
				},
			}
			err := checkReconfigure(ctx, req)
			Expect(err).Should(BeNil())
		})
	})
})
