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

package dataprotection

import (
	"context"
	"time"

	"github.com/sethvargo/go-password/password"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"

	. "github.com/onsi/ginkgo"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var _ = Describe("BackupPolicy Controller", func() {

	const timeout = time.Second * 10
	const interval = time.Second * 1
	const waitDuration = time.Second * 3

	var ctx = context.Background()

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &dataprotectionv1alpha1.BackupTool{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dataprotectionv1alpha1.BackupPolicyTemplate{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	genarateNS := func(prefix string) types.NamespacedName {
		randomStr, _ := password.Generate(6, 0, 0, true, false)
		key := types.NamespacedName{
			Name:      prefix + randomStr,
			Namespace: "default",
		}
		return key
	}

	assureBackupPolicyObj := func(backupPolicyTemplate string) *dataprotectionv1alpha1.BackupPolicy {
		By("By assure an backupPolicy obj")
		backupPolicyYaml := `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupPolicy
metadata:
  name: backup-policy-
  namespace: default
spec:
  target:
    labelsSelector:
      matchLabels:
        app.kubernetes.io/instance: wesql-cluster
    secret:
      keyPassword: password
      keyUser: username
      name: wesql-cluster
`
		backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
		Expect(yaml.Unmarshal([]byte(backupPolicyYaml), backupPolicy)).Should(Succeed())
		ns := genarateNS("backup-policy-")
		backupPolicy.Name = ns.Name
		backupPolicy.Namespace = ns.Namespace
		backupPolicy.Spec.BackupPolicyTemplateName = backupPolicyTemplate
		Expect(testCtx.CheckedCreateObj(ctx, backupPolicy)).Should(Succeed())
		return backupPolicy
	}

	deleteBackupPolicyNWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dataprotectionv1alpha1.BackupPolicy{}
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(ctx, f)
		}()).Should(Succeed())

		var err error
		f := &dataprotectionv1alpha1.BackupPolicy{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(ctx, key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(ctx, key, f) {
			f = &dataprotectionv1alpha1.BackupPolicy{}
		}
		return client.IgnoreNotFound(err)
	}

	assureBackupPolicyTemplateObj := func(backupTool string) *dataprotectionv1alpha1.BackupPolicyTemplate {
		By("By assure an backupPolicyTemplate obj")
		backupPolicyYaml := `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupPolicyTemplate
metadata:
  name: backup-policy-template-demo
  namespace: default
spec:
  schedule: "0 3 * * *"
  ttl: 168h0m0s
  backupToolName: xtrabackup-mysql
  databaseEngine: mysql
  onFailAttempted: 3
  remoteVolume:
    name: backup-remote-volume
    persistentVolumeClaim:
      claimName: backup-s3-pvc
`
		backupPolicyTemp := &dataprotectionv1alpha1.BackupPolicyTemplate{}
		Expect(yaml.Unmarshal([]byte(backupPolicyYaml), backupPolicyTemp)).Should(Succeed())
		ns := genarateNS("backup-policy-template-")
		backupPolicyTemp.Name = ns.Name
		backupPolicyTemp.Namespace = ns.Namespace
		backupPolicyTemp.Spec.BackupToolName = backupTool
		Expect(testCtx.CheckedCreateObj(ctx, backupPolicyTemp)).Should(Succeed())
		return backupPolicyTemp
	}

	deleteBackupPolicyTemplateNWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dataprotectionv1alpha1.BackupPolicyTemplate{}
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(ctx, f)
		}()).Should(Succeed())

		var err error
		f := &dataprotectionv1alpha1.BackupPolicyTemplate{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(ctx, key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(ctx, key, f) {
			f = &dataprotectionv1alpha1.BackupPolicyTemplate{}
		}
		return client.IgnoreNotFound(err)
	}

	assureBackupToolObj := func() *dataprotectionv1alpha1.BackupTool {
		By("By assure an backupTool obj")
		const backupToolYaml = `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupTool
metadata:
  name: xtrabackup-mysql
  namespace: default
spec:
  image: percona/percona-xtrabackup
  databaseEngine: mysql
  deployKind: job
  resources:
    limits:
      cpu: "1"
      memory: 2Gi

  env:
    - name: DATA_DIR
      value: /var/lib/mysql

    - name: BACKUP_DIR_PREFIX
      valueFrom:
        fieldRef:
          fieldPath: metadata.namespace

    - name: BACKUP_DIR
      value: /data/$(BACKUP_DIR_PREFIX)

  restoreCommands:
    - |
      echo "BACKUP_DIR=${BACKUP_DIR} BACKUP_NAME=${BACKUP_NAME} DATA_DIR=${DATA_DIR}" && \
      mkdir -p /tmp/data/ && cd /tmp/data \
      && xbstream -x < /${BACKUP_DIR}/${BACKUP_NAME}.xbstream \
      && xtrabackup --decompress  --target-dir=/tmp/data/ \
      && find . -name "*.qp"|xargs rm -f \
      && rm -rf ${DATA_DIR}/* \
      && rsync -avrP /tmp/data/ ${DATA_DIR}/ \
      && rm -rf /tmp/data/ \
      && chmod -R 0777 ${DATA_DIR}

  # Optional
  incrementalRestoreCommands: []
  backupCommands:
    - echo "DB_HOST=${DB_HOST} DB_USER=${DB_USER} DB_PASSWORD=${DB_PASSWORD} DATA_DIR=${DATA_DIR} BACKUP_DIR=${BACKUP_DIR} BACKUP_NAME=${BACKUP_NAME}";
      mkdir -p /${BACKUP_DIR};
      xtrabackup --compress --backup  --safe-slave-backup --slave-info --stream=xbstream --host=${DB_HOST} --user=${DB_USER} --password=${DB_PASSWORD} --datadir=${DATA_DIR} > /${BACKUP_DIR}/${BACKUP_NAME}.xbstream
  # Optional
  incrementalBackupCommands: []
`
		backupTool := &dataprotectionv1alpha1.BackupTool{}
		Expect(yaml.Unmarshal([]byte(backupToolYaml), backupTool)).Should(Succeed())
		ns := genarateNS("backup-tool-")
		backupTool.Name = ns.Name
		backupTool.Namespace = ns.Namespace
		Expect(testCtx.CheckedCreateObj(ctx, backupTool)).Should(Succeed())
		return backupTool
	}

	deleteBackupToolNWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dataprotectionv1alpha1.BackupTool{}
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(ctx, f)
		}()).Should(Succeed())

		var err error
		f := &dataprotectionv1alpha1.BackupTool{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(ctx, key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(ctx, key, f) {
			f = &dataprotectionv1alpha1.BackupTool{}
		}
		return client.IgnoreNotFound(err)
	}

	assureBackupPolicyWithoutTemplate := func() *dataprotectionv1alpha1.BackupPolicy {
		By("By assure an backupPolicy obj")
		backupPolicyYaml := `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupPolicy
metadata:
  name: backup-policy-
  namespace: default
spec:
  target:
    labelsSelector:
      matchLabels:
        app.kubernetes.io/instance: wesql-cluster
    secret:
      keyPassword: password
      keyUser: username
      name: wesql-cluster
`
		backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
		Expect(yaml.Unmarshal([]byte(backupPolicyYaml), backupPolicy)).Should(Succeed())
		ns := genarateNS("backup-policy-")
		backupPolicy.Name = ns.Name
		backupPolicy.Namespace = ns.Namespace
		Expect(testCtx.CheckedCreateObj(ctx, backupPolicy)).Should(Succeed())
		return backupPolicy
	}

	Context("When creating backupPolicy with Template", func() {
		It("Should success with no error", func() {

			By("By creating a backupTool")
			backupTool := assureBackupToolObj()

			By("By creating a backupPolicyTemplate from backupTool: " + backupTool.Name)
			template := assureBackupPolicyTemplateObj(backupTool.Name)

			By("By creating a backupPolicy from template: " + template.Name)
			toCreate := assureBackupPolicyObj(template.Name)
			key := types.NamespacedName{
				Name:      toCreate.Name,
				Namespace: toCreate.Namespace,
			}
			time.Sleep(waitDuration)
			result := &dataprotectionv1alpha1.BackupPolicy{}
			Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())
			Expect(result.Spec.Schedule).Should(Equal(template.Spec.Schedule))
			Expect(result.Spec.RemoteVolume.Name).Should(Equal(template.Spec.RemoteVolume.Name))
			Expect(result.Spec.Hooks.PreCommands).Should(Equal(template.Spec.Hooks.PreCommands))
			Expect(result.Spec.Hooks.PostCommands).Should(Equal(template.Spec.Hooks.PostCommands))
			Expect(result.Spec.Target.DatabaseEngine).Should(Equal(template.Spec.DatabaseEngine))
			Expect(*result.Spec.TTL).Should(Equal(template.Spec.TTL))
			Expect(result.Spec.BackupToolName).Should(Equal(template.Spec.BackupToolName))

			By("Deleting the scope")

			Eventually(func() error {
				_ = deleteBackupToolNWait(types.NamespacedName{Namespace: backupTool.Namespace, Name: backupTool.Name})
				_ = deleteBackupPolicyTemplateNWait(types.NamespacedName{Namespace: template.Namespace, Name: template.Name})
				return deleteBackupPolicyNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating backupPolicy without Template", func() {
		It("Should success with no error", func() {

			By("By creating a backupTool")
			backupTool := assureBackupToolObj()

			By("By creating a backupPolicy from empty template")
			toCreate := assureBackupPolicyWithoutTemplate()
			key := types.NamespacedName{
				Name:      toCreate.Name,
				Namespace: toCreate.Namespace,
			}
			time.Sleep(waitDuration)
			result := &dataprotectionv1alpha1.BackupPolicy{}
			Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())

			By("Deleting the scope")

			Eventually(func() error {
				_ = deleteBackupToolNWait(types.NamespacedName{Namespace: backupTool.Namespace, Name: backupTool.Name})
				return deleteBackupPolicyNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

})
