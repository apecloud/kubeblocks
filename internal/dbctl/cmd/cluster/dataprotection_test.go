/*
Copyright ApeCloud Inc.

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

package cluster

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmddelete "k8s.io/kubectl/pkg/cmd/delete"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/dbctl/delete"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

var _ = Describe("DataProtection", func() {
	var streams genericclioptions.IOStreams
	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
	})

	Context("backup", func() {
		It("without name", func() {
			o := &CreateBackupOptions{}
			o.IOStreams = streams
			Expect(o.Validate()).To(MatchError("missing cluster name"))
		})

		It("new command", func() {
			tf := cmdtesting.NewTestFactory().WithNamespace("default")
			defer tf.Cleanup()
			tf.ClientConfigVal = cfg
			cmd := NewCreateBackupCmd(tf, streams)
			Expect(cmd != nil).To(BeTrue())
			// must succeed otherwise exit 1 and make test fails
			_ = cmd.Flags().Set("backup-type", "snapshot")
			cmd.Run(nil, []string{"test1"})
		})
	})

	It("delete-backup", func() {
		By("test delete-backup cmd")
		tf := cmdtesting.NewTestFactory().WithNamespace("default")
		defer tf.Cleanup()
		cmd := NewDeleteBackupCmd(tf, streams)
		Expect(cmd != nil).To(BeTrue())

		clusterName := "test1"
		clusterLabel := fmt.Sprintf("%s=%s", types.InstanceLabelKey, clusterName)

		By("test delete-backup with cluster")
		deleteFlags := &delete.DeleteFlags{
			DeleteFlags: cmddelete.NewDeleteCommandFlags("containing the resource to delete."),
		}
		Expect(completeForDeleteBackup(deleteFlags, []string{clusterName})).Should(HaveOccurred())

		By("test delete-backup with cluster and force")
		deleteFlags = &delete.DeleteFlags{
			DeleteFlags: cmddelete.NewDeleteCommandFlags("containing the resource to delete."),
		}
		deleteForce := true
		deleteFlags.Force = &deleteForce
		Expect(completeForDeleteBackup(deleteFlags, []string{clusterName})).Should(Succeed())
		Expect(*deleteFlags.LabelSelector == clusterLabel).Should(BeTrue())

		By("test delete-backup with cluster and force and labels")
		deleteFlags = &delete.DeleteFlags{
			DeleteFlags: cmddelete.NewDeleteCommandFlags("containing the resource to delete."),
		}
		deleteFlags.Force = &deleteForce
		customLabel := "test=test"
		deleteFlags.LabelSelector = &customLabel
		Expect(completeForDeleteBackup(deleteFlags, []string{clusterName})).Should(Succeed())
		Expect(*deleteFlags.LabelSelector == customLabel+","+clusterLabel).Should(BeTrue())
	})

	It("list-backup", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("default")
		defer tf.Cleanup()
		cmd := NewListBackupCmd(tf, streams)
		Expect(cmd != nil).To(BeTrue())
	})

	It("delete-restore", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("default")
		defer tf.Cleanup()

		By("test delete-restore cmd")
		cmd := NewDeleteRestoreCmd(tf, streams)
		Expect(cmd != nil).To(BeTrue())

		clusterName := "test1"
		clusterLabel := fmt.Sprintf("%s=%s", types.InstanceLabelKey, clusterName)

		By("test delete-restore with cluster")
		deleteFlags := &delete.DeleteFlags{
			DeleteFlags: cmddelete.NewDeleteCommandFlags("containing the resource to delete."),
		}
		Expect(completeForDeleteRestore(deleteFlags, []string{clusterName})).Should(HaveOccurred())

		By("test delete-restore with cluster and force")
		deleteFlags = &delete.DeleteFlags{
			DeleteFlags: cmddelete.NewDeleteCommandFlags("containing the resource to delete."),
		}
		deleteForce := true
		deleteFlags.Force = &deleteForce
		Expect(completeForDeleteRestore(deleteFlags, []string{clusterName})).Should(Succeed())
		Expect(*deleteFlags.LabelSelector == clusterLabel).Should(BeTrue())

		By("test delete-restore with cluster and force and labels")
		deleteFlags = &delete.DeleteFlags{
			DeleteFlags: cmddelete.NewDeleteCommandFlags("containing the resource to delete."),
		}
		deleteFlags.Force = &deleteForce
		customLabel := "test=test"
		deleteFlags.LabelSelector = &customLabel
		Expect(completeForDeleteRestore(deleteFlags, []string{clusterName})).Should(Succeed())
		Expect(*deleteFlags.LabelSelector == customLabel+","+clusterLabel).Should(BeTrue())
	})

	It("list-restore", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("default")
		defer tf.Cleanup()
		cmd := NewListRestoreCmd(tf, streams)
		Expect(cmd != nil).To(BeTrue())
	})

	It("restore", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("default")
		defer tf.Cleanup()

		timestamp := time.Now().Format("20060102150405")
		backupName := "backup-test-" + timestamp
		clusterName := "source-cluster-" + timestamp
		newClusterName := "new-cluster-" + timestamp

		// create test cluster
		cmd := NewCreateCmd(tf, streams)
		Expect(cmd != nil).To(BeTrue())
		_ = cmd.Flags().Set("components", "../../testdata/component.yaml")
		cmd.Run(nil, []string{clusterName})

		// create backup
		cmd = NewCreateBackupCmd(tf, streams)
		Expect(cmd != nil).To(BeTrue())
		_ = cmd.Flags().Set("backup-type", "snapshot")
		_ = cmd.Flags().Set("backup-name", backupName)
		cmd.Run(nil, []string{clusterName})

		// mock labels backup
		labels := fmt.Sprintf(`{"metadata":{"labels": {"app.kubernetes.io/instance":"%s"}}}`, clusterName)
		patchByte := []byte(labels)
		_, _ = tf.FakeDynamicClient.Resource(types.BackupJobGVR()).Namespace("default").Patch(ctx, backupName,
			k8sapitypes.MergePatchType, patchByte, metav1.PatchOptions{})

		// create restore cluster
		cmd_restore := NewCreateRestoreCmd(tf, streams)
		Expect(cmd_restore != nil).To(BeTrue())
		_ = cmd_restore.Flags().Set("backup", backupName)
		cmd_restore.Run(nil, []string{newClusterName})
	})
})
