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

package backuprepo

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/pkg/cli/testing"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

var _ = Describe("backuprepo list command", func() {
	var streams genericiooptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericiooptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		repoObj1 := testing.FakeBackupRepo("test-backuprepo", false)
		repoObj2 := testing.FakeBackupRepo("default-backuprepo", true)
		backupObj := testing.FakeBackup("backup1")
		backupObj.Labels = map[string]string{associatedBackupRepoKey: "default-backuprepo"}
		backupObj.Status.Phase = dpv1alpha1.BackupPhaseCompleted
		backupObj.Status.TotalSize = "123456"
		tf.FakeDynamicClient = testing.FakeDynamicClient(repoObj1, repoObj2, backupObj)
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("should run", func() {
		cmd := newListCommand(tf, streams)
		cmd.SetArgs([]string{""})
		err := cmd.Execute()
		Expect(err).ShouldNot(HaveOccurred())
	})
})
