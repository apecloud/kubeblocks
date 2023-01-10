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
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var _ = Describe("DataProtection", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory
	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace("default")
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	Context("backup", func() {
		It("without name", func() {
			o := &CreateBackupOptions{}
			o.IOStreams = streams
			Expect(o.Validate()).To(MatchError("missing cluster name"))
		})

		It("new command", func() {
			cmd := NewCreateBackupCmd(tf, streams)
			Expect(cmd).ShouldNot(BeNil())
			// must succeed otherwise exit 1 and make test fails
			_ = cmd.Flags().Set("backup-type", "snapshot")
			cmd.Run(nil, []string{"test1"})
		})
	})

	It("delete-backup", func() {
		By("test delete-backup cmd")
		cmd := NewDeleteBackupCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		args := []string{"test1"}
		clusterLabel := util.BuildLabelSelectorByNames("", args)

		By("test delete-backup with cluster")
		o := delete.NewDeleteOptions(tf, streams, types.BackupGVR())
		Expect(completeForDeleteBackup(o, args)).Should(HaveOccurred())

		By("test delete-backup with cluster and force")
		o.Force = true
		Expect(completeForDeleteBackup(o, args)).Should(Succeed())
		Expect(o.LabelSelector == clusterLabel).Should(BeTrue())

		By("test delete-backup with cluster and force and labels")
		o.Force = true
		customLabel := "test=test"
		o.LabelSelector = customLabel
		Expect(completeForDeleteBackup(o, args)).Should(Succeed())
		Expect(o.LabelSelector == customLabel+","+clusterLabel).Should(BeTrue())
	})

	It("list-backup", func() {
		cmd := NewListBackupCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("delete-restore", func() {
		By("test delete-restore cmd")
		cmd := NewDeleteRestoreCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		args := []string{"test1"}
		clusterLabel := util.BuildLabelSelectorByNames("", args)

		By("test delete-restore with cluster")
		o := delete.NewDeleteOptions(tf, streams, types.BackupGVR())
		Expect(completeForDeleteRestore(o, args)).Should(HaveOccurred())

		By("test delete-restore with cluster and force")
		o.Force = true
		Expect(completeForDeleteRestore(o, args)).Should(Succeed())
		Expect(o.LabelSelector == clusterLabel).Should(BeTrue())

		By("test delete-restore with cluster and force and labels")
		o.Force = true
		customLabel := "test=test"
		o.LabelSelector = customLabel
		Expect(completeForDeleteRestore(o, args)).Should(Succeed())
		Expect(o.LabelSelector == customLabel+","+clusterLabel).Should(BeTrue())
	})

	It("list-restore", func() {
		cmd := NewListRestoreCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("restore", func() {
		tf.FakeDynamicClient = testing.FakeDynamicClient(testing.FakeClusterDef(), testing.FakeClusterVersion())
		timestamp := time.Now().Format("20060102150405")
		backupName := "backup-test-" + timestamp
		clusterName := "source-cluster-" + timestamp
		newClusterName := "new-cluster-" + timestamp

		// create test cluster
		cmd := NewCreateCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.Flags().Set("cluster-definition", testing.ClusterDefName)).Should(Succeed())
		Expect(cmd.Flags().Set("cluster-version", testing.ClusterVersionName)).Should(Succeed())
		Expect(cmd.Flags().Set("components", "../../testing/testdata/component.yaml")).Should(Succeed())
		Expect(cmd.Flags().Set("termination-policy", "Delete")).Should(Succeed())
		cmd.Run(nil, []string{clusterName})

		// create backup
		cmd = NewCreateBackupCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		_ = cmd.Flags().Set("backup-type", "snapshot")
		_ = cmd.Flags().Set("backup-name", backupName)
		cmd.Run(nil, []string{clusterName})

		// mock labels backup
		labels := fmt.Sprintf(`{"metadata":{"labels": {"app.kubernetes.io/instance":"%s"}}}`, clusterName)
		patchByte := []byte(labels)
		_, _ = tf.FakeDynamicClient.Resource(types.BackupGVR()).Namespace("default").Patch(context.TODO(), backupName,
			k8sapitypes.MergePatchType, patchByte, metav1.PatchOptions{})

		// create restore cluster
		cmdRestore := NewCreateRestoreCmd(tf, streams)
		Expect(cmdRestore != nil).To(BeTrue())
		_ = cmdRestore.Flags().Set("backup", backupName)
		cmdRestore.Run(nil, []string{newClusterName})
	})
})
